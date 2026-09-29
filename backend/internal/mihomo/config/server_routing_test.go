package config

import (
	"testing"
)

func TestApplyServerRoutingDefault(t *testing.T) {
	merged := map[string]interface{}{}
	applyServerRouting(merged, DefaultServerRouting(), nil)
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
	applyServerRouting(merged, sr, nil)
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

func TestApplyServerRoutingWarpDomains(t *testing.T) {
	merged := map[string]interface{}{}
	sr := ServerRoutingConfig{
		Rules:       []string{"MATCH,DIRECT"},
		WarpDomains: []string{"openai.com", "GEOSITE:google", ".chatgpt.com"},
	}
	warp := map[string]interface{}{
		"name": "WARP", "type": "wireguard", "server": "engage.cloudflareclient.com",
	}
	applyServerRouting(merged, sr, warp)
	proxies, _ := merged["proxies"].([]interface{})
	if len(proxies) != 1 {
		t.Fatalf("proxies: %#v", proxies)
	}
	rules, _ := merged["rules"].([]interface{})
	// 3 domain rules + MATCH
	if len(rules) < 4 {
		t.Fatalf("rules: %#v", rules)
	}
	if rules[0] != "DOMAIN-SUFFIX,openai.com,WARP" {
		t.Fatalf("rule0: %v", rules[0])
	}
	if rules[1] != "GEOSITE,google,WARP" {
		t.Fatalf("rule1: %v", rules[1])
	}
	if rules[2] != "DOMAIN-SUFFIX,chatgpt.com,WARP" {
		t.Fatalf("rule2: %v", rules[2])
	}
}
