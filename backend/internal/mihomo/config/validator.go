package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidateConfigYAML parses a YAML string and performs sanity checks on standard fields
func ValidateConfigYAML(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("configuration cannot be empty")
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		return fmt.Errorf("invalid YAML syntax: %w", err)
	}

	// Validate standard fields if present
	if mode, exists := raw["mode"]; exists {
		modeStr, ok := mode.(string)
		if !ok {
			return fmt.Errorf("field 'mode' must be a string")
		}
		modeStr = strings.ToLower(modeStr)
		if modeStr != "rule" && modeStr != "global" && modeStr != "direct" && modeStr != "script" {
			return fmt.Errorf("field 'mode' must be one of: rule, global, direct, script")
		}
	}

	if dns, exists := raw["dns"]; exists {
		if _, ok := dns.(map[string]interface{}); !ok {
			return fmt.Errorf("field 'dns' must be a structured map")
		}
	}

	return nil
}
