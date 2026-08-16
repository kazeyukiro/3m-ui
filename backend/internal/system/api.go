package system

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler serves system HTTP endpoints using an injected Service.
type Handler struct {
	svc       *Service
	dbPath    string
	mihomoCfg string
}

// NewHandler constructs a system HTTP handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// WithBackupPaths enables database/config export endpoints.
func (h *Handler) WithBackupPaths(dbPath, mihomoConfig string) *Handler {
	h.dbPath = dbPath
	h.mihomoCfg = mihomoConfig
	return h
}

// RegisterRoutes registers system stats routes under the provided group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/status", h.GetSystemStatus)
	rg.GET("/backup", h.ExportBackup)
	rg.POST("/backup/restore-db", h.RestoreDatabase)
}

func (h *Handler) GetSystemStatus(c *gin.Context) {
	stats := h.svc.GetStatus()
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) ExportBackup(c *gin.Context) {
	name := fmt.Sprintf("3m-ui-backup-%s.zip", time.Now().UTC().Format("20060102-150405"))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	if err := WriteZip(c.Writer, BackupPaths{DatabasePath: h.dbPath, MihomoConfig: h.mihomoCfg}); err != nil {
		_ = c.Error(err)
		return
	}
}

func (h *Handler) RestoreDatabase(c *gin.Context) {
	file, err := c.FormFile("database")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart field database is required"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()
	if h.dbPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database path is not configured"})
		return
	}
	if err := RestoreDatabase(h.dbPath, f); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "database restored; restart the panel process to reopen SQLite connections",
		"path":    filepath.Base(h.dbPath),
	})
}
