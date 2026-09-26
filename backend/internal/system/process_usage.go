package system

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// ProcessUsage is RSS + CPU for a single OS process (panel or Mihomo core).
//
// Both percentages are expressed against the capacity the dashboard reports
// for the whole system: the cgroup CFS quota when limited, otherwise the CPUs
// online. A single-threaded process pegging one of four cores therefore reads
// 25% instead of top(1)'s 100% — the same unit as the "system resources" card
// beside it, which is what lets the two cards be compared at a glance.
type ProcessUsage struct {
	PID           int     `json:"pid"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsed    float64 `json:"memory_used"`    // RSS bytes
	MemoryPercent float64 `json:"memory_percent"` // 0–100 of system RAM
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
	// Percentage against the same total the dashboard shows for system memory
	// (cgroup limit inside a container, host RAM otherwise). gopsutil's
	// MemoryPercent divides by its own total, which diverges from the system
	// card whenever cgroup accounting is in play — the two cards then disagree
	// about what 100% means.
	if total := memoryTotalForPercent(); total > 0 && out.MemoryUsed > 0 {
		out.MemoryPercent = clampPercent(out.MemoryUsed / total * 100)
	} else if mp, err := p.MemoryPercent(); err == nil {
		out.MemoryPercent = clampPercent(float64(mp))
	}
	out.CPUPercent = sampleProcessCPU(p)
	return out
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
