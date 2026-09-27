//go:build sqlite_modernc

package database

import (
	"strings"

	_ "modernc.org/sqlite"
)

const sqliteDriverName = "sqlite"

// pragmaDSN appends the memory-related pragmas to a SQLite DSN.
//
// cache_size bounds the page cache each connection keeps: at SQLite's default of
// 2000 pages, a handful of pooled connections is the entire budget on a small
// host, and the pool used to be unbounded.
//
// mmap_size=0 is stated rather than assumed. This project never trades memory
// pressure for mmap; SQLite's default is zero only because the embedded copy is
// compiled that way, so pinning it keeps the guarantee local.
//
// busy_timeout goes up because the pool is now bounded: with fewer connections a
// transient lock should be waited out rather than surfaced as SQLITE_BUSY.
//
// The pure-Go driver applies _pragma to every connection it opens, so connections
// created later inherit these too. The CGO driver used for local development does
// not honour them — see sqlite_driver_mattn.go.
func pragmaDSN(dsn string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_pragma=cache_size(-512)&_pragma=mmap_size(0)&_pragma=busy_timeout(5000)"
}
