package system

import (
	"math"
	"os"
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

// GetSystemStats returns live host metrics. Memory/disk used+total are in
// **bytes** so the frontend can format them uniformly with formatBytes.
func GetSystemStats() *SystemStats {
	cpuPercent := sampleCPU()

	var memoryInfo MemoryInfo
	if vMem, err := mem.VirtualMemory(); err == nil && vMem != nil {
		// Prefer explicit used/total; UsedPercent on Linux already accounts for
		// buffers/cache the way operators usually expect.
		percent := vMem.UsedPercent
		if vMem.Total > 0 {
			percent = float64(vMem.Used) / float64(vMem.Total) * 100
		}
		memoryInfo = MemoryInfo{
			Used:    float64(vMem.Used),
			Total:   float64(vMem.Total),
			Percent: clampPercent(percent),
		}
	}

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
