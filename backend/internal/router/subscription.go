package router

import (
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
	"github.com/kazeyukiro/3m-ui/backend/internal/subpage"
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
		target := strings.ToLower(strings.TrimSpace(c.Query("target")))
		if target == "" {
			target = detectSubTarget(c.GetHeader("User-Agent"))
		}
		// Built-in subscription info page — never route through external subconverter.
		wantsHTML := c.Query("html") == "1" || target == "html" || target == "page" ||
			(strings.HasPrefix(strings.TrimSpace(accept), "text/html") && !strings.Contains(accept, "application/"))

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
			// Native v2ray / base64 subscription (classic client subscription).
			if target == "v2ray" || target == "base64" || target == "raw" || target == "uri" {
				raw, err = converter.GenerateUserBase64Subscription(db, pu, c.Request, node.ClientURIsWithCredentials)
				if err != nil {
					log.Printf("subscription error: %v", err)
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
					return
				}
				page := subpage.LoadPageSettings(db)
				encrypt := page.Encrypt
				if q := strings.ToLower(c.Query("encrypt")); q == "0" || q == "false" {
					encrypt = false
				} else if q == "1" || q == "true" {
					encrypt = true
				}
				if !encrypt {
					if decoded, decErr := base64.StdEncoding.DecodeString(string(raw)); decErr == nil {
						raw = decoded
					}
				}
				writeSubHeaders(c, db, &pu)
				c.Header("Cache-Control", "no-store")
				c.Header("Content-Disposition", `attachment; filename="`+sanitizeFilename(pu.Username)+`.txt"`)
				c.Data(http.StatusOK, "text/plain; charset=utf-8", raw)
				return
			}
			// Native sing-box JSON outbounds (sing-box subscription).
			if target == "singbox" || target == "sing-box" || target == "sfa" || target == "sfm" {
				raw, err = converter.GenerateUserSingboxSubscription(db, pu, c.Request)
				if err != nil {
					log.Printf("subscription error: %v", err)
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
					return
				}
				writeSubHeaders(c, db, &pu)
				c.Header("Cache-Control", "no-store")
				c.Header("Content-Disposition", `attachment; filename="`+sanitizeFilename(pu.Username)+`.json"`)
				c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
				return
			}
			raw, err = converter.GenerateUserRawConfig(db, pu, c.Request)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
			return
		}
		if err != nil {
			log.Printf("subscription error: %v", err)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}

		if c.Query("raw") == "true" || target == "" || target == "mihomo" || target == "clash" || target == "meta" {
			if isProxyUser {
				writeSubHeaders(c, db, &pu)
			} else {
				c.Header("Content-Disposition", "attachment; filename=3m-ui.yaml")
			}
			c.Header("Cache-Control", "no-store")

			c.Data(http.StatusOK, "text/yaml; charset=utf-8", raw)
			return
		}

		converted, err := converter.CallSubconverterWithRequest(cfg, tok, target, raw)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		if isProxyUser {
			writeSubHeaders(c, db, &pu)
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", converted)
	}
}

// writeSubHeaders emits the standard subscription response headers that
// compatible clients (v2rayNG, Hiddify, Clash, …) read for traffic / expiry.
// Profile-Title / support-url / announce mirror common client subscription metadata.
func writeSubHeaders(c *gin.Context, db *gorm.DB, pu *models.ProxyUser) {
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
	page := subpage.LoadPageSettings(db)
	hours := page.UpdateHours
	if hours <= 0 {
		hours = 12
	}
	c.Header("Profile-Update-Interval", itoa(int64(hours)))
	title := strings.TrimSpace(page.Title)
	if title == "" {
		title = strings.TrimSpace(pu.Remark)
	}
	if title == "" {
		title = strings.TrimSpace(pu.Username)
	}
	if title == "" {
		title = "3m-ui"
	}
	c.Header("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(title)))
	if su := strings.TrimSpace(page.SupportURL); su != "" {
		c.Header("Support-Url", su)
		c.Header("support-url", su)
	}
	if an := strings.TrimSpace(page.Announce); an != "" {
		c.Header("Announce", an)
		c.Header("announce", an)
	}
	if wp := strings.TrimSpace(page.WebPageURL); wp != "" {
		c.Header("Profile-Web-Page-Url", wp)
	} else {
		// Fall back to HTML info page for this token.
		tok := strings.TrimSpace(pu.SubToken)
		if tok == "" {
			tok = strings.TrimSpace(c.Param("token"))
		}
		if tok != "" {
			scheme := requestScheme(c)
			c.Header("Profile-Web-Page-Url", scheme+"://"+c.Request.Host+"/api/v1/client/sub/"+url.PathEscape(tok)+"?html=1")
		}
	}
	c.Header("Content-Disposition", `attachment; filename="`+sanitizeFilename(title)+`.yaml"`)
}

func sanitizeFilename(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, s)
	if s == "" {
		return "subscription"
	}
	if len(s) > 64 {
		return s[:64]
	}
	return s
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
	// Path-based formats (docs.sanaei.dev style /sub /json /clash).
	api.GET("/client/json/:token", forcedTargetHandler(db, cfg, "singbox"))
	api.GET("/client/json/:token/", forcedTargetHandler(db, cfg, "singbox"))
	api.GET("/client/clash/:token", forcedTargetHandler(db, cfg, "clash"))
	api.GET("/client/clash/:token/", forcedTargetHandler(db, cfg, "clash"))
	api.GET("/client/v2ray/:token", forcedTargetHandler(db, cfg, "v2ray"))
	api.GET("/client/v2ray/:token/", forcedTargetHandler(db, cfg, "v2ray"))
}

// forcedTargetHandler serves a subscription with a fixed target format.
func forcedTargetHandler(db *gorm.DB, cfg *config.Config, target string) gin.HandlerFunc {
	inner := subscriptionHandler(db, cfg)
	return func(c *gin.Context) {
		q := c.Request.URL.Query()
		q.Set("target", target)
		c.Request.URL.RawQuery = q.Encode()
		inner(c)
	}
}

// RegisterLegacySubscriptionRoutes keeps subscription links generated by
// older releases working after upgrading to the /api/v1 route layout.
func RegisterLegacySubscriptionRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	handler := subscriptionHandler(db, cfg)
	r.GET("/sub/:token", handler)
	r.GET("/sub/:token/", handler)
	r.GET("/json/:token", forcedTargetHandler(db, cfg, "singbox"))
	r.GET("/json/:token/", forcedTargetHandler(db, cfg, "singbox"))
	r.GET("/clash/:token", forcedTargetHandler(db, cfg, "clash"))
	r.GET("/clash/:token/", forcedTargetHandler(db, cfg, "clash"))
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
		scheme := requestScheme(c)
		base := scheme + "://" + c.Request.Host + "/api/v1/client/sub/" + url.PathEscape(tok)
		c.JSON(http.StatusOK, gin.H{
			"username":       pu.Username,
			"remark":         pu.Remark,
			"enabled":        pu.Enabled,
			"blocked":        !user.IsCredentialActive(pu),
			"online":         pu.Online,
			"traffic_used":   pu.TrafficUsed,
			"traffic_limit":  pu.TrafficLimit,
			"upload_bytes":   pu.UploadBytes,
			"download_bytes": pu.DownloadBytes,
			"expire_time":    expire,
			"ip_limit":       pu.IPLimit,
			"links": gin.H{
				"mihomo":     base,
				"clash":      base + "?target=clash",
				"v2ray":      base + "?target=v2ray",
				"singbox":    base + "?target=singbox",
				"json":       scheme + "://" + c.Request.Host + "/api/v1/client/json/" + url.PathEscape(tok),
				"clash_path": scheme + "://" + c.Request.Host + "/api/v1/client/clash/" + url.PathEscape(tok),
				"html":       base + "?html=1",
				"info":       base + "?format=info",
			},
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
	case strings.Contains(u, "clash") || strings.Contains(u, "mihomo") || strings.Contains(u, "stash") ||
		strings.Contains(u, "meta") || strings.Contains(u, "clashmeta") || strings.Contains(u, "verge") ||
		strings.Contains(u, "flclash") || strings.Contains(u, "prizmdotdev"):
		return "mihomo"
	case strings.Contains(u, "surge"):
		return "surge"
	case strings.Contains(u, "quantumult"):
		return "quanx"
	case strings.Contains(u, "loon"):
		return "loon"
	case strings.Contains(u, "shadowrocket"):
		return "mihomo"
	case strings.Contains(u, "sing-box") || strings.Contains(u, "sfa") || strings.Contains(u, "sfm") ||
		strings.Contains(u, "sfi") || strings.Contains(u, "singbox"):
		return "singbox"
	// Classic v2ray / Xray clients expect base64 list of share links.
	case strings.Contains(u, "v2ray") || strings.Contains(u, "v2rayng") || strings.Contains(u, "v2rayn") ||
		strings.Contains(u, "streisand") || strings.Contains(u, "hiddify") || strings.Contains(u, "nekobox") ||
		strings.Contains(u, "nekoray") || strings.Contains(u, "v2box") || strings.Contains(u, "passwall") ||
		strings.Contains(u, "napsternet") || strings.Contains(u, "foXray") || strings.Contains(u, "foxray"):
		return "v2ray"
	default:
		// Prefer Clash/Meta YAML for generic downloaders; clients that need base64 use ?target=v2ray.
		return "mihomo"
	}
}

func writeSubHTML(c *gin.Context, db *gorm.DB, cfg *config.Config, pu models.ProxyUser, tok string) {
	scheme := requestScheme(c)
	host := c.Request.Host
	base := scheme + "://" + host + "/api/v1/client/sub/" + url.PathEscape(tok)
	var links []string
	if raw, err := converter.GenerateUserBase64Subscription(db, pu, c.Request, node.ClientURIsWithCredentials); err == nil {
		if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
			for _, line := range strings.Split(string(decoded), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					links = append(links, line)
				}
			}
		}
	}
	html, err := subpage.RenderHTML(db, pu, base, links)
	if err != nil {
		log.Printf("subscription error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", html)
}

func requestScheme(c *gin.Context) string {
	if xf := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xf != "" {
		// Take the first value when proxies send a comma-separated list.
		if i := strings.IndexByte(xf, ','); i >= 0 {
			xf = xf[:i]
		}
		xf = strings.ToLower(strings.TrimSpace(xf))
		if xf == "https" || xf == "http" {
			return xf
		}
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}
