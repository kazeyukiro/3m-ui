package user

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/credentials"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
	"gorm.io/gorm"
)

type Service struct {
	db                 *gorm.DB
	credentialsChanged func() error
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) SetCredentialsChangedHandler(fn func() error) {
	s.credentialsChanged = fn
}

func (s *Service) notifyCredentialsChanged() error {
	if s.credentialsChanged == nil {
		return nil
	}
	return s.credentialsChanged()
}

type CreateInput struct {
	Username     string     `json:"username" binding:"required"`
	Password     string     `json:"password"`
	UUID         string     `json:"uuid"`
	TrafficLimit int64      `json:"traffic_limit"`
	IPLimit      int        `json:"ip_limit"`
	Remark       string     `json:"remark"`
	ExpireTime   *time.Time `json:"expire_time"`
	Enabled      *bool      `json:"enabled"`
}

type UpdateInput struct {
	Username     string     `json:"username"`
	Password     string     `json:"password"`
	UUID         string     `json:"uuid"`
	TrafficLimit *int64     `json:"traffic_limit"`
	IPLimit      *int       `json:"ip_limit"`
	Remark       *string    `json:"remark"`
	ExpireTime   *time.Time `json:"expire_time"`
	Enabled      *bool      `json:"enabled"`
}

type Credential struct{ Username, Password, UUID string }

func (s *Service) Create(in CreateInput) (*models.ProxyUser, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return nil, errors.New("username is required")
	}
	password := in.Password
	var err error
	if password == "" {
		password, err = randomToken(24)
		if err != nil {
			return nil, fmt.Errorf("generate proxy user password: %w", err)
		}
	}
	uuid := in.UUID
	if uuid == "" {
		var err error
		uuid, err = newUUID()
		if err != nil {
			return nil, err
		}
	}
	expire := time.Time{}
	if in.ExpireTime != nil {
		expire = in.ExpireTime.UTC()
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	encrypted, err := encryptPassword(password)
	if err != nil {
		return nil, err
	}
	subTok, _ := randomHex(16)
	u := &models.ProxyUser{
		Username:          username,
		PasswordEncrypted: encrypted,
		UUID:              uuid,
		TrafficLimit:      in.TrafficLimit,
		IPLimit:           max0(in.IPLimit),
		Remark:            strings.TrimSpace(in.Remark),
		ExpireTime:        expire,
		Enabled:           enabled,
		SubToken:          subTok,
	}
	if err := s.db.Create(u).Error; err != nil {
		return nil, fmt.Errorf("create proxy user: %w", err)
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return u, fmt.Errorf("proxy user created, but Mihomo configuration could not be updated: %w", err)
	}
	return u, nil
}
