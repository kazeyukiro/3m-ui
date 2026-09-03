package config

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestGenerateListenersUsesModelTLS(t *testing.T) {
	listeners := []models.Listener{{BaseModel: models.BaseModel{ID: 1}, Name: "vless", Protocol: "vless", Port: "443", TLS: true, Enabled: true}}
	got, err := generateListeners(nil, listeners, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one compiled listener, got %d", len(got))
	}
	// Per official MetaCubeX wiki (inbound-vless), the listener YAML does NOT
	// carry a top-level `tls` field — TLS is implied by certificate/private-key
	// or by a TLS wrapper (reality-config / shadow-tls / res-tls / jls-config).
	// The model.TLS flag is consumed by the panel for UI hints only.
	if _, present := got[0]["tls"]; present {
		t.Fatalf("listener yaml must not carry top-level tls field (wiki-correct), got %#v", got[0])
	}
	if got[0]["type"] != "vless" || got[0]["port"] != 443 || got[0]["name"] != "vless" {
		t.Fatalf("unexpected listener fields: %#v", got[0])
	}
}

func TestGenerateListenersFallsBackToConfigUsersWhenCredentialStateIsExplicitlyEmpty(t *testing.T) {
	// When the credential provider reports hasCredState=true but the panel has
	// no active credentials for a listener (e.g. all bound users blocked /
	// expired), the compiled listener must fall back to the users already
	// embedded in the listener Config. Returning nil users here would make
	// mihomo reject the entire config ("unset fields: users"), taking down
	// every other listener. Falling back to config users keeps the listener
	// valid; the enforcer is responsible for disabling the listener when all
	// panel users are inactive.
	listeners := []models.Listener{{BaseModel: models.BaseModel{ID: 1}, Name: "vless", Protocol: "vless", Port: "443", Enabled: true, Config: `{"users":[{"uuid":"legacy"}]}`}}
	got, err := generateListeners(nil, listeners, map[uint][]Credential{1: {}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one compiled listener, got %d", len(got))
	}
	users, ok := got[0]["users"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected config users to be present as fallback, got %#v", got[0]["users"])
	}
	if len(users) != 1 {
		t.Fatalf("expected one fallback user, got %d (%#v)", len(users), users)
	}
	if uid, _ := users[0]["uuid"].(string); uid != "legacy" {
		t.Fatalf("expected legacy config user uuid %q, got %#v", "legacy", users[0])
	}
}
