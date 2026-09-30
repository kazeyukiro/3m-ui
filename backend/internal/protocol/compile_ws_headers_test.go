package protocol

import "testing"

// Official VLESS/VMess listeners only accept ws-path for WebSocket, not
// ws-headers (client-side Host goes in share URI / ws-opts.headers).
func TestCompileVLESSStripsWSHeaders(t *testing.T) {
	cfg := map[string]interface{}{
		"ws-path":        "/ws",
		"ws-headers":     map[string]interface{}{"Host": "cdn.example.com"},
		"allow-insecure": true,
	}
	reg := DefaultCompileRegistry()
	m, err := reg.Compile(CompileInput{
		Name:     "VLESS-WS",
		Protocol: "vless",
		Listen:   "0.0.0.0",
		Port:     10000,
		Config:   cfg,
		Users:    []UserCred{{UUID: "9d0cb9d0-964f-4ef6-897d-6c6b3ccf9e68", Username: "u1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["ws-headers"]; ok {
		t.Fatalf("ws-headers must not appear on listener: %#v", m)
	}
	if m["ws-path"] != "/ws" {
		t.Fatalf("ws-path should remain: %#v", m["ws-path"])
	}
	// Panel config keeps headers for share/export.
	if cfg["ws-headers"] == nil {
		t.Fatal("input ws-headers must stay for client export")
	}
}
