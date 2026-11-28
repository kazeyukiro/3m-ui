package realityscan

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPSProbe(t *testing.T) {
	for _, tc := range []struct {
		name       string
		h2         bool
		maxTLS     uint16
		rootStatus int
		cfHeader   bool
		body       string
		reason     string
	}{
		{name: "valid", h2: true, rootStatus: 200},
		{name: "redirect", h2: true, rootStatus: 302, reason: "http_redirect"},
		{name: "http2 required", rootStatus: 200, reason: "h2_required"},
		{name: "tls13 required", h2: true, maxTLS: tls.VersionTLS12, rootStatus: 200, reason: "tls13_required"},
		{name: "cloudflare header", h2: true, rootStatus: 200, cfHeader: true, reason: "cloudflare_detected"},
		{name: "cloudflare trace", h2: true, rootStatus: 200, body: "fl=abc\nh=test\nip=1.2.3.4\nts=1234\ncolo=SJC\n", reason: "cloudflare_detected"},
		{name: "ordinary trace path response", h2: true, rootStatus: 200, body: "<html>Page not found</html>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.cfHeader {
					w.Header().Set("CF-Ray", "test")
				}
				if r.URL.Path == "/" {
					w.WriteHeader(tc.rootStatus)
					return
				}
				w.Write([]byte(tc.body))
			}))
			server.EnableHTTP2 = tc.h2
			server.TLS = &tls.Config{MaxVersion: tc.maxTLS, CurvePreferences: []tls.CurveID{tls.X25519}}
			server.StartTLS()
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error {
				t.Error("unexpected redirect follow")
				return http.ErrUseLastResponse
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got := probeHTTPS(ctx, client, server.URL, "example.com")
			if got.Reason != tc.reason || got.Eligible != (tc.reason == "") {
				t.Fatalf("got %+v, want reason %q", got, tc.reason)
			}
		})
	}
}

func TestProbeRejectsUntrustedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	got := probeHTTPS(context.Background(), &http.Client{Timeout: time.Second}, server.URL, "example.com")
	if got.Eligible || got.Reason != "connection_or_certificate_failed" {
		t.Fatalf("accepted untrusted certificate: %+v", got)
	}
}

func TestSharedDeliverySignals(t *testing.T) {
	for _, name := range []string{"edge.fastly.net.", "tenant.cloudfront.net", "www.site.cdn.cloudflare.net", "site.edgekey.net.", "wb.esa.cdnmix.net."} {
		if !sharedDeliveryCNAME(name) {
			t.Errorf("missed delivery alias %s", name)
		}
	}
	for _, name := range []string{"fastly.net.example.com", "notfastly.net", "cdn.example.com", "www.example.com."} {
		if sharedDeliveryCNAME(name) {
			t.Errorf("false alias match %s", name)
		}
	}
	for _, tc := range []struct {
		headers map[string]string
		want    string
	}{
		{map[string]string{"X-Amz-Cf-Id": "example"}, "cloudfront"},
		{map[string]string{"Via": "1.1 edge.cloudfront.net (CloudFront)"}, "cloudfront"},
		{map[string]string{"X-Served-By": "cache-edge", "X-Timer": "S123"}, "fastly_style_cache"},
		{map[string]string{"Server": "AkamaiGHost"}, "akamai"},
		{map[string]string{"Cf-Ray": "example"}, "cloudflare"},
		{map[string]string{"Server": "nginx", "X-Cache": "HIT"}, ""},
		{map[string]string{"X-Served-By": "origin"}, ""},
	} {
		headers := http.Header{}
		for k, v := range tc.headers {
			headers.Set(k, v)
		}
		if got := sharedDeliveryHeaders(headers); got != tc.want {
			t.Errorf("headers %v: got %q, want %q", tc.headers, got, tc.want)
		}
	}
}

func TestCloudFrontHeadersExcludeOtherwiseCompatibleTarget(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Amz-Cf-Id", "example")
		w.WriteHeader(http.StatusOK)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	r := probeHTTPS(context.Background(), server.Client(), server.URL, "example.com")
	if r.Eligible || r.Reason != "shared_delivery_headers" || r.Evidence.DeliverySignal != "cloudfront" {
		t.Fatalf("unexpected result: %+v", r)
	}
	if r.Evidence.HTTPStatus != 200 || !r.Evidence.CertificateValid {
		t.Fatalf("lost review evidence: %+v", r)
	}
}
