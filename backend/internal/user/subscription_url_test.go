package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
)

// The Share page prints the URL returned by GET /users/:id/subscription. It must
// follow PUBLIC_URL / sub_path like every other subscription link (export-links,
// subscription_singbox, …) instead of the host the admin is browsing from:
// behind a reverse proxy those differ, and a link pointing at the admin host is
// unreachable for clients — which looks like "the subscription is empty".
func TestGetSubscriptionHonoursPublicURLAndSubPath(t *testing.T) {
	db := testDBSub(t)
	svc := NewService(db)
	u, err := svc.Create(CreateInput{Username: "bob", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}

	prev := config.GlobalConfig
	config.GlobalConfig = &config.Config{}
	config.GlobalConfig.Server.PublicURL = "https://sub.example.com"
	config.GlobalConfig.Server.SubPath = "/s"
	defer func() { config.GlobalConfig = prev }()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api/v1/users"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+strconv.FormatUint(uint64(u.ID), 10)+"/subscription", nil)
	req.Host = "admin.internal:8080"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := "https://sub.example.com/s/" + body.Token
	if body.URL != want {
		t.Fatalf("subscription url = %q, want %q", body.URL, want)
	}
}
