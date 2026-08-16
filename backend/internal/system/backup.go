package system

import (
	"archive/zip"
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
// Callers should stop the panel or accept that a process restart may be required.
func RestoreDatabase(dbPath string, r io.Reader) error {
	if dbPath == "" {
		return fmt.Errorf("database path is empty")
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp := dbPath + ".restore-tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dbPath)
}
