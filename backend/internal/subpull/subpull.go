package subpull

import (
	"errors"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const window = 24 * time.Hour

var ErrLimitReached = errors.New("subscription pull limit reached")

// Enforce increments the user's successful subscription-pull counter when
// limit > 0. limit <= 0 means unlimited. Uses a rolling 24h window from the
// first pull in the window.
func Enforce(db *gorm.DB, userID uint, limit int) error {
	if db == nil || userID == 0 || limit <= 0 {
		return nil
	}
	now := time.Now().UTC()
	return db.Transaction(func(tx *gorm.DB) error {
		var u models.ProxyUser
		if err := tx.
			Select("id", "sub_pull_limit", "sub_pull_count", "sub_pull_window_start").
			First(&u, userID).Error; err != nil {
			return err
		}
		// Prefer DB value if caller passed stale limit; fall back to argument.
		lim := u.SubPullLimit
		if lim <= 0 {
			lim = limit
		}
		if lim <= 0 {
			return nil
		}
		count := u.SubPullCount
		start := u.SubPullWindowStart
		if start == nil || now.Sub(*start) >= window {
			count = 0
			start = &now
		}
		if count >= lim {
			return ErrLimitReached
		}
		count++
		return tx.Model(&models.ProxyUser{}).Where("id = ?", userID).Updates(map[string]interface{}{
			"sub_pull_count":        count,
			"sub_pull_window_start": start,
		}).Error
	})
}
