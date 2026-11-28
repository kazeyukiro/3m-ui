package protocol

import (
	"strings"
	"testing"
)

func TestEffectiveWSHostFallback(t *testing.T) {
	node := NodeModel{PublicHost: "cdn.example.com"}
	if got := effectiveWSHost(TransportSpec{}, node, ""); got != "cdn.example.com" {
		t.Fatalf("public host fallback: %q", got)
	}
	if got := effectiveWSHost(TransportSpec{}, node, "sni.example.com"); got != "sni.example.com" {
		t.Fatalf("sni preferred: %q", got)
	}
	if got := effectiveWSHost(TransportSpec{WSHost: "ws.example.com"}, node, "sni.example.com"); got != "ws.example.com" {
		t.Fatalf("explicit host preferred: %q", got)
	}
}

func TestEffectiveWSPathDefault(t *testing.T) {
	if got := effectiveWSPath(TransportSpec{}); got != "/" {
		t.Fatalf("default path: %q", got)
	}
	if got := effectiveWSPath(TransportSpec{WSPath: "/ws"}); got != "/ws" {
		t.Fatalf("explicit path: %q", got)
	}
}

func TestVLESSShareWSIncludesHostAndPath(t *testing.T) {
	node := NodeModel{
		Name: "wscdn", Protocol: "vless", Port: "2053", PublicHost: "cdn.example.com", PublicPort: "443",
		TLS: true, UDP: true,
		VLESS: &VLESSSpec{
			SNI: "cdn.example.com", Fingerprint: "chrome",
			Transport: TransportSpec{Network: "ws", WSPath: "/ws"},
		},
	}
	share, err := (VLESSCompiler{}).BuildShare(ShareInput{Node: node, User: UserCred{UUID: "052c437b-8460-fd6b-ea36-ae6900000000"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(share.URI, "type=ws") {
		t.Fatalf("URI missing type=ws: %s", share.URI)
	}
	if !strings.Contains(share.URI, "path=") || !(strings.Contains(share.URI, "path=%2Fws") || strings.Contains(share.URI, "path=/ws")) {
		t.Fatalf("URI missing path=/ws: %s", share.URI)
	}
	if !strings.Contains(share.URI, "host=cdn.example.com") {
		t.Fatalf("URI missing host: %s", share.URI)
	}
	if !strings.Contains(share.URI, "security=tls") {
		t.Fatalf("URI missing security=tls: %s", share.URI)
	}
	if !strings.Contains(share.URI, "@cdn.example.com:443") {
		t.Fatalf("URI should use public port 443: %s", share.URI)
	}
	if !strings.Contains(share.ClientYAML, "path: /ws") || !strings.Contains(share.ClientYAML, "Host: cdn.example.com") {
		t.Fatalf("YAML missing path/host:\n%s", share.ClientYAML)
	}
}
