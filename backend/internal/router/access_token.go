package router

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

func RegisterAccessTokenRoutes(rg *gin.RouterGroup, d Deps) {
	h := &AccessTokenHandler{db: d.DB, cfg: d.Config}
	rg.GET("", h.List)
	rg.POST("", h.Create)
	rg.PUT("/:id", h.Update)
	rg.DELETE("/:id", h.Delete)
}

type AccessTokenHandler struct {
	db  *gorm.DB
	cfg *config.Config
}
type accessTokenCreateInput struct {
	Name       string     `json:"name"`
	ListenerID uint       `json:"listener_id" binding:"required"`
	ExpireAt   *time.Time `json:"expire_at"`
}
type accessTokenUpdateInput struct {
	Name     *string    `json:"name"`
	Enabled  *bool      `json:"enabled"`
	ExpireAt *time.Time `json:"expire_at"`
}
type accessTokenResponse struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Token      string     `json:"token,omitempty"`
	Enabled    bool       `json:"enabled"`
	ExpireAt   *time.Time `json:"expire_at"`
	ListenerID uint       `json:"listener_id"`
}

func (h *AccessTokenHandler) List(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
		return
	}
	var tokens []models.AccessToken
	if err := h.db.Order("id desc").Find(&tokens).Error; err != nil {
		log.Printf("access-token list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	out := make([]accessTokenResponse, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, accessTokenResponse{ID: t.ID, Name: t.Name, Enabled: t.Enabled, ExpireAt: t.ExpireAt, ListenerID: t.ListenerID})
	}
	c.JSON(http.StatusOK, out)
}
func (h *AccessTokenHandler) Create(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
		return
	}
	var in accessTokenCreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if in.ListenerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener_id is required"})
		return
	}
	if in.ExpireAt != nil && !in.ExpireAt.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expire_at must be in the future"})
		return
	}
	var l models.Listener
	if err := h.db.First(&l, in.ListenerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "listener not found"})
		} else {
			log.Printf("access-token create lookup listener failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	if !l.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener is disabled"})
		return
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate secure token"})
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = l.Name
	}
	token := models.AccessToken{Name: name, Token: hex.EncodeToString(buf), Enabled: true, ExpireAt: in.ExpireAt, ListenerID: l.ID}
	if err := h.db.Create(&token).Error; err != nil {
		log.Printf("access-token create save failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": token.ID, "name": token.Name, "token": token.Token, "enabled": token.Enabled, "expire_at": token.ExpireAt, "listener_id": token.ListenerID, "mihomo_link": converter.GetSubscriptionURL(h.cfg, c.Request, token.Token, "mihomo"), "clash_link": converter.GetSubscriptionURL(h.cfg, c.Request, token.Token, "clash"), "singbox_link": converter.GetSubscriptionURL(h.cfg, c.Request, token.Token, "singbox"), "shadowrocket_link": converter.GetSubscriptionURL(h.cfg, c.Request, token.Token, "shadowrocket")})
}
func (h *AccessTokenHandler) Update(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var in accessTokenUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var t models.AccessToken
	if err := h.db.First(&t, uint(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "access token not found"})
		} else {
			log.Printf("access-token update lookup failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	updates := map[string]interface{}{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
			return
		}
		updates["name"] = name
		t.Name = name
	}
	if in.Enabled != nil {
		updates["enabled"] = *in.Enabled
		t.Enabled = *in.Enabled
	}
	if in.ExpireAt != nil {
		if !in.ExpireAt.After(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expire_at must be in the future"})
			return
		}
		updates["expire_at"] = in.ExpireAt
		t.ExpireAt = in.ExpireAt
	}
	if len(updates) > 0 {
		if err := h.db.Model(&t).Updates(updates).Error; err != nil {
			log.Printf("access-token update save failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"id": t.ID, "name": t.Name, "enabled": t.Enabled, "expire_at": t.ExpireAt, "listener_id": t.ListenerID})
}
func (h *AccessTokenHandler) Delete(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	result := h.db.Delete(&models.AccessToken{}, uint(id))
	if result.Error != nil {
		log.Printf("access-token delete failed: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "access token not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
