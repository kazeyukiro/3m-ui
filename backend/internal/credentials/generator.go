package credentials

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// EnsureListenerCredentials adds a stable client credential to listeners that
// require one but do not have one yet. It deliberately has no dependency on
// node, user, or config packages so credential generation can be shared by
// validation and export without creating import cycles.
func EnsureListenerCredentials(l *models.Listener) error {
	if l == nil {
		return fmt.Errorf("listener is nil")
	}
	cfg, err := decodeConfig(l.Config)
	if err != nil {
		return err
	}
	proto := strings.ToLower(strings.TrimSpace(l.Protocol))
	if proto == "" {
		proto = strings.ToLower(strings.TrimSpace(l.Type))
	}
	if requiresUserCredentials(proto) && !hasExportCredentials(proto, cfg) {
		switch proto {
		case "vless", "vmess":
			uuid, err := randomUUID()
			if err != nil {
				return fmt.Errorf("generate client uuid: %w", err)
			}
			cfg["users"] = []interface{}{map[string]interface{}{"username": "client", "uuid": uuid}}
		case "trojan", "shadowquic":
			password, err := randomSecret(24)
			if err != nil {
				return fmt.Errorf("generate client credential: %w", err)
			}
			cfg["users"] = []interface{}{map[string]interface{}{"username": "client", "password": password}}
		case "tuic-v4":
			// Wiki inbound: token: [TOKEN] only — never users.
			password, err := randomSecret(24)
			if err != nil {
				return fmt.Errorf("generate TUIC v4 token: %w", err)
			}
			delete(cfg, "users")
			cfg["token"] = []string{password}
		case "hysteria2", "anytls", "mieru", "tuic", "tuic-v5":
			password, err := randomSecret(24)
			if err != nil {
				return fmt.Errorf("generate client credential: %w", err)
			}
			username := "client"
			if proto == "tuic" || proto == "tuic-v5" {
				username, err = randomUUID()
				if err != nil {
					return fmt.Errorf("generate TUIC client uuid: %w", err)
				}
			}
			delete(cfg, "token")
			cfg["users"] = map[string]interface{}{username: password}
		}
		encoded, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("encode listener credentials: %w", err)
		}
		l.Config = string(encoded)
	}
	return nil
}

func requiresUserCredentials(proto string) bool {
	switch strings.ToLower(proto) {
	case "vless", "vmess", "trojan", "hysteria2", "anytls", "mieru", "shadowquic", "tuic", "tuic-v4", "tuic-v5":
		return true
	default:
		return false
	}
}

func hasExportCredentials(proto string, cfg map[string]interface{}) bool {
	proto = strings.ToLower(proto)
	if proto == "tuic-v4" {
		return tokenNonEmpty(cfg["token"])
	}
	users, ok := cfg["users"]
	if !ok || users == nil {
		// plain "tuic" may be v4-style token-only
		if proto == "tuic" {
			return tokenNonEmpty(cfg["token"])
		}
		return false
	}
	switch proto {
	case "hysteria2", "anytls", "mieru", "tuic", "tuic-v5":
		m, ok := users.(map[string]interface{})
		return ok && len(m) > 0
	default:
		list, ok := users.([]interface{})
		return ok && len(list) > 0
	}
}

func tokenNonEmpty(tok interface{}) bool {
	switch v := tok.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []string:
		for _, s := range v {
			if strings.TrimSpace(s) != "" {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				return true
			}
		}
	}
	return false
}
func randomSecret(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func randomUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
func decodeConfig(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid listener configuration: %w", err)
	}
	if cfg == nil {
		return map[string]interface{}{}, nil
	}
	return cfg, nil
}
