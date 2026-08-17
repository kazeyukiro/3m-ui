package telegram

import (
	"encoding/json"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const digestDatePrefix = "telegram_daily_digest_"
const wizardPrefix = "telegram_wizard_"

func dailyDigestSent(db *gorm.DB, day string) bool {
	if db == nil || day == "" { return false }
	var row models.PanelSetting
	return db.Where("key = ?", digestDatePrefix+day).First(&row).Error == nil
}

func markDailyDigestSent(db *gorm.DB, day string) error {
	if db == nil || day == "" { return nil }
	key := digestDatePrefix + day
	var row models.PanelSetting
	err := db.Where("key = ?", key).First(&row).Error
	if err == nil { return nil }
	if err != gorm.ErrRecordNotFound { return err }
	return db.Create(&models.PanelSetting{Key: key, Value: day}).Error
}

func saveWizardState(db *gorm.DB, chatID string, state addClientWizard) error {
	if db == nil || strings.TrimSpace(chatID) == "" { return nil }
	raw, err := json.Marshal(state)
	if err != nil { return err }
	key := wizardPrefix + strings.TrimSpace(chatID)
	var row models.PanelSetting
	err = db.Where("key = ?", key).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return db.Create(&models.PanelSetting{Key: key, Value: string(raw)}).Error
	}
	if err != nil { return err }
	row.Value = string(raw)
	return db.Save(&row).Error
}

func loadWizardState(db *gorm.DB, chatID string) (*addClientWizard, error) {
	if db == nil || strings.TrimSpace(chatID) == "" { return nil, nil }
	var row models.PanelSetting
	if err := db.Where("key = ?", wizardPrefix+strings.TrimSpace(chatID)).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound { return nil, nil }
		return nil, err
	}
	var state addClientWizard
	if err := json.Unmarshal([]byte(row.Value), &state); err != nil { return nil, err }
	return &state, nil
}

func clearWizardState(db *gorm.DB, chatID string) error {
	if db == nil || strings.TrimSpace(chatID) == "" { return nil }
	return db.Where("key = ?", wizardPrefix+strings.TrimSpace(chatID)).Delete(&models.PanelSetting{}).Error
}
