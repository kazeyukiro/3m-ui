package node

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// Formal IP-style identity: self-signed is panel; we force non-panel by checking
// DecideClientSkipCertVerify with empty PEM + domain vs the resolve path.

func TestHysteria2URIWithIPHostUsesResolvedSNINoInsecureWhenDomainAccess(t *testing.T) {
	// Domain public host + formal-looking path: no PEM → domain must not force insecure.
	cfg := map[string]interface{}{
		"users": map[string]interface{}{"alice": "secret-pass"},
		"sni":   "vpn.example.com",
	}
	uris, err := hysteria2URIs("hy2-node", "vpn.example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != 1 {
		t.Fatalf("want 1 uri, got %d", len(uris))
	}
	u := uris[0]
	if strings.Contains(u, "insecure=1") || strings.Contains(u, "allowInsecure=1") {
		t.Fatalf("domain + no panel self-signed must not skip verify: %s", u)
	}
	if !strings.Contains(u, "sni=vpn.example.com") {
		t.Fatalf("expected sni=vpn.example.com in %s", u)
	}
	if !strings.HasPrefix(u, "hysteria2://") {
		t.Fatalf("scheme: %s", u)
	}
}

func TestAnyTLSURIRejectsMismatchedAccessSNIWhenCertIsIP(t *testing.T) {
	cert, _, err := certutil.GenerateSelfSigned("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	// Panel self-signed WILL skip — that is correct for GenerateSelfSigned.
	// Here we only assert SNI is not www.bing.com.
	cfg := map[string]interface{}{
		"users":       map[string]interface{}{"u1": "pass1"},
		"certificate": cert,
		"sni":         "www.bing.com",
	}
	uris, err := anytlsURIs("any", "127.0.0.1", "10671", cfg)
	if err != nil {
		t.Fatal(err)
	}
	u := uris[0]
	if strings.Contains(u, "sni=www.bing.com") {
		t.Fatalf("mismatched AccessSNI must not appear: %s", u)
	}
}

func TestClientURIsApplyProfileSNIAgainstCert(t *testing.T) {
	cert, key, err := certutil.GenerateSelfSigned("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	_ = key
	cfg, _ := json.Marshal(map[string]interface{}{
		"users":       map[string]interface{}{"u1": "p1"},
		"certificate": cert,
	})
	l := models.Listener{
		Name:      "节点3",
		Protocol:  "hysteria2",
		Port:      "43829",
		Enabled:   true,
		Config:    string(cfg),
		AccessSNI: "www.bing.com",
	}
	uris, err := ClientURIs(l, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) == 0 {
		t.Fatal("no uris")
	}
	if strings.Contains(uris[0], "www.bing.com") {
		t.Fatalf("AccessSNI must not leak into HY2 URI: %s", uris[0])
	}
}

func TestVlessRealityURIHasNoInsecureFlag(t *testing.T) {
	cfg := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"uuid": "92dcd532-84ad-4a7c-9ec4-fbc7de2ab9a4", "flow": "xtls-rprx-vision"},
		},
		"reality-config": map[string]interface{}{
			"private-key":  "cPtBoOI-_24y-pmPtQoiAG0Cv6oniTsjp9F9EosDkFM",
			"server-names": []interface{}{"www.microsoft.com"},
			"short-id":     []interface{}{"b5929842b77b4c02"},
		},
	}
	uris, err := vlessURIs("节点1", "85.149.212.214", "55523", cfg)
	if err != nil {
		t.Fatal(err)
	}
	u := uris[0]
	if !strings.Contains(u, "security=reality") {
		t.Fatalf("want reality: %s", u)
	}
	if strings.Contains(u, "insecure=") || strings.Contains(u, "allowInsecure=") {
		t.Fatalf("reality URI must not carry insecure flags: %s", u)
	}
	if !strings.Contains(u, "sni=www.microsoft.com") {
		t.Fatalf("want sni: %s", u)
	}
}

func TestVmessURIHasNameAndBase64JSON(t *testing.T) {
	cfg := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"uuid": "92dcd532-84ad-4a7c-9ec4-fbc7de2ab9a4"},
		},
	}
	uris, err := vmessURIs("my-vmess", "1.2.3.4", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.TrimPrefix(uris[0], "vmess://")
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]string
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["ps"] != "my-vmess" {
		t.Fatalf("ps: %v", obj["ps"])
	}
	if obj["add"] != "1.2.3.4" {
		t.Fatalf("add: %v", obj["add"])
	}
}
