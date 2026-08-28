package protocol

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

	VLESS        *VLESSSpec
	VMess        *VMessSpec
	Trojan       *TrojanSpec
	Shadowsocks  *ShadowsocksSpec
	Hysteria2    *Hysteria2Spec
	Generic      map[string]interface{} // passthrough for less common protocols
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

type VLESSSpec struct {
	Encryption string
	Flow       string // default flow applied when user flow empty
	Transport  TransportSpec
	Reality    *RealitySpec
	SkipCert   bool
	SNI        string
	Fingerprint string
}

type VMessSpec struct {
	Cipher     string
	AlterID    int
	Transport  TransportSpec
	Reality    *RealitySpec
	SkipCert   bool
	SNI        string
	Fingerprint string
}

type TrojanSpec struct {
	Transport   TransportSpec
	Reality     *RealitySpec
	SkipCert    bool
	SNI         string
	Fingerprint string
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
}

// Share is the m-ui style share payload.
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
