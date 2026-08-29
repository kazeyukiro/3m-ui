package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const DefaultTokenTTL = 24 * time.Hour

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResult struct {
	Token              string    `json:"token"`
	ExpiresAt          time.Time `json:"expires_at"`
	Username           string    `json:"username"`
	Role               string    `json:"role"`
	MustChangePassword bool      `json:"must_change_password"`
}

func Login(db *gorm.DB, jwtSecret string, input LoginInput) (*LoginResult, error) {
	var user models.User
	if err := db.Where("username = ?", strings.TrimSpace(input.Username)).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}
	if !CheckPasswordHash(input.Password, user.PasswordHash) {
		return nil, errors.New("invalid username or password")
	}
	if user.SessionVersion == 0 {
		user.SessionVersion = 1
		if err := db.Model(&user).Update("session_version", user.SessionVersion).Error; err != nil {
			return nil, err
		}
	}
	token, exp, err := GenerateToken(jwtSecret, user.ID, user.Username, user.Role, user.SessionVersion, DefaultTokenTTL)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, ExpiresAt: exp, Username: user.Username, Role: user.Role, MustChangePassword: user.MustChangePassword}, nil
}

// EnsureAdmin creates the first administrator only when the database has no
// administrator. An explicit THREE_M_UI_ADMIN_PASSWORD is preferred.
// When THREE_M_UI_ADMIN_PASSWORD is unset, the initial password is "admin".
// RequireAuth forces a password change on first login (MustChangePassword=true)
// before any other API can be used. Change it immediately on first login.
func EnsureAdmin(db *gorm.DB, dbPath string) (created bool, username, password string, err error) {
	var count int64
	if err := db.Model(&models.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return false, "", "", err
	}
	if count > 0 {
		return false, "", "", nil
	}

	username = strings.TrimSpace(os.Getenv("THREE_M_UI_ADMIN_USERNAME"))
	if username == "" {
		username = "admin"
	}
	password = os.Getenv("THREE_M_UI_ADMIN_PASSWORD")
	if password == "" {
		password = "admin"
	}

	hash, err := HashPassword(password)
	if err != nil {
		return false, "", "", err
	}
	if err := db.Create(&models.User{Username: username, PasswordHash: hash, Role: "admin", MustChangePassword: true, SessionVersion: 1}).Error; err != nil {
		return false, "", "", fmt.Errorf("create initial admin: %w", err)
	}

	log.Printf("[SECURITY] Initial administrator created: %s. Password is 'admin' (or THREE_M_UI_ADMIN_PASSWORD). Change it immediately on first login.", username)

	return true, username, password, nil
}

func EncodePassword(password string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(password))
}
