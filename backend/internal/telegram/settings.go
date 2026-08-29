package telegram

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const settingKey = "telegram"

type Settings struct {
	Enabled           bool     `json:"enabled"`
	BotToken          string   `json:"bot_token"`
	ChatIDs           []string `json:"chat_ids"`
	NotifyOnLogin     bool     `json:"notify_on_login"`
	NotifyOnBlock     bool     `json:"notify_on_block"`
	NotifyOnUnblock   bool     `json:"notify_on_unblock"`
	NotifyOnExpiry    bool     `json:"notify_on_expiry"`
	NotifyOnTraffic   bool     `json:"notify_on_traffic"` // warn when usage >= TrafficWarnPct
	NotifyDailyDigest bool     `json:"notify_daily_digest"`
	TrafficWarnPct    int      `json:"traffic_warn_pct"`  // default 80
	ExpiryWarnHours   int      `json:"expiry_warn_hours"` // default 72
	NotifyOnCPU       bool     `json:"notify_on_cpu"`
	CPUWarnPct        int      `json:"cpu_warn_pct"` // default 0 = disabled; e.g. 80
	// Schedule is a cron-like spec or @daily / @every 6h controlling periodic report delivery.
	Schedule string `json:"schedule"`
	// AttachBackup attaches the panel backup file to the scheduled report message.
	AttachBackup bool `json:"attach_backup"`
	// Language selects the bot/notification language ("zh" or "en").
	Language string `json:"language"`
	// EnabledEvents is a comma-separated allowlist of event names that may fire notifications
	// (e.g. "login,cpu,crash"). Use EventEnabled(name) to test membership.
	EnabledEvents string `json:"enabled_events"`
	// ExpiryWarnDays warns N days before a proxy user expires (0 = disabled).
	ExpiryWarnDays int `json:"expiry_warn_days"`
	// TrafficWarnGB warns when remaining traffic drops below this many GB (0 = disabled).
	TrafficWarnGB int `json:"traffic_warn_gb"`
	// ProxyURL routes Telegram API traffic through an http(s):// or socks5:// proxy.
	ProxyURL string `json:"proxy_url"`
	// APIServer overrides the Telegram API base URL (empty = https://api.telegram.org).
	APIServer string `json:"api_server"`
}

// normalizeChatIDList accepts values like "123", "-100123", "123,456" or multiline.
func normalizeChatIDList(ids []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}) {
			part = strings.TrimSpace(part)
			part = strings.TrimPrefix(part, "+")
			part = strings.TrimPrefix(part, "@")
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}

func DefaultSettings() Settings {
	return Settings{
		NotifyOnBlock:   true,
		NotifyOnUnblock: true,
		NotifyOnExpiry:  true,
		NotifyOnTraffic: true,
		TrafficWarnPct:  80,
		ExpiryWarnHours: 72,
		NotifyOnCPU:     false,
		CPUWarnPct:      0,
		Schedule:        "@daily",
		Language:        "zh",
		EnabledEvents:   "login,cpu,crash",
	}
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
	s.ChatIDs = normalizeChatIDList(s.ChatIDs)
	s.BotToken = strings.TrimSpace(s.BotToken)
	if s.TrafficWarnPct <= 0 || s.TrafficWarnPct > 100 {
		s.TrafficWarnPct = 80
	}
	if s.ExpiryWarnHours <= 0 {
		s.ExpiryWarnHours = 72
	}
	if strings.TrimSpace(s.Schedule) == "" {
		s.Schedule = "@daily"
	}
	if strings.TrimSpace(s.Language) == "" {
		s.Language = "zh"
	}
	if strings.TrimSpace(s.EnabledEvents) == "" {
		s.EnabledEvents = "login,cpu,crash"
	}
	return s, nil
}

func SaveSettings(db *gorm.DB, s Settings) error {
	s.ChatIDs = normalizeChatIDList(s.ChatIDs)
	s.BotToken = strings.TrimSpace(s.BotToken)
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

func NewClientFromDB(db *gorm.DB) (*Client, Settings, error) {
	s, err := LoadSettings(db)
	if err != nil {
		return nil, s, err
	}
	if !s.Enabled || s.BotToken == "" || len(s.ChatIDs) == 0 {
		return nil, s, nil
	}
	return NewClient(s.BotToken, s.ChatIDs, s.ProxyURL, s.APIServer), s, nil
}

// EventEnabled reports whether the named event is in the EnabledEvents allowlist.
// EnabledEvents is a comma-separated list (e.g. "login,cpu,crash"). Empty name returns false.
func (s Settings) EventEnabled(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, ev := range strings.Split(s.EnabledEvents, ",") {
		if strings.ToLower(strings.TrimSpace(ev)) == name {
			return true
		}
	}
	return false
}
