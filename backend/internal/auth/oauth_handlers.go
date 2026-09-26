package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/telegram"
)

func (h *Handler) publicURL() string {
	if h.cfg != nil {
		return strings.TrimSpace(h.cfg.Server.PublicURL)
	}
	return ""
}

func (h *Handler) requestOrigin(c *gin.Context) string {
	proto := c.GetHeader("X-Forwarded-Proto")
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return proto + "://" + host
}

func (h *Handler) oauthRedirectURI(c *gin.Context) string {
	base := resolveOAuthRedirectBase(h.requestOrigin(c), h.publicURL())
	return base + "/api/v1/auth/oauth/github/callback"
}

func (h *Handler) frontendLoginURL(c *gin.Context) string {
	base := resolveOAuthRedirectBase(h.requestOrigin(c), h.publicURL())
	// Prefer SPA origin: strip nothing — panel is same host as API for native install.
	return strings.TrimRight(base, "/") + "/login"
}

func (h *Handler) GithubOAuthPublic(c *gin.Context) {
	s, err := loadGithubOAuthSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, publicGithubOAuth(s))
}

func (h *Handler) GithubOAuthGetSettings(c *gin.Context) {
	s, err := loadGithubOAuthSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Never echo full secret if you prefer mask — return as-is for admin edit (same as telegram token pattern).
	c.JSON(http.StatusOK, s)
}

func (h *Handler) GithubOAuthPutSettings(c *gin.Context) {
	var s GithubOAuthSettings
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.ClientID = strings.TrimSpace(s.ClientID)
	s.ClientSecret = strings.TrimSpace(s.ClientSecret)
	if s.Enabled && (s.ClientID == "" || s.ClientSecret == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_id and client_secret are required when enabled"})
		return
	}
	if err := saveGithubOAuthSettings(h.db, s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"message":      "GitHub OAuth settings saved",
		"settings":     publicGithubOAuth(s),
		"callback_url": h.oauthRedirectURI(c),
	})
}

func (h *Handler) GithubOAuthStart(c *gin.Context) {
	clientID := clientIdentifier(c)
	if !allowLogin(clientID) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts; try again later"})
		return
	}
	s, err := loadGithubOAuthSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !s.Enabled || strings.TrimSpace(s.ClientID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "GitHub OAuth is not enabled"})
		return
	}
	state, err := newOAuthState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create state"})
		return
	}
	redirectURI := h.oauthRedirectURI(c)
	loc := githubAuthorizeURL(s.ClientID, redirectURI, state)
	c.Redirect(http.StatusFound, loc)
}

func (h *Handler) GithubOAuthCallback(c *gin.Context) {
	front := h.frontendLoginURL(c)
	fail := func(msg string) {
		c.Redirect(http.StatusFound, front+"?oauth_error="+url.QueryEscape(msg))
	}

	if e := c.Query("error"); e != "" {
		fail(e + ": " + c.Query("error_description"))
		return
	}
	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		fail("missing code or state")
		return
	}
	if !consumeOAuthState(state) {
		fail("invalid or expired state")
		return
	}

	s, err := loadGithubOAuthSettings(h.db)
	if err != nil || !s.Enabled {
		fail("oauth not configured")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	redirectURI := h.oauthRedirectURI(c)
	token, err := exchangeGithubCode(ctx, s.ClientID, s.ClientSecret, code, redirectURI)
	if err != nil {
		fail(err.Error())
		return
	}
	gh, err := fetchGithubUser(ctx, token)
	if err != nil {
		fail(err.Error())
		return
	}

	result, err := loginOrLinkGithubUser(h.db, h.secret, gh, s.AllowedLogins)
	if err != nil {
		clientID := clientIdentifier(c)
		go telegram.NotifyLoginFailed(h.db, "github:"+gh.Login, clientID)
		fail(err.Error())
		return
	}
	clientID := clientIdentifier(c)
	resetLoginLimit(clientID)
	go telegram.NotifyLogin(h.db, result.Username+" (github:"+gh.Login+")", clientID)

	q := url.Values{}
	q.Set("oauth_token", result.Token)
	q.Set("username", result.Username)
	if result.MustChangePassword {
		q.Set("must_change_password", "1")
	}
	c.Redirect(http.StatusFound, front+"?"+q.Encode())
}
