package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// ensureListenerTLSMaterial fills empty certificate/private-key for listeners
// that Mihomo requires to present server TLS credentials.
func ensureListenerTLSMaterial(protocol string, cfg map[string]interface{}) error {
	if cfg == nil {
		return nil
	}
	proto := strings.ToLower(strings.TrimSpace(protocol))
	if s, ok := cfg["certificate"].(string); ok && strings.TrimSpace(s) == "" {
		delete(cfg, "certificate")
	}
	if s, ok := cfg["private-key"].(string); ok && strings.TrimSpace(s) == "" {
		delete(cfg, "private-key")
	}
	if s, ok := cfg["private_key"].(string); ok && strings.TrimSpace(s) == "" {
		delete(cfg, "private_key")
	}
	cert, _ := cfg["certificate"].(string)
	key, _ := cfg["private-key"].(string)
	if key == "" {
		key, _ = cfg["private_key"].(string)
	}
	if strings.TrimSpace(cert) != "" && strings.TrimSpace(key) != "" {
		return nil
	}
	if strings.TrimSpace(cert) != "" || strings.TrimSpace(key) != "" {
		return nil
	}
	if !listenerProtocolNeedsCert(proto, cfg) {
		return nil
	}
	host := "localhost"
	if sni, _ := cfg["sni"].(string); strings.TrimSpace(sni) != "" {
		host = strings.TrimSpace(sni)
	}
	certPEM, keyPEM, err := selfSignedPEMs(host)
	if err != nil {
		return fmt.Errorf("listener %s: %w", proto, err)
	}
	cfg["certificate"] = certPEM
	cfg["private-key"] = keyPEM
	return nil
}

func listenerProtocolNeedsCert(proto string, cfg map[string]interface{}) bool {
	if _, ok := cfg["reality-config"]; ok {
		return false
	}
	for _, k := range []string{"shadow-tls", "res-tls", "jls-config", "tlsmirror-config"} {
		if v, ok := cfg[k]; ok && v != nil {
			return false
		}
	}
	switch proto {
	case "hysteria2", "anytls", "mieru", "tuic", "trusttunnel", "trojan":
		return true
	default:
		return false
	}
}

func selfSignedPEMs(host string) (certPEM, keyPEM string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}
	if host == "" {
		host = "localhost"
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host, Organization: []string{"3m-ui"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	return certPEM, keyPEM, nil
}
