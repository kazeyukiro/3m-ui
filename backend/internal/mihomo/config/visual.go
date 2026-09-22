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

const visualConfigName = "visual-config"

type VisualConfig struct {
	Mode     string       `json:"mode" yaml:"mode"`
	LogLevel string       `json:"logLevel" yaml:"log-level"`
	AllowLAN bool         `json:"allowLan" yaml:"allow-lan"`
	IPv6     bool         `json:"ipv6" yaml:"ipv6"`
	DNS      VisualDNS    `json:"dns" yaml:"dns"`
	Proxies  []ProxyEntry `json:"proxies" yaml:"proxies"`
	Groups   []GroupEntry `json:"proxyGroups" yaml:"proxy-groups"`
	Rules    []string     `json:"rules" yaml:"rules"`
}

type VisualDNS struct {
	Enable       bool     `json:"enable" yaml:"enable"`
	EnhancedMode string   `json:"enhancedMode" yaml:"enhanced-mode"`
	Listen       string   `json:"listen" yaml:"listen"`
	Nameserver   []string `json:"nameserver" yaml:"nameserver"`
	Fallback     []string `json:"fallback" yaml:"fallback,omitempty"`
}

type ProxyEntry struct {
	Name    string                 `json:"name" yaml:"name"`
	Type    string                 `json:"type" yaml:"type"`
	Server  string                 `json:"server" yaml:"server"`
	Port    interface{}            `json:"port" yaml:"port"`
	Options map[string]interface{} `json:"options,omitempty" yaml:",inline"`
}

// UnmarshalJSON captures unknown top-level JSON fields (cipher, password, uuid,
// etc.) into Options. Go's encoding/json does not honour yaml's `,inline`
// directive, so without this the frontend's flat JSON shape loses protocol
// fields when round-tripping through the visual config API.
func (p *ProxyEntry) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Name, _ = raw["name"].(string)
	p.Type, _ = raw["type"].(string)
	p.Server, _ = raw["server"].(string)
	p.Port = raw["port"]
	delete(raw, "name")
	delete(raw, "type")
	delete(raw, "server")
	delete(raw, "port")
	if len(raw) > 0 {
		p.Options = raw
	}
	return nil
}

// MarshalJSON emits the per-protocol fields stored in Options at the top level
// so the JSON shape matches what the frontend sends (flat object) and what
// mihomo YAML expects after yaml.Marshal (inline keys).
func (p ProxyEntry) MarshalJSON() ([]byte, error) {
	out := map[string]interface{}{
		"name":   p.Name,
		"type":   p.Type,
		"server": p.Server,
		"port":   p.Port,
	}
	for k, v := range p.Options {
		out[k] = v
	}
	return json.Marshal(out)
}

type GroupEntry struct {
	Name     string   `json:"name" yaml:"name"`
	Type     string   `json:"type" yaml:"type"`
	Proxies  []string `json:"proxies" yaml:"proxies"`
	URL      string   `json:"url,omitempty" yaml:"url,omitempty"`
	Interval int      `json:"interval,omitempty" yaml:"interval,omitempty"`
}

func DefaultVisualConfig() VisualConfig {
	return VisualConfig{
		Mode: "rule", LogLevel: "info", AllowLAN: true,
		DNS: VisualDNS{
			Enable: true, EnhancedMode: "fake-ip", Listen: "0.0.0.0:1053",
			Nameserver: []string{"119.29.29.29", "223.5.5.5"},
		},
		Proxies: []ProxyEntry{}, Groups: []GroupEntry{},
		Rules: []string{"MATCH,DIRECT"},
	}
}

func GetVisualConfig(db *gorm.DB) (VisualConfig, error) {
	cfg := DefaultVisualConfig()
	if db == nil {
		return cfg, fmt.Errorf("database is not initialized")
	}
	var fragment models.Config
	err := db.Where("name = ?", visualConfigName).First(&fragment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal([]byte(fragment.Content), &cfg); err != nil {
		return cfg, fmt.Errorf("invalid visual config: %w", err)
	}
	return cfg, nil
}

func SaveVisualConfig(db *gorm.DB, cfg VisualConfig) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch cfg.Mode {
	case "rule", "global", "direct", "script":
	default:
		return fmt.Errorf("invalid mode %q", cfg.Mode)
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.DNS.EnhancedMode == "" {
		cfg.DNS.EnhancedMode = "fake-ip"
	}
	if cfg.DNS.Listen == "" {
		cfg.DNS.Listen = "0.0.0.0:1053"
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	fragment := models.Config{Name: visualConfigName, Type: "visual", Content: string(data), Enabled: true}
	var existing models.Config
	// Include soft-deleted rows: UNIQUE(name) still applies to them.
	result := db.Unscoped().Where("name = ?", visualConfigName).First(&existing)
	if result.Error == nil {
		existing.Type, existing.Content, existing.Enabled = fragment.Type, fragment.Content, true
		existing.DeletedAt = gorm.DeletedAt{}
		return db.Unscoped().Save(&existing).Error
	}
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}
	return db.Create(&fragment).Error
}

// InjectWARPProxy merges a WARP outbound (from register YAML) into visual proxies.
// ruleMode: ""|"none" — no rule change; "match" — append MATCH,<name> (before last MATCH if any);
// "cn_direct" — ensure GEOIP,CN,DIRECT then MATCH,<name>.
func InjectWARPProxy(db *gorm.DB, proxyYAML string, proxyName string, ruleMode string) (VisualConfig, error) {
	cfg, err := GetVisualConfig(db)
	if err != nil {
		return cfg, err
	}
	var frag map[string]interface{}
	if err := yaml.Unmarshal([]byte(proxyYAML), &frag); err != nil {
		return cfg, fmt.Errorf("parse WARP yaml: %w", err)
	}
	// Accept either {proxies:[{...}]} or a single proxy map, or full config snippet.
	var proxyMap map[string]interface{}
	if list, ok := frag["proxies"].([]interface{}); ok && len(list) > 0 {
		if m, ok := list[0].(map[string]interface{}); ok {
			proxyMap = m
		}
	} else if frag["type"] != nil && frag["name"] != nil {
		proxyMap = frag
	}
	if proxyMap == nil {
		return cfg, fmt.Errorf("WARP yaml has no proxies entry")
	}
	name := proxyName
	if name == "" {
		if n, ok := proxyMap["name"].(string); ok && n != "" {
			name = n
		} else {
			name = "WARP-OUT"
		}
	}
	proxyMap["name"] = name
	typ, _ := proxyMap["type"].(string)
	server, _ := proxyMap["server"].(string)
	port := proxyMap["port"]
	opts := map[string]interface{}{}
	for k, v := range proxyMap {
		if k == "name" || k == "type" || k == "server" || k == "port" {
			continue
		}
		opts[k] = v
	}
	entry := ProxyEntry{Name: name, Type: typ, Server: server, Port: port, Options: opts}
	// Replace existing same name
	replaced := false
	for i, p := range cfg.Proxies {
		if p.Name == name {
			cfg.Proxies[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Proxies = append(cfg.Proxies, entry)
	}

	ruleMode = strings.ToLower(strings.TrimSpace(ruleMode))
	switch ruleMode {
	case "", "none":
		// leave rules
	case "match":
		cfg.Rules = appendRulePreferLastMatch(cfg.Rules, "MATCH,"+name)
	case "cn_direct":
		cfg.Rules = []string{"GEOIP,CN,DIRECT", "MATCH," + name}
	default:
		return cfg, fmt.Errorf("invalid rule_mode %q", ruleMode)
	}
	if err := SaveVisualConfig(db, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func appendRulePreferLastMatch(rules []string, matchRule string) []string {
	out := make([]string, 0, len(rules)+1)
	inserted := false
	for _, r := range rules {
		tr := strings.TrimSpace(r)
		if strings.HasPrefix(strings.ToUpper(tr), "MATCH,") {
			if !inserted {
				out = append(out, matchRule)
				inserted = true
			}
			continue // drop old MATCH lines; single final MATCH
		}
		if tr != "" {
			out = append(out, tr)
		}
	}
	if !inserted {
		out = append(out, matchRule)
	}
	return out
}
