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
	cert, _, err := GenerateSelfSigned("dzx.qzz.io")
	if err != nil {
		t.Fatal(err)
	}
	if !DecideClientSkipCertVerify(cert, "dzx.qzz.io", nil) {
		t.Fatal("panel self-signed must skip")
	}
	f := false
	if DecideClientSkipCertVerify(cert, "dzx.qzz.io", &f) {
		t.Fatal("explicit false must win")
	}
	tr := true
	if !DecideClientSkipCertVerify(cert, "dzx.qzz.io", &tr) {
		t.Fatal("explicit true must win")
	}
	// Domain connect without embedded PEM: assume public CA (do not force insecure).
	if DecideClientSkipCertVerify("", "dzx.qzz.io", nil) {
		t.Fatal("empty PEM + domain must NOT skip")
	}
	// IP-only without PEM: still skip.
	if !DecideClientSkipCertVerify("", "1.2.3.4", nil) {
		t.Fatal("empty PEM + IP must skip")
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

func TestResolveClientSNIIgnoresMismatchedAccessSNI(t *testing.T) {
	// Simulate LE IP cert: only IP SAN, AccessSNI points at an unrelated domain.
	cert, _, err := GenerateSelfSigned("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	// Panel self-signed for IP still self-signed — PreferredSNI may return 127.0.0.1.
	// For mismatch test we only need certMatchesHost behavior via ResolveClientSNI.
	sni := ResolveClientSNI("www.bing.com", "", "127.0.0.1", cert)
	// Self-signed panel cert: PreferredSNIFromCert returns 127.0.0.1 after our IP SAN support.
	if sni != "127.0.0.1" && sni != "localhost" {
		// GenerateSelfSigned may use CN=primary
		if sni == "www.bing.com" {
			t.Fatalf("mismatched AccessSNI must not win, got %q", sni)
		}
	}
	if sni == "www.bing.com" {
		t.Fatal("AccessSNI www.bing.com must not override cert identity")
	}
}
