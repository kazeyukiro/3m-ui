package node

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// Handler serves node/listener HTTP endpoints with injected dependencies.
type Handler struct {
	svc  *Service
	user *user.Service
	db   *gorm.DB
}

// NewHandler constructs a node HTTP handler.
func NewHandler(svc *Service, userSvc *user.Service, db *gorm.DB) *Handler {
	return &Handler{svc: svc, user: userSvc, db: db}
}

// RegisterRoutes registers node routes on the provided group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.ListNodes)
	rg.POST("", h.CreateNode)
	rg.GET("/:id", h.GetNode)
	rg.GET("/:id/uri", h.ExportNodeURI)
	rg.PUT("/:id", h.UpdateNode)
	rg.DELETE("/:id", h.DeleteNode)
	rg.POST("/:id/reload", h.ReloadNode)
	rg.POST("/:id/client-access", h.CreateClientAccess)
}

func (h *Handler) ListNodes(c *gin.Context) {
	list, err := h.svc.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) CreateNode(c *gin.Context) {
	var l models.Listener
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ValidateNode(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.Create(&l); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, l)
}

func (h *Handler) GetNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
		return
	}
	l, err := h.svc.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, l)
}

// isTrustedHost validates that the provided host is acceptable for URI export.
// It rejects path traversal, illegal characters, and hosts that do not match
// the configured public_url (if set).
func (h *Handler) isTrustedHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if strings.ContainsAny(host, "/\\?#@") || strings.Contains(host, "..") {
		return false
	}
	if config.GlobalConfig != nil && config.GlobalConfig.Server.PublicURL != "" {
		expected := config.GlobalConfig.Server.PublicURL
		if u, err := url.Parse(expected); err == nil && u.Host != "" {
			expected = u.Host
		}
		if eh, _, err := net.SplitHostPort(expected); err == nil {
			expected = eh
		}
		if !strings.EqualFold(host, expected) {
			return false
		}
	}
	return true
}

// ExportNodeURI returns one URI per active client credential. Public host
// comes from public_url, request Host, or loopback as last resort.
func (h *Handler) ExportNodeURI(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
		return
	}
	listener, err := h.svc.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listener not found"})
		return
	}
	if !listener.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener is disabled"})
		return
	}

	publicURL := ""
	if config.GlobalConfig != nil {
		publicURL = strings.TrimSpace(config.GlobalConfig.Server.PublicURL)
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host != "" && !h.isTrustedHost(host) {
		host = ""
	}
	if host == "" {
		host = c.Request.Host
	}
	if host != "" {
		if parsed, err := url.Parse("//" + host); err == nil && parsed.Hostname() != "" {
			host = parsed.Hostname()
		}
	}
	host = normalizeExportHostPrefer(host, listener.BindAddress, listener.Listen, publicURL)

	credentials := []user.Credential{}
	if h.user != nil {
		byListener, err := h.user.ActiveCredentialsByListener()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load active credentials: " + err.Error()})
			return
		}
		credentials = byListener[listener.ID]
	}

	uris, err := ClientURIsWithCredentials(*listener, strings.TrimSpace(host), credentials)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if uris == nil {
		uris = []string{}
	}
	c.JSON(http.StatusOK, gin.H{
		"name":     listener.Name,
		"protocol": listener.Protocol,
		"uris":     uris,
		"hint":     emptyURIHint(len(uris), len(credentials)),
	})
}

func emptyURIHint(uriCount, credCount int) string {
	if uriCount > 0 {
		return ""
	}
	if credCount == 0 {
		return "no active users bound to this listener; bind users in User Management"
	}
	return "could not build share links for this protocol/config"
}

func (h *Handler) UpdateNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
		return
	}
	var l models.Listener
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	l.ID = uint(id)
	if err := ValidateNode(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.Update(&l); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, l)
}

func (h *Handler) DeleteNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
		return
	}
	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "node deleted"})
}

func (h *Handler) ReloadNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
		return
	}
	if err := h.svc.TriggerReload(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "node reloaded"})
}

func (h *Handler) CreateClientAccess(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid listener id"})
		return
	}
	db := h.db
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
		return
	}
	var listener models.Listener
	if err := db.First(&listener, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listener not found"})
		return
	}
	if !listener.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener is disabled"})
		return
	}
	var existing models.AccessToken
	if err := db.Where("listener_id = ? AND enabled = ?", listener.ID, true).First(&existing).Error; err == nil {
		if existing.ExpireAt == nil || existing.ExpireAt.After(time.Now()) {
			c.JSON(http.StatusOK, h.clientAccessResponse(c, existing))
			return
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate secure token"})
		return
	}
	token := models.AccessToken{Name: listener.Name, Token: hex.EncodeToString(buf), Enabled: true, ListenerID: listener.ID}
	if err := db.Create(&token).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, h.clientAccessResponse(c, token))
}

func (h *Handler) clientAccessResponse(c *gin.Context, token models.AccessToken) gin.H {
	cfg := config.GlobalConfig
	return gin.H{
		"id":                token.ID,
		"name":              token.Name,
		"token":             token.Token,
		"type":              "listener",
		"listener_id":       token.ListenerID,
		"mihomo_link":       converter.GetSubscriptionURL(cfg, c.Request, token.Token, "mihomo"),
		"clash_link":        converter.GetSubscriptionURL(cfg, c.Request, token.Token, "clash"),
		"singbox_link":      converter.GetSubscriptionURL(cfg, c.Request, token.Token, "singbox"),
		"shadowrocket_link": converter.GetSubscriptionURL(cfg, c.Request, token.Token, "shadowrocket"),
	}
}
