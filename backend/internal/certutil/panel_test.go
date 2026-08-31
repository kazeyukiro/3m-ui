package certutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestIsPanelSelfSignedPEM(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "localhost", Organization: []string{"3m-ui"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if !IsPanelSelfSignedPEM(string(pemBytes)) {
		t.Fatal("expected panel self-signed detection")
	}
	if IsPanelSelfSignedPEM("") {
		t.Fatal("empty should be false")
	}
	if !ShouldSkipCertVerify(map[string]interface{}{"certificate": string(pemBytes)}) {
		t.Fatal("should skip verify for panel cert")
	}
	if !ShouldSkipCertVerify(map[string]interface{}{"skip-cert-verify": true}) {
		t.Fatal("explicit skip")
	}
}
