package converter

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// GenerateUserSingboxSubscription builds a minimal sing-box outbound document
// from the user's bound Mihomo listeners (sing-box subscription).
func GenerateUserSingboxSubscription(db *gorm.DB, pu models.ProxyUser, req *http.Request) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if !user.IsCredentialActive(pu) {
		return nil, fmt.Errorf("user is not active")
	}
	listeners, filtered, err := userBoundListeners(db, pu)
	if err != nil {
		return nil, err
	}
	serverHost := ResolveServerAddress(config.GlobalConfig, req)

	outbounds := make([]map[string]interface{}, 0)
	tagNames := make([]string, 0)
	var skipped []string
	for _, listener := range listeners {
		creds := filtered[listener.ID]
		host := ResolveListenerServer(config.GlobalConfig, req, listener)
		if host == "" {
			host = serverHost
		}
		proxies, err := listenerToProxies(listener, host, creds)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", listener.Name, err))
			continue
		}
		if len(proxies) == 0 {
			skipped = append(skipped, fmt.Sprintf("%s: empty mihomo export", listener.Name))
			continue
		}
		for _, p := range proxies {
			ob, tag := mihomoProxyToSingbox(p)
			if ob == nil {
				typ, _ := p["type"].(string)
				skipped = append(skipped, fmt.Sprintf("%s: unsupported or incomplete type %q", listener.Name, typ))
				continue
			}
			outbounds = append(outbounds, ob)
			if tag != "" {
				tagNames = append(tagNames, tag)
			}
		}
	}
	if len(outbounds) == 0 || len(tagNames) == 0 {
		if len(skipped) > 0 {
			return nil, fmt.Errorf("no exportable sing-box outbounds for user (%s)", strings.Join(skipped, "; "))
		}
		return nil, fmt.Errorf("no exportable sing-box outbounds for user")
	}
	if len(skipped) > 0 {
		log.Printf("sing-box subscription: skipped %d node(s) for user %s: %s", len(skipped), pu.Username, strings.Join(skipped, "; "))
	}
	// Selector + direct for a usable minimal config (clients often strip extras).
	outbounds = append(outbounds,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{
			"type":      "selector",
			"tag":       "proxy",
			"outbounds": append([]string{}, tagNames...),
			"default":   tagNames[0],
		},
	)
	doc := map[string]interface{}{
		"outbounds": outbounds,
	}
	return json.MarshalIndent(doc, "", "  ")
}

func mihomoProxyToSingbox(p map[string]interface{}) (map[string]interface{}, string) {
	typ, _ := p["type"].(string)
	name, _ := p["name"].(string)
	if name == "" {
		name = "proxy"
	}
	server, _ := p["server"].(string)
	port := toInt(p["port"])
	if server == "" || port == 0 {
		return nil, ""
	}
	typLower := strings.ToLower(strings.TrimSpace(typ))
	sbType := mapMihomoTypeToSingbox(typLower)
	if sbType == "" {
		return nil, ""
	}
	ob := map[string]interface{}{
		"type":        sbType,
		"tag":         name,
		"server":      server,
		"server_port": port,
	}
	switch typLower {
	case "ss", "shadowsocks":
		ob["method"] = p["cipher"]
		ob["password"] = p["password"]
		if plugin, ok := p["plugin"].(string); ok && plugin != "" {
			ob["plugin"] = plugin
			if po, ok := p["plugin-opts"]; ok {
				ob["plugin_opts"] = po
			}
		}
	case "vmess":
		ob["uuid"] = firstString(p["uuid"], p["password"])
		ob["security"] = firstString(p["cipher"], "auto")
		if alterId, ok := p["alterId"]; ok {
			ob["alter_id"] = alterId
		} else {
			ob["alter_id"] = 0
		}
	case "vless":
		ob["uuid"] = firstString(p["uuid"], p["password"])
		if flow, ok := p["flow"].(string); ok && flow != "" {
			ob["flow"] = flow
		}
		if enc, ok := p["encryption"].(string); ok && enc != "" {
			ob["encryption"] = enc
		}
	case "trojan":
		ob["password"] = p["password"]
	case "hysteria2":
		ob["password"] = p["password"]
		if up, ok := p["up"].(string); ok {
			if n := parseMbps(up); n > 0 {
				ob["up_mbps"] = n
			}
		}
		if down, ok := p["down"].(string); ok {
			if n := parseMbps(down); n > 0 {
				ob["down_mbps"] = n
			}
		}
		if obfs, ok := p["obfs"].(string); ok && obfs != "" {
			ob["obfs"] = map[string]interface{}{"type": obfs}
			if pwd, ok := p["obfs-password"].(string); ok && pwd != "" {
				ob["obfs"].(map[string]interface{})["password"] = pwd
			}
		}
	case "tuic":
		// v4: token; v5: uuid + password
		if rawToken, ok := p["token"]; ok && rawToken != nil {
			tokens := normalizeStringList(rawToken)
			if len(tokens) > 0 {
				// sing-box accepts uuid-less token mode via uuid empty + token in some builds;
				// prefer explicit token field when present (v4).
				ob["uuid"] = tokens[0]
				ob["password"] = tokens[0]
				ob["token"] = tokens
			}
		}
		if uuid, ok := p["uuid"].(string); ok && strings.TrimSpace(uuid) != "" {
			ob["uuid"] = strings.TrimSpace(uuid)
		}
		if pass, ok := p["password"].(string); ok && strings.TrimSpace(pass) != "" {
			ob["password"] = strings.TrimSpace(pass)
		}
		if cc, ok := p["congestion-controller"].(string); ok && cc != "" {
			ob["congestion_control"] = cc
		}
		if mode, ok := p["udp-relay-mode"].(string); ok && mode != "" {
			ob["udp_relay_mode"] = mode
		} else {
			ob["udp_relay_mode"] = "native"
		}
		if v, ok := p["reduce-rtt"].(bool); ok {
			ob["zero_rtt_handshake"] = v
		}
	case "anytls":
		ob["password"] = p["password"]
	case "socks5", "socks":
		ob["type"] = "socks"
		if u, ok := p["username"].(string); ok {
			ob["username"] = u
		}
		if pw, ok := p["password"].(string); ok {
			ob["password"] = pw
		}
	case "http":
		if u, ok := p["username"].(string); ok {
			ob["username"] = u
		}
		if pw, ok := p["password"].(string); ok {
			ob["password"] = pw
		}
	default:
		// Best-effort for newer/less common types still named the same in sing-box.
		if pw, ok := p["password"]; ok {
			ob["password"] = pw
		}
		if uuid, ok := p["uuid"]; ok {
			ob["uuid"] = uuid
		}
	}

	// TLS: Mihomo omits top-level tls:true for TUIC/HY2 (QUIC inherent TLS).
	// Always attach tls for protocols that need it so sing-box gets SNI/insecure/alpn.
	if needsSingboxTLS(typLower, p) {
		ob["tls"] = buildSingboxTLS(p)
	}

	if network, ok := p["network"].(string); ok && network != "" && network != "tcp" {
		tr := map[string]interface{}{"type": network}
		if opts, ok := p["ws-opts"].(map[string]interface{}); ok {
			if path, ok := opts["path"]; ok {
				tr["path"] = path
			}
			if headers, ok := opts["headers"].(map[string]interface{}); ok {
				tr["headers"] = headers
			}
		}
		if opts, ok := p["grpc-opts"].(map[string]interface{}); ok {
			if sn, ok := opts["grpc-service-name"]; ok {
				tr["service_name"] = sn
			}
		}
		ob["transport"] = tr
	}
	return ob, name
}

func needsSingboxTLS(typ string, p map[string]interface{}) bool {
	switch typ {
	case "tuic", "hysteria2", "anytls", "trojan":
		return true
	case "vless", "vmess":
		if tls, _ := p["tls"].(bool); tls {
			return true
		}
		if strings.EqualFold(fmt.Sprint(p["tls"]), "true") {
			return true
		}
		if _, ok := p["reality-opts"]; ok {
			return true
		}
		if sni, _ := p["sni"].(string); strings.TrimSpace(sni) != "" {
			return true
		}
		if sni, _ := p["servername"].(string); strings.TrimSpace(sni) != "" {
			return true
		}
		return false
	default:
		if tls, _ := p["tls"].(bool); tls {
			return true
		}
		return strings.EqualFold(fmt.Sprint(p["tls"]), "true")
	}
}

func buildSingboxTLS(p map[string]interface{}) map[string]interface{} {
	tlsObj := map[string]interface{}{"enabled": true}
	if sni, ok := p["servername"].(string); ok && strings.TrimSpace(sni) != "" {
		tlsObj["server_name"] = strings.TrimSpace(sni)
	} else if sni, ok := p["sni"].(string); ok && strings.TrimSpace(sni) != "" {
		tlsObj["server_name"] = strings.TrimSpace(sni)
	}
	if fp, ok := p["client-fingerprint"].(string); ok && fp != "" {
		tlsObj["utls"] = map[string]interface{}{"enabled": true, "fingerprint": fp}
	}
	if reality, ok := p["reality-opts"].(map[string]interface{}); ok {
		r := map[string]interface{}{"enabled": true}
		if pk, ok := reality["public-key"]; ok {
			r["public_key"] = pk
		}
		if sid, ok := reality["short-id"]; ok {
			r["short_id"] = sid
		}
		tlsObj["reality"] = r
	}
	if v, ok := p["skip-cert-verify"].(bool); ok && v {
		tlsObj["insecure"] = true
	}
	if alpn, ok := p["alpn"]; ok {
		tlsObj["alpn"] = normalizeStringList(alpn)
	}
	return tlsObj
}

func normalizeStringList(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			if strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(t) != "" {
			return []string{t}
		}
	}
	return nil
}

func mapMihomoTypeToSingbox(t string) string {
	switch strings.ToLower(t) {
	case "ss", "shadowsocks":
		return "shadowsocks"
	case "hysteria2":
		return "hysteria2"
	case "socks", "socks5":
		return "socks"
	case "snell", "mieru", "sudoku", "shadowquic", "trusttunnel":
		// No stable 1:1 outbound in stock sing-box for these panel types.
		return ""
	default:
		return strings.ToLower(t)
	}
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		var x int
		fmt.Sscanf(n, "%d", &x)
		return x
	default:
		return 0
	}
}

func firstString(vals ...interface{}) string {
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func parseMbps(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, "mbps")
	s = strings.TrimSpace(s)
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
