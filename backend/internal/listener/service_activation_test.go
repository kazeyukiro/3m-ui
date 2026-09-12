package listener

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
)

func TestListenerMutationsRestoreDatabaseAfterActivationFailure(t *testing.T) {
	for _, operation := range []string{"create", "update", "delete"} {
		t.Run(operation, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(mihomo.AllowBinaryPathPrefixForTesting(dir))
			binary := filepath.Join(dir, "fixture-core")
			// Validation succeeds, but the next process start fails. The old
			// configuration can then be restarted, exercising the rollback path.
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
			if err == nil {
				t.Fatal("mutation reported success despite the failed core activation")
			}
			var remaining []models.Listener
			if err := db.Find(&remaining).Error; err != nil {
				t.Fatal(err)
			}
			if operation == "create" {
				if len(remaining) != 0 {
					t.Fatal("failed creation left a listener in the database")
				}
			} else if len(remaining) != 1 || remaining[0].ID != candidate.ID || remaining[0].Name != candidate.Name || remaining[0].Enabled {
				t.Fatalf("previous listener was not restored: %+v", remaining)
			}
			content, err := os.ReadFile(configPath)
			if err != nil || strings.Contains(string(content), candidate.Name) {
				t.Fatalf("runtime configuration did not restore the disabled/absent listener: %v", err)
			}
			status, err := core.GetStatus()
			if err != nil || !status.Running {
				t.Fatalf("previous core did not recover: status=%+v err=%v", status, err)
			}
		})
	}
}
