package system

import (
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

func TestParseCPUSetCount(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"0-3", 4, true},
		{"0", 1, true},
		{"7", 1, true},
		{"0,2-3", 3, true},
		{"0-1,4,6-7", 5, true},
		{" 0-3 \n", 4, true},
		{"", 0, false},
		{"  ", 0, false},
		{"3-1", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, ok := parseCPUSetCount(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseCPUSetCount(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestCPUPercentCgroup(t *testing.T) {
	t0 := time.Now()
	prev := cpuSnapshot{at: t0, cg: true, cgBusyMicros: 1_000_000}
	// One second of CPU time burned over one second of wall clock, with four
	// CPUs available → 25% of capacity.
	cur := cpuSnapshot{at: t0.Add(time.Second), cg: true, cgBusyMicros: 2_000_000}
	got, ok := cpuPercent(prev, cur, 4)
	if !ok {
		t.Fatal("expected a valid delta")
	}
	if diff := got - 25; diff > 0.01 || diff < -0.01 {
		t.Fatalf("cpuPercent = %.2f, want 25 (percent of capacity)", got)
	}

	// Same burn measured against a single CPU → fully saturated.
	if got, _ := cpuPercent(prev, cur, 1); got != 100 {
		t.Fatalf("cpuPercent with capacity 1 = %.2f, want 100", got)
	}

	// Counter that went backwards (cgroup recreated, PID reuse): unusable.
	bad := cpuSnapshot{at: t0.Add(2 * time.Second), cg: true, cgBusyMicros: 500_000}
	if _, ok := cpuPercent(cur, bad, 4); ok {
		t.Fatal("expected no usable delta when the counter goes backwards")
	}

	// Zero elapsed time cannot yield a rate either.
	zero := cpuSnapshot{at: t0, cg: true, cgBusyMicros: 2_000_000}
	if _, ok := cpuPercent(prev, zero, 4); ok {
		t.Fatal("expected no usable delta over zero elapsed time")
	}

	// Mixing sources (fallback kicked in) must not be differenced.
	hostish := cpuSnapshot{at: t0.Add(time.Second), hostBusy: 5, hostAll: 10}
	if _, ok := cpuPercent(prev, hostish, 4); ok {
		t.Fatal("expected no usable delta when the counter source changes")
	}
}

func TestCPUPercentHost(t *testing.T) {
	t0 := time.Now()
	prev := cpuSnapshot{at: t0, hostBusy: 10, hostAll: 50}
	// Over the next second all four cores advanced 4s total, 1s of it busy.
	cur := cpuSnapshot{at: t0.Add(time.Second), hostBusy: 11, hostAll: 54}
	got, ok := cpuPercent(prev, cur, 4)
	if !ok {
		t.Fatal("expected a valid delta")
	}
	if diff := got - 25; diff > 0.01 || diff < -0.01 {
		t.Fatalf("host cpuPercent = %.2f, want 25", got)
	}
}

// TestCPUCapacityBounded guards the denominator shared by both cards: it has to
// be positive and it must never exceed the CPUs actually declared online,
// otherwise percentages read smaller than the work being done.
func TestCPUCapacityBounded(t *testing.T) {
	capacity := cpuCapacity()
	if capacity <= 0 {
		t.Fatalf("cpuCapacity() = %v, want > 0", capacity)
	}
	if n, err := onlineCPUs(); err == nil && capacity > float64(n) {
		t.Fatalf("cpuCapacity() = %.2f exceeds online CPUs %d", capacity, n)
	}
}

// TestMemoryFromCgroupDiscountsReclaimableCache pins the arithmetic behind the
// memory card. The cgroup's raw usage bills this container for page cache the
// kernel can reclaim on demand, so charging it to "used" is what made a nearly
// idle container look like it was about to hit its limit.
//
// Numbers below mirror a real host: 4.4GB charged, 3.0GB of it inactive page
// cache, against an 8GB cap → 51.7% gross but 16.2% actual.
func TestMemoryFromCgroupDiscountsReclaimableCache(t *testing.T) {
	// Untyped on purpose: cgroupMemory fields are float64. The int64() casts
	// in the Fatalf calls below matter on 32-bit release arches (linux/386),
	// where a bare 8GiB constant no longer fits a default int.
	const (
		usage        = 4_439_781_376
		inactiveFile = 3_045_183_488
		limit        = 8_589_934_592
		hostTotal    = 132_411_887_616
	)

	got, ok := memoryFromCgroup(cgroupMemory{usage: usage, inactiveFile: inactiveFile, limit: limit, ok: true}, hostTotal)
	if !ok {
		t.Fatal("expected usable cgroup accounting")
	}
	if int64(got.Total) != limit {
		t.Fatalf("total = %d, want the cgroup limit %d", int64(got.Total), int64(limit))
	}
	wantUsed := usage - inactiveFile
	if int64(got.Used) != int64(wantUsed) {
		t.Fatalf("used = %d, want %d (usage minus reclaimable cache)", int64(got.Used), int64(wantUsed))
	}
	grossPercent := float64(usage) / float64(limit) * 100
	if diff := got.Percent - 16.2; diff > 0.1 || diff < -0.1 {
		t.Fatalf("percent = %.2f, want ~16.2 (not the gross %.2f)", got.Percent, grossPercent)
	}

	// With nothing reclaimable recorded, all usage is pressure.
	gross, ok := memoryFromCgroup(cgroupMemory{usage: usage, limit: limit, ok: true}, hostTotal)
	if !ok {
		t.Fatal("expected usable cgroup accounting")
	}
	if diff := gross.Percent - 51.7; diff > 0.1 || diff < -0.1 {
		t.Fatalf("no-cache case percent = %.2f, want ~51.7", gross.Percent)
	}
}

// TestMemoryFromCgroupLimitResolution covers the three ways the denominator can
// end up wrong.
func TestMemoryFromCgroupLimitResolution(t *testing.T) {
	const hostTotal = 8 << 30
	const usage = 2 << 30

	// No cap configured: the machine is the cap. Reporting a host-wide figure
	// here would make the bar track other tenants' activity.
	got, ok := memoryFromCgroup(cgroupMemory{usage: usage, ok: true}, hostTotal)
	if !ok || int64(got.Total) != hostTotal {
		t.Fatalf("unlimited cgroup: got (total=%d, ok=%v), want total=%d", int64(got.Total), ok, int64(hostTotal))
	}

	// cgroup v1's PAGE_COUNTER_MAX sentinel is not a real cap either.
	const pageCounterMax = 1 << 62
	if got, ok := memoryFromCgroup(cgroupMemory{usage: usage, limit: pageCounterMax, ok: true}, hostTotal); !ok || int64(got.Total) != hostTotal {
		t.Fatalf("PAGE_COUNTER_MAX: got (total=%d, ok=%v), want total=%d", int64(got.Total), ok, int64(hostTotal))
	}

	// A cap smaller than RAM is honoured.
	const smallLimit = 4 << 30
	if got, ok := memoryFromCgroup(cgroupMemory{usage: usage, limit: smallLimit, ok: true}, hostTotal); !ok || int64(got.Total) != smallLimit {
		t.Fatalf("real cap: got (total=%d, ok=%v), want total=%d", int64(got.Total), ok, int64(smallLimit))
	}

	// No accounting at all falls through to the caller's /proc/meminfo path.
	if _, ok := memoryFromCgroup(cgroupMemory{}, hostTotal); ok {
		t.Fatal("expected no usable result when cgroup accounting is absent")
	}
}

// TestMemoryFromCgroupClamps guards against a nonsense negative "used", which
// would render as a larger-than-total bar if the cgroup's counters ever race.
func TestMemoryFromCgroupClamps(t *testing.T) {
	got, ok := memoryFromCgroup(cgroupMemory{usage: 1 << 20, inactiveFile: 8 << 20, limit: 4 << 30, ok: true}, 8<<30)
	if !ok {
		t.Fatal("expected usable cgroup accounting")
	}
	if got.Used != 0 || got.Percent != 0 {
		t.Fatalf("used=%f percent=%f, want both 0 when cache exceeds usage", got.Used, got.Percent)
	}
}

func TestSampleDiskReportsFiniteUsage(t *testing.T) {
	d := sampleDisk()
	if d.Total <= 0 {
		t.Skip("disk usage unavailable on this host")
	}
	if d.Percent < 0 || d.Percent > 100 {
		t.Fatalf("disk percent out of range: %f", d.Percent)
	}
	if d.Used > d.Total {
		t.Fatalf("disk used %d exceeds total %d", int64(d.Used), int64(d.Total))
	}
}

// The memory card reads /proc/meminfo through mem.VirtualMemory and reports
// Used/Total as-is (s-ui's GetMemInfo does the same). These cases pin the
// conversion: gopsutil's UsedPercent is trusted, a nonsensical one is
// recomputed from Used/Total, and an empty stat yields the zero value.
func TestMemoryFromHost(t *testing.T) {
	cases := []struct {
		name      string
		used      uint64
		total     uint64
		percentIn float64
		wantPct   float64
	}{
		{"gopsutil percent is used as-is", 33 << 20, 128 << 20, 25.8, 25.8},
		{"zero percent is recomputed", 64 << 20, 128 << 20, 0, 50},
		{"percent above 100 is recomputed", 64 << 20, 128 << 20, 180, 50},
		{"negative percent is recomputed", 8 << 20, 128 << 20, -5, 6.3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := memoryFromHost(&mem.VirtualMemoryStat{
				Used:        tc.used,
				Total:       tc.total,
				UsedPercent: tc.percentIn,
			})
			if info.Used != float64(tc.used) || info.Total != float64(tc.total) {
				t.Fatalf("used=%f total=%f, want %d/%d", info.Used, info.Total, tc.used, tc.total)
			}
			if info.Percent != tc.wantPct {
				t.Fatalf("percent=%f, want %f", info.Percent, tc.wantPct)
			}
		})
	}
}

func TestMemoryFromHostRejectsEmptyStats(t *testing.T) {
	if got := memoryFromHost(nil); got != (MemoryInfo{}) {
		t.Fatalf("nil stat -> %+v, want zero MemoryInfo", got)
	}
	if got := memoryFromHost(&mem.VirtualMemoryStat{Used: 1 << 20, Total: 0}); got != (MemoryInfo{}) {
		t.Fatalf("zero total -> %+v, want zero MemoryInfo", got)
	}
}

// sampleMemory must report the host figures whenever /proc/meminfo is
// readable and this process is not inside a container, even when the
// process' cgroup carries a real memory limit (a systemd MemoryMax): the cap
// is not the machine. Inside a container the container-root cap legitimately
// wins — see TestSampleMemoryUsesContainerRootNotPanelUnit.
func TestSampleMemoryPrefersHostOverCgroupLimit(t *testing.T) {
	vMem, err := mem.VirtualMemory()
	if err != nil || vMem == nil || vMem.Total <= 0 {
		t.Skip("/proc/meminfo unavailable on this host")
	}
	info := sampleMemory()
	if info.Total <= 0 || info.Percent < 0 || info.Percent > 100 {
		t.Fatalf("implausible figures: %+v", info)
	}
	if runningInContainer() {
		// The container branch may report the container's cap, but never
		// more than the machine has.
		if info.Total > float64(vMem.Total) {
			t.Fatalf("total=%f exceeds the host total %d", info.Total, vMem.Total)
		}
		return
	}
	if info.Total != float64(vMem.Total) {
		t.Fatalf("total=%f, want the host total %d", info.Total, vMem.Total)
	}
}

// forceContainerDetection makes runningInContainer() report true through the
// one signal that cannot collide with the host's own environment: the Docker
// marker file. Every other signal is blanked or pointed at a missing path,
// so detection has exactly one input.
func forceContainerDetection(t *testing.T) {
	t.Helper()
	isolateContainerEnv(t)
	resetContainerFiles(t)
	markerFile(t, &dockerEnvPath)
}

// The user-visible regression this whole mechanism exists for: the panel
// runs under systemd inside a 128MB container, the installer has written its
// 72MB MemoryMax onto the panel's own unit, and the card must report the
// container's 128MB — never the unit's cap, never the host's RAM.
func TestSampleMemoryUsesContainerRootNotPanelUnit(t *testing.T) {
	vMem, err := mem.VirtualMemory()
	if err != nil || vMem == nil || vMem.Total <= 0 {
		t.Skip("/proc/meminfo unavailable on this host")
	}
	if vMem.Total <= 128<<20 {
		t.Skip("host has 128MB or less; the cap could not be told apart from the host total")
	}
	forceContainerDetection(t)
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/")
	f.selfV2("/system.slice/3m-ui.service")
	f.cgroupV2("system.slice/3m-ui.service", 60*mb, 0, 72*mb) // the installer's cap
	f.cgroupV2("", 100*mb, 10*mb, 128*mb)                     // the container itself

	info := sampleMemory()
	if int64(info.Total) != 128*mb {
		t.Fatalf("total = %d, want the container's 128MB cap (not the unit's 72MB, not the host's %d)",
			int64(info.Total), vMem.Total)
	}
	if int64(info.Used) != 90*mb {
		t.Fatalf("used = %d, want the container's working set 90MB", int64(info.Used))
	}
	if diff := info.Percent - 70.3; diff > 0.1 || diff < -0.1 {
		t.Fatalf("percent = %.2f, want ~70.3", info.Percent)
	}
}

// An uncapped container keeps the host's RAM as Total but the container's
// own working set as Used — the figures `docker stats` reports, and never
// other tenants' activity.
func TestSampleMemoryContainerUncappedUsesHostTotal(t *testing.T) {
	vMem, err := mem.VirtualMemory()
	if err != nil || vMem == nil || vMem.Total <= 0 {
		t.Skip("/proc/meminfo unavailable on this host")
	}
	forceContainerDetection(t)
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/")
	f.selfV2("/system.slice/3m-ui.service")
	f.cgroupV2("system.slice/3m-ui.service", 60*mb, 0, 0)
	f.cgroupV2("", 100*mb, 10*mb, 0) // no cap anywhere

	info := sampleMemory()
	if info.Total != float64(vMem.Total) {
		t.Fatalf("total = %f, want the host total %d", info.Total, vMem.Total)
	}
	if int64(info.Used) != 90*mb {
		t.Fatalf("used = %d, want the container's working set 90MB", int64(info.Used))
	}
}

// A container whose accounting cannot be resolved must fall back to the host
// figures s-ui shows, not to a wrong number.
func TestSampleMemoryContainerFallsBackToHostWhenUnresolvable(t *testing.T) {
	vMem, err := mem.VirtualMemory()
	if err != nil || vMem == nil || vMem.Total <= 0 {
		t.Skip("/proc/meminfo unavailable on this host")
	}
	forceContainerDetection(t)
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/")
	f.selfV2("/somewhere/else")

	info := sampleMemory()
	if info.Total != float64(vMem.Total) {
		t.Fatalf("total = %f, want the host total %d", info.Total, vMem.Total)
	}
}
