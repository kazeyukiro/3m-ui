package cluster

import "testing"

func TestNormalizeBaseURL(t *testing.T) {
	u, err := normalizeBaseURL("https://panel.example.com:2053/")
	if err != nil || u != "https://panel.example.com:2053" {
		t.Fatalf("got %q %v", u, err)
	}
	if _, err := normalizeBaseURL("ftp://x"); err == nil {
		t.Fatal("expected scheme error")
	}
	u, err = normalizeBaseURL("panel.local:8080")
	if err != nil || u != "http://panel.local:8080" {
		t.Fatalf("got %q %v", u, err)
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
