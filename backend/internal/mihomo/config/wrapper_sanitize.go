package config

import "strings"

// sanitizeIncompleteTLSWrappers drops TLS-wrapper blocks that Mihomo would
// reject for missing required nested fields (e.g. jls-config without dest/users).
// Incomplete blocks often come from panel toggles that were saved without filling
// the required sub-fields.
func sanitizeIncompleteTLSWrappers(cfg map[string]interface{}) {
	if cfg == nil {
		return
	}
	if jls, ok := cfg["jls-config"].(map[string]interface{}); ok {
		if !wrapperEnabled(jls) || !hasNonEmptyString(jls, "dest") || !hasJLSUsers(jls) {
			delete(cfg, "jls-config")
		}
	}
	if st, ok := cfg["shadow-tls"].(map[string]interface{}); ok {
		// Minimal: if explicitly disabled or empty object, drop.
		if !wrapperEnabled(st) {
			delete(cfg, "shadow-tls")
		}
	}
	if rt, ok := cfg["res-tls"].(map[string]interface{}); ok {
		if !wrapperEnabled(rt) || !hasNonEmptyString(rt, "dest") {
			delete(cfg, "res-tls")
		}
	}
	// ShadowQUIC jls-upstream requires addr; empty block fails mihomo -t.
	if ju, ok := cfg["jls-upstream"].(map[string]interface{}); ok {
		if !hasNonEmptyString(ju, "addr") {
			delete(cfg, "jls-upstream")
		}
	}
	// allow-insecure alone without cert is valid for anytls only when intentional;
	// do not strip it here.
}

func wrapperEnabled(m map[string]interface{}) bool {
	if m == nil || len(m) == 0 {
		return false
	}
	if en, ok := m["enable"].(bool); ok {
		return en
	}
	if en, ok := m["enabled"].(bool); ok {
		return en
	}
	// Present without enable flag: treat as enabled only if it has meaningful keys.
	for k := range m {
		if k == "enable" || k == "enabled" {
			continue
		}
		return true
	}
	return false
}

func hasNonEmptyString(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) != ""
}

func hasJLSUsers(m map[string]interface{}) bool {
	raw, ok := m["users"]
	if !ok || raw == nil {
		return false
	}
	switch users := raw.(type) {
	case []interface{}:
		for _, item := range users {
			u, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			user, _ := u["username"].(string)
			pass, _ := u["password"].(string)
			if strings.TrimSpace(user) != "" && strings.TrimSpace(pass) != "" {
				return true
			}
		}
	case []map[string]interface{}:
		for _, u := range users {
			user, _ := u["username"].(string)
			pass, _ := u["password"].(string)
			if strings.TrimSpace(user) != "" && strings.TrimSpace(pass) != "" {
				return true
			}
		}
	}
	return false
}
