package hwid

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"

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

const maxHWIDLen = 128
const minHWIDLen = 4

var (
	ErrMaxDevices = errors.New("hwid device limit reached")
	ErrRequired   = errors.New("hwid required when device limit is enabled")
)

type DeviceInfo struct {
	HWID        string
	DeviceOS    string
	VerOS       string
	DeviceModel string
	UserAgent   string
	Present     bool
}

func ParseRequest(r *http.Request) DeviceInfo {
	if r == nil {
		return DeviceInfo{}
	}
	raw := strings.TrimSpace(r.Header.Get(HeaderHWID))
	if raw == "" && r.URL != nil {
		raw = strings.TrimSpace(r.URL.Query().Get("hwid"))
	}
	info := DeviceInfo{
		DeviceOS:    strings.TrimSpace(r.Header.Get(HeaderDeviceOS)),
		VerOS:       strings.TrimSpace(r.Header.Get(HeaderVerOS)),
		DeviceModel: strings.TrimSpace(r.Header.Get(HeaderDeviceModel)),
		UserAgent:   strings.TrimSpace(r.UserAgent()),
	}
	if id, ok := normalizeHWID(raw); ok {
		info.HWID = id
		info.Present = true
	}
	return info
}

func normalizeHWID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if r == 0x7f || (r < 0x20 && r != '\t') {
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	s := strings.TrimSpace(b.String())
	if len(s) < minHWIDLen {
		return "", false
	}
	if len(s) > maxHWIDLen {
		s = s[:maxHWIDLen]
	}
	return s, true
}

// Enforce applies HWID policy for a subscription fetch.
//
// limit <= 0: never deny. Best-effort device tracking only; DB errors ignored.
// limit > 0: require x-hwid; deny new devices when at capacity; known devices always OK.
func Enforce(db *gorm.DB, userID uint, limit int, info DeviceInfo, hdr http.Header) error {
	if db == nil || userID == 0 {
		return nil
	}
	active := limit > 0
	if active && hdr != nil {
		hdr.Set(HeaderActive, "true")
	}

	if !info.Present {
		if active {
			if hdr != nil {
				hdr.Set(HeaderNotSupported, "true")
			}
			return ErrRequired
		}
		return nil
	}

	now := time.Now().UTC()

	// Existing device (including soft-deleted) → refresh and allow.
	var existing models.HWIDDevice
	qerr := db.Unscoped().Where("proxy_user_id = ? AND hwid = ?", userID, info.HWID).First(&existing).Error
	if qerr == nil {
		if err := touchDevice(db, &existing, info, now); err != nil {
			log.Printf("hwid: touch user=%d: %v", userID, err)
			// Still allow — device is known.
		}
		return nil
	}
	if !errors.Is(qerr, gorm.ErrRecordNotFound) {
		log.Printf("hwid: lookup user=%d: %v", userID, qerr)
		if !active {
			return nil
		}
		// Active limit but cannot read DB → fail open to avoid mass 500.
		return nil
	}

	// New device
	if active {
		var count int64
		if err := db.Model(&models.HWIDDevice{}).Where("proxy_user_id = ?", userID).Count(&count).Error; err != nil {
			log.Printf("hwid: count user=%d: %v", userID, err)
			return nil // fail open
		}
		if int(count) >= limit {
			if hdr != nil {
				hdr.Set(HeaderMaxDevicesReached, "true")
			}
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
	if err := db.Create(&row).Error; err != nil {
		if isUniqueConflict(err) {
			// Concurrent insert — treat as known.
			var raced models.HWIDDevice
			if e2 := db.Unscoped().Where("proxy_user_id = ? AND hwid = ?", userID, info.HWID).First(&raced).Error; e2 == nil {
				_ = touchDevice(db, &raced, info, now)
				return nil
			}
		}
		log.Printf("hwid: create user=%d hwid=%q: %v", userID, info.HWID, err)
		// Unlimited: never block. Limited + under count: fail open (already passed count).
		return nil
	}
	return nil
}

func touchDevice(db *gorm.DB, existing *models.HWIDDevice, info DeviceInfo, now time.Time) error {
	existing.DeviceOS = firstNonEmpty(truncate(info.DeviceOS, 64), existing.DeviceOS)
	existing.VerOS = firstNonEmpty(truncate(info.VerOS, 64), existing.VerOS)
	existing.DeviceModel = firstNonEmpty(truncate(info.DeviceModel, 128), existing.DeviceModel)
	existing.UserAgent = firstNonEmpty(truncate(info.UserAgent, 512), existing.UserAgent)
	existing.LastSeenAt = now
	existing.DeletedAt = gorm.DeletedAt{}
	return db.Unscoped().Save(existing).Error
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
