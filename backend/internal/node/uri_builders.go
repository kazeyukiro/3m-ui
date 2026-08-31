package node

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
	"golang.org/x/crypto/curve25519"
)

func tlsParams(cfg map[string]interface{}) map[string]string {
	params := map[string]string{}
	if enabled, ok := cfg["_listener-tls"].(bool); ok && enabled {
		params["security"] = "tls"
	}
	if certificate, ok := cfg["certificate"].(string); ok && strings.TrimSpace(certificate) != "" {
		params["security"] = "tls"
	}
	if v, ok := cfg["servername"].(string); ok && v != "" {
		params["sni"] = v
	}
	if v, ok := cfg["sni"].(string); ok && v != "" {
		params["sni"] = v
	}
	if v, ok := cfg["client-fingerprint"].(string); ok && v != "" {
		params["fp"] = v
	}
	if v, ok := cfg["fingerprint"].(string); ok && v != "" {
		params["fp"] = v
	}
	if certutil.ShouldSkipCertVerify(cfg) {
		params["allowInsecure"] = "1"
	}
	return params
}

func transportParams(cfg map[string]interface{}) map[string]string {
	params := map[string]string{}
	if v, ok := cfg["ws-path"].(string); ok && v != "" {
		params["type"] = "ws"
		params["path"] = v
	}
	if headers, ok := cfg["ws-headers"].(map[string]interface{}); ok {
		if host, ok := headers["Host"].(string); ok && host != "" {
			params["host"] = host
		}
	}
	if v, ok := cfg["grpc-service-name"].(string); ok && v != "" {
		params["type"] = "grpc"
		params["serviceName"] = v
	}
	if xhttp, ok := cfg["xhttp-config"].(map[string]interface{}); ok {
		params["type"] = "xhttp"
		if path, ok := xhttp["path"].(string); ok && path != "" {
			params["path"] = path
		}
	}
	return params
}

func shadowsocksURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	cipher, _ := cfg["cipher"].(string)
	password, _ := cfg["password"].(string)
	if cipher == "" || password == "" {
		return nil, fmt.Errorf("shadowsocks listener requires cipher and password for URI export")
	}
	encoded := base64.RawURLEncoding.EncodeToString([]byte(cipher + ":" + password))
	return []string{addName("ss://"+encoded+"@"+netutil.JoinHostPort(host, port), name)}, nil
}

func vlessURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	rows := userRows(cfg)
	if len(rows) == 0 {
		return nil, fmt.Errorf("vless listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		uuid, _ := row["uuid"].(string)
		if uuid == "" {
			return nil, fmt.Errorf("vless user uuid is required")
		}
		params := tlsParams(cfg)
		params["type"] = "tcp"
		if flow, _ := row["flow"].(string); flow != "" {
			params["flow"] = flow
		}
		if encryption, _ := cfg["encryption"].(string); encryption != "" {
			params["encryption"] = encryption
		} else {
			params["encryption"] = "none"
		}
		if pe, _ := cfg["packet-encoding"].(string); pe != "" {
			params["packetEncoding"] = pe
		}
		if reality, ok := cfg["reality-config"].(map[string]interface{}); ok {
			params["security"] = "reality"
			publicKey, err := realityPublicKey(reality)
			if err != nil {
				return nil, err
			}
			params["pbk"] = publicKey
			if sid := realityShortID(reality); sid != "" {
				params["sid"] = sid
			}
			if sni, ok := firstString(reality["server-names"]); ok {
				params["sni"] = sni
			}
			if params["fp"] == "" {
				params["fp"] = "chrome"
			}
		}
		for k, v := range transportParams(cfg) {
			params[k] = v
		}
		result = append(result, addName(query("vless://"+url.PathEscape(uuid)+"@"+netutil.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func vmessURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	rows := userRows(cfg)
	if len(rows) == 0 {
		return nil, fmt.Errorf("vmess listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		uuid, _ := row["uuid"].(string)
		if uuid == "" {
			return nil, fmt.Errorf("vmess user uuid is required")
		}
		aid := stringValue(cfg["alterId"], "0")
		cipher := stringValue(cfg["cipher"], "auto")
		obj := map[string]string{"v": "2", "ps": name, "add": host, "port": port, "id": uuid, "aid": aid, "scy": cipher, "net": "tcp", "type": "none"}
		if tls, ok := tlsParams(cfg)["security"]; ok && tls == "tls" {
			obj["tls"] = "tls"
		}
		if sni := tlsParams(cfg)["sni"]; sni != "" {
			obj["sni"] = sni
		}
		if fp := tlsParams(cfg)["fp"]; fp != "" {
			obj["fp"] = fp
		}
		if ws, ok := cfg["ws-path"].(string); ok && ws != "" {
			obj["net"] = "ws"
			obj["path"] = ws
		}
		if grpc, ok := cfg["grpc-service-name"].(string); ok && grpc != "" {
			obj["net"] = "grpc"
			obj["path"] = grpc
		}
		if reality, ok := cfg["reality-config"].(map[string]interface{}); ok {
			obj["tls"] = "reality"
			publicKey, err := realityPublicKey(reality)
			if err != nil {
				return nil, err
			}
			obj["pbk"] = publicKey
			if sid := realityShortID(reality); sid != "" {
				obj["sid"] = sid
			}
			if sni, ok := firstString(reality["server-names"]); ok {
				obj["sni"] = sni
			}
			if obj["fp"] == "" {
				obj["fp"] = "chrome"
			}
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		result = append(result, "vmess://"+base64.StdEncoding.EncodeToString(data))
	}
	return result, nil
}

func trojanURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	rows := userRows(cfg)
	if len(rows) == 0 {
		return nil, fmt.Errorf("trojan listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		password, _ := row["password"].(string)
		if password == "" {
			return nil, fmt.Errorf("trojan user password is required")
		}
		params := tlsParams(cfg)
		for k, v := range transportParams(cfg) {
			params[k] = v
		}
		if params["security"] == "" {
			if _, ok := cfg["certificate"].(string); ok {
				params["security"] = "tls"
			}
		}
		if params["type"] == "" {
			params["type"] = "tcp"
		}
		if reality, ok := cfg["reality-config"].(map[string]interface{}); ok {
			params["security"] = "reality"
			publicKey, err := realityPublicKey(reality)
			if err != nil {
				return nil, err
			}
			params["pbk"] = publicKey
			if sid := realityShortID(reality); sid != "" {
				params["sid"] = sid
			}
			if params["sni"] == "" {
				if sni, ok := firstString(reality["server-names"]); ok {
					params["sni"] = sni
				}
			}
			if params["fp"] == "" {
				params["fp"] = "chrome"
			}
		}
		result = append(result, addName(query("trojan://"+url.PathEscape(password)+"@"+netutil.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func hysteria2URIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	if rows := userRows(cfg); len(rows) > 0 {
		result := make([]string, 0, len(rows))
		for _, row := range rows {
			password, _ := row["password"].(string)
			if password == "" {
				continue
			}
			params := map[string]string{}
			if v, ok := cfg["sni"].(string); ok && v != "" {
				params["sni"] = v
			}
			if certutil.ShouldSkipCertVerify(cfg) {
				params["insecure"] = "1"
			}
			if v, ok := cfg["obfs"].(string); ok && v != "" {
				params["obfs"] = v
			}
			if v, ok := cfg["obfs-password"].(string); ok && v != "" {
				params["obfs-password"] = v
			}
			// up/down are bandwidth hints; include them when set so the panel
			// credential-row branch stays consistent with the config-embedded map
			// users branch below (P3-6).
			if v, ok := cfg["up"].(string); ok && v != "" {
				params["up"] = v
			}
			if v, ok := cfg["down"].(string); ok && v != "" {
				params["down"] = v
			}
			result = append(result, addName(query("hysteria2://"+url.PathEscape(password)+"@"+netutil.JoinHostPort(host, port), params), name))
		}
		if len(result) > 0 {
			return result, nil
		}
	}
	users := userMap(cfg)
	if len(users) == 0 {
		return nil, fmt.Errorf("hysteria2 listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(users))
	for username, raw := range users {
		password, ok := raw.(string)
		if !ok || password == "" {
			return nil, fmt.Errorf("hysteria2 user %q has empty password", username)
		}
		params := map[string]string{}
		if v, ok := cfg["sni"].(string); ok && v != "" {
			params["sni"] = v
		}
		if certutil.ShouldSkipCertVerify(cfg) {
			params["insecure"] = "1"
		}
		if v, ok := cfg["obfs"].(string); ok && v != "" {
			params["obfs"] = v
		}
		if v, ok := cfg["obfs-password"].(string); ok && v != "" {
			params["obfs-password"] = v
		}
		if v, ok := cfg["up"].(string); ok && v != "" {
			params["up"] = v
		}
		if v, ok := cfg["down"].(string); ok && v != "" {
			params["down"] = v
		}
		_ = username
		result = append(result, addName(query("hysteria2://"+url.PathEscape(password)+"@"+netutil.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func tuicURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	// token is an array of strings per tuic-v4 (Mihomo listener config).
	if tokens, ok := cfg["token"].([]interface{}); ok && len(tokens) > 0 {
		result := make([]string, 0, len(tokens))
		for _, t := range tokens {
			ts, ok := t.(string)
			if !ok || strings.TrimSpace(ts) == "" {
				continue
			}
			result = append(result, addName("tuic://"+url.PathEscape(ts)+"@"+netutil.JoinHostPort(host, port), name))
		}
		if len(result) > 0 {
			return result, nil
		}
	}
	// Support both map users and array rows {uuid,password} from panel credentials.
	if rows := userRows(cfg); len(rows) > 0 {
		result := make([]string, 0, len(rows))
		for _, row := range rows {
			uuid, _ := row["uuid"].(string)
			if uuid == "" {
				uuid, _ = row["username"].(string)
			}
			password, _ := row["password"].(string)
			if uuid == "" || password == "" {
				continue
			}
			params := map[string]string{}
			for key, out := range map[string]string{"congestion-controller": "congestion-controller", "bbr-profile": "bbr-profile", "udp-relay-mode": "udp-relay-mode"} {
				if v, ok := cfg[key].(string); ok && v != "" {
					params[out] = v
				}
			}
			if v, ok := firstString(cfg["alpn"]); ok {
				params["alpn"] = v
			}
			if v, ok := cfg["sni"].(string); ok && v != "" {
				params["sni"] = v
			}
			if certutil.ShouldSkipCertVerify(cfg) {
				params["allow_insecure"] = "1"
			}
			result = append(result, addName(query("tuic://"+url.PathEscape(uuid)+":"+url.PathEscape(password)+"@"+netutil.JoinHostPort(host, port), params), name))
		}
		if len(result) > 0 {
			return result, nil
		}
	}
	users := userMap(cfg)
	if len(users) == 0 {
		return nil, fmt.Errorf("tuic V5 listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(users))
	for uuid, raw := range users {
		password, ok := raw.(string)
		if !ok || password == "" {
			return nil, fmt.Errorf("tuic user %q has empty password", uuid)
		}
		params := map[string]string{}
		for key, out := range map[string]string{"congestion-controller": "congestion-controller", "bbr-profile": "bbr-profile", "udp-relay-mode": "udp-relay-mode", "max-udp-relay-packet-size": "max-udp-relay-packet-size"} {
			if v, ok := cfg[key].(string); ok && v != "" {
				params[out] = v
			}
		}
		if v, ok := firstString(cfg["alpn"]); ok {
			params["alpn"] = v
		}
		if v, ok := cfg["sni"].(string); ok && v != "" {
			params["sni"] = v
		}
		if certutil.ShouldSkipCertVerify(cfg) {
			params["allow_insecure"] = "1"
		}
		result = append(result, addName(query("tuic://"+url.PathEscape(uuid)+":"+url.PathEscape(password)+"@"+netutil.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func anytlsURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	users := userMap(cfg)
	if len(users) == 0 {
		return nil, fmt.Errorf("anytls listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(users))
	for username, raw := range users {
		password, ok := raw.(string)
		if !ok || password == "" {
			return nil, fmt.Errorf("anytls user %q has empty password", username)
		}
		params := map[string]string{}
		if v, ok := cfg["sni"].(string); ok && v != "" {
			params["sni"] = v
		}
		if v, ok := cfg["client-fingerprint"].(string); ok && v != "" {
			params["fp"] = v
		}
		if certutil.ShouldSkipCertVerify(cfg) {
			params["insecure"] = "1"
		}
		if v, ok := cfg["idle-session-check-interval"].(string); ok && v != "" {
			params["idle_session_check_interval"] = v
		}
		if v, ok := cfg["idle-session-timeout"].(string); ok && v != "" {
			params["idle_session_timeout"] = v
		}
		if v, ok := cfg["min-idle-session"].(string); ok && v != "" {
			params["min_idle_session"] = v
		}
		_ = username
		result = append(result, addName(query("anytls://"+url.PathEscape(password)+"@"+netutil.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func realityPublicKey(cfg map[string]interface{}) (string, error) {
	if public, ok := cfg["public-key"].(string); ok && strings.TrimSpace(public) != "" {
		return public, nil
	}
	private, ok := cfg["private-key"].(string)
	if !ok || strings.TrimSpace(private) == "" {
		return "", fmt.Errorf("reality listener URI export requires reality-config.public-key or private-key")
	}
	var raw []byte
	var err error
	for _, decode := range []func(string) ([]byte, error){base64.RawStdEncoding.DecodeString, base64.StdEncoding.DecodeString, base64.RawURLEncoding.DecodeString, base64.URLEncoding.DecodeString} {
		raw, err = decode(strings.TrimSpace(private))
		if err == nil && len(raw) == 32 {
			break
		}
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("invalid Reality private key: expected 32 decoded bytes")
	}
	public, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("failed to derive Reality public key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(public), nil
}

func realityShortID(reality map[string]interface{}) string {
	if s, ok := firstString(reality["short-id"]); ok {
		return s
	}
	return ""
}

func firstString(v interface{}) (string, bool) {
	if s, ok := v.(string); ok {
		return s, s != ""
	}
	if a, ok := v.([]interface{}); ok && len(a) > 0 {
		s, _ := a[0].(string)
		return s, s != ""
	}
	if a, ok := v.([]string); ok && len(a) > 0 {
		return a[0], a[0] != ""
	}
	return "", false
}

func stringValue(v interface{}, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}
