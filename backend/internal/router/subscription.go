package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

// RegisterPublicSubscriptionRoutes mounts token-authenticated subscription
// endpoints before the administrator auth middleware. Possession of the
// high-entropy access token is the credential for these endpoints.
func RegisterPublicSubscriptionRoutes(api *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	api.GET("/client/sub/:token", func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
			return
		}

		var token models.AccessToken
		if err := db.Where("token = ? AND enabled = ?", c.Param("token"), true).First(&token).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
			return
		}
		if token.ExpireAt != nil && !token.ExpireAt.After(time.Now()) {
			c.JSON(http.StatusGone, gin.H{"error": "subscription expired"})
			return
		}

		raw, err := converter.GenerateRawConfig(db, token, c.Request)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}

		target := strings.ToLower(strings.TrimSpace(c.Query("target")))
		if target == "" {
			target = detectSubTarget(c.GetHeader("User-Agent"))
		}
		if c.Query("raw") == "true" || target == "" || target == "mihomo" || target == "clash" || target == "meta" {
			c.Header("Cache-Control", "no-store")
			c.Header("Profile-Update-Interval", "24")
			c.Header("Content-Disposition", "attachment; filename=3m-ui.yaml")
			c.Data(http.StatusOK, "text/yaml; charset=utf-8", raw)
			return
		}

		converted, err := converter.CallSubconverterWithRequest(cfg, c.Param("token"), target, raw)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", converted)
	})
}

// detectSubTarget maps common client User-Agents to subscription formats.
func detectSubTarget(ua string) string {
	u := strings.ToLower(ua)
	switch {
	case strings.Contains(u, "clash") || strings.Contains(u, "mihomo") || strings.Contains(u, "stash") || strings.Contains(u, "meta"):
		return "mihomo"
	case strings.Contains(u, "surge"):
		return "surge"
	case strings.Contains(u, "quantumult"):
		return "quanx"
	case strings.Contains(u, "loon"):
		return "loon"
	case strings.Contains(u, "shadowrocket"):
		return "mihomo"
	default:
		return "mihomo"
	}
}
