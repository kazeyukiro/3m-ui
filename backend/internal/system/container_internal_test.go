package system

import (
	"os"
	"path/filepath"
	"testing"
)

// The detection must hold on a CI runner (a bare-metal-ish VM) exactly as it
// holds inside Docker, LXC and Kubernetes, so every signal is driven through
// fixtures: environment variables via t.Setenv, files through the package
// path variables. No test here may depend on the host it runs on.

// isolateContainerEnv blanks the environment signals so a case exercises
// only the ones it sets. t.Setenv restores the real values afterwards.
func isolateContainerEnv(t *testing.T) {
	t.Helper()
	t.Setenv(containerEnvVar, "")
	t.Setenv(k8sServiceHostEnvVar, "")
	t.Setenv(panelContainerEnvVar, "")
}

// resetContainerFiles points every file signal at a path that does not
// exist, so a case starts from "no signal" and adds only what it tests.
func resetContainerFiles(t *testing.T) {
	t.Helper()
	for _, p := range []*string{&dockerEnvPath, &podmanEnvPath, &procPid1Cgroup, &procPid1Comm, &procSelfCgroup} {
		clearProcFile(t, p)
	}
}

// clearProcFile points a package path variable at a nonexistent file inside
// a fresh temp dir.
func clearProcFile(t *testing.T, path *string) {
	t.Helper()
	prev := *path
	*path = filepath.Join(t.TempDir(), "absent-"+filepath.Base(prev))
	t.Cleanup(func() { *path = prev })
}

// setProcFile points a package path variable at a file holding contents.
func setProcFile(t *testing.T, path *string, contents string) {
	t.Helper()
	prev := *path
	p := filepath.Join(t.TempDir(), filepath.Base(prev))
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	*path = p
	t.Cleanup(func() { *path = prev })
}

// markerFile points a package path variable at an existing empty file, the
// shape of /.dockerenv and /run/.containerenv.
func markerFile(t *testing.T, path *string) {
	t.Helper()
	prev := *path
	p := filepath.Join(t.TempDir(), filepath.Base(prev))
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	*path = p
	t.Cleanup(func() { *path = prev })
}

func TestContainerSignalsDecision(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T)
		want  bool
	}{
		{
			// The case that must never regress: an ordinary host.
			name: "bare metal, panel under systemd",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "0::/system.slice/3m-ui.service\n")
				setProcFile(t, &procPid1Cgroup, "0::/init.scope\n")
				setProcFile(t, &procPid1Comm, "systemd\n")
			},
			want: false,
		},
		{
			// Host units named after the runtimes are not the runtimes:
			// "docker.service" is a daemon on this machine, not a container.
			name: "bare metal, host units named after runtimes",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "0::/system.slice/docker-firewall.service\n")
				setProcFile(t, &procPid1Cgroup, "0::/init.scope\n")
				setProcFile(t, &procPid1Comm, "systemd\n")
			},
			want: false,
		},
		{
			name: "bare metal, panel outside systemd",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "0::/\n")
				setProcFile(t, &procPid1Cgroup, "0::/init.scope\n")
				setProcFile(t, &procPid1Comm, "systemd\n")
			},
			want: false,
		},
		{
			name: "docker marker file",
			setup: func(t *testing.T) {
				markerFile(t, &dockerEnvPath)
			},
			want: true,
		},
		{
			// Docker with the host cgroup namespace: the panel's own path
			// runs through the container's docker-<id>.scope.
			name: "docker host cgroup namespace, systemd inside",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "0::/system.slice/docker-9f1c.scope/system.slice/3m-ui.service\n")
				setProcFile(t, &procPid1Cgroup, "0::/system.slice/docker-9f1c.scope\n")
				setProcFile(t, &procPid1Comm, "systemd\n")
			},
			want: true,
		},
		{
			name: "panel env opt-in",
			setup: func(t *testing.T) {
				t.Setenv(panelContainerEnvVar, "1")
			},
			want: true,
		},
		{
			// The documented escape hatch: an explicit falsy value beats
			// every other signal, markers included.
			name: "panel env opt-out beats every marker",
			setup: func(t *testing.T) {
				t.Setenv(panelContainerEnvVar, "0")
				markerFile(t, &dockerEnvPath)
				markerFile(t, &podmanEnvPath)
				t.Setenv(containerEnvVar, "lxc")
				t.Setenv(k8sServiceHostEnvVar, "10.96.0.1")
			},
			want: false,
		},
		{
			name: "kubernetes service host env",
			setup: func(t *testing.T) {
				t.Setenv(k8sServiceHostEnvVar, "10.96.0.1")
			},
			want: true,
		},
		{
			name: "kubernetes cgroup path",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod1.slice/cri-containerd-abc.scope/system.slice/3m-ui.service\n")
			},
			want: true,
		},
		{
			name: "lxc container env",
			setup: func(t *testing.T) {
				t.Setenv(containerEnvVar, "lxc")
			},
			want: true,
		},
		{
			name: "lxc cgroup path",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "0::/lxc/1000/system.slice/3m-ui.service\n")
			},
			want: true,
		},
		{
			name: "podman marker file",
			setup: func(t *testing.T) {
				markerFile(t, &podmanEnvPath)
			},
			want: true,
		},
		{
			name: "systemd-nspawn container env",
			setup: func(t *testing.T) {
				t.Setenv(containerEnvVar, "systemd-nspawn")
			},
			want: true,
		},
		{
			// Private cgroup namespace: PID 1 sits at the container root
			// and the panel's unit lives below it. No marker anywhere.
			name: "pid 1 above the panel, private cgroup namespace",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "0::/system.slice/3m-ui.service\n")
				setProcFile(t, &procPid1Cgroup, "0::/\n")
				setProcFile(t, &procPid1Comm, "systemd\n")
			},
			want: true,
		},
		{
			// No systemd inside: the panel's entrypoint is PID 1 and shares
			// the container's root cgroup.
			name: "pid 1 in the same cgroup, no systemd inside",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "0::/\n")
				setProcFile(t, &procPid1Cgroup, "0::/\n")
				setProcFile(t, &procPid1Comm, "sh\n")
			},
			want: true,
		},
		{
			name: "pid 1 not a known init",
			setup: func(t *testing.T) {
				setProcFile(t, &procPid1Comm, "tini\n")
			},
			want: true,
		},
		{
			// Unreadable /proc must degrade to "no signal", never to a
			// wrong one.
			name: "pid 1 unreadable, nothing else",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "0::/system.slice/3m-ui.service\n")
			},
			want: false,
		},
		{
			name: "v1 memory controller line",
			setup: func(t *testing.T) {
				setProcFile(t, &procSelfCgroup, "12:memory:/docker/9f1c\n4:name=systemd:/init.scope\n")
			},
			want: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isolateContainerEnv(t)
			resetContainerFiles(t)
			c.setup(t)
			got := gatherContainerSignals().inContainer()
			if got != c.want {
				t.Fatalf("inContainer() = %v, want %v (signals: %+v)", got, c.want, gatherContainerSignals())
			}
		})
	}
}

// The opt-out is the operator-facing contract of THREE_M_UI_CONTAINER, so
// every spelling of "off" has to work — and nothing else may.
func TestPanelContainerEnvSpellings(t *testing.T) {
	falsy := []string{"0", "false", "no", "off", "FALSE", "Off", "No"}
	truthy := []string{"1", "true", "yes", "on", "anything-else"}
	for _, v := range falsy {
		t.Run("off-"+v, func(t *testing.T) {
			isolateContainerEnv(t)
			resetContainerFiles(t)
			t.Setenv(panelContainerEnvVar, v)
			markerFile(t, &dockerEnvPath)
			if gatherContainerSignals().inContainer() {
				t.Fatalf("THREE_M_UI_CONTAINER=%q must force host-wide figures", v)
			}
		})
	}
	for _, v := range truthy {
		t.Run("on-"+v, func(t *testing.T) {
			isolateContainerEnv(t)
			resetContainerFiles(t)
			t.Setenv(panelContainerEnvVar, v)
			if !gatherContainerSignals().inContainer() {
				t.Fatalf("THREE_M_UI_CONTAINER=%q should count as a container signal", v)
			}
		})
	}
}

func TestHasContainerCgroupSegment(t *testing.T) {
	cases := []struct {
		rel  string
		want bool
	}{
		{"/system.slice/3m-ui.service", false},
		{"/init.scope", false},
		{"/user.slice/user-1000.slice/user@1000.service", false},
		// Host units merely named after the runtimes.
		{"/system.slice/docker.service", false},
		{"/system.slice/docker-firewall.service", false},
		{"/system.slice/containerd.service", false},
		{"/machine.slice", false},
		// The real container shapes.
		{"/system.slice/docker-9f1c.scope", true},
		{"/system.slice/libpod-9f1c.scope", true},
		{"/system.slice/cri-containerd-abc.scope", true},
		{"/machine.slice/machine-nat.scope", true},
		{"/lxc.payload.nat", true},
		{"/lxc/1000", true},
		{"/docker/9f1c", true},
		{"/containerd", true},
		{"/kubepods/burstable/pod123/abc", true},
		{"/kubepods.slice/kubepods-burstable.slice", true},
	}
	for _, c := range cases {
		if got := hasContainerCgroupSegment(c.rel); got != c.want {
			t.Errorf("hasContainerCgroupSegment(%q) = %v, want %v", c.rel, got, c.want)
		}
	}
}

func TestCgroupRelPath(t *testing.T) {
	cases := []struct {
		name     string
		contents string
		want     string
	}{
		{"v2", "0::/system.slice/3m-ui.service\n", "/system.slice/3m-ui.service"},
		{"v2 root", "0::/\n", "/"},
		{"v2 wins wherever it appears", "12:memory:/docker/a\n0::/system.slice/x.service\n", "/system.slice/x.service"},
		{"v1 memory", "12:memory:/docker/9f1c\n", "/docker/9f1c"},
		{"v1 co-mounted controller", "5:memory,lxc:/lxc/1000\n", "/lxc/1000"},
		{"no memory line", "1:name=systemd:/init.scope\n", ""},
		{"empty", "", ""},
		{"garbage", "not a cgroup line\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cgroupRelPath(c.contents); got != c.want {
				t.Fatalf("cgroupRelPath(%q) = %q, want %q", c.contents, got, c.want)
			}
		})
	}
}

// runningInContainer is the only consumer-facing entry point; it must agree
// with the signal table it is built from, on this host as on any other.
func TestRunningInContainerMatchesSignals(t *testing.T) {
	isolateContainerEnv(t)
	resetContainerFiles(t)
	markerFile(t, &dockerEnvPath)
	if !runningInContainer() {
		t.Fatal("runningInContainer() = false with the Docker marker present")
	}

	isolateContainerEnv(t)
	resetContainerFiles(t)
	t.Setenv(panelContainerEnvVar, "0")
	markerFile(t, &dockerEnvPath)
	if runningInContainer() {
		t.Fatal("runningInContainer() = true with the explicit opt-out")
	}
}
