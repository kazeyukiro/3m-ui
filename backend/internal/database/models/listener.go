package models

import (
	"time"

	"gorm.io/gorm"
)

type Listener struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"not null" json:"name"`
	Protocol    string         `gorm:"type:varchar(50);not null;default:'shadowsocks'" json:"protocol"`
	Type        string         `gorm:"type:varchar(50)" json:"type"`
	Port        string         `gorm:"type:varchar(50);not null;default:'0'" json:"port"`
	BindAddress string         `gorm:"type:varchar(100);not null;default:'0.0.0.0'" json:"bind_address"`
	Listen      string         `gorm:"type:varchar(100)" json:"listen"`
	TLS         bool           `gorm:"not null;default:false" json:"tls,omitempty"`
	UDP         bool           `gorm:"not null;default:false" json:"udp,omitempty"`
	Enabled     bool           `gorm:"not null;default:false" json:"enabled"`
	Proxy       string         `gorm:"type:varchar(255)" json:"proxy,omitempty"`
	Rule        string         `gorm:"type:text" json:"rule,omitempty"`
	Config      string         `gorm:"type:text" json:"config"`
	Status      string         `gorm:"type:varchar(50);default:'inactive'" json:"status"`
	RoutingMark int            `gorm:"default:0" json:"routing_mark,omitempty"`
}
