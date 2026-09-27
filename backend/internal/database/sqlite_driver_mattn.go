//go:build !sqlite_modernc

package database

const sqliteDriverName = "sqlite3"

// pragmaDSN is a no-op on the CGO driver.
//
// mattn/go-sqlite3 does not apply _pragma from the DSN here (verified against
// v1.14.22 for both bare paths and file: URIs), so connections keep SQLite's
// defaults: a 2000-page cache and mmap disabled. Both are acceptable for local
// development, and the connection pool is still bounded, which is what stops the
// per-connection caches from multiplying. Production ships the pure-Go driver,
// where the pragmas do take effect.
func pragmaDSN(dsn string) string { return dsn }
