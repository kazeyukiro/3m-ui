package protocol

import "testing"

// TestVLESSCompilePreservesServerEncryption verifies that the top-level
// "encryption" field is preserved on VLESS listener output. Per the official
// Mihomo schema, "encryption" is a legitimate server-side field paired with
// "decryption"; stripping it silently breaks VLESS listeners that rely on it.
// Panel-only / client-export-only keys (transport_layer, security_layer,
// access_profile, _-prefixed) must still be stripped.
func TestVLESSCompilePreservesServerEncryption(t *testing.T) {
	reg := DefaultCompileRegistry()
	in := CompileInput{
		Name:     "vless-enc",
		Protocol: "vless",
		Listen:   "0.0.0.0",
		Port:     443,
		Config: map[string]interface{}{
			"flow":            "xtls-rprx-vision",
			"decryption":      "server-decryption",
			"encryption":      "none",
			"transport_layer": "raw",
			"security_layer":  "none",
		},
		Users: []UserCred{{UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"}},
	}
	m, err := reg.Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	if m["decryption"] != "server-decryption" {
		t.Fatalf("decryption missing: %#v", m["decryption"])
	}
	if m["encryption"] != "none" {
		t.Fatalf("encryption must be preserved on server listener: %#v", m["encryption"])
	}
	if _, ok := m["transport_layer"]; ok {
		t.Fatalf("transport_layer panel key leaked")
	}
	if _, ok := m["security_layer"]; ok {
		t.Fatalf("security_layer panel key leaked")
	}
}
