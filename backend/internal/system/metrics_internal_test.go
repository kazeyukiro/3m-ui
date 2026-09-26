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
