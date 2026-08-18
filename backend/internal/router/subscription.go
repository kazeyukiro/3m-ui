package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

func RegisterPublicSubscriptionRoutes(api *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	api.GET("/client/sub/:token", func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
			return
		}
		tok := strings.TrimSpace(c.Param("token"))
		if tok == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
			return
		}
		if strings.EqualFold(c.Query("format"), "info") {
			writeSubInfo(c, db, tok)
			return
		}

		var raw []byte
		var err error

		var access models.AccessToken
		if err = db.Where("token = ? AND enabled = ?", tok, true).First(&access).Error; err == nil {
			if access.ExpireAt != nil && !access.ExpireAt.After(time.Now()) {
				c.JSON(http.StatusGone, gin.H{"error": "subscription expired"})
				return
			}
			raw, err = converter.GenerateRawConfig(db, access, c.Request)
		} else if err == gorm.ErrRecordNotFound {
			var pu models.ProxyUser
			if err = db.Where("sub_token = ?", tok).First(&pu).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
				return
			}
			if !user.IsCredentialActive(pu) {
				c.JSON(http.StatusForbidden, gin.H{"error": "user is not active"})
				return
			}
			raw, err = converter.GenerateUserRawConfig(db, pu, c.Request)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
			return
		}
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

		converted, err := converter.CallSubconverterWithRequest(cfg, tok, target, raw)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", converted)
	})
}

func writeSubInfo(c *gin.Context, db *gorm.DB, tok string) {
	var pu models.ProxyUser
	if err := db.Where("sub_token = ?", tok).First(&pu).Error; err == nil {
		if !user.IsCredentialActive(pu) {
			c.JSON(http.StatusForbidden, gin.H{"error": "user is not active"})
			return
		}
		expire := ""
		if !pu.ExpireTime.IsZero() {
			expire = pu.ExpireTime.UTC().Format(time.RFC3339)
		}
		c.JSON(http.StatusOK, gin.H{
			"username":       pu.Username,
			"enabled":        pu.Enabled,
			"blocked":        false,
			"online":         pu.Online,
			"traffic_used":   pu.TrafficUsed,
			"traffic_limit":  pu.TrafficLimit,
			"upload_bytes":   pu.UploadBytes,
			"download_bytes": pu.DownloadBytes,
			"expire_time":    expire,
			"ip_limit":       pu.IPLimit,
		})
		return
	}
	var access models.AccessToken
	if err := db.Where("token = ? AND enabled = ?", tok, true).First(&access).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}
	if access.ExpireAt != nil && !access.ExpireAt.After(time.Now()) {
		c.JSON(http.StatusGone, gin.H{"error": "subscription expired"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"name":        access.Name,
		"enabled":     access.Enabled,
		"listener_id": access.ListenerID,
		"expire_at":   access.ExpireAt,
	})
}

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
