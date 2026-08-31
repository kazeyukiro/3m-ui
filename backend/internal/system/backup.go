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

// RestoreDatabase replaces the live SQLite file with the provided content.
// The content may be:
//   - A raw SQLite database file (e.g. 3m-ui.db)
//   - A zip archive exported by ExportBackup (containing 3m-ui.db + mihomo-config.yaml)
//
// Callers should stop the panel or accept that a process restart may be required.
func RestoreDatabase(dbPath, mihomoCfgPath string, r io.Reader) error {
	if dbPath == "" {
		return fmt.Errorf("database path is empty")
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	// Read the uploaded content into memory to detect format.
	// We already enforce a 128 MiB upload limit in the API handler.
	raw, err := io.ReadAll(io.LimitReader(r, 128<<20))
	if err != nil {
		return fmt.Errorf("read upload: %w", err)
	}

	// Check if it's a zip file (PK magic).
	var dbContent, mihomoCfg []byte
	if len(raw) >= 4 && raw[0] == 0x50 && raw[1] == 0x4B && raw[2] == 0x03 && raw[3] == 0x04 {
		// Zip archive — extract 3m-ui.db from it.
		dbContent, mihomoCfg, err = extractDBFromZip(raw)
		if err != nil {
			return fmt.Errorf("extract from zip: %w", err)
		}
	} else {
		// Raw SQLite database file.
		dbContent = raw
	}

	if len(dbContent) == 0 {
		return fmt.Errorf("backup file is empty or does not contain a database")
	}

	tmp := dbPath + ".restore-tmp"
	// Clean up the temp file on any failure path. If Rename succeeds the temp
	// no longer exists, so Remove is a no-op (its error is intentionally
	// discarded — the only failure mode is "not found", which is expected).
	defer os.Remove(tmp)
	if err := os.WriteFile(tmp, dbContent, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, dbPath); err != nil {
		return err
	}

	// Restore mihomo-config.yaml if the zip contained it and the path is set.
	if len(mihomoCfg) > 0 && mihomoCfgPath != "" {
		cfgDir := filepath.Dir(mihomoCfgPath)
		_ = os.MkdirAll(cfgDir, 0o750)
		cfgTmp := mihomoCfgPath + ".restore-tmp"
		defer os.Remove(cfgTmp)
		if err := os.WriteFile(cfgTmp, mihomoCfg, 0o600); err == nil {
			_ = os.Rename(cfgTmp, mihomoCfgPath)
		}
	}
	return nil
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
