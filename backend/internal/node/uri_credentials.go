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
	if len(credentials) > 0 {
		switch listener.Protocol {
		case "tuic":
			// TUIC v5 uses users: {UUID: PASSWORD} per official Mihomo docs.
			// Must match asUsersMapUUID in compile.go which keys by UUID.
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
			cfg["users"] = users
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
