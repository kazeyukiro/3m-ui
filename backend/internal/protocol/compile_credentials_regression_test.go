package protocol

import (
	"testing"
)

// P1-1: a multi-user bind on a single-secret listener must never take the
// listener offline. The runtime resolves one password deterministically:
// config secret first (fallback: first bound user).
func TestShadowsocksCompilerMultiUserDegradesGracefully(t *testing.T) {
	in := CompileInput{
		Name:     "ss",
		Protocol: "shadowsocks",
		Port:     443,
		Config:   map[string]interface{}{"cipher": "aes-128-gcm", "password": "cfg-secret"},
		Users:    []UserCred{{Username: "alice", Password: "alice-pw"}, {Username: "bob", Password: "bob-pw"}},
	}
	out, err := DefaultCompileRegistry().Compile(in)
	if err != nil {
		t.Fatalf("multi-user shadowsocks must not fail the listener: %v", err)
	}
	if out["password"] != "cfg-secret" {
		t.Fatalf("password = %v, want cfg-secret (config wins over multiple users)", out["password"])
	}
}

func TestShadowsocksCompilerMultiUserFallsBackToFirstUser(t *testing.T) {
	in := CompileInput{
		Name:     "ss",
		Protocol: "shadowsocks",
		Port:     443,
		Config:   map[string]interface{}{"cipher": "aes-128-gcm"},
		Users:    []UserCred{{Username: "alice", Password: "alice-pw"}, {Username: "bob", Password: "bob-pw"}},
	}
	out, err := DefaultCompileRegistry().Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	if out["password"] != "alice-pw" {
		t.Fatalf("password = %v, want alice-pw (first user fallback)", out["password"])
	}
}

// P1-1: the config secret is the source of truth for single-secret protocols.
// A single bound user must NOT override it — the live inbound authenticates
// with the config secret, and the exporters derive it the same way, so an
// override would desync the subscription from the running listener.
func TestShadowsocksCompilerConfigSecretWinsOverSingleUser(t *testing.T) {
	in := CompileInput{
		Name:     "ss",
		Protocol: "shadowsocks",
		Port:     443,
		Config:   map[string]interface{}{"cipher": "aes-128-gcm", "password": "cfg-secret"},
		Users:    []UserCred{{Username: "alice", Password: "alice-pw"}},
	}
	out, err := DefaultCompileRegistry().Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	if out["password"] != "cfg-secret" {
		t.Fatalf("password = %v, want cfg-secret (config secret wins over a single bound user)", out["password"])
	}
}

func TestSnellCompilerKeepsConfigPSKOverSingleUser(t *testing.T) {
	in := CompileInput{
		Name:     "snell",
		Protocol: "snell",
		Port:     443,
		Config:   map[string]interface{}{"psk": "cfg-psk", "version": "4"},
		Users:    []UserCred{{Username: "alice", Password: "alice-pw"}},
	}
	out, err := DefaultCompileRegistry().Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	if out["psk"] != "cfg-psk" {
		t.Fatalf("psk = %v, want cfg-psk (config secret wins)", out["psk"])
	}
	// Runtime must agree with the URI/client-YAML exporters (P1-1).
	if out["password"] != nil {
		t.Fatalf("snell listener must not carry a password field, got %v", out["password"])
	}
}

func TestSnellCompilerMultiUserKeepsConfigPSK(t *testing.T) {
	in := CompileInput{
		Name:     "snell",
		Protocol: "snell",
		Port:     443,
		Config:   map[string]interface{}{"psk": "cfg-psk"},
		Users:    []UserCred{{Username: "alice", Password: "alice-pw"}, {Username: "bob", Password: "bob-pw"}},
	}
	out, err := DefaultCompileRegistry().Compile(in)
	if err != nil {
		t.Fatalf("multi-user snell must not fail: %v", err)
	}
	if out["psk"] != "cfg-psk" {
		t.Fatalf("psk = %v, want cfg-psk", out["psk"])
	}
}

func TestSudokuCompilerKeepsConfigKeyOverSingleUser(t *testing.T) {
	in := CompileInput{
		Name:     "sudoku",
		Protocol: "sudoku",
		Port:     443,
		Config:   map[string]interface{}{"key": "cfg-key"},
		Users:    []UserCred{{Username: "alice", Password: "alice-pw"}},
	}
	out, err := DefaultCompileRegistry().Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	if out["key"] != "cfg-key" {
		t.Fatalf("key = %v, want cfg-key (config secret wins)", out["key"])
	}
}

// P1-4: a scalar alpn (legacy panel JSON) must be normalized to the string
// list Mihomo validates, or the whole listeners array fails to load.
func TestRegistryNormalizesScalarAlpn(t *testing.T) {
	in := CompileInput{
		Name:     "hy2",
		Protocol: "hysteria2",
		Port:     443,
		Config:   map[string]interface{}{"alpn": "h3", "password": "pw"},
	}
	out, err := DefaultCompileRegistry().Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := out["alpn"].([]string)
	if !ok || len(list) != 1 || list[0] != "h3" {
		t.Fatalf("alpn = %#v, want []string{\"h3\"}", out["alpn"])
	}
}

func TestRegistryNormalizesListAlpnWithBlanks(t *testing.T) {
	in := CompileInput{
		Name:     "hy2",
		Protocol: "hysteria2",
		Port:     443,
		Config:   map[string]interface{}{"alpn": []interface{}{"h3", "", "h2"}},
	}
	out, err := DefaultCompileRegistry().Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := out["alpn"].([]string)
	if !ok || len(list) != 2 || list[0] != "h3" || list[1] != "h2" {
		t.Fatalf("alpn = %#v, want [h3 h2]", out["alpn"])
	}
}

func TestRegistryLeavesProtocolsWithoutAlpnAlone(t *testing.T) {
	in := CompileInput{
		Name:     "vless",
		Protocol: "vless",
		Port:     443,
		Config:   map[string]interface{}{},
		Users:    []UserCred{{Username: "u", UUID: "7eeeeff7-7751-42fb-9f91-9efcea740b10"}},
	}
	out, err := DefaultCompileRegistry().Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := out["alpn"]; ok {
		t.Fatalf("unexpected alpn on vless listener: %#v", v)
	}
}

func TestNormalizeAlpnValueEdgeCases(t *testing.T) {
	got := normalizeAlpnValue("  ")
	list, ok := got.([]string)
	if !ok || len(list) != 0 {
		t.Fatalf("blank scalar -> %#v, want empty []string", got)
	}
	if passthrough := normalizeAlpnValue(42); passthrough != 42 {
		t.Fatalf("unexpected scalar type must pass through, got %#v", passthrough)
	}
}

// Sanity: the single-secret rule shared by runtime/export/URI resolution.
func TestResolveSingleSecretRule(t *testing.T) {
	m := map[string]interface{}{"psk": "cfg"}
	if _, ok := resolveSingleSecret(m, []UserCred{{Password: "u"}}, "psk"); ok {
		t.Fatal("config secret present must stand, even for a single user")
	}
	if _, ok := resolveSingleSecret(m, nil, "psk"); ok {
		t.Fatal("config secret present + zero users must keep the config secret")
	}
	if v, ok := resolveSingleSecret(map[string]interface{}{}, []UserCred{{Password: "a"}, {Password: "b"}}, "psk"); !ok || v != "a" {
		t.Fatalf("no config secret + many users → first user: %q ok=%v", v, ok)
	}
	if _, ok := resolveSingleSecret(map[string]interface{}{"psk": "cfg"}, []UserCred{{Password: "a"}, {Password: "b"}}, "psk"); ok {
		t.Fatal("config secret present + many users → keep config")
	}
	if v, ok := resolveSingleSecret(map[string]interface{}{}, nil, "psk"); ok || v != "" {
		t.Fatalf("nothing at all → no secret: %q ok=%v", v, ok)
	}
}
