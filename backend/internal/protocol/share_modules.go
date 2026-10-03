package protocol

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"golang.org/x/crypto/curve25519"
	"gopkg.in/yaml.v3"

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
)

// --- Trojan ---

func (TrojanCompiler) BuildShare(in ShareInput) (Share, error) {
	spec := in.Node.Trojan
	if spec == nil {
		return Share{}, fmt.Errorf("trojan spec missing")
	}
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	pass := in.User.Password
	if pass == "" {
		return Share{}, fmt.Errorf("trojan share requires password")
	}
	params := map[string]string{"type": strOr(spec.Transport.Network, "tcp")}
	if spec.Fingerprint != "" {
		params["fp"] = spec.Fingerprint
	}
	applyTransportParams(params, spec.Transport)
	applyALPNParams(params, spec.ALPN)
	if params["type"] == "ws" && params["host"] == "" {
		if h := effectiveWSHost(spec.Transport, in.Node, spec.SNI); h != "" {
			params["host"] = h
		}
	}
	if spec.Reality != nil {
		params["security"] = "reality"
		pbk, err := realityPublicKeyFromSpec(spec.Reality)
		if err != nil {
			return Share{}, err
		}
		params["pbk"] = pbk
		if spec.Reality.ShortID != "" {
			params["sid"] = spec.Reality.ShortID
		}
		if sni := strOr(spec.SNI, spec.Reality.ServerName); sni != "" {
			params["sni"] = sni
		}
		if params["fp"] == "" {
			params["fp"] = "chrome"
		}
		// Reality camouflage cert is not the real endpoint identity.
		params["allowInsecure"] = "1"
		params["insecure"] = "1"
	} else {
		sni, skip := resolveShareTLS(in, spec.SNI, host, spec.SkipCert)
		if sni != "" {
			params["sni"] = sni
		}
		if skip {
			params["allowInsecure"] = "1"
			params["insecure"] = "1"
		}
		if params["fp"] == "" {
			params["fp"] = "chrome"
		}
	}
	uri := shareName(
		shareQuery("trojan://"+url.PathEscape(pass)+"@"+netutil.JoinHostPort(host, port), params),
		in.Node.Name,
	)
	return Share{URI: uri, QRContent: uri}, nil
}

// --- VLESS ---

func (VLESSCompiler) BuildShare(in ShareInput) (Share, error) {
	spec := in.Node.VLESS
	if spec == nil {
		return Share{}, fmt.Errorf("vless spec missing")
	}
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	uuid := in.User.UUID
	if uuid == "" {
		return Share{}, fmt.Errorf("vless share requires uuid")
	}
	params := map[string]string{"type": strOr(spec.Transport.Network, "tcp")}
	if spec.Encryption != "" {
		params["encryption"] = spec.Encryption
	} else {
		params["encryption"] = "none"
	}
	if spec.Flow != "" && spec.Transport.CarriesFlow() {
		params["flow"] = spec.Flow
	}
	if spec.Fingerprint != "" {
		params["fp"] = spec.Fingerprint
	}
	applyTransportParams(params, spec.Transport)
	applyALPNParams(params, spec.ALPN)
	if params["type"] == "ws" && params["host"] == "" {
		if h := effectiveWSHost(spec.Transport, in.Node, spec.SNI); h != "" {
			params["host"] = h
		}
	}
	if spec.Reality != nil {
		params["security"] = "reality"
		pbk, err := realityPublicKeyFromSpec(spec.Reality)
		if err != nil {
			return Share{}, err
		}
		params["pbk"] = pbk
		if spec.Reality.ShortID != "" {
			params["sid"] = spec.Reality.ShortID
		}
		if sni := strOr(spec.SNI, spec.Reality.ServerName); sni != "" {
			params["sni"] = sni
		}
		if params["fp"] == "" {
			params["fp"] = "chrome"
		}
		params["allowInsecure"] = "1"
		params["insecure"] = "1"
	} else if in.Node.TLS || shareNeedsClientTLS(in.Node, spec.Transport.Network, host, port) || strings.TrimSpace(in.Node.Certificate) != "" {
		params["security"] = "tls"
		sni, skip := resolveShareTLS(in, spec.SNI, host, spec.SkipCert)
		if sni != "" {
			params["sni"] = sni
		}
		if skip {
			params["allowInsecure"] = "1"
			params["insecure"] = "1"
		}
		if params["fp"] == "" {
			params["fp"] = strOr(in.Node.Fingerprint, "chrome")
		}
	}
	uri := shareName(
		shareQuery("vless://"+url.PathEscape(uuid)+"@"+netutil.JoinHostPort(host, port), params),
		in.Node.Name,
	)
	yamlOut, err := vlessClientYAML(in.Node, host, port, uuid, spec)
	if err != nil {
		return Share{}, err
	}
	return Share{URI: uri, QRContent: uri, ClientYAML: yamlOut}, nil
}

func vlessClientYAML(node NodeModel, host, port, uuid string, spec *VLESSSpec) (string, error) {
	p := map[string]interface{}{
		"name":   node.Name,
		"type":   "vless",
		"server": host,
		"port":   portValue(port),
		"uuid":   uuid,
	}
	if node.UDP {
		p["udp"] = true
	}
	if spec.Flow != "" && spec.Transport.CarriesFlow() {
		p["flow"] = spec.Flow
	}
	// tls / reality
	if spec.Reality != nil {
		// reality implies tls; mihomo expects tls:true plus reality-opts
		p["tls"] = true
		if ro := realityOptsYAML(spec.Reality); ro != nil {
			p["reality-opts"] = ro
		}
		if spec.SNI != "" {
			p["servername"] = spec.SNI
		} else if spec.Reality.ServerName != "" {
			p["servername"] = spec.Reality.ServerName
		}
		if spec.Fingerprint != "" {
			p["client-fingerprint"] = spec.Fingerprint
		} else {
			p["client-fingerprint"] = "chrome"
		}
	} else if node.TLS || shareNeedsClientTLS(node, spec.Transport.Network, host, port) || strings.TrimSpace(node.Certificate) != "" {
		p["tls"] = true
		sni, skip := resolveShareTLS(ShareInput{Node: node}, spec.SNI, host, spec.SkipCert)
		if sni != "" {
			p["servername"] = sni
			p["sni"] = sni
		}
		if skip {
			p["skip-cert-verify"] = true
		}
		if spec.Fingerprint != "" {
			p["client-fingerprint"] = spec.Fingerprint
		} else {
			p["client-fingerprint"] = strOr(node.Fingerprint, "chrome")
		}
	}
	if _, isReality := p["reality-opts"]; isReality {
		p["skip-cert-verify"] = true
	}
	if len(spec.ALPN) > 0 {
		clean := make([]string, 0, len(spec.ALPN))
		for _, a := range spec.ALPN {
			if a = strings.TrimSpace(a); a != "" {
				clean = append(clean, a)
			}
		}
		if len(clean) > 0 {
			p["alpn"] = clean
		}
	}
	switch spec.Transport.Network {
	case "ws":
		// Always emit path (default "/") and Host when resolvable so CDN-fronted
		// VLESS/WS clients match the access profile without a separate ws-headers field.
		ws := map[string]interface{}{"path": effectiveWSPath(spec.Transport)}
		if h := effectiveWSHost(spec.Transport, node, spec.SNI); h != "" {
			ws["headers"] = map[string]interface{}{"Host": h}
		}
		p["network"] = "ws"
		p["ws-opts"] = ws
	case "grpc":
		grpc := map[string]interface{}{}
		if spec.Transport.GRPCService != "" {
			grpc["grpc-service-name"] = spec.Transport.GRPCService
		}
		if len(grpc) > 0 {
			p["network"] = "grpc"
			p["grpc-opts"] = grpc
		}
	case "xhttp":
		xh := map[string]interface{}{}
		if spec.Transport.XHTTPPath != "" {
			xh["path"] = spec.Transport.XHTTPPath
		}
		if len(xh) > 0 {
			p["network"] = "xhttp"
			p["xhttp-opts"] = xh
		}
	}
	raw, err := yaml.Marshal(map[string]interface{}{"proxies": []map[string]interface{}{p}})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// --- Shadowsocks ---

func (ShadowsocksCompiler) BuildShare(in ShareInput) (Share, error) {
	spec := in.Node.Shadowsocks
	if spec == nil {
		return Share{}, fmt.Errorf("shadowsocks spec missing")
	}
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	pass := in.User.Password
	if pass == "" {
		pass = spec.Password
	}
	if spec.Cipher == "" || pass == "" {
		return Share{}, fmt.Errorf("shadowsocks share requires cipher and password")
	}
	encoded := base64.RawURLEncoding.EncodeToString([]byte(spec.Cipher + ":" + pass))
	uri := shareName("ss://"+encoded+"@"+netutil.JoinHostPort(host, port), in.Node.Name)
	return Share{URI: uri, QRContent: uri}, nil
}

// --- Hysteria2 ---

func (Hysteria2Compiler) BuildShare(in ShareInput) (Share, error) {
	spec := in.Node.Hysteria2
	if spec == nil {
		return Share{}, fmt.Errorf("hysteria2 spec missing")
	}
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	pass := in.User.Password
	if pass == "" {
		return Share{}, fmt.Errorf("hysteria2 share requires password")
	}
	configured := strings.TrimSpace(spec.SNI)
	if configured == "" {
		configured = strings.TrimSpace(in.Node.AccessSNI)
	}
	sni, skipCert := resolveShareTLS(in, configured, host, spec.SkipCert)
	params := map[string]string{}
	if sni != "" {
		params["sni"] = sni
	}
	if skipCert {
		// insecure = Meta/Clash; allowInsecure = Loon / some mobile parsers
		params["insecure"] = "1"
		params["allowInsecure"] = "1"
	}
	if spec.Obfs != "" {
		params["obfs"] = spec.Obfs
	}
	if spec.ObfsPassword != "" {
		params["obfs-password"] = spec.ObfsPassword
	}
	if spec.Up != "" {
		params["up"] = spec.Up
	}
	if spec.Down != "" {
		params["down"] = spec.Down
	}
	applyALPNParams(params, spec.ALPN)
	if params["alpn"] == "" {
		params["alpn"] = "h3"
	}
	// Always emit sni for HY2 (IP formal certs previously produced bare host:port).
	if params["sni"] == "" && host != "" {
		params["sni"] = strings.Trim(host, "[]")
	}
	userinfo := url.User(pass).String()
	uri := shareName(
		shareQuery("hysteria2://"+userinfo+"@"+netutil.JoinHostPort(host, port), params),
		in.Node.Name,
	)
	extra := map[string]interface{}{"password": pass}
	if sni != "" {
		extra["sni"] = sni
	}
	if skipCert {
		extra["skip-cert-verify"] = true
	}
	if spec.Obfs != "" {
		extra["obfs"] = spec.Obfs
	}
	if spec.ObfsPassword != "" {
		extra["obfs-password"] = spec.ObfsPassword
	}
	if spec.Up != "" {
		extra["up"] = spec.Up
	}
	if spec.Down != "" {
		extra["down"] = spec.Down
	}
	if len(spec.ALPN) > 0 {
		extra["alpn"] = spec.ALPN
	}
	yamlOut, err := clientYAMLProxy("hysteria2", host, port, in, extra)
	if err != nil {
		return Share{}, err
	}
	return Share{URI: uri, QRContent: uri, ClientYAML: yamlOut}, nil
}

// --- VMess (https://wiki.metacubex.one/config/proxies/vmess/) ---

func (VMessCompiler) BuildShare(in ShareInput) (Share, error) {
	spec := in.Node.VMess
	if spec == nil {
		return Share{}, fmt.Errorf("vmess spec missing")
	}
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	uuid := strings.TrimSpace(in.User.UUID)
	if uuid == "" {
		uuid = strings.TrimSpace(in.User.Username)
	}
	if uuid == "" {
		return Share{}, fmt.Errorf("vmess share requires uuid")
	}
	cipher := strOr(spec.Cipher, "auto")
	extra := map[string]interface{}{
		"uuid":    uuid,
		"alterId": spec.AlterID,
		"cipher":  cipher,
		"udp":     true,
	}
	netw := strOr(spec.Transport.Network, "tcp")
	if netw != "" && netw != "tcp" {
		extra["network"] = netw
	}
	if spec.Reality != nil {
		extra["tls"] = true
		if opts := realityOptsYAML(spec.Reality); opts != nil {
			extra["reality-opts"] = opts
		}
		if sni := strOr(spec.SNI, spec.Reality.ServerName); sni != "" {
			extra["servername"] = sni
		}
		extra["client-fingerprint"] = strOr(spec.Fingerprint, "chrome")
		extra["skip-cert-verify"] = true
	} else if in.Node.TLS || spec.SkipCert || spec.SNI != "" || strings.TrimSpace(in.Node.Certificate) != "" {
		extra["tls"] = true
		sni, skip := resolveShareTLS(in, spec.SNI, host, spec.SkipCert)
		if sni != "" {
			extra["servername"] = sni
			extra["sni"] = sni
		}
		if skip {
			extra["skip-cert-verify"] = true
		}
		extra["client-fingerprint"] = strOr(spec.Fingerprint, strOr(in.Node.Fingerprint, "chrome"))
	}
	if len(spec.ALPN) > 0 {
		extra["alpn"] = spec.ALPN
	}
	if netw == "ws" && spec.Transport.WSPath != "" {
		// Per mihomo wiki (proxies-transport: ws-opts), the Host header belongs
		// under `headers`, not at the top level of ws-opts. vlessClientYAML
		// already emits it that way; a ws listener that pins a Host is
		// unreachable when the client YAML leaves it out.
		wsOpts := map[string]interface{}{"path": spec.Transport.WSPath}
		if spec.Transport.WSHost != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": spec.Transport.WSHost}
		}
		extra["ws-opts"] = wsOpts
	}
	if netw == "grpc" && spec.Transport.GRPCService != "" {
		extra["grpc-opts"] = map[string]interface{}{"grpc-service-name": spec.Transport.GRPCService}
	}
	yamlOut, err := clientYAMLProxy("vmess", host, port, in, extra)
	if err != nil {
		return Share{}, err
	}
	return Share{ClientYAML: yamlOut}, nil
}

// --- Snell / Sudoku (GenericCompiler) ---
// https://wiki.metacubex.one/config/proxies/snell/

func (g GenericCompiler) BuildShare(in ShareInput) (Share, error) {
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	cfg := in.Node.Generic
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	kind := strings.ToLower(g.Kind())
	switch kind {
	case "snell":
		psk := strings.TrimSpace(in.User.Password)
		if psk == "" {
			psk = strFrom(cfg, "psk")
		}
		if psk == "" {
			return Share{}, fmt.Errorf("snell share requires psk")
		}
		extra := map[string]interface{}{"psk": psk, "udp": true}
		if ver := cfg["version"]; ver != nil {
			extra["version"] = ver
		} else {
			extra["version"] = 4
		}
		yamlOut, err := clientYAMLProxy("snell", host, port, in, extra)
		if err != nil {
			return Share{}, err
		}
		return Share{ClientYAML: yamlOut}, nil
	case "sudoku":
		key := strings.TrimSpace(in.User.Password)
		if key == "" {
			key = strFrom(cfg, "key")
		}
		if key == "" {
			return Share{}, fmt.Errorf("sudoku share requires key")
		}
		extra := map[string]interface{}{"key": key}
		if m := strFrom(cfg, "aead-method"); m != "" {
			extra["aead-method"] = m
		}
		yamlOut, err := clientYAMLProxy("sudoku", host, port, in, extra)
		if err != nil {
			return Share{}, err
		}
		return Share{ClientYAML: yamlOut}, nil
	default:
		return Share{}, fmt.Errorf("protocol %q does not implement share export", kind)
	}
}

// --- AnyTLS https://wiki.metacubex.one/config/proxies/anytls/ ---

func (AnyTLSCompiler) BuildShare(in ShareInput) (Share, error) {
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	cfg := in.Node.Generic
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	pass := strings.TrimSpace(in.User.Password)
	if pass == "" {
		pass = strFrom(cfg, "password")
	}
	if pass == "" {
		return Share{}, fmt.Errorf("anytls share requires password")
	}
	fp := strOr(strings.TrimSpace(in.Node.Fingerprint), "chrome")

	var explicitSkip *bool
	if v, ok := cfg["skip-cert-verify"].(bool); ok {
		explicitSkip = &v
	}
	exp := false
	if explicitSkip != nil {
		exp = *explicitSkip
	}
	sni, skipCert := resolveShareTLS(in, strOr(strings.TrimSpace(in.Node.AccessSNI), strFrom(cfg, "sni", "servername")), host, exp)

	params := map[string]string{}
	if sni != "" {
		params["sni"] = sni
	}
	if fp != "" {
		params["fp"] = fp
	}
	if skipCert {
		params["insecure"] = "1"
		params["allowInsecure"] = "1"
	}
	uri := shareName(
		shareQuery("anytls://"+url.PathEscape(pass)+"@"+netutil.JoinHostPort(host, port), params),
		in.Node.Name,
	)

	extra := map[string]interface{}{"password": pass, "udp": true}
	extra["sni"] = sni
	extra["client-fingerprint"] = fp
	extra["skip-cert-verify"] = skipCert
	if alpn := stringListFrom(cfg, "alpn"); len(alpn) > 0 {
		extra["alpn"] = alpn
	}
	yamlOut, err := clientYAMLProxy("anytls", host, port, in, extra)
	if err != nil {
		return Share{}, err
	}
	return Share{URI: uri, QRContent: uri, ClientYAML: yamlOut}, nil
}

// resolveShareCertPEM returns PEM text from config (inline or file path).
func resolveShareCertPEM(cfg map[string]interface{}) string {
	if cfg == nil {
		return ""
	}
	cert, _ := cfg["certificate"].(string)
	cert = strings.TrimSpace(cert)
	if cert == "" {
		return ""
	}
	if strings.Contains(cert, "BEGIN CERTIFICATE") {
		return cert
	}
	// Path on disk (common when operators paste Let's Encrypt fullchain path).
	if strings.HasPrefix(cert, "/") || strings.HasSuffix(cert, ".pem") || strings.HasSuffix(cert, ".crt") {
		if b, err := os.ReadFile(cert); err == nil && len(b) > 0 {
			return string(b)
		}
	}
	return cert
}

// --- ShadowQUIC https://wiki.metacubex.one/config/proxies/shadowquic/ ---

func (ShadowQUICCompiler) BuildShare(in ShareInput) (Share, error) {
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	cfg := in.Node.Generic
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	user := strings.TrimSpace(in.User.Username)
	pass := strings.TrimSpace(in.User.Password)
	if user == "" {
		user = strFrom(cfg, "username")
	}
	if pass == "" {
		pass = strFrom(cfg, "password")
	}
	if user == "" || pass == "" {
		return Share{}, fmt.Errorf("shadowquic share requires username and password")
	}
	extra := map[string]interface{}{"username": user, "password": pass}
	sni, skip := resolveShareTLS(in, strOr(strFrom(cfg, "sni"), in.Node.AccessSNI), host, false)
	if sni != "" {
		extra["sni"] = sni
	}
	if skip {
		extra["skip-cert-verify"] = true
	}
	if alpn := stringListFrom(cfg, "alpn"); len(alpn) > 0 {
		extra["alpn"] = alpn
	} else {
		extra["alpn"] = []string{"h3"}
	}
	if cc, _ := cfg["congestion-controller"].(string); strings.TrimSpace(cc) != "" {
		extra["congestion-controller"] = cc
	} else {
		extra["congestion-controller"] = "cubic"
	}
	yamlOut, err := clientYAMLProxy("shadowquic", host, port, in, extra)
	if err != nil {
		return Share{}, err
	}
	return Share{ClientYAML: yamlOut}, nil
}

// --- Mieru https://wiki.metacubex.one/config/proxies/mieru/ ---

func (MieruCompiler) BuildShare(in ShareInput) (Share, error) {
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	cfg := in.Node.Generic
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	user := strings.TrimSpace(in.User.Username)
	pass := strings.TrimSpace(in.User.Password)
	if user == "" || pass == "" {
		return Share{}, fmt.Errorf("mieru share requires username and password")
	}
	extra := map[string]interface{}{"username": user, "password": pass}
	if tr := strFrom(cfg, "transport"); tr != "" {
		extra["transport"] = tr
	} else {
		extra["transport"] = "TCP"
	}
	if m := strFrom(cfg, "multiplexing"); m != "" {
		extra["multiplexing"] = m
	}
	yamlOut, err := clientYAMLProxy("mieru", host, port, in, extra)
	if err != nil {
		return Share{}, err
	}
	return Share{ClientYAML: yamlOut}, nil
}

// --- TrustTunnel ---

func (TrustTunnelCompiler) BuildShare(in ShareInput) (Share, error) {
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	cfg := in.Node.Generic
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	user := strings.TrimSpace(in.User.Username)
	pass := strings.TrimSpace(in.User.Password)
	if pass == "" {
		return Share{}, fmt.Errorf("trusttunnel share requires password")
	}
	extra := map[string]interface{}{"password": pass}
	if user != "" {
		extra["username"] = user
	}
	sni, skip := resolveShareTLS(in, strOr(in.Node.AccessSNI, strFrom(cfg, "sni")), host, false)
	if sni != "" {
		extra["sni"] = sni
	}
	extra["skip-cert-verify"] = skip
	yamlOut, err := clientYAMLProxy("trusttunnel", host, port, in, extra)
	if err != nil {
		return Share{}, err
	}
	return Share{ClientYAML: yamlOut}, nil
}

// --- TUIC (inbound type is always "tuic"; v4=token, v5=uuid+password) ---
// Docs: https://wiki.metacubex.one/config/proxies/tuic/
//       https://wiki.metacubex.one/config/inbound/listeners/tuic-v4/
//       https://wiki.metacubex.one/config/inbound/listeners/tuic-v5/

func (t TUICCompiler) BuildShare(in ShareInput) (Share, error) {
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	cfg := in.Node.Generic
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	kind := t.Kind()
	alpn := stringListFrom(cfg, "alpn")
	if len(alpn) == 0 {
		alpn = []string{"h3"}
	}
	cc, _ := cfg["congestion-controller"].(string)
	if strings.TrimSpace(cc) == "" {
		cc = "bbr"
	}
	var explicitSkip *bool
	if v, ok := cfg["skip-cert-verify"].(bool); ok {
		explicitSkip = &v
	}
	exp := false
	if explicitSkip != nil {
		exp = *explicitSkip
	}
	sni, skipCert := resolveShareTLS(in, strOr(strings.TrimSpace(in.Node.AccessSNI), strFrom(cfg, "sni", "servername")), host, exp)

	isV4 := kind == "tuic-v4"
	if kind == "tuic-v5" {
		isV4 = false
	} else if kind == "tuic" {
		// Legacy: token without users → v4; else v5.
		hasTok := firstNonEmptyTokenValue(cfg["token"]) != ""
		hasUsers := false
		if um, ok := cfg["users"].(map[string]interface{}); ok && len(um) > 0 {
			hasUsers = true
		}
		uuid := strings.TrimSpace(in.User.UUID)
		if uuid == "" {
			uuid = strings.TrimSpace(in.User.Username)
		}
		if hasTok && !hasUsers {
			isV4 = true
		} else if looksLikeUUID(uuid) && strings.TrimSpace(in.User.Password) != "" {
			isV4 = false
		} else {
			isV4 = hasTok && !looksLikeUUID(uuid)
		}
	}

	params := map[string]string{
		"congestion_control": cc,
		"udp_relay_mode":     "native",
	}
	if sni != "" {
		params["sni"] = sni
	}
	applyALPNParams(params, alpn)
	if skipCert {
		params["allow_insecure"] = "1"
		params["allowInsecure"] = "1"
	}

	extra := map[string]interface{}{
		"udp":                   true,
		"congestion-controller": cc,
		"udp-relay-mode":        "native",
		"alpn":                  alpn,
	}
	if sni != "" {
		extra["sni"] = sni
	}
	if skipCert {
		extra["skip-cert-verify"] = true
	}

	var uri string
	if isV4 {
		// Token lives on the listener Config — never the panel user password.
		token := firstNonEmptyTokenValue(cfg["token"])
		if token == "" {
			token = strings.TrimSpace(in.User.Password)
			// Only accept User.Password when it came from config decode (no UUID).
			if strings.TrimSpace(in.User.UUID) != "" || looksLikeUUID(in.User.Username) {
				token = ""
			}
		}
		if token == "" {
			return Share{}, fmt.Errorf("tuic-v4 share requires token in listener config")
		}
		uri = shareName(
			shareQuery("tuic://"+url.PathEscape(token)+"@"+netutil.JoinHostPort(host, port), params),
			in.Node.Name,
		)
		extra["token"] = token
	} else {
		uuid := strings.TrimSpace(in.User.UUID)
		if uuid == "" {
			uuid = strings.TrimSpace(in.User.Username)
		}
		pass := strings.TrimSpace(in.User.Password)
		if uuid == "" || pass == "" {
			return Share{}, fmt.Errorf("tuic-v5 share requires uuid and password")
		}
		userinfo := url.UserPassword(uuid, pass).String()
		uri = shareName(
			shareQuery("tuic://"+userinfo+"@"+netutil.JoinHostPort(host, port), params),
			in.Node.Name,
		)
		extra["uuid"] = uuid
		extra["password"] = pass
	}

	yamlOut, err := clientYAMLProxy("tuic", host, port, in, extra)
	if err != nil {
		return Share{}, err
	}
	return Share{URI: uri, QRContent: uri, ClientYAML: yamlOut}, nil
}

// --- helpers ---

// effectiveWSHost picks the WebSocket Host header for client share/YAML.
// CDN-fronted nodes almost always need Host; when the panel left ws-headers
// empty, fall back to SNI then public host so exports match the access profile.

// shareNeedsClientTLS is true when the client must speak TLS to the published
// endpoint (CDN edge / domain on 443) even if the origin listener is plain WS.
func shareNeedsClientTLS(node NodeModel, network, host, port string) bool {
	netw := strings.ToLower(strings.TrimSpace(network))
	if netw != "ws" && netw != "grpc" && netw != "xhttp" {
		return false
	}
	p := strings.TrimSpace(port)
	if p == "" {
		p = strings.TrimSpace(node.PublicPort)
	}
	if p != "443" && p != "8443" && p != "2053" && p != "2083" && p != "2087" && p != "2096" {
		return false
	}
	hint := strings.TrimSpace(node.AccessSNI)
	if hint == "" {
		hint = strings.TrimSpace(node.PublicHost)
	}
	if hint == "" {
		hint = strings.TrimSpace(host)
	}
	hint = strings.Trim(hint, "[]")
	return hint != "" && net.ParseIP(hint) == nil
}

func effectiveWSHost(t TransportSpec, node NodeModel, sni string) string {
	if h := strings.TrimSpace(t.WSHost); h != "" {
		return h
	}
	if h := strings.TrimSpace(sni); h != "" {
		return h
	}
	return strings.TrimSpace(node.PublicHost)
}

// effectiveWSPath returns the WS path for share export (default "/").
func effectiveWSPath(t TransportSpec) string {
	if p := strings.TrimSpace(t.WSPath); p != "" {
		return p
	}
	return "/"
}

func applyTransportParams(params map[string]string, t TransportSpec) {
	switch t.Network {
	case "ws":
		params["type"] = "ws"
		params["path"] = effectiveWSPath(t)
		if t.WSHost != "" {
			params["host"] = t.WSHost
		}
	case "grpc":
		params["type"] = "grpc"
		if t.GRPCService != "" {
			params["serviceName"] = t.GRPCService
		}
	case "xhttp":
		params["type"] = "xhttp"
		if t.XHTTPPath != "" {
			params["path"] = t.XHTTPPath
		}
	}
}

// resolveShareTLS is the single source of truth for share/subscription SNI and
// skip-cert: same rules as node.resolveURITLS / certutil.ResolveClientSNI.
// IP connect hosts with formal (e.g. LE IP SAN) certificates emit sni=<IP>.
func resolveShareTLS(in ShareInput, configuredSNI, host string, explicitSkip bool) (sni string, skip bool) {
	certPEM := materializeListenerCertPEM(strings.TrimSpace(in.Node.Certificate), in.Node.ID)
	if !strings.Contains(certPEM, "BEGIN CERTIFICATE") {
		if in.Node.Generic != nil {
			if c, ok := in.Node.Generic["certificate"].(string); ok {
				certPEM = materializeListenerCertPEM(c, in.Node.ID)
			}
		}
	}
	if !strings.Contains(certPEM, "BEGIN CERTIFICATE") {
		certPEM = materializeListenerCertPEM(resolveShareCertPEM(in.Node.Generic), in.Node.ID)
	}
	sni = certutil.ResolveClientSNI(strings.TrimSpace(configuredSNI), in.Node.PublicHost, host, certPEM)
	// QUIC/TLS clients (Loon HY2, TUIC, …) need an explicit sni on IP endpoints.
	if strings.TrimSpace(sni) == "" {
		sni = strings.Trim(strings.TrimSpace(host), "[]")
	}
	var exp *bool
	if explicitSkip {
		v := true
		exp = &v
	} else if in.Node.Generic != nil {
		if b, ok := in.Node.Generic["skip-cert-verify"].(bool); ok {
			exp = &b
		}
	}
	verify := sni
	if verify == "" {
		verify = host
	}
	skip = certutil.DecideClientSkipCertVerify(certPEM, verify, exp)
	return sni, skip
}

// shareTLSServerName picks a hostname suitable for TLS SNI on share links.
// Prefers non-IP candidates so clients (Loon, etc.) can complete the handshake
// when the published endpoint is a raw address.
func shareTLSServerName(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		c = strings.Trim(c, "[]")
		if c == "" || net.ParseIP(c) != nil {
			continue
		}
		return c
	}
	return ""
}

func shareHostLooksLikeIP(host string) bool {
	h := strings.TrimSpace(host)
	h = strings.Trim(h, "[]")
	return h != "" && net.ParseIP(h) != nil
}

func shareSkipCert(in ShareInput, configuredSNI string, host string, explicit bool) bool {
	_, skip := resolveShareTLS(in, configuredSNI, host, explicit)
	return skip
}

func applyALPNParams(params map[string]string, alpn []string) {
	clean := make([]string, 0, len(alpn))
	for _, value := range alpn {
		if value = strings.TrimSpace(value); value != "" {
			clean = append(clean, value)
		}
	}
	if len(clean) > 0 {
		params["alpn"] = strings.Join(clean, ",")
	}
}

func realityPublicKeyFromSpec(r *RealitySpec) (string, error) {
	if r == nil {
		return "", fmt.Errorf("reality config required")
	}
	publicRaw, publicSet, err := decodeRealityKey(r.PublicKey)
	if err != nil {
		return "", fmt.Errorf("invalid Reality public key: %w", err)
	}
	private := strings.TrimSpace(r.PrivateKey)
	if private == "" && !publicSet {
		return "", fmt.Errorf("reality public-key or private-key required")
	}
	if private != "" {
		privateRaw, _, err := decodeRealityKey(private)
		if err != nil {
			return "", fmt.Errorf("invalid Reality private key: %w", err)
		}
		pub, err := curve25519.X25519(privateRaw, curve25519.Basepoint)
		if err != nil {
			return "", err
		}
		if publicSet && !bytes.Equal(publicRaw, pub) {
			return "", fmt.Errorf("Reality public key does not match private key")
		}
		publicRaw = pub
	}
	return base64.RawURLEncoding.EncodeToString(publicRaw), nil
}

func decodeRealityKey(value string) ([]byte, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false, nil
	}
	var lastErr error
	for _, decode := range []func(string) ([]byte, error){
		base64.RawURLEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.StdEncoding.DecodeString,
	} {
		raw, err := decode(value)
		if err == nil {
			if len(raw) != 32 {
				return nil, true, fmt.Errorf("must decode to 32 bytes")
			}
			return raw, true, nil
		}
		lastErr = err
	}
	return nil, true, lastErr
}

func realityOptsYAML(r *RealitySpec) map[string]interface{} {
	if r == nil {
		return nil
	}
	pbk, err := realityPublicKeyFromSpec(r)
	if err != nil {
		return nil
	}
	m := map[string]interface{}{"public-key": pbk}
	if r.ShortID != "" {
		m["short-id"] = r.ShortID
	}
	return m
}

func clientYAMLProxy(typ, host, port string, in ShareInput, extra map[string]interface{}) (string, error) {
	p := map[string]interface{}{
		"name":   in.Node.Name,
		"type":   typ,
		"server": host,
		"port":   portValue(port),
	}
	if in.Node.UDP {
		p["udp"] = true
	}
	for k, v := range extra {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		p[k] = v
	}
	raw, err := yaml.Marshal(map[string]interface{}{"proxies": []map[string]interface{}{p}})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func strOr(v, def string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func firstNonEmptyTokenValue(tok interface{}) string {
	switch v := tok.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		for _, s := range v {
			if s = strings.TrimSpace(s); s != "" {
				return s
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					return s
				}
			}
		}
	}
	return ""
}
