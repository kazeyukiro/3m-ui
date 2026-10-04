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
	domainHTTPRenewBefore = 30 * 24 * time.Hour
	domainHTTPRenewTick   = 12 * time.Hour
)

// domainHTTPIssuer obtains and renews domain certificates via Let's Encrypt HTTP-01 (acmez).
// Replaces golang.org/x/crypto/acme/autocert for panel domain HTTPS.
type domainHTTPIssuer struct {
	mu       sync.Mutex
	domain   string
	email    string
	cacheDir string
	http01   *memoryHTTP01
	cert     *tls.Certificate
	stop     chan struct{}
}

func newDomainHTTPIssuer(domain, email, cacheDir string) (*domainHTTPIssuer, error) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return nil, fmt.Errorf("panel SSL: domain is required for HTTP-01")
	}
	if IsIPHost(domain) {
		return nil, fmt.Errorf("panel SSL: HTTP-01 domain issuer does not accept IP addresses")
	}
	if IsWildcardDomain(domain) {
		return nil, fmt.Errorf("panel SSL: wildcard domains require DNS-01")
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, err
	}
	iss := &domainHTTPIssuer{
		domain:   domain,
		email:    strings.TrimSpace(email),
		cacheDir: cacheDir,
		http01:   &memoryHTTP01{},
		stop:     make(chan struct{}),
	}
	if cert, err := iss.loadFromDisk(); err == nil && cert != nil {
		iss.cert = cert
		log.Printf("panel SSL: loaded HTTP-01 cert for %s (expires %s)", domain, leafNotAfter(cert).Format(time.RFC3339))
	} else {
		if err := iss.obtain(); err != nil {
			return nil, fmt.Errorf("panel SSL: obtain HTTP-01 cert for %s: %w", domain, err)
		}
	}
	go iss.renewLoop()
	return iss, nil
}

func (iss *domainHTTPIssuer) certPath() string {
	return filepath.Join(iss.cacheDir, "http-"+iss.domain+".crt")
}

func (iss *domainHTTPIssuer) keyPath() string {
	return filepath.Join(iss.cacheDir, "http-"+iss.domain+".key")
}

func (iss *domainHTTPIssuer) accountKeyPath() string {
	return filepath.Join(iss.cacheDir, "http-account.ec.key")
}

func (iss *domainHTTPIssuer) loadFromDisk() (*tls.Certificate, error) {
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

func (iss *domainHTTPIssuer) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	iss.mu.Lock()
	defer iss.mu.Unlock()
	if iss.cert == nil {
		return nil, fmt.Errorf("panel SSL: no HTTP-01 certificate loaded")
	}
	if hello != nil && hello.ServerName != "" {
		sn := strings.ToLower(strings.TrimSuffix(hello.ServerName, "."))
		if sn != iss.domain && !strings.HasSuffix(sn, "."+iss.domain) {
			// Still serve the panel cert — single-name panel setup.
		}
	}
	return iss.cert, nil
}

func (iss *domainHTTPIssuer) Close() {
	select {
	case <-iss.stop:
	default:
		close(iss.stop)
	}
}

func (iss *domainHTTPIssuer) httpHandler(fallback http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			token := strings.TrimPrefix(r.URL.Path, "/.well-known/acme-challenge/")
			if keyAuth, ok := iss.http01.lookup(token); ok {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte(keyAuth))
				return
			}
		}
		if fallback != nil {
			fallback.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

func (iss *domainHTTPIssuer) obtain() error {
	iss.mu.Lock()
	defer iss.mu.Unlock()
	return iss.obtainLocked()
}

func (iss *domainHTTPIssuer) obtainLocked() error {
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
	client := acmez.Client{
		Client: low,
		ChallengeSolvers: map[string]acmez.Solver{
			zacme.ChallengeTypeHTTP01: &domainHTTP01Solver{iss: iss},
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
	csr, err := acmez.NewCSR(certKey, []string{iss.domain})
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
	log.Printf("panel SSL: obtained HTTP-01 cert for %s (expires %s)",
		iss.domain, leafNotAfter(&tlsCert).Format(time.RFC3339))
	return nil
}

func (iss *domainHTTPIssuer) loadOrCreateAccountKey() (crypto.Signer, error) {
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
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (iss *domainHTTPIssuer) renewLoop() {
	t := time.NewTicker(domainHTTPRenewTick)
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

func (iss *domainHTTPIssuer) maybeRenew() {
	iss.mu.Lock()
	cert := iss.cert
	iss.mu.Unlock()
	if cert == nil {
		_ = iss.obtain()
		return
	}
	if time.Until(leafNotAfter(cert)) > domainHTTPRenewBefore {
		return
	}
	log.Printf("panel SSL: renewing HTTP-01 cert for %s", iss.domain)
	if err := iss.obtain(); err != nil {
		log.Printf("panel SSL: HTTP-01 renew failed: %v", err)
	}
}

type domainHTTP01Solver struct {
	iss *domainHTTPIssuer
}

func (s *domainHTTP01Solver) Present(_ context.Context, chal zacme.Challenge) error {
	if chal.Type != zacme.ChallengeTypeHTTP01 {
		return fmt.Errorf("unsupported challenge type %q", chal.Type)
	}
	s.iss.http01.Present(chal.Token, chal.KeyAuthorization)
	log.Printf("panel SSL: presenting HTTP-01 for %s", s.iss.domain)
	return nil
}

func (s *domainHTTP01Solver) CleanUp(_ context.Context, chal zacme.Challenge) error {
	if chal.Type == zacme.ChallengeTypeHTTP01 {
		s.iss.http01.CleanUp(chal.Token)
	}
	return nil
}
