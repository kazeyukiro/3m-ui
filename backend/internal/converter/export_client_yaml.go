package converter

import (
	"fmt"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gopkg.in/yaml.v3"
)

// ExportClientYAML builds an m-ui style single-document Mihomo client YAML
// containing only the proxies for this listener (for share / QR tooling).
func ExportClientYAML(l models.Listener, server string, credentials []user.Credential) (string, error) {
	proxies, err := listenerToProxies(l, server, credentials)
	if err != nil {
		return "", err
	}
	if len(proxies) == 0 {
		return "", fmt.Errorf("no client proxies generated")
	}
	raw, err := yaml.Marshal(map[string]interface{}{"proxies": proxies})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
