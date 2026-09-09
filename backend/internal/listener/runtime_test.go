package listener

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
)

func TestRuntimeRoutesObserveWithoutApplyingConfiguration(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	listeners := []models.Listener{{Name: "enabled", Protocol: "vless", Port: "12000", Enabled: true}, {Name: "disabled", Protocol: "vless", Port: "12001", Enabled: false}}
	if err := db.Create(&listeners).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewHandler(NewService(db, filepath.Join(t.TempDir(), "unused.yaml"), nil)).RegisterRoutes(router.Group("/nodes"))
	for _, tc := range []struct {
		method, path string
		code         int
	}{
		{http.MethodGet, "/nodes/runtime-status", 200},
		{http.MethodPost, "/nodes/1/check", 200},
		{http.MethodPost, "/nodes/2/check", 200},
		{http.MethodPost, "/nodes/0/check", 400},
		{http.MethodPost, "/nodes/999/check", 404},
		{http.MethodPost, "/nodes/nope/check", 400},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
		if recorder.Code != tc.code {
			t.Fatalf("%s: got %d: %s", tc.path, recorder.Code, recorder.Body.String())
		}
		if tc.path == "/nodes/runtime-status" {
			var statuses []mihomo.ListenerRuntime
			if err := json.Unmarshal(recorder.Body.Bytes(), &statuses); err != nil {
				t.Fatal(err)
			}
			if len(statuses) != 2 || statuses[0].State != "disabled" || statuses[1].State != "unknown" {
				t.Fatalf("unexpected states: %+v", statuses)
			}
		}
	}
	var after []models.Listener
	if err := db.Order("id").Find(&after).Error; err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || !after[0].Enabled || after[1].Enabled || after[0].Port != "12000" {
		t.Fatalf("read-only checks changed listeners: %+v", after)
	}
}

type recordingRuntimeInspector struct{ details []bool }

func (r *recordingRuntimeInspector) ApplyConfig(string) error {
	panic("read-only check applied configuration")
}
func (r *recordingRuntimeInspector) ListenerRuntime(listeners []models.Listener, details bool) []mihomo.ListenerRuntime {
	r.details = append(r.details, details)
	results := make([]mihomo.ListenerRuntime, len(listeners))
	for i, l := range listeners {
		results[i] = mihomo.ListenerRuntime{ID: l.ID, State: "listening", Reason: "sockets_bound", Endpoints: []mihomo.RuntimeEndpoint{}}
		if details {
			results[i].Endpoints = []mihomo.RuntimeEndpoint{{Network: "tcp", Address: "127.0.0.1", Port: 12000, Bound: true}}
		}
	}
	return results
}

func TestRuntimeSummaryIsOptInAndManualChecksIncludeDetails(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	l := models.Listener{Name: "enabled", Protocol: "vless", Port: "12000", Enabled: true}
	if err := db.Create(&l).Error; err != nil {
		t.Fatal(err)
	}
	inspector := &recordingRuntimeInspector{}
	router := gin.New()
	NewHandler(NewService(db, "unused.yaml", inspector)).RegisterRoutes(router.Group("/nodes"))
	for _, tc := range []struct {
		method, path string
		details      bool
	}{
		{http.MethodGet, "/nodes/runtime-status", true},
		{http.MethodGet, "/nodes/runtime-status?summary=true", false},
		{http.MethodPost, "/nodes/1/check?summary=true", true},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s: %d", tc.path, response.Code)
		}
		if inspector.details[len(inspector.details)-1] != tc.details {
			t.Fatalf("%s: incorrect detail mode", tc.path)
		}
		var statuses []mihomo.ListenerRuntime
		if tc.method == http.MethodPost {
			var status mihomo.ListenerRuntime
			if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			statuses = append(statuses, status)
		} else if err := json.Unmarshal(response.Body.Bytes(), &statuses); err != nil {
			t.Fatal(err)
		}
		if len(statuses) != 1 || statuses[0].State != "listening" || (len(statuses[0].Endpoints) > 0) != tc.details {
			t.Fatalf("%s: invalid payload %s", tc.path, response.Body.String())
		}
	}
}
