package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestProxyEntrySerializesTFOAndMPTCP proves that the proxy-level `tfo` /
// `mptcp` options (Mihomo's TCP Fast Open / Multipath TCP, see
// wiki.metacubex.one/config/proxies) survive the round trip through the
// visual config store and end up as top-level keys in the generated
// proxy YAML exactly as Mihomo expects.
func TestProxyEntrySerializesTFOAndMPTCP(t *testing.T) {
	in := ProxyEntry{
		Name:    "NODE-A",
		Type:    "ss",
		Server:  "example.com",
		Port:    8388,
		Options: map[string]interface{}{"tfo": true, "mptcp": true},
	}
	raw, err := yaml.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "tfo: true") {
		t.Fatalf("missing tfo in marshalled proxy:\n%s", s)
	}
	if !strings.Contains(s, "mptcp: true") {
		t.Fatalf("missing mptcp in marshalled proxy:\n%s", s)
	}

	var out ProxyEntry
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Options["tfo"] != true {
		t.Fatalf("tfo not preserved: %v", out.Options)
	}
	if out.Options["mptcp"] != true {
		t.Fatalf("mptcp not preserved: %v", out.Options)
	}
}

// TestProxyEntryDropsTFOMPTCPWhenAbsent ensures a proxy without these flags
// does not emit a `tfo: false` / `mptcp: false` line (so the values stay
// unset and Mihomo uses stock behaviour) and that an explicit false is kept.
func TestProxyEntryOmitsTFOMPTCPWhenAbsent(t *testing.T) {
	in := ProxyEntry{Name: "NODE-B", Type: "ss", Server: "x", Port: 1}
	raw, err := yaml.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "tfo") {
		t.Fatalf("tfo should be absent when unset:\n%s", s)
	}
	if strings.Contains(s, "mptcp") {
		t.Fatalf("mptcp should be absent when unset:\n%s", s)
	}

	explicit := ProxyEntry{
		Name:    "NODE-C",
		Type:    "ss",
		Server:  "x",
		Port:    1,
		Options: map[string]interface{}{"tfo": false, "mptcp": false},
	}
	raw2, _ := yaml.Marshal(explicit)
	if !strings.Contains(string(raw2), "tfo: false") {
		t.Fatalf("explicit tfo:false should be serialised:\n%s", string(raw2))
	}
}
