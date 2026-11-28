package cluster

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFederationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.RemoteServer{},
		&models.RemoteNodeMirror{},
		&models.ProxyUserRemoteNode{},
		&models.ProxyUser{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// Deleting a remote must take its mirrors and every binding pointing at them,
// otherwise the mirrors keep feeding subscriptions nobody manages any more.
func TestDeleteRemoteCascadesMirrorsAndBindings(t *testing.T) {
	db := setupFederationDB(t)
	svc := NewService(db)

	remote := models.RemoteServer{Name: "edge", BaseURL: "https://edge.example.com", Enabled: true}
	if err := db.Create(&remote).Error; err != nil {
		t.Fatalf("create remote: %v", err)
	}
	mirror := models.RemoteNodeMirror{
		RemoteServerID: remote.ID, RemoteNodeID: 7, Name: "edge-node",
		ShareURI: "trojan://x@edge.example.com:443#edge-node", Enabled: true,
	}
	if err := db.Create(&mirror).Error; err != nil {
		t.Fatalf("create mirror: %v", err)
	}
	user := models.ProxyUser{Username: "u1"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	bind := models.ProxyUserRemoteNode{ProxyUserID: user.ID, RemoteNodeMirrorID: mirror.ID}
	if err := db.Create(&bind).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}

	if err := svc.Delete(remote.ID); err != nil {
		t.Fatalf("delete remote: %v", err)
	}

	var mirrorCount, bindCount, remoteCount int64
	db.Model(&models.RemoteNodeMirror{}).Count(&mirrorCount)
	db.Model(&models.ProxyUserRemoteNode{}).Count(&bindCount)
	db.Model(&models.RemoteServer{}).Count(&remoteCount)
	if mirrorCount != 0 || bindCount != 0 || remoteCount != 0 {
		t.Fatalf("cascade left rows behind: mirrors=%d bindings=%d remotes=%d",
			mirrorCount, bindCount, remoteCount)
	}
}

// A transient failure while fetching a node's share export must not wipe the
// cached link: otherwise one bad sync silently drops the node from every
// subscription that bound it.
func TestSyncRemoteNodesKeepsCachedExportOnFetchFailure(t *testing.T) {
	t.Setenv("THREE_M_UI_ALLOW_PRIVATE", "1")
	t.Setenv("THREE_M_UI_CLUSTER_ALLOW_PRIVATE", "1")

	var uriCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nodes":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "edge-node", "protocol": "trojan", "port": "443",
					"public_host": "edge.example.com", "enabled": true},
			})
		case "/api/v1/nodes/1/uri":
			uriCalls++
			if uriCalls == 1 {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"name": "edge-node", "protocol": "trojan",
					"uri":         "trojan://good@edge.example.com:443#edge-node",
					"client_yaml": "proxies:\n  - name: edge-node\n    type: trojan\n",
				})
				return
			}
			// Second sync: the remote fails to export this node.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db := setupFederationDB(t)
	svc := NewService(db)
	remote := models.RemoteServer{Name: "edge", BaseURL: srv.URL, Enabled: true}
	if err := db.Create(&remote).Error; err != nil {
		t.Fatalf("create remote: %v", err)
	}

	first, err := svc.SyncRemoteNodes(remote.ID)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if len(first) != 1 || first[0].ShareURI != "trojan://good@edge.example.com:443#edge-node" {
		t.Fatalf("first sync did not capture the export: %+v", first)
	}

	second, err := svc.SyncRemoteNodes(remote.ID)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected 1 mirror, got %d", len(second))
	}
	got := second[0]
	if got.ShareURI != "trojan://good@edge.example.com:443#edge-node" {
		t.Fatalf("cached share_uri was wiped by a failed export: %q", got.ShareURI)
	}
	if got.ClientYAML == "" {
		t.Fatal("cached client_yaml was wiped by a failed export")
	}
	if got.LastError == "" {
		t.Fatal("expected the failed export to still be recorded in last_error")
	}
}
