package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultJWTSecret     = "3m-ui-default-jwt-secret-key-change-it-in-production"
	DefaultCredentialKey = "3m-ui-default-credential-key-change-it-in-production"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Security SecurityConfig `yaml:"security"`
	Mihomo   MihomoConfig   `yaml:"mihomo"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
	// Listen is the bind address for the panel HTTP server.
	// Empty / "0.0.0.0" / "::" → dual-stack friendly ":port" (Linux).
	// Use a concrete IPv4 or IPv6 to force a family (e.g. "127.0.0.1", "::1").
	Listen    string `yaml:"listen"`
	Mode      string `yaml:"mode"`
	PublicURL string `yaml:"public_url"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
}

type MihomoConfig struct {
	Binary string `yaml:"binary"`
	Config string `yaml:"config"`
}

var GlobalConfig *Config

func IsMihomoListenerProtocol(protocol string) bool {
	switch protocol {
	case "shadowsocks", "snell", "vmess", "vless", "trojan", "hysteria2", "hysteria2-realm", "tuic", "shadowquic", "anytls", "mieru", "sudoku", "trusttunnel":
		return true
	default:
		return false
	}
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config YAML: %w", err)
	}
	ApplyEnvOverrides(&cfg)
	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	GlobalConfig = &cfg
	return &cfg, nil
}

// ApplyEnvOverrides applies NAT-friendly environment overrides after YAML load.
// Supported:
//   THREE_M_UI_PORT / PANEL_PORT     → server.port
//   THREE_M_UI_LISTEN / PANEL_LISTEN → server.listen
//   THREE_M_UI_PUBLIC_URL / PUBLIC_URL → server.public_url
func ApplyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	for _, key := range []string{"THREE_M_UI_PORT", "PANEL_PORT"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			if p, err := strconv.Atoi(v); err == nil && p >= 1 && p <= 65535 {
				cfg.Server.Port = p
			}
			break
		}
	}
	for _, key := range []string{"THREE_M_UI_LISTEN", "PANEL_LISTEN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			cfg.Server.Listen = v
			break
		}
	}
	for _, key := range []string{"THREE_M_UI_PUBLIC_URL", "PUBLIC_URL"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			cfg.Server.PublicURL = v
			break
		}
	}
}

// ConfigPath returns the active config file path (env or common defaults).
func ConfigPath() string {
	if value := strings.TrimSpace(os.Getenv("THREE_M_UI_CONFIG")); value != "" {
		return value
	}
	for _, candidate := range []string{"/etc/3m-ui/config.yaml", "config/config.yaml", "backend/config/config.yaml"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "config/config.yaml"
}

// UpdateServerFile writes server.port / listen / public_url into the YAML config
// on disk. publicURL is written when setPublic is true (including empty to clear).
func UpdateServerFile(path string, port int, listen, publicURL string, setPublic bool) error {
	if path == "" {
		path = ConfigPath()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var root map[string]interface{}
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if root == nil {
		root = map[string]interface{}{}
	}
	server, _ := root["server"].(map[string]interface{})
	if server == nil {
		server = map[string]interface{}{}
	}
	if port >= 1 && port <= 65535 {
		server["port"] = port
	}
	if listen != "" {
		server["listen"] = listen
	}
	if setPublic {
		if publicURL == "" {
			delete(server, "public_url")
		} else {
			server["public_url"] = publicURL
		}
	}
	root["server"] = server
	out, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}



// Validate rejects insecure placeholder secrets and malformed server settings
// before they can be used to authenticate or encrypt stored credentials.
func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.Database.Path) == "" {
		return fmt.Errorf("database.path is required")
	}
	if secret := strings.TrimSpace(cfg.JWT.Secret); secret == "" || secret == DefaultJWTSecret {
		return fmt.Errorf("jwt.secret must be set to a unique secret; the default placeholder is not allowed")
	} else if len([]byte(secret)) < 32 {
		return fmt.Errorf("jwt.secret must be at least 32 bytes")
	}
	if key := strings.TrimSpace(cfg.Security.CredentialKey); key == "" || key == DefaultCredentialKey {
		return fmt.Errorf("security.credential_key must be set to a unique secret; the default placeholder is not allowed")
	} else if len([]byte(key)) < 32 {
		return fmt.Errorf("security.credential_key must be at least 32 bytes")
	}
	return nil
}

type SecurityConfig struct {
	CredentialKey string   `yaml:"credential_key"`
	CORSOrigins   []string `yaml:"cors_origins"`
}
