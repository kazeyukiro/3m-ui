package user

import (
	"errors"
	"fmt"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

func (s *Service) BindListeners(userID uint, listenerIDs []uint) error {
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var user models.ProxyUser
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		desired := make([]uint, 0, len(listenerIDs))
		seen := map[uint]struct{}{}
		for _, id := range listenerIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			desired = append(desired, id)
		}
		if len(desired) > 0 {
			var count int64
			if err := tx.Model(&models.Listener{}).Where("id IN ?", desired).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(desired)) {
				return errors.New("one or more listener ids do not exist")
			}
			if err := tx.Where("proxy_user_id = ? AND listener_id NOT IN ?", userID, desired).Delete(&models.ListenerUser{}).Error; err != nil {
				return err
			}
		} else if err := tx.Where("proxy_user_id = ?", userID).Delete(&models.ListenerUser{}).Error; err != nil {
			return err
		}
		for _, listenerID := range desired {
			var binding models.ListenerUser
			result := tx.Unscoped().Where("listener_id = ? AND proxy_user_id = ?", listenerID, userID).Find(&binding)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				if binding.DeletedAt.Valid {
					if err := tx.Unscoped().Model(&binding).Update("deleted_at", nil).Error; err != nil {
						return err
					}
				}
				continue
			}
			if err := tx.Create(&models.ListenerUser{ListenerID: listenerID, ProxyUserID: userID}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return fmt.Errorf("listener bindings updated, but Mihomo configuration could not be updated: %w", err)
	}
	return nil
}
