package converter

import (
	"reflect"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

// A listener published behind a CDN keeps a raw IP as the dial address
// (server) while the real TLS identity lives in access_sni. The Mihomo
// WebSocket `Host` header must follow that identity, never the dial IP:
// the share-link path already writes `host=<sni>`, and a client that sends
// `Host: 203.0.113.7` is dropped by any Host-routing edge.
func TestListenerToProxiesWSHostFollowsAccessSNI(t *testing.T) {
	l := models.Listener{
		Name:       "VLESS-CDN",
		Protocol:   "vless",
		Port:       "2053",
		PublicPort: "443",
		AccessSNI:  "cdn.example.com",
		Enabled:    true,
		Config:     `{"ws-path":"/ws"}`,
	}
	proxies, err := listenerToProxies(l, "203.0.113.7", []user.Credential{{UUID: "f3b71a1a-ff1e-45a4-9de9-c672aa347f62", Username: "admin"}})
	if err != nil {
		t.Fatal(err)
	}
	p := proxies[0]
	ws, _ := p["ws-opts"].(map[string]interface{})
	headers, _ := ws["headers"].(map[string]interface{})
	if got := headers["Host"]; got != "cdn.example.com" {
		t.Fatalf("ws Host = %#v, want cdn.example.com (dial address must not leak)", got)
	}
	if got := p["sni"]; got != "cdn.example.com" {
		t.Fatalf("sni = %#v", got)
	}
}

// An explicit Host header set by the operator wins over the derived SNI.
func TestListenerToProxiesWSHostKeepsExplicitHeader(t *testing.T) {
	l := models.Listener{
		Name:       "VLESS-EXPLICIT",
		Protocol:   "vless",
		Port:       "2053",
		PublicPort: "443",
		AccessSNI:  "cdn.example.com",
		Enabled:    true,
		Config:     `{"ws-path":"/ws","ws-headers":{"Host":"edge.example.net"}}`,
	}
	proxies, err := listenerToProxies(l, "203.0.113.7", []user.Credential{{UUID: "f3b71a1a-ff1e-45a4-9de9-c672aa347f62", Username: "admin"}})
	if err != nil {
		t.Fatal(err)
	}
	ws, _ := proxies[0]["ws-opts"].(map[string]interface{})
	headers, _ := ws["headers"].(map[string]interface{})
	if got := headers["Host"]; got != "edge.example.net" {
		t.Fatalf("ws Host = %#v, want the operator-supplied edge.example.net", got)
	}
}

// listener.access_alpn is the client-facing ALPN override. It must reach the
// YAML export for plain TLS transports (it previously only survived on the
// Reality path, which injected its own default), and it must beat the
// protocol default (h3) that hysteria2/tuic fall back to.
func TestListenerToProxiesCarriesAccessALPN(t *testing.T) {
	wsListener := models.Listener{
		Name:       "VLESS-ALPN",
		Protocol:   "vless",
		Port:       "443",
		PublicHost: "node.example.com",
		AccessALPN: "h2,http/1.1",
		Enabled:    true,
		Config:     `{"ws-path":"/ws"}`,
	}
	proxies, err := listenerToProxies(wsListener, "node.example.com", []user.Credential{{UUID: "f3b71a1a-ff1e-45a4-9de9-c672aa347f62", Username: "admin"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := proxies[0]["alpn"], []string{"h2", "http/1.1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("vless alpn = %#v, want %#v", got, want)
	}

	hy2Listener := models.Listener{
		Name:       "HY2-ALPN",
		Protocol:   "hysteria2",
		Port:       "443",
		PublicHost: "node.example.com",
		AccessALPN: "h2",
		Enabled:    true,
		Config:     `{"password":"secret"}`,
	}
	hy2, err := listenerToProxies(hy2Listener, "node.example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := hy2[0]["alpn"], []string{"h2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hysteria2 alpn = %#v, want %#v (access_alpn overrides the h3 default)", got, want)
	}
}

// Reality keeps its own ALPN pairing: the client must present the same list
// the server chose in reality-config, so a listener-level access_alpn must not
// silently rewrite it.
func TestListenerToProxiesRealityKeepsALPN(t *testing.T) {
	l := models.Listener{
		Name:       "REALITY-ALPN",
		Protocol:   "vless",
		Port:       "443",
		PublicHost: "www.microsoft.com",
		AccessALPN: "h2",
		Enabled:    true,
		Config:     `{"reality-config":{"enable":true,"dest":"www.microsoft.com:443","server-names":["www.microsoft.com"],"private-key":"jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0","short-id":["0123456789abcdef"]}}`,
	}
	proxies, err := listenerToProxies(l, "203.0.113.7", []user.Credential{{UUID: "f3b71a1a-ff1e-45a4-9de9-c672aa347f62", Username: "admin"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := proxies[0]["alpn"], []string{"h2", "http/1.1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reality alpn = %#v, want %#v", got, want)
	}
}
