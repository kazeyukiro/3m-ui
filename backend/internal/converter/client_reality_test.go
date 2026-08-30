package converter

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

func TestVLESSRealityClientExport(t *testing.T) {
	l := models.Listener{
		Name: "vless-in", Protocol: "vless", Port: "443", Enabled: true,
		Config: `{"flow":"xtls-rprx-vision","reality-config":{"dest":"test.com:443","private-key":"jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0","short-id":["0123456789abcdef"],"server-names":["test.com"]}}`,
	}
	creds := []user.Credential{{Username: "u1", UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	p := proxies[0]
	if p["tls"] != true || p["servername"] != "test.com" {
		t.Fatalf("%#v", p)
	}
	ro, ok := p["reality-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing reality-opts %#v", p)
	}
	pk, _ := ro["public-key"].(string)
	const want = "ZKKPN8Up66SimLskEzVwwOmIoPHTJNrDtvU5kXqGz1M"
	if pk != want {
		t.Fatalf("public-key=%q, want %q", pk, want)
	}
	if strings.ContainsAny(pk, "+/=") {
		t.Fatalf("public-key is not raw URL-safe base64: %q", pk)
	}
	if raw, err := base64.RawURLEncoding.DecodeString(pk); err != nil || len(raw) != 32 {
		t.Fatalf("public-key is not a valid 32-byte raw URL-safe base64 value: %q", pk)
	}
	if p["client-fingerprint"] != "chrome" {
		t.Fatalf("fp=%v", p["client-fingerprint"])
	}
}

func TestTrojanCertificateClientExport(t *testing.T) {
	l := models.Listener{
		Name: "trojan-in", Protocol: "trojan", Port: "443", Enabled: true,
		Config: `{"certificate":"./server.crt","private-key":"./server.key","sni":"example.com"}`,
	}
	creds := []user.Credential{{Username: "u1", Password: "secret"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	if proxies[0]["tls"] != true || proxies[0]["password"] != "secret" {
		t.Fatalf("%#v", proxies[0])
	}
}

func TestHysteria2ClientExport(t *testing.T) {
	l := models.Listener{
		Name: "hy2", Protocol: "hysteria2", Port: "443", Enabled: true,
		Config: `{"up":1000,"down":1000,"alpn":["h3"],"sni":"custom.example.com","certificate":"./server.crt","private-key":"./server.key","fingerprint":"AA:BB"}`,
	}
	creds := []user.Credential{{Username: "user1", Password: "password1"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	p := proxies[0]
	if p["password"] != "password1" || p["sni"] != "custom.example.com" || p["fingerprint"] != "AA:BB" {
		t.Fatalf("%#v", p)
	}
}

func TestShadowQUICClientExport(t *testing.T) {
	l := models.Listener{
		Name: "sq", Protocol: "shadowquic", Port: "443", Enabled: true,
		Config: `{"quic-versions":["v2"],"congestion-controller":"bbr","alpn":["h3"],"jls-upstream":{"addr":"test.com:443","sni":"test.com"}}`,
	}
	creds := []user.Credential{{Username: "u", Password: "p"}}
	proxies, err := listenerToProxies(l, "1.2.3.4", creds)
	if err != nil {
		t.Fatal(err)
	}
	p := proxies[0]
	if p["username"] != "u" || p["password"] != "p" || p["sni"] != "test.com" || p["udp"] != true {
		t.Fatalf("base fields mismatch: %#v", p)
	}
	// quic-versions / congestion-controller must pass through unchanged.
	if p["quic-versions"] == nil || fmt.Sprint(p["quic-versions"]) != "[v2]" {
		t.Fatalf("quic-versions not passed through: %#v", p["quic-versions"])
	}
	if p["congestion-controller"] != "bbr" {
		t.Fatalf("congestion-controller not passed through: %#v", p["congestion-controller"])
	}
	// zero-rtt is absent from the listener config; the converter must NOT
	// invent a default (official docs default is false, applied by mihomo).
	if _, ok := p["zero-rtt"]; ok {
		t.Fatalf("zero-rtt should not be defaulted when absent from listener config: %#v", p["zero-rtt"])
	}
}
