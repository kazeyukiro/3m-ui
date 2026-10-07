package config

import "testing"

func TestMihomoListenerProtocolAllowlist(t *testing.T) {
	for _, protocol := range MihomoListenerProtocols {
		if !IsMihomoListenerProtocol(protocol) {
			t.Fatalf("protocol %q was not accepted", protocol)
		}
	}
	for _, protocol := range []string{"tproxy", "redir", "mixed", "tunnel", "tun", "wireguard"} {
		if IsMihomoListenerProtocol(protocol) {
			t.Fatalf("protocol %q must not be exposed as a distributable listener", protocol)
		}
	}
	for _, protocol := range []string{"http", "socks"} {
		if !IsMihomoListenerProtocol(protocol) {
			t.Fatalf("protocol %q must be exposed (wiki inbound listeners)", protocol)
		}
	}
}
