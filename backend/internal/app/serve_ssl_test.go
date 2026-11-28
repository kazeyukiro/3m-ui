package app

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/acme"
)

func newTestHTTP01Manager(t *testing.T) *acme.Manager {
	t.Helper()
	mgr, err := acme.NewManager(acme.Settings{
		Enabled:    true,
		Domain:     "panel.example.com",
		CacheDir:   t.TempDir(),
		ListenHTTP: ":80",
		ListenTLS:  ":443",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}

func challengeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("key-authorization"))
			return
		}
		http.NotFound(w, r)
	})
}

// Regression for issue #106: first-time HTTP-01 issuance used to run inside
// NewManager, before the permanent :80 challenge listener was bound, so the
// CA always got "Connection refused". The ensure callback stands in for the
// ACME obtain() call and performs the CA-style token fetch.
func TestFirstTimeIssuanceRunsAfterChallengeListenerIsBound(t *testing.T) {
	mgr := newTestHTTP01Manager(t)
	if !mgr.NeedsHTTP01Listener() {
		t.Fatal("expected a first-time HTTP-01 cert to require a bound challenge listener")
	}

	ensureCalled := false
	ln, srv, err := bindThenEnsure(mgr, "127.0.0.1:0", challengeHandler(), func(_ *acme.Manager, boundAddr string) error {
		ensureCalled = true
		if boundAddr == "" {
			return errors.New("challenge listener was not bound before issuance")
		}
		resp, err := http.Get("http://" + boundAddr + "/.well-known/acme-challenge/token")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK || string(body) != "key-authorization" {
			return fmt.Errorf("challenge not served yet: status=%d body=%q", resp.StatusCode, body)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("issuance must see a working challenge listener: %v", err)
	}
	if !ensureCalled || ln == nil || srv == nil {
		t.Fatal("listener / ensure callback not wired correctly")
	}
	_ = srv.Close()
	_ = ln.Close()
}

func TestBindFailureBlocksFirstTimeIssuance(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	mgr := newTestHTTP01Manager(t)
	called := false
	_, _, err = bindThenEnsure(mgr, occupied.Addr().String(), challengeHandler(), func(_ *acme.Manager, _ string) error {
		called = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP-01 listener") {
		t.Fatalf("expected an HTTP-01 bind error, got %v", err)
	}
	if called {
		t.Fatal("issuance must not start when the challenge port cannot be bound")
	}
}

func TestEnsureFailureReleasesChallengeListener(t *testing.T) {
	mgr := newTestHTTP01Manager(t)
	want := errors.New("obtain failed")
	var bound string
	ln, _, err := bindThenEnsure(mgr, "127.0.0.1:0", challengeHandler(), func(_ *acme.Manager, addr string) error {
		bound = addr
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected issuance error, got %v", err)
	}
	if ln != nil {
		t.Fatal("listener should be closed when issuance fails")
	}
	probe, err := net.Listen("tcp", bound)
	if err != nil {
		t.Fatalf("challenge listener was not released on failure: %v", err)
	}
	_ = probe.Close()
}

func TestHTTPSRedirectTarget(t *testing.T) {
	req := func(host string) *http.Request {
		return httptest.NewRequest(http.MethodGet, "http://"+host+"/path?x=1", nil)
	}
	cases := []struct {
		name      string
		publicURL string
		settings  acme.Settings
		req       *http.Request
		tlsAddr   string
		want      string
	}{
		{
			name:     "default 443 keeps no port",
			settings: acme.Settings{Domain: "panel.example.com"},
			req:      req("internal:80"),
			tlsAddr:  ":443",
			want:     "https://panel.example.com",
		},
		{
			name:     "custom TLS port is carried into redirect",
			settings: acme.Settings{Domain: "panel.example.com"},
			req:      req("internal:80"),
			tlsAddr:  ":10087",
			want:     "https://panel.example.com:10087",
		},
		{
			name:      "public URL wins for NAT / reverse proxy",
			publicURL: "https://panel.example.com:8443",
			settings:  acme.Settings{Domain: "panel.example.com"},
			req:       req("internal:80"),
			tlsAddr:   ":10087",
			want:      "https://panel.example.com:8443",
		},
		{
			name:      "public URL without scheme",
			publicURL: "panel.example.com:9443",
			settings:  acme.Settings{Domain: "panel.example.com"},
			req:       req("internal:80"),
			tlsAddr:   ":443",
			want:      "https://panel.example.com:9443",
		},
		{
			name:      "http public URL is upgraded",
			publicURL: "http://panel.example.com",
			settings:  acme.Settings{Domain: "panel.example.com"},
			req:       req("internal:80"),
			tlsAddr:   ":10087",
			want:      "https://panel.example.com",
		},
		{
			name:     "request host port is dropped",
			settings: acme.Settings{},
			req:      req("example.com:8080"),
			tlsAddr:  ":10087",
			want:     "https://example.com:10087",
		},
		{
			name:     "IPv6 domain is bracketed",
			settings: acme.Settings{Domain: "2001:db8::1"},
			req:      req("internal:80"),
			tlsAddr:  ":10087",
			want:     "https://[2001:db8::1]:10087",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := httpsRedirectTarget(tt.publicURL, tt.settings, tt.req, tt.tlsAddr); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEffectiveEntryURLRespectsTLSAndPublicURL(t *testing.T) {
	s := acme.Settings{Domain: "panel.example.com", ListenTLS: ":10087"}
	cases := []struct {
		name      string
		publicURL string
		webPath   string
		listen    string
		https     bool
		want      string
	}{
		{"custom TLS port", "", "/secret", ":10087", true, "https://panel.example.com:10087/secret/"},
		{"public URL wins", "https://panel.example.com:8443", "/secret", ":10087", true, "https://panel.example.com:8443/secret/"},
		{"plain HTTP fallback", "", "/secret", ":10086", false, "http://panel.example.com:10086/secret/"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := acme.EffectiveEntryURL(s, tt.publicURL, tt.webPath, tt.listen, 10086, tt.https)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
