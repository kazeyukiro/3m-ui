package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	"github.com/kazeyukiro/3m-ui/backend/internal/router"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"github.com/kazeyukiro/3m-ui/backend/internal/traffic"
)

// The external subconverter integration has been removed: every supported
// target is rendered natively by the subscription handler, and an unknown
// target must be rejected with 400 instead of being forwarded to an external
// conversion service (which used to answer on 127.0.0.1:25500).
func TestSubscriptionRejectsUnsupportedTargetAfterSubconverterRemoval(t *testing.T) {
	dir := "/tmp/3m-ui-sub-target-test"
	_ = os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	cfg := &config.Config{}
	cfg.Server.Mode = "debug"
	cfg.Database.Path = dir + "/db.sqlite"
	cfg.Mihomo.Binary = "/tmp/dummy-nonexistent"
	cfg.Mihomo.Config = dir + "/config.yaml"
	cfg.JWT.Secret = "test-secret-for-sub-target"

	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}

	listener := models.Listener{
		Name:        "sub-target-listener",
		Protocol:    "vless",
		BindAddress: "0.0.0.0",
		Port:        "10086",
		Enabled:     true,
		Config:      `{"users":[{"username":"u1","uuid":"11111111-1111-1111-1111-111111111111"}]}`,
	}
	if err := db.Create(&listener).Error; err != nil {
		t.Fatalf("create listener: %v", err)
	}
	token := models.AccessToken{
		Name:       "sub-target token",
		Token:      "subtarget-token-0001",
		Enabled:    true,
		ListenerID: listener.ID,
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("create access token: %v", err)
	}

	r := router.SetupRouterWithDeps(router.Deps{
		DB:      db,
		Config:  cfg,
		Mihomo:  mihomo.NewService(cfg),
		System:  system.NewService(),
		Traffic: traffic.NewService(),
	})

	// Unknown target → 400 with an explicit error, no external service call.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/client/sub/"+token.Token+"?target=stash-clashr", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown target: expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); !strings.Contains(body, "unsupported subscription target") {
		t.Fatalf("unknown target: expected explicit error, got %s", body)
	}

	// Native targets keep rendering locally: clash/meta serve mihomo YAML.
	for _, target := range []string{"", "clash"} {
		w2 := httptest.NewRecorder()
		url := "/api/v1/client/sub/" + token.Token
		if target != "" {
			url += "?target=" + target
		}
		req2, _ := http.NewRequest("GET", url, nil)
		r.ServeHTTP(w2, req2)
		if w2.Code != http.StatusOK {
			t.Fatalf("target %q: expected 200, got %d body=%s", target, w2.Code, w2.Body.String())
		}
		if ct := w2.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/yaml") {
			t.Fatalf("target %q: expected yaml content type, got %q", target, ct)
		}
	}
}

// Under a random panel base path (the "hidden entry" from issue #103), the
// subscription HTML page — and every link / QR code / app deep link it renders —
// must embed the secret prefix. Otherwise a subscriber who opens the page via
// the hidden URL gets bare-domain links that 404 behind WebPathHandler.
func TestSubpageDeepLinkUnderWebPath(t *testing.T) {
	dir := "/tmp/3m-ui-sub-deeplink-test"
	_ = os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	cfg := &config.Config{}
	cfg.Server.Mode = "debug"
	cfg.Server.WebPath = "/secret"
	cfg.Database.Path = dir + "/db.sqlite"
	cfg.Mihomo.Binary = "/tmp/dummy-nonexistent"
	cfg.Mihomo.Config = dir + "/config.yaml"
	cfg.JWT.Secret = "test-secret-deeplink"

	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	pu := models.ProxyUser{
		Username:     "testuser",
		Enabled:      true,
		SubToken:     "deeplink-tok-0001",
		TrafficUsed:  42 * 1024 * 1024 * 1024,
		TrafficLimit: 200 * 1024 * 1024 * 1024,
	}
	if err := db.Create(&pu).Error; err != nil {
		t.Fatalf("create proxy user: %v", err)
	}

	r := router.SetupRouterWithDeps(router.Deps{
		DB:      db,
		Config:  cfg,
		Mihomo:  mihomo.NewService(cfg),
		System:  system.NewService(),
		Traffic: traffic.NewService(),
	})
	// Mirror how app.go wraps the engine with the panel base path.
	handler := router.WebPathHandler("/secret", "", r)

	// 1. Open the page through the hidden entry → 200 and links embed /secret.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/secret/api/v1/client/sub/deeplink-tok-0001?html=1", nil)
	req.Host = "panel.example.com"
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("html under web path: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "/secret/api/v1/client/sub/deeplink-tok-0001") {
		cut := body
		if len(cut) > 1500 {
			cut = cut[:1500]
		}
		t.Fatalf("subscription page under web path must embed secret prefix in links/QR; missing /secret/...:\n%s", cut)
	}
	if !strings.Contains(body, "testuser") {
		t.Errorf("page missing username")
	}
	// The clash:// / sing-box:// deep links must also carry the prefix.
	if !strings.Contains(body, "clash://install-config?url=") ||
		!strings.Contains(body, "%2Fsecret%2Fapi%2Fv1%2Fclient%2Fsub%2Fdeeplink-tok-0001") {
		t.Errorf("clash deep link missing secret prefix")
	}

	// 2. Bare /api/v1/client/ path (public allowlist) still reaches the page
	// and — having no secret — must NOT embed the prefix.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/client/sub/deeplink-tok-0001?html=1", nil)
	req2.Host = "panel.example.com"
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("html bare path: expected 200 (public allowlist), got %d", w2.Code)
	}
	if strings.Contains(w2.Body.String(), "/secret/api/v1/client/sub/deeplink-tok-0001") {
		t.Errorf("bare-path page must not embed secret prefix")
	}
}

// TestSubscriptionSingboxStyleByUserAgent pins the end-to-end behaviour: the
// request's User-Agent decides which sing-box document shape comes back.
//
// The two shapes are a compatibility fork, not a preference. sing-box 1.12/1.13
// abort on `http_clients` ("json: unknown field"), while 1.14 deprecated the
// implicit default HTTP client that rule-sets used and removes it in 1.16.0.
// SFA advertises its core version as "SFA (sing-box 1.14.4; language zh_CN)".
func TestSubscriptionSingboxStyleByUserAgent(t *testing.T) {
	dir := "/tmp/3m-ui-sb-style-test"
	_ = os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	cfg := &config.Config{}
	cfg.Server.Mode = "debug"
	cfg.Database.Path = dir + "/db.sqlite"
	cfg.Mihomo.Binary = "/tmp/dummy-nonexistent"
	cfg.Mihomo.Config = dir + "/config.yaml"
	cfg.JWT.Secret = "test-secret-sb-style"

	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	listener := models.Listener{
		Name:        "sb-style-listener",
		Protocol:    "vless",
		BindAddress: "0.0.0.0",
		Port:        "10087",
		PublicHost:  "node.example.com",
		PublicPort:  "443",
		Enabled:     true,
		Config:      `{"users":[{"username":"sbuser","uuid":"11111111-1111-1111-1111-111111111111"}]}`,
	}
	if err := db.Create(&listener).Error; err != nil {
		t.Fatalf("create listener: %v", err)
	}
	pu := models.ProxyUser{
		Username: "sbuser",
		Enabled:  true,
		SubToken: "sb-style-token-0001",
		UUID:     "11111111-1111-1111-1111-111111111111",
	}
	if err := db.Create(&pu).Error; err != nil {
		t.Fatalf("create proxy user: %v", err)
	}
	if err := db.Create(&models.ListenerUser{ProxyUserID: pu.ID, ListenerID: listener.ID}).Error; err != nil {
		t.Fatalf("bind user to listener: %v", err)
	}

	r := router.SetupRouterWithDeps(router.Deps{
		DB:      db,
		Config:  cfg,
		Mihomo:  mihomo.NewService(cfg),
		Traffic: traffic.NewService(),
		System:  system.NewService(),
	})

	cases := []struct {
		name string
		ua   string
		url  string
		want bool
	}{
		{"SFA 1.13 keeps legacy", "SFA (sing-box 1.13.4; language zh_CN)", "?target=singbox", false},
		{"SFA 1.14 gets modern", "SFA (sing-box 1.14.4; language zh_CN)", "?target=singbox", true},
		{"SFI 1.15 gets modern", "SFI/1.15.0 (sing-box 1.15.0)", "?target=singbox", true},
		{"versionless UA keeps legacy", "SFA", "?target=singbox", false},
		{"query override wins", "SFA (sing-box 1.13.4; language zh_CN)", "?target=singbox&sbver=1.14", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/client/sub/"+pu.SubToken+tc.url, nil)
			req.Header.Set("User-Agent", tc.ua)
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
			}
			var doc map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
				t.Fatalf("response is not JSON: %v\n%s", err, w.Body.String())
			}
			_, hasClients := doc["http_clients"]
			if hasClients != tc.want {
				route, _ := doc["route"].(map[string]interface{})
				t.Fatalf("http_clients present = %v, want %v (route.default_http_client=%v)",
					hasClients, tc.want, route["default_http_client"])
			}
		})
	}
}
