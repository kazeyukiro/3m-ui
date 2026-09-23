package config

import (
	"encoding/base64"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// applyServerExitFromVisual optionally sets the serving Mihomo process to exit
// via a WARP-style outbound stored in visual-config (user → node → WARP IP).
// Client community rules/groups are never applied here.
func applyServerExitFromVisual(db *gorm.DB, merged map[string]interface{}) error {
	if db == nil || merged == nil {
		if merged != nil {
			merged["rules"] = []interface{}{"MATCH,DIRECT"}
		}
		return nil
	}
	cfg, err := GetVisualConfig(db)
	if err != nil {
		merged["rules"] = []interface{}{"MATCH,DIRECT"}
		return nil
	}
	exits := serverExitProxies(cfg.Proxies)
	if len(exits) == 0 {
		merged["rules"] = []interface{}{"MATCH,DIRECT"}
		return nil
	}

	existing := proxiesAsMaps(merged["proxies"])
	nameSet := map[string]struct{}{}
	for _, m := range existing {
		if n, _ := m["name"].(string); n != "" {
			nameSet[n] = struct{}{}
		}
	}
	var primary string
	for _, p := range exits {
		m, err := normalizeServerExitProxy(p)
		if err != nil {
			return fmt.Errorf("server exit proxy %q: %w", p.Name, err)
		}
		n, _ := m["name"].(string)
		if n == "" {
			continue
		}
		if _, ok := nameSet[n]; ok {
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
	if primary == "" {
		merged["rules"] = []interface{}{"MATCH,DIRECT"}
		return nil
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

func isServerExitProxy(p ProxyEntry) bool {
	typ := strings.ToLower(strings.TrimSpace(p.Type))
	name := strings.ToUpper(strings.TrimSpace(p.Name))
	if typ == "wireguard" {
		return true
	}
	// MASQUE uses different crypto; only promote wireguard to server exit for now.
	if strings.Contains(name, "WARP") && typ == "wireguard" {
		return true
	}
	if strings.Contains(name, "WARP") && typ == "" {
		// type lost on bad round-trip — still try if private-key looks like WG
		return true
	}
	return false
}

func normalizeServerExitProxy(p ProxyEntry) (map[string]interface{}, error) {
	m := proxyEntryToMap(p)
	typ := strings.ToLower(strings.TrimSpace(fmt.Sprint(m["type"])))
	if typ == "" || typ == "masque" {
		// Force wireguard for Cloudflare WARP server exit (MASQUE schema differs).
		m["type"] = "wireguard"
		typ = "wireguard"
	}
	if typ != "wireguard" {
		return nil, fmt.Errorf("unsupported exit type %q (want wireguard)", typ)
	}
	pk := strings.TrimSpace(fmt.Sprint(m["private-key"]))
	if pk == "" || pk == "<nil>" {
		return nil, fmt.Errorf("missing private-key (re-inject WARP)")
	}
	if err := validateWireGuardPrivateKey(pk); err != nil {
		return nil, err
	}
	m["private-key"] = pk
	// Ensure peer public key (Cloudflare WARP account key is well-known).
	if strings.TrimSpace(fmt.Sprint(m["public-key"])) == "" || fmt.Sprint(m["public-key"]) == "<nil>" {
		m["public-key"] = "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo="
	}
	if _, ok := m["udp"]; !ok {
		m["udp"] = true
	}
	if _, ok := m["allowed-ips"]; !ok {
		m["allowed-ips"] = []string{"0.0.0.0/0", "::/0"}
	}
	if srv := strings.TrimSpace(fmt.Sprint(m["server"])); srv == "" || srv == "<nil>" {
		m["server"] = "engage.cloudflareclient.com"
	}
	if m["port"] == nil || fmt.Sprint(m["port"]) == "0" {
		m["port"] = 2408
	}
	return m, nil
}

func validateWireGuardPrivateKey(pk string) error {
	// Accept standard base64 (with padding) of 32 raw bytes — WireGuard/Curve25519.
	raw, err := base64.StdEncoding.DecodeString(pk)
	if err != nil {
		// try raw std without padding
		raw, err = base64.RawStdEncoding.DecodeString(pk)
	}
	if err != nil {
		return fmt.Errorf("private-key is not valid base64 (got %q): %w", truncateKey(pk), err)
	}
	if len(raw) != 32 {
		return fmt.Errorf("private-key must decode to 32 bytes (WireGuard), got %d — re-inject WARP", len(raw))
	}
	return nil
}

func truncateKey(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:8] + "…" + s[len(s)-4:]
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

func serverExitRules(visualRules []string, exitName string) []interface{} {
	if exitName == "" {
		return []interface{}{"MATCH,DIRECT"}
	}
	cnDirect := false
	for _, line := range visualRules {
		u := strings.ToUpper(strings.TrimSpace(line))
		if strings.Contains(u, "GEOIP,CN,DIRECT") {
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
