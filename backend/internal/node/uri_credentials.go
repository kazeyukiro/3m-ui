package node

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

// ClientURIsWithCredentials bridges node URI export with the canonical
// credential service. Listener.Config is server configuration and is not the
// source of truth for users managed by 3m-ui.
//
// Critical: never write an empty users/password blob over Config — that wiped
// HY2/AnyTLS passwords and made whole protocols vanish from subscriptions.
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
	flow, _ := cfg["flow"].(string)
	proto := strings.ToLower(strings.TrimSpace(listener.Protocol))

	if len(credentials) > 0 {
		switch proto {
		case "tuic", "tuic-v4", "tuic-v5":
			hasToken := false
			switch tok := cfg["token"].(type) {
			case string:
				hasToken = strings.TrimSpace(tok) != ""
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
					key := strings.TrimSpace(credential.UUID)
					if key == "" {
						key = strings.TrimSpace(credential.Username)
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
			// Map auth: username→password. Fall back to UUID as map key when
			// username is empty (panel often stores password under UUID-only users).
			users := make(map[string]interface{}, len(credentials))
			var singlePass string
			for _, credential := range credentials {
				key := strings.TrimSpace(credential.Username)
				if key == "" {
					key = strings.TrimSpace(credential.UUID)
				}
				pass := credential.Password
				if key != "" {
					users[key] = pass
				} else if pass != "" {
					singlePass = pass
				}
			}
			if len(users) > 0 {
				cfg["users"] = users
			} else if singlePass != "" {
				// Password-only HY2-style listeners.
				cfg["password"] = singlePass
			}
			// If neither produced anything, leave Config users untouched.

		case "shadowsocks", "snell", "sudoku":
			// Single-secret protocols: mirror the runtime compiler and the
			// client-YAML exporter (P1-1):
			//   config secret present → keep it (the live inbound authenticates
			//                          with exactly this secret)
			//   config secret absent  → fall back to the subscriber's own secret
			// This keeps the share link, the Mihomo YAML and the live listener
			// in agreement about the password. Resolution must not depend on
			// how many users are bound: the export path only ever sees the
			// subscribing user, while the runtime sees them all.
			field := "password"
			switch proto {
			case "snell":
				field = "psk"
			case "sudoku":
				field = "key"
			}
			if proto == "shadowsocks" {
				if _, ok := protocol.SSConfigUsers(cfg); ok {
					// Explicit multi-user (mihomo v1.19.34): resolve the client
					// password for this subscriber and strip the server-side
					// users map so the ss:// link carries the right secret.
					username := ""
					if len(credentials) > 0 {
						username = credentials[0].Username
					}
					pass, found := protocol.ResolveSSClientPassword(cfg, username)
					if !found {
						return nil, fmt.Errorf("shadowsocks multi-user: no password configured for subscriber %q", username)
					}
					cfg["password"] = pass
					delete(cfg, "users")
				} else {
					applySingleSecretForExport(cfg, field, credentials)
				}
			} else {
				applySingleSecretForExport(cfg, field, credentials)
			}

		default:
			// VLESS / VMess / Trojan / ShadowQUIC / TrustTunnel: array users.
			users := make([]interface{}, 0, len(credentials))
			for _, credential := range credentials {
				uid := strings.TrimSpace(credential.UUID)
				if uid == "" {
					uid = strings.TrimSpace(credential.Username)
				}
				row := map[string]interface{}{
					"username": credential.Username,
					"password": credential.Password,
					"uuid":     uid,
				}
				if flow != "" && protocol.TransportCarriesFlow(cfg) &&
					(proto == "vless" || proto == "vmess") {
					row["flow"] = flow
				}
				// Skip completely empty rows so we never replace a working
				// Config users array with blank panel placeholders.
				if strings.TrimSpace(credential.UUID) == "" &&
					strings.TrimSpace(credential.Username) == "" &&
					strings.TrimSpace(credential.Password) == "" {
					continue
				}
				users = append(users, row)
			}
			if len(users) > 0 {
				cfg["users"] = users
			}
		}
	}

	encoded, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare URI configuration: %w", err)
	}
	listener.Config = string(encoded)
	return ClientURIs(listener, host)
}

// applySingleSecretForExport mirrors the runtime single-secret resolution:
// config secret first, otherwise the subscriber's own password.
func applySingleSecretForExport(cfg map[string]interface{}, field string, credentials []user.Credential) {
	secret := ""
	if v, ok := cfg[field].(string); ok && strings.TrimSpace(v) != "" {
		secret = strings.TrimSpace(v)
	} else if len(credentials) > 0 && credentials[0].Password != "" {
		secret = credentials[0].Password
	}
	if secret != "" {
		cfg[field] = secret
	}
}
