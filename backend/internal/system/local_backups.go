package system

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LocalBackup is an on-disk snapshot under the data directory (install/update).
type LocalBackup struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}

func backupsDirFromDB(dbPath string) string {
	if strings.TrimSpace(dbPath) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(dbPath), "backups")
}

func listLocalBackups(dir string) ([]LocalBackup, int64, error) {
	if dir == "" {
		return nil, 0, fmt.Errorf("backup directory is not configured")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LocalBackup{}, 0, nil
		}
		return nil, 0, err
	}
	var out []LocalBackup
	var total int64
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." || strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		if info.IsDir() {
			size = dirSize(filepath.Join(dir, name))
		}
		total += size
		out = append(out, LocalBackup{
			Name:    name,
			Size:    size,
			ModTime: info.ModTime().UTC(),
			IsDir:   info.IsDir(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ModTime.After(out[j].ModTime)
	})
	return out, total, nil
}

func dirSize(path string) int64 {
	var n int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		n += info.Size()
		return nil
	})
	return n
}

// safeBackupName rejects path traversal.
func safeBackupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid backup name")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid backup name")
	}
	cleaned := filepath.Base(name)
	if cleaned != name {
		return "", fmt.Errorf("invalid backup name")
	}
	return cleaned, nil
}

func deleteLocalBackup(dir, name string) error {
	name, err := safeBackupName(name)
	if err != nil {
		return err
	}
	if dir == "" {
		return fmt.Errorf("backup directory is not configured")
	}
	path := filepath.Join(dir, name)
	// Ensure resolved path stays under dir.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(absPath, absDir+string(os.PathSeparator)) && absPath != absDir {
		return fmt.Errorf("invalid backup path")
	}
	return os.RemoveAll(path)
}

// cleanupLocalBackups keeps the newest `keep` entries and/or deletes those older
// than olderThanDays. keep<=0 means do not use keep rule; olderThanDays<=0 means
// do not use age rule. At least one rule must be positive.
func cleanupLocalBackups(dir string, keep, olderThanDays int) (deleted []string, kept int, freed int64, err error) {
	if keep <= 0 && olderThanDays <= 0 {
		return nil, 0, 0, fmt.Errorf("specify keep > 0 and/or older_than_days > 0")
	}
	list, _, err := listLocalBackups(dir)
	if err != nil {
		return nil, 0, 0, err
	}
	cutoff := time.Time{}
	if olderThanDays > 0 {
		cutoff = time.Now().UTC().Add(-time.Duration(olderThanDays) * 24 * time.Hour)
	}
	for i, b := range list {
		remove := false
		if keep > 0 && i >= keep {
			remove = true
		}
		if olderThanDays > 0 && b.ModTime.Before(cutoff) {
			remove = true
		}
		// When both rules set: delete if EITHER says remove (keep newest N AND age).
		// Actually better: keep means "always keep newest N"; age deletes older ones among the rest.
		// Simpler semantics matching user need:
		// - keep=3: delete everything after the 3 newest
		// - older_than_days=7: delete any older than 7 days
		// - both: delete if index>=keep OR older than days (but never delete if within keep? )
		// User space: "keep last 3" is the main need.
		if keep > 0 && olderThanDays > 0 {
			remove = i >= keep || b.ModTime.Before(cutoff)
		}
		if !remove {
			kept++
			continue
		}
		if err := deleteLocalBackup(dir, b.Name); err != nil {
			return deleted, kept, freed, err
		}
		deleted = append(deleted, b.Name)
		freed += b.Size
	}
	return deleted, kept, freed, nil
}
