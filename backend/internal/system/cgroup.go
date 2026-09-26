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

	cgroupV1MemUsage = "/sys/fs/cgroup/memory/memory.usage_in_bytes"
	cgroupV1MemMax   = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
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

// readCgroupMemory returns (used, total) from the process's own cgroup. Both
// are 0 when cgroup accounting is unavailable (bare metal, or cgroups not
// mounted), in which case the caller falls back to /proc/meminfo.
//
// v1 reports an absurdly large limit when unset (PAGE_COUNTER_MAX), so a limit
// larger than the host's total RAM is treated as "no limit" by the caller.
func readCgroupMemory() (used, total float64) {
	if u, ok := readUintFile(cgroupV2MemUsage); ok {
		used = float64(u)
	}
	if m, ok := readUintFile(cgroupV2MemMax); ok && m > 0 {
		total = float64(m)
	}
	if used == 0 || total == 0 {
		if u, ok := readUintFile(cgroupV1MemUsage); ok && used == 0 {
			used = float64(u)
		}
		if m, ok := readUintFile(cgroupV1MemMax); ok && m > 0 && total == 0 {
			total = float64(m)
		}
	}
	return used, total
}
