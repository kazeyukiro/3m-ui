package traffic

import "time"

type Snapshot struct {
	UploadBytes   int64 `json:"upload_bytes"`
	DownloadBytes int64 `json:"download_bytes"`
	UploadRate    int64 `json:"upload_rate"`
	DownloadRate  int64 `json:"download_rate"`
	Connections   int   `json:"connections"`
}

// UserTraffic is the per-user payload for GET /api/v1/traffic/users.
// UserID/UploadBytes/DownloadBytes/Online are the original fields; the rest
// are additive so existing consumers keep working.
type UserTraffic struct {
	UserID        uint       `json:"user_id"`
	Username      string     `json:"username"`
	UploadBytes   int64      `json:"upload_bytes"`
	DownloadBytes int64      `json:"download_bytes"`
	Online        bool       `json:"online"`
	TrafficUsed   int64      `json:"traffic_used"`
	TrafficLimit  int64      `json:"traffic_limit"`
	ExpireTime    time.Time  `json:"expire_time"`
	LastSeen      *time.Time `json:"last_seen"`
	Blocked       bool       `json:"blocked"`
}

// ConnectionView is the per-connection payload for
// GET /api/v1/traffic/connections. It reflects the ProxyUser ->
// ListenerUser -> Listener mapping without introducing any new permission
// model: ListenerID/ListenerName come from the existing Listener model, and
// ProxyUserID/Username are only populated when the connection could be
// attributed without guessing (see collector.go). Unattributed connections
// have ProxyUserID == nil and Username == "".
type ConnectionView struct {
	ID              string   `json:"id"`
	ListenerID      *uint    `json:"listener_id"`
	ListenerName    string   `json:"listener_name"`
	ProxyUserID     *uint    `json:"proxy_user_id"`
	Username        string   `json:"username"`
	Network         string   `json:"network"`
	Host            string   `json:"host"`
	SourceIP        string   `json:"source_ip"`
	DestinationIP   string   `json:"destination_ip"`
	DestinationPort string   `json:"destination_port"`
	Upload          int64    `json:"upload"`
	Download        int64    `json:"download"`
	Rule            string   `json:"rule,omitempty"`
	Chains          []string `json:"chains,omitempty"`
	Start           string   `json:"start,omitempty"`
}
