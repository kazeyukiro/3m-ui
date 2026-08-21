package telegram

import (
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

func cpuPercentSample() ([]float64, error) {
	return cpu.Percent(150*time.Millisecond, false)
}
