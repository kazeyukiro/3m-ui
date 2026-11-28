package node

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
	"golang.org/x/crypto/curve25519"
)

// clientSkipCert decides skip-cert-verify for share/subscription URIs using
// the connect host (IP vs domain vs certificate SAN).
// rowUUID returns the user UUID from a credential row, accepting username when
// the panel stored the UUID only in the username field.
func rowUUID(row map[string]interface{}) string {
	if row == nil {
		return ""
	}
	for _, key := range []string{"uuid", "id", "username"} {
		if v, ok := row[key].(string); ok {
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		}
	}
	return ""
}

func looksLikeIP(host string) bool {
	h := strings.Trim(host, "[]")
	return net.ParseIP(h) != nil
}

func clientSkipCert(cfg map[string]interface{}, connectHost string) bool {
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	var explicit *bool
	if b, ok := cfg["skip-cert-verify"].(bool); ok {
		explicit = &b
	}
	cert, _ := cfg["certificate"].(string)
	// Prefer SNI / servername as the identity the client will actually verify.
	verify := connectHost
	for _, key := range []string{"sni", "servername"} {
		if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
			verify = strings.TrimSpace(v)
			break
		}
	}
	return certutil.DecideClientSkipCertVerify(cert, verify, explicit)
}

// resolveURITLS picks client SNI and skip-cert for share/subscription URIs.
// Uses certutil.ResolveClientSNI so AccessSNI cannot override a formal cert's SANs
// (e.g. www.bing.com must not win over a Let's Encrypt IP certificate).
func resolveURITLS(cfg map[string]interface{}, host string) (sni string, skip bool) {
	certPEM := loadCertPEMFromCfg(cfg)
	configured := ""
	if cfg != nil {
		for _, key := range []string{"sni", "servername"} {
			if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
				configured = strings.TrimSpace(v)
				break
			}
		}
	}
	sni = certutil.ResolveClientSNI(configured, "", host, certPEM)
	// A self-signed fallback certificate whose only identity is "localhost"
	// is useless to a client dialling the server by IP/域名; advertising it
	// sends WS Host / SNI to localhost (P2-4). Fall back to the connect host.
	if strings.EqualFold(strings.TrimSpace(sni), "localhost") && !strings.EqualFold(strings.TrimSpace(host), "localhost") {
		sni = strings.Trim(strings.TrimSpace(host), "[]")
	}
	if strings.TrimSpace(sni) == "" {
		sni = strings.Trim(strings.TrimSpace(host), "[]")
	}
	var explicit *bool
	if cfg != nil {
		if b, ok := cfg["skip-cert-verify"].(bool); ok {
			explicit = &b
		}
	}
	verify := sni
	if verify == "" {
		verify = host
	}
	skip = certutil.DecideClientSkipCertVerify(certPEM, verify, explicit)
	return sni, skip
}

func tlsParams(cfg map[string]interface{}, connectHost string) map[string]string {
	params := map[string]string{}
	if enabled, ok := cfg["_listener-tls"].(bool); ok && enabled {
		params["security"] = "tls"
	}
	if certificate, ok := cfg["certificate"].(string); ok && strings.TrimSpace(certificate) != "" {
		params["security"] = "tls"
	}
	if v, ok := cfg["client-fingerprint"].(string); ok && v != "" {
		params["fp"] = v
	}
	if v, ok := cfg["fingerprint"].(string); ok && v != "" {
		params["fp"] = v
	}
	// URI allowInsecure: panel self-signed or explicit skip only.
	// Formal certificates must NOT force insecure. Reality uses its own path
	// and must not inherit panel-certificate skip flags into the query string.
	if _, isReality := cfg["reality-config"]; !isReality {
		sni, skip := resolveURITLS(cfg, connectHost)
		if sni != "" {
			params["sni"] = sni
		}
		if skip {
			params["insecure"] = "1"
			params["allowInsecure"] = "1"
		}
	} else {
		// Reality: still surface configured camouflage SNI when present.
		for _, key := range []string{"sni", "servername"} {
			if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
				params["sni"] = strings.TrimSpace(v)
				break
			}
		}
	}
	// TLS without fingerprint is fragile against CDN / middleboxes.
	if params["security"] == "tls" && params["fp"] == "" {
		params["fp"] = "chrome"
	}
	// ALPN: listener.access_alpn is the client-facing override. It must reach the
	// link as well, otherwise the same node advertises different TLS handshakes
	// through YAML (alpn list) and through a share URI (absent).
	if params["security"] == "tls" {
		if raw, ok := cfg["alpn"]; ok {
			if list := alpnStringList(raw); len(list) > 0 {
				params["alpn"] = strings.Join(list, ",")
			}
		}
	}
	return params
}

// alpnStringList normalizes the accepted alpn storage shapes (CSV string,
// []string, []interface{}) into a clean list.
func alpnStringList(raw interface{}) []string {
	var out []string
	switch v := raw.(type) {
	case string:
		for _, part := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' }) {
			if s := strings.TrimSpace(part); s != "" {
				out = append(out, s)
			}
		}
	case []string:
		for _, part := range v {
			if s := strings.TrimSpace(part); s != "" {
				out = append(out, s)
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	}
	return out
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
	if params["type"] == "ws" && params["host"] == "" {
		for _, key := range []string{"sni", "servername"} {
			if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
				params["host"] = strings.TrimSpace(v)
				break
			}
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
		// mode (auto / packet-up / stream-up) defaults differ between clients;
		// make the listener's choice explicit instead of relying on each
		// client's own default (P3).
		if mode := configString(xhttp, "mode"); mode != "" {
			params["mode"] = mode
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
	// Standard base64 (SIP002 letter table): legacy clients only implement
	// the +/ alphabet, and base64url's -_ makes those parsers fail (P3).
	encoded := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + password))
	params := map[string]string{}
	// SIP003 plugin: a Shadowsocks listener wrapped in simple-obfs / kcptun /
	// shadow-tls / restls / jls is unusable from a plain ss:// link — the
	// client has to speak the same wrapper (P1-5). Encode the wrapper that
	// the server actually runs so the URI stays self-sufficient.
	if plugin := shadowsocksPluginValue(cfg); plugin != "" {
		params["plugin"] = plugin
	}
	return []string{addName(query("ss://"+encoded+"@"+netutil.JoinHostPort(host, port), params), name)}, nil
}

// shadowsocksPluginValue builds the SIP003 plugin string for the wrappers a
// Shadowsocks listener may carry (simple-obfs / kcp-tun / shadow-tls / restls /
// jls). The names and option keys follow the mihomo proxies/ss wiki so both
// mihomo and V2RayN-family clients can parse them.
func shadowsocksPluginValue(cfg map[string]interface{}) string {
	if obfs, ok := cfg["simple-obfs"].(map[string]interface{}); ok && cfgBool(obfs["enable"]) {
		parts := []string{"obfs-local"}
		if mode := configString(obfs, "mode"); mode != "" {
			parts = append(parts, "obfs="+mode)
		}
		if obfsHost := configString(obfs, "host"); obfsHost != "" {
			parts = append(parts, "obfs-host="+obfsHost)
		}
		return strings.Join(parts, ";")
	}
	if kcp, ok := cfg["kcp-tun"].(map[string]interface{}); ok && cfgBool(kcp["enable"]) {
		parts := []string{"kcptun"}
		for _, key := range []string{"key", "crypt", "mode", "conn"} {
			if v := configString(kcp, key); v != "" {
				parts = append(parts, key+"="+v)
			}
		}
		return strings.Join(parts, ";")
	}
	if stls, ok := cfg["shadow-tls"].(map[string]interface{}); ok && cfgBool(stls["enable"]) {
		parts := []string{"shadow-tls"}
		if hs, ok := stls["handshake"].(map[string]interface{}); ok {
			if dest := configString(hs, "dest"); dest != "" {
				parts = append(parts, "host="+stripURIHostPort(dest))
			}
		}
		if pwd := configString(stls, "password"); pwd != "" {
			parts = append(parts, "password="+pwd)
		} else if users, ok := stls["users"].([]interface{}); ok && len(users) > 0 {
			if u, ok := users[0].(map[string]interface{}); ok {
				if pwd := configString(u, "password"); pwd != "" {
					parts = append(parts, "password="+pwd)
				}
			}
		}
		if version := configString(stls, "version"); version != "" {
			parts = append(parts, "version="+version)
		}
		return strings.Join(parts, ";")
	}
	if restls, ok := cfg["res-tls"].(map[string]interface{}); ok && cfgBool(restls["enable"]) {
		parts := []string{"restls"}
		if dest := configString(restls, "dest"); dest != "" {
			parts = append(parts, "host="+stripURIHostPort(dest))
		}
		if pwd := configString(restls, "password"); pwd != "" {
			parts = append(parts, "password="+pwd)
		}
		parts = append(parts, "version-hint=tls13")
		return strings.Join(parts, ";")
	}
	if jls, ok := cfg["jls-config"].(map[string]interface{}); ok && cfgBool(jls["enable"]) {
		parts := []string{"jls"}
		if dest := configString(jls, "dest"); dest != "" {
			parts = append(parts, "host="+stripURIHostPort(dest))
		}
		if users, ok := jls["users"].([]interface{}); ok && len(users) > 0 {
			if u, ok := users[0].(map[string]interface{}); ok {
				if un := configString(u, "username"); un != "" {
					parts = append(parts, "username="+un)
				}
				if pwd := configString(u, "password"); pwd != "" {
					parts = append(parts, "password="+pwd)
				}
			}
		}
		return strings.Join(parts, ";")
	}
	return ""
}

// configString reads a listener-config value as a string, accepting JSON
// numbers — the panel stores max-udp-relay-packet-size: 1500 and alterId: 8
// as numbers, which a plain string type-assert silently drops (P1-5/P2-5).
func configString(cfg map[string]interface{}, key string) string {
	if cfg == nil {
		return ""
	}
	return valueString(cfg[key])
}

func valueString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return valueString(float64(t))
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return strings.TrimSpace(t.String())
	default:
		return ""
	}
}

func cfgBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

// userinfoSecret renders userinfo of the form "user:secret" (or a bare secret
// when user is empty) for a share URI.
//
// Rules:
//   - exactly one literal colon separates user from secret — Go's net/url
//     splits the userinfo at the FIRST literal colon, and url.User would
//     escape it to %3A, after which Password() returns "" for every Go client;
//   - everything outside the unreserved set (ALPHA/DIGIT/-._~) is
//     percent-escaped, so @ (userinfo boundary), / ? # (authority/path
//     boundaries) and % (escape introducer) can never corrupt the link.
//
// This matches what v2rayN / mihomo / sing-box expect: they unescape the whole
// userinfo and split at the first colon. It also round-trips exactly through
// Go's url.Parse + User.String().
func userinfoSecret(user, secret string) string {
	esc := func(s string) string {
		var b strings.Builder
		for i := 0; i < len(s); i++ {
			c := s[i]
			if userinfoUnreserved(c) {
				b.WriteByte(c)
			} else {
				b.WriteString(fmt.Sprintf("%%%02X", c))
			}
		}
		return b.String()
	}
	if user == "" {
		return esc(secret)
	}
	return esc(user) + ":" + esc(secret)
}

func userinfoUnreserved(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

// stripURIHostPort removes the :port suffix from a host:port pair.
func stripURIHostPort(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

func vlessURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	rows := userRows(cfg)
	if len(rows) == 0 {
		return nil, fmt.Errorf("vless listener requires at least one user for URI export")
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		uuid := rowUUID(row)
		if uuid == "" {
			return nil, fmt.Errorf("vless user uuid is required")
		}
		params := tlsParams(cfg, host)
		params["type"] = "tcp"
		// Resolve the transport before deciding anything that depends on it.
		for k, v := range transportParams(cfg) {
			params[k] = v
		}
		// Vision flow is TCP-only. Ask the transport rather than the config: a
		// listener switched to ws/grpc/xhttp keeps its old flow in storage, and
		// exporting it produces a link no client can use.
		if flow, _ := row["flow"].(string); flow != "" && protocol.TransportCarriesFlow(cfg) {
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
		uuid := rowUUID(row)
		if uuid == "" {
			return nil, fmt.Errorf("vmess user uuid is required")
		}
		// alterId is commonly persisted as a JSON number; stringValue would
		// silently fall back to "0" and break alterId!=0 listeners (P2-5).
		aid := configString(cfg, "alterId")
		if aid == "" {
			aid = "0"
		}
		cipher := stringValue(cfg["cipher"], "auto")
		obj := map[string]string{"v": "2", "ps": name, "add": host, "port": port, "id": uuid, "aid": aid, "scy": cipher, "net": "tcp", "type": "none"}
		tlsOpts := tlsParams(cfg, host)
		if tlsOpts["security"] == "tls" {
			obj["tls"] = "tls"
		}
		if tlsOpts["sni"] != "" {
			obj["sni"] = tlsOpts["sni"]
		}
		if tlsOpts["fp"] != "" {
			obj["fp"] = tlsOpts["fp"]
		}
		// vless and trojan carry this in their query string. v2rayN's documented
		// vmess field set has no skip-certificate field, so it ignores this one,
		// but clients that do read it are the ones whose verification would
		// otherwise fail — see vmessSchema for why it is kept anyway.
		if tlsOpts["allowInsecure"] != "" {
			obj["allowInsecure"] = tlsOpts["allowInsecure"]
		}
		if ws, ok := cfg["ws-path"].(string); ok && ws != "" {
			obj["net"] = "ws"
			obj["path"] = ws
			if headers, ok := cfg["ws-headers"].(map[string]interface{}); ok {
				if h, ok := headers["Host"].(string); ok && h != "" {
					obj["host"] = h
				}
			}
			// WS Host header: prefer explicit ws-headers.Host, then SNI,
			// then server address (for CDN scenarios where WS Host ≠ server IP).
			if obj["host"] == "" {
				if h := tlsOpts["sni"]; h != "" {
					obj["host"] = h
				} else if host != "" && !looksLikeIP(host) {
					obj["host"] = host
				}
			}
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
		params := tlsParams(cfg, host)
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
		result = append(result, addName(query("trojan://"+userinfoSecret("", password)+"@"+netutil.JoinHostPort(host, port), params), name))
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
			sni, skip := resolveURITLS(cfg, host)
			if sni != "" {
				params["sni"] = sni
			}
			if skip {
				params["insecure"] = "1"
				params["allowInsecure"] = "1"
			}
			if v, ok := firstString(cfg["alpn"]); ok {
				params["alpn"] = v
			}
			if params["alpn"] == "" {
				params["alpn"] = "h3"
			}
			if params["sni"] == "" && host != "" {
				params["sni"] = strings.Trim(host, "[]")
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
			userinfo := userinfoSecret("", password)
			result = append(result, addName(query("hysteria2://"+userinfo+"@"+netutil.JoinHostPort(host, port), params), name))
		}
		if len(result) > 0 {
			return result, nil
		}
	}
	users := userMap(cfg)
	if len(users) == 0 {
		// Single password / auth string (common for panel-managed HY2).
		pass := ""
		if v, ok := cfg["password"].(string); ok {
			pass = strings.TrimSpace(v)
		}
		if pass == "" {
			if v, ok := cfg["auth"].(string); ok {
				pass = strings.TrimSpace(v)
			}
		}
		if pass != "" {
			users = map[string]interface{}{"": pass}
		}
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("hysteria2 listener requires at least one user/password for URI export")
	}
	result := make([]string, 0, len(users))
	for username, raw := range users {
		password, ok := raw.(string)
		if !ok || password == "" {
			return nil, fmt.Errorf("hysteria2 user %q has empty password", username)
		}
		params := map[string]string{}
		sni, skip := resolveURITLS(cfg, host)
		if sni != "" {
			params["sni"] = sni
		}
		if skip {
			params["insecure"] = "1"
			params["allowInsecure"] = "1"
		}
		if v, ok := firstString(cfg["alpn"]); ok {
			params["alpn"] = v
		}
		if params["alpn"] == "" {
			params["alpn"] = "h3"
		}
		if params["sni"] == "" && host != "" {
			params["sni"] = strings.Trim(host, "[]")
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
		userinfo := userinfoSecret("", password)
		result = append(result, addName(query("hysteria2://"+userinfo+"@"+netutil.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func tuicURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	// Build the shared query-params map ONCE. Mihomo's TUIC URI parser expects
	// snake_case keys (congestion_control, udp_relay_mode, max_udp_relay_packet_size,
	// bbr_profile, allow_insecure) — NOT the YAML/listener hyphen-case form.
	// NekoBox / sing-box also accept allowInsecure; emit both for compatibility.
	params := map[string]string{}
	for key, out := range map[string]string{
		"congestion-controller":     "congestion_control",
		"bbr-profile":               "bbr_profile",
		"udp-relay-mode":            "udp_relay_mode",
		"max-udp-relay-packet-size": "max_udp_relay_packet_size",
	} {
		// configString accepts JSON numbers: max-udp-relay-packet-size: 1500
		// is stored as an int and a plain string assert dropped it (P1-5).
		if v := configString(cfg, key); v != "" {
			params[out] = v
		}
	}
	// Working defaults used by most TUIC clients when the listener left these empty.
	if params["congestion_control"] == "" {
		params["congestion_control"] = "bbr"
	}
	if params["udp_relay_mode"] == "" {
		params["udp_relay_mode"] = "native"
	}
	if v, ok := firstString(cfg["alpn"]); ok {
		params["alpn"] = v
	}
	if params["alpn"] == "" {
		params["alpn"] = "h3"
	}
	sni, skip := resolveURITLS(cfg, host)
	if sni != "" {
		params["sni"] = sni
	}
	if skip {
		params["allow_insecure"] = "1"
		params["allowInsecure"] = "1"
	}

	// token is an array of strings per tuic-v4 (Mihomo listener config).
	if tokens, ok := cfg["token"].([]interface{}); ok && len(tokens) > 0 {
		result := make([]string, 0, len(tokens))
		for _, t := range tokens {
			ts, ok := t.(string)
			if !ok || strings.TrimSpace(ts) == "" {
				continue
			}
			// v4 URI: tuic://<token>@host:port?<snake_case params>#<name>
			// Pre-fix this path emitted zero query params, so v4 clients
			// lost congestion_control / udp_relay_mode / alpn / sni / allow_insecure.
			// url.User escapes @ / ? / # that PathEscape leaves intact (P0-1:
			// a token containing @ broke the userinfo boundary for every client).
			result = append(result, addName(query("tuic://"+userinfoSecret("", ts)+"@"+netutil.JoinHostPort(host, port), params), name))
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
			// v5 URI: tuic://<uuid>:<password>@host:port — userinfoSecret keeps
			// the first colon as the uuid/password separator while escaping
			// @ / / ? # inside credentials (P0-1).
			result = append(result, addName(query("tuic://"+userinfoSecret(uuid, password)+"@"+netutil.JoinHostPort(host, port), params), name))
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
		result = append(result, addName(query("tuic://"+userinfoSecret(uuid, password)+"@"+netutil.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func anytlsURIs(name, host, port string, cfg map[string]interface{}) ([]string, error) {
	users := userMap(cfg)
	if len(users) == 0 {
		return nil, fmt.Errorf("anytls listener requires at least one user for URI export")
	}
	sni, skip := resolveURITLS(cfg, host)
	result := make([]string, 0, len(users))
	for username, raw := range users {
		password, ok := raw.(string)
		if !ok || password == "" {
			return nil, fmt.Errorf("anytls user %q has empty password", username)
		}
		params := map[string]string{}
		if sni != "" {
			params["sni"] = sni
		}
		if v, ok := cfg["client-fingerprint"].(string); ok && v != "" {
			params["fp"] = v
		} else if v, ok := cfg["fingerprint"].(string); ok && v != "" {
			params["fp"] = v
		} else {
			params["fp"] = "chrome"
		}
		if skip {
			params["insecure"] = "1"
			params["allowInsecure"] = "1"
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
		result = append(result, addName(query("anytls://"+userinfoSecret("", password)+"@"+netutil.JoinHostPort(host, port), params), name))
	}
	return result, nil
}

func loadCertPEMFromCfg(cfg map[string]interface{}) string {
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
	if strings.HasPrefix(cert, "/") || strings.HasSuffix(cert, ".pem") || strings.HasSuffix(cert, ".crt") {
		if b, err := os.ReadFile(cert); err == nil {
			return string(b)
		}
	}
	return ""
}

func clientSkipCertWithPEM(cfg map[string]interface{}, certPEM, verifyHost string) bool {
	var explicit *bool
	if cfg != nil {
		if b, ok := cfg["skip-cert-verify"].(bool); ok {
			explicit = &b
		}
	}
	if certPEM == "" && cfg != nil {
		certPEM, _ = cfg["certificate"].(string)
	}
	return certutil.DecideClientSkipCertVerify(certPEM, verifyHost, explicit)
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

// uriTLSServerName returns the first non-IP hostname among candidates (for SNI).
func uriTLSServerName(candidates ...string) string {
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
