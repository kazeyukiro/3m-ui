package converter

import (
	"net/http"
	"strings"
	"testing"
)

// sing-box 真实客户端（1.14.3 stable）实测暴露的导出兼容性回归：
// 官方文档 https://sing-box.sagernet.org （outbound/snell、outbound/tuic、
// outbound/shadowsocks、shared/v2ray-transport）是唯一事实来源。
//
//  1. snell: 扁平 obfs_mode/obfs_host（v4），v6 用 mode；不是 obfs 对象
//  2. tuic: 官方无 token 字段；v4-token 节点（无 uuid）必须跳过
//  3. shadowsocks: plugin 仅 obfs-local/v2ray-plugin，plugin_opts 是字符串
//  4. xhttp 等 sing-box 尚不支持的传输：整节点跳过（否则客户端配置解析失败）
func TestSingboxSnellFlatObfsFields(t *testing.T) {
	p := map[string]interface{}{
		"type": "snell", "name": "snell", "server": "127.0.0.1", "port": 20022,
		"psk": "snell-psk-123", "version": 4,
		"obfs-opts": map[string]interface{}{"mode": "http", "host": "cdn.example.com"},
	}
	ob, _ := mihomoProxyToSingbox(p)
	if ob == nil {
		t.Fatal("snell must be exported to sing-box subscription (1.14+ native)")
	}
	if v, _ := ob["obfs_mode"].(string); v != "http" {
		t.Errorf("obfs_mode = %q, want http (flat field per official docs)", v)
	}
	if v, _ := ob["obfs_host"].(string); v != "cdn.example.com" {
		t.Errorf("obfs_host = %q, want cdn.example.com", v)
	}
	if _, bad := ob["obfs"]; bad {
		t.Errorf("obfs object must not be emitted: %v", ob["obfs"])
	}
	if v, _ := ob["psk"].(string); v != "snell-psk-123" {
		t.Errorf("psk = %q, want snell-psk-123", v)
	}
}

func TestSingboxTuicTokenSkipped(t *testing.T) {
	// v4 token 认证：官方 tuic outbound 无 token 字段且 uuid 必填 → 跳过
	p := map[string]interface{}{
		"type": "tuic", "name": "tuic-v4", "server": "127.0.0.1", "port": 20019,
		"token": []string{"token-abc-123"},
	}
	ob, _ := mihomoProxyToSingbox(p)
	if ob != nil {
		t.Fatalf("tuic v4 token node must be skipped (sing-box has no token field): %v", ob)
	}
}

func TestSingboxTuicV5UUIDExported(t *testing.T) {
	// v5 uuid+password：正常导出
	p := map[string]interface{}{
		"type": "tuic", "name": "tuic-v5", "server": "127.0.0.1", "port": 20020,
		"uuid": "c5d1300d-dd7d-4b54-a194-2b2b7332986c", "password": "pw",
	}
	ob, tag := mihomoProxyToSingbox(p)
	if ob == nil {
		t.Fatal("tuic v5 (uuid+password) must be exported")
	}
	if tag != "tuic-v5" {
		t.Errorf("tag = %q", tag)
	}
	if v, _ := ob["uuid"].(string); v != "c5d1300d-dd7d-4b54-a194-2b2b7332986c" {
		t.Errorf("uuid = %q", v)
	}
	if _, bad := ob["token"]; bad {
		t.Errorf("token must not be emitted: %v", ob["token"])
	}
}

func TestSingboxSSPluginOptsString(t *testing.T) {
	p := map[string]interface{}{
		"type": "shadowsocks", "name": "ss-obfs", "server": "127.0.0.1", "port": 20017,
		"cipher": "aes-128-gcm", "password": "4p2YYLesNRMKJNfC",
		"plugin":      "obfs",
		"plugin-opts": map[string]interface{}{"mode": "http", "host": "cdn.example.com"},
	}
	ob, _ := mihomoProxyToSingbox(p)
	if ob == nil {
		t.Fatal("ss must be exported")
	}
	if v, _ := ob["plugin"].(string); v != "obfs-local" {
		t.Errorf("plugin = %q, want obfs-local (sing-box only knows obfs-local/v2ray-plugin)", v)
	}
	opts, ok := ob["plugin_opts"].(string)
	if !ok {
		t.Fatalf("plugin_opts must be a string, got %T (%v)", ob["plugin_opts"], ob["plugin_opts"])
	}
	if !strings.Contains(opts, "obfs=http") || !strings.Contains(opts, "obfs-host=cdn.example.com") {
		t.Errorf("plugin_opts = %q, want SIP003 string obfs=http;obfs-host=cdn.example.com", opts)
	}
}

func TestSingboxUnsupportedTransportSkipped(t *testing.T) {
	// xhttp：sing-box 官方 shared/v2ray-transport 文档无此类型（ws/http/
	// httpupgrade/grpc/quic）→ 整节点跳过，否则客户端整份配置解析失败。
	base := map[string]interface{}{
		"type": "vless", "name": "less-xhttp", "server": "127.0.0.1", "port": 20014,
		"uuid": "c5d1300d-dd7d-4b54-a194-2b2b7332986c", "network": "xhttp", "flow": "",
	}
	ob, _ := mihomoProxyToSingbox(base)
	if ob != nil {
		t.Errorf("xhttp node must be skipped (sing-box has no xhttp transport): %v", ob)
	}
	// ws/grpc 必须保留
	ws := map[string]interface{}{}
	for k, v := range base {
		ws[k] = v
	}
	ws["network"] = "ws"
	if ob, _ := mihomoProxyToSingbox(ws); ob == nil {
		t.Error("ws node must be exported")
	}
	g := map[string]interface{}{}
	for k, v := range base {
		g[k] = v
	}
	g["network"] = "grpc"
	if ob, _ := mihomoProxyToSingbox(g); ob == nil {
		t.Error("grpc node must be exported")
	}
}

func TestSingboxPluginAndOptsConversion(t *testing.T) {
	plugin, opts := singboxPluginAndOpts("obfs", map[string]interface{}{
		"mode": "http", "host": "cdn.example.com"})
	if plugin != "obfs-local" {
		t.Errorf("plugin alias obfs must map to obfs-local, got %q", plugin)
	}
	if opts != "obfs=http;obfs-host=cdn.example.com" {
		t.Errorf("opts = %q, want obfs=http;obfs-host=cdn.example.com", opts)
	}
	plugin2, opts2 := singboxPluginAndOpts("v2ray-plugin", map[string]interface{}{
		"mode": "websocket", "host": "cdn.example.com", "path": "/ws", "tls": true})
	if plugin2 != "v2ray-plugin" || opts2 != "mode=websocket;host=cdn.example.com;path=/ws;tls=1" {
		t.Errorf("v2ray-plugin: %q %q", plugin2, opts2)
	}
	if !singboxTransportSupported("ws") || !singboxTransportSupported("grpc") ||
		!singboxTransportSupported("http") || !singboxTransportSupported("httpupgrade") ||
		!singboxTransportSupported("quic") {
		t.Error("official v2ray-transport types must be supported")
	}
	if singboxTransportSupported("xhttp") || singboxTransportSupported("kcp-tun") {
		t.Error("xhttp/kcp-tun must not be claimed supported")
	}
}

// TestSingboxSubscriptionDocIs1_12Compatible guards the decode compatibility
// floor. Real binaries were used to establish it:
//
//	sing-box 1.12.8 check → unknown field "http_clients"   (before)
//	sing-box 1.13.4 check → unknown field "http_clients"   (before)
//	sing-box 1.14.4 check → parses                          (before)
//
// A rejected document leaves the client with an empty profile and no visible
// reason, which is what subscribers report as "import does nothing". 1.12/1.13
// also reject `route.rule_set[].tag` as an array, and a literal `{tag}` URL
// never resolves, so each tag must be its own rule-set entry.
func TestSingboxSubscriptionDocIs1_12Compatible(t *testing.T) {
	doc := buildSingboxSubscriptionDoc([]map[string]interface{}{
		{"type": "direct", "tag": "direct"},
		{"type": "selector", "tag": "proxy", "outbounds": []string{"n1"}, "default": "n1"},
		{"type": "vless", "tag": "n1", "server": "example.com", "server_port": 443},
	})

	// 1.14-only top-level / route / tun fields must be absent.
	if _, ok := doc["http_clients"]; ok {
		t.Error("http_clients: sing-box 1.14+ only, 1.12/1.13 reject it")
	}
	route := doc["route"].(map[string]interface{})
	for _, k := range []string{"default_http_client", "find_neighbor", "dhcp_lease_files"} {
		if _, ok := route[k]; ok {
			t.Errorf("route.%s: sing-box 1.14+ only, 1.12/1.13 reject it", k)
		}
	}
	inbound := doc["inbounds"].([]map[string]interface{})[0]
	for _, k := range []string{"dns_mode", "dns_address"} {
		if _, ok := inbound[k]; ok {
			t.Errorf("tun.%s: sing-box 1.14+ only, 1.12/1.13 reject it", k)
		}
	}

	// Every rule_set carries a string tag and a concrete URL.
	sets := route["rule_set"].([]map[string]interface{})
	if len(sets) == 0 {
		t.Fatal("no rule_set entries")
	}
	seen := map[string]bool{}
	for _, rs := range sets {
		tag, ok := rs["tag"].(string)
		if !ok || tag == "" {
			t.Fatalf("rule_set tag must be a non-empty string on 1.12/1.13: %#v", rs["tag"])
		}
		if seen[tag] {
			t.Errorf("duplicate rule_set tag %q", tag)
		}
		seen[tag] = true
		if u, _ := rs["url"].(string); strings.Contains(u, "{tag}") {
			t.Errorf("rule_set %q keeps a literal {tag} placeholder: %s", tag, u)
		}
	}
	for _, tag := range []string{"apple@cn", "microsoft@cn", "category-games@cn", "cn", "cn_ip"} {
		if !seen[tag] {
			t.Errorf("rule_set %q missing", tag)
		}
	}
}

// testSingboxOutbounds is a minimal set of outbounds for document-shape tests.
func testSingboxOutbounds() []map[string]interface{} {
	return []map[string]interface{}{
		{"type": "vless", "tag": "n1", "server": "example.com", "server_port": 443, "uuid": "11111111-2222-3333-4444-555555555555"},
		{"type": "direct", "tag": "direct"},
		{"type": "selector", "tag": "proxy", "outbounds": []string{"n1"}, "default": "n1"},
	}
}

// TestSingboxModernDocDeclaresExplicitHTTPClient covers the 1.14+ style.
//
// sing-box 1.14.0 deprecated the implicit default HTTP client used to download
// remote rule-sets and removes it in 1.16.0:
//
//	WARN implicit default HTTP client using default outbound for remote rule-sets
//	     is deprecated in sing-box 1.14.0 and will be removed in sing-box 1.16.0.
//
// The modern document declares `http_clients` + `route.default_http_client` so
// that warning disappears. `detour` must name the proxy selector: a `direct`
// outbound without dial options is rejected at startup with "detour to an empty
// direct outbound makes no sense", and the selector reproduces the download
// path the implicit client used.
func TestSingboxModernDocDeclaresExplicitHTTPClient(t *testing.T) {
	doc := buildSingboxDoc(testSingboxOutbounds(), singboxDocModern)

	clients, ok := doc["http_clients"].([]map[string]interface{})
	if !ok || len(clients) != 1 {
		t.Fatalf("http_clients must be a one-element array, got %#v", doc["http_clients"])
	}
	client := clients[0]
	if client["tag"] != "default" {
		t.Errorf("http_clients[0].tag = %#v, want \"default\"", client["tag"])
	}
	if client["version"] != 2 {
		t.Errorf("http_clients[0].version = %#v, want 2 (documented default, HTTP/2)", client["version"])
	}
	if client["detour"] != "proxy" {
		t.Errorf("http_clients[0].detour = %#v, want \"proxy\"", client["detour"])
	}
	// Only documented HTTP client fields may appear (engine/tag/version/dial).
	for key := range client {
		switch key {
		case "tag", "version", "detour":
		default:
			t.Errorf("unexpected http_clients[0] field %q", key)
		}
	}
	route := doc["route"].(map[string]interface{})
	if route["default_http_client"] != "default" {
		t.Errorf("route.default_http_client = %#v, want \"default\"", route["default_http_client"])
	}
	// 1.15-only fields must not leak in either style.
	inbound := doc["inbounds"].([]map[string]interface{})[0]
	if _, ok := inbound["stack"]; ok {
		t.Error("tun.stack: deprecated in sing-box 1.15.0, must not be emitted")
	}
}

// TestSingboxDocStyleFromRequest pins the version detection that decides which
// document a client receives. A wrong guess is a hard failure, not a cosmetic
// one: 1.12/1.13 reject `http_clients` outright, and 1.16+ drops the implicit
// client the legacy document relies on.
func TestSingboxDocStyleFromRequest(t *testing.T) {
	cases := []struct {
		name  string
		ua    string
		query string
		want  singboxDocStyle
	}{
		{"SFA 1.14", "SFA (sing-box 1.14.4; language zh_CN)", "", singboxDocModern},
		{"SFA 1.15", "SFA (sing-box 1.15.0; language zh_CN)", "", singboxDocModern},
		{"SFA 2.x", "SFA (sing-box 2.0.0; language en_US)", "", singboxDocModern},
		{"SFA 1.13", "SFA (sing-box 1.13.4; language zh_CN)", "", singboxDocLegacy},
		{"SFA 1.12", "SFA (sing-box 1.12.8; language zh_CN)", "", singboxDocLegacy},
		{"libbox slash", "sing-box/1.14.4", "", singboxDocModern},
		{"libbox v-prefix", "sing-box v1.14.0", "", singboxDocModern},
		{"SFI bracket", "SFI/1.14.4 (sing-box 1.14.4)", "", singboxDocModern},
		{"no version at all", "SFA", "", singboxDocLegacy},
		{"empty", "", "", singboxDocLegacy},
		{"unrelated client", "clash-verge/v2.0.0", "", singboxDocLegacy},
		{"query forces modern", "SFA (sing-box 1.13.4)", "?sbver=1.14", singboxDocModern},
		{"query forces legacy", "SFA (sing-box 1.14.4)", "?sbver=1.13", singboxDocLegacy},
		{"query garbage falls back to UA", "SFA (sing-box 1.14.4)", "?sbver=abc", singboxDocModern},
		{"query garbage without UA", "SFA", "?sbver=abc", singboxDocLegacy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://panel.local/sub/token"+tc.query, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if tc.ua != "" {
				req.Header.Set("User-Agent", tc.ua)
			}
			if got := singboxDocStyleForRequest(req); got != tc.want {
				t.Fatalf("style = %v (%s), want %v (%s)", got, styleName(got), tc.want, styleName(tc.want))
			}
		})
	}
}

func styleName(s singboxDocStyle) string {
	if s == singboxDocModern {
		return "modern"
	}
	return "legacy"
}
