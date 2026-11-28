package converter

import (
	"strings"
	"testing"
)

// These tests pin the Surge VLESS export to the real grammar documented at
// https://manual.nssurge.com/policies/vless.html (Surge iOS 6.0.0+ /
// Mac 6.12.0+). The four "Proxy-" examples below are transcribed from that
// page; share links are the panel's own URI vocabulary (uri_builders.go).

// mustContain fails unless every fragment appears in got.
func mustContain(t *testing.T, got string, frags ...string) {
	t.Helper()
	for _, f := range frags {
		if !strings.Contains(got, f) {
			t.Fatalf("missing %q in line:\n%s", f, got)
		}
	}
}

func mustNotContain(t *testing.T, got string, frags ...string) {
	t.Helper()
	for _, f := range frags {
		if strings.Contains(got, f) {
			t.Fatalf("unexpected %q in line:\n%s", f, got)
		}
	}
}

// Example 1 on the page: RAW + REALITY + Vision.
func TestSurgeVLESS_RAWRealityVision(t *testing.T) {
	n, err := parseShareURI("vless://2cae91b3-f85c-433b-88ee-a002b9ed6d61@1.2.3.4:443" +
		"?security=reality&pbk=dwrjY6KiAMPxKdlF7T4AWFXXBtsxeHCXtBQYeCOdZV4" +
		"&sid=0123456789abcdef&flow=xtls-rprx-vision&sni=www.microsoft.com&encryption=none#Proxy-REALITY")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("RAW+REALITY+Vision must not be skipped")
	}
	got := renderSurgeNode(n)
	mustContain(t, got,
		"Proxy-REALITY = vless,1.2.3.4,443",
		"username=2cae91b3-f85c-433b-88ee-a002b9ed6d61",
		"reality-public-key=dwrjY6KiAMPxKdlF7T4AWFXXBtsxeHCXtBQYeCOdZV4",
		"reality-short-id=0123456789abcdef",
		"sni=www.microsoft.com",
		"flow=xtls-rprx-vision",
	)
	// REALITY implies TLS; tls=true and skip-cert-verify must NOT be emitted.
	mustNotContain(t, got, "tls=true", "skip-cert-verify", "encryption=", "fp=", "spx=")
}

// Example 2 on the page: WebSocket + TLS.
func TestSurgeVLESS_WSTLS(t *testing.T) {
	n, err := parseShareURI("vless://2cae91b3-f85c-433b-88ee-a002b9ed6d61@example.com:443" +
		"?type=ws&path=/vless&host=cdn.example.com&security=tls&encryption=none#Proxy-WS")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("WS+TLS must not be skipped")
	}
	got := renderSurgeNode(n)
	mustContain(t, got,
		"Proxy-WS = vless,example.com,443",
		"username=2cae91b3-f85c-433b-88ee-a002b9ed6d61",
		"ws=true", "ws-path=/vless", "ws-headers=Host:cdn.example.com", "tls=true",
	)
	mustNotContain(t, got, "grpc", "reality", "flow=", "encryption=")
}

// Example 3 on the page: RAW + TLS + Vision.
func TestSurgeVLESS_RAWTLSVision(t *testing.T) {
	n, err := parseShareURI("vless://2cae91b3-f85c-433b-88ee-a002b9ed6d61@example.com:443" +
		"?security=tls&flow=xtls-rprx-vision&encryption=none#Proxy-TLS")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("RAW+TLS+Vision must not be skipped")
	}
	got := renderSurgeNode(n)
	mustContain(t, got, "Proxy-TLS = vless,example.com,443", "tls=true", "flow=xtls-rprx-vision")
	mustNotContain(t, got, "reality", "ws=", "grpc")
}

// Example 4 on the page: gRPC + TLS.
func TestSurgeVLESS_gRPCTLS(t *testing.T) {
	n, err := parseShareURI("vless://2cae91b3-f85c-433b-88ee-a002b9ed6d61@example.com:443" +
		"?type=grpc&serviceName=mygrpc&security=tls&encryption=none#Proxy-gRPC")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("gRPC+TLS must not be skipped")
	}
	got := renderSurgeNode(n)
	mustContain(t, got,
		"Proxy-gRPC = vless,example.com,443", "grpc=true",
		"grpc-service-name=mygrpc", "tls=true",
	)
	mustNotContain(t, got, "ws=", "reality", "flow=")
}

// gRPC over REALITY is a documented combination; mode=multi and authority
// map to grpc-mode=multi and grpc-authority per the share-link table.
func TestSurgeVLESS_gRPCRealityMultiAuthority(t *testing.T) {
	n, err := parseShareURI("vless://uuid-abc@grpc.example.com:443" +
		"?type=grpc&serviceName=gs&mode=multi&authority=auth.example.com" +
		"&security=reality&pbk=PUBKEY&sid=4f3a&sni=grpc.example.com&encryption=none#gmulti")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("gRPC+REALITY must not be skipped")
	}
	got := renderSurgeNode(n)
	mustContain(t, got,
		"gmulti = vless,grpc.example.com,443", "grpc=true",
		"grpc-service-name=gs", "grpc-mode=multi", "grpc-authority=auth.example.com",
		"reality-public-key=PUBKEY", "reality-short-id=4f3a", "sni=grpc.example.com",
	)
	mustNotContain(t, got, "tls=true", "skip-cert-verify")
}

// Shared TLS parameters: alpn and skip-cert-verify (allowInsecure=1).
func TestSurgeVLESS_SharedTLSParams(t *testing.T) {
	n, err := parseShareURI("vless://uuid-x@tls.example.com:443" +
		"?security=tls&sni=tls.example.com&alpn=h2%2Chttp%2F1.1" +
		"&allowInsecure=1&fp=chrome&spx=/&packetEncoding=xudp&encryption=none#tls")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := renderSurgeNode(n)
	mustContain(t, got, `alpn="h2,http/1.1"`, "skip-cert-verify=true", "sni=tls.example.com")
	// fp / spx / packetEncoding are not used by Surge; encryption=none is
	// accepted but carries no Surge parameter of its own.
	mustNotContain(t, got, "fp=", "spx=", "packetEncoding", "encryption=")
}

// flow=xtls-rprx-vision-udp443 is the second documented Vision value.
func TestSurgeVLESS_VisionUDP443(t *testing.T) {
	n, err := parseShareURI("vless://uuid-y@r.example.com:443" +
		"?security=reality&pbk=PK&flow=xtls-rprx-vision-udp443&encryption=none#v443")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, skip := surgeSkipMsg(n); skip {
		t.Fatal("Vision udp443 over REALITY must not be skipped")
	}
	mustContain(t, renderSurgeNode(n), "flow=xtls-rprx-vision-udp443", "reality-public-key=PK")
}

// ---- Degradation matrix (Surge expressions that do NOT exist) -------------

func TestSurgeVLESS_UnsupportedMatrix(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want string // substring of the degradation reason
	}{
		{"httpupgrade", "vless://u@h:443?type=httpupgrade&security=tls&encryption=none#hu", "传输"},
		{"mkcp", "vless://u@h:443?type=mkcp&security=tls&encryption=none#k", "传输"},
		{"kcp", "vless://u@h:443?type=kcp&security=tls&encryption=none#k2", "传输"},
		{"vision-over-ws", "vless://u@h:443?type=ws&path=/v&security=tls&flow=xtls-rprx-vision&encryption=none#vw", "WebSocket"},
		{"vision-over-grpc", "vless://u@h:443?type=grpc&serviceName=gs&security=tls&flow=xtls-rprx-vision&encryption=none#vg", "gRPC"},
		{"grpc-without-tls", "vless://u@h:443?type=grpc&serviceName=gs&encryption=none#gt", "gRPC"},
		{"vision-without-tls", "vless://u@h:443?flow=xtls-rprx-vision&encryption=none#vt", "Vision"},
		{"ws-with-reality", "vless://u@h:443?type=ws&path=/v&security=reality&pbk=PK&encryption=none#wr", "WebSocket"},
		{"reality-without-pbk", "vless://u@h:443?security=reality&encryption=none#rp", "reality-public-key"},
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
			// Every degraded node must also be refused by the renderer itself,
			// so a bad combination can never reach a policy line.
			if got := renderSurgeNode(n); got != "" {
				t.Fatalf("degraded node must render empty, got: %s", got)
			}
		})
	}
}

// Trojan / VMess REALITY stays unsupported: the official VLESS page adds
// REALITY only for VLESS, and policies/trojan & policies/vmess document no
// reality-public-key parameter.
func TestSurgeVLESS_OtherProtocolsRealityStillDegraded(t *testing.T) {
	// A real trojan share link degrades on its REALITY security.
	n, err := parseShareURI("trojan://pw@h:443?security=reality&pbk=PK#tr")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if reason, skip := surgeSkipMsg(n); !skip || !strings.Contains(reason, "REALITY") {
		t.Fatalf("trojan reality must degrade: skip=%v reason=%q", skip, reason)
	}
	vmess := &shareNode{Protocol: "vmess", Method: "auto",
		Params: map[string]string{"security": "reality", "pbk": "PK"}}
	if reason, skip := surgeSkipMsg(vmess); !skip || !strings.Contains(reason, "REALITY") {
		t.Fatalf("vmess reality must degrade: skip=%v reason=%q", skip, reason)
	}
}
