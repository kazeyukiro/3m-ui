package mihomo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	psnet "github.com/shirou/gopsutil/v4/net"
)

func TestRealMihomoListenerReadinessAndPortConflict(t *testing.T) {
	binary := os.Getenv("MIHOMO_TEST_BINARY")
	if runtime.GOOS != "linux" || binary == "" {
		t.Skip("set MIHOMO_TEST_BINARY to run the isolated real-core test on Linux")
	}
	reserve, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(reserve.Addr().String())
	reserve.Close()
	var secret [24]byte
	if _, err := rand.Read(secret[:]); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("listeners: []\nrules: ['MATCH,DIRECT']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{pm: NewProcessManager(binary, path), cm: NewConfigManager(path)}
	t.Cleanup(func() { _ = svc.pm.Stop() })
	good := fmt.Sprintf("listeners:\n- name: real-ss\n  type: shadowsocks\n  listen: 0.0.0.0\n  port: %s\n  udp: true\n  cipher: aes-128-gcm\n  password: %s\nrules: ['MATCH,DIRECT']\n", port, hex.EncodeToString(secret[:]))
	if err := svc.ApplyConfig(good); err != nil {
		t.Fatalf("real-core startup: %v; logs: %v", err, svc.pm.Logs())
	}
	status := svc.ListenerRuntime([]models.Listener{{Name: "real-ss", Enabled: true}}, true)
	if status[0].State != "listening" {
		t.Fatalf("real-core status: %+v", status)
	}
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	_, blockedPort, _ := net.SplitHostPort(occupied.Addr().String())
	bad := strings.Replace(good, "port: "+port, "port: "+blockedPort, 1)
	if err := svc.ApplyConfig(bad); err == nil {
		t.Fatal("accepted a port owned by another process")
	}
	status = svc.ListenerRuntime([]models.Listener{{Name: "real-ss", Enabled: true}}, true)
	if status[0].State != "listening" {
		t.Fatalf("old real listener not restored: %+v", status)
	}
}

func TestRuntimeSocketOwnershipAndTransports(t *testing.T) {
	l := runtimeListener{Name: "ss", Type: "shadowsocks", Listen: "127.0.0.1", Port: "12000", UDP: true}
	tcp := runtimeConnection{ConnectionStat: psnet.ConnectionStat{Pid: 42, Type: syscall.SOCK_STREAM, Status: "LISTEN", Laddr: psnet.Addr{IP: "127.0.0.1", Port: 12000}}}
	udp := runtimeConnection{ConnectionStat: psnet.ConnectionStat{Pid: 42, Type: syscall.SOCK_DGRAM, Laddr: tcp.Laddr}}
	for _, tc := range []struct {
		name        string
		connections []runtimeConnection
		state       string
	}{
		{"TCP and UDP bound", []runtimeConnection{tcp, udp}, "listening"},
		{"missing UDP", []runtimeConnection{tcp}, "not_listening"},
		{"unrelated process", []runtimeConnection{{ConnectionStat: psnet.ConnectionStat{Pid: 43, Type: tcp.Type, Status: tcp.Status, Laddr: tcp.Laddr}}, udp}, "not_listening"},
		{"established TCP is not a listener", []runtimeConnection{{ConnectionStat: psnet.ConnectionStat{Pid: 42, Type: tcp.Type, Status: "ESTABLISHED", Laddr: tcp.Laddr}}, udp}, "not_listening"},
		{"different bind address", []runtimeConnection{{ConnectionStat: psnet.ConnectionStat{Pid: 42, Type: tcp.Type, Status: tcp.Status, Laddr: psnet.Addr{IP: "127.0.0.2", Port: 12000}}}, udp}, "not_listening"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := inspectListener(l, 42, tc.connections, nil); got.State != tc.state {
				t.Fatalf("got %+v, want %s", got, tc.state)
			}
		})
	}
	if got := inspectListener(l, 42, nil, errors.New("permission denied")); got.State != "unknown" {
		t.Fatalf("permission failure should be unknown: %+v", got)
	}
}

func TestRuntimeConfigurationRangesAndUDP(t *testing.T) {
	listeners, err := runtimeListeners("listeners:\n- name: quic\n  type: hysteria2\n  port: 443\n  listen: '::1'\n- name: range\n  type: vless\n  port: '12000-12002,12002'\n")
	if err != nil {
		t.Fatal(err)
	}
	endpoints, err := listeners[0].endpoints()
	if err != nil || len(endpoints) != 1 || endpoints[0].Network != "udp" {
		t.Fatalf("UDP: %+v %v", endpoints, err)
	}
	endpoints, err = listeners[1].endpoints()
	if err != nil || len(endpoints) != 3 {
		t.Fatalf("range: %+v %v", endpoints, err)
	}
	connections := []runtimeConnection{}
	for _, port := range []uint32{12000, 12002} {
		connections = append(connections, runtimeConnection{ConnectionStat: psnet.ConnectionStat{Pid: 42, Type: syscall.SOCK_STREAM, Status: "LISTEN", Laddr: psnet.Addr{IP: "0.0.0.0", Port: port}}})
	}
	if got := inspectListener(listeners[1], 42, connections, nil); got.State != "not_listening" || got.Endpoints[1].Bound {
		t.Fatalf("missed hole in range: %+v", got)
	}
	for _, invalid := range []string{"0", "65536", "3-1", "1,", "-1"} {
		if _, err := (runtimeListener{Type: "vless", Port: invalid}).endpoints(); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
	if got := inspectListener(runtimeListener{Type: "future-protocol", Port: "443"}, 42, nil, nil); got.State != "unknown" {
		t.Fatalf("unsupported: %+v", got)
	}
}

func TestRuntimeIPv6WildcardRequiresVerifiedDualStack(t *testing.T) {
	l := runtimeListener{Type: "vless", Listen: "0.0.0.0", Port: "443"}
	for _, tc := range []struct {
		name          string
		known, v6only bool
		state         string
	}{
		{"dual stack", true, false, "listening"},
		{"IPv6 only", true, true, "not_listening"},
		{"diagnostics unavailable", false, false, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := runtimeConnection{ConnectionStat: psnet.ConnectionStat{Pid: 42, Type: syscall.SOCK_STREAM, Status: "LISTEN", Laddr: psnet.Addr{IP: "::", Port: 443}}, familyKnown: tc.known, ipv6Only: tc.v6only}
			if got := inspectListener(l, 42, []runtimeConnection{conn}, nil); got.State != tc.state {
				t.Fatalf("got %+v, want %s", got, tc.state)
			}
		})
	}
}

func TestRuntimeDisabledAndStoppedCore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("listeners: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{cm: NewConfigManager(path)}
	got := svc.ListenerRuntime([]models.Listener{{BaseModel: models.BaseModel{ID: 1}}, {BaseModel: models.BaseModel{ID: 2}, Enabled: true}}, true)
	if len(got) != 2 || got[0].State != "disabled" || got[1].Reason != "core_stopped" {
		t.Fatalf("got %+v", got)
	}
}

func TestOwnedConnectionsRealLinuxSockets(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux socket ownership")
	}
	tcp, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	udp, err := net.ListenPacket("udp4", tcp.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	_, port, _ := net.SplitHostPort(tcp.Addr().String())
	connections, err := ownedConnections(context.Background(), os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	l := runtimeListener{Type: "shadowsocks", Listen: "127.0.0.1", Port: port, UDP: true}
	if got := inspectListener(l, os.Getpid(), connections, nil); got.State != "listening" {
		t.Fatalf("actual owned sockets not recognized: %+v", got)
	}
	if got := inspectListener(l, os.Getpid()+1, connections, nil); got.State != "not_listening" {
		t.Fatalf("another PID accepted: %+v", got)
	}
}

func TestApplyConfigRollsBackWhenCoreLivesButListenerIsMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux socket ownership")
	}
	dir := t.TempDir()
	t.Cleanup(AllowBinaryPathPrefixForTesting(dir))
	binary := filepath.Join(dir, "fake-core")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\ncase \"$1\" in -t|-v) exit 0;; esac\nexec sleep 60\n"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	old := "listeners: []\n"
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{pm: NewProcessManager(binary, path), cm: NewConfigManager(path)}
	t.Cleanup(func() { _ = svc.pm.Stop() })
	if err := svc.pm.Start(); err != nil {
		t.Fatal(err)
	}
	err := svc.ApplyConfig("listeners:\n- name: missing\n  type: vless\n  listen: 127.0.0.1\n  port: 29048\n")
	if err == nil || !strings.Contains(err.Error(), "did not start") {
		t.Fatalf("expected binding failure, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != old || !svc.pm.IsRunning() {
		t.Fatalf("rollback failed: config=%s err=%v", got, err)
	}
}

func inspectListener(l runtimeListener, pid int, connections []runtimeConnection, err error) ListenerRuntime {
	endpoints, endpointErr := l.endpoints()
	return inspectEndpoints(endpoints, endpointErr, indexSockets(pid, connections), err, true)
}

func TestSocketIndexRetainsVerifiedBindings(t *testing.T) {
	wildcard := runtimeConnection{ConnectionStat: psnet.ConnectionStat{Pid: 42, Type: syscall.SOCK_STREAM, Status: "LISTEN", Laddr: psnet.Addr{IP: "::", Port: 443}}}
	ipv4 := runtimeConnection{ConnectionStat: psnet.ConnectionStat{Pid: 42, Type: syscall.SOCK_STREAM, Status: "LISTEN", Laddr: psnet.Addr{IP: "0.0.0.0", Port: 443}}}
	for _, connections := range [][]runtimeConnection{{wildcard, ipv4}, {ipv4, wildcard}} {
		got := inspectListener(runtimeListener{Type: "vless", Port: "443"}, 42, connections, nil)
		if got.State != "listening" {
			t.Fatalf("unknown IPv6 masked verified IPv4: %+v", got)
		}
	}
	ipv4.Laddr.IP = "::ffff:127.0.0.1"
	if got := inspectListener(runtimeListener{Type: "vless", Listen: "127.0.0.1", Port: "443"}, 42, []runtimeConnection{ipv4}, nil); got.State != "listening" {
		t.Fatalf("mapped IPv4 address not normalized: %+v", got)
	}
}

func TestLargeRangeSummaryStillChecksMiddlePorts(t *testing.T) {
	endpoints, err := (runtimeListener{Type: "tuic", Port: "10000-50000"}).endpoints()
	if err != nil {
		t.Fatal(err)
	}
	sockets := socketIndex{}
	for _, e := range endpoints {
		sockets[endpointKey(e.Network, e.Address, e.Port)] = socketBound
	}
	delete(sockets, endpointKey("udp", "0.0.0.0", 30000))
	summary := inspectEndpoints(endpoints, nil, sockets, nil, false)
	if summary.State != "not_listening" || len(summary.Endpoints) != 0 {
		t.Fatalf("invalid summary: %+v", summary)
	}
	payload, err := json.Marshal(summary)
	if err != nil || len(payload) > 512 {
		t.Fatalf("summary size=%d, err=%v", len(payload), err)
	}
	details := inspectEndpoints(endpoints, nil, sockets, nil, true)
	if len(details.Endpoints) != 40001 || details.Endpoints[20000].Bound || !details.Endpoints[0].Bound || !details.Endpoints[40000].Bound {
		t.Fatal("details lost range hole or bound endpoints")
	}
	sockets[endpointKey("udp", "0.0.0.0", 30000)] = socketBound
	if got := inspectEndpoints(endpoints, nil, sockets, nil, false); got.State != "listening" {
		t.Fatalf("complete range: %+v", got)
	}
}

func TestReadinessDeadlinePreservesMissingEvidence(t *testing.T) {
	listeners := []runtimeListener{{Name: "missing", Type: "vless", Port: "443"}}
	endpoints, _ := listeners[0].endpoints()
	for _, initialMissing := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		calls := 0
		err := waitForListenerSockets(ctx, listeners, [][]RuntimeEndpoint{endpoints}, func() int { return 42 }, func(ctx context.Context, _ int) ([]runtimeConnection, error) {
			calls++
			if initialMissing && calls == 1 {
				return nil, nil
			}
			<-ctx.Done()
			return nil, ctx.Err()
		})
		cancel()
		if initialMissing && (err == nil || !strings.Contains(err.Error(), "did not start")) {
			t.Fatalf("lost missing evidence: %v", err)
		}
		if !initialMissing && err != nil {
			t.Fatalf("unavailable inspection treated as bind failure: %v", err)
		}
	}
}

func BenchmarkLargeRangeRuntimeSummary(b *testing.B) {
	endpoints, _ := (runtimeListener{Type: "tuic", Port: "10000-50000"}).endpoints()
	connections := make([]runtimeConnection, len(endpoints))
	for i, e := range endpoints {
		connections[i] = runtimeConnection{ConnectionStat: psnet.ConnectionStat{Pid: 42, Type: syscall.SOCK_DGRAM, Laddr: psnet.Addr{IP: e.Address, Port: e.Port}}}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if inspectEndpoints(endpoints, nil, indexSockets(42, connections), nil, false).State != "listening" {
			b.Fatal("missing port")
		}
	}
}

func TestOwnedConnectionsRealLinuxWildcards(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux socket diagnostics")
	}
	for _, network := range []string{"tcp", "tcp6", "udp", "udp6"} {
		t.Run(network, func(t *testing.T) {
			var address net.Addr
			if strings.HasPrefix(network, "tcp") {
				listener, err := net.Listen(network, "[::]:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				address = listener.Addr()
			} else {
				listener, err := net.ListenPacket(network, "[::]:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				address = listener.LocalAddr()
			}
			_, port, _ := net.SplitHostPort(address.String())
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			connections, err := ownedConnections(ctx, os.Getpid())
			if err != nil {
				t.Fatal(err)
			}
			protocol := "vless"
			if strings.HasPrefix(network, "udp") {
				protocol = "tuic"
			}
			want := "listening"
			if strings.HasSuffix(network, "6") {
				want = "not_listening"
			}
			got := inspectListener(runtimeListener{Type: protocol, Listen: "0.0.0.0", Port: port}, os.Getpid(), connections, nil)
			if got.State != want {
				t.Fatalf("IPv4 coverage: got %+v, want %s", got, want)
			}
			got = inspectListener(runtimeListener{Type: protocol, Listen: "::", Port: port}, os.Getpid(), connections, nil)
			if got.State != "listening" {
				t.Fatalf("IPv6 listener: %+v", got)
			}
		})
	}
}
