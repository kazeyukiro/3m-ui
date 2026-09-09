package mihomo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gopkg.in/yaml.v3"
)

func TestMissingListenerReportsConfigurationFailure(t *testing.T) {
	for _, tc := range []struct{ config, reason string }{
		{`{}`, "config_not_applied"},
		{`{"sni":"example.com"}`, "listener_sni_unsupported"},
		{`{"unsupported":true}`, "listener_config_invalid"},
	} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte("listeners: []\n"), 0600); err != nil {
			t.Fatal(err)
		}
		svc := &Service{pm: &ProcessManager{pid: os.Getpid()}, cm: NewConfigManager(path)}
		l := models.Listener{Name: "2", Protocol: "vless", Port: "40610", BindAddress: "0.0.0.0", Enabled: true, Config: tc.config}
		result := svc.CheckConnection(context.Background(), l, func() (string, error) { return "", fmt.Errorf("no client credentials") }, false)
		check := result.ConnectionCheck
		if check.State != "unavailable" || check.Reason != tc.reason || len(check.Steps) != 0 {
			t.Fatalf("missing node must stop before client stages: %+v", check)
		}
		if result.Reason != "config_not_applied" {
			t.Fatalf("wrong local status: %+v", result)
		}
	}
}

func TestExportedClientProxyPreservesEndpointAndProtocol(t *testing.T) {
	for _, server := range []string{"node.example.com", "2001:db8::10"} {
		for _, port := range []any{443, "8443"} {
			want := map[string]any{"name": "exported", "type": "vless", "server": server, "port": port,
				"uuid": "test-uuid", "tls": true, "servername": "tls.example.com", "skip-cert-verify": false,
				"reality-opts": map[string]any{"public-key": "test-public-key", "short-id": "abcdef"},
				"network":      "ws", "ws-opts": map[string]any{"path": "/proxy", "headers": map[string]any{"Host": "transport.example.com"}}}
			raw, _ := yaml.Marshal(map[string]any{"proxies": []map[string]any{want}})
			proxy, addr, err := exportedClientProxy(string(raw))
			want["name"] = "node-check"
			if err != nil || addr != net.JoinHostPort(server, fmt.Sprint(port)) || !reflect.DeepEqual(proxy, want) {
				t.Fatalf("exported endpoint/protocol changed: address=%s error=%v", addr, err)
			}
		}
	}
	// Keep hostname-based SNI defaults intact, instead of injecting new options.
	proxy, _, err := exportedClientProxy("proxies: [{name: test, type: vless, server: node.example.com, port: 443, tls: true}]")
	if err != nil || proxy["sni"] != nil || proxy["servername"] != nil {
		t.Fatal("implicit SNI was changed")
	}
}

func TestExportedClientProxyRejectsInvalidEndpoint(t *testing.T) {
	for _, client := range []string{
		"proxies: []",
		"proxies: [{server: '', port: 443}]",
		"proxies: [{server: 0.0.0.0, port: 443}]",
		"proxies: [{server: '::', port: 443}]",
		"proxies: [{server: 'https://node.example.com', port: 443}]",
		"proxies: [{server: node.example.com, port: 0}]",
		"proxies: [{server: node.example.com, port: 65536}]",
		"proxies: [{server: node.example.com, port: '1000-1002'}]",
		"proxies: [{server: node.example.com, port: 443, dialer-proxy: upstream}]",
	} {
		if _, _, err := exportedClientProxy(client); err == nil {
			t.Fatalf("accepted invalid profile: %s", client)
		}
	}
}

func TestConnectionCacheInvalidation(t *testing.T) {
	hash := sha256.Sum256([]byte("config"))
	l := models.Listener{BaseModel: models.BaseModel{ID: 1}}
	for _, tc := range []struct {
		name    string
		pid     int
		hash    [32]byte
		expires time.Time
		found   bool
	}{
		{"current", 42, hash, time.Now().Add(time.Minute), true},
		{"restarted", 43, hash, time.Now().Add(time.Minute), false},
		{"edited", 42, sha256.Sum256([]byte("edited")), time.Now().Add(time.Minute), false},
		{"expired", 42, hash, time.Now().Add(-time.Second), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{connectionResults: map[uint]cachedConnectionCheck{1: {ConnectionCheck{State: "available", ExpiresAt: tc.expires}, 42, hash, connectionListenerHash(l), nil}}}
			if (s.cachedConnectionResult(l, tc.pid, tc.hash) != nil) != tc.found {
				t.Fatal("wrong cache validity")
			}
		})
	}
}

func TestConnectionCacheReuseAndClientInvalidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "listeners: []\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{cm: NewConfigManager(path)}
	l := models.Listener{BaseModel: models.BaseModel{ID: 1}, Enabled: true}
	client := "original endpoint and credential"
	checks := 0
	export := func() (string, error) { checks++; return client, nil }
	first := svc.CheckConnection(context.Background(), l, export, false)
	stamp := first.ConnectionCheck.CheckedAt
	reused := svc.CheckConnection(context.Background(), l, export, true)
	if !reused.ConnectionCheck.CheckedAt.Equal(stamp) {
		t.Fatal("valid result was not reused")
	}
	if checks == 0 {
		t.Fatal("cache never validated the current client configuration")
	}
	// Same applied config and PID, but public endpoint or credential changed.
	client = "changed endpoint or credential"
	if got := svc.cachedConnectionResult(l, 0, sha256.Sum256([]byte(content))); got != nil {
		t.Fatal("cached result survived a client-only configuration change")
	}
	next := svc.CheckConnection(context.Background(), l, export, true)
	if next.ConnectionCheck.CheckedAt.Equal(stamp) {
		t.Fatal("invalid cache did not run a fresh check")
	}
	stamp = next.ConnectionCheck.CheckedAt
	forced := svc.CheckConnection(context.Background(), l, export, false)
	if forced.ConnectionCheck.CheckedAt.Equal(stamp) {
		t.Fatal("manual recheck reused cached result")
	}
}

func TestProxyTargetsRunConcurrently(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(fmt.Sprint(success), func(t *testing.T) {
			arrived := make(chan string, 2)
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				arrived <- r.URL.Query().Get("url")
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				if success && r.URL.Query().Get("url") == "https://second.invalid" {
					_, _ = io.WriteString(w, `{"delay":25}`)
				} else {
					w.WriteHeader(503)
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := controllerTestClient(server.URL)
			done := make(chan bool, 1)
			go func() {
				_, _, ok := testProxyTargets(ctx, client, []string{"https://first.invalid", "https://second.invalid"})
				done <- ok
			}()
			seen := map[string]bool{}
			for range 2 {
				select {
				case target := <-arrived:
					seen[target] = true
				case <-time.After(2 * time.Second):
					t.Fatal("targets did not start concurrently")
				}
			}
			if len(seen) != 2 {
				t.Fatal("both requests used the same target")
			}
			close(release)
			select {
			case ok := <-done:
				if ok != success {
					t.Fatal("wrong aggregate result")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("target checks did not finish")
			}
		})
	}
}

func TestProxyTargetsCancelLoserAndParent(t *testing.T) {
	for _, cancelParent := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelParent), func(t *testing.T) {
			slowStarted := make(chan struct{})
			slowCancelled := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("url") == "https://slow.invalid" {
					close(slowStarted)
					<-r.Context().Done()
					close(slowCancelled)
					return
				}
				select {
				case <-slowStarted:
				case <-r.Context().Done():
					return
				}
				if cancelParent {
					<-r.Context().Done()
					return
				}
				_, _ = io.WriteString(w, `{"delay":17}`)
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan bool, 1)
			go func() {
				_, _, ok := testProxyTargets(ctx, controllerTestClient(server.URL), []string{"https://slow.invalid", "https://fast.invalid"})
				done <- ok
			}()
			select {
			case <-slowStarted:
			case <-time.After(2 * time.Second):
				t.Fatal("slow target did not start")
			}
			if cancelParent {
				cancel()
			}
			select {
			case ok := <-done:
				if ok == cancelParent {
					t.Fatal("wrong cancelled/success result")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("check waited for a losing target")
			}
			select {
			case <-slowCancelled:
			case <-time.After(2 * time.Second):
				t.Fatal("losing request was not cancelled")
			}
		})
	}
}

// Route controller calls to the disposable HTTP server; never make a real
// request to the configured proxy or the target URLs in these unit tests.
func controllerTestClient(serverURL string) *http.Client {
	address := strings.TrimPrefix(serverURL, "http://")
	return &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", address)
	}}}
}

// Uses disposable serving and client processes. A target counter verifies that
// a wrong credential cannot pass by falling back to a direct request.
func TestRealMihomoLocalConnectionCheck(t *testing.T) {
	binary := os.Getenv("MIHOMO_TEST_BINARY")
	if binary == "" {
		t.Skip("set MIHOMO_TEST_BINARY for isolated real-core tests")
	}
	var requests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(10 * time.Millisecond) // URLTest rejects zero-millisecond samples.
		w.WriteHeader(204)
	}))
	defer target.Close()
	for _, protocol := range []string{"shadowsocks", "vless"} {
		t.Run(protocol, func(t *testing.T) {
			reserve, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := reserve.Addr().(*net.TCPAddr).Port
			reserve.Close()
			l := map[string]any{"name": "test", "type": protocol, "listen": "127.0.0.1", "port": port}
			proxy := map[string]any{"name": "node-check", "type": "ss", "server": "127.0.0.1", "port": port, "cipher": "aes-128-gcm", "password": "disposable-test-password"}
			if protocol == "shadowsocks" {
				l["cipher"], l["password"] = proxy["cipher"], proxy["password"]
			} else {
				cert, key, err := certutil.GenerateSelfSigned("localhost")
				if err != nil {
					t.Fatal(err)
				}
				l["certificate"], l["private-key"] = cert, key
				l["users"] = []map[string]any{{"username": "test", "uuid": "6f2294b8-a47c-4c73-9bea-7aa4ebca6c50"}}
				proxy = map[string]any{"name": "node-check", "type": "vless", "server": "127.0.0.1", "port": port, "uuid": "6f2294b8-a47c-4c73-9bea-7aa4ebca6c50", "tls": true, "servername": "localhost", "skip-cert-verify": true}
			}
			raw, err := yaml.Marshal(map[string]any{"listeners": []map[string]any{l}, "rules": []string{"MATCH,DIRECT"}})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			pm := NewProcessManager(binary, path)
			if err := pm.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = pm.Stop() })
			before, _ := os.ReadFile(path)
			for _, good := range []bool{true, false} {
				check := ConnectionCheck{}
				if !good {
					if protocol == "shadowsocks" {
						proxy["password"] = "wrong-test-password"
					} else {
						proxy["uuid"] = "9f2294b8-a47c-4c73-9bea-7aa4ebca6c50"
					}
				}
				count := requests.Load()
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				clientYAML, _ := yaml.Marshal(map[string]any{"proxies": []map[string]any{proxy}})
				prepared, _, err := exportedClientProxy(string(clientYAML))
				if err != nil {
					t.Fatal(err)
				}
				runClientCheck(ctx, binary, prepared, &check, []string{target.URL, target.URL + "/backup"})
				cancel()
				want := "unavailable"
				if good {
					want = "available"
				}
				if check.State != want {
					t.Fatalf("good=%v port=%s result=%+v serverLogs=%v", good, strconv.Itoa(port), check, pm.Logs())
				}
				if good && requests.Load() <= count {
					t.Fatal("test never reached target")
				}
				if !good && requests.Load() != count {
					t.Fatal("wrong credential bypassed node")
				}
			}
			// A healthy local listener must not hide an unreachable exported port.
			// Bind without listening so no other process can claim this TCP port.
			blocked, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer syscall.Close(blocked)
			if err := syscall.Bind(blocked, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
				t.Fatal(err)
			}
			bound, err := syscall.Getsockname(blocked)
			if err != nil {
				t.Fatal(err)
			}

			if protocol == "shadowsocks" {
				proxy["password"] = "disposable-test-password"
			} else {
				proxy["uuid"] = "6f2294b8-a47c-4c73-9bea-7aa4ebca6c50"
			}
			proxy["port"] = bound.(*syscall.SockaddrInet4).Port
			clientYAML, _ := yaml.Marshal(map[string]any{"proxies": []map[string]any{proxy}})
			prepared, addr, err := exportedClientProxy(string(clientYAML))
			if err != nil {
				t.Fatal(err)
			}
			count := requests.Load()
			check := ConnectionCheck{}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			runClientCheck(ctx, binary, prepared, &check, []string{target.URL, target.URL + "/backup"})
			cancel()
			if check.State != "unavailable" || requests.Load() != count || !pm.IsRunning() {
				t.Fatalf("unreachable exported endpoint %s fell back to local listener: %+v", addr, check)
			}
			proxy["port"] = port
			// Cancellation while a proxied request is in flight must reap the
			// temporary client and remove its credential-bearing directory.
			if protocol == "shadowsocks" {
				proxy["password"] = "disposable-test-password"
				started := make(chan struct{})
				release := make(chan struct{})
				slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					close(started)
					<-release
					w.WriteHeader(204)
				}))
				defer slow.Close()
				defer close(release)
				tmp := t.TempDir()
				t.Setenv("TMPDIR", tmp)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan struct{})
				go func() {
					check := ConnectionCheck{}
					runClientCheck(ctx, binary, proxy, &check, []string{slow.URL})
					close(done)
				}()
				select {
				case <-started:
				case <-time.After(5 * time.Second):
					t.Fatal("cancel test did not reach target")
				}
				cancel()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatal("cancelled test client did not stop")
				}
				files, err := os.ReadDir(tmp)
				if err != nil || len(files) != 0 {
					t.Fatalf("temporary credentials were not removed: %v", err)
				}
			}
			after, _ := os.ReadFile(path)
			if string(before) != string(after) || !pm.IsRunning() {
				t.Fatal(fmt.Sprint("serving core was modified"))
			}
		})
	}
}
