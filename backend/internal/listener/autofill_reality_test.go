package listener_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/listener"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
)

func TestRealityRequiresExplicitDestination(t *testing.T) {
	for _, config := range []string{`{"reality-config":{}}`, `{"security_layer":"reality"}`, `{"reality-config":{"dest":"  "},"sni":"www.example.com"}`} {
		l := &models.Listener{Protocol: "vless", PublicHost: "public.example.com", Config: config}
		if err := listener.AutofillListenerDefaults(l); err == nil || !strings.Contains(err.Error(), "destination is required") {
			t.Fatalf("expected missing destination error, got %v", err)
		}
	}
}

func TestRealityPreservesDestinationAndCredentialsOnSave(t *testing.T) {
	l := &models.Listener{Protocol: "vless", Config: `{"reality-config":{"dest":"chosen.example.com:443"}}`}
	if err := listener.AutofillListenerDefaults(l); err != nil {
		t.Fatal(err)
	}
	first := l.Config
	if err := listener.AutofillListenerDefaults(l); err != nil {
		t.Fatal(err)
	}
	if l.Config != first {
		t.Fatal("save rotated target or credentials")
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(l.Config), &cfg); err != nil {
		t.Fatal(err)
	}
	r := cfg["reality-config"].(map[string]interface{})
	if r["dest"] != "chosen.example.com:443" || r["server-names"].([]interface{})[0] != "chosen.example.com" {
		t.Fatalf("unexpected target/SNI: %+v", r)
	}
}

func TestRealityDerivesNamesAfterManualTargetChange(t *testing.T) {
	// The form clears names owned by its previous scan when Dest is edited.
	l := &models.Listener{
		Protocol: "vless", Port: "443", PublicHost: "public.example.com",
		Config: `{"reality-config":{"dest":"changed.example.com:443","server-names":[]}}`,
	}
	if err := listener.AutofillListenerDefaults(l); err != nil {
		t.Fatal(err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(l.Config), &cfg); err != nil {
		t.Fatal(err)
	}
	names := cfg["reality-config"].(map[string]interface{})["server-names"].([]interface{})
	if len(names) != 1 || names[0] != "changed.example.com" {
		t.Fatalf("saved names do not match the new target: %v", names)
	}
	uris, err := node.ClientURIs(*l, l.PublicHost)
	if err != nil || len(uris) != 1 {
		t.Fatalf("export URI: %v, %v", uris, err)
	}
	u, err := url.Parse(uris[0])
	if err != nil {
		t.Fatal(err)
	}
	if sni := u.Query().Get("sni"); sni != "changed.example.com" {
		t.Fatalf("exported SNI = %q, want the new target", sni)
	}
}
