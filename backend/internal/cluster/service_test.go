package cluster

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

func TestNormalizeBaseURL(t *testing.T) {
	u, err := normalizeBaseURL("https://panel.example.com:2053/")
	if err != nil || u != "https://panel.example.com:2053" {
		t.Fatalf("got %q %v", u, err)
	}
	if _, err := normalizeBaseURL("ftp://x"); err == nil {
		t.Fatal("expected scheme error")
	}
	u, err = normalizeBaseURL("panel.example.com:8080")
	if err != nil || u != "http://panel.example.com:8080" {
		t.Fatalf("got %q %v", u, err)
	}
	_ = os.Unsetenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE")
	if _, err := normalizeBaseURL("http://127.0.0.1:8080"); err == nil {
		t.Fatal("expected private/loopback rejection")
	}
	if _, err := normalizeBaseURL("http://10.0.0.1:8080"); err == nil {
		t.Fatal("expected private IP rejection")
	}
	if _, err := normalizeBaseURL("http://user:pass@panel.example.com:8080"); err == nil {
		t.Fatal("expected URL userinfo rejection")
	}
	t.Setenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE", "1")
	if _, err := normalizeBaseURL("http://127.0.0.1:8080"); err != nil {
		t.Fatalf("lab allow should pass: %v", err)
	}
	// Metadata endpoints remain blocked even when private targets are enabled.
	if err := assertClusterIPAllowed(net.ParseIP("169.254.169.254")); err == nil {
		t.Fatal("metadata endpoint must remain blocked")
	}
}

func TestSanitizeProxyPath(t *testing.T) {
	if _, err := sanitizeProxyPath("/api/v1/users"); err != nil {
		t.Fatal(err)
	}
	if _, err := sanitizeProxyPath("/api/v1/nodes/1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sanitizeProxyPath("/etc/passwd"); err == nil {
		t.Fatal("should reject")
	}
	if _, err := sanitizeProxyPath("/api/v1/../etc"); err == nil {
		t.Fatal("should reject traversal")
	}
	if _, err := sanitizeProxyPath("https://evil"); err == nil {
		t.Fatal("should reject absolute")
	}
}

func TestSafeClusterDialContextBlocksPrivateIP(t *testing.T) {
	t.Setenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE", "0")
	dial := safeClusterDialContext(&net.Dialer{Timeout: 100 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := dial(ctx, "tcp", "127.0.0.1:1"); err == nil {
		t.Fatal("expected private IP to be blocked before dialing")
	}
}
