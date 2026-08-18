package system

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

const maxRestoreDatabaseBytes = 128 << 20

type Handler struct {
	svc       *Service
	dbPath    string
	mihomoCfg string
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) WithBackupPaths(dbPath, mihomoConfig string) *Handler {
	h.dbPath = dbPath
	h.mihomoCfg = mihomoConfig
	return h
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/status", h.GetSystemStatus)
	rg.GET("/backup", h.ExportBackup)
	rg.POST("/backup/restore-db", h.RestoreDatabase)
	rg.POST("/templates/reverse-proxy", h.ReverseProxy)
	rg.POST("/templates/acme", h.ACME)
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
	if h.dbPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database path is not configured"})
		return
	}
	// FormFile otherwise permits arbitrarily large multipart requests to be
	// written to disk, turning an authenticated endpoint into a storage DoS.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRestoreDatabaseBytes)
	file, err := c.FormFile("database")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart field database is required and must be <= 128 MiB"})
		return
	}
	if file.Size > maxRestoreDatabaseBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "database backup exceeds 128 MiB limit"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()
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

func (h *Handler) ReverseProxy(c *gin.Context) {
	var req struct {
		Kind     string `json:"kind"`
		Domain   string `json:"domain"`
		Upstream string `json:"upstream"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := ReverseProxyTemplate(req.Kind, req.Domain, req.Upstream)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": out})
}

func (h *Handler) ACME(c *gin.Context) {
	var req struct {
		Domain  string `json:"domain"`
		Email   string `json:"email"`
		Webroot string `json:"webroot"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"command": ACMECommand(req.Domain, req.Email, req.Webroot)})
}
