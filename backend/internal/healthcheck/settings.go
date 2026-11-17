package healthcheck

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

// settingKey is the PanelSetting row that stores the tunnel health monitor
// configuration as JSON. It intentionally contains no secret, so the generic
// panel-settings endpoint would also expose it; the dedicated handler is used
// anyway so settings changes can be applied to the running monitor.
const settingKey = "tunnel_health"

// Defaults mirror the documented 3x-ui tunnel health monitor semantics:
// the watchdog is opt-in because a core restart drops every connected client.
const (
	DefaultURL      = "https://www.cloudflare.com/cdn-cgi/trace"
	DefaultInterval = 30 // seconds
	DefaultTimeout  = 10 // seconds
	DefaultFailures = 3
	DefaultCooldown = 300 // seconds (5m)
	MinInterval     = 1
	MinTimeout      = 1
	MinFailures     = 1
	MinCooldown     = 1
	MaxInterval     = 86400
	MaxTimeout      = 300
	MaxFailures     = 1000
	MaxCooldown     = 86400
)

// Settings controls the tunnel health watchdog.
type Settings struct {
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url"`
	Interval int    `json:"interval"` // seconds between probes
	Timeout  int    `json:"timeout"`  // seconds per probe
	Failures int    `json:"failures"` // consecutive failures before a restart
	Cooldown int    `json:"cooldown"` // minimum seconds between restarts
}

func DefaultSettings() Settings {
	return Settings{
		Enabled:  false,
		URL:      DefaultURL,
		ProxyURL: "",
		Interval: DefaultInterval,
		Timeout:  DefaultTimeout,
		Failures: DefaultFailures,
		Cooldown: DefaultCooldown,
	}
}

// Normalize clamps every field into a safe range. Zero/unset values fall back
// to defaults so a partially written row can never produce a busy loop or an
// immediate restart.
func (s Settings) Normalize() Settings {
	if strings.TrimSpace(s.URL) == "" {
		s.URL = DefaultURL
	}
	s.URL = strings.TrimSpace(s.URL)
	s.ProxyURL = strings.TrimSpace(s.ProxyURL)
	if s.Interval < MinInterval || s.Interval > MaxInterval {
		s.Interval = DefaultInterval
	}
	if s.Timeout < MinTimeout || s.Timeout > MaxTimeout {
		s.Timeout = DefaultTimeout
	}
	if s.Failures < MinFailures || s.Failures > MaxFailures {
		s.Failures = DefaultFailures
	}
	if s.Cooldown < MinCooldown || s.Cooldown > MaxCooldown {
		s.Cooldown = DefaultCooldown
	}
	return s
}

func LoadSettings(db *gorm.DB) (Settings, error) {
	s := DefaultSettings()
	if db == nil {
		return s, nil
	}
	var row models.PanelSetting
	if err := db.Where("key = ?", settingKey).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s, nil
		}
		return s, err
	}
	if strings.TrimSpace(row.Value) == "" {
		return s, nil
	}
	if err := json.Unmarshal([]byte(row.Value), &s); err != nil {
		return DefaultSettings(), err
	}
	return s.Normalize(), nil
}

func SaveSettings(db *gorm.DB, s Settings) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	s = s.Normalize()
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var row models.PanelSetting
	err = db.Where("key = ?", settingKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&models.PanelSetting{Key: settingKey, Value: string(raw)}).Error
	}
	if err != nil {
		return err
	}
	row.Value = string(raw)
	return db.Save(&row).Error
}

func (s Settings) intervalDuration() time.Duration {
	return time.Duration(s.Interval) * time.Second
}
func (s Settings) timeoutDuration() time.Duration {
	return time.Duration(s.Timeout) * time.Second
}
func (s Settings) cooldownDuration() time.Duration {
	return time.Duration(s.Cooldown) * time.Second
}
