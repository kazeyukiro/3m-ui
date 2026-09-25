package system

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/buildinfo"
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
	rg.POST("/restart", h.RestartPanel)
	rg.GET("/update-info", h.UpdateInfo)
	rg.POST("/update", h.RunUpdate)
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

// RestartPanel triggers a graceful panel restart. The handler responds 200
// first, then os.Exit(0) after a short delay so systemd's Restart=always
// brings the panel back. This is the same pattern used by RestoreDatabase.
func (h *Handler) RestartPanel(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Panel is restarting now. This page will auto-reload when it is back.",
	})
	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Printf("[INFO] Panel restart requested via web UI. Exiting.")
		os.Exit(0)
	}()
}

// UpdateInfo checks GitHub for the latest release and compares it to the
// running version. Returns current_version, latest_version, update_available.
// UpdateInfo checks GitHub for the latest release (stable and pre) and
// compares it to the running version. Returns current_version,
// current_channel, latest_stable, latest_pre, and update_available flags
// for both channels so the UI can offer switching between stable and pre.
func (h *Handler) UpdateInfo(c *gin.Context) {
	current := buildinfo.Version
	if current == "" || current == "dev" {
		current = "dev"
	}

	// Detect current channel: read the CHANNEL file alongside the binary.
	binPath, _ := os.Executable()
	baseDir := filepath.Dir(binPath)
	channelFile := filepath.Join(baseDir, "CHANNEL")
	currentChannel := "stable"
	if data, err := os.ReadFile(channelFile); err == nil {
		ch := strings.TrimSpace(string(data))
		if ch == "pre" || ch == "prerelease" {
			currentChannel = "pre"
		}
	}
	// If current version is "pre" or starts with a pre tag, detect from version too.
	if current == "pre" || strings.Contains(current, "-rc") || strings.Contains(current, "-pre") {
		currentChannel = "pre"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	// Query BOTH the stable latest release AND the pre tag simultaneously.
	type releaseInfo struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	fetchRelease := func(url string) (*releaseInfo, error) {
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		var r releaseInfo
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return nil, err
		}
		return &r, nil
	}

	stableCh := make(chan *releaseInfo, 1)
	preCh := make(chan *releaseInfo, 1)
	stableErrCh := make(chan error, 1)
	preErrCh := make(chan error, 1)

	go func() {
		r, err := fetchRelease("https://api.github.com/repos/kazeyukiro/3m-ui/releases/latest")
		if err != nil {
			stableErrCh <- err
			return
		}
		stableCh <- r
	}()
	go func() {
		r, err := fetchRelease("https://api.github.com/repos/kazeyukiro/3m-ui/releases/tags/pre")
		if err != nil {
			preErrCh <- err
			return
		}
		preCh <- r
	}()

	var stable, pre *releaseInfo
	var stableErr, preErr error
	for i := 0; i < 2; i++ {
		select {
		case r := <-stableCh:
			stable = r
		case e := <-stableErrCh:
			stableErr = e
		case r := <-preCh:
			pre = r
		case e := <-preErrCh:
			preErr = e
		case <-ctx.Done():
			stableErr = fmt.Errorf("timeout")
			preErr = fmt.Errorf("timeout")
			break
		}
	}

	// Determine the "target" channel — the one the user is NOT currently on.
	// latest_version + update_available reflect the target channel so the
	// UI can show "switch to pre" or "switch to stable" appropriately.
	targetChannel := "pre"
	if currentChannel == "pre" {
		targetChannel = "stable"
	}

	var latestTag, latestNotes, latestURL string
	if targetChannel == "stable" && stable != nil {
		latestTag = stable.TagName
		latestNotes = stable.Body
		latestURL = stable.HTMLURL
	} else if targetChannel == "pre" && pre != nil {
		latestTag = pre.TagName
		latestNotes = pre.Body
		latestURL = pre.HTMLURL
	}

	updateAvailable := false
	if current != "dev" && latestTag != "" {
		cur := strings.TrimPrefix(current, "v")
		lat := strings.TrimPrefix(latestTag, "v")
		updateAvailable = cur != lat
	}

	// Also include the other channel's version for display.
	var stableTag, preTag string
	if stable != nil {
		stableTag = stable.TagName
	}
	if pre != nil {
		preTag = pre.TagName
	}

	errMsg := ""
	if stableErr != nil && preErr != nil {
		errMsg = "cannot reach GitHub API (both stable and pre queries failed)"
	}

	c.JSON(http.StatusOK, gin.H{
		"current_version":  current,
		"current_channel":  currentChannel,
		"target_channel":   targetChannel,
		"latest_version":   latestTag,
		"latest_stable":    stableTag,
		"latest_pre":       preTag,
		"update_available": updateAvailable,
		"release_url":      latestURL,
		"release_notes":    latestNotes,
		"error":            errMsg,
	})
}

// startDetachedUpdate launches update.sh outside the panel service cgroup so
// "systemctl stop 3m-ui" does not kill the updater before it can start again.
func startDetachedUpdate(updateScript string, args []string, env []string) error {
	if _, err := exec.LookPath("systemd-run"); err == nil {
		unit := fmt.Sprintf("3m-ui-update-%d", time.Now().UnixNano())
		sr := []string{
			"--no-block",
			"--collect",
			"--unit=" + unit,
			"--description=3m-ui panel self-update",
		}
		for _, e := range env {
			if e == "" {
				continue
			}
			// Only pass through relevant vars (avoid huge Environ() blow-up of secrets if any).
			if strings.HasPrefix(e, "THREE_M_UI_") || strings.HasPrefix(e, "PATH=") ||
				strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "LANG=") {
				sr = append(sr, "--setenv="+e)
			}
		}
		sr = append(sr, updateScript)
		sr = append(sr, args...)
		out, err := exec.Command("systemd-run", sr...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemd-run: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		log.Printf("system update: scheduled via systemd-run unit %s", unit)
		return nil
	}

	cmd := exec.Command(updateScript, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Detach so the panel process exiting does not wait on / reap in a way that
	// confuses the child on non-systemd hosts.
	_ = cmd.Process.Release()
	log.Printf("system update: started via setsid (no systemd-run)")
	return nil
}

func (h *Handler) RunUpdate(c *gin.Context) {
	var body struct {
		Channel string `json:"channel"`
	}
	_ = c.ShouldBindJSON(&body)
	channel := strings.TrimSpace(body.Channel)
	if channel != "stable" && channel != "pre" {
		channel = ""
	}

	binPath, _ := os.Executable()
	baseDir := filepath.Dir(binPath)
	updateScript := filepath.Join(baseDir, "update.sh")

	if _, err := os.Stat(updateScript); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "update.sh not found at " + updateScript + ". Update via SSH: 3m-ui update",
		})
		return
	}

	// Build the command. update.sh accepts a positional argument for the
	// version tag. "pre" selects the pre channel; no arg = latest stable.
	args := []string{}
	env := append(os.Environ(), "THREE_M_UI_UPDATE_SILENT=1")
	if channel == "pre" {
		args = append(args, "pre")
		env = append(env, "THREE_M_UI_CHANNEL=pre")
	} else if channel == "stable" {
		env = append(env, "THREE_M_UI_CHANNEL=stable")
	}

	// Must not remain in the 3m-ui.service cgroup: install.sh stops the service,
	// and systemd would kill this updater mid-flight. Prefer a transient systemd
	// unit; fall back to setsid + process release.
	env = append(env, "THREE_M_UI_UPDATE_ESCAPED=1")
	if err := startDetachedUpdate(updateScript, args, env); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to start update: " + err.Error(),
		})
		return
	}

	msg := "Update started outside the panel process. The service will stop, upgrade, then start again automatically. This page will auto-reload."
	if channel == "pre" {
		msg = "Switching to pre-release channel. The panel will download the latest pre build and restart."
	} else if channel == "stable" {
		msg = "Switching to stable channel. The panel will download the latest stable release and restart."
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"channel": channel,
		"message": msg,
	})
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
