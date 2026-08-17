package models

// ListenerUser is the explicit many-to-many join between ProxyUser and Listener.
type ListenerUser struct {
	BaseModel

	ListenerID  uint `gorm:"not null;index;uniqueIndex:idx_listener_proxy_user,priority:1" json:"listener_id"`
	ProxyUserID uint `gorm:"not null;index;uniqueIndex:idx_listener_proxy_user,priority:2" json:"proxy_user_id"`

	// Deprecated legacy fields. They are not used for authentication generation.
	Username string `gorm:"size:100;not null;default:''" json:"-"`
	Password string `gorm:"type:text;not null;default:''" json:"-"`
}
