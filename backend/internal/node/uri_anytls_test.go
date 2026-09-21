package node

import (
	"strings"
	"testing"
)

func TestAnyTLSURIMatchesOfficialScheme(t *testing.T) {
	cfg := map[string]interface{}{
		"users": map[string]interface{}{
			"alice": "letmein",
		},
		"sni":              "real.example.com",
		"skip-cert-verify": true,
	}
	uris, err := anytlsURIs("My Node", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != 1 {
		t.Fatalf("want 1 uri, got %#v", uris)
	}
	u := uris[0]
	if !strings.HasPrefix(u, "anytls://letmein@example.com:443?") && !strings.HasPrefix(u, "anytls://letmein@example.com:443#") {
		// password may be encoded; still must be anytls scheme with @host
		if !strings.Contains(u, "anytls://") || !strings.Contains(u, "@example.com:443") {
			t.Fatalf("unexpected uri: %s", u)
		}
	}
	if !strings.Contains(u, "sni=real.example.com") {
		t.Fatalf("missing sni: %s", u)
	}
	if !strings.Contains(u, "insecure=1") {
		t.Fatalf("missing insecure=1: %s", u)
	}
	if strings.Contains(u, "allowInsecure") || strings.Contains(u, "fp=") {
		t.Fatalf("non-official query keys should not appear: %s", u)
	}
	if !strings.Contains(u, "#") {
		t.Fatalf("missing fragment name: %s", u)
	}
}

func TestAnyTLSURIPasswordEncoding(t *testing.T) {
	cfg := map[string]interface{}{
		"users": map[string]interface{}{
			"u": "p@ss:word/x",
		},
		"sni": "example.com",
	}
	uris, err := anytlsURIs("n", "1.2.3.4", "8443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	u := uris[0]
	// Special chars in password must be percent-encoded in userinfo.
	if strings.Contains(u, "p@ss:word/x@") {
		t.Fatalf("password not encoded: %s", u)
	}
	if !strings.Contains(u, "@1.2.3.4:8443") {
		t.Fatalf("host missing: %s", u)
	}
}
