package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type Credential struct{ Username, Password, UUID string }

var CredentialProvider func() (map[uint][]Credential, error)

type ConfigEngine struct{ db *gorm.DB }

func NewConfigEngine(db *gorm.DB) *ConfigEngine { return &ConfigEngine{db: db} }

func (ce *ConfigEngine) GenerateFinalConfig() (string, error) {
	if ce == nil || ce.db == nil {
		return "", fmt.Errorf("config engine database is not initialized")
	}
	baseBytes, err := yaml.Marshal(GetDefaultTemplate())
	if err != nil {
		return "", err
	}
	var merged map[string]interface{}
	if err := yaml.Unmarshal(baseBytes, &merged); err != nil {
		return "", err
	}
	var customFragments []models.Config
	if err := ce.db.Where("enabled = ?", true).Find(&customFragments).Error; err != nil {
		return "", fmt.Errorf("load custom config fragments: %w", err)
	}
	for _, fragment := range customFragments {
		var fragMap map[string]interface{}
		if err := yaml.Unmarshal([]byte(fragment.Content), &fragMap); err != nil {
			return "", fmt.Errorf("invalid custom config %q: %w", fragment.Name, err)
		}
		for k, v := range fragMap { merged[k] = v }
	}
	var listeners []models.Listener
	if err := ce.db.Where("enabled = ?", true).Find(&listeners).Error; err != nil {
		return "", fmt.Errorf("load enabled listeners: %w", err)
	}
	credentials := make(map[uint][]Credential)
	if CredentialProvider != nil {
		credentials, err = CredentialProvider()
		if err != nil { return "", fmt.Errorf("load listener credentials: %w", err) }
	}
	generated, err := generateListeners(listeners, credentials)
	if err != nil { return "", err }
	merged["listeners"] = generated
	finalBytes, err := yaml.Marshal(merged)
	if err != nil { return "", fmt.Errorf("serialize final configuration: %w", err) }
	return string(finalBytes), nil
}

func generateListeners(listeners []models.Listener, creds map[uint][]Credential) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(listeners))
	for _, l := range listeners {
		if !l.Enabled { continue }
		protocol := strings.ToLower(strings.TrimSpace(l.Protocol))
		if protocol == "" { protocol = strings.ToLower(strings.TrimSpace(l.Type)) }
		if !IsMihomoListenerProtocol(protocol) { return nil, fmt.Errorf("unsupported Mihomo listener protocol %q", protocol) }
		if !isValidPortString(l.Port) { return nil, fmt.Errorf("listener %q has invalid port %q", l.Name, l.Port) }
		listen := strings.TrimSpace(l.BindAddress)
		if listen == "" { listen = strings.TrimSpace(l.Listen) }
		if listen == "" { listen = "0.0.0.0" }
		var portVal interface{} = strings.TrimSpace(l.Port)
		if p, err := strconv.Atoi(strings.TrimSpace(l.Port)); err == nil { portVal = p }
		m := map[string]interface{}{"name": l.Name, "type": protocol, "port": portVal, "listen": listen}
		configMap, err := decodeListenerConfig(l.Config)
		if err != nil { return nil, fmt.Errorf("listener %q: %w", l.Name, err) }
		if l.Proxy != "" { m["proxy"] = l.Proxy }
		if l.Rule != "" { m["rule"] = l.Rule }
		if l.RoutingMark > 0 { m["routing-mark"] = l.RoutingMark }
		if l.UDP && listenerHasUDPOption(protocol) { m["udp"] = true }
		if value, ok := configMap["routing-mark"]; ok { m["routing-mark"] = value }
		for k, v := range configMap { if listenerFieldIsManaged(k) { continue }; m[k] = v }
		copyServerTLSFields(m, configMap)
		listenerCreds, hasCredentialState := creds[l.ID]
		if l.TLS {
			if !listenerSupportsTLS(protocol) { return nil, fmt.Errorf("listener %q: TLS is not supported for protocol %q", l.Name, protocol) }
			m["tls"] = true
		} else { delete(m, "tls") }

		switch protocol {
		case "shadowsocks":
			if len(listenerCreds) > 1 { return nil, fmt.Errorf("listener %q: Shadowsocks supports one password; %d active credentials are bound", l.Name, len(listenerCreds)) }
			copyOption(m, configMap, "cipher")
			if len(listenerCreds) == 1 && listenerCreds[0].Password != "" { m["password"] = listenerCreds[0].Password } else { copyOption(m, configMap, "password") }
		case "snell":
			copyOption(m, configMap, "psk"); copyOption(m, configMap, "version")
		case "vmess", "vless":
			users := make([]map[string]interface{}, 0, len(listenerCreds))
			for _, cred := range listenerCreds {
				if cred.UUID == "" { continue }
				u := map[string]interface{}{"uuid": cred.UUID}
				if cred.Username != "" { u["username"] = cred.Username }
				if flow, ok := configMap["flow"]; ok && protocol == "vless" { u["flow"] = flow }
				if alterID, ok := configMap["alterId"]; ok && protocol == "vmess" { u["alterId"] = alterID }
				users = append(users, u)
			}
			if len(users) > 0 { m["users"] = users } else if value, ok := configMap["users"]; ok && !hasCredentialState { m["users"] = value }
		case "trojan":
			users := make([]map[string]interface{}, 0, len(listenerCreds))
			for _, cred := range listenerCreds {
				if cred.Password == "" { continue }
				u := map[string]interface{}{"password": cred.Password}
				if cred.Username != "" { u["username"] = cred.Username }
				users = append(users, u)
			}
			if len(users) > 0 { m["users"] = users } else if value, ok := configMap["users"]; ok && !hasCredentialState { m["users"] = value }
		case "hysteria2", "anytls", "mieru":
			users := make(map[string]string)
			for _, cred := range listenerCreds { if cred.Username != "" && cred.Password != "" { users[cred.Username] = cred.Password } }
			if len(users) > 0 { m["users"] = users } else if value, ok := configMap["users"]; ok && !hasCredentialState { m["users"] = value }
		case "tuic":
			if value, ok := configMap["token"]; ok && !hasCredentialState { m["token"] = value } else {
				users := make(map[string]string)
				for _, cred := range listenerCreds { if cred.UUID != "" && cred.Password != "" { users[cred.UUID] = cred.Password } }
				if len(users) > 0 { m["users"] = users }
			}
		case "shadowquic", "trusttunnel":
			users := make([]map[string]interface{}, 0, len(listenerCreds))
			for _, cred := range listenerCreds { if cred.Username != "" && cred.Password != "" { users = append(users, map[string]interface{}{"username": cred.Username, "password": cred.Password}) } }
			if len(users) > 0 { m["users"] = users } else if value, ok := configMap["users"]; ok && !hasCredentialState {
				normalized, err := normalizeListenerUserList(value)
				if err != nil { return nil, fmt.Errorf("listener %q: %w", l.Name, err) }
				if normalized != nil { m["users"] = normalized }
			}
		case "sudoku":
		}
		if hasCredentialState && len(listenerCreds) == 0 {
			delete(m, "users")
			if protocol == "shadowsocks" { delete(m, "password") }
			if protocol == "tuic" { delete(m, "token") }
		}
		result = append(result, m)
	}
	return result, nil
}

func isValidPortString(s string) bool {
	s = strings.TrimSpace(s); if s == "" { return false }
	if strings.Contains(s, ",") { for _, p := range strings.Split(s, ",") { if !isValidPortString(strings.TrimSpace(p)) { return false } }; return true }
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2); if len(parts) != 2 { return false }
		start, err1 := strconv.Atoi(strings.TrimSpace(parts[0])); end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		return err1 == nil && err2 == nil && start >= 1 && end <= 65535 && start <= end
	}
	port, err := strconv.Atoi(s); return err == nil && port >= 1 && port <= 65535
}

func normalizeListenerUserList(value interface{}) ([]map[string]interface{}, error) {
	switch users := value.(type) {
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(users)); for i, item := range users { user, ok := item.(map[string]interface{}); if !ok { return nil, fmt.Errorf("users[%d] must be an object", i) }; result = append(result, user) }; return result, nil
	case map[string]interface{}:
		result := make([]map[string]interface{}, 0, len(users)); for username, password := range users { result = append(result, map[string]interface{}{"username": username, "password": fmt.Sprint(password)}) }; return result, nil
	case map[interface{}]interface{}:
		result := make([]map[string]interface{}, 0, len(users)); for rawUsername, rawPassword := range users { result = append(result, map[string]interface{}{"username": fmt.Sprint(rawUsername), "password": fmt.Sprint(rawPassword)}) }; return result, nil
	default: return nil, fmt.Errorf("users must be a list of username/password objects")
	}
}

func decodeListenerConfig(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" { return map[string]interface{}{}, nil }
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &configMap); err != nil { return nil, fmt.Errorf("invalid listener configuration (must be valid JSON): %w", err) }
	if configMap == nil { configMap = map[string]interface{}{} }
	return configMap, nil
}

func listenerFieldIsManaged(key string) bool {
	switch key {
	case "users", "username", "password", "uuid", "flow", "alterId", "tls", "servername", "sni", "skip-cert-verify", "name-cert-verify", "fingerprint", "client-fingerprint", "reality-opts", "shadow-tls-opts", "restls-opts", "jls-opts", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "mkcp-opts", "certificate", "private-key", "private_key":
		return true
	default: return false
	}
}

func copyServerTLSFields(dst, src map[string]interface{}) {
	if value, ok := src["certificate"]; ok { dst["certificate"] = value }
	if value, ok := src["private-key"]; ok { dst["private-key"] = value } else if value, ok := src["private_key"]; ok { dst["private-key"] = value }
}
func copyOption(dst, src map[string]interface{}, key string) { if value, ok := src[key]; ok { dst[key] = value } }
func listenerHasUDPOption(protocol string) bool {
	switch protocol { case "shadowsocks", "snell", "vmess", "vless", "trojan", "anytls", "trusttunnel": return true; default: return false }
}
func listenerSupportsTLS(protocol string) bool {
	switch protocol { case "vmess", "vless", "trojan", "anytls", "mieru", "trusttunnel": return true; default: return false }
}
