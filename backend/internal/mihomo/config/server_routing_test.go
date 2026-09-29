package config

import (
	"testing"
)

func TestApplyServerRoutingDefault(t *testing.T) {
	merged := map[string]interface{}{}
	applyServerRouting(merged, DefaultServerRouting())
	rules, _ := merged["rules"].([]interface{})
	if len(rules) != 1 || rules[0] != "MATCH,DIRECT" {
		t.Fatalf("rules: %#v", rules)
	}
}

func TestApplyServerRoutingCustom(t *testing.T) {
	merged := map[string]interface{}{}
	sr := ServerRoutingConfig{
		Rules: []string{"GEOIP,private,DIRECT", "DOMAIN-SUFFIX,google.com,WARP"},
		Proxies: []ProxyEntry{
			{Name: "WARP", Type: "wireguard", Server: "engage.cloudflareclient.com", Port: 2408},
		},
	}
	applyServerRouting(merged, sr)
	rules, _ := merged["rules"].([]interface{})
	if len(rules) != 3 { // + MATCH,DIRECT auto
		t.Fatalf("expected MATCH appended, got %#v", rules)
	}
	last := rules[len(rules)-1].(string)
	if last != "MATCH,DIRECT" {
		t.Fatalf("last rule: %s", last)
	}
	proxies, _ := merged["proxies"].([]interface{})
	if len(proxies) != 1 {
		t.Fatalf("proxies: %#v", proxies)
	}
}

func TestNormalizeServerRoutingEmpty(t *testing.T) {
	sr := ServerRoutingConfig{}
	normalizeServerRouting(&sr)
	if len(sr.Rules) != 1 || sr.Rules[0] != "MATCH,DIRECT" {
		t.Fatalf("%#v", sr.Rules)
	}
}
