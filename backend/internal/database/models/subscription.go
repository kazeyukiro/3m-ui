package models

import "time"

type Subscription struct {
	BaseModel
	UserID     uint       `gorm:"not null;index" json:"user_id"`
	Token      string     `gorm:"uniqueIndex;not null" json:"token"`
	Format     string     `gorm:"not null" json:"format"`
	ExpireTime *time.Time `json:"expire_time"`
}
