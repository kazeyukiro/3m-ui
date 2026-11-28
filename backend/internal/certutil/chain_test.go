package certutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestChain builds root -> intermediate -> leaf, with an IP SAN leaf that
// carries an AIA URL: the same shape as a Let's Encrypt short-lived IP cert.
type testChain struct {
	rootPEM, intPEM, leafPEM, keyPEM []byte
	intDER                           []byte
}

func newTestChain(t *testing.T) testChain {
	t.Helper()
	serial := int64(1)
	issue := func(tmpl *x509.Certificate, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) ([]byte, *x509.Certificate, *ecdsa.PrivateKey) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		serial++
		tmpl.SerialNumber = big.NewInt(serial)
		tmpl.NotBefore, tmpl.NotAfter = time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
		if parent == nil {
			parent, parentKey = tmpl, key
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
		if err != nil {
			t.Fatal(err)
		}
		cert, _ := x509.ParseCertificate(der)
		return der, cert, key
	}
	encode := func(der []byte) []byte { return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}) }
	rootDER, root, rootKey := issue(&x509.Certificate{
		Subject: pkix.Name{CommonName: "Test Root"}, IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}, nil, nil)
	intDER, inter, intKey := issue(&x509.Certificate{
		Subject: pkix.Name{CommonName: "YE1"}, IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}, root, rootKey)
	leafDER, _, leafKey := issue(&x509.Certificate{
		IPAddresses:           []net.IP{net.ParseIP("154.64.232.36")},
		IssuingCertificateURL: []string{"http://ye1.i.lencr.test/"},
	}, inter, intKey)
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatal(err)
	}
	return testChain{
		rootPEM: encode(rootDER), intPEM: encode(intDER), leafPEM: encode(leafDER), intDER: intDER,
		keyPEM: pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	}
}

func writeTestFile(t *testing.T, path string, parts ...[]byte) {
	t.Helper()
	var data []byte
	for _, part := range parts {
		data = append(data, part...)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func isolateChainSources(t *testing.T, acmeDir string, fetch func(string) ([]byte, error)) {
	t.Helper()
	oldDirs, oldFetch := chainSearchDirs, chainFetch
	chainSearchDirs = func() []string { return []string{acmeDir} }
	if fetch == nil {
		fetch = func(string) ([]byte, error) { return nil, errors.New("offline") }
	}
	chainFetch = fetch
	t.Cleanup(func() { chainSearchDirs, chainFetch = oldDirs, oldFetch })
}

func certCount(pemData []byte) int { return len(parseCertificatesAnyEncoding(pemData)) }

func TestCompleteChainKeepsFullChainAsIs(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	isolateChainSources(t, filepath.Join(dir, "acme"), func(string) ([]byte, error) {
		t.Fatal("an already complete chain must not trigger an AIA download")
		return nil, nil
	})
	in := append(append(append([]byte(nil), chain.leafPEM...), chain.intPEM...), chain.rootPEM...)
	out, st := CompleteChainPEM(in, filepath.Join(dir, "fullchain.pem"))
	if !st.Checked || !st.Complete || st.Incomplete || st.Source != "" || st.Certificates != 3 {
		t.Fatalf("status = %+v", st)
	}
	if string(out) != string(in) {
		t.Fatal("a complete chain must be returned byte-for-byte unchanged")
	}
}

func TestCompleteChainFromSiblingFullchain(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	isolateChainSources(t, filepath.Join(dir, "acme"), nil)
	hint := filepath.Join(dir, "panel", "cert.pem")
	writeTestFile(t, hint, chain.leafPEM)
	want := filepath.Join(dir, "panel", "fullchain.cer")
	writeTestFile(t, want, chain.leafPEM, chain.intPEM, chain.rootPEM)
	out, st := CompleteChainPEM(chain.leafPEM, hint)
	if !st.Complete || st.Incomplete || st.Source != want || certCount(out) != 3 {
		t.Fatalf("status = %+v, certs = %d", st, certCount(out))
	}
}

func TestCompleteChainFromAcmeShFullchain(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	acme := filepath.Join(dir, "acme")
	isolateChainSources(t, acme, nil)
	hint := filepath.Join(dir, "panel", "154.64.232.36.cer")
	writeTestFile(t, hint, chain.leafPEM)
	other := newTestChain(t)
	writeTestFile(t, filepath.Join(acme, "other.example_ecc", "fullchain.cer"), other.leafPEM, other.intPEM)
	want := filepath.Join(acme, "154.64.232.36_ecc", "fullchain.cer")
	writeTestFile(t, want, chain.leafPEM, chain.intPEM, chain.rootPEM)
	out, st := CompleteChainPEM(chain.leafPEM, hint)
	if !st.Complete || st.Incomplete || st.Source != want || certCount(out) != 3 {
		t.Fatalf("status = %+v, certs = %d", st, certCount(out))
	}
}

func TestCompleteChainFromSiblingCAFile(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	isolateChainSources(t, filepath.Join(dir, "acme"), nil)
	hint := filepath.Join(dir, "cert.pem")
	writeTestFile(t, hint, chain.leafPEM)
	want := filepath.Join(dir, "ca.cer")
	writeTestFile(t, want, chain.intPEM, chain.rootPEM)
	out, st := CompleteChainPEM(chain.leafPEM, hint)
	if !st.Complete || st.Incomplete || st.Source != want || certCount(out) != 3 {
		t.Fatalf("status = %+v, certs = %d", st, certCount(out))
	}
}

func TestCompleteChainFallsBackToAIADER(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	var fetched string
	isolateChainSources(t, filepath.Join(dir, "acme"), func(u string) ([]byte, error) {
		fetched = u
		return chain.intDER, nil // Let's Encrypt AIA endpoints return DER.
	})
	out, st := CompleteChainPEM(chain.leafPEM, filepath.Join(dir, "cert.pem"))
	if !st.Complete || st.Incomplete || fetched != "http://ye1.i.lencr.test/" ||
		st.Source != fetched || certCount(out) != 2 {
		t.Fatalf("status = %+v, fetched = %q, certs = %d", st, fetched, certCount(out))
	}
}

func TestCompleteChainRejectsUnrelatedCA(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	isolateChainSources(t, filepath.Join(dir, "acme"), nil)
	hint := filepath.Join(dir, "cert.pem")
	writeTestFile(t, hint, chain.leafPEM)
	writeTestFile(t, filepath.Join(dir, "ca.cer"), newTestChain(t).intPEM)
	out, st := CompleteChainPEM(chain.leafPEM, hint)
	if !st.Checked || !st.Incomplete || st.Complete || st.Source != "" || certCount(out) != 1 {
		t.Fatalf("status = %+v, certs = %d", st, certCount(out))
	}
}

func TestCompleteChainRejectsFullchainOfAnotherLeaf(t *testing.T) {
	chain := newTestChain(t)
	other := newTestChain(t)
	dir := t.TempDir()
	isolateChainSources(t, filepath.Join(dir, "acme"), nil)
	hint := filepath.Join(dir, "cert.pem")
	writeTestFile(t, hint, chain.leafPEM)
	writeTestFile(t, filepath.Join(dir, "fullchain.cer"), other.leafPEM, other.intPEM)
	out, st := CompleteChainPEM(chain.leafPEM, hint)
	if !st.Incomplete || certCount(out) != 1 {
		t.Fatalf("a fullchain for a different leaf must not be spliced in: %+v", st)
	}
}

func TestCompleteChainLeavesSelfSignedAlone(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	isolateChainSources(t, filepath.Join(dir, "acme"), func(string) ([]byte, error) {
		t.Fatal("a self-signed certificate must not trigger an AIA download")
		return nil, nil
	})
	pair, err := tls.X509KeyPair(chain.leafPEM, chain.keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	key := pair.PrivateKey.(*ecdsa.PrivateKey)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(99), Subject: pkix.Name{CommonName: "self"},
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	selfPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	out, st := CompleteChainPEM(selfPEM, filepath.Join(dir, "cert.pem"))
	if !st.Complete || !st.SelfSigned || st.Incomplete || string(out) != string(selfPEM) {
		t.Fatalf("status = %+v", st)
	}
}

func TestCompleteChainCacheAvoidsRepeatedAIAFetches(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	calls := 0
	isolateChainSources(t, filepath.Join(dir, "acme"), func(string) ([]byte, error) {
		calls++
		return chain.intDER, nil
	})
	hint := filepath.Join(dir, "cert.pem")
	first, st1 := CompleteChainPEM(chain.leafPEM, hint)
	second, st2 := CompleteChainPEM(chain.leafPEM, hint)
	if calls != 1 || certCount(first) != 2 || certCount(second) != 2 || st1.Source != st2.Source {
		t.Fatalf("calls=%d first=%d second=%d st1=%+v st2=%+v", calls, certCount(first), certCount(second), st1, st2)
	}
}

func TestLoadPEMPairWithChainProducesUsableTLSCertificate(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	isolateChainSources(t, filepath.Join(dir, "acme"), nil)
	hint := filepath.Join(dir, "cert.pem")
	writeTestFile(t, hint, chain.leafPEM)
	writeTestFile(t, filepath.Join(dir, "fullchain.cer"), chain.leafPEM, chain.intPEM, chain.rootPEM)
	cert, st, err := LoadPEMPairWithChain(chain.leafPEM, chain.keyPEM, hint)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) != 3 || !st.Complete {
		t.Fatalf("tls.Certificate chain = %d, status = %+v", len(cert.Certificate), st)
	}
	if _, err := x509.ParseCertificate(cert.Certificate[1]); err != nil {
		t.Fatalf("appended intermediate is not parseable: %v", err)
	}
}

func TestLoadCertificateWithChainFromDisk(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	isolateChainSources(t, filepath.Join(dir, "acme"), nil)
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	writeTestFile(t, certFile, chain.leafPEM)
	writeTestFile(t, keyFile, chain.keyPEM)
	writeTestFile(t, filepath.Join(dir, "chain.pem"), chain.intPEM, chain.rootPEM)
	cert, st, err := LoadCertificateWithChain(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) != 3 || !st.Complete || st.Source != filepath.Join(dir, "chain.pem") {
		t.Fatalf("chain = %d, status = %+v", len(cert.Certificate), st)
	}
}

func TestInspectChainPEMStates(t *testing.T) {
	chain := newTestChain(t)
	full := append(append(append([]byte(nil), chain.leafPEM...), chain.intPEM...), chain.rootPEM...)
	if st := InspectChainPEM(full); !st.Complete || st.Incomplete || st.Certificates != 3 {
		t.Fatalf("full chain: %+v", st)
	}
	if st := InspectChainPEM(chain.leafPEM); !st.Incomplete || st.Complete || st.Certificates != 1 {
		t.Fatalf("leaf only: %+v", st)
	}
	if st := InspectChainPEM(nil); st.Certificates != 0 || !st.Checked || st.Complete {
		t.Fatalf("empty: %+v", st)
	}
}

func TestInspectChainFileIsCachedAndFollowsChanges(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "cert.pem")
	writeTestFile(t, path, chain.leafPEM)
	if st := InspectChainFile(path); !st.Incomplete {
		t.Fatalf("leaf only file: %+v", st)
	}
	writeTestFile(t, path, chain.leafPEM, chain.intPEM, chain.rootPEM)
	if st := InspectChainFile(path); !st.Complete || st.Certificates != 3 {
		t.Fatalf("rewritten fullchain file: %+v", st)
	}
	if st := InspectChainFile(""); st.Checked {
		t.Fatalf("empty path should be unchecked: %+v", st)
	}
}

func TestChainStatusForPathFollowsRuntimeCompletion(t *testing.T) {
	chain := newTestChain(t)
	dir := t.TempDir()
	isolateChainSources(t, filepath.Join(dir, "acme"), nil)
	hint := filepath.Join(dir, "cert.pem")
	writeTestFile(t, hint, chain.leafPEM)
	writeTestFile(t, filepath.Join(dir, "fullchain.cer"), chain.leafPEM, chain.intPEM, chain.rootPEM)
	if _, ok := ChainStatusForPath(hint); ok {
		t.Fatal("no runtime status should exist before the first load")
	}
	out, st := CompleteChainPEM(chain.leafPEM, hint)
	got, ok := ChainStatusForPath(hint)
	if !ok || !got.Complete || got.Incomplete || got.Source != st.Source || certCount(out) != 3 {
		t.Fatalf("runtime status = %+v ok=%v, want complete with source %q", got, ok, st.Source)
	}
	if _, ok := ChainStatusForPath(filepath.Join(dir, "other.pem")); ok {
		t.Fatal("runtime status leaked to a different certificate path")
	}
	if _, ok := ChainStatusForPath(""); ok {
		t.Fatal("empty path must never have a runtime status")
	}
}

func TestAIAFetchRejectsPrivateAndMalformedURLs(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "http://127.0.0.1:1/issuer", "https://[::1]/issuer", ""} {
		if _, err := fetchAIACertificate(u); err == nil {
			t.Fatalf("AIA URL %q should have been refused", u)
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.1.1", "::1"} {
		if isPublicIP(net.ParseIP(ip)) {
			t.Fatalf("%s must not count as public", ip)
		}
	}
	if !isPublicIP(net.ParseIP("1.1.1.1")) {
		t.Fatal("1.1.1.1 must count as public")
	}
}
