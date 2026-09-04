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

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
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
	// Panel protocol names → Mihomo proxy type names (proxies/*.md).
	exportProtocol := protocol
	switch protocol {
	case "tuic-v4", "tuic-v5":
		exportProtocol = "tuic"
	case "shadowsocks":
		// Official client type is "ss", not "shadowsocks" (listener type).
		exportProtocol = "ss"
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
	base := map[string]interface{}{"type": exportProtocol, "server": server, "port": portVal}
	if l.UDP && clientSupportsUDP(protocol) {
		base["udp"] = true
	}
	// SS has no top-level tls/sni/servername/alpn fields — its TLS-like
	// wrappers (shadow-tls/restls/jls-config) are emitted as the `plugin`
	// format via applySSPluginWrappers, not via copyClientTLS.
	// ShadowQUIC uses QUIC's built-in TLS — no certificate/tls/skip-cert-verify.
	if protocol != "shadowsocks" && protocol != "shadowquic" && protocol != "mieru" {
		copyClientTLS(base, opts)
	}
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
			// Listener uses simple-obfs; client proxies/ss.md uses plugin: obfs.
			// Do not pass enable:true — client plugin-opts only wants mode/host.
			p["plugin"] = "obfs"
			optsCopy := map[string]interface{}{}
			if mode, ok := value["mode"]; ok {
				optsCopy["mode"] = mode
			}
			if host, ok := value["host"]; ok {
				optsCopy["host"] = host
			}
			p["plugin-opts"] = optsCopy
		}
		applySSPluginWrappers(p, opts)
		// Optional SS client fields documented on proxies-ss wiki
		// block 0 (udp-over-tcp / udp-over-tcp-version / ip-version /
		// smux). Forward as-is when present in the listener config JSON
		// so the panel can surface them on the outbound SS proxy.
		if v, ok := opts["udp-over-tcp"].(bool); ok && v {
			p["udp-over-tcp"] = v
		}
		if v, ok := opts["udp-over-tcp-version"].(string); ok && v != "" {
			p["udp-over-tcp-version"] = v
		}
		if v, ok := opts["ip-version"].(string); ok && v != "" {
			p["ip-version"] = v
		}
		if v, ok := opts["smux"]; ok {
			p["smux"] = v
		}
		result = append(result, p)
	case "snell":
		p := makeProxy("")
		copyOption(p, opts, "psk")
		copyOption(p, opts, "version")
		copyOption(p, opts, "udp")
		if value, ok := opts["obfs-opts"]; ok {
			p["obfs-opts"] = value
		}
		// Translate TLS-like listener wrappers (shadow-tls/res-tls/jls-config)
		// into the snell `obfs-opts` format documented in proxies/snell.md.
		// Skipped when an explicit `obfs-opts` block is already present.
		applySnellObfsOpts(p, opts)
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
			// VLESS has an `encryption` field (xtls-rprx-vision etc.); VMess does not.
			if protocol == "vless" {
				copyOption(p, opts, "encryption")
			}
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
			// SNI fallback: listener config strips "sni" (panel-only hint).
			// If no SNI was set, use the server host as SNI so the client
			// knows which hostname to verify in the TLS certificate.
			if p["sni"] == nil && p["servername"] == nil {
				p["sni"] = server
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
				"up", "down", "obfs", "obfs-password", "bbr-profile",
				"realm-opts", "alpn", "sni", "skip-cert-verify", "name-cert-verify",
				"fingerprint", "handshake-timeout",
			} {
				copyOption(p, opts, key)
			}
			// Hysteria2 wiki-documented optional fields (proxies-hysteria2
			// wiki block 0): port-hopping + obfs packet-size. Match m-ui
			// path (mui/protocol/hysteria2.go BuildShare + yaml:",omitempty"
			// tags) so unset listeners don't pollute client YAML with
			// zero-valued placeholders:
			//   ports                  (string e.g. "443-8443"; non-empty required)
			//   hop-interval           (int seconds, default 30 — emit only if > 0)
			//   obfs-min-packet-size   (int, gecko-only — emit only if > 0)
			//   obfs-max-packet-size   (int, gecko-only — emit only if > 0)
			if v, ok := opts["ports"].(string); ok && v != "" {
				p["ports"] = v
			}
			for _, k := range []string{"hop-interval", "obfs-min-packet-size", "obfs-max-packet-size"} {
				if v, ok := opts[k]; ok {
					switch n := v.(type) {
					case float64:
						if n > 0 {
							p[k] = v
						}
					case int:
						if n > 0 {
							p[k] = v
						}
					case int64:
						if n > 0 {
							p[k] = v
						}
					}
				}
			}
			// SNI fallback: use server host when not set.
			if p["sni"] == nil && p["servername"] == nil {
				p["sni"] = server
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
	case "tuic", "tuic-v4", "tuic-v5":
		// TUIC client YAML (proxies-tuic wiki) supports the optional
		// `ip` (UDP NAT source IP hint) and `name-cert-verify`
		// (cert hostname to verify) fields. Only emit them when the
		// listener carries a non-empty value, so we don't pollute the
		// client YAML with `ip: ""` placeholders.
		if token, ok := opts["token"]; ok {
			p := makeProxy("")
			p["token"] = normalizeTUICToken(token)
			for _, key := range []string{"congestion-controller", "bbr-profile", "alpn", "max-udp-relay-packet-size", "sni", "skip-cert-verify", "udp-relay-mode", "reduce-rtt", "request-timeout", "heartbeat-interval", "fast-open", "max-open-streams", "disable-sni"} {
				copyOption(p, opts, key)
			}
			if v, ok := opts["ip"].(string); ok && v != "" {
				p["ip"] = v
			}
			if v, ok := opts["name-cert-verify"].(string); ok && v != "" {
				p["name-cert-verify"] = v
			}
			ensureTUICClientDefaults(p, opts)
			result = append(result, p)
		} else {
			if len(credentials) == 0 {
				return nil, fmt.Errorf("listener %q requires TUIC V5 users or a V4 token", l.Name)
			}
			for i, cred := range credentials {
				p := makeProxy(fmt.Sprintf("%d", i+1))
				// Server users map is keyed by UUID (fallback username). Client must match.
				uuid := strings.TrimSpace(cred.UUID)
				if uuid == "" {
					uuid = strings.TrimSpace(cred.Username)
				}
				if uuid == "" {
					return nil, fmt.Errorf("listener %q: TUIC V5 user is missing uuid", l.Name)
				}
				p["uuid"] = uuid
				p["password"] = cred.Password
				for _, key := range []string{"congestion-controller", "bbr-profile", "alpn", "max-udp-relay-packet-size", "sni", "skip-cert-verify", "udp-relay-mode", "reduce-rtt", "request-timeout", "heartbeat-interval", "fast-open", "max-open-streams", "disable-sni"} {
					copyOption(p, opts, key)
				}
				if v, ok := opts["ip"].(string); ok && v != "" {
					p["ip"] = v
				}
				if v, ok := opts["name-cert-verify"].(string); ok && v != "" {
					p["name-cert-verify"] = v
				}
				ensureTUICClientDefaults(p, opts)
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
				// Final fallback: use server host as SNI.
				if p["sni"] == nil {
					p["sni"] = server
				}
			}
			if p["udp"] == nil {
				p["udp"] = true
			}
			if p["alpn"] == nil {
				p["alpn"] = []string{"h3"}
			}
			// quic-versions / congestion-controller / zero-rtt are NOT defaulted
			// here: the official docs (proxies/shadowquic.md) specify defaults of
			// v1 / cubic / false, which mihomo applies automatically when these
			// fields are absent. Inventing conflicting defaults (v2/bbr/true)
			// would override the listener's intent and break connectivity with
			// servers that don't support them. They pass through via copyOption
			// above only when present in the listener config.
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
	// tlsmirror-opts: forward the listener-side `tlsmirror-config` block
	// verbatim to the client-side `tlsmirror-opts` shape per proxies-tls
	// wiki block 0. The listener and client structures are identical
	// (primary-key + explicit-nonce-ciphersuites + transport-layer-padding
	// + connection-enrolment + sequence-watermarking-enabled +
	// embedded-traffic-generator). Wiki note: VMess is currently the only
	// protocol that supports tlsmirror on the client side; the converter
	// still forwards the block for VMess/VLESS/Trojan uniformly so a
	// listener that later migrates its transport does not silently lose
	// the operator-supplied tlsmirror config. Matches m-ui path's
	// decodeTLSMirrorOpts helper (mui/bridge.go).
	if value := tlsmirrorClientOptions(opts); value != nil {
		p["tlsmirror-opts"] = value
	}
}

// applySSPluginWrappers translates the SS listener wrapper blocks
// (shadow-tls/res-tls/jls-config) into the SS `plugin` format documented in
// proxies/ss.md. Unlike VMess/VLESS/Trojan which use `shadow-tls-opts` /
// `restls-opts` / `jls-opts`, SS carries TLS-like wrappers as a plugin:
//
//	plugin: shadow-tls
//	plugin-opts:
//	  host: example.com
//	  password: xxx
//	  version: 3
//	client-fingerprint: chrome
//
// Only the first matching enabled wrapper is emitted (they are mutually
// exclusive on a single SS listener).
func applySSPluginWrappers(p map[string]interface{}, src map[string]interface{}) {
	// kcptun (kcp-tun) is a UDP-over-TCP plugin documented on proxies-ss
	// wiki block 6. It is mutually exclusive with the TLS-like wrappers
	// below (the SS listener schema whitelists `kcp-tun` alongside
	// `shadow-tls` / `res-tls` / `jls-config`, but only one `plugin` value
	// is meaningful per SS outbound). Precedence matches the m-ui SS
	// module: kcptun is preferred over the TLS wrappers when both happen
	// to be enabled.
	if kcp, ok := src["kcp-tun"].(map[string]interface{}); ok {
		if enable, _ := kcp["enable"].(bool); enable {
			p["plugin"] = "kcptun"
			opts := map[string]interface{}{}
			// Plugin-opts field names mirror the listener kcp-tun keys
			// verbatim — proxies-ss wiki block 6 documents the exact
			// same names (key/crypt/mode/conn/autoexpire/...).
			for _, key := range []string{
				"key", "crypt", "mode", "conn", "autoexpire", "scavengettl",
				"ratelimit", "mtu", "sndwnd", "rcvwnd", "datashard", "parityshard",
				"dscp", "nocomp", "acknodelay", "nodelay", "interval", "resend",
				"sockbuf", "smuxver", "smuxbuf", "framesize", "streambuf", "keepalive",
			} {
				if v, ok := kcp[key]; ok {
					opts[key] = v
				}
			}
			p["plugin-opts"] = opts
			return
		}
	}
	if stls, ok := src["shadow-tls"].(map[string]interface{}); ok {
		if enable, _ := stls["enable"].(bool); enable {
			p["plugin"] = "shadow-tls"
			opts := map[string]interface{}{}
			if v, ok := stls["version"]; ok {
				opts["version"] = v
			}
			// Password: v2 uses the top-level password; v3 uses users[0].password.
			if pwd, ok := stls["password"].(string); ok && pwd != "" {
				opts["password"] = pwd
			} else if users, ok := stls["users"].([]interface{}); ok && len(users) > 0 {
				if u, ok := users[0].(map[string]interface{}); ok {
					if pwd, ok := u["password"].(string); ok {
						opts["password"] = pwd
					}
				}
			}
			// host from handshake.dest (strip :port).
			if hs, ok := stls["handshake"].(map[string]interface{}); ok {
				if dest, ok := hs["dest"].(string); ok {
					opts["host"] = stripHostPort(dest)
				}
			}
			p["plugin-opts"] = opts
			p["client-fingerprint"] = ssClientFingerprint(src)
			return
		}
	}
	if restls, ok := src["res-tls"].(map[string]interface{}); ok {
		if enable, _ := restls["enable"].(bool); enable {
			p["plugin"] = "restls"
			opts := map[string]interface{}{}
			if dest, ok := restls["dest"].(string); ok {
				opts["host"] = stripHostPort(dest)
			}
			if pwd, ok := restls["password"].(string); ok {
				opts["password"] = pwd
			}
			// version-hint is not in the listener config; default to tls13
			// (matches the m-ui SS module and the restls proxy doc example).
			opts["version-hint"] = "tls13"
			if script, ok := restls["restls-script"].(string); ok && script != "" {
				opts["restls-script"] = script
			}
			p["plugin-opts"] = opts
			p["client-fingerprint"] = ssClientFingerprint(src)
			return
		}
	}
	if jls, ok := src["jls-config"].(map[string]interface{}); ok {
		if enable, _ := jls["enable"].(bool); enable {
			p["plugin"] = "jls"
			opts := map[string]interface{}{}
			if dest, ok := jls["dest"].(string); ok {
				opts["host"] = stripHostPort(dest)
			}
			// JLS users: use first user's username+password (single-user case).
			if users, ok := jls["users"].([]interface{}); ok && len(users) > 0 {
				if u, ok := users[0].(map[string]interface{}); ok {
					if un, ok := u["username"].(string); ok {
						opts["username"] = un
					}
					if pwd, ok := u["password"].(string); ok {
						opts["password"] = pwd
					}
				}
			}
			if alpn, ok := jls["alpn"]; ok {
				opts["alpn"] = alpn
			}
			p["plugin-opts"] = opts
			p["client-fingerprint"] = ssClientFingerprint(src)
			return
		}
	}
}

// ssClientFingerprint returns the listener's client-fingerprint if set,
// otherwise defaults to "chrome" (the documented default for SS plugins).
func ssClientFingerprint(src map[string]interface{}) string {
	if cf, ok := src["client-fingerprint"].(string); ok && strings.TrimSpace(cf) != "" {
		return strings.TrimSpace(cf)
	}
	return "chrome"
}

// applySnellObfsOpts translates a Snell listener's TLS-like wrapper blocks
// (shadow-tls/res-tls/jls-config) into the snell `obfs-opts` format documented
// in proxies/snell.md. Snell carries these wrappers as obfs-opts (with a
// `mode` field) rather than the `plugin`/`plugin-opts` form used by SS.
//
// Only the first matching enabled wrapper is emitted (they are mutually
// exclusive on a single Snell listener). When the listener config already
// provides an explicit `obfs-opts` block it is left untouched.
func applySnellObfsOpts(p map[string]interface{}, src map[string]interface{}) {
	if _, ok := p["obfs-opts"]; ok {
		return
	}
	if stls, ok := src["shadow-tls"].(map[string]interface{}); ok {
		if enable, _ := stls["enable"].(bool); enable {
			opts := map[string]interface{}{"mode": "shadow-tls"}
			if v, ok := stls["version"]; ok {
				opts["version"] = v
			}
			// Password: v2 uses the top-level password; v3 uses users[0].password.
			if pwd, ok := stls["password"].(string); ok && pwd != "" {
				opts["password"] = pwd
			} else if users, ok := stls["users"].([]interface{}); ok && len(users) > 0 {
				if u, ok := users[0].(map[string]interface{}); ok {
					if pwd, ok := u["password"].(string); ok {
						opts["password"] = pwd
					}
				}
			}
			if alpn, ok := stls["alpn"]; ok {
				opts["alpn"] = alpn
			}
			// host from handshake.dest (strip :port).
			if hs, ok := stls["handshake"].(map[string]interface{}); ok {
				if dest, ok := hs["dest"].(string); ok {
					opts["host"] = stripHostPort(dest)
				}
			}
			p["obfs-opts"] = opts
			return
		}
	}
	if restls, ok := src["res-tls"].(map[string]interface{}); ok {
		if enable, _ := restls["enable"].(bool); enable {
			opts := map[string]interface{}{
				"mode":         "restls",
				"version-hint": "tls13",
			}
			if dest, ok := restls["dest"].(string); ok {
				opts["host"] = stripHostPort(dest)
			}
			if pwd, ok := restls["password"].(string); ok {
				opts["password"] = pwd
			}
			if script, ok := restls["restls-script"].(string); ok && script != "" {
				opts["restls-script"] = script
			}
			p["obfs-opts"] = opts
			return
		}
	}
	if jls, ok := src["jls-config"].(map[string]interface{}); ok {
		if enable, _ := jls["enable"].(bool); enable {
			opts := map[string]interface{}{"mode": "jls"}
			if dest, ok := jls["dest"].(string); ok {
				opts["host"] = stripHostPort(dest)
			}
			// JLS users: use first user's username+password (single-user case).
			if users, ok := jls["users"].([]interface{}); ok && len(users) > 0 {
				if u, ok := users[0].(map[string]interface{}); ok {
					if un, ok := u["username"].(string); ok {
						opts["username"] = un
					}
					if pwd, ok := u["password"].(string); ok {
						opts["password"] = pwd
					}
				}
			}
			p["obfs-opts"] = opts
			return
		}
	}
}

// stripHostPort removes the :port suffix from a host:port string, returning
// the bare hostname. If the input has no port, it is returned unchanged.
func stripHostPort(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
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

// ensureTUICClientDefaults fills alpn/congestion and skip-cert-verify for
// panel self-signed certificates. TUIC is QUIC/UDP and typically uses ALPN h3.
func normalizeTUICToken(token interface{}) interface{} {
	// Listener stores token as []string; proxies-tuic client uses a single string.
	switch v := token.(type) {
	case string:
		return v
	case []string:
		if len(v) > 0 {
			return v[0]
		}
		return ""
	case []interface{}:
		if len(v) > 0 {
			return fmt.Sprint(v[0])
		}
		return ""
	default:
		return token
	}
}

func ensureTUICClientDefaults(p, opts map[string]interface{}) {
	if p["alpn"] == nil {
		p["alpn"] = []string{"h3"}
	}
	if p["congestion-controller"] == nil {
		p["congestion-controller"] = "bbr"
	}
	if certutil.ShouldSkipCertVerify(opts) {
		p["skip-cert-verify"] = true
	}
	if p["udp"] == nil {
		p["udp"] = true
	}
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
	// Panel self-signed certificates → clients must skip verify.
	if certutil.ShouldSkipCertVerify(src) {
		dst["skip-cert-verify"] = true
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
	// Ensure sni is set when only servername was derived (e.g. from
	// reality-config.server-names). Trojan and other protocols use "sni"
	// as the client-side TLS hostname, not "servername".
	if dst["sni"] == nil {
		if sn, ok := dst["servername"].(string); ok && sn != "" {
			dst["sni"] = sn
		}
	}
	// ech-opts is a documented client-side TLS field (proxies-tls wiki
	// block 0) with no direct listener-side counterpart (the listener
	// carries `ech-key` only — deriving the ECH config list from a raw key
	// is non-trivial and out of scope here). When the operator sets an
	// `ech-opts` block in the listener Config JSON via the panel's
	// free-form JSON editor, forward it verbatim to the client YAML.
	if v, ok := src["ech-opts"]; ok {
		dst["ech-opts"] = v
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
		opts := map[string]interface{}{"grpc-service-name": service}
		// Optional grpc-opts fields documented on proxies-transport
		// wiki block 2 (grpc-user-agent / ping-interval /
		// max-connections / min-streams / max-streams). The listener
		// schema only whitelists `grpc-service-name`; the panel can
		// still set these via the listener Config JSON (the converter
		// forwards them verbatim when present).
		for _, key := range []string{"grpc-user-agent", "ping-interval", "max-connections", "min-streams", "max-streams"} {
			if v, ok := src[key]; ok {
				opts[key] = v
			}
		}
		dst["grpc-opts"] = opts
	}
	if value, ok := src["xhttp-config"]; ok && value != nil {
		dst["network"] = "xhttp"
		dst["xhttp-opts"] = value
	}
	// mkcp-config → mkcp-opts (VMess only per mihomo docs).
	// Listener and client field names are identical (minus `enable`).
	if mkcp, ok := src["mkcp-config"].(map[string]interface{}); ok {
		if enable, _ := mkcp["enable"].(bool); enable {
			dst["network"] = "mkcp"
			opts := map[string]interface{}{}
			for _, key := range []string{"mtu", "tti", "uplink-capacity", "downlink-capacity", "congestion", "write-buffer", "read-buffer", "seed", "header"} {
				if v, ok := mkcp[key]; ok {
					opts[key] = v
				}
			}
			dst["mkcp-opts"] = opts
		}
	}
	// mekya-config → mekya-opts (VMess only per mihomo docs).
	// The listener mekya-config field names differ from the client mekya-opts
	// field names. The kcp sub-block maps 1:1. The URL field has no listener
	// counterpart and is left empty (operator must set it manually).
	//
	// `polling-interval-initial` and `h2-pool-size` (client-only fields per
	// proxies-transport mekya-opts wiki) are passed through verbatim when
	// present in the listener's mekya-config block. NOTE: the VMess listener
	// schema in internal/mihomo/config/listener_schema_registry.go does NOT
	// currently whitelist these two paths in its NestedFields list, so the
	// panel cannot set them until Task A / a follow-up adds them. They are
	// handled here so future schema additions work transparently.
	if mekya, ok := src["mekya-config"].(map[string]interface{}); ok {
		if enable, _ := mekya["enable"].(bool); enable {
			dst["network"] = "mekya"
			opts := map[string]interface{}{}
			if v, ok := mekya["max-write-size"]; ok {
				opts["max-request-size"] = v
			}
			if v, ok := mekya["max-write-duration-ms"]; ok {
				opts["max-write-delay"] = v
			}
			// `polling-interval-initial` (int, ms) and `h2-pool-size` (int)
			// are wiki-documented client mekya-opts fields with no listener
			// counterpart in inbound-vmess. Forward as-is when supplied.
			if v, ok := mekya["polling-interval-initial"]; ok {
				opts["polling-interval-initial"] = v
			}
			if v, ok := mekya["h2-pool-size"]; ok {
				opts["h2-pool-size"] = v
			}
			if kcp, ok := mekya["kcp"].(map[string]interface{}); ok {
				kcpOpts := map[string]interface{}{}
				for _, key := range []string{"mtu", "tti", "uplink-capacity", "downlink-capacity", "congestion", "write-buffer", "read-buffer", "seed", "header"} {
					if v, ok := kcp[key]; ok {
						kcpOpts[key] = v
					}
				}
				opts["kcp"] = kcpOpts
			}
			dst["mekya-opts"] = opts
		}
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
	return base64.RawURLEncoding.EncodeToString(public), nil
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
	// restls-script is a documented client field (tls.md L190-192) used to
	// control the post-handshake Restls carrier traffic script.
	if script, ok := cfg["restls-script"].(string); ok && script != "" {
		result["restls-script"] = script
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// tlsmirrorClientOptions forwards the listener-side `tlsmirror-config` block
// to the client-side `tlsmirror-opts` shape per proxies-tls wiki block 0.
// The block is passed through verbatim since the listener and client
// structures are identical (primary-key + explicit-nonce-ciphersuites +
// transport-layer-padding + connection-enrolment + sequence-watermarking-
// enabled + embedded-traffic-generator). Returns nil when the listener has
// no tlsmirror-config (matches m-ui path's decodeTLSMirrorOpts helper in
// mui/bridge.go).
func tlsmirrorClientOptions(src map[string]interface{}) map[string]interface{} {
	cfg, ok := src["tlsmirror-config"].(map[string]interface{})
	if !ok || len(cfg) == 0 {
		return nil
	}
	result := make(map[string]interface{}, len(cfg))
	for k, v := range cfg {
		result[k] = v
	}
	return result
}

// jlsClientOptions exports JLS credentials for the client restls/jls-opts block.
//
// Known limitation (P3-2): only the first user in jls-config.users is exported.
// Although applyClientWrappers is invoked once per protocol credential, the
// credential identifier differs per protocol (UUID for vmess/vless, password
// for trojan/anytls) and cannot be matched reliably against jls-config.users
// which keys users by {username, password}. Per-user matching would require an
// explicit listener-side mapping table that the current schema does not carry.
// For the shared single-JLS-user deployment (the documented common case) this
// is correct; multi-JLS-user listeners currently fall back to the first user.
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
	case "shadowsocks", "snell", "vmess", "vless", "trojan",
		"hysteria2", "tuic", "tuic-v4", "tuic-v5", "shadowquic",
		"anytls", "trusttunnel":
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
