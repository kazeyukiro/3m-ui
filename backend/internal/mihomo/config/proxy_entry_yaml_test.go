package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProxyEntryYAMLRoundTripKeepsPrivateKey(t *testing.T) {
	in := ProxyEntry{
		Name:   "WG-OUT",
		Type:   "wireguard",
		Server: "example.com",
		Port:   2408,
		Options: map[string]interface{}{
			"private-key": "eCtXsJZ27+4PbhDkHnB923tkUn2Gj59wZw5wFA75MnU=",
			"public-key":  "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
			"ip":          "172.16.0.2",
			"udp":         true,
		},
	}
	raw, err := yaml.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "private-key:") || !strings.Contains(s, "type: wireguard") {
		t.Fatalf("marshal lost fields:\n%s", s)
	}
	var out ProxyEntry
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Type != "wireguard" {
		t.Fatalf("type=%q", out.Type)
	}
	pk, _ := out.Options["private-key"].(string)
	if pk != "eCtXsJZ27+4PbhDkHnB923tkUn2Gj59wZw5wFA75MnU=" {
		t.Fatalf("private-key=%q opts=%v", pk, out.Options)
	}
}

func TestValidateWireGuardPrivateKey(t *testing.T) {
	if err := validateWireGuardPrivateKey("eCtXsJZ27+4PbhDkHnB923tkUn2Gj59wZw5wFA75MnU="); err != nil {
		t.Fatal(err)
	}
	if err := validateWireGuardPrivateKey("not-a-key"); err == nil {
		t.Fatal("expected error")
	}
}
