package certutil

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"strings"
)

// PanelSelfSignedOrg is the Organization set by panel-generated self-signed certs.
const PanelSelfSignedOrg = "3m-ui"

// IsPanelSelfSignedPEM reports whether certPEM was issued by the panel's
// self-signed generator (Subject.Organization contains "3m-ui").
func IsPanelSelfSignedPEM(certPEM string) bool {
	cert := firstCertificate(certPEM)
	if cert == nil {
		return false
	}
	for _, org := range cert.Subject.Organization {
		if org == PanelSelfSignedOrg {
			return true
		}
	}
	// Legacy panel certs: self-issued with CN=localhost only.
	if cert.Subject.CommonName == "localhost" && cert.Issuer.CommonName == "localhost" {
		return true
	}
	return false
}

// ShouldSkipCertVerify returns true when the listener config already requests
// skip-cert-verify, or when the embedded server certificate is panel self-signed.
func ShouldSkipCertVerify(cfg map[string]interface{}) bool {
	if cfg == nil {
		return false
	}
	if b, ok := cfg["skip-cert-verify"].(bool); ok && b {
		return true
	}
	cert, _ := cfg["certificate"].(string)
	return IsPanelSelfSignedPEM(cert)
}

// PreferredSNIFromCert returns the best TLS ServerName from a certificate PEM:
// first DNS SAN, else non-empty CN. Empty when no usable name (e.g. IP-only cert).
// Used when the operator has a formal cert but left Access SNI blank.
func PreferredSNIFromCert(certPEM string) string {
	cert := firstCertificate(certPEM)
	if cert == nil {
		return ""
	}
	for _, name := range cert.DNSNames {
		name = strings.TrimSpace(name)
		if name != "" && net.ParseIP(name) == nil {
			return name
		}
	}
	cn := strings.TrimSpace(cert.Subject.CommonName)
	if cn != "" && net.ParseIP(cn) == nil {
		return cn
	}
	return ""
}

// ResolveClientSNI picks SNI for subscription/share clients.
// Order: explicit config → public host (if not IP) → connect host (if not IP) →
// certificate DNS/CN (formal cert without panel SNI) → connect host as last resort.
func ResolveClientSNI(configured, publicHost, connectHost, certPEM string) string {
	for _, cand := range []string{configured, publicHost, connectHost} {
		cand = normalizeVerifyHost(cand)
		if cand == "" {
			continue
		}
		if net.ParseIP(cand) == nil {
			return cand
		}
	}
	if sni := PreferredSNIFromCert(certPEM); sni != "" {
		return sni
	}
	// IP-only connect with no cert names: still return host for completeness.
	return normalizeVerifyHost(connectHost)
}

// DecideClientSkipCertVerify chooses skip-cert-verify for client subscription/share.
//
// Priority:
//  1. Explicit skip-cert-verify in config (operator override)
//  2. Panel-generated PEM (O=3m-ui / legacy localhost) → skip
//  3. Other self-signed PEM (Issuer signs itself) → skip
//  4. Formal / CA-signed PEM → never skip (client verifies via system trust;
//     callers must emit the correct SNI via ResolveClientSNI)
//  5. No PEM + connect host is a domain name → do not skip (public CA / CDN edge)
//  6. No PEM + IP-only or empty host → skip (nothing to verify against)
//
// Previously (5)/(6) always skipped when PEM was missing, and formal certs whose
// SAN did not match a bare IP connect host were also forced to skip — so operators
// with real certificates still saw insecure=1 / skip-cert-verify in URIs.
func DecideClientSkipCertVerify(certPEM, connectHost string, explicitSkip *bool) bool {
	if explicitSkip != nil {
		return *explicitSkip
	}
	if IsPanelSelfSignedPEM(certPEM) {
		return true
	}
	cert := firstCertificate(certPEM)
	if cert == nil {
		host := normalizeVerifyHost(connectHost)
		if host != "" && net.ParseIP(host) == nil {
			return false
		}
		return true
	}
	if isSelfSignedCert(cert) {
		return true
	}
	// Non-self-signed certificate material: trust system CAs; do not force insecure.
	return false
}

func firstCertificate(certPEM string) *x509.Certificate {
	certPEM = strings.TrimSpace(certPEM)
	if certPEM == "" || !strings.Contains(certPEM, "BEGIN CERTIFICATE") {
		return nil
	}
	rest := []byte(certPEM)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		return cert
	}
}

func isSelfSignedCert(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	return cert.CheckSignatureFrom(cert) == nil
}

func normalizeVerifyHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.Trim(host, "[]")
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(strings.TrimSpace(host))
}

func certMatchesHost(cert *x509.Certificate, host string) bool {
	if cert == nil || host == "" {
		return false
	}
	if err := cert.VerifyHostname(host); err == nil {
		return true
	}
	// VerifyHostname is strict; also accept exact CN for IP-less edge cases.
	if strings.EqualFold(cert.Subject.CommonName, host) {
		return true
	}
	for _, name := range cert.DNSNames {
		if strings.EqualFold(name, host) {
			return true
		}
	}
	for _, ip := range cert.IPAddresses {
		if ip.String() == host {
			return true
		}
	}
	return false
}
