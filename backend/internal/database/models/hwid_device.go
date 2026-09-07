package models

import "time"

// HWIDDevice tracks a client device that imported a subscription with x-hwid
// (Happ / Remnawave-compatible device limit headers).
type HWIDDevice struct {
	BaseModel

	ProxyUserID uint      `gorm:"not null;uniqueIndex:uidx_user_hwid,priority:1;index" json:"proxy_user_id"`
	HWID        string    `gorm:"size:64;not null;uniqueIndex:uidx_user_hwid,priority:2" json:"hwid"`
	DeviceOS    string    `gorm:"size:64" json:"device_os"`
	VerOS       string    `gorm:"size:64" json:"ver_os"`
	DeviceModel string    `gorm:"size:128" json:"device_model"`
	UserAgent   string    `gorm:"size:512" json:"user_agent"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}
