package converter

import (
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

func TestVLESSRealityClientExport(t *testing.T) {
	l := models.Listener{
		Name:     "vless-in",
		Protocol: "vless",
		Port:     "443",
		Enabled:  true,
		Config: `{
			"flow":"xtls-rprx-vision",
			"reality-config":{
				"dest":"test.com:443",
				"private-key":"jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
				"short-id":["0123456789abcdef"],
				"server-names":["test.com"]
			}
		}`,
	}
	creds := []user.Credential{{Username: "u1", UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("proxies=%d", len(proxies))
	}
	p := proxies[0]
	if p["tls"] != true {
		t.Fatalf("tls want true got %#v", p["tls"])
	}
	if p["servername"] != "test.com" {
		t.Fatalf("servername=%v", p["servername"])
	}
	if p["flow"] != "xtls-rprx-vision" {
		t.Fatalf("flow=%v", p["flow"])
	}
	ro, ok := p["reality-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing reality-opts: %#v", p)
	}
	pk, _ := ro["public-key"].(string)
	if strings.TrimSpace(pk) == "" {
		t.Fatalf("public-key empty: %#v", ro)
	}
	if ro["short-id"] != "0123456789abcdef" {
		t.Fatalf("short-id=%v", ro["short-id"])
	}
	if p["client-fingerprint"] != "chrome" {
		t.Fatalf("fingerprint=%v", p["client-fingerprint"])
	}
}

func TestTrojanCertificateClientExport(t *testing.T) {
	l := models.Listener{
		Name: "trojan-in", Protocol: "trojan", Port: "443", Enabled: true,
		Config: `{"certificate":"./server.crt","private-key":"./server.key","sni":"example.com"}`,
	}
	creds := []user.Credential{{Username: "u1", Password: "secret"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	p := proxies[0]
	if p["tls"] != true {
		t.Fatalf("tls=%v", p["tls"])
	}
	if p["password"] != "secret" {
		t.Fatalf("password=%v", p["password"])
	}
}
