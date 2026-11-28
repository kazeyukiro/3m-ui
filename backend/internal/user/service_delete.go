package user

import (
	"fmt"
	"log"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

func (s *Service) Delete(id uint) error {
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		// Hard-delete bindings + user so the unique username/uuid indexes can be
		// reused (soft-delete would block recreate-after-delete).
		if err := tx.Unscoped().Where("proxy_user_id = ?", id).Delete(&models.ListenerUser{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&models.ProxyUser{}, id).Error
	}); err != nil {
		return err
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return fmt.Errorf("proxy user deleted, but Mihomo configuration could not be updated: %w", err)
	}
	return nil
}

// DeleteDepleted removes proxy users that are expired or over traffic quota
// (remove expired or over-quota users). Merely disabled users are kept so admins
// can re-enable them. Returns the number of deleted rows.
func (s *Service) DeleteDepleted() (int, error) {
	var users []models.ProxyUser
	if err := s.db.Find(&users).Error; err != nil {
		return 0, err
	}
	ids := make([]uint, 0)
	now := time.Now()
	for _, u := range users {
		expired := !u.ExpireTime.IsZero() && !u.ExpireTime.After(now)
		overQuota := u.TrafficLimit > 0 && u.TrafficUsed >= u.TrafficLimit
		if expired || overQuota {
			ids = append(ids, u.ID)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("proxy_user_id IN ?", ids).Delete(&models.ListenerUser{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Where("id IN ?", ids).Delete(&models.ProxyUser{}).Error
	}); err != nil {
		return 0, err
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		log.Printf("warning: credentials changed notification failed: %v", err)
	}
	return len(ids), nil
}
