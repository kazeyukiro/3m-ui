package system

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestWriteZipSnapshotIsConsistentAndSelfContained verifies that WriteZip copies
// a live, WAL-mode database into a crash-consistent snapshot and no longer
// bundles the -wal/-shm sidecars.
//
// Regression for two issues:
//   - A raw file copy of an open, actively-written database can capture a torn
//     page; VACUUM INTO must instead produce a fully-replayed copy.
//   - The previous code bundled 3m-ui.db-wal / 3m-ui.db-shm but RestoreDatabase
//     never read them, so un-checkpointed writes were silently lost on restore.
func TestWriteZipSnapshotIsConsistentAndSelfContained(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "3m-ui.db")

	// Open a real database and keep the handle open to simulate the running
	// GORM pool writing concurrently while a backup is taken.
	src, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open source db: %v", err)
	}
	defer src.Close()
	if _, err := src.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if _, err := src.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		t.Fatalf("set WAL: %v", err)
	}
	if _, err := src.Exec(`CREATE TABLE listeners (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	// Insert enough rows that some land in the -wal file (not yet checkpointed).
	for i := 0; i < 200; i++ {
		if _, err := src.Exec(`INSERT INTO listeners (id, name) VALUES (?, ?)`, i, "listener-"+string(rune('a'+i%26))); err != nil {
			t.Fatalf("insert row %d: %v", i, err)
		}
	}

	// Snapshot while the source connection is still open and dirty.
	var buf bytes.Buffer
	if err := WriteZip(&buf, BackupPaths{DatabasePath: dbPath}); err != nil {
		t.Fatalf("WriteZip: %v", err)
	}

	// Read the archive back.
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var dbBytes []byte
	sawWal := false
	sawShm := false
	for _, f := range zr.File {
		switch f.Name {
		case "3m-ui.db":
			rc, e := f.Open()
			if e != nil {
				t.Fatalf("open 3m-ui.db in zip: %v", e)
			}
			dbBytes, e = io.ReadAll(rc)
			rc.Close()
			if e != nil {
				t.Fatalf("read 3m-ui.db in zip: %v", e)
			}
		case "3m-ui.db-wal":
			sawWal = true
		case "3m-ui.db-shm":
			sawShm = true
		}
	}
	if sawWal || sawShm {
		t.Fatalf("backup still bundles sidecars: wal=%v shm=%v", sawWal, sawShm)
	}
	if len(dbBytes) == 0 {
		t.Fatal("zip contains no 3m-ui.db")
	}

	// The snapshot must be a complete, replayed database: open it standalone
	// (no source connection, no -wal present) and confirm every row survived.
	restored := filepath.Join(dir, "restored.db")
	if err := os.WriteFile(restored, dbBytes, 0o600); err != nil {
		t.Fatalf("write restored db: %v", err)
	}
	rdb, err := sql.Open("sqlite", restored)
	if err != nil {
		t.Fatalf("open restored db: %v", err)
	}
	defer rdb.Close()
	var n int
	if err := rdb.QueryRow(`SELECT COUNT(*) FROM listeners`).Scan(&n); err != nil {
		t.Fatalf("count restored rows: %v", err)
	}
	if n != 200 {
		t.Fatalf("restored db missing rows: got %d want 200", n)
	}
}
