package models

import "time"

// ProxyUser is a user account used to authenticate against Mihomo server nodes.
// It is deliberately separate from User, which represents a 3m-ui administrator.
type ProxyUser struct {
	BaseModel

	Username string `gorm:"size:100;not null;uniqueIndex" json:"username"`
	// PasswordEncrypted contains an AES-GCM encrypted password. It is never serialized to API responses.
	PasswordEncrypted string     `gorm:"type:text" json:"-"`
	UUID              string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	TrafficLimit      int64      `gorm:"not null;default:0" json:"traffic_limit"`
	TrafficUsed       int64      `gorm:"not null;default:0" json:"traffic_used"`
	UploadBytes       int64      `gorm:"not null;default:0" json:"upload_bytes"`
	DownloadBytes     int64      `gorm:"not null;default:0" json:"download_bytes"`
	LastSeen          *time.Time `json:"last_seen"`
	Online            bool       `gorm:"not null;default:false" json:"online"`
	ExpireTime        time.Time  `json:"expire_time"`
	Enabled           bool       `gorm:"not null;default:true" json:"enabled"`
	// IPLimit is max concurrent client IPs (0 = unlimited). Max concurrent client IPs.
	IPLimit int `gorm:"not null;default:0" json:"ip_limit"`
	// HWIDLimit is max devices allowed via subscription HWID headers (0 = unlimited / tracking only when seen).
	HWIDLimit int `gorm:"not null;default:0" json:"hwid_limit"`
	// Remark is an admin-facing note (not used for auth).
	Remark string `gorm:"size:255" json:"remark"`
	// SubToken is the public subscription credential (client sub).
	// Unique when set; empty legacy rows are filled during InitDB migration.
	SubToken string `gorm:"size:64;uniqueIndex" json:"sub_token,omitempty"`
	// TelegramID is the linked Telegram chat/user numeric ID (0 = not bound).
	// Indexed so the bot can look up a proxy user from an incoming Telegram message.
	TelegramID int64 `gorm:"index;default:0" json:"telegram_id,omitempty"`
	// TelegramName is the display name of the linked Telegram account (best-effort cache).
	TelegramName string `gorm:"size:64;default:''" json:"telegram_name,omitempty"`
	// Group is an admin-facing group name for filtering and bulk ops (not auth).
	Group string `gorm:"size:64;index;default:''" json:"group"`
	// Tags is a comma-separated list of labels (admin-facing).
	Tags string `gorm:"size:255;default:''" json:"tags"`
	// TrafficResetDays: when >0, traffic counters reset every N days from LastTrafficCycleReset.
	TrafficResetDays int `gorm:"not null;default:0" json:"traffic_reset_days"`
	// ExpireRenewDays: when >0 and credential would expire, extend ExpireTime by N days (calendar cycle).
	ExpireRenewDays int `gorm:"not null;default:0" json:"expire_renew_days"`
	// LastTrafficCycleReset tracks the last per-user traffic cycle reset (UTC).
	LastTrafficCycleReset *time.Time `json:"last_traffic_cycle_reset,omitempty"`
	// StartOnFirstUse: ignore ExpireTime until the user first connects or pulls a subscription.
	StartOnFirstUse bool `gorm:"not null;default:false" json:"start_on_first_use"`
	// FirstConnectedAt is set once when traffic/subscription first sees the user.
	FirstConnectedAt *time.Time `json:"first_connected_at,omitempty"`
	// ExpireDaysAfterFirst: when StartOnFirstUse and >0, set ExpireTime = first use + N days.
	ExpireDaysAfterFirst int `gorm:"not null;default:0" json:"expire_days_after_first"`
	// ExternalLinks: newline-separated Clash/Mihomo subscription URLs merged into this user's sub.
	ExternalLinks string `gorm:"type:text" json:"external_links,omitempty"`
}

