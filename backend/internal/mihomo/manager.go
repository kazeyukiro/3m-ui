package mihomo

import (
	"fmt"
	"os"
	"path/filepath"
)

type ConfigManager struct{ configPath string }

func NewConfigManager(path string) *ConfigManager { return &ConfigManager{configPath: path} }

// SaveConfig atomically replaces the Mihomo YAML configuration. The old
// configuration is kept until the new file has been fully written and synced.
func (cm *ConfigManager) SaveConfig(content string) error {
	if cm == nil || cm.configPath == "" {
		return fmt.Errorf("mihomo config path is empty")
	}
	if content == "" {
		return fmt.Errorf("mihomo config is empty")
	}
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create mihomo config directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config.yaml.tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary mihomo config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set mihomo config permissions: %w", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write mihomo config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync mihomo config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close mihomo config: %w", err)
	}
	if err := os.Rename(tmpName, cm.configPath); err != nil {
		return fmt.Errorf("replace mihomo config: %w", err)
	}
	return nil
}

func (cm *ConfigManager) ReadConfig() (string, error) {
	if cm == nil || cm.configPath == "" {
		return "", fmt.Errorf("mihomo config path is empty")
	}
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		// Preserve os.IsNotExist so callers (ApplyConfig first-boot) can
		// distinguish a missing file from other I/O failures.
		return "", err
	}
	return string(data), nil
}
