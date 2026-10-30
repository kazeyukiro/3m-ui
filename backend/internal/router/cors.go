package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware builds a CORS middleware from configured origins.
// - empty list → deny cross-origin (no Access-Control-Allow-Origin header)
// - "*"        → allow any origin (explicit opt-in)
// - otherwise  → reflect the request Origin only when it matches an entry
func CORSMiddleware(origins []string) gin.HandlerFunc {
	allowAll := false
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			allowAll = true
			continue
		}
		allowed[o] = struct{}{}
	}

	const allowHeaders = "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With"
	const allowMethods = "POST, OPTIONS, GET, PUT, DELETE"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowAll {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "false")
		} else if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "false")
			}
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		c.Writer.Header().Set("Access-Control-Allow-Methods", allowMethods)

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// SecurityHeaders sets defensive browser-side HTTP headers on every
// response. The CSP below is the authoritative policy; index.html also
// carries the same policy as a <meta> tag, which is defense-in-depth for
// the window before the backend header arrives and is the only layer in
// front if the panel is ever served by a proxy that strips headers.
//
// The policy is applied globally rather than only on the SPA HTML: CSP
// only constrains document responses, so JSON API and subscription
// responses carry it harmlessly. Keeping it global also covers direct
// hits on /assets/*.html and any future HTML route.
func SecurityHeaders() gin.HandlerFunc {
	// Keep in sync with the <meta http-equiv="Content-Security-Policy">
	// tag in frontend/index.html. script-src has no 'unsafe-inline' —
	// the SPA is built by Vite into external files only — while
	// style-src keeps it because React inline style={{...}} (350+ uses)
	// needs it. img-src is 'self' + data: only - a SPA source audit found no remote image loading (every <img> resolves to the bundled logo under the panel's web_path prefix, no flag/avatar library), so https: is dropped to close the image-exfil gap real-browser testing exposed.
	const csp = "default-src 'self'; " +
		"script-src 'self'; " +
		"worker-src 'self' blob:; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"object-src 'none'"

	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Content-Security-Policy", csp)
		c.Next()
	}
}
