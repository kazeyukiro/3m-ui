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
	if strings.ContainsAny(share.URI, "+/") {
		t.Fatalf("URI contains non-URL-safe base64 characters: %s", share.URI)
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
