package config

import "testing"

func TestAppendRulePreferLastMatch(t *testing.T) {
	got := appendRulePreferLastMatch([]string{"GEOIP,CN,DIRECT", "MATCH,DIRECT"}, "MATCH,WARP-OUT")
	if len(got) != 2 || got[1] != "MATCH,WARP-OUT" {
		t.Fatalf("got %#v", got)
	}
	got = appendRulePreferLastMatch([]string{}, "MATCH,WARP-OUT")
	if len(got) != 1 || got[0] != "MATCH,WARP-OUT" {
		t.Fatalf("empty %#v", got)
	}
}
