package converter

import (
	"fmt"
	"net/http"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// GenerateUserRawConfig builds a multi-proxy Mihomo YAML for one ProxyUser
// across all bound enabled listeners (3x-ui client subscription parity).
func GenerateUserRawConfig(db *gorm.DB, pu models.ProxyUser, req *http.Request) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if !user.IsCredentialActive(pu) {
		return nil, fmt.Errorf("user is not active")
	}
	var binds []models.ListenerUser
	if err := db.Where("proxy_user_id = ?", pu.ID).Find(&binds).Error; err != nil {
		return nil, err
	}
	if len(binds) == 0 {
		return nil, fmt.Errorf("user is not bound to any listener")
	}
	serverHost := ResolveServerAddress(config.GlobalConfig, req)
	byListener, err := user.NewService(db).ActiveCredentialsByListener()
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	var allProxies []map[string]interface{}
	var names []string
	for _, b := range binds {
		var listener models.Listener
		if err := db.First(&listener, b.ListenerID).Error; err != nil {
			continue
		}
		if !listener.Enabled {
			continue
		}
		creds := byListener[listener.ID]
		filtered := make([]user.Credential, 0)
		for _, c := range creds {
			if c.UUID == pu.UUID || c.Username == pu.Username {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		proxies, err := listenerToProxies(listener, serverHost, filtered)
		if err != nil {
			continue
		}
		for _, p := range proxies {
			if name, ok := p["name"].(string); ok {
				p["name"] = name + "-" + pu.Username
				names = append(names, p["name"].(string))
			}
			allProxies = append(allProxies, p)
		}
	}
	if len(allProxies) == 0 {
		return nil, fmt.Errorf("no exportable proxies for user")
	}
	cfg := map[string]interface{}{
		"proxies": allProxies,
		"proxy-groups": []interface{}{
			map[string]interface{}{
				"name":    "PROXY",
				"type":    "select",
				"proxies": names,
			},
		},
		"rules": []string{"MATCH,PROXY"},
	}
	return yaml.Marshal(cfg)
}
