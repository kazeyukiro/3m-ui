package node

// NodeUser represents the user credentials mapping for Mihomo inbound protocols
type NodeUser struct {
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	UUID     string `yaml:"uuid,omitempty" json:"uuid,omitempty"`
	Flow     string `yaml:"flow,omitempty" json:"flow,omitempty"`
}

// MihomoTLSConfig represents the TLS parameters for Mihomo inbounds
type MihomoTLSConfig struct {
	Enable      bool   `yaml:"enable" json:"enable"`
	Certificate string `yaml:"certificate,omitempty" json:"certificate,omitempty"`
	PrivateKey  string `yaml:"private-key,omitempty" json:"private_key,omitempty"`
}

// MihomoListener represents a type-safe listener configuration structure
type MihomoListener struct {
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type"` // shadowsocks, vmess, vless, trojan, hysteria2, tuic
	Port   string                 `yaml:"port"`
	Listen string                 `yaml:"listen"`
	UDP    bool                   `yaml:"udp,omitempty"`
	TLS    *MihomoTLSConfig       `yaml:"tls,omitempty"`
	Users  []NodeUser             `yaml:"users,omitempty"`
	Extra  map[string]interface{} `yaml:"-,inline"` // for protocol-specific extra fields
}
