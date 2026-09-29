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

func TestWarpDomainRule(t *testing.T) {
	cases := []struct{ in, want string }{
		{"openai.com", "DOMAIN-SUFFIX,openai.com,WARP"},
		{"domain:openai.com", "DOMAIN-SUFFIX,openai.com,WARP"},
		{"full:api.openai.com", "DOMAIN,api.openai.com,WARP"},
		{"keyword:openai", "DOMAIN-KEYWORD,openai,WARP"},
		{"geosite:openai", "GEOSITE,openai,WARP"},
		{"GEOSITE:google", "GEOSITE,google,WARP"},
	}
	for _, c := range cases {
		got := warpDomainRule(c.in, "WARP")
		if got != c.want {
			t.Fatalf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestApplyServerRoutingWarpGlobal(t *testing.T) {
	merged := map[string]interface{}{}
	sr := ServerRoutingConfig{Rules: []string{"MATCH,DIRECT"}, WarpGlobal: true}
	warp := map[string]interface{}{"name": "WARP", "type": "wireguard"}
	applyServerRouting(merged, sr, warp)
	rules, _ := merged["rules"].([]interface{})
	last := rules[len(rules)-1].(string)
	if last != "MATCH,WARP" {
		t.Fatalf("last=%s rules=%v", last, rules)
	}
	if merged["sniffer"] == nil {
		t.Fatal("expected sniffer")
	}
}

func TestApplyServerRoutingWarpDomains(t *testing.T) {
	merged := map[string]interface{}{}
	sr := ServerRoutingConfig{
		Rules:       []string{"MATCH,DIRECT"},
		WarpDomains: []string{"openai.com", "geosite:google", "full:api.openai.com"},
	}
	warp := map[string]interface{}{"name": "WARP", "type": "wireguard"}
	applyServerRouting(merged, sr, warp)
	rules, _ := merged["rules"].([]interface{})
	joined := fmt.Sprintf("%v", rules)
	for _, want := range []string{
		"DOMAIN-SUFFIX,openai.com,WARP",
		"GEOSITE,google,WARP",
		"DOMAIN,api.openai.com,WARP",
	} {
		if !containsRule(rules, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
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
}

func containsRule(rules []interface{}, want string) bool {
	for _, r := range rules {
		if s, ok := r.(string); ok && s == want {
			return true
		}
	}
	return false
}
