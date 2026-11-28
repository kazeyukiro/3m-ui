package healthcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler exposes the tunnel health monitor settings, live status and a
// one-shot manual probe. Settings are persisted to the database and applied
// to the running monitor immediately.
type Handler struct {
	db      *gorm.DB
	monitor *Monitor
}

func NewHandler(db *gorm.DB, monitor *Monitor) *Handler {
	return &Handler{db: db, monitor: monitor}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/settings", h.GetSettings)
	rg.PUT("/settings", h.PutSettings)
	rg.GET("/status", h.GetStatus)
	rg.POST("/test", h.Test)
}

func (h *Handler) GetSettings(c *gin.Context) {
	s, err := LoadSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) PutSettings(c *gin.Context) {
	var s Settings
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s = s.Normalize()
	if err := SaveSettings(h.db, s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.monitor != nil {
		h.monitor.Apply(s)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "settings": s})
}

// Test runs a single probe with the active (or body-supplied) settings and
// reports the result without triggering a restart. It does not require the
// core to be running, so an operator can validate the URL/proxy before enable.
func (h *Handler) Test(c *gin.Context) {
	s := Settings{}
	bodySupplied := c.Request.Body != nil && c.Request.ContentLength != 0
	if bodySupplied {
		if err := json.NewDecoder(c.Request.Body).Decode(&s); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else if h.monitor != nil {
		s = h.monitor.Settings()
	} else {
		loaded, err := LoadSettings(h.db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		s = loaded
	}
	s = s.Normalize()

	ctx, cancel := context.WithTimeout(c.Request.Context(), s.timeoutDuration()+time.Second)
	defer cancel()
	start := time.Now()
	err := httpProber{}.Probe(ctx, s)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error(), "elapsed_ms": elapsed})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "elapsed_ms": elapsed})
}

func (h *Handler) GetStatus(c *gin.Context) {
	if h.monitor == nil {
		c.JSON(http.StatusOK, Status{})
		return
	}
	c.JSON(http.StatusOK, h.monitor.Status())
}
