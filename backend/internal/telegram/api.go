package telegram

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db: db} }
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/settings", h.GetSettings)
	rg.PUT("/settings", h.PutSettings)
	rg.POST("/test", h.Test)
}

func (h *Handler) GetSettings(c *gin.Context) {
	s, err := LoadSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := s
	if out.BotToken != "" {
		out.BotToken = maskToken(out.BotToken)
	}
	c.JSON(http.StatusOK, out)
}

type putSettingsBody struct {
	Enabled           bool     `json:"enabled"`
	BotToken          string   `json:"bot_token"`
	ChatIDs           []string `json:"chat_ids"`
	NotifyOnBlock     bool     `json:"notify_on_block"`
	NotifyOnUnblock   bool     `json:"notify_on_unblock"`
	NotifyOnExpiry    bool     `json:"notify_on_expiry"`
	NotifyOnTraffic   bool     `json:"notify_on_traffic"`
	NotifyDailyDigest bool     `json:"notify_daily_digest"`
	TrafficWarnPct    int      `json:"traffic_warn_pct"`
	ExpiryWarnHours   int      `json:"expiry_warn_hours"`
	KeepToken         bool     `json:"keep_token"`
}

func (h *Handler) PutSettings(c *gin.Context) {
	var body putSettingsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	current, _ := LoadSettings(h.db)
	s := Settings{
		Enabled: body.Enabled, BotToken: strings.TrimSpace(body.BotToken), ChatIDs: body.ChatIDs,
		NotifyOnBlock: body.NotifyOnBlock, NotifyOnUnblock: body.NotifyOnUnblock,
		NotifyOnExpiry: body.NotifyOnExpiry, NotifyOnTraffic: body.NotifyOnTraffic,
		NotifyDailyDigest: body.NotifyDailyDigest,
		TrafficWarnPct: body.TrafficWarnPct, ExpiryWarnHours: body.ExpiryWarnHours,
	}
	if body.KeepToken || s.BotToken == "" || strings.Contains(s.BotToken, "…") || strings.Contains(s.BotToken, "...") {
		s.BotToken = current.BotToken
	}
	if s.Enabled && s.BotToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram bot token is required when notifications are enabled"})
		return
	}
	if s.Enabled && len(s.ChatIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one Telegram Chat ID is required when notifications are enabled"})
		return
	}
	if err := SaveSettings(h.db, s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := s
	if out.BotToken != "" {
		out.BotToken = maskToken(out.BotToken)
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Test(c *gin.Context) {
	client, _, err := NewClientFromDB(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram is disabled or incomplete (token + chat IDs required)"})
		return
	}
	if err := client.Validate(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := client.SendText("🔔 <b>3m-ui</b> Telegram 测试消息 / test message — 连接正常 / connection OK."); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func maskToken(token string) string {
	if len(token) <= 10 {
		return "********"
	}
	return token[:6] + "…" + token[len(token)-4:]
}
