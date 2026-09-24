package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	xacme "golang.org/x/crypto/acme"
)

const (
	ipCertProfile = "shortlived"
	ipRenewBefore = 48 * time.Hour
	ipRenewTick   = 12 * time.Hour
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
func (iss *ipIssuer) accountURIPath() string {
	return filepath.Join(iss.cacheDir, "ip-account.uri")
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
			NextProtos: []string{xacme.ALPNProto},
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

func pickChallenge(az *xacme.Authorization, typ string) *xacme.Challenge {
	for i := range az.Challenges {
		if az.Challenges[i].Type == typ {
			return az.Challenges[i]
		}
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
	client := &xacme.Client{Key: accountKey, DirectoryURL: xacme.LetsEncryptURL}
	dir, err := client.Discover(ctx)
	if err != nil {
		return fmt.Errorf("ACME discover: %w", err)
	}
	var contact []string
	if iss.email != "" {
		contact = []string{"mailto:" + iss.email}
	}
	acct, err := client.Register(ctx, &xacme.Account{Contact: contact}, xacme.AcceptTOS)
	if err != nil {
		acct, err = client.GetReg(ctx, "")
		if err != nil {
			return fmt.Errorf("ACME register: %w", err)
		}
	}
	kid := ""
	if acct != nil {
		kid = acct.URI
	}
	if kid != "" {
		_ = os.WriteFile(iss.accountURIPath(), []byte(kid), 0o600)
	} else if b, e := os.ReadFile(iss.accountURIPath()); e == nil {
		kid = strings.TrimSpace(string(b))
	}
	if kid == "" {
		return fmt.Errorf("ACME account KID missing")
	}

	order, err := createOrderWithProfile(ctx, client, dir, accountKey, kid, iss.ip, ipCertProfile)
	if err != nil {
		return err
	}

	for _, zurl := range order.AuthzURLs {
		az, err := client.GetAuthorization(ctx, zurl)
		if err != nil {
			return err
		}
		if az.Status == xacme.StatusValid {
			continue
		}
		httpChal := pickChallenge(az, "http-01")
		alpnChal := pickChallenge(az, "tls-alpn-01")
		if httpChal == nil && alpnChal == nil {
			return fmt.Errorf("no http-01 or tls-alpn-01 challenge offered for IP %s", iss.ip)
		}

		var lastErr error
		if httpChal != nil {
			lastErr = iss.solveHTTP01(ctx, client, az, httpChal)
			if lastErr == nil {
				continue
			}
			log.Printf("panel SSL: HTTP-01 failed for %s: %v; trying TLS-ALPN-01", iss.ip, lastErr)
		}

		if alpnChal == nil {
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("no usable ACME challenge (open TCP :80 for HTTP-01 and/or :443 for TLS-ALPN-01)")
		}

		az2, err := client.GetAuthorization(ctx, zurl)
		if err != nil {
			return err
		}
		if az2.Status == xacme.StatusValid {
			continue
		}
		if az2.Status == xacme.StatusInvalid {
			order2, err := createOrderWithProfile(ctx, client, dir, accountKey, kid, iss.ip, ipCertProfile)
			if err != nil {
				return fmt.Errorf("HTTP-01 failed (%v); new order for TLS-ALPN-01: %w", lastErr, err)
			}
			order = order2
			ok := false
			for _, z2 := range order.AuthzURLs {
				az3, err := client.GetAuthorization(ctx, z2)
				if err != nil {
					return err
				}
				if az3.Status == xacme.StatusValid {
					ok = true
					break
				}
				alpn := pickChallenge(az3, "tls-alpn-01")
				if alpn == nil {
					return fmt.Errorf("HTTP-01 failed (%v); no tls-alpn-01 on retry order", lastErr)
				}
				if err := iss.solveTLSALPN01(ctx, client, az3, alpn); err != nil {
					return fmt.Errorf("HTTP-01 failed (%v); TLS-ALPN-01 failed: %w", lastErr, err)
				}
				ok = true
			}
			if !ok {
				return fmt.Errorf("HTTP-01 failed (%v); TLS-ALPN-01 retry incomplete", lastErr)
			}
			continue
		}

		alpn := pickChallenge(az2, "tls-alpn-01")
		if alpn == nil {
			alpn = alpnChal
		}
		if err := iss.solveTLSALPN01(ctx, client, az2, alpn); err != nil {
			if lastErr != nil {
				return fmt.Errorf("HTTP-01 failed (%v); TLS-ALPN-01 failed: %w", lastErr, err)
			}
			return fmt.Errorf("TLS-ALPN-01 failed: %w", err)
		}
	}

	order, err = client.WaitOrder(ctx, order.URI)
	if err != nil {
		return fmt.Errorf("wait order: %w", err)
	}

	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	ip := net.ParseIP(iss.ip)
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:     pkix.Name{CommonName: iss.ip},
		IPAddresses: []net.IP{ip},
	}, certKey)
	if err != nil {
		return err
	}
	der, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csrDER, true)
	if err != nil {
		return fmt.Errorf("finalize order: %w", err)
	}
	var certPEM []byte
	for _, c := range der {
		certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c})...)
	}
	keyDER, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
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
	log.Printf("panel SSL: obtained Let's Encrypt IP cert for %s (profile=%s, expires %s)",
		iss.ip, ipCertProfile, leafNotAfter(&tlsCert).Format(time.RFC3339))
	return nil
}

func (iss *ipIssuer) solveHTTP01(ctx context.Context, client *xacme.Client, az *xacme.Authorization, chal *xacme.Challenge) error {
	resp, err := client.HTTP01ChallengeResponse(chal.Token)
	if err != nil {
		return err
	}
	iss.http01.Present(chal.Token, resp)
	defer iss.http01.CleanUp(chal.Token)
	if _, err := client.Accept(ctx, chal); err != nil {
		return fmt.Errorf("accept http-01: %w", err)
	}
	if _, err := client.WaitAuthorization(ctx, az.URI); err != nil {
		return fmt.Errorf("wait authorization (http-01): %w", err)
	}
	log.Printf("panel SSL: HTTP-01 OK for %s", iss.ip)
	return nil
}

func (iss *ipIssuer) solveTLSALPN01(ctx context.Context, client *xacme.Client, az *xacme.Authorization, chal *xacme.Challenge) error {
	cert, err := client.TLSALPN01ChallengeCert(chal.Token, iss.ip)
	if err != nil {
		return fmt.Errorf("build tls-alpn-01 cert: %w", err)
	}
	iss.setALPNCert(&cert)
	defer iss.setALPNCert(nil)
	if _, err := client.Accept(ctx, chal); err != nil {
		return fmt.Errorf("accept tls-alpn-01: %w", err)
	}
	if _, err := client.WaitAuthorization(ctx, az.URI); err != nil {
		return fmt.Errorf("wait authorization (tls-alpn-01): %w", err)
	}
	log.Printf("panel SSL: TLS-ALPN-01 OK for %s", iss.ip)
	return nil
}

func createOrderWithProfile(ctx context.Context, client *xacme.Client, dir xacme.Directory, key crypto.Signer, kid, ip, profile string) (*xacme.Order, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"identifiers": []map[string]string{{"type": "ip", "value": ip}},
		"profile":     profile,
	})
	if err != nil {
		return nil, err
	}
	nonce, err := fetchNonce(ctx, client, dir.NonceURL)
	if err != nil {
		return nil, err
	}
	jws, err := signACME(key, kid, dir.OrderURL, nonce, payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dir.OrderURL, strings.NewReader(jws))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/jose+json")
	req.Header.Set("User-Agent", "3m-ui-ip-acme/1")
	hc := client.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("newOrder HTTP %d: %s", res.StatusCode, truncate(string(body), 500))
	}
	var wire struct {
		Status         string   `json:"status"`
		Authorizations []string `json:"authorizations"`
		Finalize       string   `json:"finalize"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, err
	}
	return &xacme.Order{
		URI:         res.Header.Get("Location"),
		Status:      wire.Status,
		AuthzURLs:   wire.Authorizations,
		FinalizeURL: wire.Finalize,
	}, nil
}

func fetchNonce(ctx context.Context, client *xacme.Client, nonceURL string) (string, error) {
	hc := client.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, nonceURL, nil)
	if err != nil {
		return "", err
	}
	res, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	res.Body.Close()
	n := res.Header.Get("Replay-Nonce")
	if n == "" {
		return "", fmt.Errorf("missing Replay-Nonce from %s", nonceURL)
	}
	return n, nil
}

func signACME(key crypto.Signer, kid, url, nonce string, payload []byte) (string, error) {
	opts := &jose.SignerOptions{EmbedJWK: false}
	opts.ExtraHeaders = map[jose.HeaderKey]interface{}{
		"nonce": nonce,
		"url":   url,
		"kid":   kid,
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, opts)
	if err != nil {
		return "", err
	}
	obj, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return obj.FullSerialize(), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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
