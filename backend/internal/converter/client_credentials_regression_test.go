package converter

import (
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

func ssListener() models.Listener {
	return models.Listener{
		Name:     "ss",
		Protocol: "shadowsocks",
		Port:     "443",
		Config:   `{"cipher":"aes-128-gcm","password":"cfg-secret"}`,
	}
}

// P1-1: the export must agree with the runtime — the config secret is the
// source of truth for single-secret protocols, for every bound-user count.
func TestListenerToProxiesSingleUserSSNameAndPassword(t *testing.T) {
	creds := []user.Credential{{Username: "alice", Password: "alice-pw"}}
	proxies, err := listenerToProxies(ssListener(), "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(proxies))
	}
	p := proxies[0]
	if p["password"] != "cfg-secret" {
		t.Fatalf("password = %v, want cfg-secret (config secret is the source of truth)", p["password"])
	}
	// P2-1: single user must NOT get a -1 / -alice suffix; the panel name is
	// the node name across YAML / base64 / sing-box exports.
	if p["name"] != "ss" {
		t.Fatalf("name = %v, want plain panel name ss", p["name"])
	}
}

// P1-1: multiple bound users previously errored out and dropped the node.
func TestListenerToProxiesMultiUserSSDoesNotError(t *testing.T) {
	creds := []user.Credential{
		{Username: "alice", Password: "alice-pw"},
		{Username: "bob", Password: "bob-pw"},
	}
	proxies, err := listenerToProxies(ssListener(), "1.2.3.4", creds)
	if err != nil {
		t.Fatalf("multi-user SS export errored (node vanished from subscriptions): %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(proxies))
	}
	if proxies[0]["password"] != "cfg-secret" {
		t.Fatalf("password = %v, want cfg-secret", proxies[0]["password"])
	}
}

func TestListenerToProxiesMultiUserSSNoConfigPassword(t *testing.T) {
	l := models.Listener{
		Name:     "ss2",
		Protocol: "shadowsocks",
		Port:     "443",
		Config:   `{"cipher":"aes-128-gcm"}`,
	}
	creds := []user.Credential{
		{Username: "alice", Password: "alice-pw"},
		{Username: "bob", Password: "bob-pw"},
	}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	if proxies[0]["password"] != "alice-pw" {
		t.Fatalf("password = %v, want alice-pw (first-user fallback)", proxies[0]["password"])
	}
}

func TestListenerToProxiesSnellConfigPSKWins(t *testing.T) {
	l := models.Listener{
		Name:     "snell",
		Protocol: "snell",
		Port:     "443",
		Config:   `{"psk":"cfg-psk","version":"4"}`,
	}
	creds := []user.Credential{{Username: "alice", Password: "alice-pw"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	if proxies[0]["psk"] != "cfg-psk" {
		t.Fatalf("psk = %v, want cfg-psk (config secret wins)", proxies[0]["psk"])
	}
	if proxies[0]["name"] != "snell" {
		t.Fatalf("name = %v, want snell", proxies[0]["name"])
	}
}

func TestListenerToProxiesSudokuConfigKeyWins(t *testing.T) {
	l := models.Listener{
		Name:     "sudoku",
		Protocol: "sudoku",
		Port:     "443",
		Config:   `{"key":"cfg-key","aead-method":"chacha20-poly1305"}`,
	}
	creds := []user.Credential{{Username: "alice", Password: "alice-pw"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	if proxies[0]["key"] != "cfg-key" {
		t.Fatalf("key = %v, want cfg-key (config secret wins)", proxies[0]["key"])
	}
}

// P2-1: single-user trojan/hysteria2/anytls/shadowquic/mieru/trusttunnel kept
// a forced "-1" suffix even though vless/vmess/tuic dropped it.
func TestListenerToProxiesNoForcedSequenceSuffix(t *testing.T) {
	cases := []struct {
		protocol string
		config   string
	}{
		{"trojan", `{"users":[{"username":"alice","password":"alice-pw"}]}`},
		{"hysteria2", `{"users":{"alice":"alice-pw"}}`},
		{"anytls", `{"users":{"alice":"alice-pw"}}`},
		{"shadowquic", `{"users":[{"username":"alice","password":"alice-pw"}]}`},
		{"mieru", `{"users":{"alice":"alice-pw"}}`},
		{"trusttunnel", `{"users":[{"username":"alice","password":"alice-pw"}]}`},
	}
	for _, tc := range cases {
		l := models.Listener{Name: "node", Protocol: tc.protocol, Port: "443", Config: tc.config}
		proxies, err := listenerToProxies(l, "1.2.3.4", []user.Credential{{Username: "alice", Password: "alice-pw"}})
		if err != nil {
			t.Fatalf("%s: %v", tc.protocol, err)
		}
		if len(proxies) != 1 {
			t.Fatalf("%s: expected 1 proxy, got %d", tc.protocol, len(proxies))
		}
		if proxies[0]["name"] != "node" {
			t.Fatalf("%s: name = %v, want plain panel name (P2-1)", tc.protocol, proxies[0]["name"])
		}
	}
}

func TestListenerToProxiesMultiUserGetsDisambiguatingSuffix(t *testing.T) {
	l := models.Listener{
		Name:     "hy2",
		Protocol: "hysteria2",
		Port:     "443",
		Config:   `{"users":{"alice":"alice-pw","bob":"bob-pw"}}`,
	}
	creds := []user.Credential{
		{Username: "alice", Password: "alice-pw"},
		{Username: "bob", Password: "bob-pw"},
	}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(proxies))
	}
	seen := map[string]bool{}
	for _, p := range proxies {
		name, _ := p["name"].(string)
		if name == "" || name == "hy2" {
			t.Fatalf("multi-user export must disambiguate names, got %v", proxies)
		}
		if !strings.HasPrefix(name, "hy2-") {
			t.Fatalf("name = %q, want hy2-<user> form", name)
		}
		seen[name] = true
	}
	if len(seen) != 2 {
		t.Fatalf("names collided: %v", proxies)
	}
}

// P1-4: a scalar alpn stored in the listener JSON must still export as a list.
func TestListenerToProxiesNormalizesScalarAlpn(t *testing.T) {
	l := models.Listener{
		Name:     "hy2",
		Protocol: "hysteria2",
		Port:     "443",
		Config:   `{"alpn":"h3","users":{"alice":"alice-pw"}}`,
	}
	proxies, err := listenerToProxies(l, "1.2.3.4", []user.Credential{{Username: "alice", Password: "alice-pw"}})
	if err != nil {
		t.Fatal(err)
	}
	list, ok := proxies[0]["alpn"].([]string)
	if !ok || len(list) != 1 || list[0] != "h3" {
		t.Fatalf("alpn = %#v, want []string{h3}", proxies[0]["alpn"])
	}
}
