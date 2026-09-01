package config

import "github.com/kazeyukiro/3m-ui/backend/internal/database/models"

// GenerateListenersForExport compiles persisted listeners through the same
// protocol-aware generator used by the final Mihomo configuration. Keeping
// listener compilation in one place prevents export and runtime config from
// drifting apart (for example, ShadowQUIC users being emitted as a map).
func GenerateListenersForExport(listeners []models.Listener) ([]map[string]interface{}, error) {
	return generateListeners(nil, listeners, nil)
}
