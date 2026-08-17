package models

import "time"

// ListenerVersion stores an immutable snapshot of a Listener for diff/rollback.
type ListenerVersion struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ListenerID uint      `gorm:"not null;index" json:"listener_id"`
	Version    int       `gorm:"not null" json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	Reason     string    `gorm:"type:varchar(100)" json:"reason,omitempty"`
	Snapshot   string    `gorm:"type:text;not null" json:"snapshot"`
}
