package config

import (
	"fmt"
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
	if len(rules) < 4 {
		t.Fatalf("rules: %#v", rules)
	}
	joined := fmt.Sprintf("%v", rules)
	if !containsRule(rules, "DOMAIN-SUFFIX,openai.com,WARP") {
		t.Fatalf("missing openai suffix: %s", joined)
	}
	if !containsRule(rules, "GEOSITE,google,WARP") {
		t.Fatalf("missing geosite: %s", joined)
	}
	if !containsRule(rules, "DOMAIN-SUFFIX,chatgpt.com,WARP") {
		t.Fatalf("missing chatgpt: %s", joined)
	}
	if merged["sniffer"] == nil {
		t.Fatalf("sniffer should be enabled for WARP domains")
	}
	dns, _ := merged["dns"].(map[string]interface{})
	if dns == nil || dns["enable"] != true {
		t.Fatalf("dns: %#v", dns)
	}
}

func TestApplyRuleProviders(t *testing.T) {
	merged := map[string]interface{}{}
	sr := ServerRoutingConfig{
		Rules: []string{"RULE-SET,gfw,DIRECT", "MATCH,DIRECT"},
		RuleProviders: []RuleProvider{
			{Name: "gfw", Type: "http", Behavior: "domain", Format: "mrs", URL: "https://example.com/gfw.mrs"},
		},
	}
	applyServerRouting(merged, sr, nil)
	rp, ok := merged["rule-providers"].(map[string]interface{})
	if !ok || rp["gfw"] == nil {
		t.Fatalf("rule-providers: %#v", merged["rule-providers"])
	}
	entry := rp["gfw"].(map[string]interface{})
	if entry["type"] != "http" || entry["behavior"] != "domain" {
		t.Fatalf("entry: %#v", entry)
	}
	if entry["path"] == nil || entry["url"] != "https://example.com/gfw.mrs" {
		t.Fatalf("path/url: %#v", entry)
	}
}

func containsRule(rules []interface{}, want string) bool {
	for _, r := range rules {
		if s, ok := r.(string); ok && s == want {
			return true
		}
	}
	return false
}
