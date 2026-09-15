package listener

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
)

// Listener mutations return after the DB write; Mihomo ApplyConfig is debounced.
// Activation failure must not undo the panel record (same contract as user create).
func TestListenerMutationsKeepDatabaseWhenActivationFailsAsync(t *testing.T) {
	for _, operation := range []string{"create", "update", "delete"} {
		t.Run(operation, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(mihomo.AllowBinaryPathPrefixForTesting(dir))
			binary := filepath.Join(dir, "fixture-core")
			script := "#!/bin/sh\ncase \"$1\" in\n-v) echo 'Mihomo v1.0.0'; exit 0;;\n-t) exit 0;;\nesac\nif [ -f \"$2/reject-next-start\" ]; then\n  rm \"$2/reject-next-start\"\n  echo 'fixture: runtime initialization failed' >&2\n  exit 1\nfi\nexec sleep 60\n"
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(configPath, []byte("listeners: []\n"), 0600); err != nil {
				t.Fatal(err)
			}
			core := mihomo.NewService(&config.Config{Mihomo: config.MihomoConfig{Binary: binary, Config: configPath}})
			if err := core.StartMihomo(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = core.StopMihomo() })
			db, err := database.InitDB(filepath.Join(dir, "panel.db"))
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			t.Cleanup(func() { _ = sqlDB.Close() })
			svc := NewService(db, configPath, core)
			candidate := &models.Listener{Name: "test-listener", Protocol: "shadowsocks", Port: "19389", BindAddress: "127.0.0.1", Config: `{"cipher":"aes-128-gcm","password":"test-password"}`}
			if operation != "create" {
				if err := db.Create(candidate).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "reject-next-start"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "create":
				candidate.Enabled = true
				err = svc.Create(candidate)
			case "update":
				candidate.Enabled = true
				err = svc.Update(candidate)
			case "delete":
				err = svc.Delete(candidate.ID)
			}
			if err != nil {
				t.Fatalf("mutation should succeed after DB write (async apply): %v", err)
			}
			// Drain the debounced ApplyConfig (may fail activation).
			time.Sleep(500 * time.Millisecond)
			svc.FlushConfigApplyForTest()
			var remaining []models.Listener
			if err := db.Find(&remaining).Error; err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "create", "update":
				if len(remaining) != 1 || remaining[0].Name != "test-listener" {
					t.Fatalf("listener should remain after async apply failure: %+v", remaining)
				}
			case "delete":
				if len(remaining) != 0 {
					t.Fatalf("listener should stay deleted after async apply failure: %+v", remaining)
				}
			}
		})
	}
}
