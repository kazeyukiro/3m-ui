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

// NodeDelta is raw byte growth on one listener for one user in a collection tick.
type NodeDelta struct {
	ListenerID uint
	Up, Down   int64
	Multiplier float64
}

func clampMultiplier(m float64) float64 {
	if m <= 0 {
		return 1
	}
	if m > 100 {
		return 100
	}
	return m
}

func billable(raw int64, mult float64) int64 {
	mult = clampMultiplier(mult)
	if raw == 0 {
		return 0
	}
	return int64(float64(raw)*mult + 0.5)
}

// AddSample records global counters (multiplier already applied by caller for billable totals).
func (s *UserService) AddSample(userID uint, up, down int64, online bool) error {
	return s.AddSampleDetailed(userID, up, down, nil, online)
}

// AddSampleDetailed updates user billable totals and optional per-node raw counters.
// up/down must already be billable (after node multipliers). nodeParts carry raw bytes.
func (s *UserService) AddSampleDetailed(userID uint, billedUp, billedDown int64, nodeParts []NodeDelta, online bool) error {
	if billedUp == 0 && billedDown == 0 && len(nodeParts) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if billedUp != 0 || billedDown != 0 {
			record := &models.TrafficRecord{
				ProxyUserID:   userID,
				UploadBytes:   billedUp,
				DownloadBytes: billedDown,
				Online:        online,
			}
			if err := tx.Create(record).Error; err != nil {
				return err
			}
			updates := map[string]any{
				"traffic_used":   gorm.Expr("traffic_used + ?", billedUp+billedDown),
				"upload_bytes":   gorm.Expr("upload_bytes + ?", billedUp),
				"download_bytes": gorm.Expr("download_bytes + ?", billedDown),
			}
			if online {
				now := time.Now()
				updates["last_seen"] = now
				updates["online"] = true
			}
			if err := tx.Model(&models.ProxyUser{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
				return err
			}
		}
		for _, p := range nodeParts {
			if p.ListenerID == 0 || (p.Up == 0 && p.Down == 0) {
				continue
			}
			raw := p.Up + p.Down
			var existing models.UserNodeTraffic
			qerr := tx.Where("proxy_user_id = ? AND listener_id = ?", userID, p.ListenerID).First(&existing).Error
			if qerr == gorm.ErrRecordNotFound {
				row := models.UserNodeTraffic{
					ProxyUserID:   userID,
					ListenerID:    p.ListenerID,
					UploadBytes:   p.Up,
					DownloadBytes: p.Down,
					TrafficUsed:   raw,
				}
				if cerr := tx.Create(&row).Error; cerr != nil {
					return cerr
				}
				continue
			}
			if qerr != nil {
				return qerr
			}
			if err := tx.Model(&existing).Updates(map[string]any{
				"upload_bytes":   gorm.Expr("upload_bytes + ?", p.Up),
				"download_bytes": gorm.Expr("download_bytes + ?", p.Down),
				"traffic_used":   gorm.Expr("traffic_used + ?", raw),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// OnlineGrace keeps a user marked online for this long after the last tick
// that saw an attributed connection. Without it, a single empty /connections
// snapshot (common during brief idle or API blips) flips the UI to offline.
const OnlineGrace = 5 * time.Minute

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

// MarkOffline clears online for users not seen this tick, but only after
// OnlineGrace since last_seen. Active IDs are never cleared.
func (s *UserService) MarkOffline(activeUserIDs []uint) error {
	return s.markOffline(activeUserIDs, OnlineGrace)
}

// MarkOfflineImmediate clears online without grace (e.g. Mihomo unreachable).
func (s *UserService) MarkOfflineImmediate(activeUserIDs []uint) error {
	return s.markOffline(activeUserIDs, 0)
}

func (s *UserService) markOffline(activeUserIDs []uint, grace time.Duration) error {
	q := s.db.Model(&models.ProxyUser{}).Where("online = ?", true)
	if len(activeUserIDs) > 0 {
		q = q.Where("id NOT IN ?", activeUserIDs)
	}
	if grace > 0 {
		cutoff := time.Now().Add(-grace)
		q = q.Where("last_seen IS NULL OR last_seen < ?", cutoff)
	}
	return q.Update("online", false).Error
}

// ClearNodeTraffic removes per-node counters for a user (e.g. after reset).
func (s *UserService) ClearNodeTraffic(userID uint) error {
	return s.db.Unscoped().Where("proxy_user_id = ?", userID).Delete(&models.UserNodeTraffic{}).Error
}

// ClearAllNodeTraffic clears every per-node counter (monthly global reset).
func ClearAllNodeTraffic(db *gorm.DB) error {
	return db.Unscoped().Where("1 = 1").Delete(&models.UserNodeTraffic{}).Error
}

func IsExpired(u models.ProxyUser) bool {
	return !u.ExpireTime.IsZero() && u.ExpireTime.Before(time.Now())
}
