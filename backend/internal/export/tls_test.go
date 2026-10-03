package export

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func testIPCertPEM(t *testing.T, ip string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: ip},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP(ip)},
	}
	// Mark as CA-signed-looking by different issuer? Self-signed still triggers skip.
	// For formal path DecideClientSkipCertVerify treats self-signed as skip.
	// Use same issuer/subject self-signed — SkipCert will be true.
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestBuildProfileFromConfigEmitsIPSNI(t *testing.T) {
	ip := "85.149.212.214"
	pem := testIPCertPEM(t, ip)
	l := models.Listener{
		ID:        1,
		AccessSNI: "www.bing.com", // must NOT win over IP SAN
		Port:      "43829",
	}
	cfg := map[string]interface{}{"certificate": pem}
	p := BuildProfileFromConfig(l, ip, cfg)
	if p.SNI != ip {
		t.Fatalf("SNI=%q want %q", p.SNI, ip)
	}
	if p.Host != ip {
		t.Fatalf("Host=%q", p.Host)
	}
}

func TestBuildProfileEmptyHostSNIFallback(t *testing.T) {
	l := models.Listener{Port: "443", PublicHost: "example.com"}
	p := BuildProfileFromConfig(l, "", nil)
	if p.SNI != "example.com" {
		t.Fatalf("SNI=%q", p.SNI)
	}
}
