package system_test

import (
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/system"
)

// procAvailable reports whether this host exposes /proc, which every Linux
// counter here (and the cgroup fallbacks) ultimately read from.
func procAvailable() bool {
	_, err := os.Stat("/proc/self/stat")
	return err == nil
}

// spinLoad keeps `threads` OS threads busy until stop is closed.
func spinLoad(threads int, stop <-chan struct{}) *sync.WaitGroup {
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					// Explicit no-op keeps the loop from being optimised away
					// while staying well inside one CPU.
					runtime.Gosched()
				}
			}
		}()
	}
	return &wg
}

// TestProcessCPUTracksCurrentLoad is the regression test for the process card
// reporting something other than what the process is doing right now.
//
// The previous implementation leaned on gopsutil's Process.CPUPercent(), which
// returns 100 * lifetimeCPUTime / uptime — an average over the process' whole
// history. Two symptoms followed: a long-lived panel settled on a flat,
// unchanging figure, and hosts where the start time cannot be resolved (the
// container's BootTime disagrees with /proc/stat) got a permanent 0.
//
// This test asserts the numbers actually move with the load: high while busy,
// back down once idle.
func TestProcessCPUTracksCurrentLoad(t *testing.T) {
	if !procAvailable() {
		t.Skip("process CPU sampling requires /proc")
	}

	pid := os.Getpid()
	if n := runtime.NumCPU(); n < 2 {
		t.Skipf("needs at least 2 CPUs, got %d", n)
	}

	threads := runtime.NumCPU()
	if threads > 8 {
		threads = 8
	}

	// Prime the sampler so its first baseline is established while idle.
	idle := sampleStable(t, pid)

	stop := make(chan struct{})
	wg := spinLoad(threads, stop)
	time.Sleep(400 * time.Millisecond) // let the load reach steady state
	busy := sampleStable(t, pid)
	close(stop)
	wg.Wait()

	// Wait out the load, then re-prime so the next window contains no leftover
	// busy time from the spin-down.
	time.Sleep(400 * time.Millisecond)
	after := sampleStable(t, pid)

	t.Logf("idle=%.1f%%  busy=%.1f%%  after=%.1f%%  (threads=%d)", idle, busy, after, threads)

	if busy <= idle+5 {
		t.Fatalf("process CPU did not react to load: idle=%.1f%% while busy=%.1f%%", idle, busy)
	}
	if after >= busy/2 {
		t.Fatalf("process CPU did not fall back after load stopped: busy=%.1f%% after=%.1f%%", busy, after)
	}
}

// sampleStable takes two readings and returns the second one, whose window lies
// entirely inside the current load phase. The first reading bridges whatever
// happened since the previous sample.
func sampleStable(t *testing.T, pid int) float64 {
	t.Helper()
	first := system.SampleProcessUsage(pid)
	if first.PID != pid {
		t.Fatalf("SampleProcessUsage returned pid %d, want %d", first.PID, pid)
	}
	time.Sleep(600 * time.Millisecond)
	return system.SampleProcessUsage(pid).CPUPercent
}

// TestProcessUsageSampleShape covers the fields the dashboard renders for both
// cards, including the "no PID" degenerate call the handler makes when the core
// is stopped.
func TestProcessUsageSampleShape(t *testing.T) {
	if !procAvailable() {
		t.Skip("process sampling requires /proc")
	}
	u := system.SampleProcessUsage(0)
	if u.PID != 0 || u.CPUPercent != 0 || u.MemoryUsed != 0 || u.MemoryPercent != 0 {
		t.Fatalf("zero PID should produce an empty usage, got %+v", u)
	}

	self := system.SampleProcessUsage(os.Getpid())
	if self.PID != os.Getpid() {
		t.Fatalf("pid = %d, want %d", self.PID, os.Getpid())
	}
	if self.MemoryUsed <= 0 {
		t.Fatal("own RSS should be measurable")
	}
	if self.MemoryPercent < 0 || self.MemoryPercent > 100 {
		t.Fatalf("memory percent out of range: %f", self.MemoryPercent)
	}
	if self.CPUPercent < 0 || self.CPUPercent > 100 {
		t.Fatalf("cpu percent out of range: %f", self.CPUPercent)
	}
}

// TestSystemCPUSampleReacts asserts the system card reads sensible CPU figures
// and stays within 0–100 while the machine is loaded.
func TestSystemCPUSampleReacts(t *testing.T) {
	if !procAvailable() {
		t.Skip("CPU sampling requires /proc")
	}
	stats := system.GetSystemStats()
	if stats == nil {
		t.Fatal("nil stats")
	}
	if stats.CPU.Percent < 0 || stats.CPU.Percent > 100 {
		t.Fatalf("cpu percent out of range: %f", stats.CPU.Percent)
	}

	stop := make(chan struct{})
	wg := spinLoad(runtime.NumCPU(), stop)
	defer func() { close(stop); wg.Wait() }()

	time.Sleep(500 * time.Millisecond)
	loaded := system.GetSystemStats()
	if loaded.CPU.Percent < 1 {
		t.Fatalf("system CPU did not register load: %.1f%%", loaded.CPU.Percent)
	}
	if loaded.CPU.Percent > 100 {
		t.Fatalf("system CPU above 100%%: %.1f%%", loaded.CPU.Percent)
	}
	t.Logf("system cpu: idle-ish=%.1f%% loaded=%.1f%%", stats.CPU.Percent, loaded.CPU.Percent)
}
