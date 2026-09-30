package node

import (
	"net/url"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

func TestVLESSCDNURIHasTLSAndSNI(t *testing.T) {
	l := models.Listener{
		Name:              "VLESS-CF",
		Protocol:          "vless",
		Port:              "2053",
		PublicHost:        "dzx1283.cc.cd",
		PublicPort:        "443",
		AccessSNI:         "dzx1283.cc.cd",
		ClientFingerprint: "chrome",
		Enabled:           true,
		Config:            `{"ws-path":"/ws","allow-insecure":true}`,
	}
	uris, err := ClientURIsWithCredentials(l, "dzx1283.cc.cd", []user.Credential{
		{UUID: "f3b71a1a-ff1e-45a4-9de9-c672aa347f62", Username: "admin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != 1 {
		t.Fatalf("uris=%v", uris)
	}
	u := uris[0]
	if !strings.Contains(u, "vless://") {
		t.Fatalf("not vless: %s", u)
	}
	parsed, err := url.Parse(strings.Split(u, "#")[0])
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if q.Get("security") != "tls" {
		t.Fatalf("security=%q full=%s", q.Get("security"), u)
	}
	if q.Get("sni") != "dzx1283.cc.cd" {
		t.Fatalf("sni=%q", q.Get("sni"))
	}
	if q.Get("fp") != "chrome" {
		t.Fatalf("fp=%q", q.Get("fp"))
	}
	if q.Get("type") != "ws" {
		t.Fatalf("type=%q", q.Get("type"))
	}
	if q.Get("path") != "/ws" {
		t.Fatalf("path=%q", q.Get("path"))
	}
	if q.Get("host") != "dzx1283.cc.cd" {
		t.Fatalf("host=%q", q.Get("host"))
	}
}
