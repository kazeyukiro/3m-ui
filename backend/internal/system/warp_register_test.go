package system

import (
	"encoding/json"
	"testing"
)

// TestParseWARPResponse_TopLevel verifies that the parser handles the
// CURRENT Cloudflare WARP API response shape where the device object is
// returned at the top level (no "result" wrapper).
func TestParseWARPResponse_TopLevel(t *testing.T) {
	// Real response sample (anonymized) from api.cloudflareclient.com/v0a2158/reg
	raw := []byte(`{
		"id":"f0844485-ee81-4c51-a022-a86549ebb070",
		"type":"a","model":"3m-ui","name":"",
		"key":"Gwd0cOvVb21gM4pj4TPOUF6DDDe8f2E66sky2WCnBB0=",
		"account":{"id":"c6b98d14-eae1-49ee-996d-1fdaf08f2ed2","account_type":"free"},
		"config":{
			"client_id":"qVtt",
			"peers":[{"public_key":"bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
				"endpoint":{"v4":"162.159.192.7:0","v6":"[2606:4700:d0::a29f:c007]:0","host":"engage.cloudflareclient.com:2408","ports":[2408,500,1701,4500]}}],
			"interface":{"addresses":{"v4":"172.16.0.2","v6":"2606:4700:110:8bd9:78d8:124c:4903:6618"}},
			"services":{"http_proxy":"172.16.0.1:2480"}
		},
		"token":"3cfe20e1-03d9-4fff-a14f-1638bcef29b6"
	}`)
	var parsed cfRegResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	cfg := parsed.Result.Config
	if parsed.Result.ID == "" {
		cfg = parsed.Config
	}
	v4 := cfg.Interface.Addresses.V4
	v6 := cfg.Interface.Addresses.V6
	if v4 != "172.16.0.2" {
		t.Errorf("v4 = %q, want 172.16.0.2", v4)
	}
	if v6 != "2606:4700:110:8bd9:78d8:124c:4903:6618" {
		t.Errorf("v6 = %q, want 2606:4700:110:...", v6)
	}
	if cfg.ClientID != "qVtt" {
		t.Errorf("client_id = %q, want qVtt", cfg.ClientID)
	}
}

// TestParseWARPResponse_LegacyWrapper verifies that the parser still handles
// the LEGACY Cloudflare WARP API response shape where the device object is
// wrapped in {"result": {...}}.
func TestParseWARPResponse_LegacyWrapper(t *testing.T) {
	raw := []byte(`{
		"result":{
			"id":"abc-123","type":"a","model":"3m-ui",
			"config":{
				"client_id":"Ql88",
				"peers":[{"public_key":"bmXOC+...","endpoint":{"v4":"162.159.192.4:0","v6":"[2606:4700:d0::a29f:c004]:0"}}],
				"interface":{"addresses":{"v4":"172.16.0.2","v6":"2606:4700:110:8c22:6351:7113:8bbe:ca05"}}
			},
			"token":"legacy-token-xyz"
		}
	}`)
	var parsed cfRegResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	cfg := parsed.Result.Config
	if parsed.Result.ID == "" {
		cfg = parsed.Config
	}
	v4 := cfg.Interface.Addresses.V4
	v6 := cfg.Interface.Addresses.V6
	if v4 != "172.16.0.2" {
		t.Errorf("v4 = %q, want 172.16.0.2", v4)
	}
	if v6 != "2606:4700:110:8c22:6351:7113:8bbe:ca05" {
		t.Errorf("v6 = %q, want 2606:...", v6)
	}
	if cfg.ClientID != "Ql88" {
		t.Errorf("client_id = %q, want Ql88", cfg.ClientID)
	}
}

// TestParseWARPResponse_EmptyConfig simulates the error path where Cloudflare
// returns 200 but with empty/missing addresses (e.g. risk-controlled response).
// The parser should leave v4/v6 empty so RegisterWARP can return a clear error.
func TestParseWARPResponse_EmptyConfig(t *testing.T) {
	raw := []byte(`{
		"id":"x","type":"a","model":"3m-ui",
		"config":{"client_id":"","interface":{"addresses":{"v4":"","v6":""}}}
	}`)
	var parsed cfRegResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	cfg := parsed.Result.Config
	if parsed.Result.ID == "" {
		cfg = parsed.Config
	}
	if cfg.Interface.Addresses.V4 != "" || cfg.Interface.Addresses.V6 != "" {
		t.Errorf("expected empty addresses, got v4=%q v6=%q",
			cfg.Interface.Addresses.V4, cfg.Interface.Addresses.V6)
	}
}

// TestDecodeWARPClientID_Base64 verifies the common case: Cloudflare returns
// a base64-encoded 3-byte client_id (e.g. "qVtt" → [169, 91, 109]).
func TestDecodeWARPClientID_Base64(t *testing.T) {
	got := decodeWARPClientID("qVtt")
	want := []int{169, 91, 109}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("byte %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestDecodeWARPClientID_Numeric covers the rare case where Cloudflare returns
// a numeric client_id (e.g. "21").
func TestDecodeWARPClientID_Numeric(t *testing.T) {
	got := decodeWARPClientID("21")
	if len(got) != 1 || got[0] != 21 {
		t.Errorf("got %v, want [21]", got)
	}
}

// TestDecodeWARPClientID_Empty returns nil for empty input.
func TestDecodeWARPClientID_Empty(t *testing.T) {
	if got := decodeWARPClientID(""); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := decodeWARPClientID("   "); got != nil {
		t.Errorf("got %v, want nil for whitespace-only", got)
	}
}

// TestWARPTemplate_GeneratesCorrectYAML verifies that the generated YAML
// uses separate ip + ipv6 fields (NOT a comma-joined string) per
// https://wiki.metacubex.one/en/config/proxies/wg/
func TestWARPTemplate_GeneratesCorrectYAML(t *testing.T) {
	yaml, err := WARPTemplate(
		"CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo=",
		"172.16.0.2",
		"2606:4700:110:8216:dac5:4a83:49de:6997",
		[]int{21, 91, 109},
	)
	if err != nil {
		t.Fatalf("WARPTemplate: %v", err)
	}
	// Must have separate ip + ipv6 fields.
	if !contains(yaml, "ip: 172.16.0.2") {
		t.Errorf("missing/incorrect ip field:\n%s", yaml)
	}
	if !contains(yaml, "ipv6: 2606:4700:110:8216:dac5:4a83:49de:6997") {
		t.Errorf("missing/incorrect ipv6 field:\n%s", yaml)
	}
	// Must NOT have comma-joined form.
	if contains(yaml, "172.16.0.2,2606") {
		t.Errorf("found comma-joined ip (wrong), should be separate fields:\n%s", yaml)
	}
	// Reserved must be a 3-element list.
	if !contains(yaml, "reserved:") {
		t.Errorf("missing reserved field:\n%s", yaml)
	}
	if !contains(yaml, "- 21") || !contains(yaml, "- 91") || !contains(yaml, "- 109") {
		t.Errorf("reserved list missing expected bytes:\n%s", yaml)
	}
}

// TestWARPTemplate_OmitsReservedWhenEmpty verifies that reserved is omitted
// when nil/empty (some WARP+ accounts don't need it).
func TestWARPTemplate_OmitsReservedWhenEmpty(t *testing.T) {
	yaml, err := WARPTemplate(
		"CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo=",
		"172.16.0.2",
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("WARPTemplate: %v", err)
	}
	if contains(yaml, "reserved:") {
		t.Errorf("reserved should be omitted when nil:\n%s", yaml)
	}
	if contains(yaml, "ipv6:") {
		t.Errorf("ipv6 should be omitted when empty:\n%s", yaml)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// TestWARPMasqueTemplate_GeneratesCorrectYAML verifies the masque YAML
// output aligns with https://wiki.metacubex.one/config/proxies/masque/
func TestWARPMasqueTemplate_GeneratesCorrectYAML(t *testing.T) {
	yaml, err := WARPMasqueTemplate(
		"CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo=",
		"172.16.0.2",
		"2606:4700:110:8216:dac5:4a83:49de:6997",
		"", // default network (UDP masque)
	)
	if err != nil {
		t.Fatalf("WARPMasqueTemplate: %v", err)
	}
	// type must be masque (not wireguard).
	if !contains(yaml, "type: masque") {
		t.Errorf("missing 'type: masque':\n%s", yaml)
	}
	// ip must carry CIDR /32 per masque schema.
	if !contains(yaml, "ip: 172.16.0.2/32") {
		t.Errorf("ip field must have /32 CIDR:\n%s", yaml)
	}
	// ipv6 must carry CIDR /128.
	if !contains(yaml, "ipv6: 2606:4700:110:8216:dac5:4a83:49de:6997/128") {
		t.Errorf("ipv6 field must have /128 CIDR:\n%s", yaml)
	}
	// masque has NO reserved field (wireguard-only).
	if contains(yaml, "reserved:") {
		t.Errorf("masque YAML must NOT have reserved field:\n%s", yaml)
	}
	// server + port + key fields present.
	if !contains(yaml, "server: engage.cloudflareclient.com") {
		t.Errorf("missing server:\n%s", yaml)
	}
	if !contains(yaml, "port: 2408") {
		t.Errorf("missing port:\n%s", yaml)
	}
	if !contains(yaml, "private-key: CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo=") {
		t.Errorf("missing private-key:\n%s", yaml)
	}
	// network must be absent when empty (default UDP).
	if contains(yaml, "network:") {
		t.Errorf("network should be omitted when empty:\n%s", yaml)
	}
}

// TestWARPMasqueTemplate_NetworkH2 verifies the network field is emitted
// when "h2" is specified.
func TestWARPMasqueTemplate_NetworkH2(t *testing.T) {
	yaml, err := WARPMasqueTemplate(
		"CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo=",
		"172.16.0.2",
		"",
		"h2",
	)
	if err != nil {
		t.Fatalf("WARPMasqueTemplate: %v", err)
	}
	if !contains(yaml, "network: h2") {
		t.Errorf("missing network: h2:\n%s", yaml)
	}
	// ipv6 should be omitted when empty.
	if contains(yaml, "ipv6:") {
		t.Errorf("ipv6 should be omitted when empty:\n%s", yaml)
	}
}

// TestWARPMasqueTemplate_InvalidNetwork rejects unknown network values.
func TestWARPMasqueTemplate_InvalidNetwork(t *testing.T) {
	_, err := WARPMasqueTemplate(
		"CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo=",
		"172.16.0.2", "", "unknown",
	)
	if err == nil {
		t.Fatal("expected error for invalid network, got nil")
	}
}

// TestWARPMasqueTemplate_KeepsExistingCIDR verifies that caller-supplied
// CIDRs are preserved (not doubled).
func TestWARPMasqueTemplate_KeepsExistingCIDR(t *testing.T) {
	yaml, err := WARPMasqueTemplate(
		"CJiuBUMZWavAdfelvnUUnee+sQqHwU5ObGkFxjb6zWo=",
		"10.0.0.1/24", "fd00::1/64", "",
	)
	if err != nil {
		t.Fatalf("WARPMasqueTemplate: %v", err)
	}
	if !contains(yaml, "ip: 10.0.0.1/24") {
		t.Errorf("CIDR should be preserved, not doubled:\n%s", yaml)
	}
	if !contains(yaml, "ipv6: fd00::1/64") {
		t.Errorf("IPv6 CIDR should be preserved:\n%s", yaml)
	}
}
