package system

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// ProcessUsage is RSS + CPU for a single OS process (panel or Mihomo core).
type ProcessUsage struct {
	PID           int     `json:"pid"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsed    float64 `json:"memory_used"`    // RSS bytes
	MemoryPercent float64 `json:"memory_percent"` // 0–100 of system RAM
}

var (
	procCPUMu   sync.Mutex
	procCPULast = map[int32]struct {
		pct float64
		at  time.Time
	}{}
)

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

// SampleProcessUsage returns CPU/memory for pid. CPU uses gopsutil's delta
// sampler (first call after a gap may be ~0); recent values are cached briefly.
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
	// Non-blocking percent since last sample for this PID.
	if pct, err := p.CPUPercent(); err == nil {
		v := clampPercent(pct)
		procCPUMu.Lock()
		procCPULast[int32(pid)] = struct {
			pct float64
			at  time.Time
		}{pct: v, at: time.Now()}
		procCPUMu.Unlock()
		out.CPUPercent = v
		return out
	}
	procCPUMu.Lock()
	if last, ok := procCPULast[int32(pid)]; ok && time.Since(last.at) < 30*time.Second {
		out.CPUPercent = last.pct
	}
	procCPUMu.Unlock()
	return out
}
