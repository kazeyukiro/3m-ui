package models

import "time"

// ListenerTemplate stores reusable Listener configuration without runtime state.
type ListenerTemplate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	Protocol  string    `gorm:"type:varchar(50);not null" json:"protocol"`
	Config    string    `gorm:"type:text;not null" json:"config"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
