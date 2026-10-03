package export

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestIPHostWithoutPEMUsesIPAndSkips(t *testing.T) {
	l := models.Listener{
		AccessSNI: "www.bing.com",
		Port:      "443",
	}
	cfg := map[string]interface{}{}
	p := BuildProfileFromConfig(l, "85.149.212.214", cfg)
	if p.SNI != "85.149.212.214" {
		t.Fatalf("SNI=%q want IP (not AccessSNI domain)", p.SNI)
	}
	if !p.SkipCert {
		t.Fatal("expected skip-cert on IP without PEM")
	}
}

func TestRealityApplyDoesNotOverwriteSNI(t *testing.T) {
	cfg := map[string]interface{}{
		"reality-config": map[string]interface{}{
			"server-names": []interface{}{"www.microsoft.com"},
		},
		"sni": "www.microsoft.com",
	}
	l := models.Listener{Port: "443"}
	prof := BuildProfileFromConfig(l, "1.2.3.4", cfg)
	ApplyToConfig(cfg, l, prof)
	if cfg["sni"] != "www.microsoft.com" {
		t.Fatalf("reality sni stomped to %v", cfg["sni"])
	}
	if cfg["skip-cert-verify"] != true {
		t.Fatal("reality must skip verify")
	}
}
