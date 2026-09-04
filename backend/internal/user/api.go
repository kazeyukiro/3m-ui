package user

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.List)
	rg.POST("", h.Create)
	// Static path must be registered before /:id to avoid being captured as id.
	rg.POST("/del-depleted", h.DeleteDepleted)
	rg.POST("/batch", h.Batch)
	// Telegram binding routes registered before /:id so the static segment is not captured.
	rg.GET("/by-telegram/:tgid", h.GetByTelegram)
	rg.PUT("/:id/telegram", h.BindTelegram)
	rg.DELETE("/:id/telegram", h.UnbindTelegram)
	rg.GET("/:id", h.Get)
	rg.PUT("/:id", h.Update)
	rg.DELETE("/:id", h.Delete)
	rg.POST("/:id/listeners", h.BindListeners)
	rg.GET("/:id/listeners", h.GetListeners)
	rg.POST("/:id/nodes", h.BindListeners)
	rg.POST("/:id/remote-nodes", h.BindRemoteNodes)
	rg.GET("/:id/remote-nodes", h.ListRemoteNodes)
	rg.GET("/:id/nodes", h.GetListeners)
	rg.POST("/:id/reset-traffic", h.ResetTraffic)
	rg.GET("/:id/subscription", h.GetSubscription)
	rg.POST("/:id/subscription/rotate", h.RotateSubscription)
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return 0, false
	}
	return uint(id), true
}

func (h *Handler) List(c *gin.Context) {
	f := ListFilter{Query: strings.TrimSpace(c.Query("q"))}
	if v := c.Query("enabled"); v == "true" || v == "1" {
		b := true
		f.Enabled = &b
	} else if v == "false" || v == "0" {
		b := false
		f.Enabled = &b
	}
	if v := c.Query("online"); v == "true" || v == "1" {
		b := true
		f.Online = &b
	} else if v == "false" || v == "0" {
		b := false
		f.Online = &b
	}
	if v := c.Query("blocked"); v == "true" || v == "1" {
		b := true
		f.Blocked = &b
	} else if v == "false" || v == "0" {
		b := false
		f.Blocked = &b
	}
	users, err := h.svc.ListFiltered(f)
	if err != nil {
		log.Printf("user list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	out := make([]SafeUser, 0, len(users))
	for i := range users {
		out = append(out, ToSafeUser(&users[i]))
	}
	c.JSON(http.StatusOK, out)
}

// Batch runs enable|disable|reset-traffic|delete on multiple users .
func (h *Handler) Batch(c *gin.Context) {
	var req struct {
		Action string `json:"action" binding:"required"`
		IDs    []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	n, err := h.svc.Batch(BatchAction(strings.ToLower(strings.TrimSpace(req.Action))), req.IDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"affected": n, "action": req.Action})
}

func (h *Handler) Create(c *gin.Context) {
	var in CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.Create(in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ToSafeUser(u))
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	u, err := h.svc.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		} else {
			log.Printf("user get failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, ToSafeUser(u))
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in UpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.Update(id, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ToSafeUser(u))
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) BindListeners(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req struct {
		ListenerIDs []uint `json:"listener_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.BindListeners(id, req.ListenerIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "listener_ids": req.ListenerIDs})
}

func (h *Handler) GetListeners(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	list, err := h.svc.GetListeners(id)
	if err != nil {
		log.Printf("user get-listeners failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	type nodeDTO struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Protocol    string `json:"protocol"`
		Port        string `json:"port"`
		BindAddress string `json:"bind_address"`
		Enabled     bool   `json:"enabled"`
		TLS         bool   `json:"tls"`
		UDP         bool   `json:"udp"`
		Status      string `json:"status"`
	}
	out := make([]nodeDTO, 0, len(list))
	for _, n := range list {
		out = append(out, nodeDTO{n.ID, n.Name, n.Protocol, n.Port, n.BindAddress, n.Enabled, n.TLS, n.UDP, n.Status})
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ResetTraffic(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	u, err := h.svc.ResetTraffic(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ToSafeUser(u))
}

var _ = models.Listener{}

func (h *Handler) GetSubscription(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	token, err := h.svc.EnsureSubToken(id)
	if err != nil {
		log.Printf("user ensure-sub-token failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xf := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xf != "" {
		if i := strings.IndexByte(xf, ','); i >= 0 {
			xf = xf[:i]
		}
		xf = strings.ToLower(strings.TrimSpace(xf))
		if xf == "https" || xf == "http" {
			scheme = xf
		}
	}
	base := scheme + "://" + c.Request.Host
	subURL := base + "/api/v1/client/sub/" + url.PathEscape(token)
	c.JSON(http.StatusOK, gin.H{"token": token, "url": subURL})
}

func (h *Handler) RotateSubscription(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	token, err := h.svc.RotateSubToken(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xf := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xf != "" {
		if i := strings.IndexByte(xf, ','); i >= 0 {
			xf = xf[:i]
		}
		xf = strings.ToLower(strings.TrimSpace(xf))
		if xf == "https" || xf == "http" {
			scheme = xf
		}
	}
	base := scheme + "://" + c.Request.Host
	subURL := base + "/api/v1/client/sub/" + url.PathEscape(token)
	c.JSON(http.StatusOK, gin.H{"token": token, "url": subURL})
}

// DeleteDepleted removes expired / over-quota proxy users (remove expired or over-quota users).
func (h *Handler) DeleteDepleted(c *gin.Context) {
	n, err := h.svc.DeleteDepleted()
	if err != nil {
		log.Printf("user delete-depleted failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": n})
}

// GetByTelegram looks up a proxy user by their linked Telegram chat/user ID.
func (h *Handler) GetByTelegram(c *gin.Context) {
	tgid, err := strconv.ParseInt(c.Param("tgid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid telegram id"})
		return
	}
	u, err := h.svc.GetByTelegramID(tgid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		} else {
			log.Printf("user get-by-telegram failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, ToSafeUser(u))
}

// BindTelegram links a Telegram account to an existing proxy user.
func (h *Handler) BindTelegram(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req struct {
		TelegramID   int64  `json:"telegram_id" binding:"required"`
		TelegramName string `json:"telegram_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.BindTelegram(id, req.TelegramID, req.TelegramName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	c.JSON(http.StatusOK, ToSafeUser(u))
}

// UnbindTelegram removes the Telegram account link from a proxy user.
func (h *Handler) UnbindTelegram(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.UnbindTelegram(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	c.JSON(http.StatusOK, ToSafeUser(u))
}


func (h *Handler) BindRemoteNodes(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req struct {
		MirrorIDs []uint `json:"mirror_ids"`
		RemoteNodeIDs []uint `json:"remote_node_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ids := req.MirrorIDs
	if len(ids) == 0 {
		ids = req.RemoteNodeIDs
	}
	if err := h.svc.BindRemoteNodes(id, ids); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "mirror_ids": ids})
}

func (h *Handler) ListRemoteNodes(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	ids, err := h.svc.ListRemoteNodeIDs(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mirror_ids": ids})
}
