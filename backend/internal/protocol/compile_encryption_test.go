package protocol

import "testing"

func TestVLESSCompileStripsClientEncryption(t *testing.T) {
	reg := DefaultCompileRegistry()
	in := CompileInput{
		Name:     "vless-enc",
		Protocol: "vless",
		Listen:   "0.0.0.0",
		Port:     443,
		Config: map[string]interface{}{
			"flow":            "xtls-rprx-vision",
			"decryption":      "server-decryption",
			"encryption":      "client-only-must-strip",
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
	if _, ok := m["encryption"]; ok {
		t.Fatalf("encryption must not appear on server listener: %#v", m["encryption"])
	}
	if _, ok := m["transport_layer"]; ok {
		t.Fatalf("transport_layer panel key leaked")
	}
}
