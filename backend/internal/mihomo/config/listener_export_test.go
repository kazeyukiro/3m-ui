package config_test

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
)

func TestGenerateListenersForExportNormalizesShadowQUICUsers(t *testing.T) {
	listeners, err := config.GenerateListenersForExport([]models.Listener{
		{
			BaseModel:   models.BaseModel{ID: 1},
			Name:        "shadowquic-test",
			Type:        "shadowquic",
			Protocol:    "shadowquic",
			Listen:      "0.0.0.0",
			BindAddress: "0.0.0.0",
			Port:        "443",
			Enabled:     true,
			Config:      `{"users":{"client":"secret"}}`,
		},
	})
	if err != nil {
		t.Fatalf("generate listeners: %v", err)
	}
	if len(listeners) != 1 {
		t.Fatalf("expected one listener, got %d", len(listeners))
	}

	users, ok := listeners[0]["users"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected ShadowQUIC users to be a list, got %T", listeners[0]["users"])
	}
	if len(users) != 1 || users[0]["username"] != "client" || users[0]["password"] != "secret" {
		t.Fatalf("unexpected users: %#v", users)
	}
}
