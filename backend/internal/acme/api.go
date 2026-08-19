package acme

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/ssl", h.Get)
	rg.PUT("/ssl", h.Put)
	rg.GET("/ssl/status", h.Status)
}

func (h *Handler) Get(c *gin.Context) {
	s, err := LoadSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, Status(h.db))
}

func (h *Handler) Put(c *gin.Context) {
	var in Settings
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in.Domain = strings.TrimSpace(in.Domain)
	in.Email = strings.TrimSpace(in.Email)
	in.Domain = strings.TrimPrefix(strings.TrimPrefix(in.Domain, "https://"), "http://")
	if i := strings.IndexAny(in.Domain, "/:"); i >= 0 {
		in.Domain = in.Domain[:i]
	}
	if in.Enabled {
		if in.CertFile == "" && in.Domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required when Let's Encrypt is enabled (or provide cert_file + key_file)"})
			return
		}
		if (in.CertFile == "") != (in.KeyFile == "") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cert_file and key_file must be set together"})
			return
		}
	}
	if err := SaveSettings(h.db, in); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "SSL settings saved. Restart the panel process for ListenTLS / autocert changes to take effect.",
		"settings": in,
	})
}
