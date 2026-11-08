package system

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/migration"
)

// ErrForeignBackup marks a Restore upload that belongs to another panel. It is
// a client error (400), not a server failure, because the operator simply
// picked the wrong file / endpoint.
var ErrForeignBackup = errors.New("foreign panel backup")

// ValidateMigrationConfig is injected at startup by the router layer. The
// config generator already depends on this package, so importing it here would
// create a cycle; injection keeps the schema validator as the single source of
// truth without changing the dependency graph.
var ValidateMigrationConfig func(protocol, raw string) error

// SetMigrationValidator wires the real schema validator. It must be called
// before the migration endpoints are served.
func SetMigrationValidator(fn func(protocol, raw string) error) {
	ValidateMigrationConfig = fn
}

// RegisterMigrationRoutes exposes the two migration endpoints. They are
// read-only (preview) and append-only (import); neither replaces the database.
func (h *Handler) RegisterMigrationRoutes(rg *gin.RouterGroup) {
	rg.POST("/backup/migrate/preview", h.PreviewMigration)
	rg.POST("/backup/migrate/import", h.ImportMigration)
}

// PreviewMigration parses an uploaded backup and reports what would be
// imported, without touching the database.
func (h *Handler) PreviewMigration(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not initialised"})
		return
	}
	snap, err := h.parseMigrationUpload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	validateNodeConfigs(snap)
	c.JSON(http.StatusOK, migration.Preview{
		Source:   snap.Source,
		Counts:   snap.Counts(),
		Users:    stripSecrets(snap.Users),
		Nodes:    snap.Nodes,
		Warnings: snap.Warnings,
		Skipped:  snap.Skipped,
		Tables:   snap.Tables,
	})
}

// stripSecrets keeps credentials out of preview responses while still showing
// that a credential exists (important for the operator to trust the migration).
func stripSecrets(users []migration.User) []migration.User {
	out := make([]migration.User, len(users))
	for i, u := range users {
		out[i] = u
		if out[i].UUID != "" {
			out[i].UUID = mask(out[i].UUID)
		}
		if out[i].Password != "" {
			out[i].Password = mask(out[i].Password)
		}
	}
	return out
}

func mask(s string) string {
	if len(s) <= 6 {
		return strings.Repeat("*", len(s))
	}
	return s[:3] + strings.Repeat("*", 8) + s[len(s)-3:]
}

// ImportMigration parses and appends the snapshot in one transaction. A
// timestamped backup of the current database is taken first.
func (h *Handler) ImportMigration(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not initialised"})
		return
	}
	if h.dbPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database path is not configured"})
		return
	}
	snap, err := h.parseMigrationUpload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	validateNodeConfigs(snap)
	res, err := migration.Import(h.db, snap, migration.Options{
		Backup: func() (string, error) { return h.snapshotBeforeMigration() },
	})
	if err != nil {
		log.Printf("system migration import failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[INFO] Migration import complete: source=%s users=%d listeners=%d bindings=%d backup=%s",
		res.Source, res.UsersCreated, res.NodesCreated, res.BindingsCreated, res.BackupFile)
	c.JSON(http.StatusOK, res)
}

// validateNodeConfigs checks every imported listener fragment against the
// panel's Mihomo schema. An invalid fragment is dropped (keeping the node) and
// reported, so the operator can finish it in the UI instead of getting a
// listener that can never be enabled.
func validateNodeConfigs(snap *migration.Snapshot) {
	if ValidateMigrationConfig == nil {
		return
	}
	for i := range snap.Nodes {
		n := &snap.Nodes[i]
		if strings.TrimSpace(n.Config) == "" {
			continue
		}
		if err := ValidateMigrationConfig(n.Protocol, n.Config); err != nil {
			note := "listener config needs manual review: " + err.Error()
			n.Notes = append(n.Notes, note)
			n.Config = ""
			snap.Warnings = append(snap.Warnings, fmt.Sprintf("listener %q: %s", n.Name, note))
		}
	}
}

func (h *Handler) parseMigrationUpload(c *gin.Context) (*migration.Snapshot, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRestoreDatabaseBytes)
	if err := c.Request.ParseMultipartForm(maxRestoreDatabaseBytes); err != nil {
		return nil, fmt.Errorf("parse upload: %w", err)
	}
	file, err := c.FormFile("database")
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if file.Size > maxRestoreDatabaseBytes {
		return nil, fmt.Errorf("upload exceeds 128 MiB limit")
	}
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return migration.Parse(f)
}

// snapshotBeforeMigration writes a zip snapshot of the live database into the
// backups directory so an import can always be rolled back.
func (h *Handler) snapshotBeforeMigration() (string, error) {
	dir := backupsDirFromDB(h.dbPath)
	if dir == "" {
		return "", fmt.Errorf("backup directory is not configured")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	// The timestamp has second precision, so two imports in the same second
	// used to collide on O_EXCL and abort the second import (with nothing
	// written). Retry with an incrementing suffix until a free name is found.
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("pre-migration-%s.zip", stamp)
		if i > 0 {
			name = fmt.Sprintf("pre-migration-%s-%d.zip", stamp, i)
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		err = WriteZip(f, BackupPaths{DatabasePath: h.dbPath, MihomoConfig: h.mihomoCfg})
		closeErr := f.Close()
		if err != nil {
			_ = os.Remove(path)
			return "", err
		}
		if closeErr != nil {
			_ = os.Remove(path)
			return "", closeErr
		}
		return name, nil
	}
	return "", fmt.Errorf("could not allocate a unique pre-migration backup name")
}

// guardForeignBackup returns an error when an uploaded database belongs to a
// different panel. Restore replaces the whole database, so accepting such a
// file would silently destroy the current installation. Unrecognised files
// fall through to the existing SQLite magic check, which rejects them.
func guardForeignBackup(raw []byte) error {
	source, err := migration.DetectBytes(raw)
	if err != nil {
		return nil
	}
	if source == migration.SourceXUI || source == migration.SourceSUI {
		return fmt.Errorf("%w: this file is a %s panel backup; use migration import instead of Restore (Restore would replace the whole database)", ErrForeignBackup, source)
	}
	return nil
}
