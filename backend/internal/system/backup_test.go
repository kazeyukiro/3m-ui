package system

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// makeMinimalSQLite returns a byte slice that starts with the SQLite magic
// header ("SQLite format 3\0") so RestoreDatabase accepts it. The page count
// is left at 0 — we only test the magic-header validation path, not actual
// SQLite reads (GORM would refuse to open this, but RestoreDatabase only
// checks the magic).
func makeMinimalSQLite() []byte {
	b := make([]byte, 4096)
	copy(b, []byte("SQLite format 3\x00"))
	return b
}

func TestRestoreDatabase_RejectsNonSQLiteUpload(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	// Upload a plain text file — should fail at magic-header check.
	_, err := RestoreDatabase(dbPath, "", bytes.NewReader([]byte("hello world this is not a SQLite file")))
	if err == nil {
		t.Fatal("expected error for non-SQLite upload, got nil")
	}
}

func TestRestoreDatabase_RejectsEmptyUpload(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	_, err := RestoreDatabase(dbPath, "", bytes.NewReader(nil))
	if err == nil {
		t.Fatal("expected error for empty upload, got nil")
	}
}

func TestRestoreDatabase_AcceptsRawSQLiteAndRemovesWalShm(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	// Pre-create stale WAL/SHM siblings to simulate a panel that crashed mid-txn.
	if err := os.WriteFile(dbPath+"-wal", []byte("stale-wal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath+"-shm", []byte("stale-shm"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath+"-journal", []byte("stale-journal"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := RestoreDatabase(dbPath, "", bytes.NewReader(makeMinimalSQLite()))
	if err != nil {
		t.Fatalf("RestoreDatabase: %v", err)
	}
	if result.DatabasePath != dbPath {
		t.Errorf("DatabasePath = %q, want %q", result.DatabasePath, dbPath)
	}
	// WAL/SHM/journal siblings must be removed so SQLite doesn't roll back.
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(dbPath + suffix); !os.IsNotExist(err) {
			t.Errorf("stale %s should have been removed", suffix)
		}
	}
	// Restored DB file should contain the magic header.
	got, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got[:16]) != "SQLite format 3\x00" {
		t.Errorf("restored DB missing SQLite magic, got %q", string(got[:16]))
	}
}

func TestRestoreDatabase_RestoresMihomoConfigFromZip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	mihomoPath := filepath.Join(dir, "mihomo.yaml")

	// Build a zip containing 3m-ui.db + mihomo-config.yaml.
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	dbContent := makeMinimalSQLite()
	if w, err := zw.Create("3m-ui.db"); err != nil {
		t.Fatal(err)
	} else if _, err := w.Write(dbContent); err != nil {
		t.Fatal(err)
	}
	if w, err := zw.Create("mihomo-config.yaml"); err != nil {
		t.Fatal(err)
	} else if _, err := w.Write([]byte("proxies: []\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	result, err := RestoreDatabase(dbPath, mihomoPath, bytes.NewReader(zipBuf.Bytes()))
	if err != nil {
		t.Fatalf("RestoreDatabase: %v", err)
	}
	if result.MihomoConfigPath != mihomoPath {
		t.Errorf("MihomoConfigPath = %q, want %q", result.MihomoConfigPath, mihomoPath)
	}
	got, err := os.ReadFile(mihomoPath)
	if err != nil {
		t.Fatalf("mihomo config not restored: %v", err)
	}
	if string(got) != "proxies: []\n" {
		t.Errorf("mihomo config content = %q, want 'proxies: []'", string(got))
	}
}

func TestRestoreDatabase_MihomoSkippedWhenPathEmpty(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	if w, err := zw.Create("3m-ui.db"); err != nil {
		t.Fatal(err)
	} else if _, err := w.Write(makeMinimalSQLite()); err != nil {
		t.Fatal(err)
	}
	if w, err := zw.Create("mihomo-config.yaml"); err != nil {
		t.Fatal(err)
	} else if _, err := w.Write([]byte("proxies: []\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	result, err := RestoreDatabase(dbPath, "", bytes.NewReader(zipBuf.Bytes()))
	if err != nil {
		t.Fatalf("RestoreDatabase: %v", err)
	}
	if !result.MihomoSkipped {
		t.Errorf("MihomoSkipped should be true when mihomoCfgPath is empty")
	}
	if result.MihomoConfigPath != "" {
		t.Errorf("MihomoConfigPath should be empty, got %q", result.MihomoConfigPath)
	}
}

// TestCleanupLocalBackups_KeepAlwaysProtectsNewest verifies the AND semantics:
// when both keep and older_than_days are set, the newest `keep` entries survive
// even if they're all older than the age threshold. Pre-fix this would delete
// ALL backups when they were all older than `older_than_days`, losing every
// snapshot the operator had.
func TestCleanupLocalBackups_KeepAlwaysProtectsNewest(t *testing.T) {
	dir := t.TempDir()
	// Create 3 backups all 10 days old (older than older_than_days=7).
	oldTime := time.Now().UTC().Add(-10 * 24 * time.Hour)
	for i := 0; i < 3; i++ {
		name := filepath.Join(dir, oldTime.Add(time.Duration(i)*time.Second).Format("20060102T150405Z")+"-100.tar.gz")
		if err := os.WriteFile(name, []byte("backup"+string(rune('0'+i))), 0o600); err != nil {
			t.Fatal(err)
		}
		// Override mtime to force "old".
		if err := os.Chtimes(name, oldTime, oldTime.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}

	// keep=3, older_than_days=7: with AND semantics, the 3 newest are
	// protected even though all are older than 7 days. 0 should be deleted.
	deleted, kept, _, err := cleanupLocalBackups(dir, 3, 7)
	if err != nil {
		t.Fatalf("cleanupLocalBackups: %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("expected 0 deletions (keep=3 protects newest 3 even when older than age), got %d: %v", len(deleted), deleted)
	}
	if kept != 3 {
		t.Errorf("expected 3 kept, got %d", kept)
	}
}

func TestCleanupLocalBackups_KeepOnlyDeletesBeyondKeep(t *testing.T) {
	dir := t.TempDir()
	// 5 backups, newest first by mtime.
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, time.Now().UTC().Add(-time.Duration(i)*time.Hour).Format("20060102T150405Z")+"-100.tar.gz")
		if err := os.WriteFile(name, []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// keep=2 only: delete entries with index >= 2 (3 deletions).
	deleted, kept, _, err := cleanupLocalBackups(dir, 2, 0)
	if err != nil {
		t.Fatalf("cleanupLocalBackups: %v", err)
	}
	if len(deleted) != 3 {
		t.Errorf("expected 3 deletions (keep=2 → delete 3 oldest), got %d", len(deleted))
	}
	if kept != 2 {
		t.Errorf("expected 2 kept, got %d", kept)
	}
}

func TestCleanupLocalBackups_AgeOnlyDeletesOld(t *testing.T) {
	dir := t.TempDir()
	// 1 recent + 1 old (10 days ago).
	now := time.Now().UTC()
	recentName := filepath.Join(dir, now.Format("20060102T150405Z")+"-100.tar.gz")
	oldName := filepath.Join(dir, now.Add(-10*24*time.Hour).Format("20060102T150405Z")+"-100.tar.gz")
	if err := os.WriteFile(recentName, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldName, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldName, now.Add(-10*24*time.Hour), now.Add(-10*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// older_than_days=7 only (keep=0): delete entries older than 7 days (1 deletion).
	deleted, kept, _, err := cleanupLocalBackups(dir, 0, 7)
	if err != nil {
		t.Fatalf("cleanupLocalBackups: %v", err)
	}
	if len(deleted) != 1 {
		t.Errorf("expected 1 deletion (old backup), got %d", len(deleted))
	}
	if kept != 1 {
		t.Errorf("expected 1 kept (recent backup), got %d", kept)
	}
}
