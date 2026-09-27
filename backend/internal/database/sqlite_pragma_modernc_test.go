//go:build sqlite_modernc

package database

import (
	"path/filepath"
	"testing"
)

// Production ships the pure-Go driver, where the DSN pragmas do apply. If they
// ever stop applying, the pool quietly goes back to a 2000-page cache per
// connection — the thing this tuning exists to prevent.
func TestPragmasAppliedOnPureGoDriver(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("underlying *sql.DB: %v", err)
	}
	defer sqlDB.Close()

	var cacheSize, mmapSize int
	if err := db.Raw("PRAGMA cache_size").Scan(&cacheSize).Error; err != nil {
		t.Fatalf("read cache_size: %v", err)
	}
	if err := db.Raw("PRAGMA mmap_size").Scan(&mmapSize).Error; err != nil {
		t.Fatalf("read mmap_size: %v", err)
	}
	if cacheSize != -512 {
		t.Errorf("cache_size = %d, want -512 (KiB); the default -2000 is 2MB per connection", cacheSize)
	}
	if mmapSize != 0 {
		t.Errorf("mmap_size = %d, want 0: this project never trades memory pressure for mmap", mmapSize)
	}
}

func TestPragmaDSNKeepsExistingQuery(t *testing.T) {
	got := pragmaDSN("file:/x/panel.db?mode=ro")
	if want := "file:/x/panel.db?mode=ro&_pragma="; len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("pragmaDSN dropped the existing query string: %q", got)
	}
	plain := pragmaDSN("/x/panel.db")
	if plain[:len("/x/panel.db?")] != "/x/panel.db?" {
		t.Fatalf("pragmaDSN did not start a query string: %q", plain)
	}
}
