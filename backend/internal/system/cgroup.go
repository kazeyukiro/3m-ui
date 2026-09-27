package system

import (
	"os"
	"strconv"
	"strings"
)

// cgroup accounting paths, v2 first with a v1 fallback. The v1 controller
// directory name is distro-dependent ("cpu,cpuacct" vs "cpuacct"), hence the
// probe lists below.
//
// Memory paths are deliberately absent: those files are read relative to a
// resolved cgroup directory (see cgroupMemoryFiles), because a container's
// accounting does not live at the hierarchy root.
const (
	cgroupV2CPUStat = "/sys/fs/cgroup/cpu.stat"
	cgroupV2CPUMax  = "/sys/fs/cgroup/cpu.max"
	cgroupV2CPUSet  = "/sys/fs/cgroup/cpuset.cpus.effective"
)

// cgroupV1CPUAcctDirs are the directories that may hold cpu accounting files
// under cgroup v1. They are probed in order.
var cgroupV1CPUAcctDirs = []string{
	"/sys/fs/cgroup/cpu,cpuacct",
	"/sys/fs/cgroup/cpuacct",
	"/sys/fs/cgroup/cpu",
}

// Filesystem paths, as variables so tests can point them at fixtures. Path
// resolution is the part that has to be right inside LXC, and that cannot be
// exercised against a CI runner's real /proc.
var (
	procSelfCgroup    = "/proc/self/cgroup"
	procSelfMountinfo = "/proc/self/mountinfo"
	sysfsCgroupRoot   = "/sys/fs/cgroup"
)

// cgroupV1NoLimit is the sentinel cgroup v1 reports for "no limit". It is
// astronomically larger than any real cap, so it must never be treated as one.
const cgroupV1NoLimit = 1 << 62

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

// hasRealLimit reports whether a finite memory cap is actually enforced. cgroup
// v2 writes "max" when unset (which readUintFile already rejects), while v1
// writes the PAGE_COUNTER_MAX sentinel, which parses as a number.
func hasRealLimit(m cgroupMemory) bool {
	return m.limit > 0 && m.limit < cgroupV1NoLimit
}

// cgroupJoin maps a path from /proc/self/cgroup onto a directory under mount.
func cgroupJoin(mount, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "/" {
		return mount
	}
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}
	return mount + rel
}

func cgroupDirExists(dir string) bool {
	st, err := os.Stat(dir)
	return err == nil && st.IsDir()
}

// cgroupMountRoot returns the subdirectory of the hierarchy that the kernel
// presents at mountPoint, or "" when mountPoint is the hierarchy root itself or
// the mount cannot be identified.
//
// This is the difference between a container and a plain host. With LXC's
// default cgroup:mixed, /sys/fs/cgroup is a tmpfs and the container's own cgroup
// is bind-mounted into a directory below it, so the path /proc/self/cgroup
// reports is relative to the *host* hierarchy and describes directories that do
// not exist from inside the container.
func cgroupMountRoot(mountPoint, fsType string) string {
	data, err := os.ReadFile(procSelfMountinfo)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		// mountinfo: id parent major:minor root mountpoint opts [optional...] - fstype source superopts
		fields := strings.Fields(line)
		sep := -1
		for i, f := range fields {
			if f == "-" {
				sep = i
				break
			}
		}
		if sep < 5 || sep+1 >= len(fields) {
			continue
		}
		if fields[4] != mountPoint || fields[sep+1] != fsType {
			continue
		}
		root := strings.TrimSuffix(fields[3], "/")
		if root == "" || root == "/" {
			return ""
		}
		return root
	}
	return ""
}

// cgroupDirPath resolves a /proc/self/cgroup path to a readable directory under
// mountPoint, returning "" when this process' cgroup cannot be reached there.
func cgroupDirPath(mountPoint, fsType, rel string) string {
	// Plain layout: what is mounted is the hierarchy root, so the reported path
	// is already correct.
	if dir := cgroupJoin(mountPoint, rel); cgroupDirExists(dir) {
		return dir
	}
	// Container layout: only a subtree is mounted, and the reported path starts
	// above it. Translate by dropping the part the mount hides.
	root := cgroupMountRoot(mountPoint, fsType)
	if root == "" {
		return ""
	}
	if rel != root && !strings.HasPrefix(rel, root+"/") {
		return ""
	}
	if dir := cgroupJoin(mountPoint, strings.TrimPrefix(rel, root)); cgroupDirExists(dir) {
		return dir
	}
	return ""
}

// selfCgroupDir returns the absolute sysfs directory for this process' cgroup
// on the unified hierarchy (v2) or the memory controller (v1). Empty when the
// path cannot be resolved — callers then fall back to the legacy root paths.
func selfCgroupDir() string {
	data, err := os.ReadFile(procSelfCgroup)
	if err != nil {
		return ""
	}
	var v2Rel, v1MemRel string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// v2: "0::/system.slice/3m-ui.service"
		if strings.HasPrefix(line, "0::") {
			rel := strings.TrimPrefix(line, "0::")
			if rel == "" {
				rel = "/"
			}
			v2Rel = rel
			continue
		}
		// v1: "memory:/system.slice/3m-ui.service" or "1:memory:/..."
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && (parts[1] == "memory" || strings.Contains(parts[1], "memory")) {
			v1MemRel = parts[2]
		}
	}
	if v2Rel != "" {
		if dir := cgroupDirPath(sysfsCgroupRoot, "cgroup2", v2Rel); dir != "" {
			return dir
		}
	}
	if v1MemRel != "" {
		if dir := cgroupDirPath(sysfsCgroupRoot+"/memory", "cgroup", v1MemRel); dir != "" {
			return dir
		}
	}
	return ""
}

// cgroupMemoryFiles reads memory accounting rooted at v2Dir in the cgroup v2
// layout, falling back to the v1 layout rooted at v1Dir. The two roots differ
// for the hierarchy root ("/sys/fs/cgroup" vs "/sys/fs/cgroup/memory") but are
// the same directory once a specific cgroup has been resolved. ok reports
// whether usage could be read at all.
func cgroupMemoryFiles(v2Dir, v1Dir string) (cgroupMemory, bool) {
	if u, ok := readUintFile(v2Dir + "/memory.current"); ok {
		var m cgroupMemory
		m.usage = float64(u)
		if f, ok := readUintField(v2Dir+"/memory.stat", "inactive_file"); ok {
			m.inactiveFile = float64(f)
		}
		if l, ok := readUintFile(v2Dir + "/memory.max"); ok {
			m.limit = float64(l)
		}
		m.ok = true
		return m, true
	}
	if u, ok := readUintFile(v1Dir + "/memory.usage_in_bytes"); ok {
		var m cgroupMemory
		m.usage = float64(u)
		if f, ok := readUintField(v1Dir+"/memory.stat", "total_inactive_file"); ok {
			m.inactiveFile = float64(f)
		}
		if l, ok := readUintFile(v1Dir + "/memory.limit_in_bytes"); ok {
			m.limit = float64(l)
		}
		m.ok = true
		return m, true
	}
	return cgroupMemory{}, false
}

// readCgroupMemory reads the accounting that describes the system card. The
// process' own cgroup wins when a finite cap is configured on it, because that
// cap is what the kernel will actually enforce. Otherwise the hierarchy root is
// used, so a bare-metal host still shows machine-wide pressure rather than only
// the panel's own unit.
func readCgroupMemory() cgroupMemory {
	if dir := selfCgroupDir(); dir != "" {
		if m, ok := cgroupMemoryFiles(dir, dir); ok && hasRealLimit(m) {
			return m
		}
	}
	return readRootCgroupMemory()
}

// readServiceCgroupMemory returns the accounting for the cgroup this process
// actually lives in, used to split the panel's and the core's memory so their
// sum reconciles with the system card.
func readServiceCgroupMemory() cgroupMemory {
	if dir := selfCgroupDir(); dir != "" {
		if m, ok := cgroupMemoryFiles(dir, dir); ok {
			return m
		}
	}
	// Resolution failed outright — a container whose /proc/self/cgroup is
	// expressed relative to a hierarchy it cannot see. When the hierarchy root
	// is itself bounded, that root is the domain we are accounted in, and
	// adopting it keeps this card consistent with the system card.
	//
	// The bounded test is what keeps this safe: on a bare-metal host the root
	// carries no cap, and attributing the whole machine's memory to two
	// processes would be a fabrication. There, callers keep RSS instead.
	if root, ok := cgroupMemoryFiles(sysfsCgroupRoot, sysfsCgroupRoot+"/memory"); ok && hasRealLimit(root) {
		return root
	}
	return cgroupMemory{}
}

func readRootCgroupMemory() cgroupMemory {
	m, _ := cgroupMemoryFiles(sysfsCgroupRoot, sysfsCgroupRoot+"/memory")
	return m
}
