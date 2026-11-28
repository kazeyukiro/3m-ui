package system

import (
	"math"
	"os"
	"strings"
	"testing"
)

func TestBudgetForTiers(t *testing.T) {
	cases := []struct {
		name      string
		allowance float64
		wantHeap  int64
		wantGC    int
		wantApply bool
	}{
		// Tiers mirror scripts/install.sh compute_mem_tuning().
		{"unknown allowance", 0, 0, 0, false},
		{"negative allowance", -1, 0, 0, false},
		{"64MB device", 64 << 20, tierSmallLimit, tierSmallGC, true},
		{"128MB device upper bound", tierSmallRAM, tierSmallLimit, tierSmallGC, true},
		{"just above 128MB", tierSmallRAM + 1, tierMediumLimit, tierMediumGC, true},
		{"256MB device upper bound", tierMediumRAM, tierMediumLimit, tierMediumGC, true},
		{"just above 256MB", tierMediumRAM + 1, tierLargeLimit, tierLargeGC, true},
		{"512MB device upper bound", tierLargeRAM, tierLargeLimit, tierLargeGC, true},
		{"1GB device is untouched", 1 << 30, 0, 0, false},
		{"8GB device is untouched", 8 << 30, 0, 0, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := budgetFor(tt.allowance, "", "")
			if got.Applied != tt.wantApply || got.Heap != tt.wantHeap || got.GC != tt.wantGC {
				t.Fatalf("budgetFor(%v) = %+v, want applied=%v heap=%v gc=%v",
					tt.allowance, got, tt.wantApply, tt.wantHeap, tt.wantGC)
			}
		})
	}
}

// The tiers must stay below the allowance they are chosen for, otherwise the
// panel budgets more heap than the machine actually has — the exact failure
// this tuning is meant to prevent.
func TestBudgetStaysUnderAllowance(t *testing.T) {
	for _, allowance := range []float64{tierSmallRAM, tierMediumRAM, tierLargeRAM} {
		got := budgetFor(allowance, "", "")
		if !got.Applied {
			t.Fatalf("expected a budget for allowance %v", allowance)
		}
		if float64(got.Heap) >= allowance {
			t.Errorf("heap %d is not below allowance %v", got.Heap, allowance)
		}
	}
}

// An operator who set GOMEMLIMIT or GOGC already had the runtime apply it at
// startup; silently re-applying our tiers would undo a deliberate choice.
func TestBudgetForRespectsOperatorOverride(t *testing.T) {
	cases := []struct{ name, goMemLimit, goGC string }{
		{"GOMEMLIMIT set", "512MiB", ""},
		{"GOGC set", "", "40"},
		{"both set", "512MiB", "40"},
		{"GOMEMLIMIT off", "off", ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := budgetFor(tierSmallRAM, tt.goMemLimit, tt.goGC)
			if got.Applied || got.Heap != 0 || got.GC != 0 {
				t.Fatalf("expected no override, got %+v", got)
			}
		})
	}
}

func TestParseMemTotal(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want float64
	}{
		{"typical /proc/meminfo", "MemTotal:       16304000 kB\nMemFree:         100000 kB\n", 16304000 * 1024},
		{"no trailing newline", "MemTotal:       512000 kB", 512000 * 1024},
		{"tabs as separator", "MemTotal:\t262144 kB\n", 262144 * 1024},
		{"missing key", "MemFree: 1 kB\n", 0},
		{"empty file", "", 0},
		{"unparsable value", "MemTotal:       abc kB\n", 0},
		{"no size field", "MemTotal:\n", 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseMemTotal(tt.in); got != tt.want {
				t.Fatalf("parseMemTotal(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// The parse must pick MemTotal, not some other "Total" prefixed field.
func TestParseMemTotalIgnoresLookalikes(t *testing.T) {
	raw := strings.Join([]string{
		"MemTotal:       262144 kB",
		"MemFree:         65536 kB",
		"SwapTotal:     1048576 kB",
		"HugePages_Total:       0",
		"",
	}, "\n")
	if got := parseMemTotal(raw); got != 262144*1024 {
		t.Fatalf("picked up the wrong field: %v", got)
	}
}

// Guards against accidentally booking the machine's swap into the panel's heap
// budget. This tuning must work on hosts where swap does not exist at all.
func TestMemTotalSourceExcludesSwap(t *testing.T) {
	if m := parseMemTotal("SwapTotal: 1048576 kB\n"); m != 0 {
		t.Fatalf("swap-only meminfo yielded %v, want 0", m)
	}
}

func TestMemoryAllowanceIsSane(t *testing.T) {
	got := memoryAllowance()
	if got < 0 || math.IsNaN(got) {
		t.Fatalf("memoryAllowance() = %v, want a non-negative finite value (0 means unknown)", got)
	}
}

// No //go:linkname, no syscall.Mmap in this package: the small-memory work must
// not trade heap pressure for anything the kernel cannot reclaim without swap.
func TestMemlimitIntroducesNoMapping(t *testing.T) {
	data, err := os.ReadFile("memlimit.go")
	if err != nil {
		t.Skipf("cannot read source: %v", err)
	}
	for _, forbidden := range []string{"Mmap", "Munmap", "Madvise", "syscall.Mmap", "linkname"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("memlimit.go references %s; this tuning must stay mmap-free", forbidden)
		}
	}
}
