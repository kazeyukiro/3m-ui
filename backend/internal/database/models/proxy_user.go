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
	// Remark is an admin-facing note (not used for auth).
	Remark string `gorm:"size:255" json:"remark"`
	// SubToken is the public subscription credential (client sub).
	// Unique when set; empty legacy rows are filled during InitDB migration.
	SubToken string `gorm:"size:64;uniqueIndex" json:"sub_token,omitempty"`
}
