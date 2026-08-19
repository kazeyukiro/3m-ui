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

// NOTE: full file restored in follow-up if truncated — critical methods below
func (s *Service) Create(in CreateInput) (*models.ProxyUser, error) {
	return nil, errors.New("incomplete restore — see local tree")
}
