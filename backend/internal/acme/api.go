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
	// Never echo DNS API tokens to the browser.
	out := s
	if out.DNSToken != "" {
		out.DNSToken = ""
		// Client uses has_dns_token from /ssl/status; keep shape stable.
	}
	c.JSON(http.StatusOK, out)
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
		// Strip accidental path or port from domain field.
		in.Domain = in.Domain[:i]
	}
	if in.Enabled {
		if in.CertFile == "" && in.Domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "domain or public IP is required when Let's Encrypt is enabled (or provide cert_file + key_file)"})
			return
		}
		if (in.CertFile == "") != (in.KeyFile == "") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cert_file and key_file must be set together"})
			return
		}
		if NeedsDNS01(in) && in.CertFile == "" {
			if IsIPHost(in.Domain) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "DNS-01 cannot be used with an IP address"})
				return
			}
			prev, _ := LoadSettings(h.db)
			if strings.TrimSpace(in.DNSToken) == "" {
				in.DNSToken = prev.DNSToken
			}
			if strings.TrimSpace(in.DNSToken) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "dns_token is required for DNS-01 / wildcard certificates (Cloudflare API token with Zone.DNS Edit)"})
				return
			}
			if strings.TrimSpace(in.DNSProvider) == "" {
				in.DNSProvider = "cloudflare"
			}
		}
	}
	// Never clear a stored DNS token when the client sends an empty field.
	if strings.TrimSpace(in.DNSToken) == "" {
		if prev, err := LoadSettings(h.db); err == nil && prev.DNSToken != "" {
			in.DNSToken = prev.DNSToken
		}
	}
	if err := SaveSettings(h.db, in); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"message":  "SSL settings saved. Restart the panel process for ListenTLS / acmez changes to take effect.",
		"settings": in,
	})
}
