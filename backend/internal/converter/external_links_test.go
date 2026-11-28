package converter

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestMergeExternalSubscriptionLinksSSRFProtection ensures the external-sub
// fetcher uses the SSRF-safe HTTP client: a loopback target (127.0.0.1) must
// NOT be reached when private targets are disallowed, and IS allowed when the
// lab override is enabled. This guards against re-introducing a plain
// http.Client here (see audit finding M1).
func TestMergeExternalSubscriptionLinksSSRFProtection(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "text/yaml")
		// Minimal valid Clash/Mihomo doc with one proxy.
		_, _ = w.Write([]byte("proxies:\n  - name: ext-1\n    type: ss\n    server: 1.2.3.4\n    port: 8388\n    cipher: aes-256-gcm\n    password: x\n"))
	}))
	defer srv.Close()

	t.Run("blocks_loopback_by_default", func(t *testing.T) {
		t.Setenv("THREE_M_UI_ALLOW_PRIVATE", "0")
		t.Setenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE", "0")
		var before int32 = atomic.LoadInt32(&hits)
		got, _ := mergeExternalSubscriptionLinks(srv.URL, nil, nil)
		if atomic.LoadInt32(&hits) != before {
			t.Fatal("loopback target must NOT be reached in normal mode")
		}
		if len(got) != 0 {
			t.Fatalf("expected no merged proxies, got %d", len(got))
		}
	})

	t.Run("allows_loopback_in_lab_mode", func(t *testing.T) {
		t.Setenv("THREE_M_UI_ALLOW_PRIVATE", "1")
		var before int32 = atomic.LoadInt32(&hits)
		got, names := mergeExternalSubscriptionLinks(srv.URL, nil, nil)
		if atomic.LoadInt32(&hits) == before {
			t.Fatal("loopback target should be reached in lab mode")
		}
		if len(got) != 1 || len(names) != 1 {
			t.Fatalf("expected 1 merged proxy, got proxies=%d names=%d", len(got), len(names))
		}
	})
}

// TestMergeExternalSubscriptionLinksSkipsNonHTTP ensures non-http(s) lines and
// comments are ignored, and that malformed URLs do not panic.
func TestMergeExternalSubscriptionLinksSkipsNonHTTP(t *testing.T) {
	t.Setenv("THREE_M_UI_ALLOW_PRIVATE", "0")
	got, names := mergeExternalSubscriptionLinks("# comment\n\nftp://x\nnot-a-url\n", nil, nil)
	if len(got) != 0 || len(names) != 0 {
		t.Fatalf("expected no proxies, got %d / %d", len(got), len(names))
	}
}
