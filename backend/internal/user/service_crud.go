package user

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

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
		TelegramID:        in.TelegramID,
		TelegramName:      strings.TrimSpace(in.TelegramName),
	}
	if err := s.db.Create(u).Error; err != nil {
		return nil, fmt.Errorf("create proxy user: %w", err)
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return u, fmt.Errorf("proxy user created, but Mihomo configuration could not be updated: %w", err)
	}
	return u, nil
}

func (s *Service) Update(id uint, in UpdateInput) (*models.ProxyUser, error) {
	u, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Username) != "" {
		u.Username = strings.TrimSpace(in.Username)
	}
	if in.Password != "" {
		u.PasswordEncrypted, err = encryptPassword(in.Password)
		if err != nil {
			return nil, err
		}
	}
	if in.UUID != "" {
		u.UUID = in.UUID
	}
	if in.TrafficLimit != nil {
		u.TrafficLimit = *in.TrafficLimit
	}
	if in.IPLimit != nil {
		u.IPLimit = max0(*in.IPLimit)
	}
	if in.Remark != nil {
		u.Remark = strings.TrimSpace(*in.Remark)
	}
	if in.ExpireTime != nil {
		u.ExpireTime = in.ExpireTime.UTC()
	}
	if in.Enabled != nil {
		u.Enabled = *in.Enabled
	}
	if in.TelegramID != nil {
		u.TelegramID = *in.TelegramID
	}
	if in.TelegramName != nil {
		u.TelegramName = strings.TrimSpace(*in.TelegramName)
	}
	if err := s.db.Save(u).Error; err != nil {
		return nil, fmt.Errorf("update proxy user: %w", err)
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return u, fmt.Errorf("proxy user updated, but Mihomo configuration could not be updated: %w", err)
	}
	return u, nil
}
