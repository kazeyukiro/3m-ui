package protocol

// Capability schema used by the panel node editor.
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
			{Path: "name", Label: "Name", Type: FieldString, Required: true, Description: "Listener name (unique). Becomes the Mihomo listeners[].name entry."},
			{Path: "listen", Label: "Listen", Type: FieldString, Required: true, Description: "Bind address (wiki: listen). 0.0.0.0 or :: for all interfaces."},
			{Path: "port", Label: "Port", Type: FieldString, Required: true, Description: "Listen port (wiki: port). Single port or official ports range syntax."},
			{Path: "enabled", Label: "Enabled", Type: FieldBoolean, Description: "When off, the listener is omitted from the running core config."},
			{Path: "udp", Label: "UDP", Type: FieldBoolean, Description: "Enable UDP (wiki: udp). Required for many QUIC/UDP-based protocols; 3m-ui may manage this for some types."},
		},
		AccessProfileFields: []FieldCapability{
			{Path: "public_host", Label: "Public Host", Type: FieldString, Description: "Hostname/IP written into share links and client YAML (NAT / CDN front)."},
			{Path: "public_port", Label: "Public Port", Type: FieldString, Description: "Port in share links when mapped differently than listen port (NAT)."},
			{Path: "sni", Label: "SNI", Type: FieldString, Description: "TLS Server Name for clients (wiki proxies: servername / sni). Empty may use public host."},
			{Path: "client_fingerprint", Label: "Client Fingerprint", Type: FieldString, Options: []string{"chrome", "firefox", "safari", "ios", "android", "edge", "random"}, Description: "uTLS client fingerprint (wiki: client-fingerprint)."},
			{Path: "alpn", Label: "ALPN", Type: FieldStringList, Description: "TLS ALPN list (wiki: alpn), e.g. h2, http/1.1."},
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
			{Path: "certificate", Label: "Certificate", Type: FieldText, Description: "TLS certificate PEM or path (wiki: certificate). Empty → panel may auto self-sign."},
			{Path: "private-key", Label: "Private Key", Type: FieldSecret, Description: "TLS private key PEM or path (wiki: private-key)."},
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
			{Path: "flow", Label: "Flow", Type: FieldString, Options: []string{"xtls-rprx-vision"}, Description: "XTLS Vision flow (wiki: flow). Only with TCP/raw; not with ws/grpc/xhttp."},
			{Path: "decryption", Label: "Decryption", Type: FieldText, Advanced: true, Description: "Server-side VLESS decryption (wiki/decryption). Generate with mihomo vless-x25519 / vless-mlkem768."},
			{Path: "encryption", Label: "Encryption", Type: FieldText, Advanced: true, Description: "Client-side encryption pairing; for subscription export only, not inbound YAML."},
		},
		UserFields: []FieldCapability{
			{Path: "uuid", Label: "UUID", Type: FieldString, Required: true, Description: "User id (wiki users[].uuid)."},
			{Path: "flow", Label: "Flow", Type: FieldString, Options: []string{"", "xtls-rprx-vision"}, Description: "Per-user Vision flow; empty inherits node default."},
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
			{Path: "alterId", Label: "Alter ID", Type: FieldInteger, Description: "VMess alterId (wiki). Use 0 for AEAD-only modern clients."},
		},
		UserFields: []FieldCapability{
			{Path: "uuid", Label: "UUID", Type: FieldString, Required: true, Description: "VMess user UUID."},
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
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true, Description: "Trojan password (wiki users password)."},
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
			}, Description: "Shadowsocks method (wiki: cipher). Prefer 2022-blake3-* when clients support it."},
			{Path: "password", Label: "Password", Type: FieldSecret, Description: "Node default password; per-user passwords override when using multi-user mode."},
		},
		UserFields: []FieldCapability{
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true, Description: "User password (wiki)."},
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
				{Path: "certificate", Label: "Certificate", Type: FieldText, Description: "TLS certificate PEM or path (wiki: certificate). Empty → panel may auto self-sign."},
				{Path: "private-key", Label: "Private Key", Type: FieldSecret, Description: "TLS private key PEM or path (wiki: private-key)."},
			}},
		},
		Fields: []FieldCapability{
			{Path: "up", Label: "Up", Type: FieldString, Description: "Max upload bandwidth (wiki: up), e.g. 100 Mbps."},
			{Path: "down", Label: "Down", Type: FieldString, Description: "Max download bandwidth (wiki: down)."},
			{Path: "obfs", Label: "Obfs", Type: FieldString, Options: []string{"salamander"}, Description: "QUIC obfuscation type (wiki: obfs). salamander is common."},
			{Path: "obfs-password", Label: "Obfs Password", Type: FieldSecret, Description: "Obfuscation password (wiki: obfs-password); must match client."},
			{Path: "masquerade", Label: "Masquerade", Type: FieldString, Description: "HTTP masquerade URL/path (wiki: masquerade) for browser-like probes."},
			{Path: "alpn", Label: "ALPN", Type: FieldStringList, Description: "QUIC/TLS ALPN (wiki: alpn)."},
		},
		UserFields: []FieldCapability{
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true, Description: "Hysteria2 user password (map users)."},
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
			{Path: "token", Label: "Token", Type: FieldString, Description: "TUIC v4 shared token (wiki tuic-v4: token). Leave empty for v5 users map."},
			{Path: "congestion-controller", Label: "Congestion", Type: FieldString, Options: []string{"bbr", "cubic", "new_reno"}, Description: "QUIC congestion control (wiki: congestion-controller)."},
			{Path: "alpn", Label: "ALPN", Type: FieldStringList, Description: "QUIC ALPN (wiki: alpn)."},
		},
		UserFields: []FieldCapability{
			{Path: "uuid", Label: "UUID", Type: FieldString, Required: true, Description: "TUIC v5 user UUID (wiki users: UUID: password)."},
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true, Description: "TUIC v5 user password."},
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
			{Path: "alpn", Label: "ALPN", Type: FieldStringList, Description: "QUIC ALPN for ShadowQUIC."},
			{Path: "congestion-controller", Label: "Congestion", Type: FieldString, Options: []string{"bbr", "cubic", "new_reno"}, Description: "QUIC congestion controller."},
			{Path: "zero-rtt", Label: "0-RTT", Type: FieldBoolean, Description: "Enable 0-RTT (wiki). Lower latency, slightly weaker replay properties."},
		},
		UserFields: []FieldCapability{
			{Path: "password", Label: "Password", Type: FieldSecret, Required: true, Description: "ShadowQUIC user password."},
			{Path: "username", Label: "Username", Type: FieldString, Description: "Optional username when multi-user is used."},
		},
		Features: []string{"quic", "jls-upstream"},
	}
}
