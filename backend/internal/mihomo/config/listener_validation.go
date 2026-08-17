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
	for key := range values {
		if _, blocked := reserved[key]; blocked {
			return fmt.Errorf("listener configuration field %q is managed by 3m-ui", key)
		}
		if _, allowed := schema.Fields[key]; allowed {
			continue
		}
		if _, allowed := schema.NestedFields[key]; allowed {
			continue
		}
		return fmt.Errorf("field %q is not supported for listener protocol %q", key, protocol)
	}
	return nil
}
