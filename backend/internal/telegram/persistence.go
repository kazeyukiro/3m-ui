package telegram

import (
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const digestDatePrefix = "telegram_daily_digest_"

func dailyDigestSent(db *gorm.DB, day string) bool {
	if db == nil || day == "" {
		return false
	}
	var row models.PanelSetting
	return db.Where("key = ?", digestDatePrefix+day).First(&row).Error == nil
}

func markDailyDigestSent(db *gorm.DB, day string) error {
	if db == nil || day == "" {
		return nil
	}
	key := digestDatePrefix + day
	var row models.PanelSetting
	err := db.Where("key = ?", key).First(&row).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return db.Create(&models.PanelSetting{Key: key, Value: day}).Error
}
