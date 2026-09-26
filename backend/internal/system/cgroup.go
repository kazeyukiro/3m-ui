package system

import (
	"os"
	"strconv"
	"strings"
)

// cgroup accounting paths, v2 first with a v1 fallback. The v1 controller
// directory name is distro-dependent ("cpu,cpuacct" vs "cpuacct"), hence the
// probe lists below.
const (
	cgroupV2CPUStat = "/sys/fs/cgroup/cpu.stat"
	cgroupV2CPUMax  = "/sys/fs/cgroup/cpu.max"
	cgroupV2CPUSet  = "/sys/fs/cgroup/cpuset.cpus.effective"

	cgroupV2MemUsage = "/sys/fs/cgroup/memory.current"
	cgroupV2MemMax   = "/sys/fs/cgroup/memory.max"
	cgroupV2MemStat  = "/sys/fs/cgroup/memory.stat"

	cgroupV1MemUsage = "/sys/fs/cgroup/memory/memory.usage_in_bytes"
	cgroupV1MemMax   = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	cgroupV1MemStat  = "/sys/fs/cgroup/memory/memory.stat"
)

// cgroupV1CPUAcctDirs are the directories that may hold cpu accounting files
// under cgroup v1. They are probed in order.
var cgroupV1CPUAcctDirs = []string{
	"/sys/fs/cgroup/cpu,cpuacct",
	"/sys/fs/cgroup/cpuacct",
	"/sys/fs/cgroup/cpu",
}

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

// readUintField returns the value paired with key in a space-separated "key
// value" file such as cgroup v2's cpu.stat. Returns false if the key is absent.
func readUintField(path, key string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	for _, line := range strings.FieldsFunc(string(data), func(r rune) bool { return r == '\n' }) {
		k, v, ok := strings.Cut(line, " ")
		if !ok || k != key {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" || strings.EqualFold(v, "max") {
			return 0, false
		}
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// cgroupCPUUsage returns cumulative CPU microseconds consumed by this process'
// cgroup. It reads cgroup v2 cpu.stat ("usage_usec") first and falls back to
// v1 cpuacct.usage (nanoseconds), which is converted to microseconds.
func cgroupCPUUsage() (uint64, bool) {
	if usec, ok := readUintField(cgroupV2CPUStat, "usage_usec"); ok {
		return usec, true
	}
	for _, dir := range cgroupV1CPUAcctDirs {
		if nsec, ok := readUintFile(dir + "/cpuacct.usage"); ok {
			return nsec / 1000, true
		}
	}
	return 0, false
}

// cgroupCPUQuota returns the number of CPUs this cgroup may consume when a CFS
// bandwidth limit is configured ("--cpus=" / -c /cpu quota variants). Returns
// false when there is no quota, meaning the cgroup may use every visible CPU.
func cgroupCPUQuota() (float64, bool) {
	if quota, period, ok := readCPUMaxPair(cgroupV2CPUMax); ok {
		return quota / period, true
	}
	for _, dir := range cgroupV1CPUAcctDirs {
		quota, ok := readUintFile(dir + "/cpu.cfs_quota_us")
		if !ok || quota == 0 {
			continue
		}
		period, ok := readUintFile(dir + "/cpu.cfs_period_us")
		if !ok || period == 0 {
			continue
		}
		return float64(quota) / float64(period), true
	}
	return 0, false
}

// readCPUMaxPair parses a cgroup v2 cpu.max file, which holds "<quota> <period>"
// with quota "max" when unlimited.
func readCPUMaxPair(path string) (quota, period float64, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, false
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 {
		return 0, 0, false
	}
	if strings.EqualFold(fields[0], "max") {
		return 0, 0, false
	}
	q, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, false
	}
	p, err := strconv.ParseFloat(fields[1], 64)
	if err != nil || p <= 0 {
		return 0, 0, false
	}
	if q <= 0 {
		return 0, 0, false
	}
	return q, p, true
}

// parseCPUSetCount counts CPUs listed in a cpuset range string ("0-3" or
// "0,2-3"). An empty list means "inherit everything" and is reported as
// unknown rather than zero.
func parseCPUSetCount(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	count := 0
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lo, hi, found := strings.Cut(part, "-")
		start, err := strconv.Atoi(strings.TrimSpace(lo))
		if err != nil {
			return 0, false
		}
		end := start
		if found {
			if end, err = strconv.Atoi(strings.TrimSpace(hi)); err != nil {
				return 0, false
			}
		}
		if end < start {
			return 0, false
		}
		count += end - start + 1
	}
	if count <= 0 {
		return 0, false
	}
	return count, true
}

// cgroupCPUSet returns the number of CPUs this cgroup is pinned to, when a
// cpuset restriction exists.
func cgroupCPUSet() (float64, bool) {
	if data, err := os.ReadFile(cgroupV2CPUSet); err == nil {
		if n, ok := parseCPUSetCount(string(data)); ok {
			return float64(n), true
		}
	}
	if data, err := os.ReadFile("/sys/fs/cgroup/cpuset/cpuset.cpus"); err == nil {
		if n, ok := parseCPUSetCount(string(data)); ok {
			return float64(n), true
		}
	}
	return 0, false
}

// cgroupMemory is this process' cgroup memory accounting, all in bytes.
type cgroupMemory struct {
	// usage is everything charged to the cgroup: anonymous pages, slab, kernel
	// memory **and reclaimable page cache**.
	usage float64
	// inactiveFile is the slice of usage that page cache holds and the kernel
	// may drop the moment anything else wants the pages.
	inactiveFile float64
	// limit is the cap for the cgroup, 0 when unlimited.
	limit float64
	// ok reports whether usage could be read at all.
	ok bool
}

// readCgroupMemory reads the process' own cgroup accounting. cgroup v2 is
// preferred; v1 is used only when v2 is unavailable, and each field falls back
// independently. ok is false on hosts without cgroup accounting (bare metal, or
// cgroups not mounted), where the caller must fall back to /proc/meminfo.
//
// v1 reports an absurdly large limit when unset (PAGE_COUNTER_MAX), so the
// caller treats a limit above the host's total RAM as "no limit".
func readCgroupMemory() cgroupMemory {
	var m cgroupMemory
	if u, ok := readUintFile(cgroupV2MemUsage); ok {
		m.usage = float64(u)
		if f, ok := readUintField(cgroupV2MemStat, "inactive_file"); ok {
			m.inactiveFile = float64(f)
		}
		if l, ok := readUintFile(cgroupV2MemMax); ok {
			m.limit = float64(l)
		}
		m.ok = true
		return m
	}
	if u, ok := readUintFile(cgroupV1MemUsage); ok {
		m.usage = float64(u)
		if f, ok := readUintField(cgroupV1MemStat, "total_inactive_file"); ok {
			m.inactiveFile = float64(f)
		}
		if l, ok := readUintFile(cgroupV1MemMax); ok {
			m.limit = float64(l)
		}
		m.ok = true
		return m
	}
	return m
}
