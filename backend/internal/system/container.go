package system

// Container detection.
//
// The system card reports what /proc/meminfo says, and on most container
// runtimes that describes the *host*, not the container. s-ui and 3x-ui
// accept that; this panel goes one step further: when it can tell it is
// running inside a container, it reports the container's own memory
// accounting instead (see sampleMemory and readContainerRootMemory).
//
// Getting the detection wrong is worse than not trying. A false positive on
// a bare-metal host would replace machine-wide figures with a cgroup's, and
// a missed container merely shows the host figures s-ui shows anyway. So
// detection is deliberately redundant — several independent signals, any one
// of which is enough — and every consumer degrades to the host view when the
// container accounting cannot be read. The one explicitly settable signal,
// THREE_M_UI_CONTAINER, also works as an operator opt-out.

import (
	"os"
	"strings"
)

// Filesystem paths, as variables so tests can point them at fixtures. The
// marker files and the /proc entries are the parts that cannot be exercised
// against a CI runner's real root filesystem.
var (
	dockerEnvPath  = "/.dockerenv"
	podmanEnvPath  = "/run/.containerenv"
	procPid1Cgroup = "/proc/1/cgroup"
	procPid1Comm   = "/proc/1/comm"
)

const (
	// containerEnvVar is set inside LXC ("lxc") and systemd-nspawn
	// ("systemd-nspawn") containers, and by some OCI runtimes ("oci").
	containerEnvVar = "container"
	// k8sServiceHostEnvVar is injected into every pod's environment by the
	// kubelet; nothing on a bare-metal host sets it.
	k8sServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"
	// panelContainerEnvVar is exported by this project's own Docker image.
	// Setting it to a falsy value is the documented way to force host-wide
	// figures even when every other signal says "container".
	panelContainerEnvVar = "THREE_M_UI_CONTAINER"
)

// containerCgroupSegments are cgroup path segments that only ever appear in
// a container's own path, because the runtimes create them for the
// containers they run. Matched as whole segments, so a host unit named
// "docker.service" is not mistaken for one.
var containerCgroupSegments = []string{
	"docker",
	"kubepods",
	"lxc",
	"libpod",
	"containerd",
}

// containerScopePrefixes pair with a ".scope" suffix: the systemd cgroup
// drivers name a container's cgroup "<prefix>-<id>.scope".
var containerScopePrefixes = []string{
	"docker",
	"libpod",
	"cri-containerd",
	"machine",
}

// knownInits are the /proc/1/comm values of PID 1 on hosts that are not
// containers. Anything else owning PID 1 — a shell, an entrypoint, tini —
// means this process sits at the top of a container's PID namespace.
//
// A host booting an init outside this list (a runit- or s6-based host) can
// still be misread as a container; that failure is safe, because such a
// host's hierarchy root carries no memory cap and the card keeps reporting
// machine-wide figures. The alternative — trusting the signal alone — is
// what this list exists to avoid.
var knownInits = map[string]bool{
	"systemd":     true,
	"init":        true,
	"sysvinit":    true,
	"runit":       true,
	"runit-init":  true,
	"s6-svscan":   true,
	"openrc-init": true,
	"busybox":     true,
	"shepherd":    true,
	"finit":       true,
}

// containerSignals is everything observable about the environment, gathered
// without trusting any single source.
type containerSignals struct {
	containerEnv         bool
	k8sEnv               bool
	panelContainerEnv    bool
	panelContainerEnvOff bool
	dockerEnv            bool
	podmanEnv            bool
	containerCgroupPath  bool
	pid1NotInit          bool
	pid1SharesCgroup     bool
	selfCgroupPath       string
	pid1CgroupPath       string
}

// inContainer decides from the gathered signals. An explicitly falsy
// THREE_M_UI_CONTAINER is an operator's override and wins over everything
// else, markers included.
func (s containerSignals) inContainer() bool {
	if s.panelContainerEnvOff {
		return false
	}
	return s.containerEnv || s.k8sEnv || s.panelContainerEnv ||
		s.dockerEnv || s.podmanEnv || s.containerCgroupPath ||
		s.pid1NotInit || s.pid1SharesCgroup
}

// gatherContainerSignals reads every signal. Paths go through the package
// variables so tests can substitute fixtures.
func gatherContainerSignals() containerSignals {
	var s containerSignals
	s.containerEnv = strings.TrimSpace(os.Getenv(containerEnvVar)) != ""
	s.k8sEnv = strings.TrimSpace(os.Getenv(k8sServiceHostEnvVar)) != ""
	switch v := strings.TrimSpace(os.Getenv(panelContainerEnvVar)); {
	case v == "":
	case isFalsy(v):
		s.panelContainerEnvOff = true
	default:
		s.panelContainerEnv = true
	}
	s.dockerEnv = fileExists(dockerEnvPath)
	s.podmanEnv = fileExists(podmanEnvPath)
	if rel := cgroupRelPath(readProcFile(procSelfCgroup)); rel != "" {
		s.selfCgroupPath = rel
		s.containerCgroupPath = hasContainerCgroupSegment(rel)
	}
	if rel := cgroupRelPath(readProcFile(procPid1Cgroup)); rel != "" {
		s.pid1CgroupPath = rel
	}
	if comm := strings.TrimSpace(readProcFile(procPid1Comm)); comm != "" && !knownInits[comm] {
		s.pid1NotInit = true
	}
	// Structure rather than markers: inside a container PID 1 sits at the
	// root of the container's cgroup and every other process lives at or
	// below it. On a host, PID 1 is systemd's own /init.scope, which no
	// service is ever placed under.
	if s.selfCgroupPath != "" && s.pid1CgroupPath != "" {
		s.pid1SharesCgroup = s.pid1CgroupPath == s.selfCgroupPath ||
			strings.HasPrefix(s.selfCgroupPath, strings.TrimSuffix(s.pid1CgroupPath, "/")+"/")
	}
	return s
}

// runningInContainer reports whether this process appears to run inside a
// container. Evaluated per call on purpose: the signals are a handful of
// tiny file reads (the sampler already reads /proc/meminfo, /proc/stat and
// mountinfo each poll), and a cache could only ever serve a stale answer.
func runningInContainer() bool {
	return gatherContainerSignals().inContainer()
}

// isFalsy recognises the spellings operators use for "off".
func isFalsy(v string) bool {
	switch strings.ToLower(v) {
	case "0", "false", "no", "off":
		return true
	}
	return false
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func readProcFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// cgroupRelPath extracts the path of a process' cgroup from the contents of
// a /proc/<pid>/cgroup file: the unified hierarchy's line ("0::/path") when
// cgroup v2 is in use, otherwise the memory controller's line, which is what
// the memory card accounts against. Returns "" for an unreadable or
// unrecognised file.
func cgroupRelPath(contents string) string {
	var v1Mem string
	for _, line := range strings.Split(contents, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "0::") {
			rel := strings.TrimPrefix(line, "0::")
			if rel == "" {
				rel = "/"
			}
			return rel
		}
		// v1: "12:memory:/docker/<id>" or "12:memory,lxc:/..."
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && strings.Contains(parts[1], "memory") && v1Mem == "" {
			v1Mem = parts[2]
		}
	}
	return v1Mem
}

// hasContainerCgroupSegment reports whether a cgroup path runs through a
// segment only a container runtime creates. Exact segments first (a host
// unit "docker.service" is not the "docker" hierarchy), then the runtime-
// specific shapes: "lxc.payload.<name>" monitors, "<prefix>-<id>.scope"
// systemd scopes, and the Kubernetes pod slices.
func hasContainerCgroupSegment(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			continue
		}
		if isContainerBoundarySegment(seg) {
			return true
		}
	}
	return false
}

// isContainerBoundarySegment reports whether one cgroup path segment marks
// where a container's own cgroup begins. Used both to detect containers and
// to stop path rewriting at the container instead of stripping past it.
func isContainerBoundarySegment(seg string) bool {
	for _, want := range containerCgroupSegments {
		if seg == want {
			return true
		}
	}
	if strings.HasPrefix(seg, "lxc.") || strings.HasPrefix(seg, "kubepods") {
		return true
	}
	if strings.HasSuffix(seg, ".scope") {
		for _, prefix := range containerScopePrefixes {
			if strings.HasPrefix(seg, prefix+"-") {
				return true
			}
		}
	}
	return false
}
