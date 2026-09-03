package domain

import (
	"net"
	"strconv"
	"time"
)

const (
	NodeSchemaVersion  = 2
	MihomoRepository   = "MetaCubeX/mihomo"
	MihomoSourceBranch = "Meta"
	MihomoSourceCommit = "e26714a181ac0e2fa803453c0a8e9a9ce94e31cb"

	VLESSFlowVision    = "xtls-rprx-vision"
	PacketEncodingXUDP = "xudp"
	ClientFingerprint  = "chrome"
)

type DesiredState struct {
	AsOf       time.Time `json:"as_of"`
	PublicHost string    `json:"public_host"`
	Nodes      []Node    `json:"nodes"`
}

type Endpoint struct {
	Host string `json:"host"`
	Port uint16 `json:"port"`
}

func (endpoint Endpoint) Address() string {
	return net.JoinHostPort(endpoint.Host, strconv.Itoa(int(endpoint.Port)))
}

type ProtocolKind string

const (
	ProtocolVLESS       ProtocolKind = "vless"
	ProtocolHysteria2   ProtocolKind = "hysteria2"
	ProtocolVMess       ProtocolKind = "vmess"
	ProtocolTrojan      ProtocolKind = "trojan"
	ProtocolShadowsocks ProtocolKind = "shadowsocks"
)

type Node struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Enabled        bool             `json:"enabled"`
	ListenAddress  string           `json:"listen"`
	Port           string           `json:"port"`
	Protocol       ProtocolKind     `json:"protocol"`
	SchemaVersion  int              `json:"schema_version"`
	VLESS          *VLESSSpec       `json:"vless,omitempty"`
	Hysteria2      *Hysteria2Spec   `json:"hysteria2,omitempty"`
	VMess          *VMessSpec       `json:"vmess,omitempty"`
	Trojan         *TrojanSpec      `json:"trojan,omitempty"`
	Shadowsocks    *ShadowsocksSpec `json:"shadowsocks,omitempty"`
	Users          []NodeUser       `json:"users"`
	AccessProfiles []AccessProfile  `json:"access_profiles"`
	Generation     int64            `json:"generation"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type NodeUser struct {
	ID          string                 `json:"id"`
	NodeID      string                 `json:"node_id"`
	Name        string                 `json:"name"`
	Enabled     bool                   `json:"enabled"`
	VLESS       *VLESSCredential       `json:"vless,omitempty"`
	Hysteria2   *Hysteria2Credential   `json:"hysteria2,omitempty"`
	VMess       *VMessCredential       `json:"vmess,omitempty"`
	Trojan      *TrojanCredential      `json:"trojan,omitempty"`
	Shadowsocks *ShadowsocksCredential `json:"shadowsocks,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type VLESSCredential struct {
	UUID string `json:"uuid"`
	Flow string `json:"flow,omitempty"`
}

type Hysteria2Credential struct {
	Password string `json:"password"`
}

// VMessCredential mirrors listener/inbound.VmessUser on Mihomo's Meta branch.
// Cipher is client-side VMess security and is retained with the credential so
// every exported profile has a deterministic value.
type VMessCredential struct {
	UUID    string `json:"uuid"`
	AlterID int    `json:"alter_id,omitempty"`
	Cipher  string `json:"cipher,omitempty"`
}

type TrojanCredential struct {
	Password string `json:"password"`
}

// Mihomo's Shadowsocks listener has one password rather than a users array.
// m-ui still models it as a credential for consistent lifecycle/share APIs,
// while validation permits only one effective Shadowsocks user at a time.
type ShadowsocksCredential struct {
	Password string `json:"password"`
}

type AccessProfile struct {
	ID             string    `json:"id"`
	NodeID         string    `json:"node_id"`
	Name           string    `json:"name"`
	Default        bool      `json:"default"`
	PublicHost     string    `json:"public_host"`
	PublicPort     uint16    `json:"public_port"`
	ServerName     string    `json:"server_name,omitempty"`
	Fingerprint    string    `json:"fingerprint,omitempty"`
	PacketEncoding string    `json:"packet_encoding,omitempty"`
	AllowInsecure  bool      `json:"allow_insecure,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type VLESSHandlerKind string

const (
	VLESSHandlerRaw       VLESSHandlerKind = "raw"
	VLESSHandlerWebSocket VLESSHandlerKind = "websocket"
	VLESSHandlerGRPC      VLESSHandlerKind = "grpc"
	VLESSHandlerXHTTP     VLESSHandlerKind = "xhttp"
	VMessHandlerMKCP      VLESSHandlerKind = "mkcp"
)

type VLESSSpec struct {
	Decryption     string            `json:"decryption,omitempty"`
	Handler        VLESSHandlerSpec  `json:"handler"`
	Security       VLESSSecuritySpec `json:"security"`
	Mux            MuxSpec           `json:"mux,omitempty"`
	ALPN           []string          `json:"alpn,omitempty"`
	Fingerprint    string            `json:"fingerprint,omitempty"`
	NameCertVerify string            `json:"name_cert_verify,omitempty"`
	SMux           SMuxSpec          `json:"smux,omitempty"`
	ECHOpts        map[string]any    `json:"ech_opts,omitempty"`
}

// SMuxSpec mirrors the mihomo `smux` block documented on proxies-vmess /
// proxies-vless / proxies-trojan / proxies-ss (common optional client field).
// It is intentionally a subset of MuxSpec since `smux` and `mux-option` share
// the same brutal control plane but `smux` carries an explicit `enabled` flag.
type SMuxSpec struct {
	Enabled bool       `json:"enabled,omitempty"`
	Padding bool       `json:"padding,omitempty"`
	Brutal  BrutalSpec `json:"brutal,omitempty"`
}

// VMess and Trojan share the stream handler and security building blocks that
// Mihomo exposes for their listeners. Keeping the composition typed here lets
// later protocol modules opt into only the components their source supports.
type VMessSpec struct {
	Handler             VLESSHandlerSpec  `json:"handler"`
	Security            VLESSSecuritySpec `json:"security"`
	Mux                 MuxSpec           `json:"mux,omitempty"`
	ALPN                []string          `json:"alpn,omitempty"`
	GlobalPadding       bool              `json:"global_padding,omitempty"`
	AuthenticatedLength bool              `json:"authenticated_length,omitempty"`
	Fingerprint         string            `json:"fingerprint,omitempty"`
	NameCertVerify      string            `json:"name_cert_verify,omitempty"`
	TLSMirrorOpts       map[string]any    `json:"tlsmirror_opts,omitempty"`
	SMux                SMuxSpec          `json:"smux,omitempty"`
	ECHOpts             map[string]any    `json:"ech_opts,omitempty"`
}

type TrojanSpec struct {
	Handler        VLESSHandlerSpec      `json:"handler"`
	Security       VLESSSecuritySpec     `json:"security"`
	Mux            MuxSpec               `json:"mux,omitempty"`
	Shadowsocks    TrojanShadowsocksSpec `json:"shadowsocks,omitempty"`
	ALPN           []string              `json:"alpn,omitempty"`
	Fingerprint    string                `json:"fingerprint,omitempty"`
	NameCertVerify string                `json:"name_cert_verify,omitempty"`
	TLSMirrorOpts  map[string]any        `json:"tlsmirror_opts,omitempty"`
	SMux           SMuxSpec              `json:"smux,omitempty"`
	ECHOpts        map[string]any        `json:"ech_opts,omitempty"`
}

type TrojanShadowsocksSpec struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Method   string `json:"method,omitempty"`
	Password string `json:"password,omitempty"`
}

type ShadowsocksSpec struct {
	Cipher            string            `json:"cipher"`
	UDP               bool              `json:"udp"`
	Security          VLESSSecuritySpec `json:"security"`
	Mux               MuxSpec           `json:"mux,omitempty"`
	SimpleObfs        SimpleObfsSpec    `json:"simple_obfs,omitempty"`
	Kcptun            *KCPTunConfig     `json:"kcptun,omitempty"`
	UDPOverTCP        bool              `json:"udp_over_tcp,omitempty"`
	UDPOverTCPVersion string            `json:"udp_over_tcp_version,omitempty"`
	IPVersion         string            `json:"ip_version,omitempty"`
	SMux              SMuxSpec          `json:"smux,omitempty"`
}

// KCPTunConfig mirrors the SS listener `kcp-tun` block. The m-ui SS module
// emits it as `plugin: kcptun` + `plugin-opts: {key, crypt, mode, mtu, ...}`
// per proxies-ss wiki block 6.
type KCPTunConfig struct {
	Enable      bool   `json:"enable,omitempty"`
	Key         string `json:"key,omitempty"`
	Crypt       string `json:"crypt,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Conn        int    `json:"conn,omitempty"`
	AutoExpire  int    `json:"auto_expire,omitempty"`
	ScavengeTTL int    `json:"scavenge_ttl,omitempty"`
	RateLimit   int    `json:"rate_limit,omitempty"`
	MTU         int    `json:"mtu,omitempty"`
	SndWnd      int    `json:"snd_wnd,omitempty"`
	RcvWnd      int    `json:"rcv_wnd,omitempty"`
	DataShard   int    `json:"data_shard,omitempty"`
	ParityShard int    `json:"parity_shard,omitempty"`
	DSCP        int    `json:"dscp,omitempty"`
	NoComp      bool   `json:"no_comp,omitempty"`
	AckNoDelay  bool   `json:"ack_no_delay,omitempty"`
	NoDelay     int    `json:"no_delay,omitempty"`
	Interval    int    `json:"interval,omitempty"`
	Resend      int    `json:"resend,omitempty"`
	SockBuf     int    `json:"sock_buf,omitempty"`
	SmuxVer     int    `json:"smux_ver,omitempty"`
	SmuxBuf     int    `json:"smux_buf,omitempty"`
	FrameSize   int    `json:"frame_size,omitempty"`
	StreamBuf   int    `json:"stream_buf,omitempty"`
	KeepAlive   int    `json:"keep_alive,omitempty"`
}

type SimpleObfsSpec struct {
	Enabled bool   `json:"enabled,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

type VLESSHandlerSpec struct {
	Type      VLESSHandlerKind `json:"type"`
	WebSocket *WebSocketSpec   `json:"websocket,omitempty"`
	GRPC      *GRPCSpec        `json:"grpc,omitempty"`
	XHTTP     *XHTTPConfig     `json:"xhttp,omitempty"`
	MKCP      *MKCPConfig      `json:"mkcp,omitempty"`
}

// MKCPConfig mirrors listener/inbound.MKCPConfig on Mihomo Meta. It is
// currently registered only by the VMess module.
type MKCPConfig struct {
	MTU              uint32 `json:"mtu,omitempty"`
	TTI              uint32 `json:"tti,omitempty"`
	UplinkCapacity   uint32 `json:"uplink_capacity,omitempty"`
	DownlinkCapacity uint32 `json:"downlink_capacity,omitempty"`
	Congestion       bool   `json:"congestion,omitempty"`
	WriteBuffer      uint32 `json:"write_buffer,omitempty"`
	ReadBuffer       uint32 `json:"read_buffer,omitempty"`
	Seed             string `json:"seed,omitempty"`
	Header           string `json:"header,omitempty"`
}

type WebSocketSpec struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
}

type GRPCSpec struct {
	ServiceName    string `json:"service_name"`
	GRPCUserAgent  string `json:"grpc_user_agent,omitempty"`
	PingInterval   int    `json:"ping_interval,omitempty"`
	MaxConnections int    `json:"max_connections,omitempty"`
	MinStreams     int    `json:"min_streams,omitempty"`
	MaxStreams     int    `json:"max_streams,omitempty"`
}

type XHTTPConfig struct {
	Path                 string `json:"path,omitempty"`
	Host                 string `json:"host,omitempty"`
	Mode                 string `json:"mode,omitempty"`
	XPaddingBytes        string `json:"x_padding_bytes,omitempty"`
	XPaddingObfsMode     bool   `json:"x_padding_obfs_mode,omitempty"`
	XPaddingKey          string `json:"x_padding_key,omitempty"`
	XPaddingHeader       string `json:"x_padding_header,omitempty"`
	XPaddingPlacement    string `json:"x_padding_placement,omitempty"`
	XPaddingMethod       string `json:"x_padding_method,omitempty"`
	UplinkHTTPMethod     string `json:"uplink_http_method,omitempty"`
	SessionPlacement     string `json:"session_placement,omitempty"`
	SessionKey           string `json:"session_key,omitempty"`
	SeqPlacement         string `json:"seq_placement,omitempty"`
	SeqKey               string `json:"seq_key,omitempty"`
	UplinkDataPlacement  string `json:"uplink_data_placement,omitempty"`
	UplinkDataKey        string `json:"uplink_data_key,omitempty"`
	UplinkChunkSize      string `json:"uplink_chunk_size,omitempty"`
	NoSSEHeader          bool   `json:"no_sse_header,omitempty"`
	SCStreamUpServerSecs string `json:"sc_stream_up_server_secs,omitempty"`
	SCMaxBufferedPosts   string `json:"sc_max_buffered_posts,omitempty"`
	SCMaxEachPostBytes   string `json:"sc_max_each_post_bytes,omitempty"`
}

type VLESSSecurityKind string

const (
	VLESSSecurityNone      VLESSSecurityKind = "none"
	VLESSSecurityTLS       VLESSSecurityKind = "tls"
	VLESSSecurityReality   VLESSSecurityKind = "reality"
	VLESSSecurityShadowTLS VLESSSecurityKind = "shadow-tls"
	VLESSSecurityResTLS    VLESSSecurityKind = "res-tls"
	VLESSSecurityJLS       VLESSSecurityKind = "jls"
)

type VLESSSecuritySpec struct {
	Type      VLESSSecurityKind `json:"type"`
	TLS       *TLSConfig        `json:"tls,omitempty"`
	Reality   *RealityConfig    `json:"reality,omitempty"`
	ShadowTLS *ShadowTLSConfig  `json:"shadow_tls,omitempty"`
	ResTLS    *ResTLSConfig     `json:"res_tls,omitempty"`
	JLS       *JLSConfig        `json:"jls,omitempty"`
}

type TLSConfig struct {
	Certificate    string `json:"certificate"`
	PrivateKey     string `json:"private_key"`
	ClientAuthType string `json:"client_auth_type,omitempty"`
	ClientAuthCert string `json:"client_auth_cert,omitempty"`
	ECHKey         string `json:"ech_key,omitempty"`
	AllowInsecure  bool   `json:"allow_insecure,omitempty"`
}

type RealityConfig struct {
	Destination           string               `json:"destination"`
	PrivateKey            string               `json:"private_key"`
	PublicKey             string               `json:"public_key"`
	ShortIDs              []string             `json:"short_ids"`
	ServerNames           []string             `json:"server_names"`
	MaxTimeDifference     int                  `json:"max_time_difference,omitempty"`
	Proxy                 string               `json:"proxy,omitempty"`
	LimitFallbackUpload   RealityFallbackLimit `json:"limit_fallback_upload,omitempty"`
	LimitFallbackDownload RealityFallbackLimit `json:"limit_fallback_download,omitempty"`
}

type RealityFallbackLimit struct {
	AfterBytes       uint64 `json:"after_bytes,omitempty"`
	BytesPerSec      uint64 `json:"bytes_per_sec,omitempty"`
	BurstBytesPerSec uint64 `json:"burst_bytes_per_sec,omitempty"`
}

type ShadowTLSConfig struct {
	Version                int                           `json:"version,omitempty"`
	Password               string                        `json:"password,omitempty"`
	Users                  []ShadowTLSUser               `json:"users,omitempty"`
	Handshake              ShadowTLSHandshake            `json:"handshake"`
	HandshakeForServerName map[string]ShadowTLSHandshake `json:"handshake_for_server_name,omitempty"`
	StrictMode             bool                          `json:"strict_mode,omitempty"`
	WildcardSNI            string                        `json:"wildcard_sni,omitempty"`
}

type ShadowTLSUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type ShadowTLSHandshake struct {
	Destination string `json:"destination"`
	Proxy       string `json:"proxy,omitempty"`
}

type ResTLSConfig struct {
	Destination     string `json:"destination"`
	Password        string `json:"password"`
	VersionHint     string `json:"version_hint,omitempty"`
	Script          string `json:"script,omitempty"`
	MinRecordLength int    `json:"min_record_length,omitempty"`
	Proxy           string `json:"proxy,omitempty"`
}

type JLSConfig struct {
	Users       []JLSUser `json:"users"`
	ServerName  string    `json:"server_name,omitempty"`
	Destination string    `json:"destination"`
	ALPN        []string  `json:"alpn,omitempty"`
	Proxy       string    `json:"proxy,omitempty"`
	RateLimit   uint64    `json:"rate_limit,omitempty"`
}

type JLSUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type MuxSpec struct {
	Padding bool       `json:"padding,omitempty"`
	Brutal  BrutalSpec `json:"brutal,omitempty"`
}

type BrutalSpec struct {
	Enabled bool   `json:"enabled,omitempty"`
	Up      string `json:"up,omitempty"`
	Down    string `json:"down,omitempty"`
}

type Hysteria2Spec struct {
	Obfs                  string                `json:"obfs,omitempty"`
	ObfsPassword          string                `json:"obfs_password,omitempty"`
	Certificate           string                `json:"certificate"`
	PrivateKey            string                `json:"private_key"`
	ClientAuthType        string                `json:"client_auth_type,omitempty"`
	ClientAuthCert        string                `json:"client_auth_cert,omitempty"`
	ECHKey                string                `json:"ech_key,omitempty"`
	MaxIdleTime           int                   `json:"max_idle_time,omitempty"`
	ALPN                  []string              `json:"alpn,omitempty"`
	Up                    string                `json:"up,omitempty"`
	Down                  string                `json:"down,omitempty"`
	IgnoreClientBandwidth bool                  `json:"ignore_client_bandwidth,omitempty"`
	Masquerade            string                `json:"masquerade,omitempty"`
	BBRProfile            string                `json:"bbr_profile,omitempty"`
	Mux                   MuxSpec               `json:"mux,omitempty"`
	Realm                 *Hysteria2RealmConfig `json:"realm,omitempty"`
	// Client-side fields (proxies-hysteria2 wiki). Populated by decodeHy2
	// from the listener config JSON so the m-ui client YAML emitter can
	// surface them on the outbound proxy entry.
	Ports             string `json:"ports,omitempty"`
	HopInterval       int    `json:"hop_interval,omitempty"`
	ObfsMinPacketSize int    `json:"obfs_min_packet_size,omitempty"`
	ObfsMaxPacketSize int    `json:"obfs_max_packet_size,omitempty"`
	NameCertVerify    string `json:"name_cert_verify,omitempty"`
	Fingerprint       string `json:"fingerprint,omitempty"`
	HandshakeTimeout  int    `json:"handshake_timeout,omitempty"`
}

type Hysteria2RealmConfig struct {
	Enabled        bool     `json:"enabled"`
	ServerURL      string   `json:"server_url,omitempty"`
	Token          string   `json:"token,omitempty"`
	RealmID        string   `json:"realm_id,omitempty"`
	STUNServers    []string `json:"stun_servers,omitempty"`
	ServerName     string   `json:"server_name,omitempty"`
	SkipCertVerify bool     `json:"skip_cert_verify,omitempty"`
	NameCertVerify string   `json:"name_cert_verify,omitempty"`
	Fingerprint    string   `json:"fingerprint,omitempty"`
	Certificate    string   `json:"certificate,omitempty"`
	PrivateKey     string   `json:"private_key,omitempty"`
	ALPN           []string `json:"alpn,omitempty"`
}

type Keypair struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

func (node Node) EffectiveUsers(asOf time.Time) []NodeUser {
	users := make([]NodeUser, 0, len(node.Users))
	for _, user := range node.Users {
		if !user.Enabled {
			continue
		}
		if user.ExpiresAt != nil && !user.ExpiresAt.After(asOf) {
			continue
		}
		users = append(users, user)
	}
	return users
}

func (node Node) DefaultAccessProfile() (AccessProfile, bool) {
	for _, profile := range node.AccessProfiles {
		if profile.Default {
			return profile, true
		}
	}
	return AccessProfile{}, false
}
