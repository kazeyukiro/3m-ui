package user

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
)

// ExportLinks returns subscription URLs for all (or filtered) proxy users.
// One-click export of share links for ops.
func (h *Handler) ExportLinks(c *gin.Context) {
	f := ListFilter{Query: strings.TrimSpace(c.Query("q"))}
	if g := strings.TrimSpace(c.Query("group")); g != "" {
		f.Group = g
	}
	users, err := h.svc.ListFiltered(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cfg := config.GlobalConfig
	out := make([]gin.H, 0, len(users))
	for _, u := range users {
		item := gin.H{
			"id":       u.ID,
			"username": u.Username,
			"remark":   u.Remark,
			"group":    u.Group,
			"tags":     u.Tags,
			"enabled":  u.Enabled,
		}
		if tok := strings.TrimSpace(u.SubToken); tok != "" {
			item["subscription"] = converter.GetSubscriptionURL(cfg, c.Request, tok, "")
			item["subscription_clash"] = converter.GetSubscriptionURL(cfg, c.Request, tok, "clash")
			item["subscription_v2ray"] = converter.GetSubscriptionURL(cfg, c.Request, tok, "v2ray")
			item["subscription_singbox"] = converter.GetSubscriptionURL(cfg, c.Request, tok, "singbox")
			item["sub_token"] = tok
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "count": len(out)})
}
