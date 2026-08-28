package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
	"golang.org/x/crypto/curve25519"
	"gopkg.in/yaml.v3"
)

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
	params := map[string]string{
		"encryption": strOr(spec.Encryption, "none"),
		"type":       strOr(spec.Transport.Network, "tcp"),
	}
	if flow := strOr(in.User.Flow, spec.Flow); flow != "" {
		params["flow"] = flow
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
		if params["sni"] == "" && spec.Reality.ServerName != "" {
			params["sni"] = spec.Reality.ServerName
		}
		if params["fp"] == "" {
			params["fp"] = "chrome"
		}
	} else if in.Node.TLS || spec.SNI != "" {
		params["security"] = "tls"
	}
	uri := shareName(
		shareQuery("vless://"+url.PathEscape(uuid)+"@"+netutil.JoinHostPort(host, port), params),
		in.Node.Name,
	)
	yamlDoc, _ := clientYAMLProxy("vless", host, port, in, map[string]interface{}{
		"uuid": uuid, "flow": strOr(in.User.Flow, spec.Flow), "tls": params["security"] != "",
		"servername": params["sni"], "client-fingerprint": params["fp"],
		"reality-opts": realityOptsYAML(spec.Reality),
		"network":      params["type"],
	})
	return Share{URI: uri, QRContent: uri, ClientYAML: yamlDoc}, nil
}

// --- VMess ---

func (VMessCompiler) BuildShare(in ShareInput) (Share, error) {
	spec := in.Node.VMess
	if spec == nil {
		return Share{}, fmt.Errorf("vmess spec missing")
	}
	host, port, err := shareHostPort(in.Node, "")
	if err != nil {
		return Share{}, err
	}
	uuid := in.User.UUID
	if uuid == "" {
		return Share{}, fmt.Errorf("vmess share requires uuid")
	}
	obj := map[string]string{
		"v": "2", "ps": in.Node.Name, "add": host, "port": port, "id": uuid,
		"aid": strconv.Itoa(spec.AlterID), "scy": strOr(spec.Cipher, "auto"),
		"net": strOr(spec.Transport.Network, "tcp"), "type": "none",
	}
	if spec.Transport.Network == "ws" {
		obj["path"] = spec.Transport.WSPath
	}
	if spec.Transport.Network == "grpc" {
		obj["path"] = spec.Transport.GRPCService
	}
	if spec.Reality != nil {
		obj["tls"] = "reality"
		pbk, err := realityPublicKeyFromSpec(spec.Reality)
		if err != nil {
			return Share{}, err
		}
		obj["pbk"] = pbk
		obj["sid"] = spec.Reality.ShortID
		obj["sni"] = firstNonEmpty(spec.SNI, spec.Reality.ServerName)
		obj["fp"] = strOr(spec.Fingerprint, "chrome")
	} else if in.Node.TLS || spec.SNI != "" {
		obj["tls"] = "tls"
		obj["sni"] = spec.SNI
		obj["fp"] = spec.Fingerprint
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return Share{}, err
	}
	uri := "vmess://" + base64.RawStdEncoding.EncodeToString(raw)
	return Share{URI: uri, QRContent: uri}, nil
}

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
	encoded := base64.RawStdEncoding.EncodeToString([]byte(spec.Cipher + ":" + pass))
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

func realityPublicKeyFromSpec(r *RealitySpec) (string, error) {
	if r == nil {
		return "", fmt.Errorf("reality config required")
	}
	if strings.TrimSpace(r.PublicKey) != "" {
		return r.PublicKey, nil
	}
	private := strings.TrimSpace(r.PrivateKey)
	if private == "" {
		return "", fmt.Errorf("reality public-key or private-key required")
	}
	var raw []byte
	var err error
	for _, decode := range []func(string) ([]byte, error){
		base64.RawStdEncoding.DecodeString,
		base64.StdEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
	} {
		raw, err = decode(private)
		if err == nil && len(raw) == 32 {
			break
		}
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("invalid Reality private key")
	}
	pub, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(pub), nil
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
		"udp":    true,
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

