package protocol

import (
	"bytes"
	"encoding/base64"
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/mui/domain"
	"golang.org/x/crypto/curve25519"
	"gopkg.in/yaml.v3"
)

type VLESSModule struct{}

func (VLESSModule) Kind() domain.ProtocolKind { return domain.ProtocolVLESS }

func (VLESSModule) Capability() ProtocolCapability {
	return ProtocolCapability{Kind: domain.ProtocolVLESS}
}

func (VLESSModule) Compile(
	ctx context.Context,
	node domain.Node,
	asOf time.Time,
) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if node.VLESS == nil {
		return nil, errors.New("VLESS specification is missing")
	}
	spec := node.VLESS
	listener := vlessListener{
		Name:       node.Name,
		Type:       "vless",
		Listen:     node.ListenAddress,
		Port:       node.Port,
		Decryption: spec.Decryption,
		Mux:        compileMux(spec.Mux),
	}
	if listener.Decryption == "" {
		listener.Decryption = "none"
	}
	for _, user := range effectiveUsers(node, asOf) {
		if user.VLESS == nil {
			return nil, fmt.Errorf("user %q is missing VLESS credentials", user.Name)
		}
		listener.Users = append(listener.Users, vlessUser{
			Username: user.Name,
			UUID:     user.VLESS.UUID,
			Flow:     user.VLESS.Flow,
		})
	}
	switch spec.Handler.Type {
	case domain.VLESSHandlerRaw:
	case domain.VLESSHandlerWebSocket:
		listener.WSPath = spec.Handler.WebSocket.Path
	case domain.VLESSHandlerGRPC:
		listener.GRPCServiceName = spec.Handler.GRPC.ServiceName
	case domain.VLESSHandlerXHTTP:
		listener.XHTTP = compileXHTTP(*spec.Handler.XHTTP)
	default:
		return nil, fmt.Errorf("unsupported VLESS handler %q", spec.Handler.Type)
	}
	switch spec.Security.Type {
	case domain.VLESSSecurityNone:
	case domain.VLESSSecurityTLS:
		listener.applyTLS(*spec.Security.TLS)
	case domain.VLESSSecurityReality:
		listener.Reality = compileReality(*spec.Security.Reality)
	case domain.VLESSSecurityShadowTLS:
		listener.ShadowTLS = compileShadowTLS(*spec.Security.ShadowTLS)
	case domain.VLESSSecurityResTLS:
		listener.ResTLS = compileResTLS(*spec.Security.ResTLS)
	case domain.VLESSSecurityJLS:
		listener.JLS = compileJLS(*spec.Security.JLS)
	default:
		return nil, fmt.Errorf("unsupported VLESS security %q", spec.Security.Type)
	}
	return listener, nil
}

func (VLESSModule) BuildShare(
	state domain.DesiredState,
	node domain.Node,
	user domain.NodeUser,
	profile domain.AccessProfile,
) (Share, error) {
	if node.VLESS == nil || user.VLESS == nil {
		return Share{}, errors.New("VLESS share requires VLESS node and user credentials")
	}
	host := profile.PublicHost
	if host == "" {
		host = state.PublicHost
	}
	query := url.Values{}
	query.Set("encryption", normalizedDecryption(node.VLESS.Decryption))
	query.Set("type", string(node.VLESS.Handler.Type))
	switch node.VLESS.Handler.Type {
	case domain.VLESSHandlerRaw:
		query.Set("type", "tcp")
	case domain.VLESSHandlerWebSocket:
		query.Set("type", "ws")
	}
	if profile.ServerName != "" {
		query.Set("sni", profile.ServerName)
	}
	if profile.Fingerprint != "" {
		query.Set("fp", profile.Fingerprint)
	}
	if profile.PacketEncoding != "" {
		query.Set("packetEncoding", profile.PacketEncoding)
	}
	if user.VLESS.Flow != "" {
		query.Set("flow", user.VLESS.Flow)
	}
	switch node.VLESS.Handler.Type {
	case domain.VLESSHandlerWebSocket:
		query.Set("path", node.VLESS.Handler.WebSocket.Path)
	case domain.VLESSHandlerGRPC:
		query.Set("serviceName", node.VLESS.Handler.GRPC.ServiceName)
	case domain.VLESSHandlerXHTTP:
		query.Set("path", node.VLESS.Handler.XHTTP.Path)
		query.Set("host", node.VLESS.Handler.XHTTP.Host)
		query.Set("mode", node.VLESS.Handler.XHTTP.Mode)
	}
	query.Set("security", string(node.VLESS.Security.Type))
	switch node.VLESS.Security.Type {
	case domain.VLESSSecurityReality:
		query.Set("security", "reality")
		reality := node.VLESS.Security.Reality
		if reality != nil {
			pbk := reality.PublicKey
			if pbk == "" && reality.PrivateKey != "" {
				if derived, err := deriveShareRealityPublicKey(reality.PrivateKey); err == nil {
					pbk = derived
				}
			}
			if pbk != "" {
				query.Set("pbk", pbk)
			}
			if len(reality.ShortIDs) > 0 {
				query.Set("sid", reality.ShortIDs[0])
			}
			if query.Get("sni") == "" && len(reality.ServerNames) > 0 {
				query.Set("sni", reality.ServerNames[0])
			}
			if query.Get("fp") == "" {
				query.Set("fp", "chrome")
			}
		}
	case domain.VLESSSecurityTLS:
		query.Set("security", "tls")
		if profile.AllowInsecure || (node.VLESS.Security.TLS != nil && node.VLESS.Security.TLS.AllowInsecure) {
			query.Set("allowInsecure", "1")
		}
	}
	port := int(profile.PublicPort)
	if port <= 0 {
		port = int(node.Port)
	}
	uri := (&url.URL{
		Scheme:   "vless",
		User:     url.User(user.VLESS.UUID),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		RawQuery: query.Encode(),
		Fragment: node.Name + " - " + user.Name,
	}).String()
	clientYAML, err := compileVLESSClient(node, user, profile, host)
	if err != nil {
		return Share{}, err
	}
	return Share{URI: uri, QRContent: uri, ClientYAML: clientYAML}, nil
}

type vlessListener struct {
	Name            string           `yaml:"name"`
	Type            string           `yaml:"type"`
	Listen          string           `yaml:"listen"`
	Port            string           `yaml:"port"`
	Users           []vlessUser      `yaml:"users"`
	Decryption      string           `yaml:"decryption,omitempty"`
	WSPath          string           `yaml:"ws-path,omitempty"`
	XHTTP           *xhttpConfig     `yaml:"xhttp-config,omitempty"`
	GRPCServiceName string           `yaml:"grpc-service-name,omitempty"`
	Certificate     string           `yaml:"certificate,omitempty"`
	PrivateKey      string           `yaml:"private-key,omitempty"`
	ClientAuthType  string           `yaml:"client-auth-type,omitempty"`
	ClientAuthCert  string           `yaml:"client-auth-cert,omitempty"`
	ECHKey          string           `yaml:"ech-key,omitempty"`
	AllowInsecure   bool             `yaml:"allow-insecure,omitempty"`
	ShadowTLS       *shadowTLSConfig `yaml:"shadow-tls,omitempty"`
	ResTLS          *resTLSConfig    `yaml:"res-tls,omitempty"`
	JLS             *jlsConfig       `yaml:"jls-config,omitempty"`
	Reality         *realityConfig   `yaml:"reality-config,omitempty"`
	Mux             *muxConfig       `yaml:"mux-option,omitempty"`
}

func (listener *vlessListener) applyTLS(config domain.TLSConfig) {
	listener.Certificate = config.Certificate
	listener.PrivateKey = config.PrivateKey
	listener.ClientAuthType = config.ClientAuthType
	listener.ClientAuthCert = config.ClientAuthCert
	listener.ECHKey = config.ECHKey
	listener.AllowInsecure = config.AllowInsecure
}

type vlessUser struct {
	Username string `yaml:"username,omitempty"`
	UUID     string `yaml:"uuid"`
	Flow     string `yaml:"flow,omitempty"`
}

type realityConfig struct {
	Destination           string                `yaml:"dest"`
	PrivateKey            string                `yaml:"private-key"`
	ShortIDs              []string              `yaml:"short-id"`
	ServerNames           []string              `yaml:"server-names"`
	MaxTimeDifference     int                   `yaml:"max-time-difference,omitempty"`
	Proxy                 string                `yaml:"proxy,omitempty"`
	LimitFallbackUpload   *realityFallbackLimit `yaml:"limit-fallback-upload,omitempty"`
	LimitFallbackDownload *realityFallbackLimit `yaml:"limit-fallback-download,omitempty"`
}

type realityFallbackLimit struct {
	AfterBytes       uint64 `yaml:"after-bytes,omitempty"`
	BytesPerSec      uint64 `yaml:"bytes-per-sec,omitempty"`
	BurstBytesPerSec uint64 `yaml:"burst-bytes-per-sec,omitempty"`
}

func compileReality(config domain.RealityConfig) *realityConfig {
	return &realityConfig{
		Destination:           config.Destination,
		PrivateKey:            config.PrivateKey,
		ShortIDs:              append([]string(nil), config.ShortIDs...),
		ServerNames:           append([]string(nil), config.ServerNames...),
		MaxTimeDifference:     config.MaxTimeDifference,
		Proxy:                 config.Proxy,
		LimitFallbackUpload:   compileFallbackLimit(config.LimitFallbackUpload),
		LimitFallbackDownload: compileFallbackLimit(config.LimitFallbackDownload),
	}
}

func compileFallbackLimit(config domain.RealityFallbackLimit) *realityFallbackLimit {
	if config == (domain.RealityFallbackLimit{}) {
		return nil
	}
	return &realityFallbackLimit{AfterBytes: config.AfterBytes, BytesPerSec: config.BytesPerSec, BurstBytesPerSec: config.BurstBytesPerSec}
}

type shadowTLSConfig struct {
	Enable                 bool                          `yaml:"enable"`
	Version                int                           `yaml:"version,omitempty"`
	Password               string                        `yaml:"password,omitempty"`
	Users                  []shadowTLSUser               `yaml:"users,omitempty"`
	Handshake              shadowTLSHandshake            `yaml:"handshake"`
	HandshakeForServerName map[string]shadowTLSHandshake `yaml:"handshake-for-server-name,omitempty"`
	StrictMode             bool                          `yaml:"strict-mode,omitempty"`
	WildcardSNI            string                        `yaml:"wildcard-sni,omitempty"`
}

type shadowTLSUser struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
}

type shadowTLSHandshake struct {
	Destination string `yaml:"dest"`
	Proxy       string `yaml:"proxy,omitempty"`
}

func compileShadowTLS(config domain.ShadowTLSConfig) *shadowTLSConfig {
	compiled := &shadowTLSConfig{
		Enable:      true,
		Version:     config.Version,
		Password:    config.Password,
		Handshake:   shadowTLSHandshake{Destination: config.Handshake.Destination, Proxy: config.Handshake.Proxy},
		StrictMode:  config.StrictMode,
		WildcardSNI: config.WildcardSNI,
	}
	for _, user := range config.Users {
		compiled.Users = append(compiled.Users, shadowTLSUser{Name: user.Name, Password: user.Password})
	}
	if len(config.HandshakeForServerName) > 0 {
		compiled.HandshakeForServerName = make(map[string]shadowTLSHandshake, len(config.HandshakeForServerName))
		for name, handshake := range config.HandshakeForServerName {
			compiled.HandshakeForServerName[name] = shadowTLSHandshake{Destination: handshake.Destination, Proxy: handshake.Proxy}
		}
	}
	return compiled
}

type resTLSConfig struct {
	Enable          bool   `yaml:"enable"`
	Destination     string `yaml:"dest"`
	Password        string `yaml:"password"`
	Script          string `yaml:"restls-script,omitempty"`
	MinRecordLength int    `yaml:"min-record-len,omitempty"`
	Proxy           string `yaml:"proxy,omitempty"`
}

func compileResTLS(config domain.ResTLSConfig) *resTLSConfig {
	return &resTLSConfig{Enable: true, Destination: config.Destination, Password: config.Password, Script: config.Script, MinRecordLength: config.MinRecordLength, Proxy: config.Proxy}
}

type jlsConfig struct {
	Enable      bool      `yaml:"enable"`
	Users       []jlsUser `yaml:"users"`
	ServerName  string    `yaml:"sni,omitempty"`
	Destination string    `yaml:"dest"`
	ALPN        []string  `yaml:"alpn,omitempty"`
	Proxy       string    `yaml:"proxy,omitempty"`
	RateLimit   uint64    `yaml:"rate-limit,omitempty"`
}

type jlsUser struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func compileJLS(config domain.JLSConfig) *jlsConfig {
	compiled := &jlsConfig{Enable: true, ServerName: config.ServerName, Destination: config.Destination, ALPN: append([]string(nil), config.ALPN...), Proxy: config.Proxy, RateLimit: config.RateLimit}
	for _, user := range config.Users {
		compiled.Users = append(compiled.Users, jlsUser{Username: user.Username, Password: user.Password})
	}
	return compiled
}

type muxConfig struct {
	Padding bool          `yaml:"padding,omitempty"`
	Brutal  *brutalConfig `yaml:"brutal,omitempty"`
}

type brutalConfig struct {
	Enabled bool   `yaml:"enabled"`
	Up      string `yaml:"up,omitempty"`
	Down    string `yaml:"down,omitempty"`
}

func compileMux(config domain.MuxSpec) *muxConfig {
	if !config.Padding && !config.Brutal.Enabled {
		return nil
	}
	compiled := &muxConfig{Padding: config.Padding}
	if config.Brutal.Enabled {
		compiled.Brutal = &brutalConfig{Enabled: true, Up: config.Brutal.Up, Down: config.Brutal.Down}
	}
	return compiled
}

type xhttpConfig struct {
	Path                 string `yaml:"path,omitempty"`
	Host                 string `yaml:"host,omitempty"`
	Mode                 string `yaml:"mode,omitempty"`
	XPaddingBytes        string `yaml:"x-padding-bytes,omitempty"`
	XPaddingObfsMode     bool   `yaml:"x-padding-obfs-mode,omitempty"`
	XPaddingKey          string `yaml:"x-padding-key,omitempty"`
	XPaddingHeader       string `yaml:"x-padding-header,omitempty"`
	XPaddingPlacement    string `yaml:"x-padding-placement,omitempty"`
	XPaddingMethod       string `yaml:"x-padding-method,omitempty"`
	UplinkHTTPMethod     string `yaml:"uplink-http-method,omitempty"`
	SessionPlacement     string `yaml:"session-placement,omitempty"`
	SessionKey           string `yaml:"session-key,omitempty"`
	SeqPlacement         string `yaml:"seq-placement,omitempty"`
	SeqKey               string `yaml:"seq-key,omitempty"`
	UplinkDataPlacement  string `yaml:"uplink-data-placement,omitempty"`
	UplinkDataKey        string `yaml:"uplink-data-key,omitempty"`
	UplinkChunkSize      string `yaml:"uplink-chunk-size,omitempty"`
	NoSSEHeader          bool   `yaml:"no-sse-header,omitempty"`
	SCStreamUpServerSecs string `yaml:"sc-stream-up-server-secs,omitempty"`
	SCMaxBufferedPosts   string `yaml:"sc-max-buffered-posts,omitempty"`
	SCMaxEachPostBytes   string `yaml:"sc-max-each-post-bytes,omitempty"`
}

func compileXHTTP(config domain.XHTTPConfig) *xhttpConfig {
	return &xhttpConfig{
		Path: config.Path, Host: config.Host, Mode: config.Mode,
		XPaddingBytes: config.XPaddingBytes, XPaddingObfsMode: config.XPaddingObfsMode,
		XPaddingKey: config.XPaddingKey, XPaddingHeader: config.XPaddingHeader,
		XPaddingPlacement: config.XPaddingPlacement, XPaddingMethod: config.XPaddingMethod,
		UplinkHTTPMethod: config.UplinkHTTPMethod, SessionPlacement: config.SessionPlacement,
		SessionKey: config.SessionKey, SeqPlacement: config.SeqPlacement, SeqKey: config.SeqKey,
		UplinkDataPlacement: config.UplinkDataPlacement, UplinkDataKey: config.UplinkDataKey,
		UplinkChunkSize: config.UplinkChunkSize, NoSSEHeader: config.NoSSEHeader,
		SCStreamUpServerSecs: config.SCStreamUpServerSecs, SCMaxBufferedPosts: config.SCMaxBufferedPosts,
		SCMaxEachPostBytes: config.SCMaxEachPostBytes,
	}
}

type vlessClientDocument struct {
	Proxies []vlessClientProxy `yaml:"proxies"`
}
type vlessClientProxy struct {
	Name              string              `yaml:"name"`
	Type              string              `yaml:"type"`
	Server            string              `yaml:"server"`
	Port              uint16              `yaml:"port"`
	UDP               bool                `yaml:"udp"`
	UUID              string              `yaml:"uuid"`
	Flow              string              `yaml:"flow,omitempty"`
	PacketEncoding    string              `yaml:"packet-encoding,omitempty"`
	TLS               bool                `yaml:"tls,omitempty"`
	ServerName        string              `yaml:"servername,omitempty"`
	ClientFingerprint string              `yaml:"client-fingerprint,omitempty"`
	SkipCertVerify    bool                `yaml:"skip-cert-verify,omitempty"`
	Reality           *vlessClientReality `yaml:"reality-opts,omitempty"`
	ShadowTLS         *vlessClientSecret  `yaml:"shadow-tls-opts,omitempty"`
	ResTLS            *vlessClientResTLS  `yaml:"restls-opts,omitempty"`
	JLS               *vlessClientJLS     `yaml:"jls-opts,omitempty"`
	Encryption        string              `yaml:"encryption"`
	Network           string              `yaml:"network,omitempty"`
	WS                *websocketClient    `yaml:"ws-opts,omitempty"`
	GRPC              *grpcClient         `yaml:"grpc-opts,omitempty"`
	XHTTP             *xhttpConfig        `yaml:"xhttp-opts,omitempty"`
}
type vlessClientReality struct {
	PublicKey string `yaml:"public-key"`
	ShortID   string `yaml:"short-id"`
}
type vlessClientSecret struct {
	Version  int    `yaml:"version,omitempty"`
	Password string `yaml:"password"`
}
type vlessClientResTLS struct {
	Password    string `yaml:"password"`
	VersionHint string `yaml:"version-hint"`
	Script      string `yaml:"restls-script,omitempty"`
}
type vlessClientJLS struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
type websocketClient struct {
	Path string `yaml:"path"`
}
type grpcClient struct {
	ServiceName string `yaml:"grpc-service-name"`
}

func compileVLESSClient(node domain.Node, user domain.NodeUser, profile domain.AccessProfile, host string) ([]byte, error) {
	proxy := vlessClientProxy{
		Name: node.Name + " - " + user.Name, Type: "vless", Server: host, Port: profile.PublicPort,
		UDP: true, UUID: user.VLESS.UUID, Flow: user.VLESS.Flow,
		PacketEncoding: profile.PacketEncoding, ServerName: profile.ServerName,
		ClientFingerprint: profile.Fingerprint, SkipCertVerify: profile.AllowInsecure,
		Encryption: normalizedDecryption(node.VLESS.Decryption),
	}
	switch node.VLESS.Handler.Type {
	case domain.VLESSHandlerRaw:
		proxy.Network = "tcp"
	case domain.VLESSHandlerWebSocket:
		proxy.Network = "ws"
		proxy.WS = &websocketClient{Path: node.VLESS.Handler.WebSocket.Path}
	case domain.VLESSHandlerGRPC:
		proxy.Network = "grpc"
		proxy.GRPC = &grpcClient{ServiceName: node.VLESS.Handler.GRPC.ServiceName}
	case domain.VLESSHandlerXHTTP:
		proxy.Network = "xhttp"
		proxy.XHTTP = compileXHTTP(*node.VLESS.Handler.XHTTP)
	}
	switch node.VLESS.Security.Type {
	case domain.VLESSSecurityTLS:
		proxy.TLS = true
		proxy.SkipCertVerify = proxy.SkipCertVerify || node.VLESS.Security.TLS.AllowInsecure
	case domain.VLESSSecurityReality:
		proxy.TLS = true
		reality := node.VLESS.Security.Reality
		shortID := ""
		if len(reality.ShortIDs) > 0 {
			shortID = reality.ShortIDs[0]
		}
		proxy.Reality = &vlessClientReality{PublicKey: reality.PublicKey, ShortID: shortID}
	case domain.VLESSSecurityShadowTLS:
		proxy.TLS = true
		shadowTLS := node.VLESS.Security.ShadowTLS
		password := shadowTLS.Password
		if shadowTLS.Version == 3 && len(shadowTLS.Users) > 0 {
			password = shadowTLS.Users[0].Password
			for _, candidate := range shadowTLS.Users {
				if candidate.Name == user.Name {
					password = candidate.Password
					break
				}
			}
		}
		proxy.ShadowTLS = &vlessClientSecret{Version: shadowTLS.Version, Password: password}
	case domain.VLESSSecurityResTLS:
		proxy.TLS = true
		versionHint := node.VLESS.Security.ResTLS.VersionHint
		if versionHint == "" {
			versionHint = "tls13"
		}
		proxy.ResTLS = &vlessClientResTLS{
			Password: node.VLESS.Security.ResTLS.Password, VersionHint: versionHint,
			Script: node.VLESS.Security.ResTLS.Script,
		}
	case domain.VLESSSecurityJLS:
		proxy.TLS = true
		if len(node.VLESS.Security.JLS.Users) > 0 {
			jlsUser := node.VLESS.Security.JLS.Users[0]
			for _, candidate := range node.VLESS.Security.JLS.Users {
				if candidate.Username == user.Name {
					jlsUser = candidate
					break
				}
			}
			proxy.JLS = &vlessClientJLS{Username: jlsUser.Username, Password: jlsUser.Password}
		}
	}
	return encodeClientYAML(vlessClientDocument{Proxies: []vlessClientProxy{proxy}})
}

func normalizedDecryption(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func encodeClientYAML(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
