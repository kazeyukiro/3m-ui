package models

import "gorm.io/gorm"

// PanelSetting is a simple key/value store for panel-level options that are
// edited from the UI (Telegram bot, ACME paths, etc.) without rewriting the
// process YAML config file.
type PanelSetting struct {
	gorm.Model

	Key   string `gorm:"size:100;uniqueIndex;not null" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}
