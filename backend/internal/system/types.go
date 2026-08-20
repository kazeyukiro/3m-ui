package system

type CPUInfo struct {
	Percent float64 `json:"percent"` // 0–100
}

type MemoryInfo struct {
	Used    float64 `json:"used"`    // bytes
	Total   float64 `json:"total"`   // bytes
	Percent float64 `json:"percent"` // 0–100
}

type DiskInfo struct {
	Used    float64 `json:"used"`    // bytes
	Total   float64 `json:"total"`   // bytes
	Percent float64 `json:"percent"` // 0–100
}

type NetworkInfo struct {
	Upload   float64 `json:"upload"`   // bytes/sec
	Download float64 `json:"download"` // bytes/sec
}

type SystemStats struct {
	CPU     CPUInfo     `json:"cpu"`
	Memory  MemoryInfo  `json:"memory"`
	Disk    DiskInfo    `json:"disk"`
	Network NetworkInfo `json:"network"`
}
