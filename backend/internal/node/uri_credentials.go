package node

import (
	"encoding/json"
	"fmt"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

// ClientURIsWithCredentials bridges node URI export with the canonical
// credential service. Listener.Config is server configuration and is not the
// source of truth for users managed by 3m-ui.
func ClientURIsWithCredentials(listener models.Listener, host string, credentials []user.Credential) ([]string, error) {
	var cfg map[string]interface{}
	if listener.Config != "" {
		if err := json.Unmarshal([]byte(listener.Config), &cfg); err != nil {
			return nil, fmt.Errorf("invalid listener configuration: %w", err)
		}
	}
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	// Prefer panel credentials; merge server-side flow for Vision when present.
	flow, _ := cfg["flow"].(string)
	proto := listener.Protocol
	if len(credentials) > 0 {
		switch proto {
		case "tuic", "tuic-v4", "tuic-v5":
			// v4 auth is token[]; keep existing token from config when present.
			// v5 uses users map UUID→password (wiki proxies/tuic).
			hasToken := false
			switch tok := cfg["token"].(type) {
			case string:
				hasToken = tok != ""
			case []interface{}:
				hasToken = len(tok) > 0
			case []string:
				hasToken = len(tok) > 0
			}
			if proto == "tuic-v4" || hasToken {
				// Prefer server token for v4; do not overwrite with panel user map.
			} else {
				users := make(map[string]interface{}, len(credentials))
				for _, credential := range credentials {
					key := credential.UUID
					if key == "" {
						key = credential.Username
					}
					if key != "" {
						users[key] = credential.Password
					}
				}
				if len(users) > 0 {
					cfg["users"] = users
				}
			}
		case "anytls", "hysteria2", "mieru":
			users := make(map[string]interface{}, len(credentials))
			for _, credential := range credentials {
				if credential.Username != "" {
					users[credential.Username] = credential.Password
				}
			}
			cfg["users"] = users
		default:
			users := make([]interface{}, 0, len(credentials))
			for _, credential := range credentials {
				row := map[string]interface{}{"username": credential.Username, "password": credential.Password, "uuid": credential.UUID}
				if flow != "" && (listener.Protocol == "vless" || listener.Protocol == "vmess") {
					row["flow"] = flow
				}
				users = append(users, row)
			}
			cfg["users"] = users
		}
		if listener.Protocol == "shadowsocks" {
			cfg["password"] = credentials[0].Password
		}
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare URI configuration: %w", err)
	}
	listener.Config = string(encoded)
	return ClientURIs(listener, host)
}
