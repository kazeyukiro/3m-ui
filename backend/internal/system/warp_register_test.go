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
