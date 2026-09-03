package protocol

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/crypto/curve25519"
	"gopkg.in/yaml.v3"

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
	if spec.SNI != "" {
		params["sni"] = spec.SNI
	}
	if spec.Fingerprint != "" {
		params["fp"] = spec.Fingerprint
	}
	if spec.SkipCert {
		params["allowInsecure"] = "1"
	}
	applyTransportParams(params, spec.Transport)
	applyALPNParams(params, spec.ALPN)
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
		if params["sni"] == "" {
			params["sni"] = spec.Reality.ServerName
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
	if spec.Flow != "" {
		params["flow"] = spec.Flow
	}
	if spec.SNI != "" {
		params["sni"] = spec.SNI
	}
	if spec.Fingerprint != "" {
		params["fp"] = spec.Fingerprint
	}
	if spec.SkipCert {
		params["allowInsecure"] = "1"
	}
	applyTransportParams(params, spec.Transport)
	applyALPNParams(params, spec.ALPN)
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
		if params["sni"] == "" {
			params["sni"] = spec.Reality.ServerName
		}
		if params["fp"] == "" {
			params["fp"] = "chrome"
		}
	} else if in.Node.TLS {
		// Non-Reality TLS: emit security=tls so clients don't fall back to
		// plaintext VLESS (security=none) which the URI scheme defaults to.
		params["security"] = "tls"
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
	if spec.Flow != "" {
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
	} else if node.TLS {
		p["tls"] = true
		if spec.SNI != "" {
			p["servername"] = spec.SNI
		}
		if spec.Fingerprint != "" {
			p["client-fingerprint"] = spec.Fingerprint
		}
	}
	if spec.SkipCert {
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
		ws := map[string]interface{}{}
		if spec.Transport.WSPath != "" {
			ws["path"] = spec.Transport.WSPath
		}
		if spec.Transport.WSHost != "" {
			// Per mihomo wiki (proxies-transport: ws-opts), the Host
			// header must be nested under `headers`, not placed at the
			// top level of ws-opts. See /tmp/wiki/REF.txt block 3.
			headers := map[string]interface{}{}
			headers["Host"] = spec.Transport.WSHost
			ws["headers"] = headers
		}
		if len(ws) > 0 {
			p["network"] = "ws"
			p["ws-opts"] = ws
		}
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
	params := map[string]string{}
	if spec.SNI != "" {
		params["sni"] = spec.SNI
	}
	if spec.SkipCert {
		params["insecure"] = "1"
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
	uri := shareName(
		shareQuery("hysteria2://"+url.PathEscape(pass)+"@"+netutil.JoinHostPort(host, port), params),
		in.Node.Name,
	)
	return Share{URI: uri, QRContent: uri}, nil
}

// --- helpers ---

func applyTransportParams(params map[string]string, t TransportSpec) {
	switch t.Network {
	case "ws":
		params["type"] = "ws"
		if t.WSPath != "" {
			params["path"] = t.WSPath
		}
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
