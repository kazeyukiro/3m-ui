package protocol

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func compileSS(t *testing.T, cfg map[string]interface{}) (map[string]interface{}, error) {
	t.Helper()
	return DefaultCompileRegistry().Compile(CompileInput{
		Name: "ss", Protocol: "shadowsocks", Port: 443, Config: cfg,
	})
}

// A listener without an explicit users map must keep the historical
// single-password behaviour byte for byte (explicit opt-in, v1.19.34).
func TestSS_NoUsersKeepsLegacySinglePassword(t *testing.T) {
	out, err := compileSS(t, map[string]interface{}{"cipher": "aes-128-gcm", "password": "cfg-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if out["password"] != "cfg-secret" {
		t.Fatalf("password = %v, want cfg-secret", out["password"])
	}
	if _, ok := out["users"]; ok {
		t.Fatal("no users field may be emitted without explicit opt-in")
	}
}

func TestSS_NoUsersFallsBackToFirstBoundUser(t *testing.T) {
	out, err := DefaultCompileRegistry().Compile(CompileInput{
		Name: "ss", Protocol: "shadowsocks", Port: 443,
		Config: map[string]interface{}{"cipher": "aes-128-gcm"},
		Users:  []UserCred{{Username: "alice", Password: "alice-pw"}, {Username: "bob", Password: "bob-pw"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["password"] != "alice-pw" {
		t.Fatalf("password = %v, want alice-pw", out["password"])
	}
	if _, ok := out["users"]; ok {
		t.Fatal("no users field may be emitted without explicit opt-in")
	}
}

// AEAD multi-user: the top-level password must be empty and the users map
// is emitted as username -> password (listener/sing_shadowsocks/server.go).
func TestSS_AEADMultiUserCompile(t *testing.T) {
	out, err := compileSS(t, map[string]interface{}{
		"cipher": "aes-128-gcm",
		"users":  map[string]interface{}{"alice": "pw-a", "bob": "pw-b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	users, ok := out["users"].(map[string]string)
	if !ok || len(users) != 2 || users["alice"] != "pw-a" || users["bob"] != "pw-b" {
		t.Fatalf("users = %#v, want alice/bob map", out["users"])
	}
	if _, ok := out["password"]; ok {
		t.Fatal("AEAD multi-user must not emit a top-level password")
	}
}

// 2022-blake3-aes-* multi-user: the top-level password stays as the server
// PSK and users holds the per-user PSKs.
func TestSS_2022MultiUserCompile(t *testing.T) {
	out, err := compileSS(t, map[string]interface{}{
		"cipher":   "2022-blake3-aes-128-gcm",
		"password": "server-psk",
		"users":    map[string]string{"alice": "user-psk-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["password"] != "server-psk" {
		t.Fatalf("server PSK lost: %v", out["password"])
	}
	users, ok := out["users"].(map[string]string)
	if !ok || users["alice"] != "user-psk-a" {
		t.Fatalf("users = %#v", out["users"])
	}
}

func TestSS_MultiUserRejectsUnsupportedCipher(t *testing.T) {
	_, err := compileSS(t, map[string]interface{}{
		"cipher": "none",
		"users":  map[string]interface{}{"alice": "pw-a"},
	})
	if err == nil || !strings.Contains(err.Error(), "does not support the users field") {
		t.Fatalf("expected unsupported-method error, got %v", err)
	}
}

func TestSS_MultiUserRejects2022Chacha(t *testing.T) {
	_, err := compileSS(t, map[string]interface{}{
		"cipher":   "2022-blake3-chacha20-poly1305",
		"password": "server-psk",
		"users":    map[string]interface{}{"alice": "pw-a"},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot identify users") {
		t.Fatalf("expected chacha rejection, got %v", err)
	}
}

func TestValidateSSMultiUser_Rules(t *testing.T) {
	cases := []struct {
		name     string
		cfg      map[string]interface{}
		wantErr  string
		wantMode string
	}{
		{"no users is a no-op", map[string]interface{}{"cipher": "aes-128-gcm", "password": "x"}, "", ""},
		{"AEAD ok", map[string]interface{}{"cipher": "aes-256-gcm", "users": map[string]interface{}{"a": "p"}}, "", "aead"},
		{"AEAD with password rejected", map[string]interface{}{"cipher": "aes-256-gcm", "password": "x", "users": map[string]interface{}{"a": "p"}}, "empty top-level password", ""},
		{"2022 ok", map[string]interface{}{"cipher": "2022-blake3-aes-256-gcm", "password": "srv", "users": map[string]interface{}{"a": "p"}}, "", "2022"},
		{"2022 missing PSK", map[string]interface{}{"cipher": "2022-blake3-aes-256-gcm", "users": map[string]interface{}{"a": "p"}}, "server PSK", ""},
		{"empty user password", map[string]interface{}{"cipher": "aes-128-gcm", "users": map[string]interface{}{"a": ""}}, "empty password", ""},
		{"none rejected", map[string]interface{}{"cipher": "none", "users": map[string]interface{}{"a": "p"}}, "does not support", ""},
		{"chacha rejected", map[string]interface{}{"cipher": "2022-blake3-chacha20-poly1305", "password": "s", "users": map[string]interface{}{"a": "p"}}, "cannot identify users", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, mode, err := ValidateSSMultiUser(c.cfg)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if mode != c.wantMode {
					t.Fatalf("mode = %q, want %q", mode, c.wantMode)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want error containing %q, got %v", c.wantErr, err)
				}
			}
		})
	}
}

func TestResolveSSClientPassword(t *testing.T) {
	aead := map[string]interface{}{"cipher": "aes-128-gcm", "users": map[string]interface{}{"alice": "pw-a"}}
	if pass, ok := ResolveSSClientPassword(aead, "alice"); !ok || pass != "pw-a" {
		t.Fatalf("AEAD client password = %q, %v", pass, ok)
	}
	if _, ok := ResolveSSClientPassword(aead, "bob"); ok {
		t.Fatal("unknown username must not resolve")
	}
	y2022 := map[string]interface{}{
		"cipher": "2022-blake3-aes-128-gcm", "password": "srv",
		"users": map[string]interface{}{"alice": "u-psk"},
	}
	if pass, ok := ResolveSSClientPassword(y2022, "alice"); !ok || pass != "srv:u-psk" {
		t.Fatalf("2022 client password = %q, %v; want srv:u-psk", pass, ok)
	}
	if _, ok := ResolveSSClientPassword(map[string]interface{}{"cipher": "aes-128-gcm"}, "alice"); ok {
		t.Fatal("non-multi-user config must not resolve")
	}
}

func ssShareURI(t *testing.T, cfgJSON string, u UserCred) string {
	t.Helper()
	l := models.Listener{
		Name: "ss", Protocol: "shadowsocks", Type: "shadowsocks", Port: "443",
		BindAddress: "0.0.0.0", PublicHost: "example.com", Enabled: true,
		Config: cfgJSON,
	}
	shares, err := ExportShares(l, "example.com", []UserCred{u})
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 1 {
		t.Fatalf("shares = %d, want 1", len(shares))
	}
	return shares[0].URI
}

func TestSS_ShareURI_AEADMultiUser(t *testing.T) {
	uri := ssShareURI(t, `{"cipher":"aes-128-gcm","users":{"alice":"pw-a","bob":"pw-b"}}`,
		UserCred{Username: "alice", Password: "panel-password"})
	want := base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:pw-a"))
	if !strings.Contains(uri, want) {
		t.Fatalf("AEAD share URI must carry alice's password, got %s", uri)
	}
	if strings.Contains(uri, base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:panel-password"))) {
		t.Fatal("AEAD share URI must not use the panel user password")
	}
}

func TestSS_ShareURI_2022MultiUser(t *testing.T) {
	uri := ssShareURI(t, `{"cipher":"2022-blake3-aes-128-gcm","password":"srv-psk","users":{"alice":"u-psk"}}`,
		UserCred{Username: "alice", Password: "panel-password"})
	want := base64.RawURLEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:srv-psk:u-psk"))
	if !strings.Contains(uri, want) {
		t.Fatalf("2022 share URI must carry serverPSK:userPSK, got %s", uri)
	}
}
