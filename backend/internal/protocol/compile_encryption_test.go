package protocol

import "testing"

// TestVLESSCompileStripsClientOnlyEncryption verifies that the top-level
// "encryption" field (a CLIENT-side field per proxies-vless wiki) is STRIPPED
// from the listener config.yaml output. The value remains in the panel Config
// map for converter/client.go to read when emitting the client-side proxy YAML.
// Panel-only / client-export-only keys (transport_layer, security_layer,
// access_profile, _-prefixed) must also be stripped.
func TestVLESSCompileStripsClientOnlyEncryption(t *testing.T) {
	cfg := map[string]interface{}{
		"flow":            "xtls-rprx-vision",
		"decryption":      "server-decryption",
		"encryption":      "none",
		"transport_layer": "raw",
		"security_layer":  "none",
	}
	reg := DefaultCompileRegistry()
	in := CompileInput{
		Name:     "vless-enc",
		Protocol: "vless",
		Listen:   "0.0.0.0",
		Port:     443,
		Config:   cfg,
		Users:    []UserCred{{UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"}},
	}
	m, err := reg.Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	if m["decryption"] != "server-decryption" {
		t.Fatalf("decryption missing: %#v", m["decryption"])
	}
	// encryption is a client-only field; it must NOT appear on the listener YAML.
	if _, ok := m["encryption"]; ok {
		t.Fatalf("listener yaml must not carry client-only encryption field, got %#v", m["encryption"])
	}
	// encryption is preserved in the input Config map for client YAML emission.
	if cfg["encryption"] != "none" {
		t.Fatalf("input config encryption must be preserved for client emission, got %#v", cfg["encryption"])
	}
	if _, ok := m["transport_layer"]; ok {
		t.Fatalf("transport_layer panel key leaked")
	}
	if _, ok := m["security_layer"]; ok {
		t.Fatalf("security_layer panel key leaked")
	}
}
