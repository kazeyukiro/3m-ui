package mihomo

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
)

type Service struct {
	pm *ProcessManager
	cm *ConfigManager
	// applyMu serializes configuration replacement, backup rotation, validation,
	// and process restart. Multiple API routes can call ApplyConfig concurrently.
	applyMu sync.Mutex
}

func NewService(cfg *config.Config) *Service {
	if cfg == nil {
		return &Service{}
	}
	return &Service{pm: NewProcessManager(cfg.Mihomo.Binary, cfg.Mihomo.Config), cm: NewConfigManager(cfg.Mihomo.Config)}
}

func (s *Service) StartMihomo() error {
	if s == nil || s.pm == nil {
		return fmt.Errorf("mihomo service not initialized")
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	if err := s.pm.ValidateConfig(); err != nil {
		return err
	}
	return s.pm.Start()
}

func (s *Service) StopMihomo() error {
	if s == nil || s.pm == nil {
		return fmt.Errorf("mihomo service not initialized")
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	return s.pm.Stop()
}

func (s *Service) RestartMihomo() error {
	if s == nil || s.pm == nil {
		return fmt.Errorf("mihomo service not initialized")
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	return s.pm.Restart()
}

func (s *Service) GetStatus() (*StatusResponse, error) {
	if s == nil || s.pm == nil {
		return nil, fmt.Errorf("mihomo service not initialized")
	}
	return s.pm.Status()
}

func (s *Service) SaveConfig(content string) error {
	if s == nil || s.cm == nil {
		return fmt.Errorf("mihomo service not initialized")
	}
	old, readErr := s.cm.ReadConfig()
	if err := s.cm.SaveConfig(content); err != nil {
		return err
	}
	if s.pm == nil {
		return nil
	}
	if err := s.pm.ValidateConfig(); err != nil {
		if readErr == nil {
			_ = s.cm.SaveConfig(old)
		} else {
			_ = os.Remove(s.cm.configPath)
		}
		return err
	}
	return nil
}

func (s *Service) ApplyConfig(content string) error {
	if s == nil || s.cm == nil || s.pm == nil {
		return fmt.Errorf("mihomo service not initialized")
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	old, readErr := s.cm.ReadConfig()
	if err := s.cm.SaveConfig(content); err != nil {
		return err
	}
	if err := s.pm.ValidateConfig(); err != nil {
		if readErr == nil {
			_ = s.cm.SaveConfig(old)
		} else {
			_ = os.Remove(s.cm.configPath)
		}
		return err
	}
	if err := s.pm.Restart(); err != nil {
		if readErr == nil {
			_ = s.cm.SaveConfig(old)
			_ = s.pm.Restart()
		}
		return err
	}
	return nil
}

func (s *Service) ReadConfig() (string, error) {
	if s == nil || s.cm == nil {
		return "", fmt.Errorf("mihomo service not initialized")
	}
	return s.cm.ReadConfig()
}

func (s *Service) ValidateConfig() error {
	if s == nil || s.pm == nil {
		return fmt.Errorf("mihomo service not initialized")
	}
	return s.pm.ValidateConfig()
}

func (s *Service) GetLogs(limit int) ([]string, error) {
	if s == nil || s.pm == nil {
		return nil, fmt.Errorf("mihomo service not initialized")
	}
	return s.pm.Logs(limit)
}

// WaitHealthy polls status until running or timeout.
func (s *Service) WaitHealthy(timeout time.Duration) error {
	if s == nil || s.pm == nil {
		return fmt.Errorf("mihomo service not initialized")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := s.pm.Status()
		if err == nil && st != nil && st.Running {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("mihomo not healthy within %s", timeout)
}
