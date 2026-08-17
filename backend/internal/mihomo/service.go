package mihomo

import (
	"fmt"
	"os"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
)

type Service struct {
	pm *ProcessManager
	cm *ConfigManager
}

func NewService(cfg *config.Config) *Service {
	if cfg == nil {
		return &Service{}
	}
	return &Service{pm: NewProcessManager(cfg.Mihomo.Binary, cfg.Mihomo.Config), cm: NewConfigManager(cfg.Mihomo.Config)}
}

func (s *Service) StartMihomo() error {
	if s == nil || s.pm == nil { return fmt.Errorf("mihomo service not initialized") }
	if err := s.pm.ValidateConfig(); err != nil { return err }
	return s.pm.Start()
}
func (s *Service) StopMihomo() error {
	if s == nil || s.pm == nil { return fmt.Errorf("mihomo service not initialized") }
	return s.pm.Stop()
}
func (s *Service) RestartMihomo() error {
	if s == nil || s.pm == nil { return fmt.Errorf("mihomo service not initialized") }
	return s.pm.Restart()
}
func (s *Service) GetStatus() (*StatusResponse, error) {
	if s == nil || s.pm == nil { return nil, fmt.Errorf("mihomo service not initialized") }
	return s.pm.Status()
}

// SaveConfig writes a candidate configuration and validates it when a Mihomo
// binary is available. If validation fails, the previous file is restored.
func (s *Service) SaveConfig(content string) error {
	if s == nil || s.cm == nil { return fmt.Errorf("mihomo service not initialized") }
	old, readErr := s.cm.ReadConfig()
	if err := s.cm.SaveConfig(content); err != nil { return err }
	if s.pm == nil { return nil }
	if err := s.pm.ValidateConfig(); err != nil {
		if readErr == nil { _ = s.cm.SaveConfig(old) }
		return err
	}
	return nil
}

// ApplyConfig atomically validates and activates a candidate config. A
// backup is written before changing the live file. If restart fails, the
// previous config is restored and Mihomo is restarted from the known-good
// configuration.
func (s *Service) ApplyConfig(content string) error {
	if s == nil || s.cm == nil { return fmt.Errorf("mihomo service not initialized") }
	old, readErr := s.cm.ReadConfig()
	if readErr != nil && !os.IsNotExist(readErr) { return fmt.Errorf("read current Mihomo config: %w", readErr) }
	if err := s.cm.SaveConfig(content); err != nil { return err }
	if s.pm == nil { return nil }
	wasRunning := s.pm.IsRunning()
	if err := s.pm.ValidateConfig(); err != nil {
		if readErr == nil { _ = s.cm.SaveConfig(old) }
		return err
	}
	if !wasRunning { return s.pm.Start() }
	if err := s.pm.Restart(); err != nil {
		if readErr == nil {
			if restoreErr := s.cm.SaveConfig(old); restoreErr == nil { _ = s.pm.Restart() }
		}
		return fmt.Errorf("apply Mihomo configuration: %w", err)
	}
	return nil
}

// RollbackConfig restores the last known configuration saved as <config>.bak
// and applies it through the same validation/restart path.
func (s *Service) RollbackConfig() error {
	if s == nil || s.cm == nil { return fmt.Errorf("mihomo service not initialized") }
	backupPath := s.cm.path + ".bak"
	backup, err := os.ReadFile(backupPath)
	if err != nil { return fmt.Errorf("read Mihomo config backup: %w", err) }
	return s.ApplyConfig(string(backup))
}

func (s *Service) GetLogs() ([]LogResponse, error) {
	if s == nil || s.pm == nil { return nil, fmt.Errorf("mihomo service not initialized") }
	lines := s.pm.Logs()
	result := make([]LogResponse, 0, len(lines))
	for _, line := range lines { result = append(result, LogResponse{Timestamp: time.Now(), Level: "info", Payload: line}) }
	return result, nil
}
