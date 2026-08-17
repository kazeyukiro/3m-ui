package traffic

import (
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
	res := db.Model(&models.ProxyUser{}).Where("1 = 1").Updates(map[string]interface{}{
		"traffic_used":   0,
		"upload_bytes":   0,
		"download_bytes": 0,
	})
	if res.Error != nil {
		log.Printf("traffic: monthly reset failed: %v", res.Error)
		return
	}
	if err := db.Where("key = ?", "traffic_reset_last").First(&last).Error; err == gorm.ErrRecordNotFound {
		_ = db.Create(&models.PanelSetting{Key: "traffic_reset_last", Value: today}).Error
	} else if err == nil {
		last.Value = today
		_ = db.Save(&last).Error
	}
	log.Printf("traffic: monthly reset applied for day %d (%d users)", day, res.RowsAffected)
}
