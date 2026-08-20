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
	"github.com/kazeyukiro/3m-ui/backend/internal/telegram"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
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

const (
	loginWindow     = 15 * time.Minute
	loginMaxAttempt = 8
)

// clientIdentifier returns a stable identifier for rate-limiting.
func clientIdentifier(c *gin.Context) string {
	// Never trust a client-supplied X-Forwarded-For value for authentication
	// rate limiting. A proxy can be configured to restore the real client IP
	// at the Gin layer, but accepting the raw header here lets an attacker
	// rotate arbitrary IPs and bypass the limiter.
	remote, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil && net.ParseIP(remote) != nil {
		return remote
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
		if err.Error() != "invalid username or password" {
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"error": "invalid username or password"})
		return
	}
	resetLoginLimit(clientID)
	// 3x-ui parity: optional Telegram notice on successful panel login.
	go telegram.NotifyLogin(h.db, input.Username, clientID)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ChangePassword(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "password changed successfully"})
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
