package protocol

import (
	"testing"
)

func TestListenerTransportsUDPKinds(t *testing.T) {
	for _, kind := range []string{"tuic", "tuic-v4", "tuic-v5", "hysteria", "hysteria2", "shadowquic", "mieru"} {
		got := ListenerTransports(kind)
		if len(got) != 1 || got[0] != "udp" {
			t.Fatalf("ListenerTransports(%q) = %v, want [udp]", kind, got)
		}
	}
}

func TestListenerTransportsTCPKinds(t *testing.T) {
	for _, kind := range []string{"vless", "vmess", "trojan", "shadowsocks", "snell", "anytls", "sudoku", "trusttunnel", "socks", "http"} {
		got := ListenerTransports(kind)
		if len(got) != 1 || got[0] != "tcp" {
			t.Fatalf("ListenerTransports(%q) = %v, want [tcp]", kind, got)
		}
	}
}

func TestListenerTransportsCaseInsensitive(t *testing.T) {
	if got := ListenerTransports("  TUIC-V4 "); got[0] != "udp" {
		t.Fatalf("ListenerTransports(uppercase) = %v, want [udp]", got)
	}
	if got := ListenerTransports(""); got[0] != "tcp" {
		t.Fatalf("ListenerTransports(empty) = %v, want [tcp]", got)
	}
}
