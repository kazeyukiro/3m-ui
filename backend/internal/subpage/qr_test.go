package subpage

import (
	"strings"
	"testing"
)

func TestLocalSubQRDataURI_NotURLEscaped(t *testing.T) {
	u := localSubQRDataURI("https://example.com/api/v1/client/sub/abc123")
	s := string(u)
	if s == "" {
		t.Fatal("empty QR data URI")
	}
	if !strings.HasPrefix(s, "data:image/png;base64,") {
		t.Fatalf("prefix: %s", s[:min(40, len(s))])
	}
	// html/template would turn + into %2B in URL context; raw base64 must keep +/
	payload := strings.TrimPrefix(s, "data:image/png;base64,")
	if strings.Contains(payload, "%") {
		t.Fatalf("data URI looks percent-encoded: %s", payload[:min(80, len(payload))])
	}
	if len(payload) < 100 {
		t.Fatalf("payload too short: %d", len(payload))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
