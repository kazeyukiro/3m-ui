package user

import (
	"fmt"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// ResetTraffic clears upload/download/used counters for a proxy user and
// triggers a Mihomo credential reload so previously over-quota users can
// reconnect when still otherwise eligible.
func (s *Service) ResetTraffic(id uint) (*models.ProxyUser, error) {
	u, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	u.TrafficUsed = 0
	u.UploadBytes = 0
	u.DownloadBytes = 0
	if err := s.db.Model(u).Select("TrafficUsed", "UploadBytes", "DownloadBytes").Updates(u).Error; err != nil {
		return nil, fmt.Errorf("reset traffic: %w", err)
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return u, fmt.Errorf("traffic reset, but Mihomo configuration could not be updated: %w", err)
	}
	return u, nil
}
