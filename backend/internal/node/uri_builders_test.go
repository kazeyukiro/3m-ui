package node

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRealityPublicKeyUsesRawURLEncoding(t *testing.T) {
	cfg := map[string]interface{}{
		"private-key": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
	}

	got, err := realityPublicKey(cfg)
	if err != nil {
		t.Fatal(err)
	}

	const want = "dUMdExLMSn4l_p_bWpfFC5DQHaDHrjKanEQPG6Xl4hw"
	if got != want {
		t.Fatalf("public key = %q, want %q", got, want)
	}
	if strings.ContainsAny(got, "+/=") {
		t.Fatalf("public key is not raw URL-safe base64: %q", got)
	}
	if raw, err := base64.RawURLEncoding.DecodeString(got); err != nil || len(raw) != 32 {
		t.Fatalf("public key is not a valid 32-byte raw URL-safe base64 value: %q", got)
	}
}
