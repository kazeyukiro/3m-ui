package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = "/etc/3m-ui/config.yaml"

// EnsureConfig initializes a missing configuration without replacing existing
// settings or encryption keys. Both the installer and container use this path.
func EnsureConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	dataDir := strings.TrimSpace(os.Getenv("THREE_M_UI_DATA_DIR"))
	if dataDir == "" {
		dataDir = "/var/lib/3m-ui"
	}
	binary := strings.TrimSpace(os.Getenv("THREE_M_UI_MIHOMO_BINARY"))
	if binary == "" {
		binary = "/usr/local/lib/3m-ui/mihomo"
	}
	secret, err := randomSecret()
	if err != nil {
		return err
	}
	credentialKey, err := randomSecret()
	if err != nil {
		return err
	}
	cfg := Config{
		Server:   ServerConfig{Port: 8080, Mode: "release"},
		Database: DatabaseConfig{Path: filepath.Join(dataDir, "3m-ui.db")},
		JWT:      JWTConfig{Secret: secret},
		Security: SecurityConfig{CredentialKey: credentialKey},
		Mihomo:   MihomoConfig{Binary: binary, Config: filepath.Join(dataDir, "mihomo", "config.yaml")},
	}
	ApplyEnvOverrides(&cfg)
	if err := Validate(&cfg); err != nil {
		return err
	}
	var raw bytes.Buffer
	encoder := yaml.NewEncoder(&raw)
	encoder.SetIndent(2)
	if err := encoder.Encode(&cfg); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	// A same-directory temporary file and exclusive link avoid publishing partial
	// YAML or replacing a configuration created by another initial startup.
	tmp, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmp.Name(), path); err != nil && !os.IsExist(err) {
		return fmt.Errorf("create configuration: %w", err)
	}
	return nil
}

func randomSecret() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate configuration secret: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
