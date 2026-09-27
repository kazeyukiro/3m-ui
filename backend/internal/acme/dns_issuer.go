package acme

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mholt/acmez/v3"
	zacme "github.com/mholt/acmez/v3/acme"
)

const (
	dnsRenewBefore = 30 * 24 * time.Hour
	dnsRenewTick   = 12 * time.Hour
	cfAPIBase      = "https://api.cloudflare.com/client/v4"
)

// IsWildcardDomain reports whether domain is a DNS wildcard (*.example.com).
func IsWildcardDomain(domain string) bool {
	return strings.HasPrefix(strings.TrimSpace(domain), "*.")
}

// NeedsDNS01 is true when the domain or challenge mode requires DNS-01.
func NeedsDNS01(s Settings) bool {
	ch := strings.ToLower(strings.TrimSpace(s.Challenge))
	if ch == "dns-01" || ch == "dns01" || ch == "dns" {
		return true
	}
	return IsWildcardDomain(s.Domain)
}

// certNames returns SANs for the certificate. Wildcards also include the apex
// so https://example.com and https://foo.example.com both work.
func certNames(domain string) []string {
	d := strings.TrimSpace(domain)
	d = strings.TrimSuffix(d, ".")
	if IsWildcardDomain(d) {
		apex := strings.TrimPrefix(d, "*.")
		if apex == "" {
			return []string{d}
		}
		return []string{d, apex}
	}
	if d == "" {
		return nil
	}
	return []string{d}
}

// dns01TXTValue is the standard ACME DNS-01 TXT content.
func dns01TXTValue(keyAuth string) string {
	sum := sha256.Sum256([]byte(keyAuth))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// dnsIssuer obtains and renews domain certificates via Let's Encrypt DNS-01.
type dnsIssuer struct {
	mu       sync.Mutex
	domain   string // as configured (may be *.example.com)
	names    []string
	email    string
	cacheDir string
	provider string
	token    string
	zoneHint string
	cert     *tls.Certificate
	stop     chan struct{}
}

func newDNSIssuer(s Settings) (*dnsIssuer, error) {
	domain := strings.TrimSpace(s.Domain)
	if domain == "" {
		return nil, fmt.Errorf("panel SSL: domain is required for DNS-01")
	}
	if IsIPHost(domain) {
		return nil, fmt.Errorf("panel SSL: DNS-01 is for domain names, not IP addresses")
	}
	provider := strings.ToLower(strings.TrimSpace(s.DNSProvider))
	if provider == "" {
		provider = "cloudflare"
	}
	token := strings.TrimSpace(s.DNSToken)
	if token == "" {
		return nil, fmt.Errorf("panel SSL: DNS API token is required for DNS-01 / wildcard certificates")
	}
	if provider != "cloudflare" {
		return nil, fmt.Errorf("panel SSL: unsupported DNS provider %q (supported: cloudflare)", provider)
	}
	if err := os.MkdirAll(s.CacheDir, 0o700); err != nil {
		return nil, err
	}
	iss := &dnsIssuer{
		domain:   domain,
		names:    certNames(domain),
		email:    strings.TrimSpace(s.Email),
		cacheDir: s.CacheDir,
		provider: provider,
		token:    token,
		zoneHint: strings.TrimSpace(s.DNSZone),
		stop:     make(chan struct{}),
	}
	if cert, err := iss.loadFromDisk(); err == nil && cert != nil {
		iss.cert = cert
		log.Printf("panel SSL: loaded DNS-01 cert for %s (expires %s)", domain, leafNotAfter(cert).Format(time.RFC3339))
	} else {
		if err := iss.obtain(); err != nil {
			return nil, fmt.Errorf("panel SSL: obtain DNS-01 cert for %s: %w", domain, err)
		}
	}
	go iss.renewLoop()
	return iss, nil
}

func (iss *dnsIssuer) certPath() string {
	// File name cannot contain * on all filesystems — use safe key.
	safe := strings.ReplaceAll(iss.domain, "*", "_wildcard_")
	return filepath.Join(iss.cacheDir, "dns-"+safe+".crt")
}

func (iss *dnsIssuer) keyPath() string {
	safe := strings.ReplaceAll(iss.domain, "*", "_wildcard_")
	return filepath.Join(iss.cacheDir, "dns-"+safe+".key")
}

func (iss *dnsIssuer) accountKeyPath() string {
	return filepath.Join(iss.cacheDir, "dns-account.ec.key")
}

func (iss *dnsIssuer) loadFromDisk() (*tls.Certificate, error) {
	certPEM, err := os.ReadFile(iss.certPath())
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(iss.keyPath())
	if err != nil {
		return nil, err
	}
	c, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (iss *dnsIssuer) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	iss.mu.Lock()
	defer iss.mu.Unlock()
	if iss.cert == nil {
		return nil, fmt.Errorf("panel SSL: no DNS-01 certificate loaded")
	}
	return iss.cert, nil
}

func (iss *dnsIssuer) Close() {
	select {
	case <-iss.stop:
	default:
		close(iss.stop)
	}
}

func (iss *dnsIssuer) obtain() error {
	iss.mu.Lock()
	defer iss.mu.Unlock()
	return iss.obtainLocked()
}

func (iss *dnsIssuer) obtainLocked() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	accountKey, err := iss.loadOrCreateAccountKey()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	low := &zacme.Client{
		Directory: leDirectory,
		Logger:    logger,
	}
	solver := &dns01Solver{iss: iss}
	client := acmez.Client{
		Client: low,
		ChallengeSolvers: map[string]acmez.Solver{
			zacme.ChallengeTypeDNS01: solver,
		},
	}

	var contact []string
	if iss.email != "" {
		contact = []string{"mailto:" + iss.email}
	}
	account := zacme.Account{
		Contact:              contact,
		TermsOfServiceAgreed: true,
		PrivateKey:           accountKey,
	}
	account, err = low.NewAccount(ctx, account)
	if err != nil {
		account, err = low.GetAccount(ctx, account)
		if err != nil {
			return fmt.Errorf("ACME account: %w", err)
		}
	}

	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	csr, err := acmez.NewCSR(certKey, iss.names)
	if err != nil {
		return fmt.Errorf("CSR: %w", err)
	}
	params, err := acmez.OrderParametersFromCSR(account, csr)
	if err != nil {
		return fmt.Errorf("order params: %w", err)
	}

	certs, err := client.ObtainCertificate(ctx, params)
	if err != nil {
		return err
	}
	if len(certs) == 0 || len(certs[0].ChainPEM) == 0 {
		return fmt.Errorf("ACME returned empty certificate chain")
	}

	keyDER, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	certPEM := certs[0].ChainPEM
	if err := os.WriteFile(iss.certPath(), certPEM, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(iss.keyPath(), keyPEM, 0o600); err != nil {
		return err
	}
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	iss.cert = &tlsCert
	log.Printf("panel SSL: obtained Let's Encrypt DNS-01 cert for %v (expires %s)",
		iss.names, leafNotAfter(&tlsCert).Format(time.RFC3339))
	return nil
}

func (iss *dnsIssuer) loadOrCreateAccountKey() (crypto.Signer, error) {
	path := iss.accountKeyPath()
	if b, err := os.ReadFile(path); err == nil {
		block, _ := pem.Decode(b)
		if block != nil {
			if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
				return key, nil
			}
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (iss *dnsIssuer) renewLoop() {
	t := time.NewTicker(dnsRenewTick)
	defer t.Stop()
	for {
		select {
		case <-iss.stop:
			return
		case <-t.C:
			iss.maybeRenew()
		}
	}
}

func (iss *dnsIssuer) maybeRenew() {
	iss.mu.Lock()
	cert := iss.cert
	iss.mu.Unlock()
	if cert == nil {
		_ = iss.obtain()
		return
	}
	if time.Until(leafNotAfter(cert)) > dnsRenewBefore {
		return
	}
	log.Printf("panel SSL: renewing DNS-01 cert for %s", iss.domain)
	if err := iss.obtain(); err != nil {
		log.Printf("panel SSL: DNS-01 renew failed: %v", err)
	}
}

// dns01Solver presents DNS-01 challenges via the configured provider.
type dns01Solver struct {
	iss *dnsIssuer
	// record IDs for cleanup: name+content -> id
	ids map[string]string
}

func (s *dns01Solver) Present(ctx context.Context, chal zacme.Challenge) error {
	if chal.Type != zacme.ChallengeTypeDNS01 {
		return fmt.Errorf("unsupported challenge type %q", chal.Type)
	}
	ident := chal.Identifier.Value
	// Wildcard orders still use the base domain as the DNS identifier.
	ident = strings.TrimPrefix(ident, "*.")
	name := "_acme-challenge." + ident
	value := dns01TXTValue(chal.KeyAuthorization)
	log.Printf("panel SSL: DNS-01 present %s TXT=%s", name, value)

	id, err := s.iss.cfPresentTXT(ctx, name, value)
	if err != nil {
		return err
	}
	if s.ids == nil {
		s.ids = map[string]string{}
	}
	s.ids[name+"|"+value] = id

	// Propagation wait — Cloudflare is usually fast; still give resolvers time.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(25 * time.Second):
	}
	return nil
}

func (s *dns01Solver) CleanUp(ctx context.Context, chal zacme.Challenge) error {
	if chal.Type != zacme.ChallengeTypeDNS01 {
		return nil
	}
	ident := strings.TrimPrefix(chal.Identifier.Value, "*.")
	name := "_acme-challenge." + ident
	value := dns01TXTValue(chal.KeyAuthorization)
	key := name + "|" + value
	id := ""
	if s.ids != nil {
		id = s.ids[key]
		delete(s.ids, key)
	}
	if err := s.iss.cfCleanupTXT(ctx, name, value, id); err != nil {
		log.Printf("panel SSL: DNS-01 cleanup %s: %v", name, err)
	}
	return nil
}

// --- Cloudflare ---

func (iss *dnsIssuer) cfHeaders() http.Header {
	h := make(http.Header)
	h.Set("Authorization", "Bearer "+iss.token)
	h.Set("Content-Type", "application/json")
	return h
}

func (iss *dnsIssuer) cfDo(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, cfAPIBase+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header = iss.cfHeaders()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("cloudflare: decode: %w body=%s", err, truncate(string(raw), 200))
	}
	if !envelope.Success {
		msg := "unknown error"
		if len(envelope.Errors) > 0 {
			msg = envelope.Errors[0].Message
		}
		return nil, fmt.Errorf("cloudflare: %s", msg)
	}
	return envelope.Result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (iss *dnsIssuer) cfZoneID(ctx context.Context, host string) (string, error) {
	host = strings.TrimPrefix(host, "*.")
	host = strings.TrimPrefix(host, "_acme-challenge.")
	if iss.zoneHint != "" {
		res, err := iss.cfDo(ctx, http.MethodGet, "/zones?name="+url.QueryEscape(iss.zoneHint)+"&status=active", nil)
		if err != nil {
			return "", err
		}
		var zones []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(res, &zones); err != nil {
			return "", err
		}
		if len(zones) == 0 {
			return "", fmt.Errorf("cloudflare: zone %q not found", iss.zoneHint)
		}
		return zones[0].ID, nil
	}
	// Walk labels until a zone matches.
	parts := strings.Split(host, ".")
	for i := 0; i < len(parts)-1; i++ {
		candidate := strings.Join(parts[i:], ".")
		res, err := iss.cfDo(ctx, http.MethodGet, "/zones?name="+url.QueryEscape(candidate)+"&status=active", nil)
		if err != nil {
			continue
		}
		var zones []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(res, &zones); err != nil || len(zones) == 0 {
			continue
		}
		return zones[0].ID, nil
	}
	return "", fmt.Errorf("cloudflare: could not resolve zone for %q (set dns_zone or check token permissions)", host)
}

func (iss *dnsIssuer) cfPresentTXT(ctx context.Context, name, value string) (string, error) {
	zoneID, err := iss.cfZoneID(ctx, name)
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"type":    "TXT",
		"name":    name,
		"content": value,
		"ttl":     120,
	}
	res, err := iss.cfDo(ctx, http.MethodPost, "/zones/"+zoneID+"/dns_records", body)
	if err != nil {
		// If record exists, try to find and update.
		return iss.cfUpsertTXT(ctx, zoneID, name, value)
	}
	var rec struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(res, &rec); err != nil {
		return "", err
	}
	return rec.ID, nil
}

func (iss *dnsIssuer) cfUpsertTXT(ctx context.Context, zoneID, name, value string) (string, error) {
	res, err := iss.cfDo(ctx, http.MethodGet, "/zones/"+zoneID+"/dns_records?type=TXT&name="+url.QueryEscape(name), nil)
	if err != nil {
		return "", err
	}
	var recs []struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(res, &recs); err != nil {
		return "", err
	}
	for _, r := range recs {
		if r.Content == value {
			return r.ID, nil
		}
	}
	// Create another TXT (ACME may need multiple).
	body := map[string]any{
		"type":    "TXT",
		"name":    name,
		"content": value,
		"ttl":     120,
	}
	res, err = iss.cfDo(ctx, http.MethodPost, "/zones/"+zoneID+"/dns_records", body)
	if err != nil {
		return "", err
	}
	var rec struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(res, &rec)
	return rec.ID, nil
}

func (iss *dnsIssuer) cfCleanupTXT(ctx context.Context, name, value, id string) error {
	zoneID, err := iss.cfZoneID(ctx, name)
	if err != nil {
		return err
	}
	if id != "" {
		_, err := iss.cfDo(ctx, http.MethodDelete, "/zones/"+zoneID+"/dns_records/"+id, nil)
		return err
	}
	res, err := iss.cfDo(ctx, http.MethodGet, "/zones/"+zoneID+"/dns_records?type=TXT&name="+url.QueryEscape(name), nil)
	if err != nil {
		return err
	}
	var recs []struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(res, &recs); err != nil {
		return err
	}
	for _, r := range recs {
		if r.Content == value || value == "" {
			_, _ = iss.cfDo(ctx, http.MethodDelete, "/zones/"+zoneID+"/dns_records/"+r.ID, nil)
		}
	}
	return nil
}
