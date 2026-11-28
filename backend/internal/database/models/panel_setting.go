package models

type PanelSetting struct {
	BaseModel

	Key   string `gorm:"size:100;uniqueIndex;not null" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}
