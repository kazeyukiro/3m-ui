package protocol

// Capability schema aligned with m-ui (Aethersailor/m-ui) for panel-driven node editors.
// Mihomo Meta inbound fields remain the source of truth for actual config generation.

const SchemaVersion = 1
const NodeSchemaVersion = 1

type SourceContract struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Commit     string `json:"commit,omitempty"`
}

type FieldType string

const (
	FieldString     FieldType = "string"
	FieldText       FieldType = "text"
	FieldSecret     FieldType = "secret"
	FieldBoolean    FieldType = "boolean"
	FieldInteger    FieldType = "integer"
	FieldStringList FieldType = "string-list"
)

type FieldCapability struct {
	Path        string    `json:"path"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Advanced    bool      `json:"advanced,omitempty"`
	Options     []string  `json:"options,omitempty"`
	Description string    `json:"description,omitempty"`
}

type ComponentGroup string

const (
	ComponentTransport ComponentGroup = "transport"
	ComponentSecurity  ComponentGroup = "security"
	ComponentExtension ComponentGroup = "extension"
)

type LayerCapability struct {
	Group            ComponentGroup `json:"group"`
	Required         bool           `json:"required"`
	Multiple         bool           `json:"multiple"`
	DefaultComponent string         `json:"default_component,omitempty"`
}

type ComponentCapability struct {
	Group         ComponentGroup    `json:"group"`
	Kind          string            `json:"kind"`
	Label         string            `json:"label"`
	SelectionPath string            `json:"selection_path,omitempty"`
	EnabledPath   string            `json:"enabled_path,omitempty"`
	Fields        []FieldCapability `json:"fields,omitempty"`
	Conflicts     []string          `json:"conflicts,omitempty"`
}

type ProtocolCapability struct {
	Kind       string                `json:"kind"`
	Label      string                `json:"label"`
	Layers     []LayerCapability     `json:"layers"`
	Components []ComponentCapability `json:"components"`
	Fields     []FieldCapability     `json:"fields,omitempty"`
	UserFields []FieldCapability     `json:"user_fields,omitempty"`
	Features   []string              `json:"features,omitempty"`
}

type CapabilityManifest struct {
	SchemaVersion       int                  `json:"schema_version"`
	NodeSchemaVersion   int                  `json:"node_schema_version"`
	Source              SourceContract       `json:"source"`
	NodeFields          []FieldCapability    `json:"node_fields"`
	AccessProfileFields []FieldCapability    `json:"access_profile_fields"`
	Protocols           []ProtocolCapability `json:"protocols"`
}

func DefaultManifest() CapabilityManifest {
	return CapabilityManifest{
		SchemaVersion:     SchemaVersion,
		NodeSchemaVersion: NodeSchemaVersion,
		Source: SourceContract{
			Repository: "MetaCubeX/mihomo",
			Branch:     "Meta",
		},
		NodeFields: []FieldCapability{
			{Path: "name", Label: "Name", Type: FieldString, Required: true},
			{Path: "listen", Label: "Listen", Type: FieldString, Required: true},
			{Path: "port", Label: "Port", Type: FieldString, Required: true, Description: "Single port or official ports syntax"},
			{Path: "enabled", Label: "Enabled", Type: FieldBoolean},
			{Path: "udp", Label: "UDP", Type: FieldBoolean},
		},
		AccessProfileFields: []FieldCapability{
			{Path: "public_host", Label: "Public Host", Type: FieldString, Description: "Hostname/IP used in share links and client YAML"},
			{Path: "public_port", Label: "Public Port", Type: FieldString, Description: "Override listen port in share links when behind NAT"},
			{Path: "sni", Label: "SNI", Type: FieldString},
			{Path: "client_fingerprint", Label: "Client Fingerprint", Type: FieldString, Options: []string{"chrome", "firefox", "safari", "ios", "android", "edge", "random"}},
			{Path: "alpn", Label: "ALPN", Type: FieldStringList},
		},
		Protocols: []ProtocolCapability{
			vlessCapability(),
			vmessCapability(),
			trojanCapability(),
			shadowsocksCapability(),
			hysteria2Capability(),
			tuicCapability(),
			shadowquicCapability(),
		},
	}
}

func transportSecurityLayers(defaultTransport, defaultSecurity string) []LayerCapability {
	return []LayerCapability{
		{Group: ComponentTransport, Required: true, Multiple: false, DefaultComponent: defaultTransport},
		{Group: ComponentSecurity, Required: false, Multiple: false, DefaultComponent: defaultSecurity},
	}
}

func transportComponents() []ComponentCapability {
	return transportComponentsCore(false)
}

// transportComponentsWithXHTTP is VLESS-only (MetaCubeX schema).
func transportComponentsWithXHTTP() []ComponentCapability {
	return transportComponentsCore(true)
}

func transportComponentsCore(withXHTTP bool) []ComponentCapability {
	wsConflicts := []string{"transport:grpc"}
	grpcConflicts := []string{"transport:ws"}
	if withXHTTP {
		wsConflicts = append(wsConflicts, "transport:xhttp")
		grpcConflicts = append(grpcConflicts, "transport:xhttp")
	}
	comps := []ComponentCapability{
		{Group: ComponentTransport, Kind: "raw", Label: "TCP / raw", SelectionPath: "transport_layer"},
		{Group: ComponentTransport, Kind: "ws", Label: "WebSocket", SelectionPath: "transport_layer", Fields: []FieldCapability{
			{Path: "ws-path", Label: "WS Path", Type: FieldString},
		}, Conflicts: wsConflicts},
		{Group: ComponentTransport, Kind: "grpc", Label: "gRPC", SelectionPath: "transport_layer", Fields: []FieldCapability{
			{Path: "grpc-service-name", Label: "gRPC Service Name", Type: FieldString},
		}, Conflicts: grpcConflicts},
	}
	if withXHTTP {
		comps = append(comps, ComponentCapability{
			Group: ComponentTransport, Kind: "xhttp", Label: "XHTTP", SelectionPath: "transport_layer",
			Fields: []FieldCapability{
				{Path: "xhttp_path", Label: "Path", Type: FieldString, Required: true},
				{Path: "xhttp_host", Label: "Host", Type: FieldString},
				{Path: "xhttp_mode", Label: "Mode", Type: FieldString, Options: []string{"auto", "stream-one", "stream-up", "packet-up"}},
			},
			Conflicts: []string{"transport:ws", "transport:grpc"},
		})
	}
	return comps
}


func securityComponents(withReality bool) []ComponentCapability {
	comps := []ComponentCapability{
		{Group: ComponentSecurity, Kind: "none", Label: "None", SelectionPath: "security_layer"},
		{Group: ComponentSecurity, Kind: "tls", Label: "TLS", SelectionPath: "security_layer", Fields: []FieldCapability{
			{Path: "certificate", Label: "Certificate", Type: FieldText},
			{Path: "private-key", Label: "Private Key", Type: FieldSecret},
			{Path: "alpn", Label: "ALPN", Type: FieldStringList},
			{Path: "allow-insecure", Label: "Allow Insecure", Type: FieldBoolean, Advanced: true},
		}, Conflicts: []string{"security:reality"}},
	}
	if withReality {
		comps = append(comps, ComponentCapability{
			Group: ComponentSecurity, Kind: "reality", Label: "Reality", SelectionPath: "security_layer",
			EnabledPath: "reality_enabled",
			Fields: []FieldCapability{
				{Path: "reality_dest", Label: "Dest", Type: FieldString, Required: true},
				{Path: "reality_private_key", Label: "Private Key", Type: FieldSecret, Required: true},
				{Path: "reality_short_id", Label: "Short ID", Type: FieldStringList},
				{Path: "reality_server_names", Label: "Server Names", Type: FieldStringList},
			},
			Conflicts: []string{"security:tls"},
		})
	}
	return comps
}

func vlessCapability() ProtocolCapability {
	comps := append(transportComponentsWithXHTTP(), securityComponents(true)...)
	return ProtocolCapability{
		Kind: "vless", Label: "VLESS",
		Layers:     transportSecurityLayers("raw", "reality"),
		Components: comps,
		Fields: []FieldCapability{
			{Path: "flow", Label: "Flow", Type: FieldString, Options: []string{"xtls-rprx-vision"}},
			{Path: "decryption", Label: "Decryption", Type: FieldText, Advanced: true, Description: "Server-side VLESS decryption (mihomo generate vless-x25519 / vless-mlkem768)"},
			{Path: "encryption", Label: "Encryption", Type: FieldText, Advanced: true, Description: "Client-side VLESS encryption pair (do not reuse decryption value)"},
		},
		UserFields: []FieldCapability{
			{Path: "uuid", Label: "UUID", Type: FieldString, Required: true},
			{Path: "flow", Label: "Flow", Type: FieldString, Options: []string{"", "xtls-rprx-vision"}},
		},
		Features: []string{"reality", "ws", "grpc", "xhttp", "vision"},
	}
}

func vmessCapability() ProtocolCapability {
	comps := append(transportComponents(), securityComponents(true)...)
	return ProtocolCapability{
		Kind: "vmess", Label: "VMess",
		Layers:     transportSecurityLayers("raw", "none"),
		Components: comps,
		Fields: []FieldCapability{
			{Path: "alterId", Label: "Alter ID", Type: FieldInteger},
		},
		UserFields: []FieldCapability{
			{Path: "uuid", Label: "UUID", Type: FieldString, Required: true},
		},
		Features: []string{"reality", "ws", "grpc", "mkcp", "mekya"},
	}
}

func trojanCapability() ProtocolCapability {
	comps := append(transportComponents(), securityComponents(true)...)
	return ProtocolCapability{
		Kind: "trojan", Label: "Trojan",
		Layers:     transportSecurityLayers("raw", "tls"),
		Components: comps,
		UserFields: []FieldCapability{
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true},
		},
		Features: []string{"reality", "ws", "grpc", "ss-option"},
	}
}

func shadowsocksCapability() ProtocolCapability {
	return ProtocolCapability{
		Kind: "shadowsocks", Label: "Shadowsocks",
		Layers:     []LayerCapability{},
		Components: []ComponentCapability{},
		Fields: []FieldCapability{
			{Path: "cipher", Label: "Cipher", Type: FieldString, Required: true, Options: []string{
				"2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305",
				"aes-128-gcm", "aes-192-gcm", "aes-256-gcm", "chacha20-ietf-poly1305", "xchacha20-ietf-poly1305", "none",
			}},
			{Path: "password", Label: "Password", Type: FieldSecret},
		},
		UserFields: []FieldCapability{
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true},
		},
		Features: []string{"udp", "simple-obfs", "shadow-tls"},
	}
}

func hysteria2Capability() ProtocolCapability {
	return ProtocolCapability{
		Kind: "hysteria2", Label: "Hysteria2",
		Layers: []LayerCapability{
			{Group: ComponentSecurity, Required: true, Multiple: false, DefaultComponent: "tls"},
		},
		Components: []ComponentCapability{
			{Group: ComponentSecurity, Kind: "tls", Label: "TLS", SelectionPath: "security_layer", Fields: []FieldCapability{
				{Path: "certificate", Label: "Certificate", Type: FieldText},
				{Path: "private-key", Label: "Private Key", Type: FieldSecret},
			}},
		},
		Fields: []FieldCapability{
			{Path: "up", Label: "Up", Type: FieldString},
			{Path: "down", Label: "Down", Type: FieldString},
			{Path: "obfs", Label: "Obfs", Type: FieldString, Options: []string{"salamander"}},
			{Path: "obfs-password", Label: "Obfs Password", Type: FieldSecret},
			{Path: "masquerade", Label: "Masquerade", Type: FieldString},
			{Path: "alpn", Label: "ALPN", Type: FieldStringList},
		},
		UserFields: []FieldCapability{
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true},
		},
		Features: []string{"quic", "bandwidth"},
	}
}

func tuicCapability() ProtocolCapability {
	return ProtocolCapability{
		Kind: "tuic", Label: "TUIC",
		Layers: []LayerCapability{
			{Group: ComponentSecurity, Required: true, Multiple: false, DefaultComponent: "tls"},
		},
		Components: securityComponents(false),
		Fields: []FieldCapability{
			{Path: "token", Label: "Token", Type: FieldString},
			{Path: "congestion-controller", Label: "Congestion", Type: FieldString, Options: []string{"bbr", "cubic", "new_reno"}},
			{Path: "alpn", Label: "ALPN", Type: FieldStringList},
		},
		UserFields: []FieldCapability{
			{Path: "uuid", Label: "UUID", Type: FieldString, Required: true},
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true},
		},
		Features: []string{"quic"},
	}
}

func shadowquicCapability() ProtocolCapability {
	return ProtocolCapability{
		Kind: "shadowquic", Label: "ShadowQUIC",
		Layers:     []LayerCapability{},
		Components: []ComponentCapability{},
		Fields: []FieldCapability{
			{Path: "alpn", Label: "ALPN", Type: FieldStringList},
			{Path: "congestion-controller", Label: "Congestion", Type: FieldString, Options: []string{"bbr", "cubic", "new_reno"}},
			{Path: "zero-rtt", Label: "0-RTT", Type: FieldBoolean},
		},
		UserFields: []FieldCapability{
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true},
			{Path: "username", Label: "Username", Type: FieldString},
		},
		Features: []string{"quic", "jls-upstream"},
	}
}
