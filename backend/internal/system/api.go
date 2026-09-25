package system

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	rg.GET("/backups", h.ListLocalBackups)
	rg.DELETE("/backups/:name", h.DeleteLocalBackup)
	rg.POST("/backups/cleanup", h.CleanupLocalBackups)
	rg.POST("/backup/restore-db", h.RestoreDatabase)
	rg.POST("/templates/reverse-proxy", h.ReverseProxy)
	rg.POST("/templates/acme", h.ACME)
	rg.POST("/geofiles/update", h.UpdateGeoFiles)
	rg.POST("/templates/warp", h.WARP)
	rg.POST("/templates/warp/register", h.WARPRegister)
}

func (h *Handler) GetSystemStatus(c *gin.Context) {
	stats := h.svc.GetStatus()
	c.JSON(http.StatusOK, stats)
}

// WARP returns a Mihomo WireGuard fragment for Cloudflare WARP (v1.3.6).
func (h *Handler) WARP(c *gin.Context) {
	var body struct {
		PrivateKey string `json:"private_key"`
		Address    string `json:"address"`
		IPv6       string `json:"ipv6"`
		Reserved   string `json:"reserved"`
	}
	_ = c.ShouldBindJSON(&body)
	ipv4, ipv6 := body.Address, body.IPv6
	if i := strings.IndexByte(ipv4, ','); i >= 0 {
		if ipv6 == "" {
			ipv6 = strings.TrimSpace(ipv4[i+1:])
		}
		ipv4 = strings.TrimSpace(ipv4[:i])
	}
	reserved := decodeWARPClientID(body.Reserved)
	yaml, err := WARPTemplate(body.PrivateKey, ipv4, ipv6, reserved)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"yaml": yaml})
}

func (h *Handler) WARPRegister(c *gin.Context) {
	res, err := RegisterWARP()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	mode := strings.TrimSpace(c.Query("mode"))
	switch mode {
	case "", "wireguard":
		c.JSON(http.StatusOK, gin.H{
			"yaml":        res.YAML,
			"masque_yaml": res.MasqueYAML,
			"address":     res.Address,
			"ipv6":        res.IPv6,
			"reserved":    res.Reserved,
		})
	case "masque":
		c.JSON(http.StatusOK, gin.H{
			"yaml":    res.MasqueYAML,
			"address": res.Address,
			"ipv6":    res.IPv6,
		})
	case "both":
		c.JSON(http.StatusOK, res)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid mode %q: must be wireguard, masque, or both", mode)})
	}
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
	// gin's default MultipartMemory is 32 MiB — files larger than that spill to
	// os.TempDir(). Bump it to match the request body cap so a 100 MiB backup
	// stays in memory (avoiding the disk-spill failure mode where TempDir is
	// not writable or full). 128 MiB peak RAM is acceptable for an admin-only
	// restore endpoint that already requires authentication.
	_ = c.Request.ParseMultipartForm(maxRestoreDatabaseBytes)
	file, err := c.FormFile("database")
	if err != nil {
		// Surface the actual underlying error so the operator can distinguish
		// "no file selected" / "body too large" / "multipart parse error".
		// Pre-fix this returned a generic message that masked the real cause.
		errMsg := err.Error()
		if strings.Contains(errMsg, "request body too large") {
			errMsg = "upload exceeds 128 MiB limit"
		} else if strings.Contains(errMsg, "missing boundary") {
			errMsg = "invalid upload (missing multipart boundary); refresh the page and retry"
		} else if strings.Contains(errMsg, "EOF") {
			errMsg = "upload was empty or interrupted"
		}
		log.Printf("system restore-database: FormFile error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
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
	result, err := RestoreDatabase(h.dbPath, h.mihomoCfg, f)
	if err != nil {
		log.Printf("system restore-database failed: %v", err)
		// Distinguish client errors (bad upload / not SQLite / corrupt zip)
		// from server errors (disk full / permission denied). Client errors
		// return 400 so the operator knows to fix the file, not the server.
		errMsg := err.Error()
		status := http.StatusInternalServerError
		if strings.Contains(errMsg, "not a valid SQLite database") ||
			strings.Contains(errMsg, "does not contain a database") ||
			strings.Contains(errMsg, "extract from zip") ||
			strings.Contains(errMsg, "empty or does not contain") ||
			strings.Contains(errMsg, "database path is empty") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": errMsg})
		return
	}
	log.Printf("[WARNING] Database restored from backup. Panel will exit now so systemd restarts it with the new DB. mihomo_config_skipped=%v", result.MihomoSkipped)

	resp := gin.H{
		"status":           "ok",
		"restart_required": true,
		"message":          "Database restored. The panel is exiting now so systemd (Restart=always) brings it back with the new DB. The browser will auto-reload when the panel is back.",
		"path":             filepath.Base(h.dbPath),
		"mihomo_config":    "",
	}
	if result.MihomoConfigPath != "" {
		resp["mihomo_config"] = result.MihomoConfigPath
	} else if result.MihomoSkipped {
		resp["mihomo_config"] = "skipped (panel has no mihomo.config path)"
	}

	// Flush the response, then exit so systemd's Restart=always reboots the
	// panel with the freshly-restored DB. Without this, the running GORM
	// pool keeps writing to the old (now-unlinked) SQLite inode and every
	// write is silently lost.
	c.JSON(http.StatusOK, resp)
	go func() {
		time.Sleep(500 * time.Millisecond) // let gin flush
		log.Printf("[WARNING] Panel exiting for post-restore restart.")
		os.Exit(0)
	}()
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
	cmd, err := ACMECommand(req.Domain, req.Email, req.Webroot)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"command": cmd})
}

// UpdateGeoFiles downloads MetaCubeX GeoIP/GeoSite databases next to the Mihomo config .
func (h *Handler) UpdateGeoFiles(c *gin.Context) {
	dir := filepath.Dir(h.mihomoCfg)
	if h.mihomoCfg == "" {
		dir = "/var/lib/3m-ui/mihomo"
	}
	result, err := UpdateGeoFiles(dir)
	if err != nil {
		log.Printf("system update-geofiles failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"dir": dir, "files": result})
}

func (h *Handler) backupDir() string {
	return backupsDirFromDB(h.dbPath)
}

func (h *Handler) ListLocalBackups(c *gin.Context) {
	items, total, err := listLocalBackups(h.backupDir())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total_bytes": total, "dir": h.backupDir()})
}

func (h *Handler) DeleteLocalBackup(c *gin.Context) {
	name := c.Param("name")
	if err := deleteLocalBackup(h.backupDir(), name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": name})
}

func (h *Handler) CleanupLocalBackups(c *gin.Context) {
	var body struct {
		Keep          int `json:"keep"`
		OlderThanDays int `json:"older_than_days"`
	}
	_ = c.ShouldBindJSON(&body)
	deleted, kept, freed, err := cleanupLocalBackups(h.backupDir(), body.Keep, body.OlderThanDays)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"deleted":       deleted,
		"deleted_count": len(deleted),
		"kept":          kept,
		"freed_bytes":   freed,
	})
}
