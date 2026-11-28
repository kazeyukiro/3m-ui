package migration

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database"
)

const sqliteMagic = "SQLite format 3\x00"

// maxUploadBytes mirrors the restore endpoint's cap; a migration file larger
// than this is refused before any parsing work.
const maxUploadBytes = 128 << 20

// extracted holds the database bytes plus the container they came from.
type extracted struct {
	DB        []byte
	Container string
	Name      string
}

// extractSQLite pulls a SQLite database out of the uploaded bytes. Accepted
// containers: a raw database, a zip archive, or a gzip/tar archive. A zip
// commonly also carries a config.json; only the database is needed here.
func extractSQLite(raw []byte) (extracted, error) {
	if len(raw) == 0 {
		return extracted{}, errors.New("upload is empty")
	}
	if len(raw) >= len(sqliteMagic) && string(raw[:len(sqliteMagic)]) == sqliteMagic {
		return extracted{DB: raw, Container: "sqlite"}, nil
	}
	if len(raw) >= 4 && raw[0] == 'P' && raw[1] == 'K' && raw[2] == 0x03 && raw[3] == 0x04 {
		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return extracted{}, fmt.Errorf("open zip: %w", err)
		}
		// Prefer a plausible database name, then fall back to content sniffing.
		names := make([]string, 0, len(zr.File))
		for _, f := range zr.File {
			names = append(names, f.Name)
		}
		sort.Slice(names, func(i, j int) bool { return dbNameScore(names[i]) > dbNameScore(names[j]) })
		for _, name := range names {
			var f *zip.File
			for _, zf := range zr.File {
				if zf.Name == name {
					f = zf
					break
				}
			}
			if f == nil || f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(rc, maxUploadBytes))
			rc.Close()
			if err != nil {
				continue
			}
			if isSQLite(data) {
				return extracted{DB: data, Container: "zip", Name: filepath.Base(name)}, nil
			}
		}
		return extracted{}, errors.New("zip archive does not contain a SQLite database")
	}
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return extracted{}, fmt.Errorf("open gzip: %w", err)
		}
		defer gz.Close()
		plain, err := io.ReadAll(io.LimitReader(gz, maxUploadBytes))
		if err != nil {
			return extracted{}, fmt.Errorf("read gzip: %w", err)
		}
		if isSQLite(plain) {
			return extracted{DB: plain, Container: "gzip"}, nil
		}
		tr := tar.NewReader(bytes.NewReader(plain))
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return extracted{}, fmt.Errorf("read tar: %w", err)
			}
			if hdr.FileInfo().IsDir() {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(tr, maxUploadBytes))
			if err != nil {
				continue
			}
			if isSQLite(data) {
				return extracted{DB: data, Container: "tar.gz", Name: filepath.Base(hdr.Name)}, nil
			}
		}
		return extracted{}, errors.New("tar archive does not contain a SQLite database")
	}
	return extracted{}, errors.New("unsupported backup format: expected a SQLite database, zip or tar.gz")
}

func isSQLite(b []byte) bool {
	return len(b) >= len(sqliteMagic) && string(b[:len(sqliteMagic)]) == sqliteMagic
}

func dbNameScore(name string) int {
	base := strings.ToLower(filepath.Base(name))
	switch {
	case base == "x-ui.db" || base == "s-ui.db" || base == "3m-ui.db":
		return 4
	case strings.HasSuffix(base, ".db") || strings.HasSuffix(base, ".sqlite"):
		return 3
	case strings.Contains(base, "db"):
		return 2
	default:
		return 1
	}
}

// schema describes just enough of a SQLite file to fingerprint its origin.
type schema struct {
	Tables map[string][]string // lowercased table name -> lowercased columns
}

func (s schema) has(table string, cols ...string) bool {
	c, ok := s.Tables[table]
	if !ok {
		return false
	}
	set := make(map[string]struct{}, len(c))
	for _, x := range c {
		set[x] = struct{}{}
	}
	for _, want := range cols {
		if _, ok := set[want]; !ok {
			return false
		}
	}
	return true
}

func (s schema) tableNames() []string {
	out := make([]string, 0, len(s.Tables))
	for name := range s.Tables {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func readSchema(db *sql.DB) (schema, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return schema{}, fmt.Errorf("list tables: %w", err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return schema{}, err
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return schema{}, err
	}
	rows.Close()

	out := schema{Tables: make(map[string][]string, len(names))}
	for _, name := range names {
		// Table names come from the file itself; quote as an identifier.
		q := `PRAGMA table_info("` + strings.ReplaceAll(name, `"`, `""`) + `")`
		cr, err := db.Query(q)
		if err != nil {
			continue
		}
		var cols []string
		for cr.Next() {
			var cid int
			var cname, ctype string
			var notnull, pk int
			var dflt interface{}
			if err := cr.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err != nil {
				break
			}
			cols = append(cols, strings.ToLower(cname))
		}
		cr.Close()
		out.Tables[strings.ToLower(name)] = cols
	}
	return out, nil
}

// detectSource fingerprints the schema. It deliberately keys off distinctive
// column combinations rather than a single table name, because several panels
// share generic names such as "inbounds".
func detectSource(s schema) Source {
	switch {
	case s.has("listeners", "name", "protocol", "config") && s.has("proxy_users", "username", "uuid"):
		return SourceSelf
	// sing-box based panel: clients carry a per-protocol config map and an
	// explicit inbound id list; inbounds store a type plus an options blob.
	case s.has("clients", "name", "config", "inbounds") && s.has("inbounds", "type", "options"):
		return SourceSUI
	// Xray based panel: inbounds hold a protocol plus settings/stream_settings
	// JSON blobs (newer releases may also split clients into their own table).
	case s.has("inbounds", "protocol", "settings", "stream_settings"):
		return SourceXUI
	case s.has("inbounds", "protocol", "settings", "streamsettings"):
		return SourceXUI
	default:
		return SourceUnknown
	}
}

// openSnapshot writes the extracted database to a private temp file and opens
// it read-only. The caller must close the returned cleanup function.
func openSnapshot(dbBytes []byte) (*sql.DB, func(), error) {
	dir, err := os.MkdirTemp("", "3m-ui-migration-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "source.db")
	if err := os.WriteFile(path, dbBytes, 0o600); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("write temp database: %w", err)
	}
	// mode=ro guarantees parsing can never mutate the uploaded snapshot.
	dsn := "file:" + filepath.ToSlash(path) + "?mode=ro"
	db, err := sql.Open(database.SQLiteDriverName, dsn)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("open snapshot: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		cleanup()
		return nil, nil, fmt.Errorf("open snapshot: %w", err)
	}
	return db, func() { _ = db.Close(); cleanup() }, nil
}
