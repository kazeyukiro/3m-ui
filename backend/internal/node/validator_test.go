package node

import "testing"

func TestValidateProtocolSpecificSudokuOfficialValues(t *testing.T) {
	valid := []map[string]interface{}{
		{"aead-method": "chacha20-poly1305", "table-type": "prefer_ascii", "padding-min": float64(1), "padding-max": float64(15)},
		{"aead-method": "aes-128-gcm", "table-type": "up_entropy_down_ascii", "padding-min": float64(0), "padding-max": float64(100)},
		{"aead-method": "none"},
	}
	for i, cfg := range valid {
		if err := validateProtocolSpecific("sudoku", cfg); err != nil {
			t.Fatalf("valid Sudoku case %d rejected: %v", i, err)
		}
	}
	invalid := []map[string]interface{}{
		{"aead-method": "aes-256-gcm"},
		{"table-type": "invalid"},
		{"padding-min": float64(101)},
		{"padding-min": float64(20), "padding-max": float64(10)},
		{"httpmask": map[string]interface{}{"mode": "invalid"}},
	}
	for i, cfg := range invalid {
		if err := validateProtocolSpecific("sudoku", cfg); err == nil {
			t.Fatalf("invalid Sudoku case %d unexpectedly accepted", i)
		}
	}
}

func TestValidateProtocolSpecificMieruAndTrustTunnel(t *testing.T) {
	if err := validateProtocolSpecific("mieru", map[string]interface{}{"transport": "TCP"}); err != nil {
		t.Fatal(err)
	}
	if err := validateProtocolSpecific("mieru", map[string]interface{}{"transport": "udp"}); err != nil {
		t.Fatal(err)
	}
	if err := validateProtocolSpecific("mieru", map[string]interface{}{"transport": "QUIC"}); err == nil {
		t.Fatal("expected invalid Mieru transport to be rejected")
	}
	if err := validateProtocolSpecific("trusttunnel", map[string]interface{}{"network": []interface{}{"tcp", "udp"}}); err != nil {
		t.Fatal(err)
	}
	if err := validateProtocolSpecific("trusttunnel", map[string]interface{}{"network": []interface{}{"tcp", "quic"}}); err == nil {
		t.Fatal("expected invalid TrustTunnel network to be rejected")
	}
}
