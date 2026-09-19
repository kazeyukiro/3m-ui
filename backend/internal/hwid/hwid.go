package hwid

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

// Happ / Remnawave-compatible request headers for subscription import.
const (
	HeaderHWID        = "x-hwid"
	HeaderDeviceOS    = "x-device-os"
	HeaderVerOS       = "x-ver-os"
	HeaderDeviceModel = "x-device-model"
)

// Response headers when device limit is active.
const (
	HeaderActive            = "x-hwid-active"
	HeaderNotSupported      = "x-hwid-not-supported"
	HeaderMaxDevicesReached = "x-hwid-max-devices-reached"
)

// Accept common client device ids: UUID, hex, base64url, and short opaque tokens.
// Remnawave v3 baseline was [a-zA-Z0-9=-]{10,64}; real clients also send _, +, /, :
// and lengths beyond 64 (capped at 128 to match the DB column).
var hwidPattern = regexp.MustCompile(`^[a-zA-Z0-9=_+\-./:]{8,128}$`)

var (
	ErrMaxDevices = errors.New("hwid device limit reached")
	// ErrRequired is returned when HWIDLimit > 0 but the client did not send a
	// valid x-hwid (limit cannot be enforced without a device id).
	ErrRequired = errors.New("hwid required when device limit is enabled")
)

type DeviceInfo struct {
	HWID        string
	DeviceOS    string
	VerOS       string
	DeviceModel string
	UserAgent   string
	Present     bool // client sent a valid x-hwid
}

func ParseRequest(r *http.Request) DeviceInfo {
	if r == nil {
		return DeviceInfo{}
	}
	raw := strings.TrimSpace(r.Header.Get(HeaderHWID))
	// Some clients put the id in a query parameter as a fallback.
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("hwid"))
	}
	info := DeviceInfo{
		DeviceOS:    strings.TrimSpace(r.Header.Get(HeaderDeviceOS)),
		VerOS:       strings.TrimSpace(r.Header.Get(HeaderVerOS)),
		DeviceModel: strings.TrimSpace(r.Header.Get(HeaderDeviceModel)),
		UserAgent:   strings.TrimSpace(r.UserAgent()),
	}
	if raw == "" || !hwidPattern.MatchString(raw) {
		return info
	}
	info.HWID = raw
	info.Present = true
	return info
}

// Enforce registers or refreshes the device. limit<=0 means unlimited (still
// records known devices when x-hwid is present). When limit>0, a valid x-hwid
// is required and the active device count must stay within limit.
func Enforce(db *gorm.DB, userID uint, limit int, info DeviceInfo, hdr http.Header) error {
	if db == nil || userID == 0 {
		return nil
	}
	active := limit > 0
	if active {
		hdr.Set(HeaderActive, "true")
	}
	if !info.Present {
		if active {
			hdr.Set(HeaderNotSupported, "true")
			return ErrRequired
		}
		return nil
	}

	now := time.Now().UTC()
	return db.Transaction(func(tx *gorm.DB) error {
		var existing models.HWIDDevice
		// UNIQUE(proxy_user_id, hwid) includes soft-deleted rows.
		err := tx.Unscoped().
			Where("proxy_user_id = ? AND hwid = ?", userID, info.HWID).
			First(&existing).Error
		if err == nil {
			existing.DeviceOS = firstNonEmpty(info.DeviceOS, existing.DeviceOS)
			existing.VerOS = firstNonEmpty(info.VerOS, existing.VerOS)
			existing.DeviceModel = firstNonEmpty(info.DeviceModel, existing.DeviceModel)
			existing.UserAgent = firstNonEmpty(truncate(info.UserAgent, 512), existing.UserAgent)
			existing.LastSeenAt = now
			existing.DeletedAt = gorm.DeletedAt{}
			return tx.Unscoped().Save(&existing).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if limit > 0 {
			var count int64
			if err := tx.Model(&models.HWIDDevice{}).Where("proxy_user_id = ?", userID).Count(&count).Error; err != nil {
				return err
			}
			if int(count) >= limit {
				hdr.Set(HeaderMaxDevicesReached, "true")
				return ErrMaxDevices
			}
		}
		row := models.HWIDDevice{
			ProxyUserID: userID,
			HWID:        info.HWID,
			DeviceOS:    truncate(info.DeviceOS, 64),
			VerOS:       truncate(info.VerOS, 64),
			DeviceModel: truncate(info.DeviceModel, 128),
			UserAgent:   truncate(info.UserAgent, 512),
			LastSeenAt:  now,
		}
		if err := tx.Create(&row).Error; err != nil {
			// Concurrent first-seen: unique race — refresh as existing.
			if isUniqueConflict(err) {
				var raced models.HWIDDevice
				if e2 := tx.Unscoped().Where("proxy_user_id = ? AND hwid = ?", userID, info.HWID).First(&raced).Error; e2 == nil {
					raced.LastSeenAt = now
					raced.DeletedAt = gorm.DeletedAt{}
					raced.DeviceOS = firstNonEmpty(info.DeviceOS, raced.DeviceOS)
					raced.VerOS = firstNonEmpty(info.VerOS, raced.VerOS)
					raced.DeviceModel = firstNonEmpty(info.DeviceModel, raced.DeviceModel)
					raced.UserAgent = firstNonEmpty(truncate(info.UserAgent, 512), raced.UserAgent)
					return tx.Unscoped().Save(&raced).Error
				}
			}
			return err
		}
		return nil
	})
}

func isUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate") || strings.Contains(s, "constraint")
}

func List(db *gorm.DB, userID uint) ([]models.HWIDDevice, error) {
	var rows []models.HWIDDevice
	err := db.Where("proxy_user_id = ?", userID).Order("last_seen_at desc, id desc").Find(&rows).Error
	return rows, err
}

func Delete(db *gorm.DB, userID, deviceID uint) error {
	return db.Unscoped().Where("proxy_user_id = ? AND id = ?", userID, deviceID).Delete(&models.HWIDDevice{}).Error
}

func DeleteAll(db *gorm.DB, userID uint) error {
	return db.Unscoped().Where("proxy_user_id = ?", userID).Delete(&models.HWIDDevice{}).Error
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
