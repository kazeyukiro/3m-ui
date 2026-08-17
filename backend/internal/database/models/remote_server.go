package models

import "time"

// RemoteServer registers an external 3m-ui / Mihomo host managed from this panel.
type RemoteServer struct {
	BaseModel

	Name        string     `gorm:"size:100;not null" json:"name"`
	BaseURL     string     `gorm:"size:512;not null" json:"base_url"`
	APIToken    string     `gorm:"size:256" json:"-"`
	APITokenSet bool       `gorm:"-" json:"api_token_set"`
	Enabled     bool       `gorm:"not null;default:true" json:"enabled"`
	Remark      string     `gorm:"size:255" json:"remark"`
	LastStatus  string     `gorm:"size:32" json:"last_status"`
	LastCheckAt *time.Time `json:"last_check_at"`
	LastError   string     `gorm:"type:text" json:"last_error"`
}
