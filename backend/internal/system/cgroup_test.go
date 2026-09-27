package system

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// LXC mounts /sys/fs/cgroup as a tmpfs and bind-mounts only the container's own
// cgroup into it (cgroup:mixed, the default). /proc/self/cgroup still reports a
// path relative to the host hierarchy, so the naive "append the reported path to
// /sys/fs/cgroup" lookup finds nothing. These tests pin the layouts that must
// still resolve, using a synthetic /proc and /sys so no container is needed.

type cgroupFixture struct {
	t    *testing.T
	root string // stands in for /sys/fs/cgroup
	mnt  string // stands in for /proc/self/mountinfo
	self string // stands in for /proc/self/cgroup
}

func newCgroupFixture(t *testing.T) *cgroupFixture {
	t.Helper()
	dir := t.TempDir()
	f := &cgroupFixture{
		t:    t,
		root: filepath.Join(dir, "sysfs"),
		mnt:  filepath.Join(dir, "mountinfo"),
		self: filepath.Join(dir, "self-cgroup"),
	}
	if err := os.MkdirAll(f.root, 0o755); err != nil {
		t.Fatal(err)
	}
	f.file(f.mnt, "")
	f.file(f.self, "")
	prevRoot, prevMnt, prevSelf := sysfsCgroupRoot, procSelfMountinfo, procSelfCgroup
	sysfsCgroupRoot, procSelfMountinfo, procSelfCgroup = f.root, f.mnt, f.self
	t.Cleanup(func() {
		sysfsCgroupRoot, procSelfMountinfo, procSelfCgroup = prevRoot, prevMnt, prevSelf
	})
	return f
}

func (f *cgroupFixture) file(path, content string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// mount declares a single cgroup mount whose exposed root is `root`, mirroring
// the 4th field of /proc/self/mountinfo. "/" means the whole hierarchy is
// visible, a deeper path means only that subtree is.
func (f *cgroupFixture) mount(fsType, root string) {
	f.t.Helper()
	mountPoint := f.root
	if fsType == "cgroup" {
		mountPoint = filepath.Join(f.root, "memory")
		if err := os.MkdirAll(mountPoint, 0o755); err != nil {
			f.t.Fatal(err)
		}
	}
	f.file(f.mnt, "36 35 98:0 "+root+" "+mountPoint+" rw,nosuid,nodev,noexec,relatime - "+fsType+" cgroup rw\n")
}

// cgroupV2 materialises a v2 cgroup directory ("" or "/" for the root itself).
func (f *cgroupFixture) cgroupV2(rel string, usage, inactive, limit uint64) {
	f.t.Helper()
	dir := f.root
	if rel != "" && rel != "/" {
		dir = filepath.Join(f.root, rel)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	f.file(filepath.Join(dir, "memory.current"), strconv.FormatUint(usage, 10)+"\n")
	f.file(filepath.Join(dir, "memory.stat"), "inactive_file "+strconv.FormatUint(inactive, 10)+"\n")
	f.file(filepath.Join(dir, "memory.max"), formatLimit(limit)+"\n")
}

// cgroupV1 materialises the v1 control files of a cgroup directory.
func (f *cgroupFixture) cgroupV1(rel string, usage, inactive, limit uint64) {
	f.t.Helper()
	dir := filepath.Join(f.root, "memory")
	if rel != "" && rel != "/" {
		dir = filepath.Join(dir, rel)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	f.file(filepath.Join(dir, "memory.usage_in_bytes"), strconv.FormatUint(usage, 10)+"\n")
	f.file(filepath.Join(dir, "memory.stat"), "total_inactive_file "+strconv.FormatUint(inactive, 10)+"\n")
	f.file(filepath.Join(dir, "memory.limit_in_bytes"), formatLimit(limit)+"\n")
}

func (f *cgroupFixture) selfV2(rel string) { f.file(f.self, "0::"+rel+"\n") }
func (f *cgroupFixture) selfV1(rel string) { f.file(f.self, "1:memory:"+rel+"\n") }

func formatLimit(limit uint64) string {
	if limit == 0 {
		return "max"
	}
	return strconv.FormatUint(limit, 10)
}

const (
	mb       = 1 << 20
	usageSM  = 120 * mb // cgroup usage
	reclSM   = 10 * mb  // reclaimable cache inside it
	limitSM  = 128 * mb // the container's cap
	wantUsed = usageSM - reclSM
)

// Ordinary host: the mount is the hierarchy root and the reported path is
// already correct. This is the case that must not regress.
func TestSelfCgroupDirResolvesPlainHierarchy(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/")
	f.selfV2("/system.slice/3m-ui.service")
	f.cgroupV2("system.slice/3m-ui.service", usageSM, reclSM, limitSM)

	want := filepath.Join(f.root, "system.slice/3m-ui.service")
	if got := selfCgroupDir(); got != want {
		t.Fatalf("selfCgroupDir() = %q, want %q", got, want)
	}
}

// LXC default layout: only the container's own cgroup is visible, under a
// tmpfs root, while /proc/self/cgroup names it by its host path.
func TestSelfCgroupDirResolvesBindMountedContainerCgroup(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/lxc/1000")
	f.selfV2("/lxc/1000")
	f.cgroupV2("", usageSM, reclSM, limitSM)

	got := selfCgroupDir()
	if got != f.root {
		t.Fatalf("selfCgroupDir() = %q, want the mount root %q", got, f.root)
	}
	if m := readServiceCgroupMemory(); !m.ok || int64(m.usage) != usageSM || int64(m.limit) != limitSM {
		t.Fatalf("readServiceCgroupMemory() = %+v, want usage=%d limit=%d", m, usageSM, limitSM)
	}
}

// The same bind mount also exposes the subtree below the container, so a unit
// running inside it must resolve too.
func TestSelfCgroupDirResolvesUnitInsideContainerSubtree(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/lxc/1000")
	f.selfV2("/lxc/1000/ns/system.slice/3m-ui.service")
	f.cgroupV2("ns/system.slice/3m-ui.service", usageSM, reclSM, limitSM)

	want := filepath.Join(f.root, "ns/system.slice/3m-ui.service")
	if got := selfCgroupDir(); got != want {
		t.Fatalf("selfCgroupDir() = %q, want %q", got, want)
	}
}

// The user-visible symptom: the system card and the per-process card were
// reading different cgroups and disagreeing. In a container both must land on
// the same accounting domain.
func TestContainerSystemAndProcessCardsAgree(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/lxc/1000")
	f.selfV2("/lxc/1000")
	f.cgroupV2("", usageSM, reclSM, limitSM)

	system := readCgroupMemory()
	service := readServiceCgroupMemory()
	if !system.ok || !service.ok {
		t.Fatalf("expected both cards to resolve: system=%+v service=%+v", system, service)
	}
	if int64(system.usage) != int64(service.usage) {
		t.Fatalf("cards disagree: system usage=%d, process usage=%d", int64(system.usage), int64(service.usage))
	}
	if int64(service.usage)-int64(service.inactiveFile) != wantUsed {
		t.Fatalf("process working set = %d, want %d", int64(service.usage-service.inactiveFile), wantUsed)
	}
}

// When the path cannot be resolved at all, a bounded hierarchy root is still
// the domain this process is accounted in — adopt it rather than falling back
// to RSS, which is what made the two cards disagree.
func TestReadServiceCgroupMemoryFallsBackToBoundedRoot(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/") // hierarchy root is visible...
	f.selfV2("/lxc/1000")   // ...but the reported path is not under it
	f.cgroupV2("", usageSM, reclSM, limitSM)

	if dir := selfCgroupDir(); dir != "" {
		t.Fatalf("expected no resolvable self cgroup, got %q", dir)
	}
	m := readServiceCgroupMemory()
	if !m.ok || int64(m.usage) != usageSM || int64(m.limit) != limitSM {
		t.Fatalf("fallback = %+v, want usage=%d limit=%d", m, usageSM, limitSM)
	}
}

// A bare-metal host has an unbounded root. Attributing the whole machine's
// memory to two processes would be a fabrication, so the fallback must not fire.
func TestReadServiceCgroupMemoryIgnoresUnboundedRoot(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/")
	f.selfV2("/lxc/1000")
	f.cgroupV2("", usageSM, reclSM, 0) // no cap

	if m := readServiceCgroupMemory(); m.ok {
		t.Fatalf("unbounded root must not be adopted, got %+v", m)
	}
}

// cgroup v1's sentinel means "no limit" too, so it must not be mistaken for a
// cap that makes the root adoptable.
func TestReadServiceCgroupMemoryIgnoresV1SentinelLimit(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup", "/")
	f.selfV1("/lxc/1000")
	f.cgroupV1("", usageSM, reclSM, cgroupV1NoLimit)

	if m := readServiceCgroupMemory(); m.ok {
		t.Fatalf("PAGE_COUNTER_MAX must not count as a cap, got %+v", m)
	}
}

// The system card keeps preferring the process' own cgroup, but only when a real
// cap is configured there; otherwise it reports machine-wide pressure.
func TestReadCgroupMemoryPrefersBoundedSelfCgroup(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/")
	f.selfV2("/system.slice/3m-ui.service")
	f.cgroupV2("system.slice/3m-ui.service", usageSM, reclSM, limitSM)
	f.cgroupV2("", 4*mb, 0, 8*mb) // hierarchy root, deliberately different

	if got := readCgroupMemory(); int64(got.usage) != usageSM {
		t.Fatalf("bounded self cgroup must win: got usage=%d, want %d", int64(got.usage), usageSM)
	}

	// Same layout, but the unit has no cap: the root describes the machine.
	g := newCgroupFixture(t)
	g.mount("cgroup2", "/")
	g.selfV2("/system.slice/3m-ui.service")
	g.cgroupV2("system.slice/3m-ui.service", usageSM, reclSM, 0)
	g.cgroupV2("", 4*mb, 0, 8*mb)

	if got := readCgroupMemory(); int64(got.usage) != 4*mb {
		t.Fatalf("unbounded self cgroup must defer to root: got usage=%d, want %d", int64(got.usage), 4*mb)
	}
}

// v1 resolves through the memory controller directory, which is a different
// path from the v2 root.
func TestSelfCgroupDirResolvesCgroupV1(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup", "/")
	f.selfV1("/system.slice/3m-ui.service")
	f.cgroupV1("system.slice/3m-ui.service", usageSM, reclSM, limitSM)

	want := filepath.Join(f.root, "memory", "system.slice/3m-ui.service")
	if got := selfCgroupDir(); got != want {
		t.Fatalf("selfCgroupDir() = %q, want %q", got, want)
	}
	m := readServiceCgroupMemory()
	if !m.ok || int64(m.usage) != usageSM || int64(m.inactiveFile) != reclSM {
		t.Fatalf("v1 service memory = %+v, want usage=%d inactive=%d", m, usageSM, reclSM)
	}
}

// A missing or malformed /proc must degrade to "unresolved", never to a wrong
// directory.
func TestSelfCgroupDirHandlesUnreadableProc(t *testing.T) {
	f := newCgroupFixture(t)
	f.mount("cgroup2", "/lxc/1000")
	if err := os.Remove(f.self); err != nil {
		t.Fatal(err)
	}
	if got := selfCgroupDir(); got != "" {
		t.Fatalf("missing /proc/self/cgroup resolved to %q, want \"\"", got)
	}

	g := newCgroupFixture(t)
	g.mount("cgroup2", "/lxc/1000")
	g.selfV2("/somewhere/else")
	if got := selfCgroupDir(); got != "" {
		t.Fatalf("unrelated path resolved to %q, want \"\"", got)
	}
}

func TestCgroupMountRootParsing(t *testing.T) {
	cases := []struct {
		name   string
		fsType string
		root   string
		// mnt builds a raw mountinfo line from the fixture's mount point;
		// nil means "emit a well-formed line for fsType/root".
		mnt  func(mountPoint string) string
		want string
	}{
		{"hierarchy root", "cgroup2", "/", nil, ""},
		{"bind mounted subtree", "cgroup2", "/lxc/1000", nil, "/lxc/1000"},
		{"trailing slash normalised", "cgroup2", "/lxc/1000/", nil, "/lxc/1000"},
		{"fs type mismatch", "cgroup", "/lxc/1000", nil, ""},
		{"malformed line", "cgroup2", "", func(string) string { return "not a mountinfo line\n" }, ""},
		{"empty mountinfo", "cgroup2", "", func(string) string { return "\n" }, ""},
		{"optional fields present", "cgroup2", "/lxc/1000", func(mp string) string {
			return "36 35 98:0 /lxc/1000 " + mp + " rw shared:1 master:2 - cgroup2 cgroup rw\n"
		}, "/lxc/1000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newCgroupFixture(t)
			if c.mnt != nil {
				f.file(f.mnt, c.mnt(f.root))
			} else {
				f.mount(c.fsType, c.root)
			}
			if got := cgroupMountRoot(f.root, "cgroup2"); got != c.want {
				t.Fatalf("cgroupMountRoot() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestHasRealLimitRejectsSentinels(t *testing.T) {
	cases := []struct {
		limit uint64
		want  bool
	}{
		{0, false},               // v2 "max"
		{cgroupV1NoLimit, false}, // v1 PAGE_COUNTER_MAX
		{128 * mb, true},
		{1, true},
	}
	for _, c := range cases {
		if got := hasRealLimit(cgroupMemory{limit: float64(c.limit)}); got != c.want {
			t.Errorf("hasRealLimit(%d) = %v, want %v", c.limit, got, c.want)
		}
	}
}
