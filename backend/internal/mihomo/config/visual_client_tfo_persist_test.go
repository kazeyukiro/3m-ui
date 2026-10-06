package config_test

import (
	"strings"
	"testing"

	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
)

func TestVisualConfigPersistsClientAndInboundTfo(t *testing.T) {
	db := lowMemoryTestDB(t)
	vc := mihomoConfig.DefaultVisualConfig()
	vc.InboundTfo = true
	vc.InboundMPTCP = true
	vc.ClientTfo = true
	vc.ClientMPTCP = true
	if err := mihomoConfig.SaveVisualConfig(db, vc); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := mihomoConfig.GetVisualConfig(db)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.InboundTfo || !got.InboundMPTCP {
		t.Fatalf("inbound flags lost: inboundTfo=%v inboundMptcp=%v", got.InboundTfo, got.InboundMPTCP)
	}
	if !got.ClientTfo || !got.ClientMPTCP {
		t.Fatalf("client flags lost: clientTfo=%v clientMptcp=%v", got.ClientTfo, got.ClientMPTCP)
	}

	out, err := mihomoConfig.NewConfigEngine(db).GenerateFinalConfig()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(out, "inbound-tfo: true") {
		t.Fatalf("server config missing inbound-tfo:\n%s", out)
	}
	if strings.Contains(out, "client-tfo") || strings.Contains(out, "client-mptcp") {
		t.Fatalf("client-* must not appear in serving config:\n%s", out)
	}
}
