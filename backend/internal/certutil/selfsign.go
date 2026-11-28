package certutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

// SelfSignedValidity is how long panel-generated server certs remain valid.
const SelfSignedValidity = 10 * 365 * 24 * time.Hour

// GenerateSelfSigned creates a panel-tagged ECDSA P-256 self-signed certificate.
// primary is used as CN (default localhost). extraHosts may include DNS names
// and IP addresses (e.g. SNI, PublicHost, listen IP).
func GenerateSelfSigned(primary string, extraHosts ...string) (certPEM, keyPEM string, err error) {
	primary = strings.TrimSpace(primary)
	if primary == "" {
		primary = "localhost"
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("serial: %w", err)
	}

	dns := map[string]struct{}{}
	ips := map[string]struct{}{}
	addHost := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" {
			return
		}
		// strip brackets from IPv6 literals
		h = strings.Trim(h, "[]")
		if ip := net.ParseIP(h); ip != nil {
			ips[ip.String()] = struct{}{}
			return
		}
		// host:port → host
		if host, _, err := net.SplitHostPort(h); err == nil {
			h = host
		}
		h = strings.Trim(h, "[]")
		if ip := net.ParseIP(h); ip != nil {
			ips[ip.String()] = struct{}{}
			return
		}
		dns[strings.ToLower(h)] = struct{}{}
	}
	addHost(primary)
	addHost("localhost")
	for _, h := range extraHosts {
		addHost(h)
	}

	dnsNames := make([]string, 0, len(dns))
	for d := range dns {
		dnsNames = append(dnsNames, d)
	}
	ipList := make([]net.IP, 0, len(ips))
	for s := range ips {
		if ip := net.ParseIP(s); ip != nil {
			ipList = append(ipList, ip)
		}
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   primary,
			Organization: []string{PanelSelfSignedOrg},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(SelfSignedValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipList,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", fmt.Errorf("create cert: %w", err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", fmt.Errorf("marshal key: %w", err)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	return certPEM, keyPEM, nil
}

// HostHintsFromConfig extracts SNI / server-names for SAN inclusion.
func HostHintsFromConfig(cfg map[string]interface{}) []string {
	if cfg == nil {
		return nil
	}
	var out []string
	if s, _ := cfg["sni"].(string); strings.TrimSpace(s) != "" {
		out = append(out, strings.TrimSpace(s))
	}
	if s, _ := cfg["servername"].(string); strings.TrimSpace(s) != "" {
		out = append(out, strings.TrimSpace(s))
	}
	switch v := cfg["server-names"].(type) {
	case []interface{}:
		for _, x := range v {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case []string:
		for _, s := range v {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	}
	return out
}
