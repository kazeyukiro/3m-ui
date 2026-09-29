package protocol

import "strings"

// NodeModel is the strongly-typed node view used by the protocol registry.
// Listener.Config remains JSON on disk; DecodeNodeModel maps it into this shape.
type NodeModel struct {
	Name        string
	Protocol    string
	Listen      string
	Port        string
	PublicHost  string
	PublicPort  string
	AccessSNI   string
	Fingerprint string
	AccessALPN  string
	Enabled     bool
	UDP         bool
	TLS         bool

	Users []UserCred

	VLESS       *VLESSSpec
	VMess       *VMessSpec
	Trojan      *TrojanSpec
	Shadowsocks *ShadowsocksSpec
	Hysteria2   *Hysteria2Spec
	Generic     map[string]interface{} // passthrough for less common protocols
}

type RealitySpec struct {
	PublicKey  string
	PrivateKey string
	ShortID    string
	ServerName string // first of server-names
}

type TransportSpec struct {
	// Network: tcp | ws | grpc | xhttp (empty = tcp)
	Network     string
	WSPath      string
	WSHost      string
	GRPCService string
	XHTTPPath   string
}

// CarriesFlow reports whether this transport can carry VLESS flow (XTLS
// Vision). Vision is TCP-only: the core rejects it on ws, grpc and xhttp, and a
// client that honours the flow simply fails to connect. Every place that emits a
// flow has to ask this rather than trust the stored configuration, because a
// listener whose transport was switched after the flow was set keeps the stale
// value.
func (t TransportSpec) CarriesFlow() bool {
	switch strings.ToLower(strings.TrimSpace(t.Network)) {
	case "", "tcp", "raw":
		return true
	default:
		return false
	}
}

// TransportCarriesFlow answers the same question for a raw listener config, which
// records its transport as the presence of a per-transport block rather than as a
// single named field.
func TransportCarriesFlow(cfg map[string]interface{}) bool {
	for _, key := range []string{"ws-path", "grpc-service-name", "xhttp-config", "mkcp-config", "mekya-config"} {
		value, ok := cfg[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return false
			}
		case map[string]interface{}:
			if len(typed) > 0 {
				return false
			}
		default:
			// A block is present in some other shape; treat it as configured.
			return false
		}
	}
	return true
}

type VLESSSpec struct {
	Encryption  string
	Flow        string // default flow applied when user flow empty
	Transport   TransportSpec
	Reality     *RealitySpec
	SkipCert    bool
	SNI         string
	Fingerprint string
	ALPN        []string
}

type VMessSpec struct {
	Cipher      string
	AlterID     int
	Transport   TransportSpec
	Reality     *RealitySpec
	SkipCert    bool
	SNI         string
	Fingerprint string
	ALPN        []string
}

type TrojanSpec struct {
	Transport   TransportSpec
	Reality     *RealitySpec
	SkipCert    bool
	SNI         string
	Fingerprint string
	ALPN        []string
}

type ShadowsocksSpec struct {
	Cipher   string
	Password string // when not using per-user creds
	UDP      bool
}

type Hysteria2Spec struct {
	SNI          string
	SkipCert     bool
	Obfs         string
	ObfsPassword string
	Up           string
	Down         string
	ALPN         []string
}

// Share is the share payload.
type Share struct {
	URI        string
	QRContent  string
	ClientYAML string
}

// ShareInput is everything needed to build a client share for one user.
type ShareInput struct {
	Node NodeModel
	User UserCred
}
