package converter

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// userBoundListeners returns enabled listeners bound to the proxy user together
// with the matching credential slice for each listener.
func userBoundListeners(db *gorm.DB, pu models.ProxyUser) ([]models.Listener, map[uint][]user.Credential, error) {
	var binds []models.ListenerUser
	if err := db.Where("proxy_user_id = ?", pu.ID).Find(&binds).Error; err != nil {
		return nil, nil, err
	}
	if len(binds) == 0 {
		return nil, nil, fmt.Errorf("user is not bound to any listener")
	}
	byListener, err := user.NewService(db).ActiveCredentialsByListener()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load credentials: %w", err)
	}
	listeners := make([]models.Listener, 0, len(binds))
	filtered := make(map[uint][]user.Credential, len(binds))
	for _, b := range binds {
		var listener models.Listener
		if err := db.First(&listener, b.ListenerID).Error; err != nil {
			continue
		}
		if !listener.Enabled {
			continue
		}
		creds := byListener[listener.ID]
		match := make([]user.Credential, 0)
		for _, c := range creds {
			if c.UUID == pu.UUID || c.Username == pu.Username {
				match = append(match, c)
			}
		}
		if len(match) == 0 {
			continue
		}
		listeners = append(listeners, listener)
		filtered[listener.ID] = match
	}
	if len(listeners) == 0 {
		return nil, nil, fmt.Errorf("no exportable proxies for user")
	}
	return listeners, filtered, nil
}

// GenerateUserRawConfig builds a multi-proxy Mihomo YAML for one ProxyUser
// across all bound enabled listeners (client subscription token).
func GenerateUserRawConfig(db *gorm.DB, pu models.ProxyUser, req *http.Request) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if !user.IsCredentialActive(pu) {
		return nil, fmt.Errorf("user is not active")
	}
	listeners, filtered, err := userBoundListeners(db, pu)
	if err != nil && !strings.Contains(err.Error(), "not bound") && !strings.Contains(err.Error(), "no exportable") {
		return nil, err
	}
	if err != nil {
		listeners, filtered = nil, map[uint][]user.Credential{}
	}
	serverHost := ResolveServerAddress(config.GlobalConfig, req)

	var allProxies []map[string]interface{}
	var names []string
	for _, listener := range listeners {
		creds := filtered[listener.ID]
		host := ResolveListenerServer(config.GlobalConfig, req, listener)
		if host == "" {
			host = serverHost
		}
		proxies, err := listenerToProxies(listener, host, creds)
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
	// Merge mirrored remote cluster nodes bound to this user.
	if mirrors, mErr := loadBoundRemoteMirrors(db, pu.ID); mErr == nil && len(mirrors) > 0 {
		allProxies, names = appendRemoteProxyMaps(mirrors, allProxies, names)
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

// URIGenerator builds share links for a listener + credentials (injected to avoid import cycles).
type URIGenerator func(listener models.Listener, host string, credentials []user.Credential) ([]string, error)

// GenerateUserBase64Subscription returns the classic v2ray-style subscription body:
// share links (vless://, vmess://, trojan://, …) joined by newlines, then base64-encoded.
// v2rayNG / Hiddify / Streisand clients can import it.
func GenerateUserBase64Subscription(db *gorm.DB, pu models.ProxyUser, req *http.Request, gen URIGenerator) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if gen == nil {
		return nil, fmt.Errorf("URI generator is required")
	}
	if !user.IsCredentialActive(pu) {
		return nil, fmt.Errorf("user is not active")
	}
	listeners, filtered, err := userBoundListeners(db, pu)
	if err != nil {
		return nil, err
	}
	serverHost := ResolveServerAddress(config.GlobalConfig, req)

	var links []string
	for _, listener := range listeners {
		creds := filtered[listener.ID]
		host := ResolveListenerServer(config.GlobalConfig, req, listener)
		if host == "" {
			host = serverHost
		}
		uris, err := gen(listener, host, creds)
		if err != nil {
			continue
		}
		for _, u := range uris {
			u = strings.TrimSpace(u)
			if u != "" {
				links = append(links, u)
			}
		}
	}
	if mirrors, mErr := loadBoundRemoteMirrors(db, pu.ID); mErr == nil && len(mirrors) > 0 {
		links = appendRemoteShareURIs(mirrors, links)
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("no exportable share links for user")
	}
	body := strings.Join(links, "\n")
	return []byte(EncodeBase64([]byte(body))), nil
}
