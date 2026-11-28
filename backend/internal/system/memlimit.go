package system

// This file keeps the panel itself inside the memory budget of the machine it
// runs on. Small NAT / VPS boxes often have 128-512MB of RAM, **no swap** and
// a tmpfs /tmp, so an unbounded Go heap is the difference between a working
// panel and the kernel OOM killer taking the whole box down.
//
// Deliberately no mmap, no syscalls beyond reading /proc and /sys/fs/cgroup,
// and nothing that would enable or depend on swap. Memory is reclaimed the
// ordinary way: the Go runtime returns pages it no longer needs.

import (
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v4/mem"
)

// Tiers mirror scripts/install.sh compute_mem_tuning() so that the installer,
// the service unit and the running process agree on one budget instead of
// three different guesses.
const (
	tierSmallRAM  = 128 << 20 // <=128MB total
	tierMediumRAM = 256 << 20 // <=256MB total
	tierLargeRAM  = 512 << 20 // <=512MB total

	tierSmallLimit  = 48 << 20
	tierMediumLimit = 96 << 20
	tierLargeLimit  = 192 << 20

	tierSmallGC  = 20
	tierMediumGC = 50
	tierLargeGC  = 75
)

// RuntimeBudget is what ApplyRuntimeMemoryLimit decided. Zero values mean
// "leave the runtime default alone".
type RuntimeBudget struct {
	// Heap is the soft memory limit handed to the Go runtime, in bytes.
	Heap int64
	// GC is the GOGC target, or -1 when untouched.
	GC int
	// Applied reports whether anything was actually configured.
	Applied bool
}

var runtimeLimitOnce sync.Once
var runtimeLimitResult RuntimeBudget

// ApplyRuntimeMemoryLimit bounds this process' Go heap on small devices. It is
// idempotent and safe to call from every entry point (bootstrap, server,
// maintenance commands). Returns the budget that was applied.
//
// Two properties matter here:
//   - It acts **in-process** only. debug.SetMemoryLimit is not inherited by
//     children, so the Mihomo core is never forced onto the panel's tighter GC
//     target — the core has very different working-set needs.
//   - An explicit operator choice always wins. If GOMEMLIMIT or GOGC is already
//     in the environment the Go runtime has honoured it at startup, and
//     overwriting it here would silently undo a deliberate override.
func ApplyRuntimeMemoryLimit() RuntimeBudget {
	runtimeLimitOnce.Do(func() {
		runtimeLimitResult = applyRuntimeMemoryLimitOnce()
	})
	return runtimeLimitResult
}

func applyRuntimeMemoryLimitOnce() RuntimeBudget {
	budget := budgetFor(memoryAllowance(), os.Getenv("GOMEMLIMIT"), os.Getenv("GOGC"))
	if !budget.Applied {
		return budget
	}
	debug.SetMemoryLimit(budget.Heap)
	debug.SetGCPercent(budget.GC)
	return budget
}

// budgetFor decides the runtime budget from the memory allowance and the two
// environment variables that may already carry an operator's intent. Kept side
// effect free so it can be exercised without mutating global GC state.
func budgetFor(allowance float64, goMemLimit, goGC string) RuntimeBudget {
	// The runtime reads GOMEMLIMIT/GOGC at startup; never overwrite a
	// deliberate override. "off" is Go's own spelling for "no limit" and is
	// honoured by leaving the runtime alone too.
	if goMemLimit != "" || goGC != "" {
		return RuntimeBudget{}
	}
	if allowance <= 0 {
		return RuntimeBudget{}
	}
	budget, ok := runtimeTier(allowance)
	if !ok {
		return RuntimeBudget{}
	}
	budget.Applied = true
	return budget
}

// runtimeTier maps an allowance onto the shared tuning table.
func runtimeTier(allowance float64) (RuntimeBudget, bool) {
	switch {
	case allowance <= tierSmallRAM:
		return RuntimeBudget{Heap: tierSmallLimit, GC: tierSmallGC}, true
	case allowance <= tierMediumRAM:
		return RuntimeBudget{Heap: tierMediumLimit, GC: tierMediumGC}, true
	case allowance <= tierLargeRAM:
		return RuntimeBudget{Heap: tierLargeLimit, GC: tierLargeGC}, true
	default:
		return RuntimeBudget{}, false
	}
}

// memoryAllowance is the memory this process may reasonably consume, in bytes.
// The cgroup cap wins when it is set — that is the budget the kernel will
// actually enforce — otherwise fall back to the host's total RAM. Zero means
// "unknown", never "unlimited".
//
// This deliberately reuses the accounting behind the dashboard memory card so
// the number shown to the user and the number enforced here cannot drift apart.
func memoryAllowance() float64 {
	cg := readCgroupMemory()
	hostTotal := float64(0)
	if v, err := mem.VirtualMemory(); err == nil && v != nil {
		hostTotal = float64(v.Total)
	}
	if hostTotal <= 0 {
		hostTotal = memTotalFromProc()
	}
	if cg.limit > 0 && (hostTotal <= 0 || cg.limit <= hostTotal) {
		return cg.limit
	}
	return hostTotal
}

// memTotalFromProc reads MemTotal straight from /proc/meminfo. It is only a
// fallback for hosts where gopsutil cannot produce figures; cgroup-first
// accounting above already covers containers.
func memTotalFromProc() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	return parseMemTotal(string(data))
}

// parseMemTotal reads the MemTotal line out of /proc/meminfo, in bytes.
func parseMemTotal(data string) float64 {
	for _, line := range strings.Split(data, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) != "MemTotal" {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			return 0
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0
		}
		// The unit column is always kB for MemTotal; guard against kB-less
		// output instead of silently assuming.
		if len(fields) > 1 && strings.EqualFold(fields[1], "B") {
			return float64(kb)
		}
		return float64(kb) * 1024
	}
	return 0
}

// CurrentHeapLimit reports the runtime soft memory limit currently in effect.
// The Go runtime uses math.MaxInt64 for "unlimited", so callers must compare
// against that rather than looking for zero. Exposed so a small-memory install
// can be verified from the outside without reading this file's internals.
func CurrentHeapLimit() int64 {
	// A negative argument is not applied; it only reads the current setting.
	return debug.SetMemoryLimit(-1)
}
