package node

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gopkg.in/yaml.v3"
)

func GenerateMihomoListeners(dbNodes []models.Listener) ([]map[string]interface{}, error) {
	list := make([]map[string]interface{}, 0, len(dbNodes))
	for _, node := range dbNodes {
		if !node.Enabled {
			continue
		}
		if err := ValidateNode(&node); err != nil {
			return nil, fmt.Errorf("invalid listener %q: %w", node.Name, err)
		}
		protocol := strings.ToLower(strings.TrimSpace(node.Protocol))
		if !mihomoConfig.IsMihomoListenerProtocol(protocol) {
			return nil, fmt.Errorf("unsupported Mihomo listener protocol: %s", protocol)
		}

		// Prefer numeric port for single ports to match official Mihomo listener examples.
		var portVal interface{} = strings.TrimSpace(node.Port)
		if p, err := strconv.Atoi(strings.TrimSpace(node.Port)); err == nil {
			portVal = p
		}
		entry := map[string]interface{}{
			"name":   node.Name,
			"type":   protocol,
			"port":   portVal,
			"listen": firstNonEmpty(node.BindAddress, node.Listen, "0.0.0.0"),
		}
		if node.Rule != "" {
			entry["rule"] = node.Rule
		}
		if node.Proxy != "" {
			entry["proxy"] = node.Proxy
		}
		if node.RoutingMark > 0 {
			entry["routing-mark"] = node.RoutingMark
		}
		if node.UDP && listenerSupportsUDP(protocol) {
			entry["udp"] = true
		}

		options, err := decodeAndExpand(node.Config)
		if err != nil {
			return nil, fmt.Errorf("invalid config for listener %q: %w", node.Name, err)
		}
		normalizeListenerUsers(protocol, options)
		for key, value := range options {
			if value != nil {
				entry[key] = value
			}
		}
		list = append(list, entry)
	}
	return list, nil
}

func GenerateConfigYAML(dbNodes []models.Listener) (string, error) {
	listeners, err := GenerateMihomoListeners(dbNodes)
	if err != nil {
		return "", err
	}
	data, err := yaml.Marshal(map[string]interface{}{"listeners": listeners})
	if err != nil {
		return "", fmt.Errorf("failed to marshal listeners yaml: %w", err)
	}
	return string(data), nil
}

func decodeAndExpand(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &rawMap); err != nil {
		return nil, err
	}
	return expandDottedKeys(rawMap), nil
}

func expandDottedKeys(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		value = expandValue(value)
		parts := strings.Split(key, ".")
		if len(parts) == 1 {
			dst[key] = value
			continue
		}
		cursor := dst
		for _, part := range parts[:len(parts)-1] {
			next, ok := cursor[part].(map[string]interface{})
			if !ok {
				next = map[string]interface{}{}
				cursor[part] = next
			}
			cursor = next
		}
		cursor[parts[len(parts)-1]] = value
	}
	return dst
}

func expandValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return expandDottedKeys(v)
	case []interface{}:
		for i := range v {
			v[i] = expandValue(v[i])
		}
	}
	return value
}

func normalizeListenerUsers(protocol string, options map[string]interface{}) {
	users, ok := options["users"].([]interface{})
	if !ok || len(users) == 0 {
		return
	}
	if protocol == "anytls" || protocol == "hysteria2" || protocol == "mieru" || protocol == "tuic" {
		m := map[string]interface{}{}
		for _, raw := range users {
			row, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			name := strings.TrimSpace(fmt.Sprint(row["username"]))
			if name == "" {
				name = strings.TrimSpace(fmt.Sprint(row["uuid"]))
			}
			password := row["password"]
			if name != "" && password != nil {
				m[name] = password
			}
		}
		if len(m) > 0 {
			options["users"] = m
		}
		return
	}
	for _, raw := range users {
		row, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		switch protocol {
		case "vmess":
			delete(row, "flow")
		case "vless":
			delete(row, "alterId")
		case "trojan", "shadowquic", "trusttunnel":
			delete(row, "uuid")
			delete(row, "flow")
			delete(row, "alterId")
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func listenerSupportsUDP(protocol string) bool {
	switch protocol {
	case "shadowsocks", "snell", "vmess", "vless", "trojan", "anytls", "trusttunnel":
		return true
	default:
		return false
	}
}
