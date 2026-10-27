package router_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kazeyukiro/3m-ui/backend/internal/auth"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	"github.com/kazeyukiro/3m-ui/backend/internal/router"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"github.com/kazeyukiro/3m-ui/backend/internal/traffic"
)

func TestHealthAndMihomoAPIs(t *testing.T) {
	_ = os.RemoveAll("/tmp/3m-ui-router-test")

	cfg := &config.Config{}
	cfg.Server.Mode = "debug"
	cfg.Database.Path = "/tmp/3m-ui-router-test/db.sqlite"
	cfg.Mihomo.Binary = "/tmp/dummy-nonexistent"
	cfg.Mihomo.Config = "/tmp/3m-ui-router-test/config.yaml"
	cfg.JWT.Secret = "super-secret-token-key-for-testing-purposes"
	cfg.Security.CORSOrigins = nil

	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	mihomoSvc := mihomo.NewService(cfg)
	systemSvc := system.NewService()
	trafficSvc := traffic.NewService()

	r := router.SetupRouterWithDeps(router.Deps{
		DB:      db,
		Config:  cfg,
		Mihomo:  mihomoSvc,
		System:  systemSvc,
		Traffic: trafficSvc,
	})

	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected access-token management endpoint to require auth, got %d", w.Code)
		}
	}

	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/client/sub/does-not-exist", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected missing public subscription to return 404, got %d", w.Code)
		}
	}

	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/health", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["status"] != "ok" {
			t.Fatalf("expected health status 'ok', got '%v'", resp)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("expected no CORS allow-origin when no origins are configured, got %q", got)
		}
	}

	{
		_, username, password, err := auth.EnsureAdmin(db, cfg.Database.Path)
		if err != nil {
			t.Fatalf("ensure admin: %v", err)
		}
		result, err := auth.Login(db, cfg.JWT.Secret, auth.LoginInput{Username: username, Password: password})
		if err != nil {
			t.Fatalf("login: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+result.Token)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK && w.Code != http.StatusForbidden {
			t.Fatalf("expected status 200 or 403, got %d body=%s", w.Code, w.Body.String())
		}

		if w.Code == http.StatusOK {
			var resp map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			for _, key := range []string{"mihomo", "system", "listeners"} {
				if _, exists := resp[key]; !exists {
					t.Fatalf("expected aggregator to contain %q", key)
				}
			}
		}
	}

	_ = os.RemoveAll("/tmp/3m-ui-router-test")
}

func TestCORSConfiguredOrigin(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Mode = "debug"
	cfg.Security.CORSOrigins = []string{"https://panel.example.com"}
	cfg.JWT.Secret = "test-secret"

	r := router.SetupRouter(cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/v1/health", nil)
	req.Header.Set("Origin", "https://panel.example.com")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://panel.example.com" {
		t.Fatalf("expected reflected origin, got %q", got)
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/health", nil)
	req2.Header.Set("Origin", "https://evil.example.com")
	r.ServeHTTP(w2, req2)
	if got := w2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow-origin for disallowed origin, got %q", got)
	}
}

func TestFrontendCompressionAndCaching(t *testing.T) {
	script := []byte(strings.Repeat("console.log('frontend asset');\n", 4096))
	html := []byte("<!doctype html>" + strings.Repeat("<p>Application</p>", 100))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Header("Vary", "Origin"); c.Next() })
	r.GET("/api/v1/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	router.MountFrontend(r, fstest.MapFS{
		"web/dist/index.html":                &fstest.MapFile{Data: html},
		"web/dist/assets/index-Abcd123_.js":  &fstest.MapFile{Data: script},
		"web/dist/assets/style-Abcd123_.css": &fstest.MapFile{Data: []byte("body{color:red}")},
		"web/dist/logo.svg":                  &fstest.MapFile{Data: []byte("<svg></svg>")},
		"web/dist/manifest.webmanifest":      &fstest.MapFile{Data: []byte(`{"name":"3m-ui"}`)},
	}, "")
	request := func(method, path string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out
	}
	const url = "/assets/index-Abcd123_.js"
	plain := request("GET", url, nil)
	if plain.Code != 200 || !bytes.Equal(plain.Body.Bytes(), script) || plain.Header().Get("Content-Encoding") != "" {
		t.Fatalf("identity response: %d %v", plain.Code, plain.Header())
	}
	if plain.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatal(plain.Header())
	}
	compressed := request("GET", url, map[string]string{"Accept-Encoding": "gzip"})
	if compressed.Code != 200 || compressed.Header().Get("Content-Encoding") != "gzip" || compressed.Body.Len() >= len(script)/4 {
		t.Fatalf("compression: %d %v", compressed.Code, compressed.Header())
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(decoded, script) {
		t.Fatal("compressed content differs")
	}
	if compressed.Header().Get("Content-Type") != plain.Header().Get("Content-Type") {
		t.Fatal("compressed MIME type changed")
	}
	vary := strings.Join(compressed.Header().Values("Vary"), ",")
	if !strings.Contains(vary, "Origin") || !strings.Contains(vary, "Accept-Encoding") {
		t.Fatal(vary)
	}
	etag, gzipETag := plain.Header().Get("ETag"), compressed.Header().Get("ETag")
	if etag == "" || gzipETag == "" || etag == gzipETag {
		t.Fatal("missing representation-specific validators")
	}
	for _, tc := range []struct {
		encoding, tag string
		status        int
	}{
		{"identity", etag, 304}, {"gzip", gzipETag, 304}, {"gzip", etag, 200}, {"identity", gzipETag, 200},
	} {
		result := request("GET", url, map[string]string{"Accept-Encoding": tc.encoding, "If-None-Match": tc.tag})
		if result.Code != tc.status || (tc.status == 304 && result.Body.Len() != 0) {
			t.Fatalf("conditional %v: %d", tc, result.Code)
		}
	}
	for _, tc := range []struct {
		encoding   string
		compressed bool
	}{
		{"gzip", true}, {"br, gzip;q=0.5", true}, {"GZIP", true}, {"*", true},
		{"gzip;q=0", false}, {"*;q=1, gzip;q=0", false}, {"gzip;q=0, *;q=1", false},
		{"br", false}, {"gzip;q=invalid", false}, {"gzip;q=1.5", false},
	} {
		result := request("GET", url, map[string]string{"Accept-Encoding": tc.encoding})
		if (result.Header().Get("Content-Encoding") == "gzip") != tc.compressed {
			t.Fatalf("encoding %q: %v", tc.encoding, result.Header())
		}
	}
	head := request("HEAD", url, map[string]string{"Accept-Encoding": "gzip"})
	if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") != compressed.Header().Get("Content-Length") {
		t.Fatalf("HEAD: %v", head)
	}
	partial := request("GET", url, map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-9", "If-Range": etag})
	if partial.Code != 206 || partial.Header().Get("Content-Encoding") != "" || !bytes.Equal(partial.Body.Bytes(), script[:10]) {
		t.Fatalf("range: %v", partial)
	}
	for _, path := range []string{"/", "/index.html", "/core"} {
		result := request("GET", path, nil)
		if result.Code != 200 || !bytes.Equal(result.Body.Bytes(), html) || result.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("HTML %s: %v", path, result)
		}
		cached := request("GET", path, map[string]string{"If-None-Match": result.Header().Get("ETag")})
		if cached.Code != 304 {
			t.Fatalf("HTML revalidation %s: %d", path, cached.Code)
		}
	}
	for _, path := range []string{"/assets/missing-12345678.js", "/assets", "/api/missing"} {
		result := request("GET", path, nil)
		if result.Code != 404 || bytes.Contains(result.Body.Bytes(), html) || strings.Contains(result.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("missing %s: %v", path, result)
		}
	}
	for _, path := range []string{"/logo.svg", "/manifest.webmanifest"} {
		result := request("GET", path, nil)
		if result.Code != 200 || result.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("unhashed %s: %v", path, result)
		}
	}
	health := request("GET", "/api/v1/health", map[string]string{"Accept-Encoding": "gzip"})
	if health.Code != 200 || health.Header().Get("Cache-Control") != "" || health.Header().Get("Content-Encoding") != "" {
		t.Fatal("static policy leaked onto API")
	}
}

// Core update routes use the same administrator-only middleware as lifecycle
// controls. Rejected callers must not reach an updater or external releases API.
func TestCoreUpdateRoutesRequireAuthentication(t *testing.T) {
	cfg := &config.Config{}
	r := router.SetupRouter(cfg)
	for _, item := range []struct{ method, path string }{
		{"GET", "/api/v1/mihomo/releases"}, {"GET", "/api/v1/mihomo/update"},
		{"POST", "/api/v1/mihomo/update"}, {"POST", "/api/v1/mihomo/update/rollback"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(item.method, item.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: %d", item.method, item.path, w.Code)
		}
	}
}

// The authoritative CSP (matching the index.html meta tag) must be sent on
// every response, SPA and API alike — the header is what actually constrains
// documents; JSON responses only carry it harmlessly.
func TestSecurityHeadersCSP(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Mode = "debug"
	cfg.Database.Path = "/tmp/3m-ui-router-test-csp/db.sqlite"
	cfg.JWT.Secret = "test-secret"

	r := router.SetupRouter(cfg)

	for _, path := range []string{"/api/v1/health", "/login"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		r.ServeHTTP(w, req)
		csp := w.Header().Get("Content-Security-Policy")
		if csp == "" {
			t.Fatalf("%s: missing Content-Security-Policy", path)
		}
		for _, want := range []string{
			"default-src 'self'",
			"script-src 'self';",
			"style-src 'self' 'unsafe-inline'",
			"connect-src 'self'",
			"frame-ancestors 'none'",
			"object-src 'none'",
		} {
			if !strings.Contains(csp, want) {
				t.Fatalf("%s: CSP %q missing %q", path, csp, want)
			}
		}
		// script-src must never widen to inline execution.
		if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
			t.Fatalf("%s: script-src must not allow inline: %q", path, csp)
		}
		// img-src must be same-origin + data: only. A SPA source audit
		// found no remote image loading, so https: is omitted to close the
		// image-exfiltration gap surfaced during real-browser testing.
		if !strings.Contains(csp, "img-src 'self' data:") {
			t.Fatalf("%s: CSP %q missing img-src 'self' data:", path, csp)
		}
		if strings.Contains(csp, "img-src 'self' data: https:") || strings.Contains(csp, "img-src 'self' https:") {
			t.Fatalf("%s: img-src must not allow https:: %q", path, csp)
		}
		if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Fatalf("%s: X-Frame-Options = %q", path, got)
		}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("%s: X-Content-Type-Options = %q", path, got)
	}
	}
}

// TestWebPathHidesPanel verifies the random base-path feature end to end:
// the panel UI/API are only reachable under /secret/, the bare domain returns
// 404, health probes stay public, and the active prefix is injected into the
// served index.html as a <meta> tag (NOT a <script>, which the page CSP blocks).
func TestWebPathHidesPanel(t *testing.T) {
	_ = os.RemoveAll("/tmp/3m-ui-router-webpath-test")

	cfg := &config.Config{}
	cfg.Server.Mode = "debug"
	cfg.Server.WebPath = "/secret"
	cfg.Database.Path = "/tmp/3m-ui-router-webpath-test/db.sqlite"
	cfg.Mihomo.Binary = "/tmp/dummy-nonexistent"
	cfg.Mihomo.Config = "/tmp/3m-ui-router-webpath-test/config.yaml"
	cfg.JWT.Secret = "super-secret-token-key-for-testing-purposes"
	cfg.Security.CORSOrigins = nil

	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer os.RemoveAll("/tmp/3m-ui-router-webpath-test")

	engine := router.SetupRouterWithDeps(router.Deps{
		DB:      db,
		Config:  cfg,
		Mihomo:  mihomo.NewService(cfg),
		System:  system.NewService(),
		Traffic: traffic.NewService(),
	})
	router.MountFrontend(engine, fstest.MapFS{
		"web/dist/index.html":      &fstest.MapFile{Data: []byte("<!doctype html><html><head><title>3m-ui</title></head><body></body></html>")},
		"web/dist/assets/index.js": &fstest.MapFile{Data: []byte("x")},
	}, "/secret")
	// Enforce the base path exactly like app.Run does: strip before matching.
	r := router.WebPathHandler("/secret", "", engine)

	cases := []struct {
		path      string
		wantCode  int
		wantMeta bool
	}{
		{"/secret/", 200, true},
		{"/secret", http.StatusMovedPermanently, false},
		{"/", 404, false},
		{"/api/v1/health", 200, false},
		{"/api/v1/auth/login", 404, false},
		{"/secret/api/v1/health", 200, false},
		{"/secret/assets/index.js", 200, false},
		{"/assets/index.js", 404, false},
		// SPA deep links: refreshing /secret/nodes in a browser must hand back
		// the shell — carrying the injected meta — while the same link on the
		// bare domain stays hidden.
		{"/secret/nodes", 200, true},
		{"/secret/settings", 200, true},
		{"/nodes", 404, false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if w.Code != tc.wantCode {
				t.Fatalf("path %q: got %d, want %d", tc.path, w.Code, tc.wantCode)
			}
			if tc.wantMeta {
				if !strings.Contains(w.Body.String(), `<meta name="x-web-path" content="/secret">`) {
					t.Fatalf("path %q: index missing injected meta:\n%s", tc.path, w.Body.String())
				}
				if strings.Contains(w.Body.String(), "window.__WEB_PATH__") {
					t.Fatalf("path %q: must not inject a <script> (blocked by page CSP)", tc.path)
				}
			}
		})
	}
}
