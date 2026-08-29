package protocol

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/mui/domain"
)

type VMessModule struct{}

func (VMessModule) Kind() domain.ProtocolKind { return domain.ProtocolVMess }
func (VMessModule) Capability() ProtocolCapability {
	return ProtocolCapability{Kind: domain.ProtocolVMess}
}

func (VMessModule) Compile(ctx context.Context, node domain.Node, asOf time.Time) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if node.VMess == nil {
		return nil, errors.New("VMess specification is missing")
	}
	spec := node.VMess
	listener := vmessListener{
		Name: node.Name, Type: "vmess", Listen: node.ListenAddress, Port: node.Port,
		Mux: compileMux(spec.Mux),
	}
	for _, user := range effectiveUsers(node, asOf) {
		if user.VMess == nil {
			return nil, fmt.Errorf("user %q is missing VMess credentials", user.Name)
		}
		listener.Users = append(listener.Users, vmessListenerUser{
			Username: user.Name, UUID: user.VMess.UUID, AlterID: user.VMess.AlterID,
		})
	}
	if err := applyClassicHandler(&listener.WSPath, &listener.GRPCServiceName, spec.Handler); err != nil {
		return nil, err
	}
	if spec.Handler.Type == domain.VMessHandlerMKCP {
		listener.MKCP = compileMKCP(*spec.Handler.MKCP, true)
	}
	if err := listener.Security.apply(spec.Security, false); err != nil {
		return nil, err
	}
	return listener, nil
}

func (VMessModule) BuildShare(state domain.DesiredState, node domain.Node, user domain.NodeUser, profile domain.AccessProfile) (Share, error) {
	if node.VMess == nil || user.VMess == nil {
		return Share{}, errors.New("VMess share requires VMess node and user credentials")
	}
	host := shareHost(state, profile)
	network, path, serviceName := classicTransportShare(specHandler(node.VMess.Handler))
	security := string(node.VMess.Security.Type)
	if security == string(domain.VLESSSecurityNone) {
		security = ""
	}
	payload := map[string]any{
		"v": "2", "ps": node.Name + " - " + user.Name, "add": host,
		"port": sharePortString(profile, node.Port), "id": user.VMess.UUID,
		"aid": strconv.Itoa(user.VMess.AlterID), "scy": defaultCipher(user.VMess.Cipher),
		"net": network, "type": "none", "host": profile.ServerName, "path": path,
		"tls": security, "sni": profile.ServerName, "fp": profile.Fingerprint,
	}
	if serviceName != "" {
		payload["path"] = serviceName
	}
	if node.VMess.Handler.Type == domain.VMessHandlerMKCP {
		payload["type"] = node.VMess.Handler.MKCP.Header
		payload["path"] = node.VMess.Handler.MKCP.Seed
	}
	if node.VMess.Security.Type == domain.VLESSSecurityReality && node.VMess.Security.Reality != nil {
		r := node.VMess.Security.Reality
		pbk := r.PublicKey
		if pbk == "" && r.PrivateKey != "" {
			if derived, err := deriveShareRealityPublicKey(r.PrivateKey); err == nil {
				pbk = derived
			}
		}
		if pbk != "" {
			payload["pbk"] = pbk
		}
		if len(r.ShortIDs) > 0 {
			payload["sid"] = r.ShortIDs[0]
		}
		if payload["sni"] == "" && len(r.ServerNames) > 0 {
			payload["sni"] = r.ServerNames[0]
		}
		if payload["fp"] == "" || payload["fp"] == nil {
			payload["fp"] = "chrome"
		}
		payload["tls"] = "reality"
	}
	// Standard VMess share uses decimal port string; fall back to node listen port.
	if portStr, _ := payload["port"].(string); portStr == "" || portStr == "0" {
		payload["port"] = sharePortString(profile, node.Port)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Share{}, fmt.Errorf("encode VMess share: %w", err)
	}
	// v2rayN accepts both Std and Raw; StdEncoding with padding is most widely compatible.
	uri := "vmess://" + base64.StdEncoding.EncodeToString(encoded)
	clientYAML, err := compileClassicClient(node, user, profile, host)
	if err != nil {
		return Share{}, err
	}
	return Share{URI: uri, QRContent: uri, ClientYAML: clientYAML}, nil
}

type TrojanModule struct{}

func (TrojanModule) Kind() domain.ProtocolKind { return domain.ProtocolTrojan }
func (TrojanModule) Capability() ProtocolCapability {
	return ProtocolCapability{Kind: domain.ProtocolTrojan}
}

func (TrojanModule) Compile(ctx context.Context, node domain.Node, asOf time.Time) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if node.Trojan == nil {
		return nil, errors.New("trojan specification is missing")
	}
	spec := node.Trojan
	listener := trojanListener{
		Name: node.Name, Type: "trojan", Listen: node.ListenAddress, Port: node.Port,
		Mux: compileMux(spec.Mux),
	}
	for _, user := range effectiveUsers(node, asOf) {
		if user.Trojan == nil {
			return nil, fmt.Errorf("user %q is missing Trojan credentials", user.Name)
		}
		listener.Users = append(listener.Users, trojanListenerUser{Username: user.Name, Password: user.Trojan.Password})
	}
	if err := applyClassicHandler(&listener.WSPath, &listener.GRPCServiceName, spec.Handler); err != nil {
		return nil, err
	}
	if err := listener.Security.apply(spec.Security, true); err != nil {
		return nil, err
	}
	if spec.Shadowsocks.Enabled {
		listener.Shadowsocks = &trojanShadowsocksListener{Enabled: true, Method: spec.Shadowsocks.Method, Password: spec.Shadowsocks.Password}
	}
	return listener, nil
}

func (TrojanModule) BuildShare(state domain.DesiredState, node domain.Node, user domain.NodeUser, profile domain.AccessProfile) (Share, error) {
	if node.Trojan == nil || user.Trojan == nil {
		return Share{}, errors.New("trojan share requires Trojan node and user credentials")
	}
	host := shareHost(state, profile)
	query := url.Values{}
	network, path, serviceName := classicTransportShare(specHandler(node.Trojan.Handler))
	query.Set("type", network)
	if path != "" {
		query.Set("path", path)
	}
	if serviceName != "" {
		query.Set("serviceName", serviceName)
	}
	applySecurityQuery(query, node.Trojan.Security, profile)
	if node.Trojan.Shadowsocks.Enabled {
		query.Set("encryption", "ss;"+node.Trojan.Shadowsocks.Method+":"+node.Trojan.Shadowsocks.Password)
	}
	uri := (&url.URL{
		Scheme: "trojan", User: url.User(user.Trojan.Password),
		Host:     net.JoinHostPort(host, sharePortString(profile, node.Port)),
		RawQuery: query.Encode(), Fragment: node.Name + " - " + user.Name,
	}).String()
	clientYAML, err := compileClassicClient(node, user, profile, host)
	if err != nil {
		return Share{}, err
	}
	return Share{URI: uri, QRContent: uri, ClientYAML: clientYAML}, nil
}

type classicSecurityListener struct {
	Certificate    string           `yaml:"certificate,omitempty"`
	PrivateKey     string           `yaml:"private-key,omitempty"`
	ClientAuthType string           `yaml:"client-auth-type,omitempty"`
	ClientAuthCert string           `yaml:"client-auth-cert,omitempty"`
	ECHKey         string           `yaml:"ech-key,omitempty"`
	AllowInsecure  bool             `yaml:"allow-insecure,omitempty"`
	ShadowTLS      *shadowTLSConfig `yaml:"shadow-tls,omitempty"`
	ResTLS         *resTLSConfig    `yaml:"res-tls,omitempty"`
	JLS            *jlsConfig       `yaml:"jls-config,omitempty"`
	Reality        *realityConfig   `yaml:"reality-config,omitempty"`
}

func (listener *classicSecurityListener) apply(spec domain.VLESSSecuritySpec, allowNone bool) error {
	switch spec.Type {
	case domain.VLESSSecurityNone:
		listener.AllowInsecure = allowNone
	case domain.VLESSSecurityTLS:
		listener.Certificate, listener.PrivateKey = spec.TLS.Certificate, spec.TLS.PrivateKey
		listener.ClientAuthType, listener.ClientAuthCert = spec.TLS.ClientAuthType, spec.TLS.ClientAuthCert
		listener.ECHKey = spec.TLS.ECHKey
		if allowNone {
			listener.AllowInsecure = spec.TLS.AllowInsecure
		}
	case domain.VLESSSecurityReality:
		listener.Reality = compileReality(*spec.Reality)
	case domain.VLESSSecurityShadowTLS:
		listener.ShadowTLS = compileShadowTLS(*spec.ShadowTLS)
	case domain.VLESSSecurityResTLS:
		listener.ResTLS = compileResTLS(*spec.ResTLS)
	case domain.VLESSSecurityJLS:
		listener.JLS = compileJLS(*spec.JLS)
	default:
		return fmt.Errorf("unsupported stream security %q", spec.Type)
	}
	return nil
}

type vmessListener struct {
	Name            string                  `yaml:"name"`
	Type            string                  `yaml:"type"`
	Listen          string                  `yaml:"listen"`
	Port            string                  `yaml:"port"`
	Users           []vmessListenerUser     `yaml:"users"`
	WSPath          string                  `yaml:"ws-path,omitempty"`
	GRPCServiceName string                  `yaml:"grpc-service-name,omitempty"`
	MKCP            *mkcpListenerConfig     `yaml:"mkcp-config,omitempty"`
	Security        classicSecurityListener `yaml:",inline"`
	Mux             *muxConfig              `yaml:"mux-option,omitempty"`
}
type vmessListenerUser struct {
	Username string `yaml:"username,omitempty"`
	UUID     string `yaml:"uuid"`
	AlterID  int    `yaml:"alterId,omitempty"`
}

type trojanListener struct {
	Name            string                     `yaml:"name"`
	Type            string                     `yaml:"type"`
	Listen          string                     `yaml:"listen"`
	Port            string                     `yaml:"port"`
	Users           []trojanListenerUser       `yaml:"users"`
	WSPath          string                     `yaml:"ws-path,omitempty"`
	GRPCServiceName string                     `yaml:"grpc-service-name,omitempty"`
	Security        classicSecurityListener    `yaml:",inline"`
	Mux             *muxConfig                 `yaml:"mux-option,omitempty"`
	Shadowsocks     *trojanShadowsocksListener `yaml:"ss-option,omitempty"`
}
type trojanListenerUser struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password"`
}
type trojanShadowsocksListener struct {
	Enabled  bool   `yaml:"enabled"`
	Method   string `yaml:"method"`
	Password string `yaml:"password"`
}

func applyClassicHandler(wsPath, grpcServiceName *string, handler domain.VLESSHandlerSpec) error {
	switch handler.Type {
	case domain.VLESSHandlerRaw:
	case domain.VLESSHandlerWebSocket:
		*wsPath = handler.WebSocket.Path
	case domain.VLESSHandlerGRPC:
		*grpcServiceName = handler.GRPC.ServiceName
	case domain.VMessHandlerMKCP:
	default:
		return fmt.Errorf("unsupported classic stream handler %q", handler.Type)
	}
	return nil
}

type classicClientDocument struct {
	Proxies []classicClientProxy `yaml:"proxies"`
}
type classicClientProxy struct {
	Name              string              `yaml:"name"`
	Type              string              `yaml:"type"`
	Server            string              `yaml:"server"`
	Port              uint16              `yaml:"port"`
	UDP               bool                `yaml:"udp"`
	UUID              string              `yaml:"uuid,omitempty"`
	AlterID           int                 `yaml:"alterId"`
	Cipher            string              `yaml:"cipher,omitempty"`
	Password          string              `yaml:"password,omitempty"`
	Network           string              `yaml:"network,omitempty"`
	TLS               bool                `yaml:"tls,omitempty"`
	ServerName        string              `yaml:"servername,omitempty"`
	SNI               string              `yaml:"sni,omitempty"`
	ClientFingerprint string              `yaml:"client-fingerprint,omitempty"`
	SkipCertVerify    bool                `yaml:"skip-cert-verify,omitempty"`
	PacketEncoding    string              `yaml:"packet-encoding,omitempty"`
	Reality           *vlessClientReality `yaml:"reality-opts,omitempty"`
	ShadowTLS         *vlessClientSecret  `yaml:"shadow-tls-opts,omitempty"`
	ResTLS            *vlessClientResTLS  `yaml:"restls-opts,omitempty"`
	JLS               *vlessClientJLS     `yaml:"jls-opts,omitempty"`
	WS                *websocketClient    `yaml:"ws-opts,omitempty"`
	GRPC              *grpcClient         `yaml:"grpc-opts,omitempty"`
	MKCP              *mkcpClientConfig   `yaml:"mkcp-opts,omitempty"`
	TrojanShadowsocks *trojanClientSS     `yaml:"ss-opts,omitempty"`
}

func compileClassicClient(node domain.Node, user domain.NodeUser, profile domain.AccessProfile, host string) ([]byte, error) {
	proxy := classicClientProxy{
		Name: node.Name + " - " + user.Name, Type: string(node.Protocol), Server: host,
		Port: profile.PublicPort, UDP: true,
		ClientFingerprint: profile.Fingerprint, SkipCertVerify: profile.AllowInsecure,
	}
	var handler domain.VLESSHandlerSpec
	var security domain.VLESSSecuritySpec
	switch node.Protocol {
	case domain.ProtocolVMess:
		proxy.UUID, proxy.AlterID, proxy.Cipher = user.VMess.UUID, user.VMess.AlterID, defaultCipher(user.VMess.Cipher)
		proxy.ServerName = profile.ServerName
		proxy.PacketEncoding = profile.PacketEncoding
		handler, security = node.VMess.Handler, node.VMess.Security
	case domain.ProtocolTrojan:
		proxy.Password = user.Trojan.Password
		proxy.SNI = profile.ServerName
		handler, security = node.Trojan.Handler, node.Trojan.Security
		if node.Trojan.Shadowsocks.Enabled {
			proxy.TrojanShadowsocks = &trojanClientSS{
				Enabled: true, Method: node.Trojan.Shadowsocks.Method,
				Password: node.Trojan.Shadowsocks.Password,
			}
		}
	default:
		return nil, fmt.Errorf("unsupported classic client protocol %q", node.Protocol)
	}
	switch handler.Type {
	case domain.VLESSHandlerRaw:
		proxy.Network = "tcp"
	case domain.VLESSHandlerWebSocket:
		proxy.Network, proxy.WS = "ws", &websocketClient{Path: handler.WebSocket.Path}
	case domain.VLESSHandlerGRPC:
		proxy.Network, proxy.GRPC = "grpc", &grpcClient{ServiceName: handler.GRPC.ServiceName}
	case domain.VMessHandlerMKCP:
		proxy.Network, proxy.MKCP = "mkcp", compileMKCPClient(*handler.MKCP)
	}
	applyClassicClientSecurity(&proxy, security, user, profile)
	if node.Protocol == domain.ProtocolTrojan {
		// Mihomo's Trojan outbound is intrinsically TLS-capable and has no
		// `tls` option; the listener's selected wrapper is represented by the
		// specific reality/shadow/restls/jls fields instead.
		proxy.TLS = false
	}
	return encodeClientYAML(classicClientDocument{Proxies: []classicClientProxy{proxy}})
}

type trojanClientSS struct {
	Enabled  bool   `yaml:"enabled"`
	Method   string `yaml:"method"`
	Password string `yaml:"password"`
}

func applyClassicClientSecurity(proxy *classicClientProxy, security domain.VLESSSecuritySpec, user domain.NodeUser, profile domain.AccessProfile) {
	if security.Type == domain.VLESSSecurityNone {
		return
	}
	proxy.TLS = true
	switch security.Type {
	case domain.VLESSSecurityTLS:
		proxy.SkipCertVerify = proxy.SkipCertVerify || security.TLS.AllowInsecure
	case domain.VLESSSecurityReality:
		shortID := ""
		if len(security.Reality.ShortIDs) > 0 {
			shortID = security.Reality.ShortIDs[0]
		}
		proxy.Reality = &vlessClientReality{PublicKey: security.Reality.PublicKey, ShortID: shortID}
	case domain.VLESSSecurityShadowTLS:
		password := security.ShadowTLS.Password
		if security.ShadowTLS.Version == 3 && len(security.ShadowTLS.Users) > 0 {
			password = security.ShadowTLS.Users[0].Password
			for _, candidate := range security.ShadowTLS.Users {
				if candidate.Name == user.Name {
					password = candidate.Password
					break
				}
			}
		}
		proxy.ShadowTLS = &vlessClientSecret{Version: security.ShadowTLS.Version, Password: password}
	case domain.VLESSSecurityResTLS:
		proxy.ResTLS = &vlessClientResTLS{Password: security.ResTLS.Password, VersionHint: defaultStringValue(security.ResTLS.VersionHint, "tls13"), Script: security.ResTLS.Script}
	case domain.VLESSSecurityJLS:
		if len(security.JLS.Users) > 0 {
			candidate := security.JLS.Users[0]
			for _, item := range security.JLS.Users {
				if item.Username == user.Name {
					candidate = item
					break
				}
			}
			proxy.JLS = &vlessClientJLS{Username: candidate.Username, Password: candidate.Password}
		}
	}
	_ = profile
}

func applySecurityQuery(query url.Values, security domain.VLESSSecuritySpec, profile domain.AccessProfile) {
	sec := string(security.Type)
	if sec == "" || sec == string(domain.VLESSSecurityNone) {
		// Trojan/VMess clients typically expect explicit tls when certificates are used.
		if security.TLS != nil {
			sec = "tls"
		}
	}
	if sec != "" && sec != string(domain.VLESSSecurityNone) {
		query.Set("security", sec)
	}
	if profile.ServerName != "" {
		query.Set("sni", profile.ServerName)
	}
	if profile.Fingerprint != "" {
		query.Set("fp", profile.Fingerprint)
	} else if security.Type == domain.VLESSSecurityReality {
		query.Set("fp", "chrome")
	}
	if security.Type == domain.VLESSSecurityTLS && security.TLS != nil && (profile.AllowInsecure || security.TLS.AllowInsecure) {
		query.Set("allowInsecure", "1")
	}
	if security.Type == domain.VLESSSecurityReality && security.Reality != nil {
		r := security.Reality
		pbk := r.PublicKey
		if pbk == "" && r.PrivateKey != "" {
			if derived, err := deriveShareRealityPublicKey(r.PrivateKey); err == nil {
				pbk = derived
			}
		}
		if pbk != "" {
			query.Set("pbk", pbk)
		}
		if len(r.ShortIDs) > 0 {
			query.Set("sid", r.ShortIDs[0])
		}
		if query.Get("sni") == "" && len(r.ServerNames) > 0 {
			query.Set("sni", r.ServerNames[0])
		}
	}
}

// sharePortString returns the client-facing port for standard share URIs.
func sharePortString(profile domain.AccessProfile, nodePort string) string {
	if profile.PublicPort > 0 {
		return strconv.Itoa(int(profile.PublicPort))
	}
	if p := strings.TrimSpace(nodePort); p != "" && p != "0" {
		return p
	}
	return "0"
}

func shareHost(state domain.DesiredState, profile domain.AccessProfile) string {
	if profile.PublicHost != "" {
		return profile.PublicHost
	}
	return state.PublicHost
}
func defaultCipher(value string) string { return defaultStringValue(value, "auto") }
func defaultStringValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func specHandler(handler domain.VLESSHandlerSpec) domain.VLESSHandlerSpec { return handler }
func classicTransportShare(handler domain.VLESSHandlerSpec) (network, path, serviceName string) {
	switch handler.Type {
	case domain.VLESSHandlerWebSocket:
		return "ws", handler.WebSocket.Path, ""
	case domain.VLESSHandlerGRPC:
		return "grpc", "", handler.GRPC.ServiceName
	case domain.VMessHandlerMKCP:
		return "mkcp", handler.MKCP.Seed, ""
	default:
		return "tcp", "", ""
	}
}

type mkcpListenerConfig struct {
	Enable           bool   `yaml:"enable"`
	MTU              uint32 `yaml:"mtu,omitempty"`
	TTI              uint32 `yaml:"tti,omitempty"`
	UplinkCapacity   uint32 `yaml:"uplink-capacity,omitempty"`
	DownlinkCapacity uint32 `yaml:"downlink-capacity,omitempty"`
	Congestion       bool   `yaml:"congestion,omitempty"`
	WriteBuffer      uint32 `yaml:"write-buffer,omitempty"`
	ReadBuffer       uint32 `yaml:"read-buffer,omitempty"`
	Seed             string `yaml:"seed,omitempty"`
	Header           string `yaml:"header,omitempty"`
}

type mkcpClientConfig struct {
	MTU              uint32 `yaml:"mtu,omitempty"`
	TTI              uint32 `yaml:"tti,omitempty"`
	UplinkCapacity   uint32 `yaml:"uplink-capacity,omitempty"`
	DownlinkCapacity uint32 `yaml:"downlink-capacity,omitempty"`
	Congestion       bool   `yaml:"congestion,omitempty"`
	WriteBuffer      uint32 `yaml:"write-buffer,omitempty"`
	ReadBuffer       uint32 `yaml:"read-buffer,omitempty"`
	Seed             string `yaml:"seed,omitempty"`
	Header           string `yaml:"header,omitempty"`
}

func compileMKCP(config domain.MKCPConfig, enabled bool) *mkcpListenerConfig {
	return &mkcpListenerConfig{
		Enable: enabled, MTU: config.MTU, TTI: config.TTI,
		UplinkCapacity: config.UplinkCapacity, DownlinkCapacity: config.DownlinkCapacity,
		Congestion: config.Congestion, WriteBuffer: config.WriteBuffer,
		ReadBuffer: config.ReadBuffer, Seed: config.Seed, Header: config.Header,
	}
}

func compileMKCPClient(config domain.MKCPConfig) *mkcpClientConfig {
	return &mkcpClientConfig{
		MTU: config.MTU, TTI: config.TTI, UplinkCapacity: config.UplinkCapacity,
		DownlinkCapacity: config.DownlinkCapacity, Congestion: config.Congestion,
		WriteBuffer: config.WriteBuffer, ReadBuffer: config.ReadBuffer,
		Seed: config.Seed, Header: config.Header,
	}
}
