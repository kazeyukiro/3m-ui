package traffic

import (
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// AddSample records an incremental (per-tick delta, not cumulative) upload
// and download sample for a proxy user: it appends a TrafficRecord history
// row and updates ProxyUser's cumulative counters (TrafficUsed,
// UploadBytes, DownloadBytes) plus LastSeen/Online when online is true.
func (s *UserService) AddSample(userID uint, up, down int64, online bool) error {
	record := &models.TrafficRecord{
		ProxyUserID:   userID,
		UploadBytes:   up,
		DownloadBytes: down,
		Online:        online,
	}
	if err := s.db.Create(record).Error; err != nil {
		return err
	}

	updates := map[string]any{
		"traffic_used":   gorm.Expr("traffic_used + ?", up+down),
		"upload_bytes":   gorm.Expr("upload_bytes + ?", up),
		"download_bytes": gorm.Expr("download_bytes + ?", down),
	}
	if online {
		now := time.Now()
		updates["last_seen"] = now
		updates["online"] = true
	}

	return s.db.Model(&models.ProxyUser{}).
		Where("id = ?", userID).
		Updates(updates).Error
}

// MarkOnline sets online=true and refreshes last_seen for users that have an
// attributed connection this tick, even when traffic deltas are zero (idle
// keep-alive). Without this, a connected user with no byte transfer stays offline.
func (s *UserService) MarkOnline(userIDs []uint) error {
	if len(userIDs) == 0 {
		return nil
	}
	now := time.Now()
	return s.db.Model(&models.ProxyUser{}).
		Where("id IN ?", userIDs).
		Updates(map[string]any{
			"online":    true,
			"last_seen": now,
		}).Error
}

// MarkOffline clears the Online flag for every proxy user not present in
// the given set of currently-active user IDs. Called once per collection
// tick after AddSample/MarkOnline has marked the currently-seen users online.
// An empty active set marks every currently-online user offline.
func (s *UserService) MarkOffline(activeUserIDs []uint) error {
	q := s.db.Model(&models.ProxyUser{}).Where("online = ?", true)
	if len(activeUserIDs) > 0 {
		q = q.Where("id NOT IN ?", activeUserIDs)
	}
	return q.Update("online", false).Error
}

func IsExpired(u models.ProxyUser) bool {
	return !u.ExpireTime.IsZero() && u.ExpireTime.Before(time.Now())
}
