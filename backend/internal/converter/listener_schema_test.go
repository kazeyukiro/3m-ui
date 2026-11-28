package converter

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

func TestHysteria2ClientExportOmitsRealmOpts(t *testing.T) {
	proxies, err := listenerToProxies(models.Listener{
		Name:     "hy2",
		Protocol: "hysteria2",
		Port:     "443",
		Config:   `{"realm-opts":{"enable":true,"server-url":"https://realm.example"},"alpn":["h3"]}`,
	}, "example.com", []user.Credential{{Password: "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one proxy, got %d", len(proxies))
	}
	if _, ok := proxies[0]["realm-opts"]; ok {
		t.Fatal("realm-opts must not be emitted into Hysteria2 client proxy config")
	}
}

func TestListenerToProxiesDoesNotLeakServerTLSSecrets(t *testing.T) {
	listener := models.Listener{
		Name:     "vless-reality",
		Protocol: "vless",
		Port:     "443",
		Enabled:  true,
		Config:   `{"certificate":"/etc/server.crt","private-key":"/etc/server.key","flow":"xtls-rprx-vision","ws-path":"/ws","reality-config":{"private-key":"server-secret","short-id":["0123456789abcdef"]}}`,
	}
	proxies, err := listenerToProxies(listener, "example.com", []user.Credential{{Username: "alice", UUID: "11111111-1111-4111-8111-111111111111"}})
	if err != nil {
		t.Fatalf("listenerToProxies failed: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one proxy, got %d", len(proxies))
	}
	proxy := proxies[0]
	if proxy["tls"] != true {
		t.Fatal("TLS was not enabled from server certificate configuration")
	}
	for _, key := range []string{"certificate", "private-key", "reality-config"} {
		if _, ok := proxy[key]; ok {
			t.Fatalf("server-only field %q leaked into client config", key)
		}
	}
	if proxy["uuid"] != "11111111-1111-4111-8111-111111111111" {
		t.Fatal("listener credential UUID was not exported")
	}
	ws, ok := proxy["ws-opts"].(map[string]interface{})
	if !ok || ws["path"] != "/ws" {
		t.Fatal("WebSocket listener transport was not converted")
	}
}

func TestListenerToProxiesRejectsRealm(t *testing.T) {
	_, err := listenerToProxies(models.Listener{Name: "realm", Protocol: "hysteria2-realm", Port: "10820"}, "example.com", nil)
	if err == nil {
		t.Fatal("expected realm listener to be rejected as a client proxy export")
	}
}

func TestListenerToProxiesSupportsExtendedProtocols(t *testing.T) {
	cases := []struct {
		name     string
		protocol string
		config   string
		creds    []user.Credential
	}{
		{"snell", "snell", `{"psk":"secret","version":4}`, nil},
		{"anytls", "anytls", `{"certificate":"cert","private-key":"key"}`, []user.Credential{{Username: "alice", Password: "secret"}}},
		{"mieru", "mieru", `{"transport":"TCP"}`, []user.Credential{{Username: "alice", Password: "secret"}}},
		{"sudoku", "sudoku", `{"key":"client-key","aead-method":"chacha20-poly1305"}`, nil},
		{"trusttunnel", "trusttunnel", `{"certificate":"cert","private-key":"key"}`, []user.Credential{{Username: "alice", Password: "secret"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proxies, err := listenerToProxies(models.Listener{Name: tc.name, Protocol: tc.protocol, Port: "443", Config: tc.config}, "example.com", tc.creds)
			if err != nil {
				t.Fatalf("listenerToProxies failed: %v", err)
			}
			if len(proxies) == 0 {
				t.Fatal("expected at least one client proxy")
			}
		})
	}
}

func TestVMessClientExportMapsListenerMuxToSmux(t *testing.T) {
	proxies, err := listenerToProxies(models.Listener{
		Name: "vmess-mux", Protocol: "vmess", Port: "443",
		Config: `{"users":[{"username":"alice","uuid":"11111111-1111-4111-8111-111111111111"}],"mux-option":{"padding":true,"brutal":{"enabled":true,"up":100,"down":200}}}`,
	}, "example.com", []user.Credential{{Username: "alice", UUID: "11111111-1111-4111-8111-111111111111"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one proxy, got %d", len(proxies))
	}
	smux, ok := proxies[0]["smux"].(map[string]interface{})
	if !ok || smux["enabled"] != true || smux["padding"] != true {
		t.Fatalf("unexpected smux export: %#v", proxies[0]["smux"])
	}
	if _, ok := smux["brutal-opts"]; !ok {
		t.Fatalf("missing brutal-opts in smux export: %#v", smux)
	}
}

func TestListenerToProxiesEmptyMuxOptionSkipped(t *testing.T) {
	l := models.Listener{
		Name: "vmess-mux-empty", Protocol: "vmess", Port: "443", Enabled: true,
		Config: `{"users":[{"username":"alice","uuid":"11111111-1111-4111-8111-111111111111"}],"mux-option":{}}`,
	}
	proxies, err := listenerToProxies(l, "example.com", []user.Credential{{UUID: "11111111-1111-4111-8111-111111111111", Username: "alice"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := proxies[0]["smux"]; ok {
		t.Fatalf("empty mux-option must not export smux: %#v", proxies[0]["smux"])
	}
}
