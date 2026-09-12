package user

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// reclaimSoftDeletedUsername renames soft-deleted rows that still hold username
// (and uuid when provided) so SQLite UNIQUE indexes can be reused after recreate.
func (s *Service) reclaimSoftDeletedUsername(username string, excludeID uint) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}
	var active int64
	q := s.db.Model(&models.ProxyUser{}).Where("username = ?", username)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&active).Error; err != nil {
		return fmt.Errorf("check username uniqueness: %w", err)
	}
	if active > 0 {
		return fmt.Errorf("username %q already exists", username)
	}
	var soft []models.ProxyUser
	sq := s.db.Unscoped().Where("username = ? AND deleted_at IS NOT NULL", username)
	if excludeID != 0 {
		sq = sq.Where("id <> ?", excludeID)
	}
	if err := sq.Find(&soft).Error; err != nil {
		return fmt.Errorf("check soft-deleted usernames: %w", err)
	}
	for _, row := range soft {
		newName := fmt.Sprintf("%s__deleted_%d", username, row.ID)
		newUUID := fmt.Sprintf("%s-deleted-%d", row.UUID, row.ID)
		if len(newUUID) > 64 {
			newUUID = fmt.Sprintf("deleted-%d", row.ID)
		}
		newTok := fmt.Sprintf("%s_del_%d", row.SubToken, row.ID)
		if len(newTok) > 64 {
			newTok = fmt.Sprintf("del_%d", row.ID)
		}
		if err := s.db.Unscoped().Model(&models.ProxyUser{}).Where("id = ?", row.ID).Updates(map[string]any{
			"username":  newName,
			"uuid":      newUUID,
			"sub_token": newTok,
		}).Error; err != nil {
			return fmt.Errorf("reclaim soft-deleted username %q: %w", username, err)
		}
	}
	return nil
}

func (s *Service) Create(in CreateInput) (*models.ProxyUser, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return nil, errors.New("username is required")
	}
	if err := s.reclaimSoftDeletedUsername(username, 0); err != nil {
		return nil, err
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
	subTok, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generate proxy user sub token: %w", err)
	}
	u := &models.ProxyUser{
		Username:          username,
		PasswordEncrypted: encrypted,
		UUID:              uuid,
		TrafficLimit:      in.TrafficLimit,
		IPLimit:           max0(in.IPLimit),
		HWIDLimit:         max0(in.HWIDLimit),
		Remark:            strings.TrimSpace(in.Remark),
		Group:             strings.TrimSpace(in.Group),
		Tags:              strings.TrimSpace(in.Tags),
		TrafficResetDays:  max0(in.TrafficResetDays),
		StartOnFirstUse:      in.StartOnFirstUse,
		ExpireDaysAfterFirst:  max0(in.ExpireDaysAfterFirst),
		ExternalLinks:         strings.TrimSpace(in.ExternalLinks),
		ExpireRenewDays:   max0(in.ExpireRenewDays),
		ExpireTime:        expire,
		Enabled:           enabled,
		SubToken:          subTok,
		TelegramID:        in.TelegramID,
		TelegramName:      strings.TrimSpace(in.TelegramName),
	}
	if err := s.db.Create(u).Error; err != nil {
		msg := err.Error()
		if strings.Contains(msg, "UNIQUE") && strings.Contains(msg, "username") {
			return nil, fmt.Errorf("username %q already exists", username)
		}
		if strings.Contains(msg, "UNIQUE") && strings.Contains(msg, "uuid") {
			return nil, fmt.Errorf("uuid already exists; leave empty to auto-generate")
		}
		return nil, fmt.Errorf("create proxy user: %w", err)
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		// The proxy user has ALREADY been persisted to the DB at this point, so
		// returning an error here would cause the API layer to translate it into
		// a 400/500 — even though the user was created successfully. The only
		// thing that failed is the Mihomo config hot-reload, which the next
		// scheduler/enforcer tick (or any later credential change) will retry.
		// Log the warning and return the created user so the caller sees success.
		log.Printf("warning: proxy user %d created, but Mihomo config could not be updated: %v", u.ID, err)
	}
	return u, nil
}

func (s *Service) Update(id uint, in UpdateInput) (*models.ProxyUser, error) {
	u, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Username) != "" {
		next := strings.TrimSpace(in.Username)
		if next != u.Username {
			if err := s.reclaimSoftDeletedUsername(next, id); err != nil {
				return nil, err
			}
		}
		u.Username = next
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
	if in.HWIDLimit != nil {
		u.HWIDLimit = max0(*in.HWIDLimit)
	}
	if in.Remark != nil {
		u.Remark = strings.TrimSpace(*in.Remark)
	}
	if in.Group != nil {
		u.Group = strings.TrimSpace(*in.Group)
	}
	if in.Tags != nil {
		u.Tags = strings.TrimSpace(*in.Tags)
	}
	if in.TrafficResetDays != nil {
		u.TrafficResetDays = max0(*in.TrafficResetDays)
	}
	if in.ExpireRenewDays != nil {
		u.ExpireRenewDays = max0(*in.ExpireRenewDays)
	}
	if in.StartOnFirstUse != nil {
		u.StartOnFirstUse = *in.StartOnFirstUse
	}
	if in.ExpireDaysAfterFirst != nil {
		u.ExpireDaysAfterFirst = max0(*in.ExpireDaysAfterFirst)
	}
	if in.ExternalLinks != nil {
		u.ExternalLinks = strings.TrimSpace(*in.ExternalLinks)
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
