package protocol

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/mui/domain"
)

type Hysteria2Module struct{}

func (Hysteria2Module) Kind() domain.ProtocolKind { return domain.ProtocolHysteria2 }

func (Hysteria2Module) Capability() ProtocolCapability {
	return ProtocolCapability{Kind: domain.ProtocolHysteria2}
}

func (Hysteria2Module) Compile(
	ctx context.Context,
	node domain.Node,
	asOf time.Time,
) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if node.Hysteria2 == nil {
		return nil, errors.New("hysteria2 specification is missing")
	}
	spec := node.Hysteria2
	listener := hysteria2Listener{
		Name: node.Name, Type: "hysteria2", Listen: node.ListenAddress, Port: node.Port,
		Users: make(map[string]string), Obfs: spec.Obfs, ObfsPassword: spec.ObfsPassword,
		ObfsMinPacketSize: spec.ObfsMinPacketSize, ObfsMaxPacketSize: spec.ObfsMaxPacketSize,
		Certificate: spec.Certificate, PrivateKey: spec.PrivateKey,
		ClientAuthType: spec.ClientAuthType, ClientAuthCert: spec.ClientAuthCert, ECHKey: spec.ECHKey,
		MaxIdleTime: spec.MaxIdleTime, ALPN: append([]string(nil), spec.ALPN...), Up: spec.Up, Down: spec.Down,
		IgnoreClientBandwidth: spec.IgnoreClientBandwidth, Masquerade: spec.Masquerade,
		CWND: spec.CWND, BBRProfile: spec.BBRProfile, UDPMTU: spec.UDPMTU, Mux: compileMux(spec.Mux),
		InitialStreamReceiveWindow:     spec.InitialStreamReceiveWindow,
		MaxStreamReceiveWindow:         spec.MaxStreamReceiveWindow,
		InitialConnectionReceiveWindow: spec.InitialConnectionReceiveWindow,
		MaxConnectionReceiveWindow:     spec.MaxConnectionReceiveWindow,
	}
	if spec.Realm != nil {
		listener.Realm = compileHysteria2Realm(*spec.Realm)
	}
	for _, user := range effectiveUsers(node, asOf) {
		if user.Hysteria2 == nil {
			return nil, fmt.Errorf("user %q is missing Hysteria2 credentials", user.Name)
		}
		listener.Users[user.Name] = user.Hysteria2.Password
	}
	return listener, nil
}

func (Hysteria2Module) BuildShare(
	state domain.DesiredState,
	node domain.Node,
	user domain.NodeUser,
	profile domain.AccessProfile,
) (Share, error) {
	if node.Hysteria2 == nil || user.Hysteria2 == nil {
		return Share{}, errors.New("hysteria2 share requires Hysteria2 node and user credentials")
	}
	host := profile.PublicHost
	if host == "" {
		host = state.PublicHost
	}
	query := url.Values{}
	if profile.ServerName != "" {
		query.Set("sni", profile.ServerName)
	}
	if profile.AllowInsecure {
		query.Set("insecure", "1")
	}
	if node.Hysteria2.Obfs != "" {
		query.Set("obfs", node.Hysteria2.Obfs)
		query.Set("obfs-password", node.Hysteria2.ObfsPassword)
	}
	port := int(profile.PublicPort)
	if port <= 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(node.Port)); err == nil {
			port = n
		}
	}
	// Official scheme: hysteria2://auth@host:port/?sni=&insecure=&obfs=&obfs-password=
	// (hy2:// is accepted by many clients as an alias; we emit hysteria2://)
	uri := (&url.URL{
		Scheme: "hysteria2", User: url.User(user.Hysteria2.Password),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		RawQuery: query.Encode(), Fragment: node.Name + " - " + user.Name,
	}).String()
	client := hysteria2ClientDocument{Proxies: []hysteria2ClientProxy{{
		Name: node.Name + " - " + user.Name, Type: "hysteria2", Server: host,
		Port: uint16(port), Password: user.Hysteria2.Password,
		Up: node.Hysteria2.Up, Down: node.Hysteria2.Down, BBRProfile: node.Hysteria2.BBRProfile,
		Obfs: node.Hysteria2.Obfs, ObfsPassword: node.Hysteria2.ObfsPassword,
		ObfsMinPacketSize: node.Hysteria2.ObfsMinPacketSize, ObfsMaxPacketSize: node.Hysteria2.ObfsMaxPacketSize,
		ServerName: profile.ServerName, SkipCertVerify: profile.AllowInsecure,
		ALPN: append([]string(nil), node.Hysteria2.ALPN...), Realm: compileHysteria2Realm(hysteria2RealmOrZero(node.Hysteria2)),
	}}}
	clientYAML, err := encodeClientYAML(client)
	if err != nil {
		return Share{}, err
	}
	return Share{URI: uri, QRContent: uri, ClientYAML: clientYAML}, nil
}

type hysteria2Listener struct {
	Name                           string                `yaml:"name"`
	Type                           string                `yaml:"type"`
	Listen                         string                `yaml:"listen"`
	Port                           string                `yaml:"port"`
	Users                          map[string]string     `yaml:"users,omitempty"`
	Obfs                           string                `yaml:"obfs,omitempty"`
	ObfsPassword                   string                `yaml:"obfs-password,omitempty"`
	ObfsMinPacketSize              int                   `yaml:"obfs-min-packet-size,omitempty"`
	ObfsMaxPacketSize              int                   `yaml:"obfs-max-packet-size,omitempty"`
	Certificate                    string                `yaml:"certificate"`
	PrivateKey                     string                `yaml:"private-key"`
	ClientAuthType                 string                `yaml:"client-auth-type,omitempty"`
	ClientAuthCert                 string                `yaml:"client-auth-cert,omitempty"`
	ECHKey                         string                `yaml:"ech-key,omitempty"`
	MaxIdleTime                    int                   `yaml:"max-idle-time,omitempty"`
	ALPN                           []string              `yaml:"alpn,omitempty"`
	Up                             string                `yaml:"up,omitempty"`
	Down                           string                `yaml:"down,omitempty"`
	IgnoreClientBandwidth          bool                  `yaml:"ignore-client-bandwidth,omitempty"`
	Masquerade                     string                `yaml:"masquerade,omitempty"`
	CWND                           int                   `yaml:"cwnd,omitempty"`
	BBRProfile                     string                `yaml:"bbr-profile,omitempty"`
	UDPMTU                         int                   `yaml:"udp-mtu,omitempty"`
	Mux                            *muxConfig            `yaml:"mux-option,omitempty"`
	Realm                          *hysteria2RealmConfig `yaml:"realm-opts,omitempty"`
	InitialStreamReceiveWindow     uint64                `yaml:"initial-stream-receive-window,omitempty"`
	MaxStreamReceiveWindow         uint64                `yaml:"max-stream-receive-window,omitempty"`
	InitialConnectionReceiveWindow uint64                `yaml:"initial-connection-receive-window,omitempty"`
	MaxConnectionReceiveWindow     uint64                `yaml:"max-connection-receive-window,omitempty"`
}

type hysteria2RealmConfig struct {
	Enable         bool     `yaml:"enable,omitempty"`
	ServerURL      string   `yaml:"server-url,omitempty"`
	Token          string   `yaml:"token,omitempty"`
	RealmID        string   `yaml:"realm-id,omitempty"`
	STUNServers    []string `yaml:"stun-servers,omitempty"`
	ServerName     string   `yaml:"sni,omitempty"`
	SkipCertVerify bool     `yaml:"skip-cert-verify,omitempty"`
	NameCertVerify string   `yaml:"name-cert-verify,omitempty"`
	Fingerprint    string   `yaml:"fingerprint,omitempty"`
	Certificate    string   `yaml:"certificate,omitempty"`
	PrivateKey     string   `yaml:"private-key,omitempty"`
	ALPN           []string `yaml:"alpn,omitempty"`
	Proxy          string   `yaml:"proxy,omitempty"`
}

func compileHysteria2Realm(config domain.Hysteria2RealmConfig) *hysteria2RealmConfig {
	if !config.Enabled && config.ServerURL == "" {
		return nil
	}
	return &hysteria2RealmConfig{
		Enable: config.Enabled, ServerURL: config.ServerURL, Token: config.Token, RealmID: config.RealmID,
		STUNServers: append([]string(nil), config.STUNServers...), ServerName: config.ServerName,
		SkipCertVerify: config.SkipCertVerify, NameCertVerify: config.NameCertVerify,
		Fingerprint: config.Fingerprint, Certificate: config.Certificate, PrivateKey: config.PrivateKey,
		ALPN: append([]string(nil), config.ALPN...), Proxy: config.Proxy,
	}
}

type hysteria2ClientDocument struct {
	Proxies []hysteria2ClientProxy `yaml:"proxies"`
}
type hysteria2ClientProxy struct {
	Name              string                `yaml:"name"`
	Type              string                `yaml:"type"`
	Server            string                `yaml:"server"`
	Port              uint16                `yaml:"port"`
	Password          string                `yaml:"password"`
	Up                string                `yaml:"up,omitempty"`
	Down              string                `yaml:"down,omitempty"`
	BBRProfile        string                `yaml:"bbr-profile,omitempty"`
	Obfs              string                `yaml:"obfs,omitempty"`
	ObfsPassword      string                `yaml:"obfs-password,omitempty"`
	ObfsMinPacketSize int                   `yaml:"obfs-min-packet-size,omitempty"`
	ObfsMaxPacketSize int                   `yaml:"obfs-max-packet-size,omitempty"`
	ServerName        string                `yaml:"sni,omitempty"`
	SkipCertVerify    bool                  `yaml:"skip-cert-verify,omitempty"`
	ALPN              []string              `yaml:"alpn,omitempty"`
	Realm             *hysteria2RealmConfig `yaml:"realm-opts,omitempty"`
}

func hysteria2RealmOrZero(spec *domain.Hysteria2Spec) domain.Hysteria2RealmConfig {
	if spec.Realm == nil {
		return domain.Hysteria2RealmConfig{}
	}
	return *spec.Realm
}
