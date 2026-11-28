package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

func checkTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestConnectionClientExportDoesNotGenerateCredentials(t *testing.T) {
	db := checkTestDB(t)
	l := models.Listener{Name: "no-user", PublicHost: "node.example.com", Protocol: "vless", Port: "12345", Enabled: true, Config: `{}`}
	if err := db.Create(&l).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, "", nil)
	if _, err := svc.connectionClientYAML(l); err == nil {
		t.Fatal("exported missing credentials")
	}
	var after models.Listener
	if err := db.First(&after, l.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Config != `{}` {
		t.Fatal("check generated credentials")
	}
	l.Config = `{"users":[{"username":"test","uuid":"6f2294b8-a47c-4c73-9bea-7aa4ebca6c50"}]}`
	if err := db.Save(&l).Error; err != nil {
		t.Fatal(err)
	}
	yaml, err := svc.connectionClientYAML(l)
	if err != nil || !strings.Contains(yaml, "6f2294b8-a47c-4c73-9bea-7aa4ebca6c50") {
		t.Fatalf("existing credentials: %v", err)
	}
	if err := db.First(&after, l.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Config != l.Config {
		t.Fatal("export changed existing configuration")
	}
}

func TestConnectionCheckRoutesAndStoppedCore(t *testing.T) {
	db := checkTestDB(t)
	l := models.Listener{Name: "test", Protocol: "vless", Port: "12345", Enabled: true, Config: `{}`}
	if err := db.Create(&l).Error; err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "mihomo.yaml")
	if err := os.WriteFile(path, []byte("listeners: []"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Mihomo.Config = path
	svc := NewService(db, path, mihomo.NewService(cfg))
	router := gin.New()
	NewHandler(svc, nil, db).RegisterClientRoutes(router.Group("/nodes"))
	for _, tc := range []struct {
		path string
		code int
	}{{"0", 400}, {"bad", 400}, {"999", 404}, {"1", 200}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/nodes/"+tc.path+"/connection-check", nil))
		if w.Code != tc.code {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
		if tc.code == 200 {
			var result mihomo.ListenerRuntime
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.ConnectionCheck == nil || result.ConnectionCheck.Reason != "core_stopped" {
				t.Fatalf("unexpected check: %+v", result)
			}
			polled := svc.mihomo.ListenerRuntime([]models.Listener{l}, false)[0]
			if polled.ConnectionCheck == nil || polled.ConnectionCheck.Reason != "core_stopped" {
				t.Fatal("polling lost the stopped-core check result")
			}
		}
	}
}

func TestConnectionClientExportRejectsInactiveBinding(t *testing.T) {
	db := checkTestDB(t)
	l := models.Listener{Name: "inactive-user", Protocol: "vless", Port: "12345", Enabled: true, Config: `{"users":[{"username":"legacy","uuid":"6f2294b8-a47c-4c73-9bea-7aa4ebca6c50"}]}`}
	if err := db.Create(&l).Error; err != nil {
		t.Fatal(err)
	}
	u := models.ProxyUser{Username: "disabled", UUID: "9f2294b8-a47c-4c73-9bea-7aa4ebca6c50"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&u).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ListenerUser{ListenerID: l.ID, ProxyUserID: u.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(db, "", nil).connectionClientYAML(l); err == nil {
		t.Fatal("export fell back to legacy credentials despite an inactive binding")
	}
}

func TestConnectionClientExportUsesAccessEndpoint(t *testing.T) {
	db := checkTestDB(t)
	svc := NewService(db, "", nil)
	previous := config.GlobalConfig
	config.GlobalConfig = nil
	t.Cleanup(func() { config.GlobalConfig = previous })
	l := models.Listener{Name: "test", Protocol: "vless", Port: "12345", BindAddress: "127.0.0.1", Enabled: true,
		Config: `{"users":[{"username":"test","uuid":"6f2294b8-a47c-4c73-9bea-7aa4ebca6c50"}]}`}
	if _, err := svc.connectionClientYAML(l); err == nil {
		t.Fatal("missing access host fell back to loopback")
	}
	for _, row := range []models.PanelSetting{
		{Key: protocol.KeyPublicHost, Value: "global.example.com"},
		{Key: protocol.KeyPublicPort, Value: "8443"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		host, port, wantHost string
		wantPort             int
	}{
		{"", "", "global.example.com", 8443},
		{"node.example.com", "9443", "node.example.com", 9443},
	} {
		l.PublicHost, l.PublicPort = tc.host, tc.port
		raw, err := svc.connectionClientYAML(l)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Proxies []struct {
				Server string
				Port   int
			}
		}
		if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.Proxies) != 1 || doc.Proxies[0].Server != tc.wantHost || doc.Proxies[0].Port != tc.wantPort {
			t.Fatalf("export did not use access endpoint: %+v", doc.Proxies)
		}
	}
}

func TestConnectionRouteCacheTracksCurrentClientSettings(t *testing.T) {
	db := checkTestDB(t)
	l := models.Listener{Name: "cache-test", Protocol: "vless", Port: "12345", PublicHost: "node.example.com", Enabled: true,
		Config: `{"users":[{"username":"test","uuid":"6f2294b8-a47c-4c73-9bea-7aa4ebca6c50"}]}`}
	if err := db.Create(&l).Error; err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("listeners: []"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Mihomo.Config = path
	svc := NewService(db, path, mihomo.NewService(cfg))
	router := gin.New()
	NewHandler(svc, nil, db).RegisterClientRoutes(router.Group("/nodes"))
	request := func(reuse bool) *mihomo.ConnectionCheck {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/nodes/%d/connection-check?reuse=%t", l.ID, reuse), nil))
		var result mihomo.ListenerRuntime
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.ConnectionCheck == nil {
			t.Fatal("invalid check response")
		}
		return result.ConnectionCheck
	}
	first := request(false)
	if !request(true).CheckedAt.Equal(first.CheckedAt) {
		t.Fatal("unchanged client configuration was not reused")
	}
	for _, update := range []func(){
		func() {
			if err := db.Model(&l).Update("public_port", "8443").Error; err != nil {
				t.Fatal(err)
			}
		},
		func() {
			if err := db.Create(&models.PanelSetting{Key: protocol.KeySNI, Value: "changed.example.com"}).Error; err != nil {
				t.Fatal(err)
			}
		},
		func() {
			if err := db.Model(&l).Update("config", `{"users":[{"username":"test","uuid":"9f2294b8-a47c-4c73-9bea-7aa4ebca6c50"}]}`).Error; err != nil {
				t.Fatal(err)
			}
		},
	} {
		update()
		// Applied YAML and process did not change. Even lightweight polling
		// must discard a result for an outdated public endpoint or credential.
		if svc.mihomo.ListenerRuntime([]models.Listener{l}, false)[0].ConnectionCheck != nil {
			t.Fatal("polling retained an obsolete client configuration result")
		}
		next := request(true)
		if next.CheckedAt.Equal(first.CheckedAt) {
			t.Fatal("edited client configuration reused the old result")
		}
		first = next
	}
	if request(false).CheckedAt.Equal(first.CheckedAt) {
		t.Fatal("manual check did not bypass cache")
	}
}
