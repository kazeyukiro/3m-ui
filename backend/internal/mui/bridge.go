package mui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mui/domain"
	muiprotocol "github.com/kazeyukiro/3m-ui/backend/internal/mui/protocol"
	"golang.org/x/crypto/curve25519"
	"gopkg.in/yaml.v3"
)

// Cred is a panel-side credential used when adapting Listeners to m-ui nodes.
type Cred struct {
	Username string
	Password string
	UUID     string
	Flow     string
}

// ListenerToNode adapts a 3m-ui Listener + credentials into an m-ui domain.Node.
func ListenerToNode(l models.Listener, creds []Cred) (domain.Node, error) {
	cfg := map[string]interface{}{}
	if strings.TrimSpace(l.Config) != "" {
		if err := json.Unmarshal([]byte(l.Config), &cfg); err != nil {
			return domain.Node{}, fmt.Errorf("listener config: %w", err)
		}
	}
	proto := domain.ProtocolKind(strings.ToLower(strings.TrimSpace(l.Protocol)))
	nodeID := fmt.Sprintf("%d", l.ID)
	node := domain.Node{
		ID:            nodeID,
		Name:          l.Name,
		Enabled:       l.Enabled,
		ListenAddress: firstNonEmpty(l.Listen, l.BindAddress, "0.0.0.0"),
		// Listen port must come from the listener bind port, not PublicPort (NAT map).
		Port:          firstNonEmpty(strings.TrimSpace(l.Port), "0"),
		Protocol:      proto,
		SchemaVersion: domain.NodeSchemaVersion,
	}

	// Access profile
	pubPort := uint16(0)
	if p := firstNonEmpty(l.PublicPort, l.Port); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
			pubPort = uint16(n)
		}
	}
	profile := domain.AccessProfile{
		ID:            nodeID + "-default",
		NodeID:        nodeID,
		Name:          "default",
		Default:       true,
		PublicHost:    strings.TrimSpace(l.PublicHost),
		PublicPort:    pubPort,
		ServerName:    strings.TrimSpace(l.AccessSNI),
		Fingerprint:   strings.TrimSpace(l.ClientFingerprint),
		AllowInsecure: boolCfg(cfg, "skip-cert-verify") || certutil.IsPanelSelfSignedPEM(strCfg(cfg, "certificate")),
	}
	if profile.ServerName == "" {
		profile.ServerName = strCfg(cfg, "sni", "servername")
	}
	if profile.Fingerprint == "" {
		profile.Fingerprint = strCfg(cfg, "client-fingerprint", "fingerprint")
	}
	if profile.Fingerprint == "" {
		profile.Fingerprint = domain.ClientFingerprint
	}
	node.AccessProfiles = []domain.AccessProfile{profile}

	// Users from panel credentials
	for i, c := range creds {
		u := domain.NodeUser{
			ID:      fmt.Sprintf("%s-u%d", nodeID, i+1),
			NodeID:  nodeID,
			Name:    firstNonEmpty(c.Username, fmt.Sprintf("user-%d", i+1)),
			Enabled: true,
		}
		switch proto {
		case domain.ProtocolVLESS:
			u.VLESS = &domain.VLESSCredential{UUID: c.UUID, Flow: c.Flow}
		case domain.ProtocolVMess:
			u.VMess = &domain.VMessCredential{UUID: c.UUID, Cipher: "auto"}
		case domain.ProtocolTrojan:
			u.Trojan = &domain.TrojanCredential{Password: c.Password}
		case domain.ProtocolHysteria2:
			u.Hysteria2 = &domain.Hysteria2Credential{Password: c.Password}
		case domain.ProtocolShadowsocks:
			u.Shadowsocks = &domain.ShadowsocksCredential{Password: c.Password}
		}
		node.Users = append(node.Users, u)
	}

	// Fall back to users embedded in listener config JSON when no panel credentials.
	if len(node.Users) == 0 {
		if rawUsers, ok := cfg["users"].([]interface{}); ok {
			for i, item := range rawUsers {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				u := domain.NodeUser{
					ID:      fmt.Sprintf("%s-cfg%d", nodeID, i+1),
					NodeID:  nodeID,
					Name:    firstNonEmpty(strMap(m, "username"), strMap(m, "name"), fmt.Sprintf("user-%d", i+1)),
					Enabled: true,
				}
				switch proto {
				case domain.ProtocolVLESS:
					u.VLESS = &domain.VLESSCredential{UUID: strMap(m, "uuid"), Flow: strMap(m, "flow")}
				case domain.ProtocolVMess:
					u.VMess = &domain.VMessCredential{UUID: strMap(m, "uuid"), Cipher: "auto"}
				case domain.ProtocolTrojan:
					u.Trojan = &domain.TrojanCredential{Password: strMap(m, "password")}
				case domain.ProtocolHysteria2:
					u.Hysteria2 = &domain.Hysteria2Credential{Password: strMap(m, "password")}
				case domain.ProtocolShadowsocks:
					u.Shadowsocks = &domain.ShadowsocksCredential{Password: strMap(m, "password")}
				}
				node.Users = append(node.Users, u)
			}
		}
	}

	if proto == domain.ProtocolShadowsocks && len(node.Users) == 0 {
		if pass := strCfg(cfg, "password"); pass != "" {
			node.Users = append(node.Users, domain.NodeUser{
				ID: nodeID + "-ss", NodeID: nodeID, Name: "default", Enabled: true,
				Shadowsocks: &domain.ShadowsocksCredential{Password: pass},
			})
		}
	}

	// Protocol specs from flat Mihomo listener JSON.
	//
	// Follow-up (P3-8 / X-MUI-1): only 5 of the 12 supported Mihomo listener
	// protocols are decoded here (vless, vmess, trojan, shadowsocks, hysteria2).
	// The remaining 7 (tuic, shadowquic, anytls, mieru, sudoku, trusttunnel,
	// snell) fall back to the 3m-ui native / legacy share builders via the
	// three-tier fallback in internal/node/api.go. Adding decoders for those
	// protocols here is tracked as a separate feature task.
	switch proto {
	case domain.ProtocolVLESS:
		node.VLESS = decodeVLESS(cfg, l.TLS)
	case domain.ProtocolVMess:
		node.VMess = decodeVMess(cfg, l.TLS)
	case domain.ProtocolTrojan:
		node.Trojan = decodeTrojan(cfg, l.TLS)
	case domain.ProtocolShadowsocks:
		node.Shadowsocks = decodeSS(cfg)
	case domain.ProtocolHysteria2:
		node.Hysteria2 = decodeHy2(cfg)
	default:
		return domain.Node{}, fmt.Errorf("protocol %q is not supported by m-ui port", proto)
	}
	return node, nil
}

// BuildShare adapts a listener and builds an m-ui share for the first/default user,
// or for all users when building URI lists.
func BuildShares(l models.Listener, publicHost string, creds []Cred) ([]muiprotocol.Share, error) {
	node, err := ListenerToNode(l, creds)
	if err != nil {
		return nil, err
	}
	if publicHost != "" {
		for i := range node.AccessProfiles {
			if node.AccessProfiles[i].Default {
				node.AccessProfiles[i].PublicHost = publicHost
			}
		}
	}
	profile, ok := node.DefaultAccessProfile()
	if !ok {
		return nil, fmt.Errorf("access profile missing")
	}
	if profile.PublicHost == "" {
		profile.PublicHost = publicHost
	}
	enrichAccessProfileFromNode(&profile, node, l)
	flowDefault := listenerFlowHint(l)
	state := domain.DesiredState{AsOf: time.Now().UTC(), PublicHost: profile.PublicHost}
	reg := muiprotocol.DefaultRegistry()
	out := make([]muiprotocol.Share, 0, len(node.Users))
	for _, user := range node.Users {
		// Ensure user/profile node id match for registry checks.
		user.NodeID = node.ID
		profile.NodeID = node.ID
		if user.VLESS != nil && strings.TrimSpace(user.VLESS.Flow) == "" && flowDefault != "" {
			user.VLESS.Flow = flowDefault
		}
		s, err := reg.BuildShare(state, node, user, profile)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// CompileListener compiles a listener via m-ui protocol modules into a YAML-ready map.
func CompileListener(l models.Listener, creds []Cred) (map[string]interface{}, error) {
	node, err := ListenerToNode(l, creds)
	if err != nil {
		return nil, err
	}
	compiled, err := muiprotocol.DefaultRegistry().Compile(context.Background(), node, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	// m-ui listener structs use yaml tags; JSON would emit PascalCase field names.
	raw, err := yaml.Marshal(compiled)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func decodeVLESS(cfg map[string]interface{}, tls bool) *domain.VLESSSpec {
	spec := &domain.VLESSSpec{
		Decryption:     strCfg(cfg, "encryption", "decryption"),
		Handler:        decodeHandler(cfg),
		Security:       decodeSecurity(cfg, tls),
		ALPN:           decodeALPN(cfg),
		Fingerprint:    strCfg(cfg, "fingerprint"),
		NameCertVerify: strCfg(cfg, "name-cert-verify"),
		SMux:           decodeSMux(cfg),
		ECHOpts:        decodeECHOpts(cfg),
	}
	if spec.Decryption == "" {
		spec.Decryption = "none"
	}
	return spec
}

func decodeVMess(cfg map[string]interface{}, tls bool) *domain.VMessSpec {
	return &domain.VMessSpec{
		Handler:             decodeHandler(cfg),
		Security:            decodeSecurity(cfg, tls),
		ALPN:                decodeALPN(cfg),
		GlobalPadding:       boolCfg(cfg, "global-padding"),
		AuthenticatedLength: boolCfg(cfg, "authenticated-length"),
		Fingerprint:         strCfg(cfg, "fingerprint"),
		NameCertVerify:      strCfg(cfg, "name-cert-verify"),
		TLSMirrorOpts:       decodeTLSMirrorOpts(cfg),
		SMux:                decodeSMux(cfg),
		ECHOpts:             decodeECHOpts(cfg),
	}
}

func decodeTrojan(cfg map[string]interface{}, tls bool) *domain.TrojanSpec {
	return &domain.TrojanSpec{
		Handler:        decodeHandler(cfg),
		Security:       decodeSecurity(cfg, tls),
		ALPN:           decodeALPN(cfg),
		Fingerprint:    strCfg(cfg, "fingerprint"),
		NameCertVerify: strCfg(cfg, "name-cert-verify"),
		TLSMirrorOpts:  decodeTLSMirrorOpts(cfg),
		SMux:           decodeSMux(cfg),
		ECHOpts:        decodeECHOpts(cfg),
	}
}

func decodeSS(cfg map[string]interface{}) *domain.ShadowsocksSpec {
	return &domain.ShadowsocksSpec{
		Cipher:            strCfg(cfg, "cipher"),
		UDP:               boolCfg(cfg, "udp"),
		Security:          decodeSSSecurity(cfg),
		Kcptun:            decodeKCPTunConfig(cfg),
		UDPOverTCP:        boolCfg(cfg, "udp-over-tcp"),
		UDPOverTCPVersion: strCfg(cfg, "udp-over-tcp-version"),
		IPVersion:         strCfg(cfg, "ip-version"),
		SMux:              decodeSMux(cfg),
	}
}

// decodeKCPTunConfig reads the SS listener `kcp-tun` block (whitelisted by the
// SS schema registry) into a domain KCPTunConfig. Returns nil when the block is
// absent or disabled, mirroring the SS module's `plugin: kcptun` emission gate.
func decodeKCPTunConfig(cfg map[string]interface{}) *domain.KCPTunConfig {
	src, ok := cfg["kcp-tun"].(map[string]interface{})
	if !ok || !boolCfg(src, "enable") {
		return nil
	}
	return &domain.KCPTunConfig{
		Enable:      true,
		Key:         strCfg(src, "key"),
		Crypt:       strCfg(src, "crypt"),
		Mode:        strCfg(src, "mode"),
		Conn:        intCfg(src, "conn"),
		AutoExpire:  intCfg(src, "autoexpire"),
		ScavengeTTL: intCfg(src, "scavengettl"),
		RateLimit:   intCfg(src, "ratelimit"),
		MTU:         intCfg(src, "mtu"),
		SndWnd:      intCfg(src, "sndwnd"),
		RcvWnd:      intCfg(src, "rcvwnd"),
		DataShard:   intCfg(src, "datashard"),
		ParityShard: intCfg(src, "parityshard"),
		DSCP:        intCfg(src, "dscp"),
		NoComp:      boolCfg(src, "nocomp"),
		AckNoDelay:  boolCfg(src, "acknodelay"),
		NoDelay:     intCfg(src, "nodelay"),
		Interval:    intCfg(src, "interval"),
		Resend:      intCfg(src, "resend"),
		SockBuf:     intCfg(src, "sockbuf"),
		SmuxVer:     intCfg(src, "smuxver"),
		SmuxBuf:     intCfg(src, "smuxbuf"),
		FrameSize:   intCfg(src, "framesize"),
		StreamBuf:   intCfg(src, "streambuf"),
		KeepAlive:   intCfg(src, "keepalive"),
	}
}

// decodeSSSecurity decodes the SS listener wrapper blocks (shadow-tls /
// res-tls / jls-config) into a VLESSSecuritySpec so the m-ui SS module's
// shadowsocksPlugin emission code fires. SS listeners have no reality-config
// or certificate path — only the three plugin-style wrappers.
func decodeSSSecurity(cfg map[string]interface{}) domain.VLESSSecuritySpec {
	if stls, ok := cfg["shadow-tls"].(map[string]interface{}); ok && boolCfg(stls, "enable") {
		return domain.VLESSSecuritySpec{
			Type:      domain.VLESSSecurityShadowTLS,
			ShadowTLS: decodeShadowTLSConfig(stls),
		}
	}
	if restls, ok := cfg["res-tls"].(map[string]interface{}); ok && boolCfg(restls, "enable") {
		return domain.VLESSSecuritySpec{
			Type:   domain.VLESSSecurityResTLS,
			ResTLS: decodeResTLSConfig(restls),
		}
	}
	if jls, ok := cfg["jls-config"].(map[string]interface{}); ok && boolCfg(jls, "enable") {
		return domain.VLESSSecuritySpec{
			Type: domain.VLESSSecurityJLS,
			JLS:  decodeJLSConfig(jls),
		}
	}
	return domain.VLESSSecuritySpec{Type: domain.VLESSSecurityNone}
}

// decodeShadowTLSConfig maps the listener shadow-tls block into the domain
// ShadowTLSConfig consumed by the m-ui SS and classic-stream modules.
func decodeShadowTLSConfig(src map[string]interface{}) *domain.ShadowTLSConfig {
	cfg := &domain.ShadowTLSConfig{
		Version:  int(uint32Cfg(src, "version")),
		Password: strCfg(src, "password"),
	}
	if hs, ok := src["handshake"].(map[string]interface{}); ok {
		cfg.Handshake = domain.ShadowTLSHandshake{
			Destination: strCfg(hs, "dest"),
			Proxy:       strMap(hs, "proxy"),
		}
	}
	if users, ok := src["users"].([]interface{}); ok {
		for _, item := range users {
			if u, ok := item.(map[string]interface{}); ok {
				cfg.Users = append(cfg.Users, domain.ShadowTLSUser{
					Name:     strMap(u, "name"),
					Password: strMap(u, "password"),
				})
			}
		}
	}
	return cfg
}

// decodeResTLSConfig maps the listener res-tls block into the domain ResTLSConfig.
func decodeResTLSConfig(src map[string]interface{}) *domain.ResTLSConfig {
	return &domain.ResTLSConfig{
		Destination:     strCfg(src, "dest"),
		Password:        strCfg(src, "password"),
		Script:          strCfg(src, "restls-script"),
		MinRecordLength: int(uint32Cfg(src, "min-record-len")),
		Proxy:           strCfg(src, "proxy"),
	}
}

// decodeJLSConfig maps the listener jls-config block into the domain JLSConfig.
func decodeJLSConfig(src map[string]interface{}) *domain.JLSConfig {
	cfg := &domain.JLSConfig{
		ServerName:  strCfg(src, "sni"),
		Destination: strCfg(src, "dest"),
		Proxy:       strCfg(src, "proxy"),
		ALPN:        decodeALPN(src),
	}
	if users, ok := src["users"].([]interface{}); ok {
		for _, item := range users {
			if u, ok := item.(map[string]interface{}); ok {
				cfg.Users = append(cfg.Users, domain.JLSUser{
					Username: strMap(u, "username"),
					Password: strMap(u, "password"),
				})
			}
		}
	}
	return cfg
}

func decodeHy2(cfg map[string]interface{}) *domain.Hysteria2Spec {
	return &domain.Hysteria2Spec{
		Certificate:       strCfg(cfg, "certificate"),
		PrivateKey:        strCfg(cfg, "private-key"),
		Up:                strCfg(cfg, "up"),
		Down:              strCfg(cfg, "down"),
		Obfs:              strCfg(cfg, "obfs"),
		ObfsPassword:      strCfg(cfg, "obfs-password"),
		ALPN:              decodeALPN(cfg),
		Ports:             strCfg(cfg, "ports"),
		HopInterval:       intCfg(cfg, "hop-interval"),
		ObfsMinPacketSize: intCfg(cfg, "obfs-min-packet-size"),
		ObfsMaxPacketSize: intCfg(cfg, "obfs-max-packet-size"),
		NameCertVerify:    strCfg(cfg, "name-cert-verify"),
		Fingerprint:       strCfg(cfg, "fingerprint"),
		HandshakeTimeout:  intCfg(cfg, "handshake-timeout"),
	}
}

// decodeALPN reads the alpn field from a Mihomo listener config JSON. The field
// is usually a []interface{} (from encoding/json) but some imports carry it as
// []string; a bare string is also tolerated.
func decodeALPN(cfg map[string]interface{}) []string {
	alpn, ok := cfg["alpn"]
	if !ok {
		return nil
	}
	switch a := alpn.(type) {
	case []interface{}:
		out := make([]string, 0, len(a))
		for _, v := range a {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []string:
		if len(a) == 0 {
			return nil
		}
		return a
	case string:
		if strings.TrimSpace(a) == "" {
			return nil
		}
		return []string{a}
	}
	return nil
}

func decodeHandler(cfg map[string]interface{}) domain.VLESSHandlerSpec {
	h := domain.VLESSHandlerSpec{Type: domain.VLESSHandlerRaw}
	if path := strCfg(cfg, "ws-path"); path != "" {
		h.Type = domain.VLESSHandlerWebSocket
		h.WebSocket = &domain.WebSocketSpec{Path: path, Headers: decodeWSHeaders(cfg)}
	}
	if svc := strCfg(cfg, "grpc-service-name"); svc != "" {
		h.Type = domain.VLESSHandlerGRPC
		h.GRPC = decodeGRPCSpec(cfg)
	}
	if xhttp, ok := cfg["xhttp-config"].(map[string]interface{}); ok {
		h.Type = domain.VLESSHandlerXHTTP
		h.XHTTP = &domain.XHTTPConfig{Path: strMap(xhttp, "path"), Host: strMap(xhttp, "host"), Mode: strMap(xhttp, "mode")}
	}
	// mkcp-config → MKCP handler (VMess only per mihomo transport docs).
	if mkcp, ok := cfg["mkcp-config"].(map[string]interface{}); ok && boolCfg(mkcp, "enable") {
		h.Type = domain.VMessHandlerMKCP
		h.MKCP = decodeMKCPConfig(mkcp)
	}
	// mekya-config: the m-ui domain types do not yet model a Mekya handler.
	// The 3m-ui native converter (converter/client.go copyTransport) emits
	// mekya-opts directly for the client YAML path; the m-ui bridge path
	// falls back to raw TCP here. Adding a Mekya handler type is tracked as
	// a separate feature task.
	return h
}

// decodeWSHeaders reads the listener-side `ws-headers` map (a free-form
// map[string]string of HTTP headers carried by vmess/vless/trojan listener
// configs to populate the client-side `ws-opts.headers` block per
// proxies-transport wiki block 3). Returns nil when no header entries are
// present so the m-ui client YAML emitter omits the headers key entirely.
func decodeWSHeaders(cfg map[string]interface{}) map[string]string {
	raw, ok := cfg["ws-headers"].(map[string]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				out[k] = trimmed
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// decodeSMux reads the optional `smux` block from listener config JSON.
// mihomo listener schemas (vmess/vless/trojan) whitelist `smux` as a nested
// map so the m-ui bridge can surface it on the client YAML unchanged.
func decodeSMux(cfg map[string]interface{}) domain.SMuxSpec {
	src, ok := cfg["smux"].(map[string]interface{})
	if !ok || len(src) == 0 {
		return domain.SMuxSpec{}
	}
	sm := domain.SMuxSpec{Enabled: boolCfg(src, "enabled"), Padding: boolCfg(src, "padding")}
	if brutal, ok := src["brutal"].(map[string]interface{}); ok {
		sm.Brutal = domain.BrutalSpec{
			Enabled: boolCfg(brutal, "enabled"),
			Up:      strCfg(brutal, "up"),
			Down:    strCfg(brutal, "down"),
		}
	}
	return sm
}

// decodeTLSMirrorOpts forwards the listener-side `tlsmirror-config` block to
// the client-side `tlsmirror-opts` shape per proxies-tls wiki. The block is
// passed through verbatim since the listener and client structures are
// identical (primary-key + explicit-nonce-ciphersuites + transport-layer-
// padding + connection-enrolment + sequence-watermarking-enabled + embedded-
// traffic-generator). Returns nil when the listener has no tlsmirror-config.
func decodeTLSMirrorOpts(cfg map[string]interface{}) map[string]any {
	src, ok := cfg["tlsmirror-config"].(map[string]interface{})
	if !ok || len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// decodeECHOpts forwards the listener-side `ech-opts` block (a documented
// client-side TLS field per proxies-tls wiki block 0) to the m-ui client YAML
// emitter. The listener side carries `ech-key` (a raw key string) which would
// require non-trivial derivation to expand into a `config` list — for the
// LOW-priority path we forward the operator-supplied `ech-opts` block verbatim
// and leave the listener-side `ech-key` → `ech-opts.config` derivation as a
// follow-up. Returns nil when no `ech-opts` block is set.
func decodeECHOpts(cfg map[string]interface{}) map[string]any {
	src, ok := cfg["ech-opts"].(map[string]interface{})
	if !ok || len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// decodeGRPCSpec reads the optional grpc-opts fields documented on
// proxies-transport wiki block 2 (grpc-user-agent / ping-interval /
// max-connections / min-streams / max-streams). The listener schema only
// whitelists `grpc-service-name`; these extended fields can be set via the
// panel's free-form JSON editor and are surfaced on the m-ui client YAML
// verbatim when present.
func decodeGRPCSpec(cfg map[string]interface{}) *domain.GRPCSpec {
	g := &domain.GRPCSpec{ServiceName: strCfg(cfg, "grpc-service-name")}
	if v := strCfg(cfg, "grpc-user-agent"); v != "" {
		g.GRPCUserAgent = v
	}
	if v := intCfg(cfg, "ping-interval"); v != 0 {
		g.PingInterval = v
	}
	if v := intCfg(cfg, "max-connections"); v != 0 {
		g.MaxConnections = v
	}
	if v := intCfg(cfg, "min-streams"); v != 0 {
		g.MinStreams = v
	}
	if v := intCfg(cfg, "max-streams"); v != 0 {
		g.MaxStreams = v
	}
	return g
}

// intCfg reads a numeric field from a JSON-decoded map as int. Handles the
// float64 (encoding/json default) and integer widenings also accepted by
// uint32Cfg above.
func intCfg(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case uint32:
		return int(v)
	}
	return 0
}

// decodeMKCPConfig maps the listener mkcp-config block into the domain
// MKCPConfig consumed by the m-ui VMess module's compileClassicClient.
func decodeMKCPConfig(src map[string]interface{}) *domain.MKCPConfig {
	return &domain.MKCPConfig{
		MTU:              uint32Cfg(src, "mtu"),
		TTI:              uint32Cfg(src, "tti"),
		UplinkCapacity:   uint32Cfg(src, "uplink-capacity"),
		DownlinkCapacity: uint32Cfg(src, "downlink-capacity"),
		Congestion:       boolCfg(src, "congestion"),
		WriteBuffer:      uint32Cfg(src, "write-buffer"),
		ReadBuffer:       uint32Cfg(src, "read-buffer"),
		Seed:             strCfg(src, "seed"),
		Header:           strCfg(src, "header"),
	}
}

// uint32Cfg reads a numeric field from a JSON-decoded map as uint32.
// encoding/json decodes numbers as float64; int/int64 are also tolerated.
func uint32Cfg(m map[string]interface{}, key string) uint32 {
	switch v := m[key].(type) {
	case float64:
		return uint32(v)
	case int:
		return uint32(v)
	case int64:
		return uint32(v)
	case uint32:
		return v
	}
	return 0
}

func decodeSecurity(cfg map[string]interface{}, tlsFlag bool) domain.VLESSSecuritySpec {
	if raw, ok := cfg["reality-config"].(map[string]interface{}); ok && raw != nil {
		shortIDs := []string{}
		if s, ok := raw["short-id"].(string); ok && s != "" {
			shortIDs = []string{s}
		} else if arr, ok := raw["short-id"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					shortIDs = append(shortIDs, s)
				}
			}
		}
		names := []string{}
		if arr, ok := raw["server-names"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					names = append(names, s)
				}
			}
		}
		priv := strMap(raw, "private-key")
		pub := strMap(raw, "public-key")
		if pub == "" && priv != "" {
			if derived, err := deriveRealityPublicKey(priv); err == nil {
				pub = derived
			}
		}
		return domain.VLESSSecuritySpec{
			Type: domain.VLESSSecurityReality,
			Reality: &domain.RealityConfig{
				PrivateKey:  priv,
				PublicKey:   pub,
				ShortIDs:    shortIDs,
				ServerNames: names,
				Destination: strMap(raw, "dest"),
			},
		}
	}
	if tlsFlag || strCfg(cfg, "certificate") != "" {
		return domain.VLESSSecuritySpec{
			Type: domain.VLESSSecurityTLS,
			TLS: &domain.TLSConfig{
				Certificate:   strCfg(cfg, "certificate"),
				PrivateKey:    strCfg(cfg, "private-key"),
				AllowInsecure: boolCfg(cfg, "skip-cert-verify") || certutil.IsPanelSelfSignedPEM(strCfg(cfg, "certificate")),
			},
		}
	}
	return domain.VLESSSecuritySpec{Type: domain.VLESSSecurityNone}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func strCfg(cfg map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := cfg[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func strMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func boolCfg(cfg map[string]interface{}, key string) bool {
	b, _ := cfg[key].(bool)
	return b
}

func enrichAccessProfileFromNode(profile *domain.AccessProfile, node domain.Node, l models.Listener) {
	if profile == nil {
		return
	}
	if profile.Fingerprint == "" {
		profile.Fingerprint = firstNonEmpty(strings.TrimSpace(l.ClientFingerprint), domain.ClientFingerprint)
	}
	if profile.ServerName == "" {
		profile.ServerName = strings.TrimSpace(l.AccessSNI)
	}
	if profile.ServerName == "" && node.VLESS != nil && node.VLESS.Security.Reality != nil {
		if names := node.VLESS.Security.Reality.ServerNames; len(names) > 0 {
			profile.ServerName = names[0]
		}
	}
	if profile.ServerName == "" && node.VMess != nil && node.VMess.Security.Reality != nil {
		if names := node.VMess.Security.Reality.ServerNames; len(names) > 0 {
			profile.ServerName = names[0]
		}
	}
	if profile.ServerName == "" && node.Trojan != nil && node.Trojan.Security.Reality != nil {
		if names := node.Trojan.Security.Reality.ServerNames; len(names) > 0 {
			profile.ServerName = names[0]
		}
	}
	ensureRealityPublicKey(node.VLESS)
	ensureRealityPublicKeyVMess(node.VMess)
	ensureRealityPublicKeyTrojan(node.Trojan)
}

func ensureRealityPublicKey(spec *domain.VLESSSpec) {
	if spec == nil || spec.Security.Reality == nil {
		return
	}
	r := spec.Security.Reality
	if r.PublicKey == "" && r.PrivateKey != "" {
		if pub, err := deriveRealityPublicKey(r.PrivateKey); err == nil {
			r.PublicKey = pub
		}
	}
}

func ensureRealityPublicKeyVMess(spec *domain.VMessSpec) {
	if spec == nil || spec.Security.Reality == nil {
		return
	}
	r := spec.Security.Reality
	if r.PublicKey == "" && r.PrivateKey != "" {
		if pub, err := deriveRealityPublicKey(r.PrivateKey); err == nil {
			r.PublicKey = pub
		}
	}
}

func ensureRealityPublicKeyTrojan(spec *domain.TrojanSpec) {
	if spec == nil || spec.Security.Reality == nil {
		return
	}
	r := spec.Security.Reality
	if r.PublicKey == "" && r.PrivateKey != "" {
		if pub, err := deriveRealityPublicKey(r.PrivateKey); err == nil {
			r.PublicKey = pub
		}
	}
}

func listenerFlowHint(l models.Listener) string {
	if strings.TrimSpace(l.Config) == "" {
		return ""
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(l.Config), &cfg); err != nil {
		return ""
	}
	return strCfg(cfg, "flow")
}

func deriveRealityPublicKey(private string) (string, error) {
	private = strings.TrimSpace(private)
	if private == "" {
		return "", fmt.Errorf("empty private key")
	}
	var raw []byte
	var err error
	for _, decode := range []func(string) ([]byte, error){
		base64.RawURLEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.StdEncoding.DecodeString,
	} {
		raw, err = decode(private)
		if err == nil && len(raw) == 32 {
			break
		}
		raw = nil
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("invalid Reality private key")
	}
	pub, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(pub), nil
}
