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
	if first["auto_route"] != true {
		t.Fatalf("auto_route: %#v", first["auto_route"])
	}
	route, ok := parsed["route"].(map[string]interface{})
	if !ok || route["final"] != "proxy" {
		t.Fatalf("route.final: %#v", route)
	}
	obs, ok := parsed["outbounds"].([]interface{})
	if !ok || len(obs) != 3 {
		t.Fatalf("outbounds: %#v", obs)
	}
}
