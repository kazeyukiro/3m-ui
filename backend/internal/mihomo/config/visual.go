package config

import (
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
		Rules: []string{"GEOIP,CN,DIRECT", "MATCH,DIRECT"},
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
	result := db.Where("name = ?", visualConfigName).First(&existing)
	if result.Error == nil {
		existing.Type, existing.Content, existing.Enabled = fragment.Type, fragment.Content, true
		return db.Save(&existing).Error
	}
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}
	return db.Create(&fragment).Error
}
