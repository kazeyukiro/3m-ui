package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// serverRoutingName is the panel_settings / config fragment key for *serving*
// Mihomo rules (post-inbound egress). Distinct from visual-config, which only
// shapes client subscription YAML.
const serverRoutingName = "server-routing"

// ServerRoutingConfig controls how traffic leaves the VPS after hitting a
// listener — the Mihomo equivalent of 3x-ui's Xray outbounds + routing rules.
type ServerRoutingConfig struct {
	Proxies     []ProxyEntry `json:"proxies" yaml:"proxies"`
	Groups      []GroupEntry `json:"proxyGroups" yaml:"proxy-groups"`
	Rules       []string     `json:"rules" yaml:"rules"`
	WarpDomains []string     `json:"warpDomains" yaml:"warp-domains,omitempty"`
}

// DefaultServerRouting is pure direct egress (historical 3m-ui behaviour).
func DefaultServerRouting() ServerRoutingConfig {
	return ServerRoutingConfig{
		Proxies: nil,
		Groups:  nil,
		Rules:   []string{"MATCH,DIRECT"},
	}
}

func GetServerRouting(db *gorm.DB) (ServerRoutingConfig, error) {
	cfg := DefaultServerRouting()
	if db == nil {
		return cfg, nil
	}
	var row models.PanelSetting
	err := db.Where("key = ?", serverRoutingName).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Legacy: some installs may only have rules in a Config fragment.
		var frag models.Config
		if e2 := db.Where("name = ?", serverRoutingName).First(&frag).Error; e2 == nil && strings.TrimSpace(frag.Content) != "" {
			if e3 := yaml.Unmarshal([]byte(frag.Content), &cfg); e3 == nil {
				normalizeServerRouting(&cfg)
				return cfg, nil
			}
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if strings.TrimSpace(row.Value) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(row.Value), &cfg); err != nil {
		return cfg, fmt.Errorf("parse server-routing: %w", err)
	}
	normalizeServerRouting(&cfg)
	return cfg, nil
}

func SaveServerRouting(db *gorm.DB, cfg ServerRoutingConfig) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	normalizeServerRouting(&cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	var row models.PanelSetting
	err = db.Where("key = ?", serverRoutingName).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&models.PanelSetting{Key: serverRoutingName, Value: string(raw)}).Error
	}
	if err != nil {
		return err
	}
	row.Value = string(raw)
	return db.Save(&row).Error
}

func normalizeServerRouting(cfg *ServerRoutingConfig) {
	if cfg == nil {
		return
	}
	if len(cfg.Rules) == 0 {
		cfg.Rules = []string{"MATCH,DIRECT"}
		return
	}
	hasMatch := false
	for _, r := range cfg.Rules {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(r)), "MATCH,") {
			hasMatch = true
			break
		}
	}
	if !hasMatch {
		cfg.Rules = append(cfg.Rules, "MATCH,DIRECT")
	}
}

// applyServerRouting writes proxies / groups / rules into the serving config map.
// warpProxy, when non-nil, is injected (or replaces same-name entry) as the WARP outbound.
func applyServerRouting(merged map[string]interface{}, sr ServerRoutingConfig, warpProxy map[string]interface{}) {
	normalizeServerRouting(&sr)
	proxies := proxyEntriesToYAML(sr.Proxies)
	if warpProxy != nil {
		name, _ := warpProxy["name"].(string)
		if name == "" {
			name = "WARP"
			warpProxy["name"] = name
		}
		// Replace existing same-name proxy if present.
		out := make([]interface{}, 0, len(proxies)+1)
		replaced := false
		for _, p := range proxies {
			m, ok := p.(map[string]interface{})
			if ok {
				if n, _ := m["name"].(string); n == name {
					out = append(out, warpProxy)
					replaced = true
					continue
				}
			}
			out = append(out, p)
		}
		if !replaced {
			out = append(out, warpProxy)
		}
		proxies = out
	}
	if len(proxies) > 0 {
		merged["proxies"] = proxies
	}
	if len(sr.Groups) > 0 {
		merged["proxy-groups"] = groupEntriesToYAML(sr.Groups)
	} else {
		merged["proxy-groups"] = []interface{}{}
	}
	rules := make([]interface{}, 0, len(sr.Rules)+len(sr.WarpDomains)+4)
	// Managed WARP domain rules first (m-ui style).
	warpName := "WARP"
	if warpProxy != nil {
		if n, _ := warpProxy["name"].(string); n != "" {
			warpName = n
		}
	}
	for _, d := range sr.WarpDomains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		// Allow GEOSITE:xxx or DOMAIN-SUFFIX style payloads.
		upper := strings.ToUpper(d)
		if strings.HasPrefix(upper, "GEOSITE:") {
			rules = append(rules, "GEOSITE,"+strings.TrimSpace(d[8:])+","+warpName)
		} else if strings.HasPrefix(upper, "DOMAIN,") || strings.HasPrefix(upper, "DOMAIN-SUFFIX,") {
			rules = append(rules, d+","+warpName)
		} else {
			rules = append(rules, "DOMAIN-SUFFIX,"+d+","+warpName)
		}
	}
	for _, r := range sr.Rules {
		r = strings.TrimSpace(r)
		if r != "" {
			rules = append(rules, r)
		}
	}
	if len(rules) == 0 {
		rules = []interface{}{"MATCH,DIRECT"}
	}
	merged["rules"] = rules
}

func proxyEntriesToYAML(entries []ProxyEntry) []interface{} {
	out := make([]interface{}, 0, len(entries))
	for _, p := range entries {
		m := map[string]interface{}{
			"name":   p.Name,
			"type":   p.Type,
			"server": p.Server,
			"port":   p.Port,
		}
		for k, v := range p.Options {
			m[k] = v
		}
		out = append(out, m)
	}
	return out
}

func groupEntriesToYAML(entries []GroupEntry) []interface{} {
	out := make([]interface{}, 0, len(entries))
	for _, g := range entries {
		m := map[string]interface{}{
			"name": g.Name,
			"type": g.Type,
		}
		if len(g.Proxies) > 0 {
			m["proxies"] = g.Proxies
		}
		if g.URL != "" {
			m["url"] = g.URL
		}
		if g.Interval > 0 {
			m["interval"] = g.Interval
		}
		out = append(out, m)
	}
	return out
}
