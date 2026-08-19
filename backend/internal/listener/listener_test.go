package listener_test

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/listener"
)

func TestGenerateConfigYAML(t *testing.T) {
	dbListeners := []models.Listener{
		{
			Name:        "test-shadowsocks",
			Type:        "shadowsocks",
			Protocol:    "shadowsocks",
			Listen:      "0.0.0.0",
			BindAddress: "0.0.0.0",
			Port:        "1080",
			Enabled:     true,
			UDP:         true,
			Config:      `{"cipher":"aes-256-gcm","password":"pass"}`,
		},
		{
			Name:    "disabled",
			Type:    "vless",
			Listen:  "127.0.0.1",
			Port:    "1081",
			Enabled: false,
		},
	}

	yamlStr, err := listener.GenerateConfigYAML(dbListeners)
	if err != nil {
		t.Fatalf("failed to generate yaml: %v", err)
	}
	if yamlStr == "" {
		t.Fatal("expected non-empty yaml")
	}
	if !contains(yamlStr, "test-shadowsocks") {
		t.Fatal("expected yaml to contain enabled listener")
	}
	if contains(yamlStr, "disabled") {
		t.Fatal("expected yaml NOT to contain disabled listener")
	}
	if !contains(yamlStr, "cipher: aes-256-gcm") {
		t.Fatal("expected yaml to contain listener protocol properties")
	}
}

func TestGenerateConfigYAMLRejectsExcludedProtocol(t *testing.T) {
	_, err := listener.GenerateConfigYAML([]models.Listener{{Name: "socks", Type: "socks", Protocol: "socks", Listen: "0.0.0.0", Port: "1080", Enabled: true}})
	if err == nil {
		t.Fatal("expected excluded SOCKS listener protocol to be rejected")
	}
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
