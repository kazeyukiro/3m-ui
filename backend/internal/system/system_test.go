package system_test

import (
	"os"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/system"
)

func TestGetSystemStats(t *testing.T) {
	stats := system.GetSystemStats()

	if stats == nil {
		t.Fatal("expected non-nil stats response")
	}

	if stats.CPU.Percent < 0 {
		t.Fatalf("unexpected CPU value: %f", stats.CPU.Percent)
	}

	if stats.Memory.Percent < 0 || stats.Memory.Percent > 100 {
		t.Fatalf("unexpected Memory percent: %f", stats.Memory.Percent)
	}

	if stats.Disk.Percent < 0 || stats.Disk.Percent > 100 {
		t.Fatalf("unexpected Disk percent: %f", stats.Disk.Percent)
	}

	if stats.Network.Upload < 0 || stats.Network.Download < 0 {
		t.Fatal("unexpected Network rate metrics")
	}
}

// memorySkewTolerance absorbs the skew between two separately taken memory
// samples. Where the panel is the bulk of its own accounting domain — a
// container, or a sandbox running the test binary alone — system-used and this
// process' RSS are nearly the same number, so the comparison has no margin of
// its own. The failure these tests guard was a mismatch of *sources*: out by
// multiples (host /proc/meminfo against the cgroup), never by a percent.
func memorySkewTolerance(total float64) float64 {
	tol := total * 0.02
	if floor := float64(8 << 20); tol < floor {
		return floor
	}
	return tol
}

// Guards the invariant the dashboard relies on: the "system resources" card and
// the "process usage" card must describe the same memory pool. Reading host
// /proc/meminfo while the processes are charged to a cgroup made system-used
// smaller than the sum of the RSS values listed beside it.
func TestSystemMemoryMatchesProcessMemory(t *testing.T) {
	// Sample the process first so the system reading — the reference — is the
	// newer of the two. Otherwise the process figure can reflect allocations
	// made after the system figure was taken.
	self := system.SampleProcessUsage(os.Getpid())
	stats := system.GetSystemStats()
	if stats == nil || stats.Memory.Total <= 0 || stats.Memory.Used <= 0 {
		t.Skip("memory stats unavailable on this host")
	}
	if self.MemoryUsed <= 0 {
		t.Skip("could not sample own RSS")
	}

	if self.MemoryUsed > stats.Memory.Used+memorySkewTolerance(stats.Memory.Total) {
		t.Fatalf("system memory used (%d bytes) is smaller than this process' RSS (%d bytes); "+
			"the two cards are reading different memory sources",
			int64(stats.Memory.Used), int64(self.MemoryUsed))
	}

	want := self.MemoryUsed / stats.Memory.Total * 100
	if diff := want - self.MemoryPercent; diff > 0.2 || diff < -0.2 {
		t.Fatalf("process memory percent %.2f does not use the system card's denominator (want %.2f)",
			self.MemoryPercent, want)
	}
}
