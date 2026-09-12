package auth

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/totp"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/telegram"
	"gorm.io/gorm"
)

type loginAttempt struct {
	count   int
	blocked time.Time
	last    time.Time
}

var loginLimiter = struct {
	sync.Mutex
	items map[string]loginAttempt
}{items: make(map[string]loginAttempt)}

var passwordChangeLimiter = struct {
	sync.Mutex
	items map[string]loginAttempt
}{items: make(map[string]loginAttempt)}

const (
	loginWindow              = 15 * time.Minute
	loginMaxAttempt          = 8
	passwordChangeWindow     = 15 * time.Minute
	passwordChangeMaxAttempt = 3
)

// clientIdentifier returns a stable identifier for rate-limiting.
//
// By default the direct RemoteAddr IP is used so that a client cannot rotate
// arbitrary IPs via a spoofed X-Forwarded-For header. As a pragmatic
// improvement, when the direct connection originates from a loopback /
// private / link-local address (the typical case for a same-host reverse
// proxy such as nginx or caddy) we DO trust the right-most valid IP in
// X-Forwarded-For. This restores per-client rate-limiting granularity when
// deployed behind a local trusted proxy while still rejecting client-supplied
// XFF values on direct (public) exposure.
func clientIdentifier(c *gin.Context) string {
	remote, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil {
		if ip := net.ParseIP(remote); ip != nil {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
				if xff := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); xff != "" {
					parts := strings.Split(xff, ",")
					for i := len(parts) - 1; i >= 0; i-- {
						candidate := strings.TrimSpace(parts[i])
						if p := net.ParseIP(candidate); p != nil {
							return p.String()
						}
					}
				}
			}
			return ip.String()
		}
	}
	if ip := net.ParseIP(strings.TrimSpace(c.Request.RemoteAddr)); ip != nil {
		return ip.String()
	}
	return "unknown"
}

func allowLogin(ip string) bool {
	now := time.Now()
	loginLimiter.Lock()
	defer loginLimiter.Unlock()

	for key, attempt := range loginLimiter.items {
		if now.Sub(attempt.last) > loginWindow {
			delete(loginLimiter.items, key)
		}
	}

	attempt := loginLimiter.items[ip]
	if !attempt.blocked.IsZero() && now.Before(attempt.blocked) {
		return false
	}
	if attempt.last.IsZero() || now.Sub(attempt.last) > loginWindow {
		attempt.count = 0
	}
	attempt.last = now
	if attempt.count >= loginMaxAttempt {
		attempt.blocked = now.Add(loginWindow)
		loginLimiter.items[ip] = attempt
		return false
	}
	attempt.count++
	loginLimiter.items[ip] = attempt
	return true
}

func resetLoginLimit(ip string) {
	loginLimiter.Lock()
	delete(loginLimiter.items, ip)
	loginLimiter.Unlock()
}

// allowPasswordChange applies a per-client throttle to the password-change
// endpoint. The bound is intentionally tight (3 attempts / 15 min) because the
// caller is already authenticated, so the only legitimate traffic is a single
// user choosing a new password. Anything beyond that pattern is brute-forcing
// the current_password field or probing for hash collisions.
func allowPasswordChange(ip string) bool {
	now := time.Now()
	passwordChangeLimiter.Lock()
	defer passwordChangeLimiter.Unlock()

	for key, attempt := range passwordChangeLimiter.items {
		if now.Sub(attempt.last) > passwordChangeWindow {
			delete(passwordChangeLimiter.items, key)
		}
	}

	attempt := passwordChangeLimiter.items[ip]
	if !attempt.blocked.IsZero() && now.Before(attempt.blocked) {
		return false
	}
	if attempt.last.IsZero() || now.Sub(attempt.last) > passwordChangeWindow {
		attempt.count = 0
	}
	attempt.last = now
	if attempt.count >= passwordChangeMaxAttempt {
		attempt.blocked = now.Add(passwordChangeWindow)
		passwordChangeLimiter.items[ip] = attempt
		return false
	}
	attempt.count++
	passwordChangeLimiter.items[ip] = attempt
	return true
}

func resetPasswordChangeLimit(ip string) {
	passwordChangeLimiter.Lock()
	delete(passwordChangeLimiter.items, ip)
	passwordChangeLimiter.Unlock()
}

type Handler struct {
	db     *gorm.DB
	secret string
	cfg    *config.Config
}

func NewHandler(db *gorm.DB, cfg *config.Config) *Handler {
	secret := ""
	if cfg != nil {
		secret = cfg.JWT.Secret
	}
	return &Handler{db: db, secret: secret, cfg: cfg}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/login", h.Login)
	rg.POST("/password", RequireAuth(h.db, h.secret), h.ChangePassword)
	rg.GET("/me", RequireAuth(h.db, h.secret), h.Me)
	rg.POST("/totp/setup", RequireAuth(h.db, h.secret), h.TOTPSetup)
	rg.POST("/totp/enable", RequireAuth(h.db, h.secret), h.TOTPEnable)
	rg.POST("/totp/disable", RequireAuth(h.db, h.secret), h.TOTPDisable)
}

func (h *Handler) Login(c *gin.Context) {
	clientID := clientIdentifier(c)
	if !allowLogin(clientID) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts; try again later"})
		return
	}

	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
		return
	}
	result, err := Login(h.db, h.secret, input)
	if err != nil {
		status := http.StatusUnauthorized
		msg := "invalid username or password"
		if err.Error() == "invalid totp code" {
			msg = "invalid totp code"
		} else if err.Error() != "invalid username or password" {
			status = http.StatusInternalServerError
		}
		go telegram.NotifyLoginFailed(h.db, input.Username, clientID)
		c.JSON(status, gin.H{"error": msg})
		return
	}
	if result != nil && result.TOTPRequired {
		c.JSON(http.StatusOK, result)
		return
	}
	resetLoginLimit(clientID)
	go telegram.NotifyLogin(h.db, input.Username, clientID)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) TOTPSetup(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Model(&models.User{}).Where("id = ?", claims.UserID).Updates(map[string]interface{}{
		"totp_secret":  secret,
		"totp_enabled": false,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"secret": secret,
		"otpauth_url": totp.OTPAuthURL("3m-ui", claims.Username, secret),
	})
}

func (h *Handler) TOTPEnable(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code required"})
		return
	}
	var user models.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}
	if strings.TrimSpace(user.TOTPSecret) == "" || !totp.Verify(user.TOTPSecret, req.Code, 1) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid totp code"})
		return
	}
	if err := h.db.Model(&user).Update("totp_enabled", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "totp_enabled": true})
}

func (h *Handler) TOTPDisable(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password required"})
		return
	}
	var user models.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}
	if !CheckPasswordHash(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}
	if user.TOTPEnabled && !totp.Verify(user.TOTPSecret, req.Code, 1) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid totp code"})
		return
	}
	if err := h.db.Model(&user).Updates(map[string]interface{}{"totp_enabled": false, "totp_secret": ""}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "totp_enabled": false})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	clientID := clientIdentifier(c)
	if !allowPasswordChange(clientID) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many password change attempts; try again later"})
		return
	}

	claims, ok := ClaimsFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current_password and new_password are required"})
		return
	}
	if len([]rune(req.NewPassword)) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new password must be at least 8 characters"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.NewPassword), []byte(req.CurrentPassword)) == 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new password must differ from current password"})
		return
	}

	var user models.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !CheckPasswordHash(req.CurrentPassword, user.PasswordHash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current password is incorrect"})
		return
	}
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	nextSessionVersion := user.SessionVersion + 1
	if nextSessionVersion == 0 {
		nextSessionVersion = 1
	}
	if err := h.db.Model(&user).Updates(map[string]any{
		"password_hash":        hash,
		"must_change_password": false,
		"session_version":      nextSessionVersion,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save password"})
		return
	}
	resetPasswordChangeLimit(clientID)

	// Issue a fresh token bound to the new session_version. Without this, the
	// browser keeps the pre-change JWT and every subsequent API call returns 401
	// ("session has been invalidated"), which looks like "cannot enter the panel".
	token, exp, err := GenerateToken(h.secret, user.ID, user.Username, user.Role, nextSessionVersion, DefaultTokenTTL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "password changed successfully; please log in again",
			"relogin": true,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":               "ok",
		"message":              "password changed successfully",
		"token":                token,
		"expires_at":           exp,
		"username":             user.Username,
		"must_change_password": false,
	})
}

func (h *Handler) Me(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var user models.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":              claims.UserID,
		"username":             claims.Username,
		"role":                 claims.Role,
		"expires_at":           claims.ExpiresAt,
		"must_change_password": user.MustChangePassword,
		"totp_enabled":         user.TOTPEnabled,
	})
}

// RequireAuth validates the Bearer JWT against the provided secret and loads
// the administrator from the provided database.
func RequireAuth(db *gorm.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := TokenFromRequest(c.GetHeader("Authorization"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		claims, err := ParseToken(secret, token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		if db == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var user models.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
		if !strings.EqualFold(user.Role, "admin") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "administrator access required"})
			return
		}
		if user.SessionVersion == 0 || claims.SessionVersion == 0 || user.SessionVersion != claims.SessionVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session has been invalidated; please log in again"})
			return
		}

		c.Set("auth.claims", claims)
		c.Set("auth.user", &user)

		path := c.Request.URL.Path
		if path != "/api/v1/auth/password" && path != "/api/v1/auth/me" && user.MustChangePassword {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "password change required",
				"code":  "PASSWORD_CHANGE_REQUIRED",
			})
			return
		}

		c.Next()
	}
}

func ClaimsFromContext(c *gin.Context) (*JWTClaims, bool) {
	value, ok := c.Get("auth.claims")
	if !ok {
		return nil, false
	}
	claims, ok := value.(*JWTClaims)
	return claims, ok
}
