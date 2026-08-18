package mihomo

import (
	"fmt"
	"os"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
)

type Service struct { pm *ProcessManager; cm *ConfigManager }

func NewService(cfg *config.Config) *Service {
	if cfg == nil { return &Service{} }
	return &Service{pm: NewProcessManager(cfg.Mihomo.Binary, cfg.Mihomo.Config), cm: NewConfigManager(cfg.Mihomo.Config)}
}
func (s *Service) StartMihomo() error { if s == nil || s.pm == nil { return fmt.Errorf("mihomo service not initialized") }; if err := s.pm.ValidateConfig(); err != nil { return err }; return s.pm.Start() }
func (s *Service) StopMihomo() error { if s == nil || s.pm == nil { return fmt.Errorf("mihomo service not initialized") }; return s.pm.Stop() }
func (s *Service) RestartMihomo() error { if s == nil || s.pm == nil { return fmt.Errorf("mihomo service not initialized") }; return s.pm.Restart() }
func (s *Service) GetStatus() (*StatusResponse, error) { if s == nil || s.pm == nil { return nil, fmt.Errorf("mihomo service not initialized") }; return s.pm.Status() }

func (s *Service) SaveConfig(content string) error {
	if s == nil || s.cm == nil { return fmt.Errorf("mihomo service not initialized") }
	old, readErr := s.cm.ReadConfig()
	if err := s.cm.SaveConfig(content); err != nil { return err }
	if s.pm == nil { return nil }
	if err := s.pm.ValidateConfig(); err != nil { if readErr == nil { _ = s.cm.SaveConfig(old) }; return err }
	return nil
}

// ApplyConfig validates and activates a candidate configuration. Before the
// live file is changed, the current file is copied to <config>.bak. If
// validation or start/restart fails, the previous configuration is restored.
func (s *Service) ApplyConfig(content string) error {
	if s == nil || s.cm == nil { return fmt.Errorf("mihomo service not initialized") }
	old, readErr := s.cm.ReadConfig()
	if readErr != nil && !os.IsNotExist(readErr) { return fmt.Errorf("read current Mihomo config: %w", readErr) }
	if readErr == nil {
		if err := os.WriteFile(s.cm.configPath+".bak", []byte(old), 0600); err != nil { return fmt.Errorf("backup Mihomo config: %w", err) }
	}
	if err := s.cm.SaveConfig(content); err != nil { return err }
	if s.pm == nil { return nil }
	wasRunning := s.pm.IsRunning()
	if err := s.pm.ValidateConfig(); err != nil {
		if readErr == nil { _ = s.cm.SaveConfig(old) }
		return fmt.Errorf("validate Mihomo configuration: %w", err)
	}
	if !wasRunning {
		if err := s.pm.Start(); err != nil {
			if readErr == nil {
				if restoreErr := s.cm.SaveConfig(old); restoreErr != nil {
					return fmt.Errorf("start Mihomo: %v; restore previous config: %w", err, restoreErr)
				}
			}
			return fmt.Errorf("start Mihomo: %w", err)
		}
		return nil
	}
	if err := s.pm.Restart(); err != nil {
		if readErr == nil {
			if restoreErr := s.cm.SaveConfig(old); restoreErr == nil { _ = s.pm.Restart() }
		}
		return fmt.Errorf("apply Mihomo configuration: %w", err)
	}
	return nil
}

func (s *Service) RollbackConfig() error {
	if s == nil || s.cm == nil { return fmt.Errorf("mihomo service not initialized") }
	backup, err := os.ReadFile(s.cm.configPath + ".bak")
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
