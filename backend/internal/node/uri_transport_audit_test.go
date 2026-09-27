package node

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// vmessURIs builds a v2rayN-style JSON blob, so its fields have to be checked
// through the base64 payload rather than a query string.
func decodeVMess(t *testing.T, uri string) map[string]interface{} {
	t.Helper()
	if !strings.HasPrefix(uri, "vmess://") {
		t.Fatalf("not a vmess URI: %s", uri)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "vmess://"))
	if err != nil {
		t.Fatalf("decode vmess payload: %v", err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal vmess payload: %v (%s)", err, raw)
	}
	return obj
}

func vmessURI(t *testing.T, cfg map[string]interface{}) string {
	t.Helper()
	uris, err := vmessURIs("node", "example.com", "443", cfg)
	if err != nil {
		t.Fatalf("vmessURIs: %v", err)
	}
	if len(uris) != 1 {
		t.Fatalf("expected 1 URI, got %d", len(uris))
	}
	return uris[0]
}

func vmessCfg() map[string]interface{} {
	return map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"username": "u", "uuid": testUUID},
		},
	}
}

// The Host header of a ws listener is separate from its path. It used to be
// dropped, so the exported link could not reach a listener that pins a Host —
// while the client YAML for the very same listener worked.
func TestVMessURICarriesWebSocketHost(t *testing.T) {
	cfg := vmessCfg()
	cfg["ws-path"] = "/ws"
	cfg["ws-headers"] = map[string]interface{}{"Host": "cdn.example.com"}

	obj := decodeVMess(t, vmessURI(t, cfg))
	if obj["net"] != "ws" {
		t.Fatalf("net = %v, want ws", obj["net"])
	}
	if obj["path"] != "/ws" {
		t.Fatalf("path = %v, want /ws", obj["path"])
	}
	if obj["host"] != "cdn.example.com" {
		t.Fatalf("host = %v, want the configured ws Host", obj["host"])
	}
}

func TestVMessURIOmitsWebSocketHostWhenUnset(t *testing.T) {
	cfg := vmessCfg()
	cfg["ws-path"] = "/ws"

	obj := decodeVMess(t, vmessURI(t, cfg))
	if _, present := obj["host"]; present {
		t.Fatalf("host must stay absent when no Host header is configured: %#v", obj)
	}
}

// vless and trojan carried allowInsecure in the query string; vmess dropped it,
// so a listener whose certificate the client cannot verify exported a link that
// failed the handshake.
func TestVMessURICarriesAllowInsecure(t *testing.T) {
	cfg := vmessCfg()
	cfg["skip-cert-verify"] = true
	cfg["certificate"] = "cert"

	obj := decodeVMess(t, vmessURI(t, cfg))
	if obj["allowInsecure"] != "1" {
		t.Fatalf("allowInsecure = %v, want 1", obj["allowInsecure"])
	}
}

// A hysteria2 listener stores its users either as rows or as a map. Both branches
// produced different TLS flags for the same listener, so the exported link
// depended on the storage shape.
func TestHysteria2URIsAgreeAcrossUserShapes(t *testing.T) {
	common := map[string]interface{}{
		"skip-cert-verify": true,
		"up":               "100 Mbps",
	}
	rows := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"username": "u", "password": "p"},
		},
	}
	mapped := map[string]interface{}{
		"users": map[string]interface{}{"u": "p"},
	}
	for k, v := range common {
		rows[k] = v
		mapped[k] = v
	}

	want := map[string]string{"insecure": "1", "allowInsecure": "1"}
	for name, cfg := range map[string]map[string]interface{}{"rows": rows, "map": mapped} {
		t.Run(name, func(t *testing.T) {
			uris, err := hysteria2URIs("node", "example.com", "443", cfg)
			if err != nil {
				t.Fatalf("hysteria2URIs: %v", err)
			}
			if len(uris) != 1 {
				t.Fatalf("expected 1 URI, got %d", len(uris))
			}
			parsed, err := url.Parse(uris[0])
			if err != nil {
				t.Fatalf("parse URI: %v", err)
			}
			values := parsed.Query()
			for key, want := range want {
				if values.Get(key) != want {
					t.Fatalf("%s = %q, want %q: %s", key, values.Get(key), want, uris[0])
				}
			}
		})
	}
}
