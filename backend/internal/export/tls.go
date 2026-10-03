// Package export is the single source of truth for client-facing TLS identity
// (SNI, skip-cert, certificate PEM) used by URI share, Clash/Mihomo YAML and
// subscription pipelines.
package export

import (
	"net"
	"os"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/certstore"
	"github.com/kazeyukiro/3m-ui/backend/internal/certutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
)

// Profile is the resolved client TLS identity for one listener + connect host.
type Profile struct {
	Host        string // normalized connect host
	Port        string // single port for URI export
	CertPEM     string // materialised PEM (never a bare path when load succeeds)
	SNI         string // always set when Host is non-empty
	SkipCert    bool
	Fingerprint string
	ALPN        string // raw AccessALPN or config string
}

// MaterializeCertPEM expands inline PEM, filesystem paths, or certstore material.
func MaterializeCertPEM(raw string, listenerID uint) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "BEGIN CERTIFICATE") {
		return raw
	}
	if raw != "" && (strings.HasPrefix(raw, "/") || strings.HasSuffix(raw, ".pem") || strings.HasSuffix(raw, ".crt") || strings.HasSuffix(raw, ".cer")) {
		if b, err := os.ReadFile(raw); err == nil {
			s := string(b)
			if strings.Contains(s, "BEGIN CERTIFICATE") {
				return s
			}
		}
	}
	if listenerID != 0 {
		if c, _, ok := certstore.Load(listenerID); ok {
			return c
		}
	}
	return ""
}

// BuildProfile resolves host/port/cert/SNI/skip for a listener. configuredSNI is
// optional (listener AccessSNI / config sni already considered via AccessSNI).
func BuildProfile(l models.Listener, connectHost, configuredSNI string, explicitSkip *bool) Profile {
	host := netutil.NormalizeHost(connectHost)
	if host == "" {
		host = netutil.NormalizeHost(l.PublicHost)
	}
	port := strings.TrimSpace(l.PublicPort)
	if port == "" {
		port = strings.TrimSpace(l.Port)
	}

	// Certificate: prefer config field then certstore.
	certPEM := ""
	if strings.TrimSpace(l.Config) != "" {
		// lightweight extract without full JSON decode dependency for path/PEM only
		certPEM = extractCertField(l.Config)
	}
	certPEM = MaterializeCertPEM(certPEM, l.ID)

	cfgSNI := strings.TrimSpace(configuredSNI)
	if cfgSNI == "" {
		cfgSNI = strings.TrimSpace(l.AccessSNI)
	}
	sni := certutil.ResolveClientSNI(cfgSNI, l.PublicHost, host, certPEM)
	if strings.TrimSpace(sni) == "" {
		sni = strings.Trim(host, "[]")
	}

	verify := sni
	if verify == "" {
		verify = host
	}
	skip := certutil.DecideClientSkipCertVerify(certPEM, verify, explicitSkip)

	fp := strings.TrimSpace(l.ClientFingerprint)
	alpn := strings.TrimSpace(l.AccessALPN)

	return Profile{
		Host:        host,
		Port:        port,
		CertPEM:     certPEM,
		SNI:         sni,
		SkipCert:    skip,
		Fingerprint: fp,
		ALPN:        alpn,
	}
}

// ApplyToConfig writes profile fields into a listener config map used by URI builders.
func ApplyToConfig(cfg map[string]interface{}, l models.Listener, p Profile) {
	if cfg == nil {
		return
	}
	cfg["_listener-tls"] = l.TLS
	cfg["_listener-udp"] = l.UDP
	if p.CertPEM != "" {
		cfg["certificate"] = p.CertPEM
	}
	if p.SNI != "" {
		cfg["sni"] = p.SNI
		cfg["servername"] = p.SNI
	}
	if p.Fingerprint != "" {
		cfg["client-fingerprint"] = p.Fingerprint
		cfg["fingerprint"] = p.Fingerprint
	}
	if p.ALPN != "" {
		cfg["alpn"] = p.ALPN
	}
	if p.SkipCert {
		cfg["skip-cert-verify"] = true
	} else {
		delete(cfg, "skip-cert-verify")
	}

	// WS Host header for CDN edges when missing.
	hasWS := false
	if wsPath, ok := cfg["ws-path"].(string); ok && strings.TrimSpace(wsPath) != "" {
		hasWS = true
	}
	hint := strings.Trim(p.SNI, "[]")
	if hint == "" {
		hint = strings.Trim(p.Host, "[]")
	}
	domainHint := hint != "" && net.ParseIP(hint) == nil
	cdnPort := p.Port == "443" || p.Port == "8443" || p.Port == "2053" || p.Port == "2083" || p.Port == "2087" || p.Port == "2096"
	if hasWS && domainHint && cdnPort {
		cfg["_listener-tls"] = true
	}
	if p.CertPEM != "" {
		cfg["_listener-tls"] = true
	}
	if _, ok := cfg["reality-config"]; ok {
		cfg["_listener-tls"] = true
	}
	if hasWS {
		headers, _ := cfg["ws-headers"].(map[string]interface{})
		if headers == nil {
			headers = map[string]interface{}{}
		}
		if h, _ := headers["Host"].(string); strings.TrimSpace(h) == "" && domainHint {
			headers["Host"] = hint
			cfg["ws-headers"] = headers
		}
	}
}

// ApplyToProxyMap writes sni/servername/skip-cert-verify onto a Clash/Mihomo proxy map.
// Reality proxies keep camouflage servername and always skip verify.
func ApplyToProxyMap(p map[string]interface{}, profile Profile, forceTLS bool) {
	if p == nil {
		return
	}
	if _, isReality := p["reality-opts"]; isReality {
		p["skip-cert-verify"] = true
		return
	}
	typ, _ := p["type"].(string)
	typ = strings.ToLower(strings.TrimSpace(typ))
	tlsNative := typ == "hysteria2" || typ == "tuic" || typ == "anytls" || typ == "shadowquic" || typ == "trusttunnel"
	tlsOn := forceTLS || tlsNative
	switch v := p["tls"].(type) {
	case bool:
		tlsOn = tlsOn || v
	case string:
		tlsOn = tlsOn || strings.EqualFold(v, "true")
	}
	if !tlsOn {
		return
	}
	if profile.SNI != "" {
		p["sni"] = profile.SNI
		p["servername"] = profile.SNI
	}
	if profile.SkipCert {
		p["skip-cert-verify"] = true
	} else {
		delete(p, "skip-cert-verify")
	}
	if profile.Fingerprint != "" {
		if p["client-fingerprint"] == nil {
			p["client-fingerprint"] = profile.Fingerprint
		}
	}
}

func extractCertField(configJSON string) string {
	// Avoid full parse dependency: look for "certificate":"..."
	const key = `"certificate"`
	i := strings.Index(configJSON, key)
	if i < 0 {
		return ""
	}
	rest := configJSON[i+len(key):]
	rest = strings.TrimLeft(rest, " \t\r\n:")
	if len(rest) == 0 {
		return ""
	}
	if rest[0] == '"' {
		rest = rest[1:]
		var b strings.Builder
		for j := 0; j < len(rest); j++ {
			c := rest[j]
			if c == '\\' && j+1 < len(rest) {
				b.WriteByte(rest[j+1])
				j++
				continue
			}
			if c == '"' {
				break
			}
			b.WriteByte(c)
		}
		return b.String()
	}
	return ""
}

// BuildProfileFromConfig is like BuildProfile but uses an already-decoded config map
// for certificate / sni / skip-cert-verify fields.
func BuildProfileFromConfig(l models.Listener, connectHost string, cfg map[string]interface{}) Profile {
	var explicit *bool
	configured := strings.TrimSpace(l.AccessSNI)
	certRaw := ""
	if cfg != nil {
		if b, ok := cfg["skip-cert-verify"].(bool); ok {
			explicit = &b
		}
		if v, ok := cfg["certificate"].(string); ok {
			certRaw = v
		}
		for _, key := range []string{"sni", "servername"} {
			if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
				if configured == "" {
					configured = strings.TrimSpace(v)
				}
				break
			}
		}
	}
	host := netutil.NormalizeHost(connectHost)
	if host == "" {
		host = netutil.NormalizeHost(l.PublicHost)
	}
	port := strings.TrimSpace(l.PublicPort)
	if port == "" {
		port = strings.TrimSpace(l.Port)
	}
	certPEM := MaterializeCertPEM(certRaw, l.ID)
	sni := certutil.ResolveClientSNI(configured, l.PublicHost, host, certPEM)
	if strings.TrimSpace(sni) == "" {
		sni = strings.Trim(host, "[]")
	}
	verify := sni
	if verify == "" {
		verify = host
	}
	skip := certutil.DecideClientSkipCertVerify(certPEM, verify, explicit)
	fp := strings.TrimSpace(l.ClientFingerprint)
	if fp == "" && cfg != nil {
		for _, key := range []string{"client-fingerprint", "fingerprint"} {
			if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
				fp = strings.TrimSpace(v)
				break
			}
		}
	}
	alpn := strings.TrimSpace(l.AccessALPN)
	if alpn == "" && cfg != nil {
		if v, ok := cfg["alpn"].(string); ok {
			alpn = strings.TrimSpace(v)
		}
	}
	return Profile{
		Host:        host,
		Port:        port,
		CertPEM:     certPEM,
		SNI:         sni,
		SkipCert:    skip,
		Fingerprint: fp,
		ALPN:        alpn,
	}
}
