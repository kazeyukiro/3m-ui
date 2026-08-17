package models

type Config struct {
	BaseModel
	Name    string `gorm:"uniqueIndex;not null" json:"name"`
	Type    string `gorm:"type:varchar(50);not null;default:'custom'" json:"type"`
	Content string `gorm:"type:text" json:"content"`
	Enabled bool   `gorm:"not null;default:true" json:"enabled"`
}
