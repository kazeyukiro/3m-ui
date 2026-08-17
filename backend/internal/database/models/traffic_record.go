package models

type TrafficRecord struct {
	BaseModel

	ProxyUserID   uint  `gorm:"index;not null" json:"proxy_user_id"`
	UploadBytes   int64 `gorm:"not null;default:0" json:"upload_bytes"`
	DownloadBytes int64 `gorm:"not null;default:0" json:"download_bytes"`
	Online        bool  `gorm:"default:false" json:"online"`
}
