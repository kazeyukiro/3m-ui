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
	"github.com/kazeyukiro/3m-ui/backend/internal/subpull"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// defaultSubpageTitle mirrors subpage.Settings' install-time default title.
// When the operator has not customized it, the per-user remark/username is
// used for Profile-Title instead (see writeSubHeaders).
const defaultSubpageTitle = "3m-ui Subscription"

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
		// Built-in subscription info page — always rendered locally.
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
			user.TouchFirstUse(db, pu.ID)
			if err := subpull.Enforce(db, pu.ID, pu.SubPullLimit); err != nil {
				if errors.Is(err, subpull.ErrLimitReached) {
					c.Header("Retry-After", "3600")
					c.JSON(http.StatusTooManyRequests, gin.H{
						"error": "subscription pull limit reached (per 24h)",
						"limit": pu.SubPullLimit,
					})
					return
				}
				log.Printf("subpull enforce: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription pull limit check failed"})
				return
			}
			if wantsHTML {
				writeSubHTML(c, db, cfg, pu, tok)
				return
			}
			// Native v2ray / base64 subscription (classic client subscription).
			// v2rayNG / v2rayN / most classic clients require a Base64 body of
			// share links. target=v2ray|base64 always returns Base64.
			// target=uri|raw may return plaintext when encrypt=0.
			if target == "v2ray" || target == "base64" || target == "raw" || target == "uri" {
				raw, err = converter.GenerateUserBase64Subscription(db, pu, c.Request, node.ClientURIsWithCredentials)
				if err != nil {
					log.Printf("subscription error: %v", err)
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
					return
				}
				forceBase64 := target == "v2ray" || target == "base64"
				page := subpage.LoadPageSettings(db)
				encrypt := page.Encrypt
				if q := strings.ToLower(c.Query("encrypt")); q == "0" || q == "false" {
					encrypt = false
				} else if q == "1" || q == "true" {
					encrypt = true
				}
				// Classic v2ray subscription must stay Base64 regardless of the
				// HTML page "encrypt URI list" toggle — that toggle only applies
				// to target=uri|raw (human-readable link lists).
				if !forceBase64 && !encrypt {
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
			// Native Loon / Surge / Quantumult X exports. These clients speak
			// their own node-line grammar; render them locally instead of
			// depending on an external conversion service.
			switch target {
			case "loon":
				raw, err = converter.GenerateUserLoonSubscription(db, pu, c.Request, node.ClientURIsWithCredentials)
			case "surge":
				raw, err = converter.GenerateUserSurgeSubscription(db, pu, c.Request, node.ClientURIsWithCredentials)
			case "quanx", "quantumult", "qx":
				raw, err = converter.GenerateUserQuanXSubscription(db, pu, c.Request, node.ClientURIsWithCredentials)
			}
			if err != nil {
				log.Printf("subscription error: %v", err)
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
				return
			}
			switch target {
			case "loon", "surge", "quanx", "quantumult", "qx":
				writeSubHeaders(c, db, &pu)
				c.Header("Cache-Control", "no-store")
				c.Header("Content-Disposition", `attachment; filename="`+sanitizeFilename(pu.Username)+`.conf"`)
				c.Data(http.StatusOK, "text/plain; charset=utf-8", raw)
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

		// Every target above this point is rendered natively. The external
		// subconverter integration has been removed, so an unrecognized
		// target is a client error rather than a forwarded service call.
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "unsupported subscription target: " + target,
		})
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
	// Subscription-Userinfo: omit expire entirely when the account never
	// expires. Some clients read expire=0 as "expired since epoch" instead of
	// "no limit" (P1-6); leaving the field out is the de-facto convention.
	expire := int64(0)
	if !pu.ExpireTime.IsZero() {
		expire = pu.ExpireTime.Unix()
	}
	userinfo := "upload=" + itoa(upload) + "; download=" + itoa(download) + "; total=" + itoa(total)
	if expire != 0 {
		userinfo += "; expire=" + itoa(expire)
	}
	c.Header("Subscription-Userinfo", userinfo)
	page := subpage.LoadPageSettings(db)
	hours := page.UpdateHours
	if hours <= 0 {
		hours = 12
	}
	c.Header("Profile-Update-Interval", itoa(int64(hours)))
	// The install default title ("3m-ui Subscription") is not a real choice
	// by the operator, so prefer the account's own remark/username: every
	// client subscription then names the account it belongs to (P3). A
	// custom sub_title from the page settings still wins.
	title := strings.TrimSpace(page.Title)
	if title == "" || title == defaultSubpageTitle {
		if remark := strings.TrimSpace(pu.Remark); remark != "" {
			title = remark
		} else if username := strings.TrimSpace(pu.Username); username != "" {
			title = username
		}
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
			c.Header("Profile-Web-Page-Url", subBaseURL(c, tok)+"?html=1")
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
	// Root-level short paths (clients hardcode /v2ray/<token>) live on the
	// root engine — see RegisterLegacySubscriptionRoutes. Registering them on
	// the /api/v1 group would expose them at /api/v1/v2ray/:token instead and
	// the bare path would still fall through to the SPA HTML (P2-6).
}

// RegisterCustomSubPathRoutes mounts subscription handlers under an optional
// short path (e.g. /sub/:token) on the root engine, matching s-ui style paths.
func RegisterCustomSubPathRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	if r == nil || cfg == nil {
		return
	}
	prefix := config.NormalizeSubPath(cfg.Server.SubPath)
	if prefix == "" {
		return
	}
	handler := subscriptionHandler(db, cfg)
	r.GET(prefix+"/:token", handler)
	r.GET(prefix+"/:token/", handler)
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
	// Root-level v2ray short path (clients hardcode /v2ray/<token>). Without
	// it the request falls through to the SPA HTML (P2-6).
	r.GET("/v2ray/:token", forcedTargetHandler(db, cfg, "v2ray"))
	r.GET("/v2ray/:token/", forcedTargetHandler(db, cfg, "v2ray"))
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
		base := subBaseURL(c, tok)
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
				"loon":       base + "?target=loon",
				"surge":      base + "?target=surge",
				"quanx":      base + "?target=quanx",
				"json":       subRootURL(c) + "json/" + url.PathEscape(tok),
				"clash_path": subRootURL(c) + "clash/" + url.PathEscape(tok),
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

// subRootURL is the panel's client API root (scheme://host<basePath>/api/v1/client/),
// including the random "hidden entry" prefix when one is configured.
func subRootURL(c *gin.Context) string {
	prefix := strings.TrimSuffix(c.GetHeader("X-Web-Path"), "/")
	return requestScheme(c) + "://" + c.Request.Host + prefix + "/api/v1/client/"
}

// subBaseURL builds the canonical subscription URL for a token, including the
// panel base path (the random "hidden entry" prefix) when the request arrived
// under one. WebPathHandler records the stripped prefix in the X-Web-Path
// header, so links / QR codes / app deep links rendered on the subscription
// page stay reachable through the hidden path instead of pointing at the bare
// domain (which always 404s).
func subBaseURL(c *gin.Context, tok string) string {
	return subRootURL(c) + "sub/" + url.PathEscape(tok)
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
	base := subBaseURL(c, tok)
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
	// The page is a single-file template whose interactive layer lives in an
	// inline <script> block; the global SecurityHeaders policy (script-src
	// 'self', no 'unsafe-inline') would block it and silently kill copy /
	// i18n / toast. Re-issue the same policy with per-response sha256 hashes
	// for this page's inline scripts (external scripts stay 'self').
	c.Header("Content-Security-Policy", subpage.InlineScriptCSP(html))
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
