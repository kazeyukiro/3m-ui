package certutil

import (
	"strings"
	"testing"
)

func TestGenerateSelfSignedAndDetect(t *testing.T) {
	cert, key, err := GenerateSelfSigned("example.com", "1.2.3.4", "localhost")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cert, "BEGIN CERTIFICATE") || !strings.Contains(key, "BEGIN EC PRIVATE KEY") {
		t.Fatalf("unexpected pem shapes")
	}
	if !IsPanelSelfSignedPEM(cert) {
		t.Fatal("expected panel org tag")
	}
	if !ShouldSkipCertVerify(map[string]interface{}{"certificate": cert}) {
		t.Fatal("self-signed should skip verify")
	}
	if !ShouldSkipCertVerify(map[string]interface{}{"skip-cert-verify": true}) {
		t.Fatal("explicit skip")
	}
	if IsPanelSelfSignedPEM("") {
		t.Fatal("empty should be false")
	}
}

func TestDecideClientSkipCertVerify(t *testing.T) {
	cert, _, err := GenerateSelfSigned("localhost")
	if err != nil {
		t.Fatal(err)
	}
	// Panel self-signed → skip even when connecting via public host
	if !DecideClientSkipCertVerify(cert, "dzx.qzz.io", nil) {
		t.Fatal("panel cert should skip for public host")
	}
	f := false
	if DecideClientSkipCertVerify(cert, "dzx.qzz.io", &f) {
		t.Fatal("explicit false must win")
	}
	tr := true
	if !DecideClientSkipCertVerify(cert, "dzx.qzz.io", &tr) {
		t.Fatal("explicit true must win")
	}
	// No PEM → skip (panel default)
	if !DecideClientSkipCertVerify("", "dzx.qzz.io", nil) {
		t.Fatal("empty cert should skip")
	}
}

func TestHostHintsFromConfig(t *testing.T) {
	h := HostHintsFromConfig(map[string]interface{}{
		"sni":          "a.example",
		"server-names": []interface{}{"b.example"},
	})
	if len(h) < 2 {
		t.Fatalf("hints=%v", h)
	}
}
