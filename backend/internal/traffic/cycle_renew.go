package traffic

import (
	"log"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

// MaybeApplyUserCycles applies per-user traffic reset and expire renewal.
// Safe to call frequently; each user is updated at most once per cycle window.
func MaybeApplyUserCycles(db *gorm.DB) {
	if db == nil {
		return
	}
	var users []models.ProxyUser
	if err := db.Where("traffic_reset_days > 0 OR expire_renew_days > 0").Find(&users).Error; err != nil {
		log.Printf("traffic: cycle query failed: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, u := range users {
		updates := map[string]interface{}{}
		if u.TrafficResetDays > 0 {
			need := false
			if u.LastTrafficCycleReset == nil {
				need = true
			} else {
				next := u.LastTrafficCycleReset.Add(time.Duration(u.TrafficResetDays) * 24 * time.Hour)
				if !now.Before(next) {
					need = true
				}
			}
			if need {
				updates["traffic_used"] = 0
				updates["upload_bytes"] = 0
				updates["download_bytes"] = 0
				updates["last_traffic_cycle_reset"] = now
			}
		}
		if u.ExpireRenewDays > 0 && !u.ExpireTime.IsZero() {
			// Renew when expired or within 24h of expiry.
			window := u.ExpireTime.Add(-24 * time.Hour)
			if !now.Before(window) {
				base := u.ExpireTime
				if base.Before(now) {
					base = now
				}
				updates["expire_time"] = base.Add(time.Duration(u.ExpireRenewDays) * 24 * time.Hour)
			}
		}
		if len(updates) == 0 {
			continue
		}
		if err := db.Model(&models.ProxyUser{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			log.Printf("traffic: cycle update user %d failed: %v", u.ID, err)
		}
	}
}
