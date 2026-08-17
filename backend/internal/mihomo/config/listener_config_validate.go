package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateListenerConfig validates protocol-specific JSON config against the
// Mihomo listener schema allowlist (top-level and known nested object keys).
func ValidateListenerConfig(protocol, raw string) error {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return fmt.Errorf("invalid listener config JSON: %w", err)
	}
	schema, ok := GetMihomoListenerSchema(protocol)
	if !ok {
		return nil
	}
	return validateMap(cfg, schema, "")
}

func validateMap(cfg map[string]interface{}, schema ListenerSchema, path string) error {
	for key, value := range cfg {
		full := key
		if path != "" {
			full = path + "." + key
		}

		if path == "" {
			if _, allowed := schema.Fields[key]; !allowed {
				return fmt.Errorf("unknown field %q for protocol %q", key, schema.Protocol)
			}
		} else {
			parentKeys, ok := schema.NestedFields[path]
			if ok {
				if _, allowed := parentKeys[key]; !allowed {
					return fmt.Errorf("unknown nested field %q for protocol %q", full, schema.Protocol)
				}
			}
		}

		switch v := value.(type) {
		case map[string]interface{}:
			if err := validateMap(v, schema, full); err != nil {
				return err
			}
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if err := validateMap(m, schema, full); err != nil {
						return err
					}
				}
			}
		default:
			if path == "" {
				if _, isNested := schema.NestedFields[key]; isNested {
					return fmt.Errorf("field %q must be an object", key)
				}
			}
		}
	}
	return nil
}
