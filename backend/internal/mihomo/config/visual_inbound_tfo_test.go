package config_test

import (
	"strings"
	"testing"

	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
)

// inbound-tfo / inbound-mptcp are global general keys Mihomo applies to every
// inbound listener socket. They reach the serving config through the
// visual-config fragment, so toggling them in the panel must surface in the
// generated YAML exactly as Mihomo expects.
func TestGenerateFinalConfigIncludesInboundTfo(t *testing.T) {
	db := lowMemoryTestDB(t)
	vc := mihomoConfig.DefaultVisualConfig()
	vc.InboundTfo = true
	vc.InboundMPTCP = true
	if err := mihomoConfig.SaveVisualConfig(db, vc); err != nil {
		t.Fatalf("save visual config: %v", err)
	}
	out, err := mihomoConfig.NewConfigEngine(db).GenerateFinalConfig()
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	if !strings.Contains(out, "inbound-tfo: true") {
		t.Fatalf("expected inbound-tfo: true in generated config:\n%s", out)
	}
	if !strings.Contains(out, "inbound-mptcp: true") {
		t.Fatalf("expected inbound-mptcp: true in generated config:\n%s", out)
	}
}

// When the switch is off the key must not flip to `true` in the serving config
// (it stays false / off, matching Mihomo's default — visual-config mirrors the
// always-present allow-lan behaviour rather than omitting a disabled flag).
func TestGenerateFinalConfigOmitsInboundTfoWhenDisabled(t *testing.T) {
	db := lowMemoryTestDB(t)
	vc := mihomoConfig.DefaultVisualConfig()
	vc.InboundTfo = false
	vc.InboundMPTCP = false
	if err := mihomoConfig.SaveVisualConfig(db, vc); err != nil {
		t.Fatalf("save visual config: %v", err)
	}
	out, err := mihomoConfig.NewConfigEngine(db).GenerateFinalConfig()
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	if strings.Contains(out, "inbound-tfo: true") {
		t.Fatalf("inbound-tfo must not be true when disabled:\n%s", out)
	}
	if strings.Contains(out, "inbound-mptcp: true") {
		t.Fatalf("inbound-mptcp must not be true when disabled:\n%s", out)
	}
}
