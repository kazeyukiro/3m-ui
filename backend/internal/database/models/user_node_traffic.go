package models

// UserNodeTraffic stores per-listener raw traffic for a proxy user.
// ProxyUser.TrafficUsed accumulates billable bytes (raw × node multiplier).
type UserNodeTraffic struct {
	BaseModel

	ProxyUserID   uint  `gorm:"column:proxy_user_id;not null;uniqueIndex:uidx_user_listener_traffic,priority:1;index" json:"proxy_user_id"`
	ListenerID    uint  `gorm:"column:listener_id;not null;uniqueIndex:uidx_user_listener_traffic,priority:2;index" json:"listener_id"`
	UploadBytes   int64 `gorm:"column:upload_bytes;not null;default:0" json:"upload_bytes"`
	DownloadBytes int64 `gorm:"column:download_bytes;not null;default:0" json:"download_bytes"`
	TrafficUsed   int64 `gorm:"column:traffic_used;not null;default:0" json:"traffic_used"`
}

func (UserNodeTraffic) TableName() string { return "user_node_traffics" }
