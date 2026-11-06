package acme

import (
	"path/filepath"
	"testing"
)

func TestCertificatePathsModes(t *testing.T) {
	cache := "/var/lib/3m-ui/acme"
	cases := []struct {
		name     string
		s        Settings
		wantCert string
		wantKey  string
	}{
		{
			name:     "manual PEM wins even with a domain set",
			s:        Settings{CacheDir: cache, Domain: "example.com", CertFile: "/etc/ssl/cert.pem", KeyFile: "/etc/ssl/key.pem"},
			wantCert: "/etc/ssl/cert.pem",
			wantKey:  "/etc/ssl/key.pem",
		},
		{
			name:     "HTTP-01 domain (lower-cased)",
			s:        Settings{CacheDir: cache, Domain: "DZX1283.CC.CD"},
			wantCert: filepath.Join(cache, "http-dzx1283.cc.cd.crt"),
			wantKey:  filepath.Join(cache, "http-dzx1283.cc.cd.key"),
		},
		{
			name:     "IP shortlived cert (dots sanitized)",
			s:        Settings{CacheDir: cache, Domain: "85.149.212.214"},
			wantCert: filepath.Join(cache, "ip-85_149_212_214-cert.pem"),
			wantKey:  filepath.Join(cache, "ip-85_149_212_214-key.pem"),
		},
		{
			name:     "IPv6 shortlived cert",
			s:        Settings{CacheDir: cache, Domain: "2a14:7dc0:202:9db::532"},
			wantCert: filepath.Join(cache, "ip-2a14_7dc0_202_9db__532-cert.pem"),
			wantKey:  filepath.Join(cache, "ip-2a14_7dc0_202_9db__532-key.pem"),
		},
		{
			name:     "DNS-01 explicit challenge",
			s:        Settings{CacheDir: cache, Domain: "example.com", Challenge: "dns-01", DNSProvider: "cloudflare", DNSToken: "x"},
			wantCert: filepath.Join(cache, "dns-example.com.crt"),
			wantKey:  filepath.Join(cache, "dns-example.com.key"),
		},
		{
			name:     "wildcard implies DNS-01",
			s:        Settings{CacheDir: cache, Domain: "*.example.com"},
			wantCert: filepath.Join(cache, "dns-_wildcard_.example.com.crt"),
			wantKey:  filepath.Join(cache, "dns-_wildcard_.example.com.key"),
		},
		{
			name: "no certificate configured",
			s:    Settings{CacheDir: cache},
		},
		{
			name: "no cache dir",
			s:    Settings{Domain: "example.com"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCert, gotKey := CertificatePaths(tc.s)
			if gotCert != tc.wantCert || gotKey != tc.wantKey {
				t.Fatalf("CertificatePaths() = (%q, %q), want (%q, %q)", gotCert, gotKey, tc.wantCert, tc.wantKey)
			}
			// The status API's cert_path must resolve to the very same file.
			if tc.wantCert != "" && domainCertPath(tc.s) != tc.wantCert {
				t.Fatalf("domainCertPath() = %q, want %q", domainCertPath(tc.s), tc.wantCert)
			}
		})
	}
}

// Issuers and the status/consumer paths must never drift: if someone changes a
// file name in one place, this test fails.
func TestIssuersWriteToCertificatePaths(t *testing.T) {
	cache := t.TempDir()
	ip := &ipIssuer{cacheDir: cache, ip: normalizeIPHost("85.149.212.214")}
	dns := &dnsIssuer{cacheDir: cache, domain: "*.example.com"}
	httpIss := &domainHTTPIssuer{cacheDir: cache, domain: "dzx1283.cc.cd"}
	cases := []struct {
		name     string
		gotCert  string
		gotKey   string
		settings Settings
	}{
		{"ip", ip.certPath(), ip.keyPath(), Settings{CacheDir: cache, Domain: "85.149.212.214"}},
		{"dns", dns.certPath(), dns.keyPath(), Settings{CacheDir: cache, Domain: "*.example.com"}},
		{"http", httpIss.certPath(), httpIss.keyPath(), Settings{CacheDir: cache, Domain: "dzx1283.cc.cd"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantCert, wantKey := CertificatePaths(tc.settings)
			if tc.gotCert != wantCert || tc.gotKey != wantKey {
				t.Fatalf("issuer paths (%q, %q) != CertificatePaths (%q, %q)",
					tc.gotCert, tc.gotKey, wantCert, wantKey)
			}
		})
	}
}
