package netutil

import "testing"

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		"example.com":             "example.com",
		"http://example.com:2053": "example.com",
		"[2001:db8::1]":           "2001:db8::1",
		"[2001:db8::1]:443":       "2001:db8::1",
		"2001:db8::1":             "2001:db8::1",
		"127.0.0.1:8080":          "127.0.0.1",
		"::1":                     "::1",
		"[::1]:8080":              "::1",
	}
	for in, want := range cases {
		if got := NormalizeHost(in); got != want {
			t.Fatalf("NormalizeHost(%q)=%q want %q", in, got, want)
		}
	}
}

func TestJoinHostPortIPv6(t *testing.T) {
	got := JoinHostPort("2001:db8::1", "443")
	if got != "[2001:db8::1]:443" {
		t.Fatalf("got %q", got)
	}
	got = JoinHostPort("[2001:db8::1]", "443")
	if got != "[2001:db8::1]:443" {
		t.Fatalf("got %q", got)
	}
}

func TestIsIPv6(t *testing.T) {
	if !IsIPv6("2001:db8::1") || IsIPv6("1.2.3.4") || IsIPv6("example.com") {
		t.Fatal("IsIPv6 mismatch")
	}
}
