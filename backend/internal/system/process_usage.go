package system

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// ProcessUsage is CPU + memory for a single OS process (panel or Mihomo core).
//
// CPU percentages use the same capacity as the system card (cgroup CFS quota or
// online CPUs). Memory is reported on the **cgroup working-set** basis used by
// the system card and by systemd's MemoryCurrent: when the panel and core share
// one unit, SampleProcessUsagePair splits that cgroup figure by RSS share so the
// two rows sum to the service total instead of double-counting shared pages.
type ProcessUsage struct {
	PID           int     `json:"pid"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsed    float64 `json:"memory_used"`    // bytes (cgroup share, or RSS fallback)
	MemoryPercent float64 `json:"memory_percent"` // 0–100 of system/cgroup total
}

// procBaseline is one process CPU counter reading, differenced against the
// previous one to get a rate.
type procBaseline struct {
	// Cumulative user+system CPU seconds consumed by the PID.
	total float64
	at    time.Time
	pct   float64
}

const (
	// procBaselineWindow: see sampleCPU — one short wait buys a real delta so
	// the first render does not read 0%.
	procBaselineWindow = 120 * time.Millisecond
	// procBaselineTTL: how long an unseen PID keeps its delta state before it
	// is dropped, bounding the map on hosts that churn PIDs.
	procBaselineTTL = 5 * time.Minute
)

var (
	procCPUMu    sync.Mutex
	procCPUBase  = map[int32]*procBaseline{}
	procCPUSweep time.Time
)

// dropProcBaseline removes state for a PID that no longer exists, so a
// restarted core does not resume from the previous incarnation's counter.
func dropProcBaseline(pid int32) {
	procCPUMu.Lock()
	delete(procCPUBase, pid)
	procCPUMu.Unlock()
}

// reapProcBaselines discards state for PIDs that have not been sampled within
// procBaselineTTL. Swept lazily (at most once a minute) because every call site
// already holds the lock.
func reapProcBaselines(now time.Time) {
	if !procCPUSweep.IsZero() && now.Sub(procCPUSweep) < time.Minute {
		return
	}
	for pid, b := range procCPUBase {
		if now.Sub(b.at) >= procBaselineTTL {
			delete(procCPUBase, pid)
		}
	}
	procCPUSweep = now
}

// processCPUTotal returns cumulative CPU seconds genuinely spent on the CPU for
// the given handle.
//
// Only user+system are counted. The TimesStat also carries an Iowait field,
// but for a process gopsutil derives it from block-I/O delay ticks, which is
// waiting rather than consuming — adding it inflates the figure for I/O heavy
// workloads.
func processCPUTotal(p *process.Process) (float64, bool) {
	t, err := p.Times()
	if err != nil || t == nil {
		return 0, false
	}
	return t.User + t.System, true
}

// SampleProcessUsage returns CPU/memory for pid.
//
// CPU is deliberately measured by differencing /proc/<pid>/stat ourselves.
// gopsutil's Process.CPUPercent() cannot be used because it returns
// 100 * lifetimeCPUTime / uptime: an average over the process' entire history,
// not what it is doing now. A panel that has been up for a week reports a flat
// near-constant number that lags real spikes by hours, and when the start time
// cannot be resolved (BootTime from sysinfo disagrees with /proc/stat inside
// some containers/gVisor) totalTime goes negative and the call silently yields
// 0. Validate either way.
func SampleProcessUsage(pid int) ProcessUsage {
	out := sampleProcessUsageRaw(pid)
	// Single-process path: attribute the whole cgroup working set to this PID
	// so MemoryUsed stays ≤ system Memory.Used (same denominator as the system card).
	applyCgroupMemoryShares([]*ProcessUsage{&out})
	return out
}

// SampleProcessUsagePair samples panel and core together and splits the unit's
// cgroup working-set memory by RSS share. Shared library pages are no longer
// double-counted; panel.MemoryUsed + core.MemoryUsed matches systemd MemoryCurrent
// (and the system card's Used when both live in the same cgroup).
func SampleProcessUsagePair(panelPID, corePID int) (panel, core ProcessUsage) {
	panel = sampleProcessUsageRaw(panelPID)
	core = sampleProcessUsageRaw(corePID)
	applyCgroupMemoryShares([]*ProcessUsage{&panel, &core})
	return panel, core
}

// sampleProcessUsageRaw fills CPU and raw RSS; memory fields are adjusted by
// applyCgroupMemoryShares.
func sampleProcessUsageRaw(pid int) ProcessUsage {
	out := ProcessUsage{PID: pid}
	if pid <= 0 {
		return out
	}
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return out
	}
	if mi, err := p.MemoryInfo(); err == nil && mi != nil {
		out.MemoryUsed = float64(mi.RSS)
	}
	out.CPUPercent = sampleProcessCPU(p)
	return out
}

// applyCgroupMemoryShares rewrites MemoryUsed/MemoryPercent so the values use
// the same cgroup working set as sampleMemory(). Shares are proportional to
// each process' RSS (only a relative weight — the absolute bytes come from the
// cgroup). Processes with no RSS or outside a readable cgroup keep RSS-based
// figures against memoryTotalForPercent().
func applyCgroupMemoryShares(procs []*ProcessUsage) {
	if len(procs) == 0 {
		return
	}
	total := memoryTotalForPercent()
	cg := readServiceCgroupMemory()
	hostTotal := total
	info, ok := memoryFromCgroup(cg, hostTotal)
	if !ok || info.Used <= 0 {
		for _, u := range procs {
			if u == nil || u.PID <= 0 {
				continue
			}
			if total > 0 && u.MemoryUsed > 0 {
				u.MemoryPercent = clampPercent(u.MemoryUsed / total * 100)
			}
		}
		return
	}
	if info.Total > 0 {
		total = info.Total
	}

	var rssSum float64
	for _, u := range procs {
		if u != nil && u.PID > 0 && u.MemoryUsed > 0 {
			rssSum += u.MemoryUsed
		}
	}

	if rssSum <= 0 {
		// No RSS weights: give the whole working set to the first live PID.
		for _, u := range procs {
			if u != nil && u.PID > 0 {
				u.MemoryUsed = info.Used
				if total > 0 {
					u.MemoryPercent = clampPercent(u.MemoryUsed / total * 100)
				}
				return
			}
		}
		return
	}

	var assigned float64
	var last *ProcessUsage
	for _, u := range procs {
		if u == nil || u.PID <= 0 {
			continue
		}
		share := info.Used * (u.MemoryUsed / rssSum)
		u.MemoryUsed = share
		assigned += share
		last = u
		if total > 0 {
			u.MemoryPercent = clampPercent(u.MemoryUsed / total * 100)
		}
	}
	// Absorb floating error on the last process so the sum matches the cgroup.
	if last != nil {
		last.MemoryUsed += info.Used - assigned
		if last.MemoryUsed < 0 {
			last.MemoryUsed = 0
		}
		if total > 0 {
			last.MemoryPercent = clampPercent(last.MemoryUsed / total * 100)
		}
	}
}

// memoryTotalForPercent returns the memory total the dashboard displays for
// system memory, so per-process percentages are expressed against the same
// denominator. GetSystemStats is TTL-cached, so this costs nothing extra on a
// dashboard poll that already samples system memory.
func memoryTotalForPercent() float64 {
	if stats := GetSystemStats(); stats != nil {
		return stats.Memory.Total
	}
	return 0
}

// sampleProcessCPU returns pid's CPU use since the previous sample, as a
// percentage of the available CPU capacity.
func sampleProcessCPU(p *process.Process) float64 {
	pid := p.Pid
	capacity := cpuCapacity()
	if capacity <= 0 {
		return 0
	}

	now := time.Now()
	total, ok := processCPUTotal(p)
	if !ok {
		// The PID is gone (or no longer readable). Drop its baseline: if that
		// PID ever comes back it belongs to a different process.
		dropProcBaseline(pid)
		return 0
	}

	procCPUMu.Lock()
	defer procCPUMu.Unlock()
	reapProcBaselines(now)

	prev := procCPUBase[pid]
	cur := &procBaseline{total: total, at: now}
	procCPUBase[pid] = cur
	if prev != nil {
		cur.pct = prev.pct
	}

	if prev == nil {
		// No baseline yet: sample twice over a short window so even the first
		// render carries a measured value.
		time.Sleep(procBaselineWindow)
		second := time.Now()
		total2, ok := processCPUTotal(p)
		if !ok {
			dropProcBaselineLocked(pid)
			return 0
		}
		cur.total, cur.at = total2, second
		prev = &procBaseline{total: total, at: now}
	}

	elapsed := cur.at.Sub(prev.at).Seconds()
	delta := cur.total - prev.total
	if elapsed <= 0 {
		return clampPercent(cur.pct)
	}
	if delta < 0 {
		// The PID was recycled: these counters belong to a different process
		// now, so there is no valid delta. Report the last real reading rather
		// than something fabricated; the baseline was already reset above, so
		// the next call differences against this PID's actual start.
		return clampPercent(cur.pct)
	}
	// A zero delta is a legitimate "this process was idle". Only negative
	// deltas are impossible; echoing the previous value instead of reporting 0
	// would leave the card stuck at whatever it last peaked at, because a
	// tickless kernel accrues nothing for a fully sleeping process.
	cur.pct = delta / elapsed / capacity * 100
	return clampPercent(cur.pct)
}

func dropProcBaselineLocked(pid int32) {
	delete(procCPUBase, pid)
}
