package user

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

// EnsureSubToken returns a stable public subscription token for the user,
// creating one if missing (client subscription token).
//
// TOCTOU (R2-5.5): the Count check below ("does any user already have this
// token?") followed by the Update is a classic time-of-check/time-of-use race.
// Two concurrent EnsureSubToken callers could both see n==0 and then both
// attempt to write the same token. The Count check is therefore BEST-EFFORT:
// it only reduces collision probability (and saves a wasted UPDATE round-trip
// in the common case). The real safety net is the UNIQUE INDEX on
// ProxyUser.SubToken declared in the model migration — a duplicate write will
// fail with a unique-constraint violation and the loop will retry with a
// freshly generated token. The same reasoning applies to RotateSubToken below.
func (s *Service) EnsureSubToken(id uint) (string, error) {
	var u models.ProxyUser
	if err := s.db.First(&u, id).Error; err != nil {
		return "", err
	}
	if strings.TrimSpace(u.SubToken) != "" {
		return u.SubToken, nil
	}
	for i := 0; i < 5; i++ {
		token, err := randomHex(16)
		if err != nil {
			return "", err
		}
		var n int64
		if err := s.db.Model(&models.ProxyUser{}).Where("sub_token = ?", token).Count(&n).Error; err != nil {
			return "", err
		}
		if n > 0 {
			continue
		}
		if err := s.db.Model(&u).Update("sub_token", token).Error; err != nil {
			return "", err
		}
		return token, nil
	}
	return "", fmt.Errorf("failed to allocate unique sub token")
}

// RotateSubToken issues a new subscription token (invalidates old URL).
func (s *Service) RotateSubToken(id uint) (string, error) {
	var u models.ProxyUser
	if err := s.db.First(&u, id).Error; err != nil {
		return "", err
	}
	for i := 0; i < 5; i++ {
		token, err := randomHex(16)
		if err != nil {
			return "", err
		}
		var n int64
		if err := s.db.Model(&models.ProxyUser{}).Where("sub_token = ? AND id <> ?", token, id).Count(&n).Error; err != nil {
			return "", err
		}
		if n > 0 {
			continue
		}
		if err := s.db.Model(&u).Update("sub_token", token).Error; err != nil {
			return "", err
		}
		return token, nil
	}
	return "", fmt.Errorf("failed to allocate unique sub token")
}

// FindBySubToken looks up a user by public subscription token.
func (s *Service) FindBySubToken(token string) (*models.ProxyUser, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var u models.ProxyUser
	if err := s.db.Where("sub_token = ?", token).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
