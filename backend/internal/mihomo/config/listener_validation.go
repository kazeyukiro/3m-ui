package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateListenerConfig validates the JSON fragment stored on a Listener.
// The fragment is intentionally limited to protocol-specific Mihomo fields;
// identity and transport fields owned by the Listener model cannot be
// overridden from Config JSON.
func ValidateListenerConfig(protocol, raw string) error {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	schema, ok := MihomoListenerSchemas[protocol]
	if !ok {
		return fmt.Errorf("unsupported Mihomo listener protocol %q", protocol)
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var values map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return fmt.Errorf("listener configuration must be valid JSON: %w", err)
	}
	if values == nil {
		return fmt.Errorf("listener configuration must be a JSON object")
	}

	reserved := map[string]struct{}{
		"name": {}, "type": {}, "port": {}, "listen": {}, "bind_address": {},
		"proxy": {}, "rule": {}, "enabled": {}, "status": {}, "tls": {}, "udp": {},
		"routing-mark": {},
	}
	return validateListenerObject(schema, values, "", reserved)
}

func validateListenerObject(schema ListenerSchema, values map[string]interface{}, prefix string, reserved map[string]struct{}) error {
	for key, value := range values {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if prefix == "" {
			if _, blocked := reserved[key]; blocked {
				return fmt.Errorf("listener configuration field %q is managed by 3m-ui", key)
			}
			if _, allowed := schema.Fields[key]; !allowed {
				if _, isParent := schema.NestedFields[key]; !isParent {
					return fmt.Errorf("field %q is not supported for listener protocol %q", key, schema.Protocol)
				}
			}
		} else {
			parent := schema.NestedFields[prefix]
			if _, allowed := parent[key]; !allowed {
				return fmt.Errorf("field %q is not supported for listener protocol %q", path, schema.Protocol)
			}
		}

		if _, hasChildren := schema.NestedFields[path]; hasChildren {
			child, ok := value.(map[string]interface{})
			if !ok {
				return fmt.Errorf("field %q must be a JSON object", path)
			}
			if err := validateListenerObject(schema, child, path, reserved); err != nil {
				return err
			}
		}
	}
	return nil
}
