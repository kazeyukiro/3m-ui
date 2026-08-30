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
// response. CSP is intentionally NOT set here: API responses are JSON and a
// global CSP would risk breaking them, while the SPA shell already carries a
// tight Content-Security-Policy via a <meta> tag in index.html. The headers
// below harden both API and SPA responses against common MIME-sniffing,
// framing, and reflected-XSS pitfalls.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-XSS-Protection", "1; mode=block")
		// Don't set CSP on API responses — only on the SPA HTML.
		// The meta CSP in index.html handles the SPA.
		c.Next()
	}
}
