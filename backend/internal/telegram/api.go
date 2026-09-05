package telegram

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
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
	rg.POST("/set-my-commands", h.SetMyCommands)
	rg.GET("/bot-info", h.BotInfo)
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

// flexInt accepts JSON numbers or numeric strings (Ant Design Input type=number).
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			*f = 0
			return nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		*f = flexInt(n)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}

type putSettingsBody struct {
	Enabled           bool     `json:"enabled"`
	BotToken          string   `json:"bot_token"`
	ChatIDs           []string `json:"chat_ids"`
	NotifyOnLogin     bool     `json:"notify_on_login"`
	NotifyOnBlock     bool     `json:"notify_on_block"`
	NotifyOnUnblock   bool     `json:"notify_on_unblock"`
	NotifyOnExpiry    bool     `json:"notify_on_expiry"`
	NotifyOnTraffic   bool     `json:"notify_on_traffic"`
	NotifyDailyDigest bool     `json:"notify_daily_digest"`
	NotifyOnCPU       bool     `json:"notify_on_cpu"`
	TrafficWarnPct    flexInt  `json:"traffic_warn_pct"`
	ExpiryWarnHours   flexInt  `json:"expiry_warn_hours"`
	CPUWarnPct        flexInt  `json:"cpu_warn_pct"`
	Schedule          string   `json:"schedule"`
	AttachBackup      bool     `json:"attach_backup"`
	Language          string   `json:"language"`
	EnabledEvents     string   `json:"enabled_events"`
	ExpiryWarnDays    flexInt  `json:"expiry_warn_days"`
	TrafficWarnGB     flexInt  `json:"traffic_warn_gb"`
	ProxyURL          string   `json:"proxy_url"`
	APIServer         string   `json:"api_server"`
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
		NotifyOnLogin: body.NotifyOnLogin,
		NotifyOnBlock: body.NotifyOnBlock, NotifyOnUnblock: body.NotifyOnUnblock,
		NotifyOnExpiry: body.NotifyOnExpiry, NotifyOnTraffic: body.NotifyOnTraffic,
		NotifyDailyDigest: body.NotifyDailyDigest,
		NotifyOnCPU:       body.NotifyOnCPU,
		TrafficWarnPct:    int(body.TrafficWarnPct), ExpiryWarnHours: int(body.ExpiryWarnHours),
		CPUWarnPct:     int(body.CPUWarnPct),
		Schedule:       strings.TrimSpace(body.Schedule),
		AttachBackup:   body.AttachBackup,
		Language:       strings.TrimSpace(body.Language),
		EnabledEvents:  strings.TrimSpace(body.EnabledEvents),
		ExpiryWarnDays: int(body.ExpiryWarnDays),
		TrafficWarnGB:  int(body.TrafficWarnGB),
		ProxyURL:       strings.TrimSpace(body.ProxyURL),
		APIServer:      strings.TrimSpace(body.APIServer),
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

// defaultBotCommands returns the command menu registered with Telegram via setMyCommands.
// Descriptions are bilingual (zh/en) to match the bot's existing reply style.
func defaultBotCommands() []BotCommand {
	return []BotCommand{
		{Command: "start", Description: "开始 / Start & help"},
		{Command: "help", Description: "查看可用命令 / Show commands"},
		{Command: "status", Description: "面板与核心概览 / Panel & core overview"},
		{Command: "id", Description: "显示当前 Telegram Chat ID / Show chat ID"},
		{Command: "usage", Description: "查询当前账号用量 / Show my usage"},
		{Command: "users", Description: "代理用户列表 / List proxy users"},
		{Command: "online", Description: "当前在线用户 / Online users"},
		{Command: "listeners", Description: "入站节点列表 / Inbound listeners"},
		{Command: "traffic", Description: "流量快照 / Traffic snapshot"},
		{Command: "restart", Description: "重启 Mihomo 核心 / Restart core"},
		{Command: "backup", Description: "下载/发送备份 / Backup"},
		{Command: "search", Description: "按用户名/备注搜索 / Search users"},
	}
}

// SetMyCommands registers the 3m-ui command menu with Telegram via setMyCommands.
func (h *Handler) SetMyCommands(c *gin.Context) {
	s, err := LoadSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(s.BotToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram bot token is not configured"})
		return
	}
	client := NewClient(s.BotToken, s.ChatIDs, s.ProxyURL, s.APIServer)
	resp, err := client.SetMyCommands(defaultBotCommands())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// BotInfo calls Telegram getMe and returns {username, first_name, id}.
func (h *Handler) BotInfo(c *gin.Context) {
	s, err := LoadSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(s.BotToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram bot token is not configured"})
		return
	}
	client := NewClient(s.BotToken, s.ChatIDs, s.ProxyURL, s.APIServer)
	info, err := client.GetMe()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}
