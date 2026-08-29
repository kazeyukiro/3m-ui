package converter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"golang.org/x/crypto/curve25519"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

func ResolveServerAddress(cfg *config.Config, req *http.Request) string {
	if envURL := os.Getenv("PUBLIC_URL"); envURL != "" {
		return cleanURLHost(envURL)
	}
	if cfg != nil && cfg.Server.PublicURL != "" {
		return cleanURLHost(cfg.Server.PublicURL)
	}
	if req != nil && req.Host != "" {
		return cleanURLHost(req.Host)
	}
	return "127.0.0.1"
}

// ResolveListenerServer prefers per-listener PublicHost (IPv4/IPv6/domain),
// then the global public URL / request host. Used for share links and client YAML.
func ResolveListenerServer(cfg *config.Config, req *http.Request, l models.Listener) string {
	if h := netutil.NormalizeHost(l.PublicHost); h != "" && !netutil.IsUnspecifiedBind(h) {
		return h
	}
	return ResolveServerAddress(cfg, req)
}

// ResolveListenerPort prefers PublicPort when set (NAT / dual-stack publish).
func ResolveListenerPort(l models.Listener) string {
	if p := strings.TrimSpace(l.PublicPort); p != "" {
		return p
	}
	return strings.TrimSpace(l.Port)
}

func cleanURLHost(raw string) string {
	u := netutil.NormalizeHost(raw)
	if u == "" || strings.ContainsAny(u, "\r\n/\\") {
		return "127.0.0.1"
	}
	return u
}

func GetSubscriptionURL(cfg *config.Config, req *http.Request, token string, target string) string {
	var base string
	if envURL := os.Getenv("PUBLIC_URL"); envURL != "" {
		base = strings.TrimSpace(envURL)
	} else if cfg != nil && cfg.Server.PublicURL != "" {
		base = strings.TrimSpace(cfg.Server.PublicURL)
	} else if req != nil && req.Host != "" {
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		} else if proto := req.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
			scheme = "https"
		}
		// req.Host already includes [ipv6]:port when applicable.
		base = fmt.Sprintf("%s://%s", scheme, req.Host)
	} else {
		base = "http://127.0.0.1:8080"
	}
	base = strings.TrimSuffix(base, "/")
	pathToken := url.PathEscape(token)
	if target == "" {
		return fmt.Sprintf("%s/api/v1/client/sub/%s", base, pathToken)
	}
	return fmt.Sprintf("%s/api/v1/client/sub/%s?target=%s", base, pathToken, url.QueryEscape(strings.ToLower(strings.TrimSpace(target))))
}

func GenerateRawConfig(db *gorm.DB, token models.AccessToken, req *http.Request) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if token.ListenerID == 0 {
		return nil, fmt.Errorf("access token is not bound to a listener")
	}
	var listener models.Listener
	if err := db.First(&listener, token.ListenerID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("listener not found")
		}
		return nil, fmt.Errorf("failed to fetch listener: %w", err)
	}
	if !listener.Enabled {
		return nil, fmt.Errorf("listener is disabled")
	}
	serverHost := ResolveListenerServer(config.GlobalConfig, req, listener)
	credentials := []user.Credential{}
	if byListener, err := user.NewService(db).ActiveCredentialsByListener(); err != nil {
		return nil, fmt.Errorf("failed to load listener credentials: %w", err)
	} else {
		credentials = byListener[listener.ID]
	}
	proxies, err := listenerToProxies(listener, serverHost, credentials)
	if err != nil {
		return nil, err
	}
	if len(proxies) == 0 {
		return nil, fmt.Errorf("listener %q has no exportable client credentials", listener.Name)
	}
	names := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		if name, ok := proxy["name"].(string); ok {
			names = append(names, name)
		}
	}
	cfg := map[string]interface{}{
		"proxies": proxies,
		"proxy-groups": []interface{}{
			map[string]interface{}{"name": "PROXY", "type": "select", "proxies": names},
		},
		"rules": []string{"MATCH,PROXY"},
	}
	return yaml.Marshal(cfg)
}

func listenerToProxies(l models.Listener, server string, credentials []user.Credential) ([]map[string]interface{}, error) {
	protocol := strings.ToLower(strings.TrimSpace(l.Protocol))
	if protocol == "" {
		protocol = strings.ToLower(strings.TrimSpace(l.Type))
	}
	if !config.IsMihomoListenerProtocol(protocol) {
		return nil, fmt.Errorf("unsupported listener protocol %q", protocol)
	}
	if protocol == "hysteria2-realm" {
		return nil, fmt.Errorf("hysteria2-realm is an auxiliary realm service and has no client proxy configuration")
	}
	opts, err := decodeOptions(l.Config)
	if err != nil {
		return nil, fmt.Errorf("invalid listener config for %q: %w", l.Name, err)
	}
	portStr := ResolveListenerPort(l)
	var portVal interface{} = portStr
	if p, err := strconv.Atoi(portStr); err == nil {
		portVal = p
	}
	server = netutil.NormalizeHost(server)
	base := map[string]interface{}{"type": protocol, "server": server, "port": portVal}
	if l.UDP && clientSupportsUDP(protocol) {
		base["udp"] = true
	}
	copyClientTLS(base, opts)
	copyTransport(base, opts)
	makeProxy := func(suffix string) map[string]interface{} {
		p := cloneMap(base)
		name := l.Name
		if suffix != "" {
			name += "-" + suffix
		}
		p["name"] = name
		return p
	}
	result := make([]map[string]interface{}, 0, max(1, len(credentials)))
	switch protocol {
	case "shadowsocks":
		p := makeProxy(credentialSuffix(credentials, 0))
		copyOption(p, opts, "cipher")
		if len(credentials) > 1 {
			return nil, fmt.Errorf("listener %q: Shadowsocks can export only one password", l.Name)
		}
		if len(credentials) == 1 && credentials[0].Password != "" {
			p["password"] = credentials[0].Password
		} else if value, ok := opts["password"]; ok {
			p["password"] = value
		}
		if value, ok := opts["simple-obfs"].(map[string]interface{}); ok && boolValue(value["enable"]) {
			p["plugin"] = "simple-obfs"
			p["plugin-opts"] = value
		}
		applyClientWrappers(p, opts)
		result = append(result, p)
	case "snell":
		p := makeProxy("")
		copyOption(p, opts, "psk")
		copyOption(p, opts, "version")
		copyOption(p, opts, "udp")
		if value, ok := opts["obfs-opts"]; ok {
			p["obfs-opts"] = value
		}
		result = append(result, p)
	case "vmess", "vless":
		if len(credentials) == 0 {
			return nil, fmt.Errorf("listener %q requires at least one active user for %s client export", l.Name, protocol)
		}
		for i, cred := range credentials {
			if cred.UUID == "" {
				continue
			}
			suffix := ""
			if len(credentials) > 1 {
				suffix = credentialSuffix(credentials, i)
				if suffix == "" {
					suffix = fmt.Sprintf("%d", i+1)
				}
			}
			p := makeProxy(suffix)
			p["uuid"] = cred.UUID
			copyOption(p, opts, "cipher")
			copyOption(p, opts, "packet-encoding")
			copyOption(p, opts, "global-padding")
			copyOption(p, opts, "authenticated-length")
			// encryption (client) and decryption (server) are a generated pair from
			// `mihomo generate vless-x25519` / `vless-mlkem768` — never copy decryption→encryption.
			copyOption(p, opts, "encryption")
			if flow := userFieldFromOpts(opts, cred.UUID, "flow"); flow != nil {
				p["flow"] = flow
			} else {
				copyOption(p, opts, "flow")
			}
			if flow, ok := p["flow"].(string); ok && strings.Contains(strings.ToLower(flow), "vision") {
				p["tls"] = true
			}
			if alterID := userFieldFromOpts(opts, cred.UUID, "alterId"); alterID != nil {
				p["alterId"] = alterID
			} else {
				copyOption(p, opts, "alterId")
			}
			applyClientWrappers(p, opts)
			result = append(result, p)
		}
	case "trojan":
		if len(credentials) == 0 {
			return nil, fmt.Errorf("listener %q requires at least one active user for Trojan client export", l.Name)
		}
		for i, cred := range credentials {
			p := makeProxy(fmt.Sprintf("%d", i+1))
			p["password"] = cred.Password
			for _, key := range []string{"sni", "alpn", "fingerprint", "client-fingerprint", "skip-cert-verify", "name-cert-verify"} {
				copyOption(p, opts, key)
			}
			applyClientWrappers(p, opts)
			if value, ok := opts["ss-option"]; ok {
				p["ss-opts"] = value
			}
			result = append(result, p)
		}
	case "hysteria2":
		if len(credentials) == 0 {
			return nil, fmt.Errorf("listener %q requires at least one active user for Hysteria 2 client export", l.Name)
		}
		for i, cred := range credentials {
			p := makeProxy(fmt.Sprintf("%d", i+1))
			p["password"] = cred.Password
			for _, key := range []string{
				"up", "down", "obfs", "obfs-password", "masquerade", "bbr-profile",
				"realm-opts", "alpn", "sni", "servername", "skip-cert-verify", "name-cert-verify",
				"fingerprint", "ca", "ca-str",
			} {
				copyOption(p, opts, key)
			}
			if p["alpn"] == nil {
				p["alpn"] = []string{"h3"}
			}
			if p["sni"] == nil {
				if sn, ok := p["servername"].(string); ok && sn != "" {
					p["sni"] = sn
				}
			}
			if value, ok := opts["ech-opts"]; ok {
				p["ech-opts"] = value
			}
			result = append(result, p)
		}
	case "tuic":
		if token, ok := opts["token"]; ok {
			p := makeProxy("")
			p["token"] = token
			for _, key := range []string{"congestion-controller", "bbr-profile", "max-idle-time", "authentication-timeout", "alpn", "max-udp-relay-packet-size"} {
				copyOption(p, opts, key)
			}
			result = append(result, p)
		} else {
			if len(credentials) == 0 {
				return nil, fmt.Errorf("listener %q requires TUIC V5 users or a V4 token", l.Name)
			}
			for i, cred := range credentials {
				p := makeProxy(fmt.Sprintf("%d", i+1))
				p["uuid"] = cred.UUID
				p["password"] = cred.Password
				for _, key := range []string{"congestion-controller", "bbr-profile", "max-idle-time", "authentication-timeout", "alpn", "max-udp-relay-packet-size"} {
					copyOption(p, opts, key)
				}
				result = append(result, p)
			}
		}
	case "shadowquic":
		if len(credentials) == 0 {
			return nil, fmt.Errorf("listener %q requires at least one active user for ShadowQUIC client export", l.Name)
		}
		for i, cred := range credentials {
			p := makeProxy(fmt.Sprintf("%d", i+1))
			p["username"] = cred.Username
			p["password"] = cred.Password
			for _, key := range []string{
				"sni", "alpn", "quic-versions", "zero-rtt", "udp-over-stream",
				"keep-alive-interval", "congestion-controller", "up", "down", "cwnd",
				"bbr-profile", "max-datagram-frame-size", "max-open-streams",
				"recv-window-conn", "recv-window", "disable-mtu-discovery",
			} {
				copyOption(p, opts, key)
			}
			if p["sni"] == nil {
				if ju, ok := opts["jls-upstream"].(map[string]interface{}); ok {
					if sni, ok := ju["sni"].(string); ok && strings.TrimSpace(sni) != "" {
						p["sni"] = sni
					} else if addr, ok := ju["addr"].(string); ok && strings.TrimSpace(addr) != "" {
						host := addr
						if h, _, err := net.SplitHostPort(addr); err == nil {
							host = h
						}
						p["sni"] = host
					}
				}
			}
			if p["udp"] == nil {
				p["udp"] = true
			}
			if p["alpn"] == nil {
				p["alpn"] = []string{"h3"}
			}
			if p["quic-versions"] == nil {
				p["quic-versions"] = []string{"v2"}
			}
			if p["congestion-controller"] == nil {
				p["congestion-controller"] = "bbr"
			}
			if p["zero-rtt"] == nil {
				p["zero-rtt"] = true
			}
			result = append(result, p)
		}
	case "anytls":
		if len(credentials) == 0 {
			return nil, fmt.Errorf("listener %q requires at least one active user for AnyTLS client export", l.Name)
		}
		for i, cred := range credentials {
			p := makeProxy(fmt.Sprintf("%d", i+1))
			p["password"] = cred.Password
			for _, key := range []string{
				"client-fingerprint", "udp", "idle-session-check-interval",
				"idle-session-timeout", "min-idle-session", "sni", "alpn",
				"skip-cert-verify", "name-cert-verify",
			} {
				copyOption(p, opts, key)
			}
			applyClientWrappers(p, opts)
			result = append(result, p)
		}
	case "mieru":
		if len(credentials) == 0 {
			return nil, fmt.Errorf("listener %q requires at least one active user for Mieru client export", l.Name)
		}
		for i, cred := range credentials {
			p := makeProxy(fmt.Sprintf("%d", i+1))
			p["username"] = cred.Username
			p["password"] = cred.Password
			for _, key := range []string{"transport", "multiplexing", "handshake-mode", "traffic-pattern"} {
				copyOption(p, opts, key)
			}
			result = append(result, p)
		}
	case "sudoku":
		p := makeProxy("")
		for _, key := range []string{
			"key", "aead-method", "padding-min", "padding-max", "table-type",
			"custom-table", "custom-tables", "handshake-timeout",
			"enable-pure-downlink", "httpmask",
		} {
			copyOption(p, opts, key)
		}
		result = append(result, p)
	case "trusttunnel":
		if len(credentials) == 0 {
			return nil, fmt.Errorf("listener %q requires at least one active user for TrustTunnel client export", l.Name)
		}
		for i, cred := range credentials {
			p := makeProxy(fmt.Sprintf("%d", i+1))
			p["username"] = cred.Username
			p["password"] = cred.Password
			for _, key := range []string{
				"client-fingerprint", "health-check", "udp", "sni", "alpn",
				"skip-cert-verify", "name-cert-verify", "quic", "congestion-controller",
				"bbr-profile", "max-connections", "min-streams", "max-streams",
			} {
				copyOption(p, opts, key)
			}
			result = append(result, p)
		}
	}
	return result, nil
}

func applyClientWrappers(p map[string]interface{}, opts map[string]interface{}) {
	if value := realityClientOptions(opts); value != nil {
		p["reality-opts"] = value
		if p["client-fingerprint"] == nil {
			p["client-fingerprint"] = "chrome"
		}
		if p["tls"] == nil {
			p["tls"] = true
		}
		if p["udp"] == nil {
			p["udp"] = true
		}
		if p["alpn"] == nil {
			p["alpn"] = []string{"h2", "http/1.1"}
		}
		if p["network"] == nil {
			p["network"] = "tcp"
		}
	}
	if value := shadowTLSClientOptions(opts); value != nil {
		p["shadow-tls-opts"] = value
	}
	if value := resTLSClientOptions(opts); value != nil {
		p["restls-opts"] = value
	}
	if value := jlsClientOptions(opts); value != nil {
		p["jls-opts"] = value
	}
}

func decodeOptions(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	var options map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &options); err != nil {
		return nil, err
	}
	if options == nil {
		options = map[string]interface{}{}
	}
	return options, nil
}

func copyClientTLS(dst, src map[string]interface{}) {
	if _, ok := src["certificate"]; ok {
		dst["tls"] = true
	}
	if _, ok := src["reality-config"]; ok {
		dst["tls"] = true
	}
	if enabledWrapper(src, "shadow-tls") || enabledWrapper(src, "res-tls") || enabledWrapper(src, "jls-config") {
		dst["tls"] = true
	}
	for _, key := range []string{"sni", "servername", "alpn", "fingerprint", "client-fingerprint", "skip-cert-verify", "name-cert-verify"} {
		copyOption(dst, src, key)
	}
	if dst["sni"] == nil && dst["servername"] == nil {
		if reality, ok := src["reality-config"].(map[string]interface{}); ok {
			if sni, ok := firstStringValue(reality["server-names"]); ok {
				dst["servername"] = sni
			}
		}
		if dst["servername"] == nil {
			if jls, ok := src["jls-config"].(map[string]interface{}); ok {
				if sni, ok := jls["sni"].(string); ok && sni != "" {
					dst["servername"] = sni
				} else if dest, ok := jls["dest"].(string); ok && dest != "" {
					host := dest
					if h, _, err := net.SplitHostPort(dest); err == nil {
						host = h
					}
					dst["servername"] = host
				}
			}
		}
	}
	if dst["servername"] == nil {
		if sni, ok := dst["sni"].(string); ok && sni != "" {
			dst["servername"] = sni
		}
	}
}

func enabledWrapper(src map[string]interface{}, key string) bool {
	raw, ok := src[key]
	if !ok || raw == nil {
		return false
	}
	if m, ok := raw.(map[string]interface{}); ok {
		if en, ok := m["enable"].(bool); ok {
			return en
		}
		return len(m) > 0
	}
	return false
}

func copyTransport(dst, src map[string]interface{}) {
	if path, ok := src["ws-path"].(string); ok && strings.TrimSpace(path) != "" {
		dst["network"] = "ws"
		wsOpts := map[string]interface{}{"path": path}
		if headers, ok := src["ws-headers"].(map[string]interface{}); ok && len(headers) > 0 {
			wsOpts["headers"] = headers
		}
		dst["ws-opts"] = wsOpts
	}
	if service, ok := src["grpc-service-name"].(string); ok && strings.TrimSpace(service) != "" {
		dst["network"] = "grpc"
		dst["grpc-opts"] = map[string]interface{}{"grpc-service-name": service}
	}
	if value, ok := src["xhttp-config"]; ok && value != nil {
		dst["network"] = "xhttp"
		dst["xhttp-opts"] = value
	}
}

func realityClientOptions(src map[string]interface{}) map[string]interface{} {
	cfg, ok := src["reality-config"].(map[string]interface{})
	if !ok {
		return nil
	}
	publicKey, err := deriveRealityPublicKey(cfg)
	if err != nil || publicKey == "" {
		return nil
	}
	result := map[string]interface{}{"public-key": publicKey}
	if ids, ok := cfg["short-id"].([]interface{}); ok && len(ids) > 0 {
		result["short-id"] = ids[0]
	} else if value, ok := cfg["short-id"]; ok {
		result["short-id"] = value
	}
	if v, ok := cfg["support-x25519mlkem768"]; ok {
		result["support-x25519mlkem768"] = v
	} else {
		result["support-x25519mlkem768"] = true
	}
	return result
}

func deriveRealityPublicKey(cfg map[string]interface{}) (string, error) {
	if public, ok := cfg["public-key"].(string); ok && strings.TrimSpace(public) != "" {
		return strings.TrimSpace(public), nil
	}
	private, ok := cfg["private-key"].(string)
	if !ok || strings.TrimSpace(private) == "" {
		return "", fmt.Errorf("reality-config missing public-key and private-key")
	}
	var raw []byte
	var err error
	for _, decode := range []func(string) ([]byte, error){
		base64.RawStdEncoding.DecodeString, base64.StdEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString, base64.URLEncoding.DecodeString,
	} {
		raw, err = decode(strings.TrimSpace(private))
		if err == nil && len(raw) == 32 {
			break
		}
		raw = nil
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("invalid Reality private key")
	}
	public, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(public), nil
}

func firstStringValue(v interface{}) (string, bool) {
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

func shadowTLSClientOptions(src map[string]interface{}) map[string]interface{} {
	cfg, ok := src["shadow-tls"].(map[string]interface{})
	if !ok {
		return nil
	}
	result := map[string]interface{}{}
	for _, key := range []string{"version", "password"} {
		if value, ok := cfg[key]; ok {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func resTLSClientOptions(src map[string]interface{}) map[string]interface{} {
	cfg, ok := src["res-tls"].(map[string]interface{})
	if !ok {
		return nil
	}
	result := map[string]interface{}{}
	if value, ok := cfg["password"]; ok {
		result["password"] = value
	}
	if value, ok := cfg["version-hint"]; ok {
		result["version-hint"] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func jlsClientOptions(src map[string]interface{}) map[string]interface{} {
	cfg, ok := src["jls-config"].(map[string]interface{})
	if !ok {
		return nil
	}
	result := map[string]interface{}{}
	if users, ok := cfg["users"].([]interface{}); ok && len(users) > 0 {
		if first, ok := users[0].(map[string]interface{}); ok {
			if value, ok := first["username"]; ok {
				result["username"] = value
			}
			if value, ok := first["password"]; ok {
				result["password"] = value
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func copyOption(dst, src map[string]interface{}, key string) {
	if value, ok := src[key]; ok {
		dst[key] = value
	}
}

func boolValue(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func clientSupportsUDP(protocol string) bool {
	switch protocol {
	case "shadowsocks", "snell", "vmess", "vless", "trojan", "anytls", "trusttunnel":
		return true
	default:
		return false
	}
}

func credentialSuffix(credentials []user.Credential, index int) string {
	if index >= 0 && index < len(credentials) && credentials[index].Username != "" {
		return credentials[index].Username
	}
	return ""
}

func userFieldFromOpts(opts map[string]interface{}, uuid, field string) interface{} {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return nil
	}
	list, ok := opts["users"].([]interface{})
	if !ok {
		return nil
	}
	for _, item := range list {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(row["uuid"])) != uuid {
			continue
		}
		if value, exists := row[field]; exists && value != nil && fmt.Sprint(value) != "" {
			return value
		}
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
