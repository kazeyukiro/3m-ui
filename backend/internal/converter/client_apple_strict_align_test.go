package converter

import (
	"strings"
	"testing"
)

// Strict-alignment tests (revision 2 of the official-docs fact baseline).
//
// Sources, transcribed verbatim:
//   Surge: manual.nssurge.com/policies/vless.html  (4-row matrix, XHTTP,
//          VLESS Encryption), plus vmess / trojan / shadowsocks / hysteria2 /
//          tuic / anytls / tls
//   Loon:  nsloon.app/docs/Node and nsloon.app/en/docs/Node
//
// Two rules are pinned everywhere below:
//   * a value outside a documented closed set degrades the WHOLE line
//     (renderer returns "" and the generator writes a comment);
//   * no parameter name or value is emitted unless the manual documents it.

// ---- Surge VLESS: official examples 5-7 (XHTTP / VLESS Encryption) --------

func TestStrict_SurgeVLESS_XHTTPReality(t *testing.T) {
	// Official example "Proxy-XHTTP": XHTTP + REALITY (stream-one over HTTP/2).
	n, err := parseShareURI("vless://2cae91b3-f85c-433b-88ee-a002b9ed6d61@1.2.3.4:443" +
		"?type=xhttp&path=/xhttp&sni=www.microsoft.com" +
		"&security=reality&pbk=dwrjY6KiAMPxKdlF7T4AWFXXBtsxeHCXtBQYeCOdZV4&encryption=none#Proxy-XHTTP")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("official XHTTP+REALITY example must not be degraded")
	}
	got := renderSurgeNode(n)
	mustContain(t, got,
		"Proxy-XHTTP = vless,1.2.3.4,443",
		"username=2cae91b3-f85c-433b-88ee-a002b9ed6d61",
		"xhttp=true", "xhttp-path=/xhttp", "sni=www.microsoft.com",
		"reality-public-key=dwrjY6KiAMPxKdlF7T4AWFXXBtsxeHCXtBQYeCOdZV4")
	// REALITY implies TLS; tls=true / skip-cert-verify must not be emitted.
	mustNotContain(t, got, "tls=true", "ws=", "grpc", "skip-cert-verify", "encryption=")
}

func TestStrict_SurgeVLESS_XHTTPTLSCDN(t *testing.T) {
	// Official example "Proxy-XHTTP-CDN": XHTTP + TLS through a CDN.
	n, err := parseShareURI("vless://2cae91b3-f85c-433b-88ee-a002b9ed6d61@cdn.example.com:443" +
		"?type=xhttp&path=/xhttp&security=tls&encryption=none#Proxy-XHTTP-CDN")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("official XHTTP+TLS example must not be degraded")
	}
	got := renderSurgeNode(n)
	mustContain(t, got, "Proxy-XHTTP-CDN = vless,cdn.example.com,443",
		"xhttp=true", "xhttp-path=/xhttp", "tls=true")
	mustNotContain(t, got, "reality", "ws=", "grpc", "encryption=")
}

func TestStrict_SurgeVLESS_VLESSEncryptionVision(t *testing.T) {
	// Official example "Proxy-Enc": RAW + VLESS Encryption + Vision, no TLS.
	// The manual: "Vision requires TLS 1.3 ... and the RAW transport. With
	// VLESS Encryption, Vision runs on the encryption layer instead: it needs
	// no TLS".
	n, err := parseShareURI("vless://2cae91b3-f85c-433b-88ee-a002b9ed6d61@1.2.3.4:443" +
		"?flow=xtls-rprx-vision&encryption=mlkem768x25519plus.native.0rtt.vbS7cJA...Lc9lQ#Proxy-Enc")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("official VLESS Encryption + Vision example must not be degraded")
	}
	got := renderSurgeNode(n)
	mustContain(t, got, "Proxy-Enc = vless,1.2.3.4,443",
		"flow=xtls-rprx-vision",
		"encryption=mlkem768x25519plus.native.0rtt.vbS7cJA...Lc9lQ")
	mustNotContain(t, got, "tls=true", "reality", "ws=", "grpc")
}

func TestStrict_SurgeVLESS_WSEncryptionVision(t *testing.T) {
	// Matrix row: WebSocket / "With VLESS Encryption" for Vision flow.
	n, err := parseShareURI("vless://u@ws.example.com:443" +
		"?type=ws&path=/v&flow=xtls-rprx-vision" +
		"&encryption=mlkem768x25519plus.native.1rtt.KEY&security=tls#wse")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("WS + VLESS Encryption + Vision is a documented combination")
	}
	got := renderSurgeNode(n)
	mustContain(t, got, "ws=true", "ws-path=/v", "flow=xtls-rprx-vision",
		"encryption=mlkem768x25519plus.native.1rtt.KEY", "tls=true")
}

func TestStrict_SurgeVLESS_XHTTPModes(t *testing.T) {
	// "xhttp-mode: Optional, auto, packet-up, stream-up or stream-one".
	for _, mode := range []string{"auto", "packet-up", "stream-up", "stream-one"} {
		n, _ := parseShareURI("vless://u@x.example.com:443" +
			"?type=xhttp&path=/x&mode=" + mode + "&security=tls&encryption=none#m")
		if _, skip := surgeSkipMsg(n); skip {
			t.Fatalf("xhttp-mode=%s must be accepted", mode)
		}
		got := renderSurgeNode(n)
		mustContain(t, got, "xhttp=true", "xhttp-mode="+mode)
	}
	bad, _ := parseShareURI("vless://u@x.example.com:443" +
		"?type=xhttp&path=/x&mode=bogus&security=tls&encryption=none#bad")
	if reason, skip := surgeSkipMsg(bad); !skip || !strings.Contains(reason, "xhttp-mode") {
		t.Fatalf("xhttp-mode=bogus must degrade: skip=%v reason=%q", skip, reason)
	}
	if got := renderSurgeNode(bad); got != "" {
		t.Fatalf("degraded xhttp mode must render empty: %s", got)
	}
}

func TestStrict_SurgeVLESS_SplithttpAlias(t *testing.T) {
	// Share-link table: "type=xhttp (or splithttp), path, host, mode".
	n, _ := parseShareURI("vless://u@x.example.com:443" +
		"?type=splithttp&path=/x&host=cdn.example.com&mode=packet-up&security=tls&encryption=none#sh")
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("splithttp alias must be accepted")
	}
	got := renderSurgeNode(n)
	mustContain(t, got, "xhttp=true", "xhttp-path=/x",
		"xhttp-host=cdn.example.com", "xhttp-mode=packet-up")
	mustNotContain(t, got, "ws=", "grpc")
}

// ---- Surge VLESS: closed sets and the official 4-row matrix ---------------

func TestStrict_SurgeVLESS_ClosedSetDegradation(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want string
	}{
		{"flow-out-of-set", "vless://u@h:443?flow=xtls-rprx-direct&encryption=none#f", "flow"},
		{"sid-non-hex", "vless://u@h:443?security=reality&pbk=PK&sid=zzzz&encryption=none#s", "十六进制"},
		{"sid-too-long", "vless://u@h:443?security=reality&pbk=PK&sid=0123456789abcdef0&encryption=none#s2", "十六进制"},
		{"grpc-mode-out-of-set", "vless://u@h:443?type=grpc&serviceName=gs&mode=bogus&security=tls&encryption=none#gm", "grpc-mode"},
		{"xhttp-mode-out-of-set", "vless://u@h:443?type=xhttp&path=/x&mode=bogus&security=tls&encryption=none#xm", "xhttp-mode"},
		{"ws-path-no-slash", "vless://u@h:443?type=ws&path=no-slash&security=tls&encryption=none#wp", "ws-path"},
		{"encryption-bad-shape", "vless://u@h:443?security=tls&encryption=garbage#eb", "mlkem768x25519plus"},
		{"grpc-never-vision", "vless://u@h:443?type=grpc&serviceName=gs&flow=xtls-rprx-vision&encryption=mlkem768x25519plus.native.1rtt.KEY&security=tls#gv", "gRPC"},
		{"ws-vision-needs-encryption", "vless://u@h:443?type=ws&path=/v&flow=xtls-rprx-vision&security=tls&encryption=none#wv", "WebSocket"},
		{"xhttp-vision-needs-encryption", "vless://u@h:443?type=xhttp&path=/x&flow=xtls-rprx-vision&security=tls&encryption=none#xv", "XHTTP"},
		{"raw-vision-needs-security", "vless://u@h:443?flow=xtls-rprx-vision&encryption=none#rv", "Vision"},
		{"httpupgrade", "vless://u@h:443?type=httpupgrade&security=tls&encryption=none#hu", "传输"},
		{"mkcp", "vless://u@h:443?type=mkcp&security=tls&encryption=none#mk", "传输"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, err := parseShareURI(c.uri)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			reason, skip := surgeSkipMsg(n)
			if !skip {
				t.Fatalf("expected degradation, got rendered line: %s", renderSurgeNode(n))
			}
			if !strings.Contains(reason, c.want) {
				t.Fatalf("reason %q lacks %q", reason, c.want)
			}
			if got := renderSurgeNode(n); got != "" {
				t.Fatalf("degraded node must render empty: %s", got)
			}
		})
	}
}

// ---- Surge: other protocols' documented closed sets -----------------------

func TestStrict_SurgeSS_ClosedSets(t *testing.T) {
	base := func(method string, plugin string) *shareNode {
		return &shareNode{Name: "ss", Protocol: "ss", Host: "h", Port: "443",
			Password: "pw", Method: method, Params: map[string]string{"plugin": plugin}}
	}
	// Surge's list includes `none`; Loon's does not.
	for _, m := range []string{"aes-128-gcm", "2022-blake3-aes-256-gcm", "none"} {
		if _, skip := surgeSkipMsg(base(m, "")); skip {
			t.Fatalf("Surge SS method %q is documented and must render", m)
		}
	}
	// Loon has bf-cfb / camellia-* / idea-cfb that Surge does NOT document.
	for _, m := range []string{"bf-cfb", "camellia-128-cfb", "idea-cfb", "bogus"} {
		if reason, skip := surgeSkipMsg(base(m, "")); !skip || !strings.Contains(reason, "加密方式") {
			t.Fatalf("Surge SS method %q must degrade: skip=%v reason=%q", m, skip, reason)
		}
	}
	// "obfs-uri: Only meaningful with obfs=http."
	gotTLS := renderSurgeNode(base("aes-128-gcm", "obfs-local;obfs=tls;obfs-uri=/x"))
	mustContain(t, gotTLS, "obfs=tls")
	mustNotContain(t, gotTLS, "obfs-uri")
	gotHTTP := renderSurgeNode(base("aes-128-gcm", "obfs-local;obfs=http;obfs-uri=/x"))
	mustContain(t, gotHTTP, "obfs=http", "obfs-uri=/x")
	if _, skip := surgeSkipMsg(base("aes-128-gcm", "obfs-local;obfs=bogus")); !skip {
		t.Fatal("Surge SS obfs=bogus must degrade")
	}
}

func TestStrict_SurgeAnyTLS_Transport(t *testing.T) {
	// policies/anytls.html documents only password and reuse - no transport.
	ws, _ := parseShareURI("anytls://pw@h:443?type=ws&security=tls#a")
	if reason, skip := surgeSkipMsg(ws); !skip || !strings.Contains(reason, "TCP") {
		t.Fatalf("Surge anytls over ws must degrade: skip=%v reason=%q", skip, reason)
	}
	if got := renderSurgeNode(ws); got != "" {
		t.Fatalf("degraded anytls must render empty: %s", got)
	}
	// A plain anytls link still renders with the shared TLS parameters.
	plain, _ := parseShareURI("anytls://pw@h:443?alpn=h3#a")
	mustContain(t, renderSurgeNode(plain), "anytls,h,443", "password=pw", "alpn=h3")
}

// ---- Loon: documented closed sets -----------------------------------------

func TestStrict_LoonSS_Methods(t *testing.T) {
	base := func(method string) *shareNode {
		return &shareNode{Name: "ss", Protocol: "ss", Host: "h", Port: "443",
			Password: "pw", Method: method, Params: map[string]string{}}
	}
	// Verbatim from docs/Node "支持的加密方式".
	for _, m := range []string{"aes-128-gcm", "bf-cfb", "idea-cfb", "salsa20"} {
		got := renderLoonNode(base(m))
		mustContain(t, got, "Shadowsocks,h,443,"+m+",\"pw\"")
	}
	// Loon's list has NO `none` (Surge's does) - a useful cross-check.
	for _, m := range []string{"none", "bogus"} {
		if reason, skip := loonSkipMsg(base(m)); !skip || !strings.Contains(reason, "加密方式") {
			t.Fatalf("Loon SS method %q must degrade: skip=%v reason=%q", m, skip, reason)
		}
		if got := renderLoonNode(base(m)); got != "" {
			t.Fatalf("degraded Loon SS must render empty: %s", got)
		}
	}
}

func TestStrict_LoonVMess_Ciphers(t *testing.T) {
	base := func(method string) *shareNode {
		return &shareNode{Name: "v", Protocol: "vmess", Host: "h", Port: "443",
			Password: "uuid", Method: method, Params: map[string]string{}}
	}
	// docs/Node: none, auto, aes-128-cfb, aes-128-gcm, chacha20-ietf-poly1305.
	for _, m := range []string{"none", "auto", "aes-128-cfb", "chacha20-ietf-poly1305"} {
		mustContain(t, renderLoonNode(base(m)), "VMess,h,443,"+m+",\"uuid\"")
	}
	for _, m := range []string{"aes-256-gcm", "bogus"} {
		if got := renderLoonNode(base(m)); got != "" {
			t.Fatalf("Loon VMess cipher %q must degrade: %s", m, got)
		}
	}
}

func TestStrict_LoonVLESS_Flows(t *testing.T) {
	base := func(flow string) *shareNode {
		return &shareNode{Name: "v", Protocol: "vless", Host: "h", Port: "443",
			Password: "uuid", Params: map[string]string{"flow": flow}}
	}
	// docs/Node VLESS: "`flow` 当前支持 `xtls-rprx-vision`" only.
	mustContain(t, renderLoonNode(base("xtls-rprx-vision")), "flow=xtls-rprx-vision")
	if reason, skip := loonSkipMsg(base("xtls-rprx-vision-udp443")); !skip || !strings.Contains(reason, "flow") {
		t.Fatalf("Loon VLESS udp443 flow must degrade: skip=%v reason=%q", skip, reason)
	}
	if got := renderLoonNode(base("xtls-rprx-vision-udp443")); got != "" {
		t.Fatalf("degraded Loon VLESS must render empty: %s", got)
	}
	// Surge documents BOTH flow values - the sets really are client-specific.
	n := &shareNode{Name: "v", Protocol: "vless", Host: "h", Port: "443",
		Password: "uuid", Params: map[string]string{
			"flow": "xtls-rprx-vision-udp443", "security": "reality", "pbk": "PK", "encryption": "none"}}
	mustContain(t, renderSurgeNode(n), "flow=xtls-rprx-vision-udp443")
}

func TestStrict_LoonHysteria2_Obfs(t *testing.T) {
	base := func(obfs string) *shareNode {
		return &shareNode{Name: "h", Protocol: "hysteria2", Host: "h", Port: "443",
			Password: "pw", Params: map[string]string{"obfs": obfs, "obfs-password": "opw"}}
	}
	// docs/Node Hysteria 2 documents salamander-password only.
	mustContain(t, renderLoonNode(base("salamander")), "salamander-password=opw")
	if reason, skip := loonSkipMsg(base("gecko")); !skip || !strings.Contains(reason, "Gecko") {
		t.Fatalf("Loon Hysteria2 gecko must degrade: skip=%v reason=%q", skip, reason)
	}
	if got := renderLoonNode(base("gecko")); got != "" {
		t.Fatalf("degraded Loon Hy2 must render empty: %s", got)
	}
	// Surge documents both, and they are mutually exclusive.
	mustContain(t, renderSurgeNode(base("gecko")), "gecko-password=opw")
}

// ---- Loon: official declaration forms and kept empirical behaviour --------

func TestStrict_Loon_OfficialQuotingForms(t *testing.T) {
	// Every Format line in docs/Node writes the positional credential in
	// double quotes: Shadowsocks,...,"password" / VMess,...,"UUID" /
	// VLESS,...,"UUID" / Trojan,...,"password" / Hysteria2,...,"password" /
	// AnyTLS,...,"password".
	nodes := []*shareNode{
		{Name: "a", Protocol: "ss", Host: "h", Port: "443", Password: "pw", Method: "aes-128-gcm", Params: map[string]string{}},
		{Name: "b", Protocol: "vmess", Host: "h", Port: "443", Password: "uuid", Method: "auto", Params: map[string]string{}},
		{Name: "c", Protocol: "vless", Host: "h", Port: "443", Password: "uuid", Params: map[string]string{}},
		{Name: "d", Protocol: "trojan", Host: "h", Port: "443", Password: "pw", Params: map[string]string{}},
		{Name: "e", Protocol: "hysteria2", Host: "h", Port: "443", Password: "pw", Params: map[string]string{}},
		{Name: "f", Protocol: "anytls", Host: "h", Port: "443", Password: "pw", Params: map[string]string{}},
	}
	wants := []string{
		`Shadowsocks,h,443,aes-128-gcm,"pw"`,
		`VMess,h,443,auto,"uuid"`,
		`VLESS,h,443,"uuid"`,
		`Trojan,h,443,"pw"`,
		`Hysteria2,h,443,"pw"`,
		`AnyTLS,h,443,"pw"`,
	}
	for i, n := range nodes {
		mustContain(t, renderLoonNode(n), wants[i])
	}
}

func TestStrict_Loon_KeptEmpiricalBehaviours(t *testing.T) {
	// Three documented-vs-measured deviations that the user explicitly chose
	// to KEEP. They are asserted here so a future "alignment" cannot silently
	// drop them, and they must be disclosed in the evidence report.

	// 1) docs/Node documents public-key / short-id for AnyTLS REALITY, but a
	//    real-device check found Loon reliably parses only the password-only
	//    form, so REALITY args are dropped.
	at, _ := parseShareURI("anytls://pw@h:443?security=reality&pbk=PK&sid=abcd&sni=www.microsoft.com#ar")
	got := renderLoonNode(at)
	mustContain(t, got, `AnyTLS,h,443,"pw"`, "sni=www.microsoft.com")
	mustNotContain(t, got, "public-key", "short-id", "over-tls")

	// 2) docs/Node uses server-ports for port hopping only, but Loon's Hysteria 2
	//    plugin does not dial a single-port node without the field.
	hy, _ := parseShareURI("hy2://pw@h:13290#hy")
	mustContain(t, renderLoonNode(hy), `server-ports="13290"`)

	// 3) The official Trojan REALITY example has NO over-tls=true, while
	//    VMess/VLESS REALITY examples do. The exporter emits it uniformly.
	tr, _ := parseShareURI("trojan://pw@h:443?security=reality&pbk=PK&sid=abcd&sni=www.example.com#tr")
	mustContain(t, renderLoonNode(tr), "over-tls=true", `public-key="PK"`)
}
