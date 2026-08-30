package converter

import (
	"encoding/json"
	"fmt"
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
	for _, listener := range listeners {
		creds := filtered[listener.ID]
		host := ResolveListenerServer(config.GlobalConfig, req, listener)
		if host == "" {
			host = serverHost
		}
		proxies, err := listenerToProxies(listener, host, creds)
		if err != nil {
			continue
		}
		for _, p := range proxies {
			ob, tag := mihomoProxyToSingbox(p)
			if ob == nil {
				continue
			}
			outbounds = append(outbounds, ob)
			if tag != "" {
				tagNames = append(tagNames, tag)
			}
		}
	}
	if len(outbounds) == 0 || len(tagNames) == 0 {
		return nil, fmt.Errorf("no exportable sing-box outbounds for user")
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
	ob := map[string]interface{}{
		"type":        mapMihomoTypeToSingbox(typ),
		"tag":         name,
		"server":      server,
		"server_port": port,
	}
	switch strings.ToLower(typ) {
	case "ss", "shadowsocks":
		ob["method"] = p["cipher"]
		ob["password"] = p["password"]
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
			ob["up_mbps"] = parseMbps(up)
		}
		if down, ok := p["down"].(string); ok {
			ob["down_mbps"] = parseMbps(down)
		}
	case "tuic":
		// TUIC v4 authenticates with a token (string or []string in mihomo);
		// v5 uses uuid + password. sing-box's TUIC outbound accepts `token`
		// ([]string) for v4 and `uuid`/`password` for v5. Without this, v4
		// mihomo proxies produce credential-less sing-box outbounds.
		if rawToken, ok := p["token"]; ok && rawToken != nil {
			var tokens []string
			switch t := rawToken.(type) {
			case []interface{}:
				tokens = make([]string, 0, len(t))
				for _, v := range t {
					if s, ok := v.(string); ok && s != "" {
						tokens = append(tokens, s)
					}
				}
			case []string:
				tokens = t
			case string:
				if strings.TrimSpace(t) != "" {
					tokens = []string{t}
				}
			}
			if len(tokens) > 0 {
				ob["token"] = tokens
			}
		} else {
			if uuid, ok := p["uuid"].(string); ok && uuid != "" {
				ob["uuid"] = uuid
			}
			if pass, ok := p["password"].(string); ok && pass != "" {
				ob["password"] = pass
			}
		}
	default:
		// Keep generic fields best-effort.
		if pw, ok := p["password"]; ok {
			ob["password"] = pw
		}
		if uuid, ok := p["uuid"]; ok {
			ob["uuid"] = uuid
		}
	}
	// TLS / reality / transport — best-effort mapping from Mihomo fields.
	if tls, _ := p["tls"].(bool); tls || strings.EqualFold(fmt.Sprint(p["tls"]), "true") {
		tlsObj := map[string]interface{}{"enabled": true}
		if sni, ok := p["servername"].(string); ok && sni != "" {
			tlsObj["server_name"] = sni
		} else if sni, ok := p["sni"].(string); ok && sni != "" {
			tlsObj["server_name"] = sni
		}
		if fp, ok := p["client-fingerprint"].(string); ok && fp != "" {
			tlsObj["utls"] = map[string]interface{}{"enabled": true, "fingerprint": fp}
		}
		if reality, ok := p["reality-opts"].(map[string]interface{}); ok {
			tlsObj["reality"] = map[string]interface{}{
				"enabled":    true,
				"public_key": reality["public-key"],
				"short_id":   reality["short-id"],
			}
		}
		if v, ok := p["skip-cert-verify"].(bool); ok && v {
			tlsObj["insecure"] = true
		}
		if alpn, ok := p["alpn"]; ok {
			tlsObj["alpn"] = alpn
		}
		ob["tls"] = tlsObj
	}
	if network, ok := p["network"].(string); ok && network != "" && network != "tcp" {
		tr := map[string]interface{}{"type": network}
		if opts, ok := p["ws-opts"].(map[string]interface{}); ok {
			tr["path"] = opts["path"]
			if headers, ok := opts["headers"].(map[string]interface{}); ok {
				tr["headers"] = headers
			}
		}
		if opts, ok := p["grpc-opts"].(map[string]interface{}); ok {
			tr["service_name"] = opts["grpc-service-name"]
		}
		ob["transport"] = tr
	}
	return ob, name
}

func mapMihomoTypeToSingbox(t string) string {
	switch strings.ToLower(t) {
	case "ss", "shadowsocks":
		return "shadowsocks"
	case "hysteria2":
		return "hysteria2"
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
