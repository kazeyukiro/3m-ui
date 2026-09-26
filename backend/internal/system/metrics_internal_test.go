package system

import (
	"testing"
	"time"
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
		t.Fatalf("total = %d, want the cgroup limit %d", int64(got.Total), limit)
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
		t.Fatalf("unlimited cgroup: got (total=%d, ok=%v), want total=%d", int64(got.Total), ok, hostTotal)
	}

	// cgroup v1's PAGE_COUNTER_MAX sentinel is not a real cap either.
	const pageCounterMax = 1 << 62
	if got, ok := memoryFromCgroup(cgroupMemory{usage: usage, limit: pageCounterMax, ok: true}, hostTotal); !ok || int64(got.Total) != hostTotal {
		t.Fatalf("PAGE_COUNTER_MAX: got (total=%d, ok=%v), want total=%d", int64(got.Total), ok, hostTotal)
	}

	// A cap smaller than RAM is honoured.
	const smallLimit = 4 << 30
	if got, ok := memoryFromCgroup(cgroupMemory{usage: usage, limit: smallLimit, ok: true}, hostTotal); !ok || int64(got.Total) != smallLimit {
		t.Fatalf("real cap: got (total=%d, ok=%v), want total=%d", int64(got.Total), ok, smallLimit)
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
