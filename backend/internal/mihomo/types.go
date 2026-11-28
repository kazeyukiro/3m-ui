package mihomo

import "time"

// StatusResponse represents the response sent to the frontend status endpoint
type StatusResponse struct {
	Running bool   `json:"running"`
	Version string `json:"version"`
	PID     int    `json:"pid"`
	Uptime  string `json:"uptime"`
}

// VersionInfo represents the parsed mihomo -v output
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Build   string `json:"build"`
}

// LogResponse represents log stream or mock log records
type LogResponse struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Payload   string    `json:"payload"`
}
