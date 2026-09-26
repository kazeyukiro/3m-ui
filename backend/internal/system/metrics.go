package system

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
)

var (
	netMu    sync.Mutex
	lastRecv uint64
	lastSent uint64
	lastTime time.Time
	// Last rates reported, reused when a sample lands too close to the
	// previous one for the delta to be meaningful.
	lastDown float64
	lastUp   float64
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

// cpuBaselineWindow is how long the very first CPU measurement waits for a
// second reading. Without a baseline there is nothing to difference against,
// so the alternative is reporting 0% until the next poll.
const cpuBaselineWindow = 120 * time.Millisecond

// cpuSnapshot is one reading of whichever CPU counter we can trust, plus the
// instant it was taken. Both cards (host and process) are derived by
// differencing two snapshots, which is what turns a monotonic counter into a
// rate.
type cpuSnapshot struct {
	at time.Time
	// cgroup accounting: cumulative CPU microseconds burned by the whole
	// cgroup, summed across cores.
	cgBusyMicros uint64
	cg           bool
	// Host accounting: cumulative busy/total seconds summed across all cores.
	hostBusy float64
	hostAll  float64
}

// takeCPUSnapshot prefers cgroup CPU accounting over the host /proc/stat.
//
// The difference matters inside a container: /proc/stat describes the whole
// machine, so a container pinned to 1 of 8 cores reads at most 12.5% no matter
// how hard it works — the panel looked idle while it was actually throttled.
// The cgroup counter only ticks for this container, which is also why quota
// (rather than host core count) is the right denominator for it.
func takeCPUSnapshot() (cpuSnapshot, bool) {
	var s cpuSnapshot
	if micros, ok := cgroupCPUUsage(); ok {
		s.cg = true
		s.cgBusyMicros = micros
		s.at = time.Now()
		return s, true
	}
	// Fallback: aggregate /proc/stat line. Busy is "everything except idle and
	// iowait", matching gopsutil's own definition so the number agrees with
	// what top(1) shows.
	times, err := cpu.Times(false)
	if err != nil || len(times) == 0 {
		return cpuSnapshot{}, false
	}
	t := times[0]
	all := t.Total() - t.Guest - t.GuestNice
	s.hostAll = all
	s.hostBusy = all - t.Idle - t.Iowait
	s.at = time.Now()
	return s, true
}

// onlineCPUs returns the number of logical CPUs /proc reports.
func onlineCPUs() (int, error) {
	n, err := cpu.Counts(true)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// cpuCapacityTTLCached reuses the last capacity for this long. The limits only
// change when someone re-configures the container, and this value is read on
// every poll by both cards, so re-reading three files each time buys nothing.
const cpuCapacityTTL = 10 * time.Second

var (
	capacityMu    sync.Mutex
	capacityCache float64
	capacityAt    time.Time
)

// cpuCapacity returns how many CPUs the current accounting domain has. Every
// limit that applies narrows it: a CFS quota, a cpuset pin, and finally the
// number of CPUs actually online. It is the denominator both the system card
// and the process card divide by, so they can never disagree.
func cpuCapacity() float64 {
	capacityMu.Lock()
	defer capacityMu.Unlock()
	if capacityCache > 0 && time.Since(capacityAt) < cpuCapacityTTL {
		return capacityCache
	}
	capacityCache = computeCPUCapacity()
	capacityAt = time.Now()
	return capacityCache
}

func computeCPUCapacity() float64 {
	caps := []float64{}
	if q, ok := cgroupCPUQuota(); ok && q > 0 {
		caps = append(caps, q)
	}
	if n, ok := cgroupCPUSet(); ok && n > 0 {
		caps = append(caps, n)
	}
	if n, err := onlineCPUs(); err == nil && n > 0 {
		caps = append(caps, float64(n))
	}
	if len(caps) == 0 {
		if n := runtime.NumCPU(); n > 0 {
			return float64(n)
		}
		return 0
	}
	min := caps[0]
	for _, c := range caps[1:] {
		if c < min {
			min = c
		}
	}
	return min
}

// cpuPercent turns two snapshots into a busy percentage of the available CPU
// capacity. ok is false when the pair cannot be differenced (source changed,
// clock went backwards, zero elapsed time), in which case the caller keeps the
// previous reading rather than reporting a bogus one.
func cpuPercent(prev, cur cpuSnapshot, capacity float64) (float64, bool) {
	if prev.cg != cur.cg {
		return 0, false
	}
	elapsed := cur.at.Sub(prev.at).Seconds()
	if elapsed <= 0 || capacity <= 0 {
		return 0, false
	}
	if cur.cg {
		if cur.cgBusyMicros < prev.cgBusyMicros {
			// Counter reset / cgroup recreated: nothing to difference.
			return 0, false
		}
		// Zero consumption is a real reading (nothing ran), not a failure —
		// returning false here would pin the card at its last value forever on
		// an idle host.
		busySec := float64(cur.cgBusyMicros-prev.cgBusyMicros) / 1e6
		return math.Min(100, math.Max(0, busySec/elapsed/capacity*100)), true
	}
	dAll := cur.hostAll - prev.hostAll
	if dAll < 0 {
		return 0, false
	}
	if dAll == 0 {
		// Nothing ticked: genuinely idle rather than unmeasurable.
		return 0, true
	}
	busy := cur.hostBusy - prev.hostBusy
	if busy < 0 {
		busy = 0
	}
	return math.Min(100, math.Max(0, busy/dAll*100)), true
}

var (
	cpuMu      sync.Mutex
	cpuPrev    *cpuSnapshot
	cpuLastPct float64
)

// sampleCPU returns CPU busy percent over the interval since the previous
// sample.
//
// Deliberately difference-based rather than self-blocking: the old
// cpu.Percent(200ms) insertion, when the dashboard polls every second, only
// described 200ms of each second and cost that wait on the request thread. Two
// non-blocking readings describe the whole interval since the last poll, which
// is what a refreshing dashboard actually claims to show.
func sampleCPU() float64 {
	capacity := cpuCapacity()
	if capacity <= 0 {
		return 0
	}

	cpuMu.Lock()
	defer cpuMu.Unlock()

	if cpuPrev == nil {
		first, ok := takeCPUSnapshot()
		if !ok {
			return cpuLastPct
		}
		// One short wait buys a real delta, so the very first render shows a
		// measured value instead of a misleading 0%.
		time.Sleep(cpuBaselineWindow)
		second, ok := takeCPUSnapshot()
		if !ok {
			return cpuLastPct
		}
		cpuPrev = &second
		if pct, ok := cpuPercent(first, second, capacity); ok {
			cpuLastPct = clampPercent(pct)
		}
		return cpuLastPct
	}

	cur, ok := takeCPUSnapshot()
	if !ok {
		return cpuLastPct
	}
	prev := *cpuPrev
	cpuPrev = &cur
	if pct, ok := cpuPercent(prev, cur, capacity); ok {
		cpuLastPct = clampPercent(pct)
	}
	return cpuLastPct
}

// diskAnchor returns the directory whose filesystem the disk card should
// describe: wherever the panel's SQLite database lives. That is the volume
// growth actually threatens — in Docker it is commonly a mounted data volume
// while "/" is the container's overlay layer, so reporting "/" hides the
// pressure operators care about.
//
// disk.Usage resolves any path to its containing filesystem via statfs, so no
// manual mountpoint lookup is needed.
func diskAnchor() string {
	if cfg := config.GlobalConfig; cfg != nil && cfg.Database.Path != "" {
		return filepath.Dir(cfg.Database.Path)
	}
	return ""
}

func sampleDisk() DiskInfo {
	var candidates []string
	if home := os.Getenv("HOME"); home != "" {
		candidates = append(candidates, home)
	}
	// Data directory commonly used by the panel installer.
	candidates = append(candidates, "/var/lib/3m-ui", "/usr/local/lib/3m-ui")

	var fallback *disk.UsageStat
	for _, path := range candidates {
		u, err := disk.Usage(path)
		if err != nil || u == nil || u.Total == 0 {
			continue
		}
		if fallback == nil || u.Total > fallback.Total {
			fallback = u
		}
	}

	// The anchor wins when it resolves; "/" is only the last resort, because
	// it describes the wrong device whenever the data directory is mounted.
	if anchor := diskAnchor(); anchor != "" {
		if u, err := disk.Usage(anchor); err == nil && u != nil && u.Total > 0 {
			return diskInfo(u)
		}
	}

	if fallback != nil {
		return diskInfo(fallback)
	}
	u, err := disk.Usage("/")
	if err != nil || u == nil || u.Total == 0 {
		return DiskInfo{}
	}
	return diskInfo(u)
}

func diskInfo(u *disk.UsageStat) DiskInfo {
	return DiskInfo{
		Used:    float64(u.Used),
		Total:   float64(u.Total),
		Percent: clampPercent(u.UsedPercent),
	}
}

// memoryFromCgroup turns raw cgroup counters into the MemoryInfo the card
// shows. ok is false when nothing usable came back.
//
// Two things make this more than an division:
//
//  1. The limit is only meaningful when it is real. cgroup v1 reports
//     PAGE_COUNTER_MAX (~2^63) or a value above physical RAM when nothing was
//     configured, and a fully unlimited container has no cap at all. In both
//     cases the machine is what this process can actually consume, so it
//     becomes the denominator — otherwise the card silently switches to
//     host-wide figures and starts tracking other tenants' activity.
//
//  2. "Used" discounts reclaimable page cache (the kubelet working-set
//     convention, what container runtimes report as a percentage). Raw usage
//     is a *gross* figure: it bills this cgroup for every file page it has
//     touched, including ones the kernel would hand back instantly under
//     pressure. Reading mihomo's geoip database or downloading a rule provider
//     once is enough to pin the bar against the limit while gigabytes remain
//     available. Measured on this host: 51.7% gross vs 16.2% working set.
//     Only *inactive* cache is discounted; pages actively mapped and used stay
//     charged, which keeps this figure able to hold every process' RSS.
//
// This also keeps "used" meaning the same thing on both branches — the
// /proc/meminfo fallback already excludes buffers and cache.
func memoryFromCgroup(cg cgroupMemory, hostTotal float64) (MemoryInfo, bool) {
	if !cg.ok {
		return MemoryInfo{}, false
	}
	total := cg.limit
	if hostTotal > 0 && total > hostTotal {
		total = 0
	}
	if total <= 0 {
		total = hostTotal
	}
	if total <= 0 {
		return MemoryInfo{}, false
	}

	used := cg.usage - cg.inactiveFile
	if used < 0 {
		used = 0
	}
	return MemoryInfo{
		Used:    used,
		Total:   total,
		Percent: clampPercent(used / total * 100),
	}, true
}

// sampleMemory returns host memory usage.
//
// Inside a container or under a systemd MemoryMax, /proc/meminfo (which
// mem.VirtualMemory reads) describes the *host*, not the cgroup the panel
// actually lives in. That mismatch is what made the dashboard read a smaller
// "system memory used" than the sum of the per-process RSS shown right next to
// it. Preferring cgroup accounting keeps both cards on the same basis, and the
// per-process percentages divide by this same total.
//
// The CPU card uses cgroup accounting for the same reason, so the two resource
// bars answer the same question: how loaded is the thing we are running in.
func sampleMemory() MemoryInfo {
	cg := readCgroupMemory()

	vMem, err := mem.VirtualMemory()
	if err != nil || vMem == nil {
		// No /proc/meminfo at all — cgroup is the only source available.
		if info, ok := memoryFromCgroup(cg, 0); ok {
			return info
		}
		return MemoryInfo{}
	}
	hostTotal := float64(vMem.Total)

	if info, ok := memoryFromCgroup(cg, hostTotal); ok {
		return info
	}

	// No cgroup accounting (bare metal): report host figures. gopsutil already
	// discounts buffers and cache here, and UsedPercent is that same ratio, so
	// it is used as-is rather than recomputed from Used/Total.
	used := float64(vMem.Used)
	total := hostTotal
	percent := vMem.UsedPercent
	// Only substitute used/total when gopsutil handed back a nonsensical value.
	if total > 0 && (percent <= 0 || percent > 100) {
		percent = used / total * 100
	}
	return MemoryInfo{
		Used:    used,
		Total:   total,
		Percent: clampPercent(percent),
	}
}

// statsTTL bounds how often a fresh sample is taken. It also sets a floor on
// the interval a reading describes: network throughput is a rate, and a rate
// differenced over a few milliseconds is dominated by rounding noise. Requests
// arriving inside the window share one sample; a 500ms TTL still leaves each
// 1s poll with its own fresh reading.
const statsTTL = 500 * time.Millisecond

// minNetSampleInterval is the shortest delta the network rate is computed
// over. Anything tighter produces rates wildly larger than reality.
const minNetSampleInterval = 250 * time.Millisecond

var (
	statsMu    sync.Mutex
	statsCache *SystemStats
	statsAt    time.Time
	// Sampling serialises so two concurrent request handlers cannot both
	// difference against the same network counters — the second would measure
	// ~0 seconds of traffic and report a huge spike.
	sampleMu sync.Mutex
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

	sampleMu.Lock()
	// Double-check inside the barrier: another goroutine may have sampled
	// while we were waiting for it.
	statsMu.Lock()
	if statsCache != nil && time.Since(statsAt) < statsTTL {
		cached := *statsCache
		statsMu.Unlock()
		sampleMu.Unlock()
		return &cached
	}
	statsMu.Unlock()

	stats := sampleSystemStats()

	statsMu.Lock()
	statsCache = stats
	statsAt = time.Now()
	statsMu.Unlock()
	sampleMu.Unlock()
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
			duration := now.Sub(lastTime)
			if duration >= minNetSampleInterval {
				secs := duration.Seconds()
				// Guard against counter reset (e.g. interface re-create).
				if currRecv >= lastRecv {
					lastDown = float64(currRecv-lastRecv) / secs
				}
				if currSent >= lastSent {
					lastUp = float64(currSent-lastSent) / secs
				}
			}
		}
		lastRecv = currRecv
		lastSent = currSent
		lastTime = now
		networkInfo.Download = lastDown
		networkInfo.Upload = lastUp
		netMu.Unlock()
	}

	return &SystemStats{
		CPU:     CPUInfo{Percent: cpuPercent},
		Memory:  memoryInfo,
		Disk:    diskInfo,
		Network: networkInfo,
	}
}
