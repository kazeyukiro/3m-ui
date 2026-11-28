package node

import (
	"net/url"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

// access_alpn must survive into the share URI, so the same node describes the
// same TLS handshake whether a client reads the Clash YAML or the vless link.
func TestAccessALPNReachesShareURI(t *testing.T) {
	l := models.Listener{
		Name:       "VLESS-ALPN",
		Protocol:   "vless",
		Port:       "2053",
		PublicHost: "cdn.example.com",
		PublicPort: "443",
		AccessSNI:  "cdn.example.com",
		AccessALPN: "h2,http/1.1",
		Enabled:    true,
		Config:     `{"ws-path":"/ws","allow-insecure":true}`,
	}
	uris, err := ClientURIsWithCredentials(l, "cdn.example.com", []user.Credential{
		{UUID: "f3b71a1a-ff1e-45a4-9de9-c672aa347f62", Username: "admin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != 1 {
		t.Fatalf("uris=%v", uris)
	}
	parsed, err := url.Parse(strings.Split(uris[0], "#")[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Query().Get("alpn"); got != "h2,http/1.1" {
		t.Fatalf("alpn=%q full=%s", got, uris[0])
	}
	if got := parsed.Query().Get("security"); got != "tls" {
		t.Fatalf("security=%q", got)
	}
}

// Without access_alpn the link must not invent one, and cleartext listeners
// must never carry a TLS-only parameter.
func TestAccessALPNomittedWhenUnset(t *testing.T) {
	cleartext := models.Listener{
		Name:       "VLESS-PLAIN",
		Protocol:   "vless",
		Port:       "8080",
		PublicHost: "node.example.com",
		Enabled:    true,
		Config:     `{"ws-path":"/ws"}`,
	}
	uris, err := ClientURIsWithCredentials(cleartext, "node.example.com", []user.Credential{
		{UUID: "f3b71a1a-ff1e-45a4-9de9-c672aa347f62", Username: "admin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(strings.Split(uris[0], "#")[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Query().Get("alpn"); got != "" {
		t.Fatalf("alpn=%q, want absent", got)
	}
}
