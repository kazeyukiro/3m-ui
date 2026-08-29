package converter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"golang.org/x/crypto/curve25519"
)

// listenerToProxies converts a server Listener into Mihomo client proxy maps.
// Restored core paths for CI; extended wrappers live alongside when present.
func listenerToProxies(l models.Listener, server string, credentials []user.Credential) ([]map[string]interface{}, error) {
	opts, err := decodeOptions(l.Config)
	if err != nil {
		return nil, err
	}
	proto := strings.ToLower(strings.TrimSpace(l.Protocol))
	if proto == "" {
		proto = strings.ToLower(strings.TrimSpace(l.Type))
	}
	host := netutil.NormalizeHost(server)
	if host == "" {
		return nil, fmt.Errorf("server host is required")
	}
	port := ResolveListenerPort(l)
	if port == "" {
		port = strings.TrimSpace(l.Port)
	}
	portNum, _ := strconv.Atoi(port)

	var out []map[string]interface{}
	build := func(name string, base map[string]interface{}) {
		p := map[string]interface{}{"name": name, "type": proto, "server": host, "port": portNum, "udp": true}
		for k, v := range base {
			p[k] = v
		}
		applyClientWrappers(p, opts)
		out = append(out, p)
	}

	switch proto {
	case "vless":
		uuids := credentialUUIDs(credentials, opts)
		if len(uuids) == 0 {
			return nil, fmt.Errorf("vless requires uuid")
		}
		for i, uid := range uuids {
			name := l.Name
			if len(uuids) > 1 {
				name = fmt.Sprintf("%s-%d", l.Name, i+1)
			}
			base := map[string]interface{}{"uuid": uid, "encryption": "none"}
			if flow, _ := opts["flow"].(string); flow != "" {
				base["flow"] = flow
			}
			copyClientTLS(base, opts)
			build(name, base)
		}
	case "vmess":
		uuids := credentialUUIDs(credentials, opts)
		if len(uuids) == 0 {
			return nil, fmt.Errorf("vmess requires uuid")
		}
		for i, uid := range uuids {
			name := l.Name
			if len(uuids) > 1 {
				name = fmt.Sprintf("%s-%d", l.Name, i+1)
			}
			base := map[string]interface{}{"uuid": uid, "alterId": 0, "cipher": "auto"}
			copyClientTLS(base, opts)
			build(name, base)
		}
	case "trojan":
		passes := credentialPasswords(credentials, opts)
		if len(passes) == 0 {
			return nil, fmt.Errorf("trojan requires password")
		}
		for i, pass := range passes {
			name := l.Name
			if len(passes) > 1 {
				name = fmt.Sprintf("%s-%d", l.Name, i+1)
			}
			base := map[string]interface{}{"password": pass}
			copyClientTLS(base, opts)
			build(name, base)
		}
	case "shadowsocks":
		cipher, _ := opts["cipher"].(string)
		pass, _ := opts["password"].(string)
		if pass == "" && len(credentials) > 0 {
			pass = credentials[0].Password
		}
		if cipher == "" || pass == "" {
			return nil, fmt.Errorf("shadowsocks requires cipher and password")
		}
		build(l.Name, map[string]interface{}{"cipher": cipher, "password": pass})
	case "hysteria2":
		passes := credentialPasswords(credentials, opts)
		if len(passes) == 0 {
			return nil, fmt.Errorf("hysteria2 requires password")
		}
		for i, pass := range passes {
			name := l.Name
			if len(passes) > 1 {
				name = fmt.Sprintf("%s-%d", l.Name, i+1)
			}
			base := map[string]interface{}{"password": pass}
			if sni, _ := opts["sni"].(string); sni != "" {
				base["sni"] = sni
			}
			if scv, ok := opts["skip-cert-verify"].(bool); ok {
				base["skip-cert-verify"] = scv
			}
			build(name, base)
		}
	default:
		return nil, fmt.Errorf("unsupported client export protocol %q", proto)
	}
	return out, nil
}

func decodeOptions(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("invalid listener config: %w", err)
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m, nil
}

func credentialUUIDs(credentials []user.Credential, opts map[string]interface{}) []string {
	var out []string
	for _, c := range credentials {
		if c.UUID != "" {
			out = append(out, c.UUID)
		}
	}
	if len(out) > 0 {
		return out
	}
	if users, ok := opts["users"].([]interface{}); ok {
		for _, u := range users {
			if m, ok := u.(map[string]interface{}); ok {
				if uid, _ := m["uuid"].(string); uid != "" {
					out = append(out, uid)
				}
			}
		}
	}
	return out
}

func credentialPasswords(credentials []user.Credential, opts map[string]interface{}) []string {
	var out []string
	for _, c := range credentials {
		if c.Password != "" {
			out = append(out, c.Password)
		}
	}
	if len(out) > 0 {
		return out
	}
	if pass, _ := opts["password"].(string); pass != "" {
		return []string{pass}
	}
	if users, ok := opts["users"].([]interface{}); ok {
		for _, u := range users {
			if m, ok := u.(map[string]interface{}); ok {
				if p, _ := m["password"].(string); p != "" {
					out = append(out, p)
				}
			}
		}
	}
	return out
}

func copyClientTLS(dst, src map[string]interface{}) {
	if rc, ok := src["reality-config"].(map[string]interface{}); ok && rc != nil {
		dst["tls"] = true
		dst["reality-opts"] = map[string]interface{}{}
		if pbk, _ := rc["public-key"].(string); pbk != "" {
			dst["reality-opts"].(map[string]interface{})["public-key"] = pbk
		} else if priv, _ := rc["private-key"].(string); priv != "" {
			if pub, err := deriveX25519Public(priv); err == nil {
				dst["reality-opts"].(map[string]interface{})["public-key"] = pub
			}
		}
		if sid := firstShortID(rc["short-id"]); sid != "" {
			dst["reality-opts"].(map[string]interface{})["short-id"] = sid
		}
		if names, ok := rc["server-names"].([]interface{}); ok && len(names) > 0 {
			if s, ok := names[0].(string); ok {
				dst["servername"] = s
			}
		}
		dst["client-fingerprint"] = "chrome"
		return
	}
	if _, ok := src["certificate"]; ok {
		dst["tls"] = true
	}
}

func firstShortID(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func deriveX25519Public(private string) (string, error) {
	var raw []byte
	var err error
	for _, decode := range []func(string) ([]byte, error){
		base64.RawURLEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.StdEncoding.DecodeString,
	} {
		raw, err = decode(strings.TrimSpace(private))
		if err == nil && len(raw) == 32 {
			break
		}
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("invalid private key")
	}
	pub, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(pub), nil
}

func applyClientWrappers(p map[string]interface{}, opts map[string]interface{}) {
	// Transport hints from server listener config.
	if path, _ := opts["ws-path"].(string); path != "" {
		p["network"] = "ws"
		p["ws-opts"] = map[string]interface{}{"path": path}
	}
	if svc, _ := opts["grpc-service-name"].(string); svc != "" {
		p["network"] = "grpc"
		p["grpc-opts"] = map[string]interface{}{"grpc-service-name": svc}
	}
}

// silence unused imports when JSON only used in decodeOptions
var _ = json.Marshal
