package converter

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
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
	usedTags := map[string]int{}
	appendProxy := func(p map[string]interface{}, source string) {
		ob, tag := mihomoProxyToSingbox(p)
		if ob == nil {
			typ, _ := p["type"].(string)
			skipped = append(skipped, fmt.Sprintf("%s: unsupported or incomplete type %q", source, typ))
			return
		}
		// sing-box tags must be unique within the document.
		if n, ok := usedTags[tag]; ok {
			usedTags[tag] = n + 1
			tag = fmt.Sprintf("%s-%d", tag, n+1)
			ob["tag"] = tag
		} else {
			usedTags[tag] = 1
		}
		outbounds = append(outbounds, ob)
		tagNames = append(tagNames, tag)
	}

	for _, listener := range listeners {
		creds := filtered[listener.ID]
		host := ResolveListenerServer(config.GlobalConfig, req, listener)
		if host == "" {
			host = serverHost
		}
		proxies, err := listenerToProxies(listener, host, creds)
		if err != nil || len(proxies) == 0 {
			// Same recovery as Clash YAML: Share ClientYAML uses export TLS profile.
			pcreds := make([]protocol.UserCred, 0, len(creds))
			for _, c := range creds {
				pcreds = append(pcreds, protocol.UserCred{Username: c.Username, Password: c.Password, UUID: c.UUID})
			}
			if shares, err2 := protocol.ExportShares(listener, host, pcreds); err2 == nil {
				for _, sh := range shares {
					proxies = append(proxies, proxiesFromClientYAML(sh.ClientYAML, listener.Name, 0)...)
				}
			}
			if len(proxies) == 0 {
				if err != nil {
					skipped = append(skipped, fmt.Sprintf("%s: %v", listener.Name, err))
				} else {
					skipped = append(skipped, fmt.Sprintf("%s: empty mihomo export", listener.Name))
				}
				continue
			}
		}
		for _, p := range proxies {
			appendProxy(p, listener.Name)
		}
	}

	// Parity with Clash/v2ray subscriptions: remote mirrors + external links.
	if mirrors, mErr := loadBoundRemoteMirrors(db, pu.ID); mErr == nil && len(mirrors) > 0 {
		var remoteProxies []map[string]interface{}
		remoteProxies, _ = appendRemoteProxyMaps(mirrors, remoteProxies, nil)
		for _, p := range remoteProxies {
			name, _ := p["name"].(string)
			if name == "" {
				name = "remote"
			}
			appendProxy(p, name)
		}
	}
	if strings.TrimSpace(pu.ExternalLinks) != "" {
		var ext []map[string]interface{}
		ext, _ = mergeExternalSubscriptionLinks(pu.ExternalLinks, ext, nil)
		for _, p := range ext {
			name, _ := p["name"].(string)
			if name == "" {
				name = "external"
			}
			appendProxy(p, name)
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
	doc := buildSingboxSubscriptionDoc(outbounds)
	return json.MarshalIndent(doc, "", "  ")
}

// buildSingboxSubscriptionDoc wraps outbounds with a default TUN inbound and
// route so official SFI/SFM clients create a system VPN interface. Without
// inbounds, outbounds alone never capture traffic (issue #88).
//
// Default split: private + geosite-cn + geoip-cn → direct; everything else →
// selector "proxy" (same idea as Clash GEOSITE,cn / GEOIP,CN).
func buildSingboxSubscriptionDoc(outbounds []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"log": map[string]interface{}{
			"level": "warn",
		},
		"dns": defaultSingboxDNS(),
		"inbounds": []map[string]interface{}{
			defaultSingboxTUNInbound(),
		},
		"outbounds": outbounds,
		"route": map[string]interface{}{
			"rules": []map[string]interface{}{
				{"action": "sniff"},
				{
					"type":   "logical",
					"mode":   "or",
					"action": "hijack-dns",
					"rules": []map[string]interface{}{
						{"protocol": "dns"},
						{"port": 53},
					},
				},
				{"ip_is_private": true, "action": "route", "outbound": "direct"},
				{"rule_set": "geosite-cn", "action": "route", "outbound": "direct"},
				{"rule_set": "geoip-cn", "action": "route", "outbound": "direct"},
			},
			"rule_set":              defaultSingboxCNRuleSets(),
			"final":                 "proxy",
			"auto_detect_interface": true,
			"default_domain_resolver": map[string]interface{}{
				"server": "local",
			},
		},
	}
}

// defaultSingboxCNRuleSets downloads official binary rule-sets (sing-box 1.8+).
// download_detour=direct so first fetch works before proxy is up.
func defaultSingboxCNRuleSets() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"tag":             "geosite-cn",
			"type":            "remote",
			"format":          "binary",
			"url":             "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
			"download_detour": "direct",
		},
		{
			"tag":             "geoip-cn",
			"type":            "remote",
			"format":          "binary",
			"url":             "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs",
			"download_detour": "direct",
		},
	}
}

// defaultSingboxDNS uses the post-1.12 server object format (type/tag/server)
// and avoids deprecated DNS rule address filters (ip_is_private / ip_cidr without
// match_response — removed path in 1.16). Private destinations are handled by
// route rules instead. See https://sing-box.sagernet.org/migration/
func defaultSingboxDNS() map[string]interface{} {
	return map[string]interface{}{
		"servers": []map[string]interface{}{
			{
				"type":   "local",
				"tag":    "local",
				"detour": "direct",
			},
			{
				"type":   "https",
				"tag":    "remote",
				"server": "8.8.8.8",
				"detour": "proxy",
			},
		},
		"final":    "remote",
		"strategy": "prefer_ipv4",
	}
}

// defaultSingboxTUNInbound follows current Tun schema:
// - address (not inet4_address / inet6_address, removed in 1.12)
// - no stack (deprecated 1.15, removed in 1.17 — client uses sing-tun default)
// - no inbound sniff / domain_strategy (removed in 1.13; use route rule actions)
// - dns_mode hijack (1.14+) with route hijack-dns rules
// https://sing-box.sagernet.org/configuration/inbound/tun/
func defaultSingboxTUNInbound() map[string]interface{} {
	return map[string]interface{}{
		"type":           "tun",
		"tag":            "tun-in",
		"interface_name": "singbox_tun",
		"address":        []string{"172.18.0.1/30"},
		"mtu":            1500,
		"auto_route":     true,
		"route_exclude_address": []string{
			"10.0.0.0/8",
			"100.64.0.0/10",
			"169.254.0.0/16",
			"172.16.0.0/12",
			"192.0.0.0/24",
			"192.168.0.0/16",
		},
		"strict_route": true,
		"dns_mode":     "hijack",
	}
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
		if pw := firstString(p["password"], p["auth"]); pw != "" {
			ob["password"] = pw
		} else if p["password"] != nil {
			ob["password"] = p["password"]
		}
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
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	case string:
		var x int
		fmt.Sscanf(strings.TrimSpace(n), "%d", &x)
		return x
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return int(i)
		}
		f, err := n.Float64()
		if err == nil {
			return int(f)
		}
		return 0
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
