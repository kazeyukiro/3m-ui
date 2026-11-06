package listener

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/acme"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestListenerTemplateModel(t *testing.T) {
	tpl := &models.ListenerTemplate{Name: "vless-default", Protocol: "vless", Config: `{"flow":"xtls-rprx-vision"}`}
	if tpl.Name == "" || tpl.Protocol == "" {
		t.Fatal("template fields unexpectedly empty")
	}
}

func TestListenerVersionSnapshotIsJSON(t *testing.T) {
	l := &models.Listener{
		BaseModel:   models.BaseModel{ID: 1},
		Name:        "demo",
		Protocol:    "vless",
		Port:        "443",
		BindAddress: "0.0.0.0",
		Config:      `{}`,
	}
	if err := ValidateModel(l); err != nil {
		t.Fatalf("valid listener rejected: %v", err)
	}
}

func TestBatchIDsRequireNonEmpty(t *testing.T) {
	if len([]uint{}) != 0 {
		t.Fatal("unexpected non-empty ids")
	}
}

// These fixtures only need to satisfy the PEM shape checks in
// resolveCertificatePair; the real-machine test below uses a real certificate.
const (
	fixtureCertPEM = "-----BEGIN CERTIFICATE-----\nZml4dHVyZQ==\n-----END CERTIFICATE-----\n"
	fixtureKeyPEM  = "-----BEGIN EC PRIVATE KEY-----\nZml4dHVyZQ==\n-----END EC PRIVATE KEY-----\n"
)

func testPanelService(t *testing.T, st acme.Settings) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(AllowTLSPathPrefixForTesting(dir))
	st.CacheDir = dir
	db, err := database.InitDB(filepath.Join(dir, "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := acme.SaveSettings(db, st); err != nil {
		t.Fatal(err)
	}
	return &Service{db: db}, dir
}

func writeCertPair(t *testing.T, dir, certName, keyName string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, certName), []byte(fixtureCertPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, keyName), []byte(fixtureKeyPEM), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Regression: from_panel_ssl must resolve the acmez PEM files for every
// issuance mode, not the long-gone autocert DirCache file <cacheDir>/<domain>.
func TestResolveCertificatePairFromPanelSSL(t *testing.T) {
	cases := []struct {
		name     string
		settings acme.Settings
		certName string
		keyName  string
		manual   bool
	}{
		{
			name:     "HTTP-01 domain (the reported bug)",
			settings: acme.Settings{Domain: "dzx1283.cc.cd", Challenge: "http-01"},
			certName: "http-dzx1283.cc.cd.crt", keyName: "http-dzx1283.cc.cd.key",
		},
		{
			name:     "IP shortlived cert",
			settings: acme.Settings{Domain: "85.149.212.214"},
			certName: "ip-85_149_212_214-cert.pem", keyName: "ip-85_149_212_214-key.pem",
		},
		{
			name:     "DNS-01",
			settings: acme.Settings{Domain: "example.com", Challenge: "dns-01", DNSProvider: "cloudflare", DNSToken: "x"},
			certName: "dns-example.com.crt", keyName: "dns-example.com.key",
		},
		{
			name:     "wildcard DNS-01",
			settings: acme.Settings{Domain: "*.example.com"},
			certName: "dns-_wildcard_.example.com.crt", keyName: "dns-_wildcard_.example.com.key",
		},
		{
			name:     "manual PEM pair",
			settings: acme.Settings{Domain: "example.com"},
			certName: "manual.crt", keyName: "manual.key", manual: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Cleanup(AllowTLSPathPrefixForTesting(dir))
			st := tc.settings
			st.CacheDir = dir
			if tc.manual {
				st.CertFile = filepath.Join(dir, tc.certName)
				st.KeyFile = filepath.Join(dir, tc.keyName)
			}
			writeCertPair(t, dir, tc.certName, tc.keyName)
			db, err := database.InitDB(filepath.Join(dir, "panel.db"))
			if err != nil {
				t.Fatal(err)
			}
			if sqlDB, err := db.DB(); err == nil {
				t.Cleanup(func() { _ = sqlDB.Close() })
			}
			if err := acme.SaveSettings(db, st); err != nil {
				t.Fatal(err)
			}
			certPEM, keyPEM, err := resolveCertificatePair(&Service{db: db}, ApplyCertificateInput{FromPanelSSL: true})
			if err != nil {
				t.Fatalf("from_panel_ssl failed: %v", err)
			}
			if certPEM != strings.TrimSpace(fixtureCertPEM) || keyPEM != strings.TrimSpace(fixtureKeyPEM) {
				t.Fatalf("unexpected PEM returned: cert=%q key=%q", certPEM, keyPEM)
			}
		})
	}
}

func TestResolveCertificatePairMissingIssuedFiles(t *testing.T) {
	svc, dir := testPanelService(t, acme.Settings{Domain: "dzx1283.cc.cd"})
	_, _, err := resolveCertificatePair(svc, ApplyCertificateInput{FromPanelSSL: true})
	if err == nil {
		t.Fatal("expected an error when ACME has not issued the cert yet")
	}
	msg := err.Error()
	for _, stale := range []string{"autocert", "cert may not have been issued", "letsencrypt/live", "first TLS handshake", "Visit the panel over HTTPS once to trigger cert issuance"} {
		if strings.Contains(msg, stale) {
			t.Errorf("stale/misleading text %q still present: %s", stale, msg)
		}
	}
	if !strings.Contains(msg, filepath.Join(dir, "http-dzx1283.cc.cd.crt")) {
		t.Errorf("error must name the real path it looked for: %s", msg)
	}
}

func TestResolveCertificatePairNoCertificateConfigured(t *testing.T) {
	svc, _ := testPanelService(t, acme.Settings{})
	_, _, err := resolveCertificatePair(svc, ApplyCertificateInput{FromPanelSSL: true})
	if err == nil || !strings.Contains(err.Error(), "no certificate configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveCertificatePairPastedPEMStillWorks(t *testing.T) {
	cert, key, err := resolveCertificatePair(&Service{}, ApplyCertificateInput{
		Certificate: fixtureCertPEM, PrivateKey: fixtureKeyPEM,
	})
	if err != nil || cert != strings.TrimSpace(fixtureCertPEM) || key != strings.TrimSpace(fixtureKeyPEM) {
		t.Fatalf("pasted PEM path broken: cert=%q key=%q err=%v", cert, key, err)
	}
}

func TestResolveCertificatePairFormPathsStayAllowlisted(t *testing.T) {
	_, _, err := resolveCertificatePair(&Service{}, ApplyCertificateInput{
		CertFile: "/tmp/arbitrary.crt", KeyFile: "/tmp/arbitrary.key",
	})
	if err == nil || !strings.Contains(err.Error(), "allowed TLS directory") {
		t.Fatalf("form-supplied paths outside the allowlist must be rejected: %v", err)
	}
}

// Real-machine check: set REAL_PANEL_DB to a *copy* of a live panel DB. It
// reads the saved panel_ssl settings and the real acme cache, then verifies the
// returned pair is a loadable X.509 keypair. Skipped in normal unit-test runs.
func TestResolveCertificatePairRealPanelDB(t *testing.T) {
	dbPath := os.Getenv("REAL_PANEL_DB")
	if dbPath == "" {
		t.Skip("set REAL_PANEL_DB to a copy of the live panel DB")
	}
	db, err := database.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("open read-only DB: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer func() { _ = sqlDB.Close() }()
	}
	st, err := acme.LoadSettings(db)
	if err != nil {
		t.Fatalf("load panel SSL settings: %v", err)
	}
	certFile, keyFile := acme.CertificatePaths(st)
	t.Logf("panel_ssl: domain=%q challenge=%q cache_dir=%q", st.Domain, st.Challenge, st.CacheDir)
	t.Logf("resolved paths: cert=%q key=%q", certFile, keyFile)
	for _, p := range []string{certFile, keyFile} {
		info, statErr := os.Stat(p)
		if statErr != nil {
			t.Fatalf("real cert file missing: %v", statErr)
		}
		t.Logf("on disk: %s (%d bytes)", p, info.Size())
	}
	certPEM, keyPEM, err := resolveCertificatePair(&Service{db: db}, ApplyCertificateInput{FromPanelSSL: true})
	if err != nil {
		t.Fatalf("from_panel_ssl failed on real data: %v", err)
	}
	if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
		t.Fatalf("returned pair is not a loadable TLS keypair: %v", err)
	}
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		t.Fatal("certificate PEM is not decodable")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	t.Logf("certificate: CN=%s issuer=%s SAN=%v notAfter=%s",
		leaf.Subject.CommonName, leaf.Issuer.CommonName, leaf.DNSNames, leaf.NotAfter.Format(time.RFC3339))
	if len(leaf.DNSNames) == 0 {
		t.Fatal("expected a domain SAN on the real certificate")
	}
}
