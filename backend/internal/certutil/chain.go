package certutil

// Certificate-chain completion for panel and listener certificates.
//
// Why this exists: browsers and iOS fetch missing intermediates from the leaf's
// AIA extension, so a panel that serves only the leaf looks fine in a browser.
// Node.js (Mihomo Party), Go and Dart (FlClash) do not, and fail with
// "unable to verify the first certificate" when downloading a subscription.
// Let's Encrypt's short-lived IP certificates use the Gen Y hierarchy, whose
// full chain is leaf + YE1/YR1 + cross-signed roots; missing any link breaks
// verification. Completing the chain on the server side fixes the client
// without falling back to skip-cert-verify.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ChainStatus is a machine-readable description of a certificate chain.
// It is safe to expose through the SSL status API: it contains no certificate
// material, no host names and no file paths beyond the chain source label.
type ChainStatus struct {
	Checked      bool   `json:"checked"`
	Complete     bool   `json:"complete"`
	Incomplete   bool   `json:"incomplete"`
	SelfSigned   bool   `json:"self_signed"`
	Certificates int    `json:"certificates"`
	Source       string `json:"source"`
}

var fullchainNames = []string{"fullchain.cer", "fullchain.pem", "fullchain.crt", "fullchain"}
var caChainNames = []string{"ca.cer", "ca.pem", "ca.crt", "chain.pem", "chain.cer", "chain.crt"}

// chainSearchDirs lists extra roots (besides the certificate's own directory)
// that may hold a matching acme.sh fullchain. A variable so tests can isolate
// the host's real ~/.acme.sh.
var chainSearchDirs = func() []string {
	dirs := []string{"/root/.acme.sh"}
	if home, err := os.UserHomeDir(); err == nil && filepath.Clean(home) != "/root" {
		dirs = append(dirs, filepath.Join(home, ".acme.sh"))
	}
	return dirs
}

// chainFetch downloads an AIA issuer certificate. Replaced by a stub in tests.
// The default transport refuses private/loopback addresses: AIA URLs come from
// certificate extensions, i.e. from data we do not control.
var chainFetch = fetchAIACertificate

func fetchAIACertificate(rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("refusing AIA URL %q", rawURL)
	}
	client := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}
				d := &net.Dialer{Timeout: 3 * time.Second}
				for _, ip := range ips {
					if isPublicIP(ip.IP) {
						return d.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
					}
				}
				return nil, fmt.Errorf("AIA host %q resolves to non-public addresses", host)
			},
		},
	}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AIA %s: HTTP %d", rawURL, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<10))
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	return true
}

const chainCacheTTL = 5 * time.Minute

type chainCacheEntry struct {
	pem     []byte
	status  ChainStatus
	expires time.Time
}

var (
	chainCacheMu sync.Mutex
	chainCache   = map[string]chainCacheEntry{}
	lastChainMu  sync.Mutex
	lastChain    ChainStatus
	// lastByPath keeps the runtime result per certificate file so the status
	// API can report the chain the server actually served, not just the raw
	// bytes on disk (which may still be leaf-only after an in-memory fix).
	lastByPath = map[string]ChainStatus{}
)

func recordChainStatus(s ChainStatus) {
	lastChainMu.Lock()
	lastChain = s
	lastChainMu.Unlock()
}

func recordChainStatusForPath(path string, s ChainStatus) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return
	}
	lastChainMu.Lock()
	if len(lastByPath) > 256 {
		lastByPath = map[string]ChainStatus{}
	}
	lastByPath[path] = s
	lastChainMu.Unlock()
}

// ChainStatusForPath returns the most recent runtime chain status for a
// certificate file. ok is false when that file has not been loaded by this
// process (callers should fall back to InspectChainFile).
func ChainStatusForPath(path string) (ChainStatus, bool) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return ChainStatus{}, false
	}
	lastChainMu.Lock()
	defer lastChainMu.Unlock()
	s, ok := lastByPath[path]
	return s, ok
}

// LastChainStatus returns the status of the most recent runtime chain load.
// It never reads disk or touches the network.
func LastChainStatus() ChainStatus {
	lastChainMu.Lock()
	defer lastChainMu.Unlock()
	return lastChain
}

func chainCacheKey(certPEM []byte, hintPath string) string {
	h := sha256.Sum256(certPEM)
	return hex.EncodeToString(h[:]) + "|" + hintPath
}

// CompleteChainPEM returns certPEM with any missing intermediates appended.
// hintPath is the on-disk path of the certificate when known (it anchors the
// sibling fullchain/ca search); it may be empty for pasted PEM.
//
// The result is cached for chainCacheTTL so a subscription or status call
// cannot repeatedly re-read the filesystem or re-download an AIA issuer.
func CompleteChainPEM(certPEM []byte, hintPath string) ([]byte, ChainStatus) {
	key := chainCacheKey(certPEM, hintPath)
	chainCacheMu.Lock()
	if e, ok := chainCache[key]; ok && time.Now().Before(e.expires) {
		chainCacheMu.Unlock()
		out := append([]byte(nil), e.pem...)
		recordChainStatus(e.status)
		recordChainStatusForPath(hintPath, e.status)
		return out, e.status
	}
	chainCacheMu.Unlock()

	out, status := completeChainPEM(certPEM, hintPath)

	chainCacheMu.Lock()
	if len(chainCache) > 256 {
		chainCache = map[string]chainCacheEntry{}
	}
	chainCache[key] = chainCacheEntry{pem: append([]byte(nil), out...), status: status, expires: time.Now().Add(chainCacheTTL)}
	chainCacheMu.Unlock()
	recordChainStatus(status)
	recordChainStatusForPath(hintPath, status)
	return out, status
}

func completeChainPEM(certPEM []byte, hintPath string) ([]byte, ChainStatus) {
	certs := parseCertificatesAnyEncoding(certPEM)
	if len(certs) == 0 {
		return certPEM, ChainStatus{Checked: true}
	}
	if len(certs) > 1 {
		return certPEM, ChainStatus{Checked: true, Complete: true, Certificates: len(certs)}
	}
	leaf := certs[0]
	// A self-signed leaf has no issuer to append. CheckSignature (rather than
	// CheckSignatureFrom) is used because a plain self-signed leaf often lacks
	// the CA basic constraint that CheckSignatureFrom requires.
	if bytes.Equal(leaf.RawIssuer, leaf.RawSubject) &&
		leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature) == nil {
		return certPEM, ChainStatus{Checked: true, Complete: true, SelfSigned: true, Certificates: 1}
	}

	chain, source := findChain(hintPath, leaf)
	if len(chain) == 0 {
		return certPEM, ChainStatus{Checked: true, Incomplete: true, Certificates: 1}
	}
	out := append([]byte(nil), bytes.TrimRight(certPEM, "\r\n")...)
	out = append(out, '\n')
	for _, c := range chain {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
	}
	return out, ChainStatus{Checked: true, Complete: true, Certificates: 1 + len(chain), Source: source}
}

// findChain tries, in order: a matching fullchain in the cert directory or an
// acme.sh directory, a sibling ca/chain file, then the leaf's AIA issuer.
func findChain(hintPath string, leaf *x509.Certificate) ([]*x509.Certificate, string) {
	certDir := ""
	if hintPath != "" {
		certDir = filepath.Dir(hintPath)
	}

	var fullchainCandidates []string
	if certDir != "" {
		for _, name := range fullchainNames {
			fullchainCandidates = append(fullchainCandidates, filepath.Join(certDir, name))
		}
	}
	for _, dir := range chainSearchDirs() {
		for _, name := range fullchainNames {
			matches, _ := filepath.Glob(filepath.Join(dir, "*", name))
			fullchainCandidates = append(fullchainCandidates, matches...)
		}
	}
	for _, path := range fullchainCandidates {
		certs := readPEMCertificates(path)
		// The first certificate must be this exact leaf; otherwise the file
		// belongs to another domain and appending it would corrupt the chain.
		if len(certs) > 1 && bytes.Equal(certs[0].Raw, leaf.Raw) && chainLinksToLeaf(leaf, certs[1:]) {
			return certs[1:], path
		}
	}

	if certDir != "" {
		for _, name := range caChainNames {
			path := filepath.Join(certDir, name)
			certs := readPEMCertificates(path)
			for i := range certs {
				if chainLinksToLeaf(leaf, certs[i:]) {
					return certs[i:], path
				}
			}
		}
	}

	for _, rawURL := range leaf.IssuingCertificateURL {
		u, err := url.Parse(rawURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			continue
		}
		data, err := chainFetch(rawURL)
		if err != nil {
			continue
		}
		for _, c := range parseCertificatesAnyEncoding(data) {
			if leaf.CheckSignatureFrom(c) == nil {
				return []*x509.Certificate{c}, rawURL
			}
		}
	}
	return nil, ""
}

// chainLinksToLeaf verifies every appended certificate actually signs the one
// before it. This is what stops a stray ca.cer from being spliced in.
func chainLinksToLeaf(leaf *x509.Certificate, chain []*x509.Certificate) bool {
	if len(chain) == 0 {
		return false
	}
	cur := leaf
	for _, c := range chain {
		if cur.CheckSignatureFrom(c) != nil {
			return false
		}
		cur = c
	}
	return true
}

func readPEMCertificates(path string) []*x509.Certificate {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseCertificatesAnyEncoding(data)
}

// parseCertificatesAnyEncoding accepts PEM and DER (Let's Encrypt AIA
// endpoints return DER) and ignores non-certificate PEM blocks.
func parseCertificatesAnyEncoding(data []byte) []*x509.Certificate {
	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		if c, err := x509.ParseCertificate(block.Bytes); err == nil {
			certs = append(certs, c)
		}
	}
	if len(certs) == 0 {
		if parsed, err := x509.ParseCertificates(data); err == nil {
			certs = parsed
		}
	}
	return certs
}

// LoadPEMPairWithChain builds a tls.Certificate from PEM bytes, completing the
// chain first. hintPath should be the certificate's file path when available.
func LoadPEMPairWithChain(certPEM, keyPEM []byte, hintPath string) (*tls.Certificate, ChainStatus, error) {
	completed, status := CompleteChainPEM(certPEM, hintPath)
	cert, err := tls.X509KeyPair(completed, keyPEM)
	if err != nil {
		return nil, status, err
	}
	return &cert, status, nil
}

// LoadCertificateWithChain loads a cert/key pair from disk and completes the
// chain. It is the drop-in replacement for tls.LoadX509KeyPair on panel TLS.
func LoadCertificateWithChain(certFile, keyFile string) (*tls.Certificate, ChainStatus, error) {
	certPEM, err := os.ReadFile(filepath.Clean(certFile))
	if err != nil {
		return nil, ChainStatus{}, fmt.Errorf("read certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(filepath.Clean(keyFile))
	if err != nil {
		return nil, ChainStatus{}, fmt.Errorf("read private key: %w", err)
	}
	return LoadPEMPairWithChain(certPEM, keyPEM, certFile)
}

// InspectChainPEM reports the static state of a PEM chain without consulting
// the filesystem or the network. Used by the SSL status API.
func InspectChainPEM(certPEM []byte) ChainStatus {
	certs := parseCertificatesAnyEncoding(certPEM)
	if len(certs) == 0 {
		return ChainStatus{Checked: true}
	}
	if len(certs) > 1 {
		return ChainStatus{Checked: true, Complete: true, Certificates: len(certs)}
	}
	leaf := certs[0]
	if bytes.Equal(leaf.RawIssuer, leaf.RawSubject) &&
		leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature) == nil {
		return ChainStatus{Checked: true, Complete: true, SelfSigned: true, Certificates: 1}
	}
	return ChainStatus{Checked: true, Incomplete: true, Certificates: 1}
}

var (
	inspectMu    sync.Mutex
	inspectCache = map[string]inspectEntry{}
)

type inspectEntry struct {
	status ChainStatus
}

// InspectChainFile statically inspects a certificate file. Results are cached
// by path + size + mtime so the status endpoint (polled every few seconds)
// never re-reads a changing file more often than it changes.
func InspectChainFile(path string) ChainStatus {
	path = strings.TrimSpace(path)
	if path == "" {
		return ChainStatus{}
	}
	fi, err := os.Stat(path)
	if err != nil {
		return ChainStatus{}
	}
	key := fmt.Sprintf("%s|%d|%d", path, fi.Size(), fi.ModTime().UnixNano())
	inspectMu.Lock()
	if e, ok := inspectCache[key]; ok {
		inspectMu.Unlock()
		return e.status
	}
	inspectMu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return ChainStatus{}
	}
	status := InspectChainPEM(data)

	inspectMu.Lock()
	if len(inspectCache) > 256 {
		inspectCache = map[string]inspectEntry{}
	}
	inspectCache[key] = inspectEntry{status: status}
	inspectMu.Unlock()
	return status
}
