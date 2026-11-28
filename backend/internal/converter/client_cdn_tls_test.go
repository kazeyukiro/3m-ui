package converter

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

func TestListenerToProxiesCDNWSFillsServername(t *testing.T) {
	l := models.Listener{
		Name:       "VLESS-CF",
		Protocol:   "vless",
		Port:       "2053",
		PublicHost: "dzx1283.cc.cd",
		PublicPort: "443",
		AccessSNI:  "dzx1283.cc.cd",
		Enabled:    true,
		Config:     `{"ws-path":"/ws","allow-insecure":true}`,
	}
	proxies, err := listenerToProxies(l, "dzx1283.cc.cd", []user.Credential{{UUID: "f3b71a1a-ff1e-45a4-9de9-c672aa347f62", Username: "admin"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("len=%d", len(proxies))
	}
	p := proxies[0]
	if p["tls"] != true {
		t.Fatalf("tls: %#v", p["tls"])
	}
	if p["servername"] != "dzx1283.cc.cd" {
		t.Fatalf("servername: %#v", p["servername"])
	}
	if p["client-fingerprint"] != "chrome" {
		t.Fatalf("fingerprint: %#v", p["client-fingerprint"])
	}
	ws, _ := p["ws-opts"].(map[string]interface{})
	headers, _ := ws["headers"].(map[string]interface{})
	if headers["Host"] != "dzx1283.cc.cd" {
		t.Fatalf("Host: %#v", headers)
	}
}
