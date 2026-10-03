package export

import (
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestIPHostWithoutPEMKeepsAccessSNIAndSkips(t *testing.T) {
	// Without PEM, domain AccessSNI is still the SNI clients should send (HY2-style);
	// skip-cert must be true because the dial target is a bare IP.
	l := models.Listener{
		AccessSNI: "www.bing.com",
		Port:      "443",
	}
	cfg := map[string]interface{}{}
	p := BuildProfileFromConfig(l, "85.149.212.214", cfg)
	if p.SNI != "www.bing.com" {
		t.Fatalf("SNI=%q want AccessSNI domain", p.SNI)
	}
	if !p.SkipCert {
		t.Fatal("expected skip-cert on IP without PEM")
	}
}

func TestIPHostWithoutPEMFallsBackToIP(t *testing.T) {
	l := models.Listener{Port: "443"}
	p := BuildProfileFromConfig(l, "85.149.212.214", nil)
	if p.SNI != "85.149.212.214" {
		t.Fatalf("SNI=%q want connect IP", p.SNI)
	}
	if !p.SkipCert {
		t.Fatal("expected skip-cert")
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
