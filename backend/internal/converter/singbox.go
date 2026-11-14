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
	if err != nil && !strings.Contains(err.Error(), "not bound") && !strings.Contains(err.Error(), "no exportable") {
		return nil, err
	}
	if err != nil {
		// A user may hold only remote cluster mirrors (no local listener).
		// The mirror merge below supplies the outbounds; bail out later if empty.
		listeners, filtered = nil, map[uint][]user.Credential{}
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
	// Outbounds: proxy selector + direct (CN/overseas split via remote rule-sets).
	outbounds = append(outbounds,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{
			"type":      "selector",
			"tag":       "proxy",
			"outbounds": append([]string{}, tagNames...),
			"default":   tagNames[0],
		},
	)
	style := singboxDocStyleForRequest(req)
	if style == singboxDocModern {
		log.Printf("sing-box subscription: user %s reports core >= 1.14, emitting explicit remote rule-set HTTP client", pu.Username)
	}
	doc := buildSingboxDoc(outbounds, style)
	return json.MarshalIndent(doc, "", "  ")
}

// singboxDocStyle selects the field set that matches the requesting core.
//
// The two styles exist because sing-box 1.12/1.13 hard-reject a document that
// carries 1.14 fields (`json: unknown field`), while 1.14+ deprecates the
// implicit default HTTP client used for remote rule-sets and removes it in
// 1.16.0. There is no single document that satisfies both, so the style is
// chosen from the client's core version.
type singboxDocStyle int

const (
	// singboxDocLegacy omits every 1.14-only field: playable on 1.12/1.13.
	// Remote rule-sets fall back to the implicit default HTTP client, which
	// sing-box warns about on 1.14+.
	singboxDocLegacy singboxDocStyle = iota
	// singboxDocModern declares the remote rule-set HTTP client explicitly,
	// which is what 1.14+ requires once the implicit behaviour is gone.
	singboxDocModern
)

// singboxModernCoreMinor is the first 1.14 release line. `http_clients` and
// `route.default_http_client` were both introduced in sing-box 1.14.0.
const singboxModernCoreMinor = 14

// singboxDocStyleForRequest picks the document style for an incoming request.
//
// Priority: explicit query override, then the core version advertised in the
// User-Agent, then the conservative legacy style. SFA sends
// "SFA (sing-box 1.14.4; language zh_CN)"; libbox defaults to
// "sing-box/<version>". Unknown or version-less agents keep the legacy style so
// that older cores are never handed a document they cannot decode.
func singboxDocStyleForRequest(req *http.Request) singboxDocStyle {
	if req == nil {
		return singboxDocLegacy
	}
	if v := strings.TrimSpace(req.URL.Query().Get("sbver")); v != "" {
		if major, minor, ok := parseSingboxVersion(v); ok {
			if major > 1 || (major == 1 && minor >= singboxModernCoreMinor) {
				return singboxDocModern
			}
			return singboxDocLegacy
		}
	}
	if major, minor, ok := singboxVersionFromUA(req.UserAgent()); ok {
		if major > 1 || (major == 1 && minor >= singboxModernCoreMinor) {
			return singboxDocModern
		}
	}
	return singboxDocLegacy
}

// singboxVersionFromUA extracts the core version advertised in a User-Agent.
// It only accepts a version that directly follows "sing-box", so unrelated
// numbers elsewhere in the string are not mistaken for the core version.
func singboxVersionFromUA(ua string) (major, minor int, ok bool) {
	lower := strings.ToLower(ua)
	for _, marker := range []string{"sing-box", "singbox"} {
		idx := strings.Index(lower, marker)
		if idx < 0 {
			continue
		}
		rest := lower[idx+len(marker):]
		rest = strings.TrimLeft(rest, " /v")
		if major, minor, ok = parseSingboxVersion(rest); ok {
			return major, minor, true
		}
	}
	return 0, 0, false
}

// parseSingboxVersion reads a leading "<major>.<minor>" pair.
func parseSingboxVersion(s string) (major, minor int, ok bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		major = major*10 + int(s[i]-'0')
		i++
	}
	if i == 0 || i >= len(s) || s[i] != '.' {
		return 0, 0, false
	}
	i++
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		minor = minor*10 + int(s[i]-'0')
		i++
	}
	if i == start {
		return 0, 0, false
	}
	return major, minor, true
}

// buildSingboxSubscriptionDoc renders the legacy (1.12+) document. See
// buildSingboxDoc for the compatibility floor and the reason two styles exist.
func buildSingboxSubscriptionDoc(outbounds []map[string]interface{}) map[string]interface{} {
	return buildSingboxDoc(outbounds, singboxDocLegacy)
}

// buildSingboxDoc renders a CN/overseas split config with jsDelivr remote
// rule-sets (no panel hosting).
//
// Compatibility floor is sing-box 1.12.0, verified by running `sing-box check`
// against real 1.12.8 / 1.13.4 / 1.14.4 builds. `dns.servers[].predefined` is
// 1.12+, which is why 1.11 cannot be supported without the removed legacy DNS
// format.
//
// Two styles are needed because 1.12/1.13 reject a document containing 1.14
// fields outright, while 1.14 deprecates the implicit default HTTP client used
// to download remote rule-sets and removes it in 1.16.0:
//
//	legacy → no `http_clients` / `route.default_http_client`; 1.14 logs
//	         "implicit default HTTP client ... is deprecated".
//	modern → both fields declared; verified to start on 1.14.4 with no warning.
//
// `detour` names the proxy selector rather than `direct`, because a `direct`
// outbound without dial options is rejected with "detour to an empty direct
// outbound makes no sense"; pointing at the selector also reproduces exactly
// what the implicit client did (download through the final outbound), so the
// modern document changes no behaviour.
func buildSingboxDoc(outbounds []map[string]interface{}, style singboxDocStyle) map[string]interface{} {
	route := map[string]interface{}{
		"rules":                   defaultSingboxRouteRules(),
		"rule_set":                defaultSingboxRemoteRuleSets(),
		"final":                   "proxy",
		"auto_detect_interface":   true,
		"default_domain_resolver": "ali",
	}
	doc := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "warn",
		},
		"dns": defaultSingboxDNS(),
		"inbounds": []map[string]interface{}{
			defaultSingboxTUNInbound(),
		},
		"outbounds": outbounds,
		"route":     route,
	}
	if style == singboxDocModern {
		doc["http_clients"] = defaultSingboxHTTPClients()
		route["default_http_client"] = "default"
	}
	return doc
}

// defaultSingboxHTTPClients declares the HTTP client sing-box uses to fetch
// remote rule-sets (1.14.0+). Only documented fields are emitted: `tag` plus
// `version` (HTTP/2 is the documented default) and `detour` (a dial field).
func defaultSingboxHTTPClients() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"tag":     "default",
			"version": 2,
			"detour":  "proxy",
		},
	}
}

// defaultSingboxRemoteRuleSets — MetaCubeX geosite/geoip via jsDelivr.
//
// One rule_set per tag: `route.rule_set[].tag` is a string in every sing-box
// release up to 1.13, so the multi-tag form (`"tag": [...]` plus a `{tag}`
// placeholder in `url`) is rejected there — and a literal `{tag}` URL never
// resolves at runtime. Emitting one entry per tag keeps the document valid from
// 1.12 through 1.14+ while producing the same five rule-sets.
func defaultSingboxRemoteRuleSets() []map[string]interface{} {
	geosite := []string{"apple@cn", "microsoft@cn", "category-games@cn", "cn"}
	out := make([]map[string]interface{}, 0, len(geosite)+1)
	for _, tag := range geosite {
		out = append(out, map[string]interface{}{
			"type":            "remote",
			"tag":             tag,
			"url":             "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/" + tag + ".srs",
			"update_interval": "48h",
		})
	}
	out = append(out, map[string]interface{}{
		"type":            "remote",
		"tag":             "cn_ip",
		"format":          "binary",
		"url":             "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/cn.srs",
		"update_interval": "48h",
	})
	return out
}

// defaultSingboxDNS — CN domains → AliDNS; else Google DoH via proxy (template simplified).
func defaultSingboxDNS() map[string]interface{} {
	return map[string]interface{}{
		"servers": []map[string]interface{}{
			{
				"type": "hosts",
				"tag":  "hosts",
				"predefined": map[string]interface{}{
					"dns.alidns.com": []string{"223.5.5.5", "223.6.6.6"},
					"dns.google":     []string{"8.8.8.8", "8.8.4.4"},
				},
			},
			{
				"tag":             "google",
				"type":            "https",
				"server":          "dns.google",
				"domain_resolver": "hosts",
				"detour":          "proxy",
			},
			{
				"tag":             "ali",
				"type":            "https",
				"server":          "dns.alidns.com",
				"domain_resolver": "hosts",
			},
		},
		"rules": []map[string]interface{}{
			{
				"rule_set": []string{"apple@cn", "microsoft@cn", "category-games@cn", "cn"},
				"action":   "route",
				"server":   "ali",
			},
			{"action": "route", "server": "google"},
		},
		"final": "google",
	}
}

// defaultSingboxRouteRules — CN geosite/geoip → direct; rest → proxy (template core path).
func defaultSingboxRouteRules() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type":   "logical",
			"mode":   "or",
			"action": "hijack-dns",
			"rules": []map[string]interface{}{
				{"port": 53},
				{"protocol": "dns"},
			},
		},
		{"ip_is_private": true, "action": "route", "outbound": "direct"},
		{"action": "sniff", "timeout": "100ms"},
		{"domain_suffix": []string{"googleapis.cn"}, "action": "route", "outbound": "proxy"},
		{
			"rule_set": []string{"apple@cn", "microsoft@cn", "category-games@cn", "cn"},
			"action":   "route",
			"outbound": "direct",
		},
		{"action": "resolve"},
		{"rule_set": "cn_ip", "action": "route", "outbound": "direct"},
	}
}

func defaultSingboxTUNInbound() map[string]interface{} {
	return map[string]interface{}{
		"type":           "tun",
		"tag":            "tun-in",
		"interface_name": "singbox_tun",
		"address":        []string{"172.19.0.1/30"},
		"mtu":            9000,
		"auto_route":     true,
		"strict_route":   true,
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
			// sing-box 官方文档 (shadowsocks outbound)：plugin 仅支持
			// obfs-local / v2ray-plugin，且 plugin_opts 是 ";" 连接的字符串，
			// 不是 Clash 风格的 map。
			if sbPlugin, sbOpts := singboxPluginAndOpts(plugin, p["plugin-opts"]); sbPlugin != "" {
				ob["plugin"] = sbPlugin
				if sbOpts != "" {
					ob["plugin_opts"] = sbOpts
				}
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
		// sing-box 官方文档 (tuic outbound)：仅有 uuid/password 等字段，
		// 没有 token——v4 token 认证的节点 sing-box 无法使用，跳过导出。
		uuid, _ := p["uuid"].(string)
		if strings.TrimSpace(uuid) == "" {
			return nil, ""
		}
		ob["uuid"] = strings.TrimSpace(uuid)
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
	case "snell":
		// sing-box 官方文档 (snell outbound, 1.14.0+)：扁平字段
		// obfs_mode ("none"/"http") / obfs_host，不是 obfs 对象；
		// v6 使用 mode 字段（面板暂只导 v4 HTTP 混淆）。
		if psk := firstString(p["psk"], p["password"]); psk != "" {
			ob["psk"] = psk
		}
		if v, ok := p["version"]; ok {
			ver := toInt(v)
			ob["version"] = ver
			if ver == 6 {
				if mode, _ := p["obfs-opts"].(map[string]interface{})["mode"].(string); mode != "" {
					ob["mode"] = mode
				}
			} else if obfsOpts, ok := p["obfs-opts"].(map[string]interface{}); ok && len(obfsOpts) > 0 {
				if mode, ok := obfsOpts["mode"].(string); ok && mode != "" {
					ob["obfs_mode"] = mode
				}
				if host, ok := obfsOpts["host"].(string); ok && host != "" {
					ob["obfs_host"] = host
				}
			}
		}
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
		// sing-box 官方文档 (shared/v2ray-transport) 仅支持
		// ws/http/httpupgrade/grpc/quic；XHTTP 等 sing-box 尚不支持的
		// 传输若写入会导致整份配置解析失败——整节点跳过导出
		// （Clash 订阅仍保留该节点供 mihomo 使用）。
		if !singboxTransportSupported(network) {
			return nil, ""
		}
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

// singboxPluginAndOpts 将 Clash 风格的 plugin/plugin-opts 转换为 sing-box
// 官方文档 (shadowsocks outbound) 要求的 SIP003 字符串形式：
//
//	plugin ∈ {obfs-local, v2ray-plugin}，plugin_opts 为 ";" 连接的 k=v 串，
//	例如 "obfs=http;obfs-host=cdn.example.com"。
func singboxPluginAndOpts(plugin string, opts interface{}) (string, string) {
	plugin = strings.ToLower(strings.TrimSpace(plugin))
	if plugin == "obfs" {
		// Clash/mihomo 旧简写，sing-box 只认 obfs-local。
		plugin = "obfs-local"
	}
	m, _ := opts.(map[string]interface{})
	str := func(k string) string {
		s, _ := m[k].(string)
		return strings.TrimSpace(s)
	}
	switch plugin {
	case "obfs-local":
		parts := make([]string, 0, 2)
		if mode := str("mode"); mode != "" {
			parts = append(parts, "obfs="+mode)
		}
		if host := str("host"); host != "" {
			parts = append(parts, "obfs-host="+host)
		}
		return plugin, strings.Join(parts, ";")
	case "v2ray-plugin":
		parts := make([]string, 0, 4)
		if mode := str("mode"); mode != "" {
			parts = append(parts, "mode="+mode)
		}
		if host := str("host"); host != "" {
			parts = append(parts, "host="+host)
		}
		if path := str("path"); path != "" {
			parts = append(parts, "path="+path)
		}
		if tls, _ := m["tls"].(bool); tls {
			parts = append(parts, "tls=1")
		}
		return plugin, strings.Join(parts, ";")
	}
	return plugin, ""
}

// singboxTransportSupported 对照 sing-box 官方 shared/v2ray-transport 文档：
// ws / http / httpupgrade / grpc / quic。XHTTP 等 sing-box 尚不支持的
// 传输名（如 xhttp）返回 false。
func singboxTransportSupported(network string) bool {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "ws", "websocket", "http", "httpupgrade", "grpc", "quic":
		return true
	}
	return false
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
	lower := strings.ToLower(t)
	switch lower {
	case "ss", "shadowsocks":
		return "shadowsocks"
	case "hysteria2":
		return "hysteria2"
	case "socks", "socks5":
		return "socks"
	case "mieru", "sudoku", "shadowquic", "trusttunnel":
		// No stable 1:1 outbound in stock sing-box for these panel types.
		// NOTE: snell DOES have a native sing-box outbound — it falls through
		// the default branch and is handled by the snell case above (P2-2).
		return ""
	default:
		return lower
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
