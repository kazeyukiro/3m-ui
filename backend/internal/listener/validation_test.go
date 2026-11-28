package listener

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestValidateModel(t *testing.T) {
	valid := &models.Listener{Name: "vless", Protocol: "VLESS", Port: "443", Config: `{"flow":"xtls-rprx-vision"}`}
	if err := ValidateModel(valid); err != nil {
		t.Fatalf("valid listener rejected: %v", err)
	}
	if valid.Protocol != "vless" {
		t.Fatalf("protocol was not normalized: %q", valid.Protocol)
	}

	invalid := &models.Listener{Name: "bad", Protocol: "vless", Type: "trojan", Port: "443"}
	if err := ValidateModel(invalid); err == nil {
		t.Fatal("protocol/type mismatch was accepted")
	}

	reserved := &models.Listener{Name: "bad-config", Protocol: "vless", Port: "443", Config: `{"port":8443}`}
	if err := ValidateModel(reserved); err == nil {
		t.Fatal("reserved config field was accepted")
	}
}
