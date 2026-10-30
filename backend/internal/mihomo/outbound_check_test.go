package mihomo

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	mihomoconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gopkg.in/yaml.v3"
)

func TestOutboundConnectionReportsDisabledAndUnavailableClient(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	svc := &Service{}
	for _, enabled := range []bool{false, true} {
		if err := mihomoconfig.SaveServerRouting(db, mihomoconfig.ServerRoutingConfig{
			Proxies: []mihomoconfig.ProxyEntry{{Name: "edge /测试", Type: "socks5", Server: "edge.example", Port: 1080, Options: map[string]any{"password": "server-only-secret"}}},
			DisabledOutboundNames: func() []string {
				if !enabled {
					return []string{"edge /测试"}
				}
				return nil
			}(),
		}); err != nil {
			t.Fatal(err)
		}
		result, err := svc.CheckOutboundConnection(context.Background(), db, "edge /测试")
		if err != nil {
			t.Fatal(err)
		}
		want := "disabled"
		if enabled {
			want = "unknown"
		}
		if result.State != want || result.CheckedAt.IsZero() || result.ExpiresAt.Before(result.CheckedAt) {
			t.Fatalf("enabled=%v result=%+v", enabled, result)
		}
		body, _ := json.Marshal(result)
		if strings.Contains(string(body), "server-only-secret") {
			t.Fatal("test exposed proxy credentials")
		}
	}
}

func TestRealMihomoOutboundConnectionAPI(t *testing.T) {
	binary := os.Getenv("MIHOMO_TEST_BINARY")
	if binary == "" {
		t.Skip("set MIHOMO_TEST_BINARY for isolated real-core tests")
	}
	t.Cleanup(AllowBinaryPathPrefixForTesting(filepath.Dir(binary)))
	var requests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(204)
	}))
	defer target.Close()
	previousTargets := connectionTestTargets
	connectionTestTargets = []string{target.URL, target.URL + "/backup"}
	t.Cleanup(func() { connectionTestTargets = previousTargets })
	reserve, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reserve.Addr().(*net.TCPAddr).Port
	reserve.Close()
	content, _ := yaml.Marshal(map[string]any{
		"listeners": []map[string]any{{"name": "test", "type": "shadowsocks", "listen": "127.0.0.1", "port": port, "cipher": "aes-128-gcm", "password": "test-secret"}},
		"rules":     []string{"MATCH,DIRECT"},
	})
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, content, 0600); err != nil {
		t.Fatal(err)
	}
	serving := NewProcessManager(binary, configPath)
	if err := serving.Start(); err != nil {
		t.Fatal(err)
	}
	defer serving.Stop()
	upstreamSocket, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	upstreamPort := upstreamSocket.Addr().(*net.TCPAddr).Port
	upstreamSocket.Close()
	upstreamContent, _ := yaml.Marshal(map[string]any{
		"listeners": []map[string]any{{"name": "upstream", "type": "shadowsocks", "listen": "127.0.0.1", "port": upstreamPort, "cipher": "aes-128-gcm", "password": "test-secret"}},
		"rules":     []string{"MATCH,DIRECT"},
	})
	upstreamPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(upstreamPath, upstreamContent, 0600); err != nil {
		t.Fatal(err)
	}
	upstream := NewProcessManager(binary, upstreamPath)
	if err := upstream.Start(); err != nil {
		t.Fatal(err)
	}
	defer upstream.Stop()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	r := gin.New()
	RegisterOutboundConnectionRoute(r.Group("/config"), db, &Service{pm: NewProcessManager(binary, configPath)})
	for _, good := range []bool{true, false} {
		password := "test-secret"
		if !good {
			password = "wrong-secret"
		}
		err := mihomoconfig.SaveServerRouting(db, mihomoconfig.ServerRoutingConfig{
			Proxies: []mihomoconfig.ProxyEntry{
				{Name: "edge /测试?#", Type: "ss", Server: "127.0.0.1", Port: port, Options: map[string]any{"cipher": "aes-128-gcm", "password": password, "dialer-proxy": "egress"}},
				{Name: "upstream", Type: "ss", Server: "127.0.0.1", Port: upstreamPort, Options: map[string]any{"cipher": "aes-128-gcm", "password": "test-secret"}},
			},
			Groups: []mihomoconfig.GroupEntry{{Name: "egress", Type: "select", Proxies: []string{"upstream"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		before := requests.Load()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/config/server-outbounds/connection-check?"+url.Values{"name": {"edge /测试?#"}}.Encode(), nil))
		if w.Code != 200 {
			t.Fatalf("test API: %d %s", w.Code, w.Body.String())
		}
		var result ConnectionCheck
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		want := "unavailable"
		if good {
			want = "available"
		}
		if result.State != want || len(result.Steps) != 3 {
			t.Fatalf("good=%v result=%+v logs=%v", good, result, serving.Logs())
		}
		if good && requests.Load() <= before {
			t.Fatal("outbound test never reached the target")
		}
		if !good && requests.Load() != before {
			t.Fatal("bad credentials bypassed the outbound")
		}
		if strings.Contains(w.Body.String(), "test-secret") || strings.Contains(w.Body.String(), "wrong-secret") {
			t.Fatal("test API exposed credentials")
		}
	}
	after, err := os.ReadFile(configPath)
	if err != nil || !bytes.Equal(content, after) {
		t.Fatal("connectivity test changed the serving configuration")
	}
}
