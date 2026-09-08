package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
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
// When THREE_M_UI_ADMIN_PASSWORD is unset, a random password is returned once
// to the caller. Only its bcrypt hash is stored in the database.
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
		password, err = GeneratePassword()
		if err != nil {
			return false, "", "", err
		}
	}
	if len(password) < 8 || len(password) > 72 {
		return false, "", "", fmt.Errorf("initial administrator password must be between 8 and 72 bytes")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return false, "", "", err
	}
	if err := db.Create(&models.User{Username: username, PasswordHash: hash, Role: "admin", MustChangePassword: true, SessionVersion: 1}).Error; err != nil {
		return false, "", "", fmt.Errorf("create initial admin: %w", err)
	}

	return true, username, password, nil
}

// GeneratePassword creates a cryptographically random initial or reset password.
func GeneratePassword() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate administrator password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func EncodePassword(password string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(password))
}

// ResetAdminPassword sets the first administrator password to the given plaintext
// and forces password change on next login. Does not create
// an admin if none exists.
func ResetAdminPassword(db *gorm.DB, plaintext string) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if len(plaintext) < 8 || len(plaintext) > 72 {
		return fmt.Errorf("administrator password must be between 8 and 72 bytes")
	}
	var u models.User
	if err := db.Where("role = ?", "admin").Order("id asc").First(&u).Error; err != nil {
		return fmt.Errorf("no administrator found: %w", err)
	}
	hash, err := HashPassword(plaintext)
	if err != nil {
		return err
	}
	return db.Model(&u).Updates(map[string]interface{}{
		"password_hash":        hash,
		"must_change_password": true,
		"session_version":      u.SessionVersion + 1,
	}).Error
}
