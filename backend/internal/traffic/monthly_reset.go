package traffic

import (
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

// MaybeResetMonthlyTraffic resets all users' traffic counters when today matches
// the configured reset day (1-31) in PanelSetting key "traffic_reset_day".
func MaybeResetMonthlyTraffic(db *gorm.DB) {
	if db == nil {
		return
	}
	var daySetting models.PanelSetting
	if err := db.Where("key = ?", "traffic_reset_day").First(&daySetting).Error; err != nil {
		return
	}
	day, err := strconv.Atoi(daySetting.Value)
	if err != nil || day < 1 || day > 31 {
		return
	}
	now := time.Now()
	if now.Day() != day {
		return
	}
	today := now.Format("2006-01-02")
	var last models.PanelSetting
	if err := db.Where("key = ?", "traffic_reset_last").First(&last).Error; err == nil && last.Value == today {
		return
	}

	// Claim the day BEFORE resetting traffic. If the reset itself fails (e.g.
	// DB locked), the next tick will see today already claimed and skip the
	// reset rather than double-counting. If the claim fails, do not reset —
	// safer to skip a day than to reset twice.
	var claimed bool
	if err := db.Transaction(func(tx *gorm.DB) error {
		var existing models.PanelSetting
		err := tx.Where("key = ?", "traffic_reset_last").First(&existing).Error
		if err == nil {
			if existing.Value == today {
				// Another worker already claimed today.
				return nil
			}
			existing.Value = today
			if uerr := tx.Save(&existing).Error; uerr != nil {
				return uerr
			}
			claimed = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if cerr := tx.Create(&models.PanelSetting{Key: "traffic_reset_last", Value: today}).Error; cerr != nil {
			return cerr
		}
		claimed = true
		return nil
	}); err != nil {
		log.Printf("traffic: failed to claim monthly reset day: %v", err)
		return
	}
	if !claimed {
		// Another worker already claimed today within the transaction.
		return
	}

	res := db.Model(&models.ProxyUser{}).Where("1 = 1").Updates(map[string]interface{}{
		"traffic_used":   0,
		"upload_bytes":   0,
		"download_bytes": 0,
	})
	if res.Error != nil {
		log.Printf("traffic: monthly reset failed (day already claimed): %v", res.Error)
		return
	}
	log.Printf("traffic: monthly reset applied for day %d (%d users)", day, res.RowsAffected)
}
