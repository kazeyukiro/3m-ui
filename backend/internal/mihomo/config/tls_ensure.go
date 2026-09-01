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
//
// For anytls/trojan/hy2/etc., panel-generated self-signed PEMs are injected when
// missing. Incomplete alternate wrappers (empty jls-config) must NOT suppress
// cert generation — sanitizeIncompleteTLSWrappers runs first.
//
// When a cert pair is injected, allow-insecure is cleared: that flag means
// "TLS disabled for nginx/caddy front", which is incompatible with a normal
// AnyTLS client that dials TLS to the listener.
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
		// Normal TLS mode: allow-insecure must not stay true.
		delete(cfg, "allow-insecure")
		return nil
	}
	// Partial pair (only cert or only key) cannot work — drop and regenerate.
	if strings.TrimSpace(cert) != "" || strings.TrimSpace(key) != "" {
		delete(cfg, "certificate")
		delete(cfg, "private-key")
		delete(cfg, "private_key")
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
	// Self-signed TLS is active — disable the "no TLS" ingress mode.
	delete(cfg, "allow-insecure")
	return nil
}

func listenerProtocolNeedsCert(proto string, cfg map[string]interface{}) bool {
	if _, ok := cfg["reality-config"]; ok {
		return false
	}
	// Official: allow-insecure=true means TLS off at the listener (nginx/caddy front).
	// Do NOT invent a self-signed cert or clear this flag — that silently breaks nodes.
	if b, ok := cfg["allow-insecure"].(bool); ok && b {
		switch proto {
		case "anytls", "trojan", "vmess", "vless":
			return false
		}
	}
	// Only a *complete* alternate wrapper replaces certificate/private-key.
	if completeShadowTLS(cfg) || completeResTLS(cfg) || completeJLS(cfg) {
		return false
	}
	if _, ok := cfg["tlsmirror-config"]; ok && cfg["tlsmirror-config"] != nil {
		return false
	}
	switch proto {
	case "hysteria2", "anytls", "tuic", "tuic-v4", "tuic-v5", "trusttunnel", "trojan":
		return true
	default:
		return false
	}
}

func completeJLS(cfg map[string]interface{}) bool {
	jls, ok := cfg["jls-config"].(map[string]interface{})
	if !ok || jls == nil {
		return false
	}
	dest, _ := jls["dest"].(string)
	if strings.TrimSpace(dest) == "" {
		return false
	}
	return hasJLSUsers(jls)
}

func completeResTLS(cfg map[string]interface{}) bool {
	rt, ok := cfg["res-tls"].(map[string]interface{})
	if !ok || rt == nil {
		return false
	}
	if en, ok := rt["enable"].(bool); ok && !en {
		return false
	}
	dest, _ := rt["dest"].(string)
	return strings.TrimSpace(dest) != ""
}

func completeShadowTLS(cfg map[string]interface{}) bool {
	st, ok := cfg["shadow-tls"].(map[string]interface{})
	if !ok || st == nil {
		return false
	}
	if en, ok := st["enable"].(bool); ok && !en {
		return false
	}
	// v2 password or v3 users or handshake.dest
	if p, _ := st["password"].(string); strings.TrimSpace(p) != "" {
		return true
	}
	if _, ok := st["users"]; ok {
		return true
	}
	if hs, ok := st["handshake"].(map[string]interface{}); ok {
		if d, _ := hs["dest"].(string); strings.TrimSpace(d) != "" {
			return true
		}
	}
	return false
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
