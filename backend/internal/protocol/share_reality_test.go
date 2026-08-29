package protocol

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestVLESSShareRealityPublicKeyIsRawURLSafeEverywhere(t *testing.T) {
	const privateKey = "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0"
	const want = "dUMdExLMSn4l_p_bWpfFC5DQHaDHrjKanEQPG6Xl4hw"

	node := NodeModel{
		Name:       "vless-reality",
		Protocol:   "vless",
		PublicHost: "example.com",
		Port:       "443",
		Enabled:    true,
		VLESS: &VLESSSpec{
			Flow: "xtls-rprx-vision",
			Reality: &RealitySpec{
				PrivateKey: privateKey,
				ShortID:    "0123456789abcdef",
				ServerName: "example.com",
			},
		},
	}

	share, err := (VLESSCompiler{}).BuildShare(ShareInput{
		Node: node,
		User: UserCred{UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(share.URI, "pbk="+want) {
		t.Fatalf("URI does not contain expected URL-safe public key: %s", share.URI)
	}
	if strings.Contains(share.URI, "pbk="+want+"+") || strings.Contains(share.URI, "pbk="+want+"/") {
		t.Fatalf("URI public-key parameter contains non-URL-safe characters: %s", share.URI)
	}
	if !strings.Contains(share.ClientYAML, "public-key: "+want) {
		t.Fatalf("client YAML does not contain expected URL-safe public key: %s", share.ClientYAML)
	}
	if strings.ContainsAny(want, "+/=") {
		t.Fatal("test public key is not URL-safe base64")
	}
	if raw, err := base64.RawURLEncoding.DecodeString(want); err != nil || len(raw) != 32 {
		t.Fatalf("expected a valid 32-byte raw URL-safe public key: %v", err)
	}
}

func TestRealityShareRejectsMismatchedPublicAndPrivateKeys(t *testing.T) {
	node := NodeModel{
		Name:       "vless-reality-mismatch",
		Protocol:   "vless",
		PublicHost: "example.com",
		Port:       "443",
		Enabled:    true,
		VLESS: &VLESSSpec{
			Reality: &RealitySpec{
				PrivateKey: "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
				PublicKey:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			},
		},
	}

	_, err := (VLESSCompiler{}).BuildShare(ShareInput{
		Node: node,
		User: UserCred{UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match private key") {
		t.Fatalf("expected Reality key mismatch error, got %v", err)
	}
}

func TestRealityShareNormalizesStandardBase64PublicKey(t *testing.T) {
	const privateKey = "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0"
	const wantURL = "dUMdExLMSn4l_p_bWpfFC5DQHaDHrjKanEQPG6Xl4hw"

	raw, err := base64.RawURLEncoding.DecodeString(wantURL)
	if err != nil {
		t.Fatal(err)
	}
	standard := base64.RawStdEncoding.EncodeToString(raw)

	got, err := realityPublicKeyFromSpec(&RealitySpec{PrivateKey: privateKey, PublicKey: standard})
	if err != nil {
		t.Fatal(err)
	}
	if got != wantURL {
		t.Fatalf("expected normalized URL-safe public key %q, got %q", wantURL, got)
	}
}

func TestShareClientYAMLReflectsNodeUDP(t *testing.T) {
	baseNode := NodeModel{
		Name:       "vless-tcp",
		Protocol:   "vless",
		PublicHost: "example.com",
		Port:       "443",
		Enabled:    true,
		VLESS:      &VLESSSpec{},
	}

	share, err := (VLESSCompiler{}).BuildShare(ShareInput{
		Node: baseNode,
		User: UserCred{UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(share.ClientYAML, "udp: true") {
		t.Fatalf("TCP-only node must not export udp: true: %s", share.ClientYAML)
	}

	baseNode.UDP = true
	share, err = (VLESSCompiler{}).BuildShare(ShareInput{
		Node: baseNode,
		User: UserCred{UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(share.ClientYAML, "udp: true") {
		t.Fatalf("UDP-enabled node must export udp: true: %s", share.ClientYAML)
	}
}

func TestVLESSSharePreservesALPNInURIAndYAML(t *testing.T) {
	node := NodeModel{
		Name:       "vless-alpn",
		Protocol:   "vless",
		PublicHost: "example.com",
		Port:       "443",
		Enabled:    true,
		VLESS: &VLESSSpec{
			SNI:  "example.com",
			ALPN: []string{"h2", "http/1.1"},
		},
	}
	share, err := (VLESSCompiler{}).BuildShare(ShareInput{
		Node: node,
		User: UserCred{UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(share.URI, "alpn=h2%2Chttp%2F1.1") {
		t.Fatalf("URI lost ALPN: %s", share.URI)
	}
	if !strings.Contains(share.ClientYAML, "alpn:") || !strings.Contains(share.ClientYAML, "- h2") || !strings.Contains(share.ClientYAML, "- http/1.1") {
		t.Fatalf("client YAML lost ALPN: %s", share.ClientYAML)
	}
}

func TestHysteria2SharePreservesALPN(t *testing.T) {
	node := NodeModel{
		Name:       "hy2-alpn",
		Protocol:   "hysteria2",
		PublicHost: "example.com",
		Port:       "443",
		Enabled:    true,
		Hysteria2: &Hysteria2Spec{
			SNI:  "example.com",
			ALPN: []string{"h3"},
		},
	}
	share, err := (Hysteria2Compiler{}).BuildShare(ShareInput{
		Node: node,
		User: UserCred{Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(share.URI, "alpn=h3") {
		t.Fatalf("Hysteria2 URI lost ALPN: %s", share.URI)
	}
}
