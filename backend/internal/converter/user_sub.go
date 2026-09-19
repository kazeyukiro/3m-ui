package converter

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
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
		// Listener is bound: always export it. Protocols whose auth lives in
		// Config (TUIC token/users, SS password, snell psk, …) often have no
		// panel-mirrored credential row matching this user UUID — skipping
		// them made subscription omit the node while per-node URI still worked.
		if len(match) == 0 {
			match = append(match, creds...)
		}
		if len(match) == 0 {
			match = append(match, user.Credential{
				Username: pu.Username,
				Password: pu.Password,
				UUID:     pu.UUID,
			})
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
	var skipReasons []string
	for _, listener := range listeners {
		creds := filtered[listener.ID]
		host := ResolveListenerServer(config.GlobalConfig, req, listener)
		if host == "" {
			host = serverHost
		}
		proxies, err := listenerToProxies(listener, host, creds)
		if err != nil || len(proxies) == 0 {
			// Fallback: protocol registry ClientYAML (same path as node URI share).
			pcreds := make([]protocol.UserCred, 0, len(creds))
			for _, c := range creds {
				pcreds = append(pcreds, protocol.UserCred{Username: c.Username, Password: c.Password, UUID: c.UUID})
			}
			if shares, err2 := protocol.ExportShares(listener, host, pcreds); err2 == nil {
				for _, sh := range shares {
					if maps := proxiesFromClientYAML(sh.ClientYAML, listener.Name, 0); len(maps) > 0 {
						proxies = append(proxies, maps...)
					}
				}
			}
			if len(proxies) == 0 {
				if err != nil {
					skipReasons = append(skipReasons, fmt.Sprintf("%s: %v", listener.Name, err))
				} else {
					skipReasons = append(skipReasons, fmt.Sprintf("%s: empty export", listener.Name))
				}
				continue
			}
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
	if strings.TrimSpace(pu.ExternalLinks) != "" {
		allProxies, names = mergeExternalSubscriptionLinks(pu.ExternalLinks, allProxies, names)
	}
	if len(allProxies) == 0 {
		if len(skipReasons) > 0 {
			return nil, fmt.Errorf("no exportable proxies for user (%s)", strings.Join(skipReasons, "; "))
		}
		return nil, fmt.Errorf("no exportable proxies for user")
	}
	return yaml.Marshal(clientSubscriptionDocument(allProxies, names))
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
	var skipReasons []string
	for _, listener := range listeners {
		creds := filtered[listener.ID]
		host := ResolveListenerServer(config.GlobalConfig, req, listener)
		if host == "" {
			host = serverHost
		}
		uris, err := gen(listener, host, creds)
		if err != nil || len(uris) == 0 {
			pcreds := make([]protocol.UserCred, 0, len(creds))
			for _, c := range creds {
				pcreds = append(pcreds, protocol.UserCred{Username: c.Username, Password: c.Password, UUID: c.UUID})
			}
			if shares, err2 := protocol.ExportShareURIs(listener, host, pcreds); err2 == nil && len(shares) > 0 {
				uris = shares
				err = nil
			}
		}
		if err != nil {
			skipReasons = append(skipReasons, fmt.Sprintf("%s: %v", listener.Name, err))
			continue
		}
		if len(uris) == 0 {
			skipReasons = append(skipReasons, fmt.Sprintf("%s: empty URI export", listener.Name))
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
		if len(skipReasons) > 0 {
			return nil, fmt.Errorf("no exportable share links for user (%s)", strings.Join(skipReasons, "; "))
		}
		return nil, fmt.Errorf("no exportable share links for user")
	}
	body := strings.Join(links, "\n")
	return []byte(EncodeBase64([]byte(body))), nil
}
