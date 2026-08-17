package mihomo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyConfigCreatesBackupAndRollbackRestoresPreviousConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	old := "mixed-port: 7890\n"
	candidate := "mixed-port: 7891\n"
	if err := os.WriteFile(path, []byte(old), 0600); err != nil { t.Fatal(err) }

	svc := &Service{cm: NewConfigManager(path)}
	if err := svc.ApplyConfig(candidate); err != nil { t.Fatalf("ApplyConfig: %v", err) }
	got, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	if string(got) != candidate { t.Fatalf("applied config = %q, want %q", got, candidate) }

	backup, err := os.ReadFile(path + ".bak")
	if err != nil { t.Fatal(err) }
	if string(backup) != old { t.Fatalf("backup = %q, want %q", backup, old) }

	if err := svc.RollbackConfig(); err != nil { t.Fatalf("RollbackConfig: %v", err) }
	got, err = os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	if string(got) != old { t.Fatalf("rolled back config = %q, want %q", got, old) }
}

func TestApplyConfigDoesNotRequireMihomoProcessManager(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("mixed-port: 7890\n"), 0600); err != nil { t.Fatal(err) }
	svc := &Service{cm: NewConfigManager(path)}
	if err := svc.ApplyConfig("mixed-port: 7892\n"); err != nil { t.Fatalf("ApplyConfig: %v", err) }
}
