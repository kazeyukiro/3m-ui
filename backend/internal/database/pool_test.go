package database

import (
	"path/filepath"
	"testing"
)

// The pool must be bounded — every SQLite connection keeps its own page cache and
// gorm's default is unlimited — but never down to 1, because iterating a result
// set holds a connection and a query issued inside that loop would deadlock.
func TestInitDBCapsConnectionPool(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("underlying *sql.DB: %v", err)
	}
	defer sqlDB.Close()

	got := sqlDB.Stats().MaxOpenConnections
	if got != maxSQLiteConns {
		t.Fatalf("MaxOpenConnections = %d, want %d", got, maxSQLiteConns)
	}
	if got < 2 {
		t.Fatalf("MaxOpenConnections = %d; a single connection deadlocks on "+
			"queries issued while iterating a result set", got)
	}
}

func TestOpenReadOnlyCapsConnectionPool(t *testing.T) {
	path := filepath.Join(t.TempDir(), "readonly.db")
	seed, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if seedDB, err := seed.DB(); err == nil {
		seedDB.Close()
	}

	db, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("underlying *sql.DB: %v", err)
	}
	defer sqlDB.Close()

	if got := sqlDB.Stats().MaxOpenConnections; got < 2 || got > maxSQLiteConns {
		t.Fatalf("MaxOpenConnections = %d, want between 2 and %d", got, maxSQLiteConns)
	}
}
