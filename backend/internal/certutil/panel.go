package certutil

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
)

// PanelSelfSignedOrg is the Organization set by panel-generated self-signed certs.
const PanelSelfSignedOrg = "3m-ui"

// IsPanelSelfSignedPEM reports whether certPEM was issued by the panel's
// self-signed generator (Subject.Organization contains "3m-ui").
func IsPanelSelfSignedPEM(certPEM string) bool {
	certPEM = strings.TrimSpace(certPEM)
	if certPEM == "" || !strings.Contains(certPEM, "BEGIN CERTIFICATE") {
		return false
	}
	rest := []byte(certPEM)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		for _, org := range cert.Subject.Organization {
			if org == PanelSelfSignedOrg {
				return true
			}
		}
		// Also accept legacy panel certs that only set CN=localhost without org
		// if Issuer is self and CN is localhost (generated before org tagging).
		if cert.Subject.CommonName == "localhost" && cert.Issuer.CommonName == "localhost" {
			return true
		}
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
	// allow-insecure on server is a different mode (TLS offload); still skip client verify
	// is wrong for that mode — only check certificate.
	cert, _ := cfg["certificate"].(string)
	return IsPanelSelfSignedPEM(cert)
}
