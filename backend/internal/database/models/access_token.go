package models

import "time"

// AccessToken grants a client access to one Listener's generated node configuration.
type AccessToken struct {
	BaseModel
	Name       string     `gorm:"not null" json:"name"`
	Token      string     `gorm:"uniqueIndex;not null" json:"-"`
	Enabled    bool       `gorm:"default:true" json:"enabled"`
	ExpireAt   *time.Time `json:"expire_at"`
	ListenerID uint       `gorm:"index" json:"listener_id"`
}
