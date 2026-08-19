package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
)

func TestValidateConfigYAML(t *testing.T) {
	// Valid YAML
	valid := `
mode: rule
port: 7890
dns:
  enable: true
`
	if err := mihomoConfig.ValidateConfigYAML(valid); err != nil {
		t.Fatalf("expected valid config to pass validation, got %v", err)
	}

	// Invalid YAML
	invalidSyntax := `
mode: rule: broken
`
	if err := mihomoConfig.ValidateConfigYAML(invalidSyntax); err == nil {
		t.Fatal("expected invalid syntax to fail validation")
	}

	// Invalid mode value
	invalidMode := `
mode: super-proxy
`
	if err := mihomoConfig.ValidateConfigYAML(invalidMode); err == nil {
		t.Fatal("expected invalid mode value to fail validation")
	}
}

func TestConfigEngineGeneration(t *testing.T) {
	dbPath := "/tmp/3m-ui-config-engine-test.db"
	_ = os.Remove(dbPath)

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}

	engine := mihomoConfig.NewConfigEngine(db)

	// Add a listener to database
	listener := models.Listener{
		Name:    "hk-gate",
		Type:    "mixed",
		Listen:  "127.0.0.1",
		Port:    "8081",
		Enabled: true,
	}
	if err := db.Create(&listener).Error; err != nil {
		t.Fatalf("failed to insert listener: %v", err)
	}

	// Add a custom configuration fragment to database
	fragment := models.Config{
		Name:    "dns-override",
		Type:    "custom",
		Content: "dns:\n  listen: 127.0.0.1:5353",
		Enabled: true,
	}
	if err := db.Create(&fragment).Error; err != nil {
		t.Fatalf("failed to insert config fragment: %v", err)
	}

	// Generate final config
	finalYaml, err := engine.GenerateFinalConfig()
	if err != nil {
		t.Fatalf("failed to generate final config: %v", err)
	}

	if !strings.Contains(finalYaml, "hk-gate") {
		t.Fatal("expected final config to contain listener 'hk-gate'")
	}

	if !strings.Contains(finalYaml, "127.0.0.1:5353") {
		t.Fatal("expected final config to contain overridden dns listener port")
	}

	_ = os.Remove(dbPath)
}
