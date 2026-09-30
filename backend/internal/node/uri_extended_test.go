package node

import (
	"strings"
	"testing"
)

func TestSudokuURIsSerializeNestedHTTPMaskAndCustomTables(t *testing.T) {
	cfg := map[string]interface{}{
		"key":           "server-key",
		"aead-method":   "chacha20-poly1305",
		"custom-tables": []interface{}{"xpxvvpvv", "vxpvxvvp"},
		"httpmask":      map[string]interface{}{"disable": true, "mode": "stream", "path-root": "aabbcc"},
	}
	uris, err := sudokuURIs("node", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != 1 {
		t.Fatalf("expected one URI, got %d", len(uris))
	}
	for _, want := range []string{
		"key=server-key",
		"custom-tables=xpxvvpvv%2Cvxpvxvvp",
		"httpmask-disable=1",
		"httpmask-mode=stream",
		"httpmask-path-root=aabbcc",
	} {
		if !strings.Contains(uris[0], want) {
			t.Fatalf("URI missing %q: %s", want, uris[0])
		}
	}
}
