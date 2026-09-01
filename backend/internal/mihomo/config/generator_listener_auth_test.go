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
	if len(got) != 1 || got[0]["tls"] != true {
		t.Fatalf("expected tls=true, got %#v", got)
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
