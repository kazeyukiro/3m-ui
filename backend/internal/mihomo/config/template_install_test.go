package config

import "testing"

func TestFreshInstallDoesNotRequireGeoDownloads(t *testing.T) {
	t.Setenv("THREE_M_UI_MIHOMO_CONTROLLER", "127.0.0.1:19090")
	base := GetDefaultTemplate()
	if base.ExternalController != "127.0.0.1:19090" {
		t.Fatal("controller override not applied")
	}
	for _, rules := range [][]string{base.Rules, DefaultVisualConfig().Rules} {
		if len(rules) != 1 || rules[0] != "MATCH,DIRECT" {
			t.Fatal("fresh direct routing should not require external geodata")
		}
	}
}
