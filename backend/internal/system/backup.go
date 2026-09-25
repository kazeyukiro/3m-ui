package system

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BackupPaths holds filesystem locations needed to export a panel snapshot.
type BackupPaths struct {
	DatabasePath string
	MihomoConfig string
}

// WriteZip creates a zip archive containing the SQLite database and Mihomo config
// when present. The caller owns closing the writer.
func WriteZip(w io.Writer, paths BackupPaths) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	addFile := func(name, path string) error {
		if path == "" {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("%s is a directory", path)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = name
		hdr.Method = zip.Deflate
		out, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, f)
		return err
	}

	if err := addFile("3m-ui.db", paths.DatabasePath); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	if err := addFile("mihomo-config.yaml", paths.MihomoConfig); err != nil {
		return fmt.Errorf("mihomo config: %w", err)
	}
	meta, err := zw.Create("backup-meta.txt")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(meta, "created_at=%s\nsource=3m-ui\n", time.Now().UTC().Format(time.RFC3339))
	return err
}

// RestoreResult carries forward what was actually written so the API layer
// can surface warnings to the operator (e.g. mihomo config silently dropped
// when the panel has no mihomo.config path configured).
type RestoreResult struct {
	DatabasePath     string // absolute path of the restored SQLite file
	MihomoConfigPath string // non-empty if mihomo config was restored
	MihomoSkipped    bool   // true if zip contained mihomo-config.yaml but mihomoCfgPath was empty
}

// RestoreDatabase replaces the live SQLite file with the provided content.
// The content may be:
//   - A raw SQLite database file (e.g. 3m-ui.db)
//   - A zip archive exported by ExportBackup (containing 3m-ui.db + mihomo-config.yaml)
//
// The caller MUST restart the panel process after this returns — the running
// GORM connection pool still holds the old (now-unlinked) SQLite inode, and
// every subsequent write goes to that orphaned inode (silently lost on next
// restart). See api.go RestoreDatabase handler which triggers an os.Exit.
//
// Side effects:
//   - Validates the SQLite magic header ("SQLite format 3\0") before rename
//     so a corrupt or non-DB upload fails loudly instead of breaking the boot.
//   - Removes any stale -journal / -wal / -shm siblings of the DB so SQLite
//     doesn't roll back the just-restored file to a stale transaction state.
func RestoreDatabase(dbPath, mihomoCfgPath string, r io.Reader) (RestoreResult, error) {
	result := RestoreResult{DatabasePath: dbPath}
	if dbPath == "" {
		return result, fmt.Errorf("database path is empty")
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return result, err
	}

	// Read the uploaded content into memory to detect format.
	// We already enforce a 128 MiB upload limit in the API handler.
	raw, err := io.ReadAll(io.LimitReader(r, 128<<20))
	if err != nil {
		return result, fmt.Errorf("read upload: %w", err)
	}

	// Check if it's a zip file (PK magic).
	var dbContent, mihomoCfg []byte
	if len(raw) >= 4 && raw[0] == 0x50 && raw[1] == 0x4B && raw[2] == 0x03 && raw[3] == 0x04 {
		// Zip archive — extract 3m-ui.db from it.
		dbContent, mihomoCfg, err = extractDBFromZip(raw)
		if err != nil {
			return result, fmt.Errorf("extract from zip: %w", err)
		}
	} else {
		// Raw SQLite database file.
		dbContent = raw
	}

	if len(dbContent) == 0 {
		return result, fmt.Errorf("backup file is empty or does not contain a database")
	}

	// Validate the SQLite magic header. SQLite files always start with the
	// 16-byte string "SQLite format 3\0". Rejecting non-DB uploads here
	// prevents the panel from boot-looping on a corrupt restore.
	const sqliteMagic = "SQLite format 3\x00"
	if len(dbContent) < len(sqliteMagic) || string(dbContent[:len(sqliteMagic)]) != sqliteMagic {
		return result, fmt.Errorf("not a valid SQLite database: missing magic header")
	}

	tmp := dbPath + ".restore-tmp"
	// Clean up the temp file on any failure path. If Rename succeeds the temp
	// no longer exists, so Remove is a no-op (its error is intentionally
	// discarded — the only failure mode is "not found", which is expected).
	defer os.Remove(tmp)
	if err := os.WriteFile(tmp, dbContent, 0o600); err != nil {
		return result, err
	}
	if err := os.Rename(tmp, dbPath); err != nil {
		return result, err
	}

	// Remove stale SQLite auxiliary files so they don't roll back the
	// just-restored DB. SQLite recreates these on next open as needed.
	// Errors are ignored — "not exist" is the common case.
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}

	// Restore mihomo-config.yaml if the zip contained it and the path is set.
	if len(mihomoCfg) > 0 {
		if mihomoCfgPath == "" {
			result.MihomoSkipped = true
		} else {
			cfgDir := filepath.Dir(mihomoCfgPath)
			_ = os.MkdirAll(cfgDir, 0o750)
			cfgTmp := mihomoCfgPath + ".restore-tmp"
			defer os.Remove(cfgTmp)
			if err := os.WriteFile(cfgTmp, mihomoCfg, 0o600); err != nil {
				return result, fmt.Errorf("write mihomo config: %w", err)
			}
			if err := os.Rename(cfgTmp, mihomoCfgPath); err != nil {
				return result, fmt.Errorf("rename mihomo config: %w", err)
			}
			result.MihomoConfigPath = mihomoCfgPath
		}
	}
	return result, nil
}

// extractDBFromZip reads a zip archive and returns the contents of
// 3m-ui.db and mihomo-config.yaml (if present).
func extractDBFromZip(raw []byte) (dbContent, mihomoCfg []byte, err error) {
	zr, zErr := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if zErr != nil {
		return nil, nil, fmt.Errorf("open zip: %w", zErr)
	}
	for _, f := range zr.File {
		switch f.Name {
		case "3m-ui.db":
			rc, e := f.Open()
			if e != nil {
				return nil, nil, fmt.Errorf("open %s in zip: %w", f.Name, e)
			}
			dbContent, e = io.ReadAll(rc)
			rc.Close()
			if e != nil {
				return nil, nil, fmt.Errorf("read %s in zip: %w", f.Name, e)
			}
		case "mihomo-config.yaml":
			rc, e := f.Open()
			if e != nil {
				return nil, nil, fmt.Errorf("open %s in zip: %w", f.Name, e)
			}
			mihomoCfg, e = io.ReadAll(rc)
			rc.Close()
			if e != nil {
				return nil, nil, fmt.Errorf("read %s in zip: %w", f.Name, e)
			}
		}
	}
	if dbContent == nil {
		return nil, nil, fmt.Errorf("zip does not contain 3m-ui.db")
	}
	return dbContent, mihomoCfg, nil
}
