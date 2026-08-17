package user

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

func (s *Service) EnsureSubToken(id uint) (string, error) {
	var u models.ProxyUser
	if err := s.db.First(&u, id).Error; err != nil {
		return "", err
	}
	if strings.TrimSpace(u.SubToken) != "" {
		return u.SubToken, nil
	}
	token, err := randomHex(16)
	if err != nil {
		return "", err
	}
	u.SubToken = token
	if err := s.db.Model(&u).Update("sub_token", token).Error; err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) RotateSubToken(id uint) (string, error) {
	var u models.ProxyUser
	if err := s.db.First(&u, id).Error; err != nil {
		return "", err
	}
	token, err := randomHex(16)
	if err != nil {
		return "", err
	}
	u.SubToken = token
	if err := s.db.Model(&u).Update("sub_token", token).Error; err != nil {
		return "", err
	}
	return token, nil
}

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
