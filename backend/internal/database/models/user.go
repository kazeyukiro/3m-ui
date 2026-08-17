package models

type User struct {
	BaseModel
	Username           string `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash       string `gorm:"not null" json:"-"`
	Role               string `gorm:"not null" json:"role"`
	MustChangePassword bool   `gorm:"not null;default:false" json:"must_change_password"`
	SessionVersion     uint   `gorm:"not null;default:1" json:"-"`
}
