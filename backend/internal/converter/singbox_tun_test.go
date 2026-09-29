package converter

import (
	"encoding/json"
	"testing"
)

func TestBuildSingboxSubscriptionDocIncludesTUN(t *testing.T) {
	outbounds := []map[string]interface{}{
		{"type": "direct", "tag": "direct"},
		{"type": "selector", "tag": "proxy", "outbounds": []string{"n1"}, "default": "n1"},
		{"type": "vless", "tag": "n1", "server": "example.com", "server_port": 443},
	}
	doc := buildSingboxSubscriptionDoc(outbounds)
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	inbounds, ok := parsed["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		t.Fatalf("missing inbounds: %s", raw)
	}
	first, _ := inbounds[0].(map[string]interface{})
	if first["type"] != "tun" {
		t.Fatalf("first inbound not tun: %#v", first)
	}
	if _, has := first["stack"]; has {
		t.Fatalf("stack must be omitted (client default): %#v", first)
	}
	if first["interface_name"] != "singbox_tun" {
		t.Fatalf("interface_name: %#v", first["interface_name"])
	}
	if first["dns_mode"] != "hijack" {
		t.Fatalf("dns_mode: %#v", first["dns_mode"])
	}
	dns, ok := parsed["dns"].(map[string]interface{})
	if !ok || dns["final"] != "remote" {
		t.Fatalf("dns: %#v", parsed["dns"])
	}
	// 1.14+: DNS rules must not use legacy address filters without match_response.
	if rules, ok := dns["rules"].([]interface{}); ok {
		for _, r := range rules {
			m, _ := r.(map[string]interface{})
			if _, has := m["ip_is_private"]; has {
				if match, _ := m["match_response"].(bool); !match {
					t.Fatalf("deprecated DNS ip_is_private without match_response: %#v", m)
				}
			}
		}
	}
	route, ok := parsed["route"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing route")
	}
	if _, ok := route["default_domain_resolver"]; !ok {
		t.Fatalf("missing default_domain_resolver: %#v", route)
	}
	if first["auto_route"] != true {
		t.Fatalf("auto_route: %#v", first["auto_route"])
	}
	if _, has := first["sniff"]; has {
		t.Fatalf("legacy inbound sniff must not be set (removed in sing-box 1.13): %#v", first)
	}
	if route["final"] != "proxy" {
		t.Fatalf("route.final: %#v", route)
	}
	rules, ok := route["rules"].([]interface{})
	if !ok || len(rules) < 1 {
		t.Fatalf("route.rules missing sniff/hijack-dns: %#v", route)
	}
	foundSniff := false
	for _, r := range rules {
		m, _ := r.(map[string]interface{})
		if m["action"] == "sniff" {
			foundSniff = true
		}
	}
	if !foundSniff {
		t.Fatalf("route.rules should include action=sniff: %#v", rules)
	}
	obs, ok := parsed["outbounds"].([]interface{})
	if !ok || len(obs) != 3 {
		t.Fatalf("outbounds: %#v", obs)
	}
}
