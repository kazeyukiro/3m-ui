package config

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestGenerateListenersUsesNativeSchema(t *testing.T) {
	listeners := []models.Listener{
		{BaseModel: models.BaseModel{ID: 1}, Name: "ss", Protocol: "shadowsocks", Port: "443", BindAddress: "0.0.0.0", Enabled: true, Config: `{"cipher":"aes-256-gcm"}`},
		{BaseModel: models.BaseModel{ID: 2}, Name: "vless", Protocol: "vless", Port: "8443", BindAddress: "0.0.0.0", Enabled: true, Config: `{"flow":"xtls-rprx-vision","certificate":"cert","private-key":"key","reality-config":{"dest":"example.com:443","private-key":"secret","short-id":["0123456789abcdef"],"server-names":["example.com"]}}`},
	}
	creds := map[uint][]Credential{
		1: {{Username: "alice", Password: "ss-pass"}},
		2: {{Username: "alice", UUID: "11111111-1111-4111-8111-111111111111"}},
	}

	result, err := generateListeners(nil, listeners, creds)
	if err != nil {
		t.Fatalf("generateListeners failed: %v", err)
	}
	if result[0]["password"] != "ss-pass" {
		t.Fatal("Shadowsocks listener password was not generated")
	}
	vlessUsers, ok := result[1]["users"].([]map[string]interface{})
	if !ok || len(vlessUsers) != 1 || vlessUsers[0]["uuid"] == "" {
		t.Fatal("VLESS listener users were not generated")
	}
	if result[1]["tls"] != nil {
		t.Fatal("listener TLS must not use a nested tls object")
	}
	if result[1]["certificate"] != "cert" || result[1]["private-key"] != "key" {
		t.Fatal("listener certificate/private-key fields were not preserved")
	}
	reality, ok := result[1]["reality-config"].(map[string]interface{})
	if !ok {
		t.Fatalf("reality-config was not preserved as an object: %#v", result[1]["reality-config"])
	}
	if reality["dest"] != "example.com:443" || reality["private-key"] != "secret" {
		t.Fatalf("Reality dest/private-key were not preserved: %#v", reality)
	}
	if !isStringList(reality["short-id"]) {
		t.Fatalf("Reality short-id was not preserved as a list: %#v", reality["short-id"])
	}
	if !isStringList(reality["server-names"]) {
		t.Fatalf("Reality server-names was not preserved as a list: %#v", reality["server-names"])
	}
}

// JSON/YAML decode typically yields []interface{}, not []string.
func isStringList(v interface{}) bool {
	switch xs := v.(type) {
	case []string:
		return len(xs) > 0
	case []interface{}:
		if len(xs) == 0 {
			return false
		}
		for _, item := range xs {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func TestGenerateShadowQUICNormalizesObjectUsers(t *testing.T) {
	listeners := []models.Listener{{
		Name:        "shadowquic",
		Protocol:    "shadowquic",
		Port:        "8443",
		BindAddress: "0.0.0.0",
		Enabled:     true,
		Config:      `{"users":{"alice":"secret","bob":"secret2"}}`,
	}}

	result, err := generateListeners(nil, listeners, nil)
	if err != nil {
		t.Fatalf("generateListeners failed: %v", err)
	}
	users, ok := result[0]["users"].([]map[string]interface{})
	if !ok || len(users) != 2 {
		t.Fatalf("ShadowQUIC users must be a list, got %T", result[0]["users"])
	}
	for _, user := range users {
		if user["username"] == "" || user["password"] == "" {
			t.Fatalf("invalid normalized ShadowQUIC user: %#v", user)
		}
	}
}

func TestGenerateShadowQUICUsesCredentialsAsList(t *testing.T) {
	listeners := []models.Listener{{
		BaseModel:   models.BaseModel{ID: 1},
		Name:        "shadowquic",
		Protocol:    "shadowquic",
		Port:        "8443",
		BindAddress: "0.0.0.0",
		Enabled:     true,
	}}
	creds := map[uint][]Credential{
		1: {{Username: "alice", Password: "secret"}},
	}

	result, err := generateListeners(nil, listeners, creds)
	if err != nil {
		t.Fatalf("generateListeners failed: %v", err)
	}
	users, ok := result[0]["users"].([]map[string]interface{})
	if !ok || len(users) != 1 || users[0]["username"] != "alice" || users[0]["password"] != "secret" {
		t.Fatalf("unexpected ShadowQUIC users: %#v", result[0]["users"])
	}
}

func TestGenerateListenersRejectsExcludedProtocols(t *testing.T) {
	for _, protocol := range []string{"socks", "http", "tproxy", "redir", "mixed", "tunnel", "tun", "wireguard"} {
		_, err := generateListeners(nil, []models.Listener{{Name: "bad", Protocol: protocol, Port: "1080", Enabled: true}}, nil)
		if err == nil {
			t.Fatalf("expected protocol %q to be rejected", protocol)
		}
	}
}

func TestGenerateListenersSkipsDisabled(t *testing.T) {
	result, err := generateListeners(nil, []models.Listener{
		{Name: "on", Protocol: "shadowsocks", Port: "1080", Enabled: true, Config: `{"cipher":"aes-256-gcm","password":"x"}`},
		{Name: "off", Protocol: "vless", Port: "1081", Enabled: false},
	}, nil)
	if err != nil {
		t.Fatalf("generateListeners failed: %v", err)
	}
	if len(result) != 1 || result[0]["name"] != "on" {
		t.Fatalf("expected only enabled listener, got %#v", result)
	}
}
