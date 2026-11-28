package node

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
)

// P0-1 regression: passwords containing @ : / ? # must survive the URI
// round-trip byte-for-byte. url.PathEscape left @ (and :) unescaped, which
// broke the userinfo boundary for every client that splits at the first @.
//
// Assertion semantics: User.String() returns the unescaped "user:secret"
// concatenation, which is exactly what mihomo / v2rayN / sing-box recover by
// unescaping the userinfo and splitting at the first colon.
const specialPw = "p@ss:w/rd%1#x"

func parseURIRoundTrip(t *testing.T, uri string) *url.URL {
	t.Helper()
	sp, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("client cannot parse URI %q: %v", uri, err)
	}
	return sp
}

func assertUserinfo(t *testing.T, uri, user, secret string) {
	t.Helper()
	sp := parseURIRoundTrip(t, uri)
	if sp.User == nil {
		t.Fatalf("URI has no userinfo: %s", uri)
	}
	// NOTE: Userinfo.String() re-escapes, so compare the DECODED parts.
	// A Go client splits the userinfo at the first literal colon; a v2rayN /
	// mihomo client unescapes first. Both recover the same credential as long
	// as we escape @ / ? # / % and never leave the separator ambiguous.
	recovered := sp.User.Username()
	if pw, ok := sp.User.Password(); ok {
		recovered += ":" + pw
	}
	want := user + ":" + secret
	if user == "" {
		want = secret
	}
	if recovered != want {
		t.Fatalf("userinfo round-trip broken: got %q want %q (uri=%s)", recovered, want, uri)
	}
}

func TestTrojanURIEscapesUserinfoSpecialChars(t *testing.T) {
	cfg := map[string]interface{}{
		"users": []interface{}{map[string]interface{}{"username": "bob", "password": specialPw}},
	}
	uris, err := trojanURIs("hy2", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertUserinfo(t, uris[0], "", specialPw)
}

func TestTUICV5URIEscapesUserinfoSpecialChars(t *testing.T) {
	cfg := map[string]interface{}{
		"users": map[string]interface{}{"c5d1300d-dd7d-4b54-a194-2b2b7332986c": specialPw},
	}
	uris, err := tuicURIs("tuic-v5", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != 1 {
		t.Fatalf("expected 1 uri, got %d", len(uris))
	}
	assertUserinfo(t, uris[0], "c5d1300d-dd7d-4b54-a194-2b2b7332986c", specialPw)
}

func TestTUICV4TokenEscapesUserinfoSpecialChars(t *testing.T) {
	cfg := map[string]interface{}{
		"token": []interface{}{"tok@en:123"},
	}
	uris, err := tuicURIs("tuic-v4", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertUserinfo(t, uris[0], "", "tok@en:123")
}

func TestShadowQUICURIEscapesUserinfoSpecialChars(t *testing.T) {
	cfg := map[string]interface{}{
		"users": []interface{}{map[string]interface{}{"username": "bob", "password": specialPw}},
	}
	uris, err := shadowQUICURIs("sq", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertUserinfo(t, uris[0], "bob", specialPw)
}

func TestMieruURIEscapesUserinfoSpecialChars(t *testing.T) {
	cfg := map[string]interface{}{
		"users": map[string]interface{}{"bob": specialPw},
	}
	uris, err := mieruURIs("n", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertUserinfo(t, uris[0], "bob", specialPw)
}

func TestTrustTunnelURIEscapesUserinfoSpecialChars(t *testing.T) {
	// TrustTunnel (like ShadowQUIC) uses the array users form.
	cfg := map[string]interface{}{
		"users": []interface{}{map[string]interface{}{"username": "bob", "password": specialPw}},
	}
	uris, err := trustTunnelURIs("n", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertUserinfo(t, uris[0], "bob", specialPw)
}

// P1-5: ss+simple-obfs must export a SIP003 plugin so clients speak the wrapper.
func TestShadowsocksURIExportsSimpleObfsPlugin(t *testing.T) {
	cfg := map[string]interface{}{
		"cipher":      "aes-128-gcm",
		"password":    "pw",
		"simple-obfs": map[string]interface{}{"enable": true, "mode": "http"},
	}
	uris, err := shadowsocksURIs("ss-obfs", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	q := parseURIRoundTrip(t, uris[0]).Query()
	if got := q.Get("plugin"); got != "obfs-local;obfs=http" {
		t.Fatalf("plugin param = %q, want obfs-local;obfs=http (uri=%s)", got, uris[0])
	}
}

func TestShadowsocksURIExportsShadowTLSPlugin(t *testing.T) {
	cfg := map[string]interface{}{
		"cipher":   "aes-128-gcm",
		"password": "pw",
		"shadow-tls": map[string]interface{}{
			"enable":    true,
			"version":   float64(3),
			"password":  "tls-pw",
			"handshake": map[string]interface{}{"dest": "www.microsoft.com:443"},
		},
	}
	uris, err := shadowsocksURIs("ss-stls", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	q := parseURIRoundTrip(t, uris[0]).Query()
	if got := q.Get("plugin"); got != "shadow-tls;host=www.microsoft.com;password=tls-pw;version=3" {
		t.Fatalf("plugin = %q (uri=%s)", got, uris[0])
	}
}

func TestShadowsocksURIPluginOrderPrefersSimpleObfs(t *testing.T) {
	// Both wrappers configured: the server applies simple-obfs after
	// shadow-tls; the URI must describe the outermost wrapper the client needs.
	cfg := map[string]interface{}{
		"cipher":      "aes-128-gcm",
		"password":    "pw",
		"simple-obfs": map[string]interface{}{"enable": true, "mode": "tls"},
		"kcp-tun":     map[string]interface{}{"enable": true, "mode": "fast"},
	}
	uris, err := shadowsocksURIs("ss", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := parseURIRoundTrip(t, uris[0]).Query().Get("plugin"); !strings.HasPrefix(got, "obfs-local") {
		t.Fatalf("plugin = %q, want obfs-local first", got)
	}
}

func TestShadowsocksURIPluginDisabledOmitted(t *testing.T) {
	cfg := map[string]interface{}{
		"cipher":      "aes-128-gcm",
		"password":    "pw",
		"simple-obfs": map[string]interface{}{"enable": false, "mode": "http"},
	}
	uris, err := shadowsocksURIs("ss", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(uris[0], "plugin=") {
		t.Fatalf("disabled wrapper must not emit plugin: %s", uris[0])
	}
}

// P3: the SIP002 userinfo base64 must use the standard alphabet so legacy
// clients (which only implement +/ ) can decode it.
func TestShadowsocksURIUsesStandardBase64Alphabet(t *testing.T) {
	cfg := map[string]interface{}{
		"cipher":   "aes-128-gcm",
		"password": "p+w/ord==",
	}
	uris, err := shadowsocksURIs("ss", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.TrimPrefix(strings.SplitN(uris[0], "@", 2)[0], "ss://")
	decoded, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		t.Fatalf("std base64 decode failed (uri=%s): %v", uris[0], err)
	}
	if string(decoded) != "aes-128-gcm:p+w/ord==" {
		t.Fatalf("decoded %q", decoded)
	}
}

// P1-5/P2-5: JSON numbers for max-udp-relay-packet-size / alterId must not be
// silently dropped by string-only type assertions.
func TestTUICURIKeepsNumericMaxUDPRelayPacketSize(t *testing.T) {
	cfg := map[string]interface{}{
		"users":                     map[string]interface{}{"2dab6251-bed1-4fa3-8c67-fb3fea4343c2": "pw"},
		"max-udp-relay-packet-size": float64(1500),
		"congestion-controller":     "bbr",
	}
	uris, err := tuicURIs("tuic-v5", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := parseURIRoundTrip(t, uris[0]).Query().Get("max_udp_relay_packet_size"); got != "1500" {
		t.Fatalf("max_udp_relay_packet_size = %q, want 1500 (uri=%s)", got, uris[0])
	}
}

func TestVMessURIKeepsNumericAlterID(t *testing.T) {
	cfg := map[string]interface{}{
		"users":   []interface{}{map[string]interface{}{"username": "user", "uuid": "7eeeeff7-7751-42fb-9f91-9efcea740b10"}},
		"alterId": float64(8),
		"cipher":  "auto",
	}
	uris, err := vmessURIs("vmess-ws", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uris[0], "vmess://"))
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["aid"] != "8" {
		t.Fatalf("aid = %q, want 8 (vmess=%s)", obj["aid"], raw)
	}
}

// P3: xhttp mode must be exported so clients do not rely on divergent defaults.
func TestVlessURIExportsXHTTPMode(t *testing.T) {
	cfg := map[string]interface{}{
		"users":        []interface{}{map[string]interface{}{"username": "user", "uuid": "7eeeeff7-7751-42fb-9f91-9efcea740b10"}},
		"xhttp-config": map[string]interface{}{"mode": "auto", "path": "/xh"},
	}
	uris, err := vlessURIs("less-xhttp", "example.com", "443", cfg)
	if err != nil {
		t.Fatal(err)
	}
	q := parseURIRoundTrip(t, uris[0]).Query()
	if got := q.Get("mode"); got != "auto" {
		t.Fatalf("mode = %q, want auto (uri=%s)", got, uris[0])
	}
	if got := q.Get("path"); got != "/xh" {
		t.Fatalf("path = %q, want /xh", got)
	}
}

// P2-4: a self-signed fallback cert whose only identity is "localhost" must
// not be advertised as SNI to clients dialling a real host.
func TestResolveURITLSNeverAdvertisesLocalhost(t *testing.T) {
	certPEM, _, err := certutil.GenerateSelfSigned("localhost")
	if err != nil {
		t.Fatal(err)
	}
	sni, _ := resolveURITLS(map[string]interface{}{"certificate": certPEM}, "example.com")
	if strings.EqualFold(sni, "localhost") {
		t.Fatalf("SNI must not be localhost when dialing example.com, got %q", sni)
	}
}
