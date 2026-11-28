package converter

import "testing"

func TestEnsureVMessClientRequiredFieldsDefaults(t *testing.T) {
	p := map[string]interface{}{"type": "vmess", "uuid": "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"}
	ensureVMessClientRequiredFields("vmess", p)
	if p["alterId"] != 0 {
		t.Fatalf("alterId: %#v", p["alterId"])
	}
	if p["cipher"] != "auto" {
		t.Fatalf("cipher: %#v", p["cipher"])
	}
}

func TestEnsureVMessClientRequiredFieldsPreserves(t *testing.T) {
	p := map[string]interface{}{"alterId": 1, "cipher": "aes-128-gcm"}
	ensureVMessClientRequiredFields("vmess", p)
	if p["alterId"] != 1 {
		t.Fatalf("alterId: %#v", p["alterId"])
	}
	if p["cipher"] != "aes-128-gcm" {
		t.Fatalf("cipher: %#v", p["cipher"])
	}
}

func TestEnsureVMessClientRequiredFieldsIgnoresVLESS(t *testing.T) {
	p := map[string]interface{}{"type": "vless"}
	ensureVMessClientRequiredFields("vless", p)
	if _, ok := p["alterId"]; ok {
		t.Fatalf("vless must not get alterId: %#v", p)
	}
	if _, ok := p["cipher"]; ok {
		t.Fatalf("vless must not get cipher: %#v", p)
	}
}
