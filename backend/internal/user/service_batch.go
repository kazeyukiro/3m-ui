package user

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

// ListFilter supports search across proxy users.
type ListFilter struct {
	Query   string // username / remark substring
	Enabled *bool
	Online  *bool
	Blocked *bool // computed from IsCredentialActive
}

func (s *Service) ListFiltered(f ListFilter) ([]models.ProxyUser, error) {
	q := s.db.Model(&models.ProxyUser{}).Order("id desc")
	if term := strings.TrimSpace(f.Query); term != "" {
		like := "%" + term + "%"
		q = q.Where("username LIKE ? OR remark LIKE ?", like, like)
	}
	if f.Enabled != nil {
		q = q.Where("enabled = ?", *f.Enabled)
	}
	if f.Online != nil {
		q = q.Where("online = ?", *f.Online)
	}
	var users []models.ProxyUser
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	if f.Blocked == nil {
		return users, nil
	}
	out := make([]models.ProxyUser, 0, len(users))
	for _, u := range users {
		blocked := !IsCredentialActive(u)
		if blocked == *f.Blocked {
			out = append(out, u)
		}
	}
	return out, nil
}

// BatchAction is a bulk operation over proxy user IDs.
type BatchAction string

const (
	BatchEnable       BatchAction = "enable"
	BatchDisable      BatchAction = "disable"
	BatchResetTraffic BatchAction = "reset-traffic"
	BatchDelete       BatchAction = "delete"
)

func (s *Service) Batch(action BatchAction, ids []uint) (int, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids is required")
	}
	// de-dupe
	seen := map[uint]struct{}{}
	clean := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return 0, errors.New("ids is required")
	}

	switch action {
	case BatchEnable:
		res := s.db.Model(&models.ProxyUser{}).Where("id IN ?", clean).Update("enabled", true)
		if res.Error != nil {
			return 0, res.Error
		}
		if err := s.notifyCredentialsChanged(); err != nil {
			log.Printf("warning: credentials changed notification failed: %v", err)
		}
		return int(res.RowsAffected), nil
	case BatchDisable:
		res := s.db.Model(&models.ProxyUser{}).Where("id IN ?", clean).Update("enabled", false)
		if res.Error != nil {
			return 0, res.Error
		}
		if err := s.notifyCredentialsChanged(); err != nil {
			log.Printf("warning: credentials changed notification failed: %v", err)
		}
		return int(res.RowsAffected), nil
	case BatchResetTraffic:
		res := s.db.Model(&models.ProxyUser{}).Where("id IN ?", clean).Updates(map[string]interface{}{
			"traffic_used":   0,
			"upload_bytes":   0,
			"download_bytes": 0,
		})
		if res.Error != nil {
			return 0, res.Error
		}
		return int(res.RowsAffected), nil
	case BatchDelete:
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Unscoped().Where("proxy_user_id IN ?", clean).Delete(&models.ListenerUser{}).Error; err != nil {
				return err
			}
			return tx.Unscoped().Where("id IN ?", clean).Delete(&models.ProxyUser{}).Error
		}); err != nil {
			return 0, err
		}
		if err := s.notifyCredentialsChanged(); err != nil {
			log.Printf("warning: credentials changed notification failed: %v", err)
		}
		return len(clean), nil
	default:
		return 0, fmt.Errorf("unknown batch action %q (enable|disable|reset-traffic|delete)", action)
	}
}
