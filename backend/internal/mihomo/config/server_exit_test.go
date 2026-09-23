package config

import "testing"

func TestServerExitRules(t *testing.T) {
	r := serverExitRules(nil, "WARP-OUT")
	if len(r) != 1 || r[0] != "MATCH,WARP-OUT" {
		t.Fatalf("got %#v", r)
	}
	r = serverExitRules([]string{"GEOIP,CN,DIRECT", "MATCH,WARP-OUT"}, "WARP-OUT")
	if len(r) != 2 || r[1] != "MATCH,WARP-OUT" {
		t.Fatalf("cn mode %#v", r)
	}
}

func TestIsServerExitProxy(t *testing.T) {
	if !isServerExitProxy(ProxyEntry{Name: "WARP-OUT", Type: "wireguard"}) {
		t.Fatal("expected wireguard")
	}
	if isServerExitProxy(ProxyEntry{Name: "ss1", Type: "ss"}) {
		t.Fatal("ss should not be server exit")
	}
}
