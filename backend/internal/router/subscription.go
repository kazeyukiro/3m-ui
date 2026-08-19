package router

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
	"github.com/kazeyukiro/3m-ui/backend/internal/subpage"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

func subscriptionHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		// Browser / ?html=1 → subscription information page (custom template support).
		accept := strings.ToLower(c.GetHeader("Accept"))
		wantsHTML := c.Query("html") == "1" || (strings.HasPrefix(strings.TrimSpace(accept), "text/html") && !strings.Contains(accept, "application/"))

		target := strings.ToLower(strings.TrimSpace(c.Query("target")))
		if target == "" {
			target = detectSubTarget(c.GetHeader("User-Agent"))
		}

		var raw []byte
		var err error
		var pu models.ProxyUser
		var isProxyUser bool

		var access models.AccessToken
		if err = db.Where("token = ? AND enabled = ?", tok, true).First(&access).Error; err == nil {
			if access.ExpireAt != nil && !access.ExpireAt.After(time.Now()) {
				c.JSON(http.StatusGone, gin.H{"error": "subscription expired"})
				return
			}
			// Access-token path currently only has Mihomo YAML export.
			raw, err = converter.GenerateRawConfig(db, access, c.Request)
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			if err = db.Where("sub_token = ?", tok).First(&pu).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
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
			isProxyUser = true
			if wantsHTML {
				writeSubHTML(c, db, cfg, pu, tok)
				return
			}
			// Native v2ray / base64 subscription (3x-ui /sub/ parity).
			if target == "v2ray" || target == "base64" || target == "raw" || target == "uri" {
				raw, err = converter.GenerateUserBase64Subscription(db, pu, c.Request, node.ClientURIsWithCredentials)
				if err != nil {
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
					return
				}
				writeSubHeaders(c, &pu)
				c.Header("Cache-Control", "no-store")
				c.Header("Content-Disposition", "attachment; filename=subscription.txt")
				c.Data(http.StatusOK, "text/plain; charset=utf-8", raw)
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

		if c.Query("raw") == "true" || target == "" || target == "mihomo" || target == "clash" || target == "meta" {
			if isProxyUser {
				writeSubHeaders(c, &pu)
			}
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
		if isProxyUser {
			writeSubHeaders(c, &pu)
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", converted)
	}
}

// writeSubHeaders emits the standard subscription response headers that
// compatible clients (v2rayNG, Hiddify, Clash, …) read for traffic / expiry.
func writeSubHeaders(c *gin.Context, pu *models.ProxyUser) {
	if pu == nil {
		return
	}
	upload := pu.UploadBytes
	download := pu.DownloadBytes
	total := pu.TrafficLimit
	expire := int64(0)
	if !pu.ExpireTime.IsZero() {
		expire = pu.ExpireTime.Unix()
	}
	c.Header("Subscription-Userinfo",
		"upload="+itoa(upload)+"; download="+itoa(download)+"; total="+itoa(total)+"; expire="+itoa(expire))
	c.Header("Profile-Update-Interval", "12")
}

func itoa(n int64) string {
	if n < 0 {
		n = 0
	}
	return strconv.FormatInt(n, 10)
}

func RegisterPublicSubscriptionRoutes(api *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	handler := subscriptionHandler(db, cfg)
	api.GET("/client/sub/:token", handler)
	api.GET("/client/sub/:token/", handler)
}

// RegisterLegacySubscriptionRoutes keeps subscription links generated by
// older releases working after upgrading to the /api/v1 route layout.
func RegisterLegacySubscriptionRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	handler := subscriptionHandler(db, cfg)
	r.GET("/sub/:token", handler)
	r.GET("/sub/:token/", handler)
	r.GET("/api/client/sub/:token", handler)
	r.GET("/api/client/sub/:token/", handler)
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
	// Classic v2ray / Xray clients expect base64 list of share links (3x-ui /sub/ parity).
	case strings.Contains(u, "v2ray") || strings.Contains(u, "v2rayng") || strings.Contains(u, "v2rayn") ||
		strings.Contains(u, "streisand") || strings.Contains(u, "hiddify") || strings.Contains(u, "nekobox") ||
		strings.Contains(u, "nekoray") || strings.Contains(u, "sing-box") || strings.Contains(u, "sfa") ||
		strings.Contains(u, "sfm") || strings.Contains(u, "sfi"):
		return "v2ray"
	default:
		return "mihomo"
	}
}

func writeSubHTML(c *gin.Context, db *gorm.DB, cfg *config.Config, pu models.ProxyUser, tok string) {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xf := c.GetHeader("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	}
	base := scheme + "://" + c.Request.Host + "/api/v1/client/sub/" + tok
	var links []string
	if raw, err := converter.GenerateUserBase64Subscription(db, pu, c.Request, node.ClientURIsWithCredentials); err == nil {
		// Decode is not needed; body is base64 of newline-joined URIs — leave empty for page.
		_ = raw
	}
	html, err := subpage.RenderHTML(db, pu, base, links)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", html)
}
