package listener

import (
	"fmt"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gopkg.in/yaml.v3"
)

// GenerateConfigYAML converts persisted listeners to Mihomo's native listener
// schema. Runtime configuration and export now share the same protocol-aware
// compiler so listener serialization cannot drift between code paths.
func GenerateConfigYAML(dbListeners []models.Listener) (string, error) {
	listenersList, err := mihomoConfig.GenerateListenersForExport(dbListeners)
	if err != nil {
		return "", err
	}

	yamlBytes, err := yaml.Marshal(&MihomoConfig{Listeners: listenersList})
	if err != nil {
		return "", fmt.Errorf("failed to marshal mihomo config to yaml: %w", err)
	}
	return string(yamlBytes), nil
}
