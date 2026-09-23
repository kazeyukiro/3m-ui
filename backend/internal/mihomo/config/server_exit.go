package config

import (
	"strings"

	"gorm.io/gorm"
)

// applyServerExitFromVisual optionally sets the serving Mihomo process to exit
// via a WARP-style outbound stored in visual-config (user → node → WARP IP).
// Client community rules/groups are never applied here.
func applyServerExitFromVisual(db *gorm.DB, merged map[string]interface{}) error {
	if db == nil || merged == nil {
		merged["rules"] = []interface{}{"MATCH,DIRECT"}
		return nil
	}
	cfg, err := GetVisualConfig(db)
	if err != nil {
		merged["rules"] = []interface{}{"MATCH,DIRECT"}
		return nil // soft: keep DIRECT if visual missing
	}
	exits := serverExitProxies(cfg.Proxies)
	if len(exits) == 0 {
		merged["rules"] = []interface{}{"MATCH,DIRECT"}
		return nil
	}

	// Merge into proxies list (append; do not wipe unrelated fragment proxies).
	existing := proxiesAsMaps(merged["proxies"])
	nameSet := map[string]struct{}{}
	for _, m := range existing {
		if n, _ := m["name"].(string); n != "" {
			nameSet[n] = struct{}{}
		}
	}
	var primary string
	for _, p := range exits {
		m := proxyEntryToMap(p)
		n, _ := m["name"].(string)
		if n == "" {
			continue
		}
		if _, ok := nameSet[n]; ok {
			// replace same name
			for i, em := range existing {
				if en, _ := em["name"].(string); en == n {
					existing[i] = m
					break
				}
			}
		} else {
			existing = append(existing, m)
			nameSet[n] = struct{}{}
		}
		if primary == "" {
			primary = n
		}
	}
	merged["proxies"] = existing
	merged["rules"] = serverExitRules(cfg.Rules, primary)
	return nil
}

func serverExitProxies(list []ProxyEntry) []ProxyEntry {
	var out []ProxyEntry
	for _, p := range list {
		if isServerExitProxy(p) {
			out = append(out, p)
		}
	}
	return out
}

// WARP WireGuard / MASQUE-style outbounds intended for panel host exit.
func isServerExitProxy(p ProxyEntry) bool {
	typ := strings.ToLower(strings.TrimSpace(p.Type))
	name := strings.ToUpper(strings.TrimSpace(p.Name))
	if typ == "wireguard" {
		return true
	}
	// Future-proof: names from InjectWARPProxy
	if strings.Contains(name, "WARP") {
		return true
	}
	return false
}

func proxyEntryToMap(p ProxyEntry) map[string]interface{} {
	m := map[string]interface{}{
		"name":   p.Name,
		"type":   p.Type,
		"server": p.Server,
		"port":   p.Port,
	}
	for k, v := range p.Options {
		m[k] = v
	}
	return m
}

func proxiesAsMaps(v interface{}) []map[string]interface{} {
	out := []map[string]interface{}{}
	switch t := v.(type) {
	case []map[string]interface{}:
		return append(out, t...)
	case []interface{}:
		for _, item := range t {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

// Prefer CN direct + MATCH exit when visual already encodes that; else MATCH,exit.
func serverExitRules(visualRules []string, exitName string) []interface{} {
	if exitName == "" {
		return []interface{}{"MATCH,DIRECT"}
	}
	cnDirect := false
	for _, line := range visualRules {
		u := strings.ToUpper(strings.TrimSpace(line))
		if strings.Contains(u, "GEOIP,CN,DIRECT") || strings.Contains(u, "GEOIP,CN ,DIRECT") {
			cnDirect = true
			break
		}
	}
	if cnDirect {
		return []interface{}{
			"GEOIP,CN,DIRECT,no-resolve",
			"MATCH," + exitName,
		}
	}
	return []interface{}{"MATCH," + exitName}
}
