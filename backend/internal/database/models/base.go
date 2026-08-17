package models

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel replaces gorm.Model with explicit lowercase JSON tags so the
// frontend receives "id" / "created_at" instead of Go's default "ID".
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
