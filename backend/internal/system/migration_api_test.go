package system

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database"
)

// newSQLiteFile creates a real SQLite database (VACUUM INTO requires a valid
// file, not just a magic header) with the supplied DDL.
func newSQLiteFile(t *testing.T, ddl ...string) (path string, raw []byte) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "source.db")
	db, err := sql.Open(database.SQLiteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range ddl {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, raw
}

// Two imports inside the same second must not collide on the backup filename.
// Before the fix the second one failed with "file exists" and imported nothing.
func TestSnapshotBeforeMigrationUniqueNames(t *testing.T) {
	dir := t.TempDir()
	dbPath, _ := newSQLiteFile(t, "CREATE TABLE t (x INTEGER)")
	// Put the live DB inside dir so backupsDirFromDB resolves to dir/backups.
	live := filepath.Join(dir, "3m-ui.db")
	if err := os.WriteFile(live, mustRead(t, dbPath), 0o600); err != nil {
		t.Fatal(err)
	}

	h := &Handler{dbPath: live}
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		name, err := h.snapshotBeforeMigration()
		if err != nil {
			t.Fatalf("snapshot %d: %v", i, err)
		}
		if seen[name] {
			t.Fatalf("duplicate backup name on iteration %d: %s", i, name)
		}
		seen[name] = true
		full := filepath.Join(dir, "backups", name)
		info, err := os.Stat(full)
		if err != nil || info.Size() == 0 {
			t.Fatalf("backup %s missing or empty: %v", name, err)
		}
		zr, err := zip.NewReader(bytes.NewReader(mustRead(t, full)), info.Size())
		if err != nil {
			t.Fatalf("open backup zip: %v", err)
		}
		found := false
		for _, f := range zr.File {
			if filepath.Ext(f.Name) != ".db" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			head := make([]byte, 16)
			_, _ = rc.Read(head)
			rc.Close()
			if string(head) == "SQLite format 3\x00" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("backup %s does not hold a SQLite snapshot", name)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A s-ui / 3x-ui backup must be rejected by the whole-database Restore path
// with a client-error sentinel, so the handler returns 400 instead of 500.
func TestGuardForeignBackupIsClientError(t *testing.T) {
	_, raw := newSQLiteFile(t,
		"CREATE TABLE clients (id INTEGER PRIMARY KEY, name TEXT, config TEXT, inbounds TEXT)",
		"CREATE TABLE inbounds (id INTEGER PRIMARY KEY, type TEXT, options TEXT)",
	)
	err := guardForeignBackup(raw)
	if err == nil {
		t.Fatal("foreign panel backup was not rejected")
	}
	if !errors.Is(err, ErrForeignBackup) {
		t.Fatalf("error is not ErrForeignBackup: %v", err)
	}
	if got := err.Error(); !bytes.Contains([]byte(got), []byte("migration import instead of Restore")) {
		t.Fatalf("error lacks migration hint: %s", got)
	}
	// Unrecognised bytes fall through to the existing SQLite magic check.
	if err := guardForeignBackup([]byte("not a database")); err != nil {
		t.Fatalf("junk should not be flagged as foreign backup: %v", err)
	}
}
