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
// listener — server-side Mihomo proxies, groups, and rules after traffic hits listeners.
// RuleProvider is one Mihomo rule-providers entry
// (https://wiki.metacubex.one/config/rule-providers/).
type RuleProvider struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`                // http | file | inline
	Behavior  string   `json:"behavior"`            // domain | ipcidr | classical
	Format    string   `json:"format,omitempty"`    // yaml | text | mrs (default yaml)
	URL       string   `json:"url,omitempty"`       // required when type=http
	Path      string   `json:"path,omitempty"`      // under Mihomo -d home; auto if empty
	Interval  int      `json:"interval,omitempty"`  // seconds; http update interval
	Proxy     string   `json:"proxy,omitempty"`     // download via this outbound (e.g. DIRECT)
	Payload   []string `json:"payload,omitempty"`   // inline only
	SizeLimit int      `json:"sizeLimit,omitempty"` // bytes; 0 = unlimited
}

type ServerRoutingConfig struct {
	Proxies       []ProxyEntry   `json:"proxies" yaml:"proxies"`
	Groups        []GroupEntry   `json:"proxyGroups" yaml:"proxy-groups"`
	Rules         []string       `json:"rules" yaml:"rules"`
	WarpDomains   []string       `json:"warpDomains" yaml:"warp-domains,omitempty"`
	RuleProviders []RuleProvider `json:"ruleProviders" yaml:"rule-providers,omitempty"`
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
	// Managed WARP domain rules first .
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
		upper := strings.ToUpper(d)
		switch {
		case strings.HasPrefix(upper, "GEOSITE:"):
			code := strings.TrimSpace(d[8:])
			if code != "" {
				rules = append(rules, "GEOSITE,"+code+","+warpName)
			}
		case strings.HasPrefix(upper, "DOMAIN,"), strings.HasPrefix(upper, "DOMAIN-SUFFIX,"), strings.HasPrefix(upper, "DOMAIN-KEYWORD,"):
			if strings.Count(d, ",") >= 2 {
				rules = append(rules, d)
			} else {
				rules = append(rules, d+","+warpName)
			}
		default:
			d = strings.TrimPrefix(d, ".")
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
	applyRuleProviders(merged, sr.RuleProviders)
}

// applyRuleProviders writes top-level rule-providers per MetaCubeX wiki.
func applyRuleProviders(merged map[string]interface{}, providers []RuleProvider) {
	if len(providers) == 0 {
		delete(merged, "rule-providers")
		return
	}
	out := make(map[string]interface{}, len(providers))
	for _, p := range providers {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(p.Type))
		if typ == "" {
			typ = "http"
		}
		behavior := strings.ToLower(strings.TrimSpace(p.Behavior))
		if behavior == "" {
			behavior = "classical"
		}
		format := strings.ToLower(strings.TrimSpace(p.Format))
		if format == "" {
			format = "yaml"
		}
		entry := map[string]interface{}{
			"type":     typ,
			"behavior": behavior,
		}
		if format != "yaml" || typ == "http" || typ == "file" {
			entry["format"] = format
		}
		switch typ {
		case "http":
			if u := strings.TrimSpace(p.URL); u != "" {
				entry["url"] = u
			}
			path := strings.TrimSpace(p.Path)
			if path == "" {
				// Relative to Mihomo -d home (SAFE_PATHS). Official docs allow omitting path
				// (url MD5 filename); we pin a stable path for operators.
				ext := format
				if ext == "" {
					ext = "yaml"
				}
				path = "./rule-providers/" + name + "." + ext
			}
			entry["path"] = path
			if p.Interval > 0 {
				entry["interval"] = p.Interval
			} else {
				entry["interval"] = 86400
			}
			if px := strings.TrimSpace(p.Proxy); px != "" {
				entry["proxy"] = px
			} else {
				entry["proxy"] = "DIRECT"
			}
			if p.SizeLimit > 0 {
				entry["size-limit"] = p.SizeLimit
			}
		case "file":
			path := strings.TrimSpace(p.Path)
			if path == "" {
				path = "./rule-providers/" + name + ".yaml"
			}
			entry["path"] = path
		case "inline":
			if len(p.Payload) > 0 {
				entry["payload"] = p.Payload
			}
		}
		out[name] = entry
	}
	if len(out) == 0 {
		delete(merged, "rule-providers")
		return
	}
	merged["rule-providers"] = out
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
