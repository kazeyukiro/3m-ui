package mihomo

import (
	"errors"
	"fmt"
	"os"
	"strings"
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

// SetCrashHandler forwards a crash-notification callback to the underlying
// ProcessManager. Safe to call before Start(); a nil service or pm is a
// no-op so callers (e.g. app/container.go) can wire unconditionally after
// NewService without guarding against the cfg==nil degenerate service.
func (s *Service) SetCrashHandler(fn func(exitErr error)) {
	if s == nil || s.pm == nil {
		return
	}
	s.pm.SetCrashHandler(fn)
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
	hadOld := readErr == nil
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) && !os.IsNotExist(readErr) {
		return fmt.Errorf("read current Mihomo config: %w", readErr)
	}
	if err := s.cm.SaveConfig(content); err != nil {
		return err
	}
	if s.pm == nil {
		return nil
	}
	if err := s.pm.ValidateConfig(); err != nil {
		if hadOld {
			_ = s.cm.SaveConfig(old)
		} else {
			_ = os.Remove(s.cm.configPath)
		}
		return err
	}
	return nil
}

// ApplyConfig validates and activates a candidate configuration. Before the
// live file is changed, the current file is copied to <config>.bak. If
// validation or start/restart fails, the previous configuration is restored.
func (s *Service) ApplyConfig(content string) error {
	if s == nil || s.cm == nil {
		return fmt.Errorf("mihomo service not initialized")
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	old, readErr := s.cm.ReadConfig()
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) && !os.IsNotExist(readErr) {
		return fmt.Errorf("read current Mihomo config: %w", readErr)
	}
	if readErr == nil {
		if err := os.WriteFile(s.cm.configPath+".bak", []byte(old), 0600); err != nil {
			return fmt.Errorf("backup Mihomo config: %w", err)
		}
	}
	if err := s.cm.SaveConfig(content); err != nil {
		return err
	}
	if s.pm == nil {
		return nil
	}
	wasRunning := s.pm.IsRunning()
	if err := s.pm.ValidateConfig(); err != nil {
		if readErr == nil {
			_ = s.cm.SaveConfig(old)
		} else {
			_ = os.Remove(s.cm.configPath)
		}
		return fmt.Errorf("validate Mihomo configuration: %w", err)
	}
	if !wasRunning {
		if err := s.pm.Start(); err != nil {
			if readErr == nil {
				if restoreErr := s.cm.SaveConfig(old); restoreErr != nil {
					return fmt.Errorf("start Mihomo: %v; restore previous config: %w", err, restoreErr)
				}
			} else {
				_ = os.Remove(s.cm.configPath)
			}
			return fmt.Errorf("start Mihomo: %w", err)
		}
		return nil
	}
	if err := s.pm.Restart(); err != nil {
		if readErr == nil {
			if restoreErr := s.cm.SaveConfig(old); restoreErr != nil {
				return fmt.Errorf("apply Mihomo configuration: %v; restore previous config: %w", err, restoreErr)
			}
			if restartErr := s.pm.Restart(); restartErr != nil {
				return fmt.Errorf("apply Mihomo configuration: %v; restored previous config but restart also failed: %w", err, restartErr)
			}
		}
		return fmt.Errorf("apply Mihomo configuration: %w", err)
	}
	return nil
}

func (s *Service) RollbackConfig() error {
	if s == nil || s.cm == nil {
		return fmt.Errorf("mihomo service not initialized")
	}
	backup, err := os.ReadFile(s.cm.configPath + ".bak")
	if err != nil {
		return fmt.Errorf("read Mihomo config backup: %w", err)
	}
	return s.ApplyConfig(string(backup))
}

func (s *Service) GetLogs() ([]LogResponse, error) {
	if s == nil || s.pm == nil {
		return nil, fmt.Errorf("mihomo service not initialized")
	}
	lines := s.pm.Logs()
	result := make([]LogResponse, 0, len(lines))
	for _, line := range lines {
		ts, payload := parseStoredLogLine(line)
		result = append(result, LogResponse{
			Timestamp: ts,
			Level:     inferLogLevel(payload),
			Payload:   payload,
		})
	}
	return result, nil
}

// parseStoredLogLine extracts the RFC3339 timestamp prefix that
// appendLogLocked writes at the start of every stored log line. The stored
// format is "<RFC3339> <line>". When the prefix parses as a valid RFC3339
// timestamp, it is returned as `ts` and the remainder (after the separating
// space) is returned as `payload`. When the line does not start with a
// parseable timestamp, time.Now() is used as a fallback (matching the
// original behaviour) and the whole line becomes the payload.
func parseStoredLogLine(line string) (time.Time, string) {
	// RFC3339 timestamps are at least 20 characters ("2006-01-02T15:04:05Z")
	// and are followed by a single space before the actual log payload.
	idx := strings.IndexByte(line, ' ')
	if idx < 20 {
		return time.Now(), line
	}
	ts, err := time.Parse(time.RFC3339, line[:idx])
	if err != nil {
		return time.Now(), line
	}
	return ts, strings.TrimPrefix(line[idx+1:], " ")
}

// inferLogLevel maps common mihomo log prefixes to a log level. mihomo emits
// lines such as "[INFO] ...", "INFO[0001] ...", "ERROR ...", "WARN ...", or
// "DEBUG ...". Returns "info" (the historical default) when no known prefix
// is found.
func inferLogLevel(payload string) string {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return "info"
	}
	upper := strings.ToUpper(strings.TrimPrefix(trimmed, "["))
	switch {
	case strings.HasPrefix(upper, "ERROR"), strings.HasPrefix(upper, "FATAL"):
		return "error"
	case strings.HasPrefix(upper, "WARN"):
		return "warning"
	case strings.HasPrefix(upper, "DEBUG"):
		return "debug"
	case strings.HasPrefix(upper, "INFO"):
		return "info"
	}
	return "info"
}
