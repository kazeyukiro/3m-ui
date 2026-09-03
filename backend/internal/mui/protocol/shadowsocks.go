package protocol

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/mui/domain"
)

type ShadowsocksModule struct{}

func (ShadowsocksModule) Kind() domain.ProtocolKind { return domain.ProtocolShadowsocks }
func (ShadowsocksModule) Capability() ProtocolCapability {
	return ProtocolCapability{Kind: domain.ProtocolShadowsocks}
}

func (ShadowsocksModule) Compile(ctx context.Context, node domain.Node, asOf time.Time) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if node.Shadowsocks == nil {
		return nil, errors.New("shadowsocks specification is missing")
	}
	users := effectiveUsers(node, asOf)
	if len(users) != 1 || users[0].Shadowsocks == nil {
		return nil, errors.New("shadowsocks listener requires exactly one effective credential")
	}
	spec := node.Shadowsocks
	listener := shadowsocksListener{
		Name: node.Name, Type: "shadowsocks", Listen: node.ListenAddress, Port: node.Port,
		Password: users[0].Shadowsocks.Password, Cipher: spec.Cipher, UDP: spec.UDP,
		Mux: compileMux(spec.Mux),
	}
	if err := listener.Security.apply(spec.Security, false); err != nil {
		return nil, err
	}
	if spec.SimpleObfs.Enabled {
		listener.SimpleObfs = &simpleObfsListener{Enable: true, Mode: spec.SimpleObfs.Mode}
	}
	return listener, nil
}

func (ShadowsocksModule) BuildShare(state domain.DesiredState, node domain.Node, user domain.NodeUser, profile domain.AccessProfile) (Share, error) {
	if node.Shadowsocks == nil || user.Shadowsocks == nil {
		return Share{}, errors.New("shadowsocks share requires Shadowsocks node and user credentials")
	}
	host := shareHost(state, profile)
	query := url.Values{}
	plugin, pluginOptions := shadowsocksPlugin(node.Shadowsocks, user, profile)
	if plugin != "" {
		sharePlugin := plugin
		if plugin == "obfs" {
			sharePlugin = "obfs-local"
		}
		parts := []string{sharePlugin}
		for _, key := range sortedKeys(pluginOptions) {
			parts = append(parts, key+"="+fmt.Sprint(pluginOptions[key]))
		}
		query.Set("plugin", strings.Join(parts, ";"))
	}
	userinfo := base64.RawURLEncoding.EncodeToString([]byte(node.Shadowsocks.Cipher + ":" + user.Shadowsocks.Password))
	port := int(profile.PublicPort)
	if port <= 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(node.Port)); err == nil {
			port = n
		}
	}
	// SIP002: ss://BASE64URL(method:password)@host:port/?plugin=...#name
	uri := (&url.URL{
		Scheme: "ss", Host: net.JoinHostPort(host, strconv.Itoa(port)),
		RawQuery: query.Encode(), Fragment: node.Name + " - " + user.Name,
	}).String()
	uri = "ss://" + userinfo + "@" + strings.TrimPrefix(uri, "ss://")
	client := shadowsocksClientDocument{Proxies: []shadowsocksClientProxy{{
		Name: node.Name + " - " + user.Name, Type: "ss", Server: host, Port: uint16(port),
		Cipher: node.Shadowsocks.Cipher, Password: user.Shadowsocks.Password, UDP: node.Shadowsocks.UDP,
		Plugin: plugin, PluginOptions: pluginOptions, ClientFingerprint: profile.Fingerprint,
		UDPOverTCP:        node.Shadowsocks.UDPOverTCP,
		UDPOverTCPVersion: node.Shadowsocks.UDPOverTCPVersion,
		IPVersion:         node.Shadowsocks.IPVersion,
		SMux:              compileSMux(node.Shadowsocks.SMux),
	}}}
	clientYAML, err := encodeClientYAML(client)
	if err != nil {
		return Share{}, err
	}
	return Share{URI: uri, QRContent: uri, ClientYAML: clientYAML}, nil
}

type shadowsocksListener struct {
	Name       string                  `yaml:"name"`
	Type       string                  `yaml:"type"`
	Listen     string                  `yaml:"listen"`
	Port       string                  `yaml:"port"`
	Password   string                  `yaml:"password"`
	Cipher     string                  `yaml:"cipher"`
	UDP        bool                    `yaml:"udp"`
	Security   classicSecurityListener `yaml:",inline"`
	Mux        *muxConfig              `yaml:"mux-option,omitempty"`
	SimpleObfs *simpleObfsListener     `yaml:"simple-obfs,omitempty"`
}
type simpleObfsListener struct {
	Enable bool   `yaml:"enable"`
	Mode   string `yaml:"mode"`
}

type shadowsocksClientDocument struct {
	Proxies []shadowsocksClientProxy `yaml:"proxies"`
}
type shadowsocksClientProxy struct {
	Name              string         `yaml:"name"`
	Type              string         `yaml:"type"`
	Server            string         `yaml:"server"`
	Port              uint16         `yaml:"port"`
	Cipher            string         `yaml:"cipher"`
	Password          string         `yaml:"password"`
	UDP               bool           `yaml:"udp"`
	Plugin            string         `yaml:"plugin,omitempty"`
	PluginOptions     map[string]any `yaml:"plugin-opts,omitempty"`
	ClientFingerprint string         `yaml:"client-fingerprint,omitempty"`
	UDPOverTCP        bool           `yaml:"udp-over-tcp,omitempty"`
	UDPOverTCPVersion string         `yaml:"udp-over-tcp-version,omitempty"`
	IPVersion         string         `yaml:"ip-version,omitempty"`
	SMux              *smuxClient    `yaml:"smux,omitempty"`
}

func shadowsocksPlugin(spec *domain.ShadowsocksSpec, user domain.NodeUser, profile domain.AccessProfile) (string, map[string]any) {
	if spec.SimpleObfs.Enabled {
		return "obfs", map[string]any{"mode": spec.SimpleObfs.Mode}
	}
	// kcptun is a UDP-over-TCP wrapper that rides on the SS `plugin` format per
	// proxies-ss wiki block 6. It is mutually exclusive with the TLS-like
	// wrappers below (shadow-tls / res-tls / jls) since the listener schema
	// only allows one `plugin` value per SS outbound.
	if spec.Kcptun != nil && spec.Kcptun.Enable {
		return "kcptun", kcptunPluginOpts(spec.Kcptun)
	}
	switch spec.Security.Type {
	case domain.VLESSSecurityShadowTLS:
		config := spec.Security.ShadowTLS
		password := config.Password
		if config.Version == 3 && len(config.Users) > 0 {
			password = config.Users[0].Password
			for _, candidate := range config.Users {
				if candidate.Name == user.Name {
					password = candidate.Password
					break
				}
			}
		}
		return "shadow-tls", map[string]any{"version": config.Version, "password": password, "host": pluginHost(profile, config.Handshake.Destination)}
	case domain.VLESSSecurityResTLS:
		config := spec.Security.ResTLS
		return "restls", map[string]any{
			"host": pluginHost(profile, config.Destination), "password": config.Password,
			"version-hint":  defaultStringValue(config.VersionHint, "tls13"),
			"restls-script": config.Script, "skip-cert-verify": profile.AllowInsecure,
		}
	case domain.VLESSSecurityJLS:
		config := spec.Security.JLS
		if len(config.Users) == 0 {
			return "", nil
		}
		credential := config.Users[0]
		for _, candidate := range config.Users {
			if candidate.Username == user.Name {
				credential = candidate
				break
			}
		}
		return "jls", map[string]any{
			"host": pluginHost(profile, config.Destination), "username": credential.Username,
			"password": credential.Password, "alpn": config.ALPN,
		}
	default:
		return "", nil
	}
}

func pluginHost(profile domain.AccessProfile, destination string) string {
	if profile.ServerName != "" {
		return profile.ServerName
	}
	if host, _, err := net.SplitHostPort(destination); err == nil {
		return host
	}
	return destination
}

// kcptunPluginOpts maps the listener `kcp-tun` block (via domain.KCPTunConfig)
// into the SS `plugin-opts` shape documented on proxies-ss wiki block 6. Only
// non-zero / non-default values are emitted so the client YAML stays clean
// (matching the wiki example which comments out most optional fields).
func kcptunPluginOpts(c *domain.KCPTunConfig) map[string]any {
	opts := map[string]any{}
	if c.Key != "" {
		opts["key"] = c.Key
	}
	if c.Crypt != "" {
		opts["crypt"] = c.Crypt
	}
	if c.Mode != "" {
		opts["mode"] = c.Mode
	}
	if c.Conn != 0 {
		opts["conn"] = c.Conn
	}
	if c.AutoExpire != 0 {
		opts["autoexpire"] = c.AutoExpire
	}
	if c.ScavengeTTL != 0 {
		opts["scavengettl"] = c.ScavengeTTL
	}
	if c.MTU != 0 {
		opts["mtu"] = c.MTU
	}
	if c.RateLimit != 0 {
		opts["ratelimit"] = c.RateLimit
	}
	if c.SndWnd != 0 {
		opts["sndwnd"] = c.SndWnd
	}
	if c.RcvWnd != 0 {
		opts["rcvwnd"] = c.RcvWnd
	}
	if c.DataShard != 0 {
		opts["datashard"] = c.DataShard
	}
	if c.ParityShard != 0 {
		opts["parityshard"] = c.ParityShard
	}
	if c.DSCP != 0 {
		opts["dscp"] = c.DSCP
	}
	if c.NoComp {
		opts["nocomp"] = c.NoComp
	}
	if c.AckNoDelay {
		opts["acknodelay"] = c.AckNoDelay
	}
	if c.NoDelay != 0 {
		opts["nodelay"] = c.NoDelay
	}
	if c.Interval != 0 {
		opts["interval"] = c.Interval
	}
	if c.Resend != 0 {
		opts["resend"] = c.Resend
	}
	if c.SockBuf != 0 {
		opts["sockbuf"] = c.SockBuf
	}
	if c.SmuxVer != 0 {
		opts["smuxver"] = c.SmuxVer
	}
	if c.SmuxBuf != 0 {
		opts["smuxbuf"] = c.SmuxBuf
	}
	if c.FrameSize != 0 {
		opts["framesize"] = c.FrameSize
	}
	if c.StreamBuf != 0 {
		opts["streambuf"] = c.StreamBuf
	}
	if c.KeepAlive != 0 {
		opts["keepalive"] = c.KeepAlive
	}
	return opts
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if value == nil || value == "" || value == false {
			continue
		}
		keys = append(keys, key)
	}
	slicesSort(keys)
	return keys
}

func slicesSort(values []string) {
	for index := 1; index < len(values); index++ {
		for current := index; current > 0 && values[current] < values[current-1]; current-- {
			values[current], values[current-1] = values[current-1], values[current]
		}
	}
}
