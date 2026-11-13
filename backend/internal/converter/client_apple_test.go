package converter

import (
	"net/url"
	"strings"
	"testing"
)

func TestParseShareURIHysteria2(t *testing.T) {
	n, err := parseShareURI("hysteria2://CAikPKr_Q41vRdXNpDQwX3wL@85.149.212.214:13290#%E8%8A%82%E7%82%B92")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Protocol != "hysteria2" || n.Password != "CAikPKr_Q41vRdXNpDQwX3wL" {
		t.Fatalf("proto/password = %q/%q", n.Protocol, n.Password)
	}
	if n.Host != "85.149.212.214" || n.Port != "13290" {
		t.Fatalf("host/port = %q/%q", n.Host, n.Port)
	}
	if n.Name != "节点2" {
		t.Fatalf("name = %q", n.Name)
	}
}

func TestParseShareURIHysteria2InsecureObfs(t *testing.T) {
	n, err := parseShareURI("hy2://pw@hy2.example.com:443/?sni=hy2.example.com&insecure=1&allowInsecure=1&obfs=salamander&obfs-password=obfspw123&alpn=h3&down=100+Mbps")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !shareInsecure(n.Params) {
		t.Fatalf("expected insecure true")
	}
	if n.Params["obfs-password"] != "obfspw123" {
		t.Fatalf("obfs-password = %q", n.Params["obfs-password"])
	}
	if mbpsValue(n.Params["down"]) != "100" {
		t.Fatalf("mbps = %q", mbpsValue(n.Params["down"]))
	}
}

func TestParseShareURIMultiPort(t *testing.T) {
	n, err := parseShareURI("hysteria2://pw@host.example.com:443,5000-6000#hop")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Port != "443" || n.Params["mport"] != "5000-6000" {
		t.Fatalf("port/mport = %q/%q", n.Port, n.Params["mport"])
	}
}

func TestParseShareURIShadowSocks(t *testing.T) {
	n, err := parseShareURI("ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@ss.example.com:8388/?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dbing.com#ss-node")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Protocol != "ss" || n.Method != "aes-128-gcm" || n.Password != "password" {
		t.Fatalf("ss = %q/%q/%q", n.Protocol, n.Method, n.Password)
	}
	if n.Params["plugin"] == "" || !strings.Contains(n.Params["plugin"], "obfs-local") {
		t.Fatalf("plugin = %q", n.Params["plugin"])
	}
	if n.Name != "ss-node" {
		t.Fatalf("name = %q", n.Name)
	}
}

func TestParseShareURIVMess(t *testing.T) {
	// vmess://base64({"v":"2","ps":"v1","add":"vm.example.com","port":"443","id":"52396e06-041a-4cc2-be5c-8525eb457809","aid":"0","scy":"auto","net":"ws","type":"none","tls":"tls","sni":"vm.example.com","host":"cdn.example.com","path":"/ws"})
	body := "eyJ2IjoiMiIsInBzIjoidjEiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjUyMzk2ZTA2LTA0MWEtNGNjMi1iZTVjLTg1MjVlYjQ1NzgwOSIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0Ijoid3MiLCJ0eXBlIjoibm9uZSIsInRscyI6InRscyIsInNuaSI6InZtLmV4YW1wbGUuY29tIiwiaG9zdCI6ImNkbi5leGFtcGxlLmNvbSIsInBhdGgiOiIvd3MifQ=="
	n, err := parseShareURI("vmess://" + body + "#v1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Protocol != "vmess" || n.Method != "auto" || n.Password != "52396e06-041a-4cc2-be5c-8525eb457809" {
		t.Fatalf("vmess = %q/%q/%q", n.Protocol, n.Method, n.Password)
	}
	if n.Params["type"] != "ws" || n.Params["path"] != "/ws" || n.Params["host"] != "cdn.example.com" {
		t.Fatalf("ws params = %v", n.Params)
	}
	if n.Params["security"] != "tls" || n.Params["sni"] != "vm.example.com" {
		t.Fatalf("tls params = %v", n.Params)
	}
}

func TestParseShareURIVLESSReality(t *testing.T) {
	n, err := parseShareURI("vless://52396e06-041a-4cc2-be5c-8525eb457809@vless.example.com:443?security=reality&pbk=PUBKEY&sid=4f3a&flow=xtls-rprx-vision&sni=www.example.com#reality")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Protocol != "vless" || n.Password != "52396e06-041a-4cc2-be5c-8525eb457809" {
		t.Fatalf("vless = %q/%q", n.Protocol, n.Password)
	}
	if n.Params["pbk"] != "PUBKEY" || n.Params["sid"] != "4f3a" || n.Params["flow"] != "xtls-rprx-vision" {
		t.Fatalf("reality params = %v", n.Params)
	}
}

func TestParseShareURIIPv6(t *testing.T) {
	n, err := parseShareURI("hysteria2://pw@[2001:db8::1]:443?insecure=1#v6")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Host != "2001:db8::1" || n.Port != "443" {
		t.Fatalf("v6 host/port = %q/%q", n.Host, n.Port)
	}
}

func TestRenderLoonHysteria2Salamander(t *testing.T) {
	n, _ := parseShareURI("hy2://pw@85.149.212.214:13290?sni=cdn.example.com&insecure=1&obfs=salamander&obfs-password=obfspw123&alpn=h3&down=200+Mbps#loon-node")
	got := renderLoonNode(n)
	// Real-world Loon: server-ports must be present even without port hopping,
	// so a single-port node renders server-ports="<main port>".
	want := `loon-node = Hysteria2,85.149.212.214,13290,"pw",sni=cdn.example.com,skip-cert-verify=true,alpn=h3,salamander-password=obfspw123,server-ports="13290",download-bandwidth=200,udp=true,block-quic=false`
	if got != want {
		t.Fatalf("loon line:\n got %q\nwant %q", got, want)
	}
}

func TestRenderLoonHysteria2ServerPortsSinglePort(t *testing.T) {
	// No mport: server-ports falls back to the main port — Loon's Hysteria 2
	// plugin does not dial a node without the field.
	n, _ := parseShareURI("hy2://pw@85.149.212.214:13290#node")
	got := renderLoonNode(n)
	if !strings.Contains(got, `server-ports="13290"`) {
		t.Fatalf("loon server-ports must always be rendered, got %q", got)
	}
	if !strings.HasPrefix(got, `node = Hysteria2,85.149.212.214,13290,"pw",`) {
		t.Fatalf("loon line prefix wrong: %q", got)
	}
}

func TestRenderLoonHysteria2ServerPorts(t *testing.T) {
	// Loon documents server-ports as comma-separated entries with colon
	// ranges: server-ports="20000:20100". Dash ranges from the share URI
	// must be converted, not passed through (Surge syntax).
	n, _ := parseShareURI("hy2://pw@hy2.example.com:443,5000-6000?mport=443,5000-6000#hop")
	got := renderLoonNode(n)
	if !strings.Contains(got, `server-ports="443,5000:6000"`) {
		t.Fatalf("loon server-ports wrong format: %s", got)
	}
	if strings.Contains(got, "5000-6000") || strings.Contains(got, ";") {
		t.Fatalf("loon server-ports must not use dash/semicolon: %s", got)
	}
}

func TestRenderLoonHysteria2GeckoSkipped(t *testing.T) {
	// Loon has no gecko-password in its Hysteria 2 section: the renderer
	// emits nothing and the generator degrades it to a comment.
	n, _ := parseShareURI("hy2://pw@hy2.example.com:443?obfs=gecko&obfs-password=gpw#g")
	if got := renderLoonNode(n); got != "" {
		t.Fatalf("loon should not render gecko hy2, got %q", got)
	}
}

func TestRenderLoonAnyTLSRealitySkipped(t *testing.T) {
	// AnyTLS + REALITY is not emitted for Loon: the password-only AnyTLS form
	// is the one Loon reliably parses; reality anytls nodes degrade upstream.
	n, _ := parseShareURI("anytls://pw@at.example.com:443?security=reality&pbk=dR2euO5X-8ENLeBjO9ZfXQ2RDlg_uBEHBHBNBQTapyU&sid=a0997793b8119118&sni=www.microsoft.com#at-r")
	got := renderLoonNode(n)
	if strings.Contains(got, "public-key") || strings.Contains(got, "short-id") || strings.Contains(got, "over-tls") {
		t.Fatalf("loon anytls must not carry reality/tls args: %s", got)
	}
	if !strings.Contains(got, `AnyTLS,at.example.com,443,"pw"`) || !strings.Contains(got, "sni=www.microsoft.com") {
		t.Fatalf("loon anytls base args missing: %s", got)
	}
}

func TestRenderLoonSSPlugin(t *testing.T) {
	n, _ := parseShareURI("ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@ss.example.com:8388/?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dbing.com#ss-node")
	got := renderLoonNode(n)
	if !strings.Contains(got, "obfs-name=http") || !strings.Contains(got, "obfs-host=bing.com") {
		t.Fatalf("loon ss = %q", got)
	}
}

func TestRenderLoonVLESSReality(t *testing.T) {
	n, _ := parseShareURI("vless://uuid@vless.example.com:443?security=reality&pbk=PUBKEY&sid=4f3a&flow=xtls-rprx-vision&sni=www.example.com#r")
	got := renderLoonNode(n)
	for _, frag := range []string{"VLESS", "transport=tcp", "over-tls=true", `public-key="PUBKEY"`, "short-id=4f3a", "flow=xtls-rprx-vision", "udp=true"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("loon vless missing %q: %s", frag, got)
		}
	}
}

func TestRenderLoonHTTPTransport(t *testing.T) {
	// nsloon.app/en/docs/Node: VMess/VLESS support tcp, ws and http; "host
	// and path apply to WebSocket or HTTP transports".
	n, _ := parseShareURI("vless://uuid@vless.example.com:80?type=http&path=/h2&host=cdn.example.com#h2")
	got := renderLoonNode(n)
	for _, frag := range []string{"transport=http", "path=/h2", "host=cdn.example.com"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("loon vless http missing %q: %s", frag, got)
		}
	}
	vm, _ := parseShareURI("vmess://eyJ2IjoiMiIsInBzIjoiaDIiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI4MCIsImlkIjoiNTIzOTZlMDYtMDQxYS00Y2MyLWJlNWMtODUyNWViNDU3ODA5IiwiYWlkIjoiMCIsInNjeSI6ImF1dG8iLCJuZXQiOiJodHRwIiwidHlwZSI6Im5vbmUiLCJwYXRoIjoiL2gyIiwiaG9zdCI6ImNkbi5leGFtcGxlLmNvbSJ9#h2")
	got = renderLoonNode(vm)
	for _, frag := range []string{"VMess", "transport=http", "path=/h2", "host=cdn.example.com"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("loon vmess http missing %q: %s", frag, got)
		}
	}
}

func TestRenderLoonTrojanHTTPAsWebSocket(t *testing.T) {
	// Loon docs: trojan supports tcp and ws, "for compatibility,
	// transport=http is handled as WebSocket".
	n, _ := parseShareURI("trojan://pwd@trojan.example.com:443?type=http&path=/t&host=cdn.example.com&security=tls&sni=cdn.example.com#t")
	got := renderLoonNode(n)
	for _, frag := range []string{"Trojan", "transport=ws", "path=/t", "host=cdn.example.com", "over-tls=true"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("loon trojan http-as-ws missing %q: %s", frag, got)
		}
	}
}

func TestRenderLoonUnsupportedTransportDegraded(t *testing.T) {
	// Loon's transport vocabulary (tcp/ws/http) has no grpc / xhttp form;
	// those links used to fall through to a silently wrong transport=tcp.
	for _, uri := range []string{
		"vless://uuid@vless.example.com:443?type=grpc&serviceName=gs&security=tls&sni=example.com#g",
		"vless://uuid@vless.example.com:443?type=xhttp&path=/x&security=tls&sni=example.com#x",
		"vless://uuid@vless.example.com:443?type=quic&security=tls&sni=example.com#q",
		"vmess://eyJ2IjoiMiIsInBzIjoiZyIsImFkZCI6InZtLmV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyIsImlkIjoiNTIzOTZlMDYtMDQxYS00Y2MyLWJlNWMtODUyNWViNDU3ODA5IiwiYWlkIjoiMCIsInNjeSI6ImF1dG8iLCJuZXQiOiJncnBjIiwidHlwZSI6Im5vbmUiLCJ0bHMiOiJ0bHMiLCJzbmkiOiJ2bS5leGFtcGxlLmNvbSJ9#g",
		"trojan://pwd@trojan.example.com:443?type=xhttp&path=/x&security=tls&sni=cdn.example.com#x",
		"anytls://pwd@any.example.com:443?type=ws&path=/ws&security=tls&sni=any.example.com#w",
	} {
		n, _ := parseShareURI(uri)
		if got := renderLoonNode(n); got != "" {
			t.Fatalf("loon must degrade unsupported transport: %s", got)
		}
	}
}

func TestRenderSurgeHysteria2(t *testing.T) {
	n, _ := parseShareURI("hy2://pw@hy2.example.com:443,1234;5000-6000?sni=hy2.example.com&insecure=1&obfs=salamander&obfs-password=obfspw123&down=100+Mbps#surge-node")
	got := renderSurgeNode(n)
	want := `surge-node = hysteria2,hy2.example.com,443,password=pw,sni=hy2.example.com,skip-cert-verify=true,salamander-password=obfspw123,port-hopping="1234;5000-6000",download-bandwidth=100`
	if got != want {
		t.Fatalf("surge line:\n got %q\nwant %q", got, want)
	}
}

func TestRenderSurgeVmessWS(t *testing.T) {
	n, _ := parseShareURI("vmess://eyJ2IjoiMiIsInBzIjoidjEiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjUyMzk2ZTA2LTA0MWEtNGNjMi1iZTVjLTg1MjVlYjQ1NzgwOSIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0Ijoid3MiLCJ0eXBlIjoibm9uZSIsInRscyI6InRscyIsInNuaSI6InZtLmV4YW1wbGUuY29tIiwiaG9zdCI6ImNkbi5leGFtcGxlLmNvbSIsInBhdGgiOiIvd3MifQ==#v1")
	// scy=auto is the panel's default but has NO documented Surge
	// encrypt-method counterpart (policies/vmess.html lists only aes-128-gcm
	// and chacha20-ietf-poly1305). Per the user's decision, a value outside a
	// documented closed set degrades the WHOLE line to a comment.
	reason, skip := surgeSkipMsg(n)
	if !skip || !strings.Contains(reason, "aes-128-gcm") {
		t.Fatalf("scy=auto vmess must degrade: skip=%v reason=%q", skip, reason)
	}
	if got := renderSurgeNode(n); got != "" {
		t.Fatalf("degraded vmess must render empty: %s", got)
	}
	// With a documented cipher the same WS+TLS node renders cleanly and never
	// invents vmess-aead: the manual says it "must match the server's setting"
	// and gives no rule to derive it from the cipher.
	n.Method = "chacha20-ietf-poly1305"
	got := renderSurgeNode(n)
	mustContain(t, got,
		"vmess,vm.example.com,443", "username=52396e06",
		"encrypt-method=chacha20-ietf-poly1305", "ws=true",
		"ws-path=/ws", "ws-headers=Host:cdn.example.com", "tls=true")
	// "encrypt-method=" is the documented Surge name; only the QX-style bare
	// "method=" key would be wrong here.
	mustNotContain(t, got, "vmess-aead", "uuid=")
	if strings.Contains(got, ",method=") {
		t.Fatalf("surge vmess must not use QX-style method=: %s", got)
	}
}

func TestRenderSurgeShadowsocksKeyword(t *testing.T) {
	n, _ := parseShareURI("ss://YWVzLTEyOC1nY206cGFzcw==@ss.example.com:443#ss1")
	got := renderSurgeNode(n)
	want := `ss1 = ss,ss.example.com,443,encrypt-method=aes-128-gcm,password=pass,udp-relay=true`
	if got != want {
		t.Fatalf("surge ss line:\n got %q\nwant %q", got, want)
	}
}

func TestRenderSurgeTUICv5(t *testing.T) {
	n, _ := parseShareURI("tuic://91066351-60f7-44b9-ad7e-c4bcc82524a6:pw@tuic.example.com:443?alpn=h3&sni=tuic.example.com#t5")
	got := renderSurgeNode(n)
	if !strings.Contains(got, "tuic-v5,tuic.example.com,443") || !strings.Contains(got, "uuid=91066351") || !strings.Contains(got, "password=pw") || !strings.Contains(got, "alpn=h3") {
		t.Fatalf("surge tuic-v5 wrong: %s", got)
	}
	if strings.Contains(got, "token=") || strings.Contains(got, "udp-relay") {
		t.Fatalf("surge tuic-v5 must not carry token=/udp-relay: %s", got)
	}
}

func TestRenderSurgeTUICv4(t *testing.T) {
	n, _ := parseShareURI("tuic://token-abc@tuic.example.com:443?alpn=h3#t4")
	got := renderSurgeNode(n)
	if !strings.Contains(got, "tuic,tuic.example.com,443") || !strings.Contains(got, "token=token-abc") {
		t.Fatalf("surge tuic v4 wrong: %s", got)
	}
}

func TestSurgeSkipMsg(t *testing.T) {
	// Surge iOS 6.0.0+ / Mac 6.12.0+ supports VLESS; a plain RAW VLESS node
	// must no longer be commented out. The combination matrix is covered in
	// client_apple_surge_vless_test.go.
	if _, skip := surgeSkipMsg(&shareNode{Protocol: "vless", Params: map[string]string{}}); skip {
		t.Fatal("surge should now keep a plain vless node")
	}
	if reason, skip := surgeSkipMsg(&shareNode{Protocol: "trojan", Params: map[string]string{"security": "reality"}}); !skip || !strings.Contains(reason, "REALITY") {
		t.Fatalf("surge should skip reality trojan: skip=%v reason=%q", skip, reason)
	}
	if _, skip := surgeSkipMsg(&shareNode{Protocol: "hysteria2", Params: map[string]string{}}); skip {
		t.Fatal("surge should keep hysteria2")
	}
	// Surge VMess only accepts aes-128-gcm / chacha20-ietf-poly1305.
	if reason, skip := surgeSkipMsg(&shareNode{Protocol: "vmess", Method: "aes-256-gcm", Params: map[string]string{}}); !skip || !strings.Contains(reason, "aes-128-gcm") {
		t.Fatalf("surge should skip aes-256-gcm vmess: skip=%v reason=%q", skip, reason)
	}
	if _, skip := surgeSkipMsg(&shareNode{Protocol: "vmess", Method: "chacha20-ietf-poly1305", Params: map[string]string{}}); skip {
		t.Fatal("surge should keep chacha20 vmess")
	}
}

func TestRenderSurgeVmessMethodWhitelist(t *testing.T) {
	// chacha20-ietf-poly1305 is the only non-default Surge vmess method we emit.
	n := &shareNode{Name: "vc", Protocol: "vmess", Host: "vm.example.com", Port: "443", Password: "uuid-x", Method: "chacha20-ietf-poly1305"}
	if got := renderSurgeNode(n); !strings.Contains(got, "encrypt-method=chacha20-ietf-poly1305") || strings.Contains(got, "vmess-aead") {
		t.Fatalf("surge chacha20 vmess wrong: %s", got)
	}
	// aes-128-gcm may be explicit or Surge's default; both accepted, but never vmess-aead.
	n2 := &shareNode{Name: "vg", Protocol: "vmess", Host: "vm.example.com", Port: "443", Password: "uuid-x", Method: "aes-128-gcm"}
	if got := renderSurgeNode(n2); strings.Contains(got, "vmess-aead") {
		t.Fatalf("surge aes-128-gcm vmess should be explicit: %s", got)
	}
}

func TestRenderSurgeAlpnPassthrough(t *testing.T) {
	// alpn is a shared TLS parameter for every TLS/QUIC policy.
	n, _ := parseShareURI(`trojan://pw@tj.example.com:443?alpn=h2%2Chttp%2F1.1&sni=tj.example.com#tj`)
	got := renderSurgeNode(n)
	if !strings.Contains(got, `alpn="h2,http/1.1"`) {
		t.Fatalf("surge trojan alpn not quoted-passthrough: %s", got)
	}
	n2, _ := parseShareURI(`anytls://pw@at.example.com:443?alpn=h3#at`)
	if got := renderSurgeNode(n2); !strings.Contains(got, "alpn=h3") {
		t.Fatalf("surge anytls alpn missing: %s", got)
	}
}

func TestRenderQuanXVmess(t *testing.T) {
	n, _ := parseShareURI("vmess://eyJ2IjoiMiIsInBzIjoidjEiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjUyMzk2ZTA2LTA0MWEtNGNjMi1iZTVjLTg1MjVlYjQ1NzgwOSIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0Ijoid3MiLCJ0eXBlIjoibm9uZSIsInRscyI6InRscyIsInNuaSI6InZtLmV4YW1wbGUuY29tIiwiaG9zdCI6ImNkbi5leGFtcGxlLmNvbSIsInBhdGgiOiIvd3MifQ==#v1")
	got := renderQuanXNode(n)
	// sample.conf vmess-ws-tls-02: WebSocket over TLS is obfs=wss with
	// obfs-host/obfs-uri — never over-tls plus tls-host.
	for _, frag := range []string{"vmess=vm.example.com:443", "method=none", "password=52396e06", "obfs=wss", "obfs-uri=/ws", "obfs-host=cdn.example.com", "tag=v1"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("qx vmess missing %q: %s", frag, got)
		}
	}
	for _, bad := range []string{"over-tls", "tls-host"} {
		if strings.Contains(got, bad) {
			t.Fatalf("qx vmess ws must not carry %q: %s", bad, got)
		}
	}
}

func TestRenderQuanXShadowsocksKeyword(t *testing.T) {
	// sample.conf writes shadowsocks=…, never ss=.
	n, _ := parseShareURI("ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@ss.example.com:8388#ss-node")
	got := renderQuanXNode(n)
	if !strings.HasPrefix(got, "shadowsocks=ss.example.com:8388, method=aes-128-gcm") {
		t.Fatalf("qx shadowsocks keyword: %s", got)
	}
	if strings.Contains(got, " ss=") || strings.HasPrefix(got, "ss=") {
		t.Fatalf("qx must not use ss= keyword: %s", got)
	}
}

func TestRenderQuanXSSPluginForms(t *testing.T) {
	ok, _ := parseShareURI("ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@ss.example.com:80?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dbing.com#obfs")
	if got := renderQuanXNode(ok); !strings.Contains(got, "obfs=http, obfs-host=bing.com") {
		t.Fatalf("qx simple-obfs mapping: %s", got)
	}
	// kcptun / shadow-tls / restls / jls have no QX obfs counterpart.
	for _, plugin := range []string{"kcptun;key=x", "shadow-tls;host=apple.com;password=x", "restls;host=apple.com", "jls;password=x"} {
		n, _ := parseShareURI("ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@ss.example.com:80?plugin=" + url.QueryEscape(plugin) + "#wrap")
		if got := renderQuanXNode(n); got != "" {
			t.Fatalf("qx must drop unsupported ss plugin %q: %s", plugin, got)
		}
	}
}

func TestRenderQuanXVmessTcpTLS(t *testing.T) {
	// sample.conf vmess-tls-02: obfs=over-tls with the SNI in obfs-host and
	// tls-verification — not the trojan-style over-tls=true/tls-host pair.
	n, _ := parseShareURI("vmess://eyJ2IjoiMiIsInBzIjoidjEiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjUyMzk2ZTA2LTA0MWEtNGNjMi1iZTVjLTg1MjVlYjQ1NzgwOSIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0IjoidGNwIiwidHlwZSI6Im5vbmUiLCJ0bHMiOiJ0bHMiLCJzbmkiOiJ2bS5leGFtcGxlLmNvbSJ9#v1")
	got := renderQuanXNode(n)
	for _, frag := range []string{"obfs=over-tls", "obfs-host=vm.example.com", "tls-verification=true"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("qx vmess tls missing %q: %s", frag, got)
		}
	}
	for _, bad := range []string{"over-tls=true", "tls-host="} {
		if strings.Contains(got, bad) {
			t.Fatalf("qx vmess tls must not carry %q: %s", bad, got)
		}
	}
}

func TestRenderQuanXVmessTcpTLSInsecure(t *testing.T) {
	n, _ := parseShareURI("vmess://eyJ2IjoiMiIsInBzIjoidjEiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjUyMzk2ZTA2LTA0MWEtNGNjMi1iZTVjLTg1MjVlYjQ1NzgwOSIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0IjoidGNwIiwidHlwZSI6Im5vbmUiLCJ0bHMiOiJ0bHMiLCJzbmkiOiJ2bS5leGFtcGxlLmNvbSJ9#v1")
	n.Params["allowInsecure"] = "1"
	if got := renderQuanXNode(n); !strings.Contains(got, "tls-verification=false") {
		t.Fatalf("qx insecure must set tls-verification=false: %s", got)
	}
}

func TestRenderQuanXVmessReality(t *testing.T) {
	n, _ := parseShareURI("vmess://eyJ2IjoiMiIsInBzIjoicjEiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjUyMzk2ZTA2LTA0MWEtNGNjMi1iZTVjLTg1MjVlYjQ1NzgwOSIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0IjoidGNwIiwidHlwZSI6Im5vbmUiLCJ0bHMiOiJyZWFsaXR5Iiwic25pIjoid3d3LmV4YW1wbGUuY29tIiwicGJrIjoiazRVeGV6MHNqbDhiS2FaSDJWZ2k4LVdERnNoTUw1MVFreEtGTFdGSU9OayIsInNpZCI6IjAxMjM0NTY3ODlhYmNkZWYifQ==#r1")
	got := renderQuanXNode(n)
	for _, frag := range []string{"obfs=over-tls", "obfs-host=www.example.com", "reality-base64-pubkey=k4Uxez0sjl8bKaZH2Vgi8-WDFshML51QkxKFLWFIONk", "reality-hex-shortid=0123456789abcdef"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("qx vmess reality missing %q: %s", frag, got)
		}
	}
	// sample.conf reality lines never pair with tls-verification.
	if strings.Contains(got, "tls-verification") {
		t.Fatalf("qx reality must not carry tls-verification: %s", got)
	}
}

func TestRenderQuanXVlessVisionReality(t *testing.T) {
	// sample.conf vless-tls-reality-vision-01.
	n, _ := parseShareURI("vless://52396e06-041a-4cc2-be5c-8525eb457809@vless.example.com:443?security=reality&pbk=k4Uxez0sjl8bKaZH2Vgi8-WDFshML51QkxKFLWFIONk&sid=0123456789abcdef&flow=xtls-rprx-vision&sni=www.example.com#reality")
	got := renderQuanXNode(n)
	want := "vless=vless.example.com:443, method=none, password=52396e06-041a-4cc2-be5c-8525eb457809, vless-flow=xtls-rprx-vision, obfs=over-tls, obfs-host=www.example.com, reality-base64-pubkey=k4Uxez0sjl8bKaZH2Vgi8-WDFshML51QkxKFLWFIONk, reality-hex-shortid=0123456789abcdef, udp-relay=true, tag=reality"
	if got != want {
		t.Fatalf("qx vless reality line:\n got %q\nwant %q", got, want)
	}
}

func TestRenderQuanXTrojanTLS(t *testing.T) {
	// sample.conf trojan-tls-03: over-tls=true with tls-host, the opposite
	// family spelling from vmess.
	n, _ := parseShareURI("trojan://pwd@192.168.1.1:443?security=tls&sni=example.com#trojan")
	got := renderQuanXNode(n)
	for _, frag := range []string{"trojan=192.168.1.1:443, password=pwd", "over-tls=true", "tls-host=example.com", "tls-verification=true", "udp-relay=true"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("qx trojan tls missing %q: %s", frag, got)
		}
	}
	if strings.Contains(got, "obfs=over-tls") {
		t.Fatalf("qx trojan tls must not use obfs=over-tls: %s", got)
	}
}

func TestRenderQuanXTrojanWSS(t *testing.T) {
	n, _ := parseShareURI("trojan://pwd@192.168.1.1:443?type=ws&path=/path&host=example.com&security=tls&sni=example.com#trojan-wss")
	got := renderQuanXNode(n)
	for _, frag := range []string{"obfs=wss", "obfs-host=example.com", "obfs-uri=/path"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("qx trojan wss missing %q: %s", frag, got)
		}
	}
	for _, bad := range []string{"over-tls", "tls-host"} {
		if strings.Contains(got, bad) {
			t.Fatalf("qx trojan wss must not carry %q: %s", bad, got)
		}
	}
	// sample.conf: trojan is TLS-only, a plain (non-wss) ws has no line.
	plain, _ := parseShareURI("trojan://pwd@192.168.1.1:443?type=ws&path=/path&host=example.com#trojan-ws")
	if got := renderQuanXNode(plain); got != "" {
		t.Fatalf("qx trojan plain ws must degrade: %s", got)
	}
}

func TestRenderQuanXAnyTLSReality(t *testing.T) {
	n, _ := parseShareURI("anytls://pwd@example.com:443?security=reality&pbk=k4Uxez0sjl8bKaZH2Vgi8-WDFshML51QkxKFLWFIONk&sid=0123456789abcdef&sni=apple.com#anytls")
	got := renderQuanXNode(n)
	for _, frag := range []string{"anytls=example.com:443, password=pwd", "over-tls=true", "tls-host=apple.com", "reality-base64-pubkey=k4Uxez0sjl8bKaZH2Vgi8-WDFshML51QkxKFLWFIONk", "reality-hex-shortid=0123456789abcdef", "udp-relay=true"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("qx anytls reality missing %q: %s", frag, got)
		}
	}
	// The panel's anytls export omits the security keyword; anytls is TLS by
	// protocol definition, so it renders with over-tls=true, not a comment.
	plain, _ := parseShareURI("anytls://pwd@example.com:443?sni=apple.com&insecure=1#anytls-plain")
	got = renderQuanXNode(plain)
	for _, frag := range []string{"anytls=example.com:443, password=pwd", "over-tls=true", "tls-host=apple.com", "tls-verification=false"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("qx anytls plain missing %q: %s", frag, got)
		}
	}
	// anytls off tcp has no sample.conf line.
	ws, _ := parseShareURI("anytls://pwd@example.com:443?type=ws&path=/ws&security=tls#anytls-ws")
	if got := renderQuanXNode(ws); got != "" {
		t.Fatalf("qx anytls over ws must degrade: %s", got)
	}
}

func TestRenderQuanXVmessMethodWhitelist(t *testing.T) {
	cases := map[string]string{
		"aes-128-gcm":       "method=aes-128-gcm",
		"chacha20-poly1305": "method=chacha20-poly1305",
		"":                  "method=none",
		"auto":              "method=none",
		"zero":              "method=none",
	}
	for scy, want := range cases {
		n, _ := parseShareURI("vmess://eyJ2IjoiMiIsInBzIjoidjEiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjUyMzk2ZTA2LTA0MWEtNGNjMi1iZTVjLTg1MjVlYjQ1NzgwOSIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0IjoidGNwIiwidHlwZSI6Im5vbmUifQ==#v1")
		n.Method = scy
		got := renderQuanXNode(n)
		if !strings.Contains(got, want) {
			t.Fatalf("qx vmess scy=%q want %q: %s", scy, want, got)
		}
	}
	for _, scy := range []string{"aes-256-gcm", "aes-128-cfb", "rc4-md5"} {
		n, _ := parseShareURI("vmess://eyJ2IjoiMiIsInBzIjoidjEiLCJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjUyMzk2ZTA2LTA0MWEtNGNjMi1iZTVjLTg1MjVlYjQ1NzgwOSIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0IjoidGNwIiwidHlwZSI6Im5vbmUifQ==#v1")
		n.Method = scy
		if got := renderQuanXNode(n); got != "" {
			t.Fatalf("qx vmess scy=%q must degrade: %s", scy, got)
		}
	}
}

func TestRenderQuanXUnsupportedTransportDegraded(t *testing.T) {
	// QX has no grpc / xhttp obfs form in sample.conf.
	for _, uri := range []string{
		"vless://52396e06-041a-4cc2-be5c-8525eb457809@vless.example.com:443?type=grpc&serviceName=gs&security=tls&sni=example.com#grpc",
		"vless://52396e06-041a-4cc2-be5c-8525eb457809@vless.example.com:443?type=xhttp&path=/x&security=tls&sni=example.com#xhttp",
		"trojan://pwd@192.168.1.1:443?type=grpc&serviceName=gs&security=tls&sni=example.com#tgrpc",
	} {
		n, _ := parseShareURI(uri)
		if got := renderQuanXNode(n); got != "" {
			t.Fatalf("qx must degrade unsupported transport: %s", got)
		}
	}
}

func TestQuanXAlpnHex(t *testing.T) {
	// sample.conf ss-tls-03: tls-alpn=02683208687474702f312e31 is h2 + http/1.1.
	if got := quanxAlpnHex("h2,http/1.1"); got != "02683208687474702f312e31" {
		t.Fatalf("alpn hex = %q", got)
	}
	if got := quanxAlpnHex(""); got != "" {
		t.Fatalf("empty alpn = %q", got)
	}
	if got := quanxAlpnHex(" , "); got != "" {
		t.Fatalf("blank alpn = %q", got)
	}
}

func TestRenderQuanXAlpnPassthrough(t *testing.T) {
	n, _ := parseShareURI("trojan://pwd@192.168.1.1:443?security=tls&sni=example.com&alpn=h2,http/1.1#alpn")
	got := renderQuanXNode(n)
	if !strings.Contains(got, "tls-alpn=02683208687474702f312e31") {
		t.Fatalf("qx alpn missing: %s", got)
	}
}

func TestRenderQuanXHysteria2Skipped(t *testing.T) {
	n, _ := parseShareURI("hy2://pw@85.149.212.214:13290#hy2")
	if renderQuanXNode(n) != "" {
		t.Fatalf("QX must not render hysteria2: %s", renderQuanXNode(n))
	}
}

func TestQuoteValue(t *testing.T) {
	cases := map[string]string{
		"plain": "plain",
		"a,b":   `"a,b"`,
		`x"y`:   `"xy"`,
	}
	for in, want := range cases {
		if got := quoteValue(in); got != want {
			t.Fatalf("quoteValue(%q) = %q want %q", in, got, want)
		}
	}
}

func TestHopFormatSemicolon(t *testing.T) {
	if got := hopFormatSemicolon("443,5000-6000"); got != "443;5000-6000" {
		t.Fatalf("hop = %q", got)
	}
	if got := hopFormatSemicolon("443;5000-6000"); got != "443;5000-6000" {
		t.Fatalf("hop = %q", got)
	}
}
