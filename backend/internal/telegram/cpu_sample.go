package telegram

import (
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// cpuPercentSample returns a single-element slice giving the host's overall
// CPU utilization over a short sampling window. The 150ms blocking interval
// is acceptable here because callers run inside the notifier's periodic
// scheduler tick — not on a request hot path.
func cpuPercentSample() ([]float64, error) {
	return cpu.Percent(150*time.Millisecond, false)
}

// memoryPercentSample returns the host's overall RAM used-percentage at the
// instant of the call. Unlike CPU sampling this is instantaneous (no window),
// because gopsutil's mem.VirtualMemory() reads /proc/meminfo directly.
func memoryPercentSample() (float64, error) {
	m, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return m.UsedPercent, nil
}
