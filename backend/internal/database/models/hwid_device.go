package models

import "time"

// HWIDDevice tracks a client device that imported a subscription with x-hwid
// (Happ / Remnawave-compatible device limit headers).
//
// Column names are explicit: GORM's default snake_case turns "HWID" into
// "h_w_i_d", which breaks SQLite (no such column: hwid).
type HWIDDevice struct {
	BaseModel

	ProxyUserID uint      `gorm:"column:proxy_user_id;not null;uniqueIndex:uidx_user_hwid,priority:1;index" json:"proxy_user_id"`
	HWID        string    `gorm:"column:hwid;size:128;not null;uniqueIndex:uidx_user_hwid,priority:2" json:"hwid"`
	DeviceOS    string    `gorm:"column:device_os;size:64" json:"device_os"`
	VerOS       string    `gorm:"column:ver_os;size:64" json:"ver_os"`
	DeviceModel string    `gorm:"column:device_model;size:128" json:"device_model"`
	UserAgent   string    `gorm:"column:user_agent;size:512" json:"user_agent"`
	LastSeenAt  time.Time `gorm:"column:last_seen_at" json:"last_seen_at"`
}

func (HWIDDevice) TableName() string { return "hwid_devices" }
