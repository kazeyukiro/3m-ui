package models

type Listener struct {
	BaseModel
	Name        string `gorm:"not null;uniqueIndex" json:"name"`
	Protocol    string `gorm:"type:varchar(50);not null;default:'shadowsocks'" json:"protocol"`
	Type        string `gorm:"type:varchar(50)" json:"type"`
	Port        string `gorm:"type:varchar(50);not null;default:'0'" json:"port"`
	BindAddress string `gorm:"type:varchar(100);not null;default:'0.0.0.0'" json:"bind_address"`
	Listen      string `gorm:"type:varchar(100)" json:"listen"`
	TLS         bool   `gorm:"not null;default:false" json:"tls,omitempty"`
	UDP         bool   `gorm:"not null;default:false" json:"udp,omitempty"`
	Enabled     bool   `gorm:"not null;default:false" json:"enabled"`
	Proxy       string `gorm:"type:varchar(255)" json:"proxy,omitempty"`
	Rule        string `gorm:"type:text" json:"rule,omitempty"`
	Config      string `gorm:"type:text" json:"config"`
	Status      string `gorm:"type:varchar(50);default:'inactive'" json:"status"`
	RoutingMark int    `gorm:"default:0" json:"routing_mark,omitempty"`

	// Per-node Access Profile (m-ui style) — used for share links / client export.
	PublicHost        string `gorm:"type:varchar(255)" json:"public_host,omitempty"`
	PublicPort        string `gorm:"type:varchar(32)" json:"public_port,omitempty"`
	AccessSNI         string `gorm:"type:varchar(255);column:access_sni" json:"access_sni,omitempty"`
	ClientFingerprint string `gorm:"type:varchar(64)" json:"client_fingerprint,omitempty"`
	AccessALPN        string `gorm:"type:varchar(255);column:access_alpn" json:"access_alpn,omitempty"`
}
