package traffic

import (
	"sync"
	"time"
)

type Service struct {
	mu       sync.Mutex
	last     Snapshot
	lastTime time.Time
}

func NewService() *Service {
	return &Service{lastTime: time.Now()}
}

func (s *Service) Update(totalUpload, totalDownload int64, connections int) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	seconds := now.Sub(s.lastTime).Seconds()
	result := Snapshot{
		UploadBytes:   totalUpload,
		DownloadBytes: totalDownload,
		Connections:   connections,
	}

	firstCall := s.last.UploadBytes == 0 && s.last.DownloadBytes == 0
	if seconds > 0 && !firstCall {
		uploadDelta := totalUpload - s.last.UploadBytes
		downloadDelta := totalDownload - s.last.DownloadBytes
		// Mihomo counters can reset when the core restarts. Never expose a
		// negative bandwidth rate during that transition.
		if uploadDelta < 0 {
			uploadDelta = 0
		}
		if downloadDelta < 0 {
			downloadDelta = 0
		}
		result.UploadRate = int64(float64(uploadDelta) / seconds)
		result.DownloadRate = int64(float64(downloadDelta) / seconds)
	}
	// On the very first call s.last is the zero Snapshot, so the delta would
	// equal the entire cumulative counter and the rate would be a meaningless
	// "total traffic since process start / seconds since NewService" value.
	// Skip rate computation on the first sample; UploadRate/DownloadRate stay 0
	// and the next call will produce a correct delta-based rate.

	s.last = result
	s.lastTime = now
	return result
}

// ApplySample records a snapshot using an upload/download rate obtained
// directly from Mihomo's /traffic endpoint (an instantaneous reading)
// instead of deriving the rate from the delta between two polls. Cumulative
// totals and connection count still come from /connections. Used by the
// traffic collector; Update above is kept for existing callers/tests that
// only have cumulative totals.
func (s *Service) ApplySample(totalUpload, totalDownload int64, connections int, uploadRate, downloadRate int64) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := Snapshot{
		UploadBytes:   totalUpload,
		DownloadBytes: totalDownload,
		Connections:   connections,
		UploadRate:    uploadRate,
		DownloadRate:  downloadRate,
	}
	s.last = result
	s.lastTime = time.Now()
	return result
}

// Current returns the most recently recorded snapshot without mutating
// state, so HTTP handlers can read it without perturbing rate calculation.
func (s *Service) Current() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}
