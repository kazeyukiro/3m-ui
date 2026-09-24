package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mholt/acmez/v3"
	zacme "github.com/mholt/acmez/v3/acme"
)

const (
	ipCertProfile = "shortlived"
	ipRenewBefore = 48 * time.Hour
	ipRenewTick   = 12 * time.Hour
	leDirectory   = "https://acme-v02.api.letsencrypt.org/directory"
)

// IsIPHost reports whether host is a bare IPv4/IPv6 address (optional brackets).
func IsIPHost(host string) bool {
	h := strings.TrimSpace(host)
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")
	return net.ParseIP(h) != nil
}

func normalizeIPHost(host string) string {
	h := strings.TrimSpace(host)
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")
	ip := net.ParseIP(h)
	if ip == nil {
		return h
	}
	return ip.String()
}

type memoryHTTP01 struct {
	mu      sync.RWMutex
	records map[string]string
}

func (p *memoryHTTP01) Present(token, keyAuth string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.records == nil {
		p.records = map[string]string{}
	}
	p.records[token] = keyAuth
}

func (p *memoryHTTP01) CleanUp(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.records, token)
}

func (p *memoryHTTP01) lookup(token string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.records[token]
	return v, ok
}

type ipIssuer struct {
	mu       sync.Mutex
	ip       string
	email    string
	cacheDir string
	http01   *memoryHTTP01
	chalMu   sync.RWMutex
	alpnCert *tls.Certificate
	cert     *tls.Certificate
	stop     chan struct{}
	// challengeBound is set when permanent panel listeners answer ACME challenges.
	challengeBound bool
}

func newIPIssuer(ip, email, cacheDir string) (*ipIssuer, error) {
	ip = normalizeIPHost(ip)
	if net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("panel SSL: invalid IP address %q", ip)
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, err
	}
	iss := &ipIssuer{
		ip:       ip,
		email:    strings.TrimSpace(email),
		cacheDir: cacheDir,
		http01:   &memoryHTTP01{},
		stop:     make(chan struct{}),
	}
	if cert, err := iss.loadFromDisk(); err == nil && cert != nil {
		iss.cert = cert
		log.Printf("panel SSL: loaded IP cert for %s (expires %s)", ip, leafNotAfter(cert).Format(time.RFC3339))
	} else {
		if err := iss.obtain(); err != nil {
			return nil, fmt.Errorf("panel SSL: obtain IP cert for %s: %w", ip, err)
		}
	}
	go iss.renewLoop()
	return iss, nil
}

// MarkChallengeBound indicates permanent HTTP/TLS listeners are up.
func (iss *ipIssuer) MarkChallengeBound() {
	iss.mu.Lock()
	defer iss.mu.Unlock()
	iss.challengeBound = true
}

func sanitizeIPFilename(ip string) string {
	return strings.ReplaceAll(strings.ReplaceAll(ip, ":", "_"), ".", "_")
}

func (iss *ipIssuer) certPath() string {
	return filepath.Join(iss.cacheDir, "ip-"+sanitizeIPFilename(iss.ip)+"-cert.pem")
}
func (iss *ipIssuer) keyPath() string {
	return filepath.Join(iss.cacheDir, "ip-"+sanitizeIPFilename(iss.ip)+"-key.pem")
}
func (iss *ipIssuer) accountKeyPath() string {
	return filepath.Join(iss.cacheDir, "ip-account.key")
}

func leafNotAfter(cert *tls.Certificate) time.Time {
	if cert == nil || len(cert.Certificate) == 0 {
		return time.Time{}
	}
	c, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return time.Time{}
	}
	return c.NotAfter
}

func (iss *ipIssuer) loadFromDisk() (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(iss.certPath(), iss.keyPath())
	if err != nil {
		return nil, err
	}
	if time.Until(leafNotAfter(&cert)) < time.Hour {
		return nil, fmt.Errorf("cert expired or near expiry")
	}
	return &cert, nil
}

func (iss *ipIssuer) obtain() error {
	return iss.obtainLocked()
}

func (iss *ipIssuer) alpnChallengeCert() *tls.Certificate {
	iss.chalMu.RLock()
	defer iss.chalMu.RUnlock()
	return iss.alpnCert
}

func (iss *ipIssuer) setALPNCert(c *tls.Certificate) {
	iss.chalMu.Lock()
	iss.alpnCert = c
	iss.chalMu.Unlock()
}

func (iss *ipIssuer) startTempChallengeListeners() (cleanup func()) {
	var closers []io.Closer
	cleanup = func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i].Close()
		}
	}

	ln80, err := net.Listen("tcp", ":80")
	if err != nil {
		log.Printf("panel SSL: temp HTTP-01 :80 unavailable (%v); will try TLS-ALPN-01 if needed", err)
	} else {
		closers = append(closers, ln80)
		srv := &http.Server{
			Handler: iss.httpHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "ACME challenge only", http.StatusNotFound)
			})),
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() { _ = srv.Serve(ln80) }()
		log.Printf("panel SSL: temp ACME HTTP-01 listening on :80")
	}

	ln443, err := net.Listen("tcp", ":443")
	if err != nil {
		log.Printf("panel SSL: temp TLS-ALPN-01 :443 unavailable (%v)", err)
	} else {
		closers = append(closers, ln443)
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{acmez.ACMETLS1Protocol},
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				c := iss.alpnChallengeCert()
				if c == nil {
					return nil, fmt.Errorf("no TLS-ALPN-01 challenge cert ready")
				}
				return c, nil
			},
		}
		go func() {
			for {
				conn, err := ln443.Accept()
				if err != nil {
					return
				}
				go func(c net.Conn) {
					defer c.Close()
					tlsConn := tls.Server(c, tlsCfg)
					_ = tlsConn.Handshake()
					_ = tlsConn.Close()
				}(conn)
			}
		}()
		log.Printf("panel SSL: temp ACME TLS-ALPN-01 listening on :443")
	}
	return cleanup
}

// ipChallengeSolver implements acmez.Solver for HTTP-01 and TLS-ALPN-01.
type ipChallengeSolver struct {
	iss *ipIssuer
}

func (s *ipChallengeSolver) Present(_ context.Context, chal zacme.Challenge) error {
	switch chal.Type {
	case zacme.ChallengeTypeHTTP01:
		s.iss.http01.Present(chal.Token, chal.KeyAuthorization)
		log.Printf("panel SSL: presenting HTTP-01 for %s", s.iss.ip)
	case zacme.ChallengeTypeTLSALPN01:
		cert, err := acmez.TLSALPN01ChallengeCert(chal)
		if err != nil {
			return fmt.Errorf("TLS-ALPN-01 challenge cert: %w", err)
		}
		s.iss.setALPNCert(cert)
		log.Printf("panel SSL: presenting TLS-ALPN-01 for %s", s.iss.ip)
	default:
		return fmt.Errorf("unsupported challenge type %q", chal.Type)
	}
	return nil
}

func (s *ipChallengeSolver) CleanUp(_ context.Context, chal zacme.Challenge) error {
	switch chal.Type {
	case zacme.ChallengeTypeHTTP01:
		s.iss.http01.CleanUp(chal.Token)
	case zacme.ChallengeTypeTLSALPN01:
		s.iss.setALPNCert(nil)
	}
	return nil
}

func (iss *ipIssuer) obtainLocked() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	iss.mu.Lock()
	bound := iss.challengeBound
	iss.mu.Unlock()
	if !bound {
		cleanup := iss.startTempChallengeListeners()
		defer cleanup()
		time.Sleep(200 * time.Millisecond)
	}

	accountKey, err := iss.loadOrCreateAccountKey()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	low := &zacme.Client{
		Directory: leDirectory,
		Logger:    logger,
	}
	client := acmez.Client{
		Client: low,
		ChallengeSolvers: map[string]acmez.Solver{
			zacme.ChallengeTypeHTTP01:    &ipChallengeSolver{iss: iss},
			zacme.ChallengeTypeTLSALPN01: &ipChallengeSolver{iss: iss},
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
		// Existing account with same key — try lookup.
		account, err = low.GetAccount(ctx, account)
		if err != nil {
			return fmt.Errorf("ACME account: %w", err)
		}
	}

	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	// NewCSR puts IPs only in SAN IPAddresses (no Common Name) — required by LE.
	csr, err := acmez.NewCSR(certKey, []string{iss.ip})
	if err != nil {
		return fmt.Errorf("CSR: %w", err)
	}
	params, err := acmez.OrderParametersFromCSR(account, csr)
	if err != nil {
		return fmt.Errorf("order params: %w", err)
	}
	params.Profile = ipCertProfile

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
	iss.mu.Lock()
	iss.cert = &tlsCert
	iss.mu.Unlock()
	log.Printf("panel SSL: obtained Let's Encrypt IP cert for %s (profile=%s via acmez, expires %s)",
		iss.ip, ipCertProfile, leafNotAfter(&tlsCert).Format(time.RFC3339))
	return nil
}

func (iss *ipIssuer) loadOrCreateAccountKey() (crypto.Signer, error) {
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

func (iss *ipIssuer) renewLoop() {
	t := time.NewTicker(ipRenewTick)
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

func (iss *ipIssuer) maybeRenew() {
	iss.mu.Lock()
	need := iss.cert == nil || time.Until(leafNotAfter(iss.cert)) <= ipRenewBefore
	iss.mu.Unlock()
	if !need {
		return
	}
	log.Printf("panel SSL: renewing IP cert for %s", iss.ip)
	if err := iss.obtainLocked(); err != nil {
		log.Printf("panel SSL: IP cert renew failed for %s: %v", iss.ip, err)
	}
}

func (iss *ipIssuer) certificate() *tls.Certificate {
	iss.mu.Lock()
	defer iss.mu.Unlock()
	return iss.cert
}

func (iss *ipIssuer) httpHandler(fallback http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			token := strings.TrimPrefix(r.URL.Path, "/.well-known/acme-challenge/")
			if token != "" {
				if keyAuth, ok := iss.http01.lookup(token); ok {
					w.Header().Set("Content-Type", "text/plain")
					_, _ = w.Write([]byte(keyAuth))
					return
				}
			}
		}
		if fallback != nil {
			fallback.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

func (iss *ipIssuer) close() {
	select {
	case <-iss.stop:
	default:
		close(iss.stop)
	}
}
