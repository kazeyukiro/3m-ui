package node

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mui"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
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

// RegisterClientRoutes registers only routes that are unique to the node
// handler. The listener handler already owns the shared CRUD/reload routes.
func (h *Handler) RegisterClientRoutes(rg *gin.RouterGroup) {
	rg.GET("/:id/uri", h.ExportNodeURI)
	rg.POST("/:id/client-access", h.CreateClientAccess)
}

func (h *Handler) ListNodes(c *gin.Context) {
	list, err := h.svc.GetAll()
	if err != nil {
		log.Printf("node list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
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
	if err := h.svc.Create(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "listener not found"})
		return
	}
	c.JSON(http.StatusOK, l)
}

func (h *Handler) isTrustedHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}
	return strings.EqualFold(host, "localhost")
}

// ExportNodeURI returns share links (m-ui style: uri / uris / qr / client_yaml).
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
	ap := protocol.LoadAccessProfile(h.db)
	if ap.PublicHost != "" {
		publicURL = ap.PublicHost
	}
	// Apply global access-profile fields as fallbacks when the per-listener
	// values are empty. Per-listener values always take precedence so admins
	// can still override a panel-wide default for an individual node.
	if ap.PublicPort != "" && strings.TrimSpace(listener.PublicPort) == "" {
		listener.PublicPort = ap.PublicPort
	}
	if ap.SNI != "" && strings.TrimSpace(listener.AccessSNI) == "" {
		listener.AccessSNI = ap.SNI
	}
	if ap.ClientFingerprint != "" && strings.TrimSpace(listener.ClientFingerprint) == "" {
		listener.ClientFingerprint = ap.ClientFingerprint
	}
	if len(ap.ALPN) > 0 && strings.TrimSpace(listener.AccessALPN) == "" {
		listener.AccessALPN = strings.Join(ap.ALPN, ",")
	}
	if host := strings.TrimSpace(listener.PublicHost); host != "" {
		publicURL = host
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host != "" && !h.isTrustedHost(host) {
		host = ""
	}
	if host == "" {
		host = c.Request.Host
	}
	host = netutil.NormalizeHost(host)
	host = normalizeExportHostPrefer(host, listener.BindAddress, listener.Listen, publicURL)
	host = netutil.NormalizeHost(host)
	// If publicURL was explicitly resolved (config / access profile / listener
	// PublicHost), it is authoritative and must override any client-controlled
	// Host header (X-Forwarded-Host / Request.Host) used above. This prevents
	// share links from advertising an attacker-supplied host.
	if publicURL != "" {
		if u, perr := url.Parse(publicURL); perr == nil && u.Host != "" {
			host = netutil.NormalizeHost(u.Host)
		}
	}

	credentials := []user.Credential{}
	if h.user != nil {
		byListener, err := h.user.ActiveCredentialsByListener()
		if err != nil {
			log.Printf("export-node-uri load credentials failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		credentials = byListener[listener.ID]
	}

	// Three-tier share export: m-ui protocol port -> 3m-ui registry -> legacy URIs.
	//
	// Follow-up (P3-9 / X-MUI-2): the three tiers produce inconsistent output
	// shapes (m-ui emits URIs + client YAML from the protocol port; the
	// 3m-ui registry emits its own URI format; the legacy builder emits
	// plain URIs only). When the first tier succeeds the client gets the
	// m-ui shape, otherwise it silently degrades to a different shape for
	// the same listener. Unifying the output contract across tiers is
	// tracked as a separate design task.
	var uris []string
	var clientYAML, primary string
	muiCreds := make([]mui.Cred, 0, len(credentials))
	for _, c := range credentials {
		muiCreds = append(muiCreds, mui.Cred{Username: c.Username, Password: c.Password, UUID: c.UUID})
	}
	if shares, err := mui.BuildShares(*listener, host, muiCreds); err == nil && len(shares) > 0 {
		uris = make([]string, 0, len(shares))
		for _, s := range shares {
			if s.URI != "" {
				uris = append(uris, s.URI)
			}
			if clientYAML == "" && len(s.ClientYAML) > 0 {
				clientYAML = string(s.ClientYAML)
			}
		}
		if len(uris) > 0 {
			primary = uris[0]
		}
	} else {
		creds := make([]protocol.UserCred, 0, len(credentials))
		for _, c := range credentials {
			creds = append(creds, protocol.UserCred{Username: c.Username, Password: c.Password, UUID: c.UUID})
		}
		if shares, err := protocol.ExportShares(*listener, host, creds); err == nil && len(shares) > 0 {
			uris = make([]string, 0, len(shares))
			for _, s := range shares {
				if s.URI != "" {
					uris = append(uris, s.URI)
				}
				if clientYAML == "" && s.ClientYAML != "" {
					clientYAML = s.ClientYAML
				}
			}
			if len(uris) > 0 {
				primary = uris[0]
			}
		} else {
			legacy, lerr := ClientURIsWithCredentials(*listener, host, credentials)
			if lerr != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": lerr.Error()})
				return
			}
			uris = legacy
			if uris == nil {
				uris = []string{}
			}
			if len(uris) > 0 {
				primary = uris[0]
			}
			clientYAML, _ = converter.ExportClientYAML(*listener, host, credentials)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"name":        listener.Name,
		"protocol":    listener.Protocol,
		"uri":         primary,
		"uris":        uris,
		"qr_content":  primary,
		"client_yaml": clientYAML,
		"hint":        emptyURIHint(len(uris), len(credentials)),
		"typed":       true,
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
	if err := h.svc.Update(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
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
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) CreateClientAccess(c *gin.Context) {
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
	var body struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = listener.Name + "-access"
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	db := h.db
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database unavailable"})
		return
	}
	token := models.AccessToken{Name: name, Token: hex.EncodeToString(buf), Enabled: true, ListenerID: listener.ID}
	if err := db.Create(&token).Error; err != nil {
		log.Printf("node client-access create failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
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
