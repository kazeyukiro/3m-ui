package config

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestGenerateListenersUsesModelTLS(t *testing.T) {
	listeners := []models.Listener{{ID: 1, Name: "vless", Protocol: "vless", Port: "443", TLS: true, Enabled: true}}
	got, err := generateListeners(listeners, nil)
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0]["tls"] != true { t.Fatalf("expected tls=true, got %#v", got) }
}

func TestGenerateListenersDoesNotFallbackWhenCredentialStateIsExplicitlyEmpty(t *testing.T) {
	listeners := []models.Listener{{ID: 1, Name: "vless", Protocol: "vless", Port: "443", Enabled: true, Config: `{"users":[{"uuid":"legacy"}]}`}}
	got, err := generateListeners(listeners, map[uint][]Credential{1: {}})
	if err != nil { t.Fatal(err) }
	if _, ok := got[0]["users"]; ok { t.Fatalf("legacy users leaked into explicitly empty credential state: %#v", got[0]["users"]) }
}
