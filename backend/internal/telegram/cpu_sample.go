package telegram

import (
	"errors"

	"github.com/kazeyukiro/3m-ui/backend/internal/system"
)

// errSampleUnavailable is returned when neither the panel's own accounting nor
// /proc could produce a reading.
var errSampleUnavailable = errors.New("system metrics unavailable")

// cpuPercentSample returns a single-element slice giving overall CPU
// utilization, sourced from the same sampler that feeds the dashboard.
//
// Delegating matters because the two must agree: reading /proc/stat directly
// here meant /proc/stat's host-wide arithmetic again, so inside a container
// pinned to a fraction of the machine the alert could never reach its
// threshold even while the panel was throttled. One sampler, one answer.
//
// Unlike the old 150ms blocking window this only re-samples when the shared
// TTL has expired, so a notifier tick costs nothing when idle.
func cpuPercentSample() ([]float64, error) {
	stats := system.GetSystemStats()
	if stats == nil {
		return nil, errSampleUnavailable
	}
	return []float64{stats.CPU.Percent}, nil
}

// memoryPercentSample returns overall memory used-percent, likewise shared with
// the dashboard — cgroup accounting inside a container instead of host meminfo,
// which describes a budget this process cannot use.
func memoryPercentSample() (float64, error) {
	stats := system.GetSystemStats()
	if stats == nil {
		return 0, errSampleUnavailable
	}
	return stats.Memory.Percent, nil
}
