package system

import (
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

var (
	netMu    sync.Mutex
	lastRecv uint64
	lastSent uint64
	lastTime time.Time

	cpuMu       sync.Mutex
	cpuLast     []float64
	cpuLastTime time.Time
)

func clampPercent(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return math.Round(v*10) / 10
}

// sampleCPU returns overall CPU busy percent. Caches the previous sample so
// rapid dashboard polls (every few seconds) still produce a stable reading;
// a zero-interval call returns the last known value.
func sampleCPU() float64 {
	cpuMu.Lock()
	defer cpuMu.Unlock()

	// Prefer a short blocking sample — reliable on Linux hosts/VPS.
	percents, err := cpu.Percent(200*time.Millisecond, false)
	if err == nil && len(percents) > 0 {
		v := clampPercent(percents[0])
		cpuLast = percents
		cpuLastTime = time.Now()
		return v
	}
	// Fallback: non-blocking times-based sample after first call.
	percents, err = cpu.Percent(0, false)
	if err == nil && len(percents) > 0 {
		v := clampPercent(percents[0])
		cpuLast = percents
		cpuLastTime = time.Now()
		return v
	}
	if len(cpuLast) > 0 && time.Since(cpuLastTime) < 30*time.Second {
		return clampPercent(cpuLast[0])
	}
	return 0
}

func sampleDisk() DiskInfo {
	candidates := []string{"/"}
	if home := os.Getenv("HOME"); home != "" {
		candidates = append(candidates, home)
	}
	// Data directory commonly used by the panel installer.
	candidates = append(candidates, "/var/lib/3m-ui", "/usr/local/lib/3m-ui")

	var best *disk.UsageStat
	for _, path := range candidates {
		u, err := disk.Usage(path)
		if err != nil || u == nil || u.Total == 0 {
			continue
		}
		// Prefer the root filesystem; otherwise keep the largest volume seen.
		if path == "/" {
			best = u
			break
		}
		if best == nil || u.Total > best.Total {
			best = u
		}
	}
	if best == nil {
		return DiskInfo{}
	}
	return DiskInfo{
		Used:    float64(best.Used),
		Total:   float64(best.Total),
		Percent: clampPercent(best.UsedPercent),
	}
}

// cgroup memory accounting paths (v2 first, v1 fallback).
const (
	cgroupV2Usage = "/sys/fs/cgroup/memory.current"
	cgroupV2Max   = "/sys/fs/cgroup/memory.max"
	cgroupV1Usage = "/sys/fs/cgroup/memory/memory.usage_in_bytes"
	cgroupV1Max   = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
)

// readUintFile reads a single unsigned integer from path. Returns false when
// the file is missing, unreadable, holds "max" (cgroup v2 unlimited), or does
// not parse as a number.
func readUintFile(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(data))
	if s == "" || strings.EqualFold(s, "max") {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// readCgroupMemory returns (used, total) from the process's own cgroup. Both
// are 0 when cgroup accounting is unavailable (bare metal, or cgroups not
// mounted), in which case the caller falls back to /proc/meminfo.
//
// v1 reports an absurdly large limit when unset (PAGE_COUNTER_MAX), so a limit
// larger than the host's total RAM is treated as "no limit".
func readCgroupMemory() (used, total float64) {
	if u, ok := readUintFile(cgroupV2Usage); ok {
		used = float64(u)
	}
	if m, ok := readUintFile(cgroupV2Max); ok && m > 0 {
		total = float64(m)
	}
	if used == 0 || total == 0 {
		if u, ok := readUintFile(cgroupV1Usage); ok && used == 0 {
			used = float64(u)
		}
		if m, ok := readUintFile(cgroupV1Max); ok && m > 0 && total == 0 {
			total = float64(m)
		}
	}
	return used, total
}

// sampleMemory returns host memory usage.
//
// Inside a container or under a systemd MemoryMax, /proc/meminfo (which
// mem.VirtualMemory reads) describes the *host*, not the cgroup the panel
// actually lives in. That mismatch is what made the dashboard read a smaller
// "system memory used" than the sum of the per-process RSS shown right next to
// it. Preferring cgroup accounting keeps both cards on the same basis:
// cgroup usage already contains every process' RSS plus page cache, so the sum
// of the processes can never exceed it.
//
// Falls back to /proc/meminfo on hosts without cgroup accounting, where
// UsedPercent (rather than a raw used/total division) matches what operators
// see in free(1) / top(1).
func sampleMemory() MemoryInfo {
	cgroupUsed, cgroupTotal := readCgroupMemory()

	vMem, err := mem.VirtualMemory()
	if err != nil || vMem == nil {
		// No /proc/meminfo at all — cgroup is the only source available.
		if cgroupTotal > 0 && cgroupUsed > 0 {
			return MemoryInfo{
				Used:    cgroupUsed,
				Total:   cgroupTotal,
				Percent: clampPercent(cgroupUsed / cgroupTotal * 100),
			}
		}
		return MemoryInfo{}
	}

	hostTotal := float64(vMem.Total)
	// cgroup v1 reports PAGE_COUNTER_MAX (~ 2^63/2^64) or a value larger than
	// physical RAM when no limit is set; neither is a real cap.
	if cgroupTotal > 0 && hostTotal > 0 && cgroupTotal > hostTotal {
		cgroupTotal = 0
	}

	if cgroupUsed > 0 && cgroupTotal > 0 {
		return MemoryInfo{
			Used:    cgroupUsed,
			Total:   cgroupTotal,
			Percent: clampPercent(cgroupUsed / cgroupTotal * 100),
		}
	}

	// No usable cgroup accounting: report host figures.
	used := float64(vMem.Used)
	total := hostTotal
	percent := vMem.UsedPercent
	// Only substitute used/total when gopsutil handed back a nonsensical value;
	// UsedPercent normally already discounts buffers/cache the way operators
	// expect, so it must not be overwritten unconditionally.
	if total > 0 && (percent <= 0 || percent > 100) {
		percent = used / total * 100
	}
	return MemoryInfo{
		Used:    used,
		Total:   total,
		Percent: clampPercent(percent),
	}
}

// statsTTL bounds how often a fresh sample is taken. Measuring CPU blocks for
// 200ms, so with the dashboard polling every second a sample-per-request would
// spend a fifth of a core on measurement alone - and it would multiply with
// every open tab. Requests arriving inside the window share one sample; a
// 500ms TTL still leaves each 1s poll with its own fresh reading.
const statsTTL = 500 * time.Millisecond

var (
	statsMu    sync.Mutex
	statsCache *SystemStats
	statsAt    time.Time
)

// GetSystemStats returns host metrics, reusing a sample taken within statsTTL.
func GetSystemStats() *SystemStats {
	statsMu.Lock()
	if statsCache != nil && time.Since(statsAt) < statsTTL {
		cached := *statsCache
		statsMu.Unlock()
		return &cached
	}
	statsMu.Unlock()

	stats := sampleSystemStats()

	statsMu.Lock()
	statsCache = stats
	statsAt = time.Now()
	statsMu.Unlock()
	return stats
}

// sampleSystemStats returns live host metrics. Memory/disk used+total are in
// **bytes** so the frontend can format them uniformly with formatBytes.
func sampleSystemStats() *SystemStats {
	cpuPercent := sampleCPU()
	memoryInfo := sampleMemory()
	diskInfo := sampleDisk()

	var networkInfo NetworkInfo
	if netIO, err := net.IOCounters(false); err == nil && len(netIO) > 0 {
		netMu.Lock()
		now := time.Now()
		currRecv := netIO[0].BytesRecv
		currSent := netIO[0].BytesSent
		if !lastTime.IsZero() {
			duration := now.Sub(lastTime).Seconds()
			if duration > 0 {
				// Guard against counter reset (e.g. interface re-create).
				if currRecv >= lastRecv {
					networkInfo.Download = float64(currRecv-lastRecv) / duration
				}
				if currSent >= lastSent {
					networkInfo.Upload = float64(currSent-lastSent) / duration
				}
			}
		}
		lastRecv = currRecv
		lastSent = currSent
		lastTime = now
		netMu.Unlock()
	}

	return &SystemStats{
		CPU:     CPUInfo{Percent: cpuPercent},
		Memory:  memoryInfo,
		Disk:    diskInfo,
		Network: networkInfo,
	}
}
