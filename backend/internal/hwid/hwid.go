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
	"gorm.io/gorm/clause"
)

// Happ / Remnawave-compatible request headers for subscription import.
const (
	HeaderHWID        = "x-hwid"
	HeaderDeviceOS    = "x-device-os"
	HeaderVerOS       = "x-ver-os"
	HeaderDeviceModel = "x-device-model"
)

// Response headers when device limit is active (Remnawave / v2rayTUN compatible).
const (
	HeaderActive            = "x-hwid-active"
	HeaderNotSupported      = "x-hwid-not-supported"
	HeaderMaxDevicesReached = "x-hwid-max-devices-reached"
	HeaderLimitFlag         = "x-hwid-limit" // v2rayTUN expects this when limit is on
)

const maxHWIDLen = 128
const minHWIDLen = 4

var (
	ErrMaxDevices = errors.New("hwid device limit reached")
	ErrRequired   = errors.New("hwid required when device limit is enabled")
)

// Alternate request header names some clients use.
var hwidHeaderAliases = []string{
	HeaderHWID,
	"x-device-id",
	"x-hardware-id",
	"hwid",
}

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
	raw := ""
	for _, h := range hwidHeaderAliases {
		if v := strings.TrimSpace(r.Header.Get(h)); v != "" {
			raw = v
			break
		}
	}
	if raw == "" && r.URL != nil {
		for _, q := range []string{"hwid", "x-hwid", "device_id"} {
			if v := strings.TrimSpace(r.URL.Query().Get(q)); v != "" {
				raw = v
				break
			}
		}
	}
	info := DeviceInfo{
		DeviceOS:    firstHeader(r, HeaderDeviceOS, "x-os", "device-os"),
		VerOS:       firstHeader(r, HeaderVerOS, "x-os-version", "x-ver-os"),
		DeviceModel: firstHeader(r, HeaderDeviceModel, "x-model", "device-model"),
		UserAgent:   strings.TrimSpace(r.UserAgent()),
	}
	if id, ok := normalizeHWID(raw); ok {
		info.HWID = id
		info.Present = true
	}
	return info
}

func firstHeader(r *http.Request, names ...string) string {
	for _, n := range names {
		if v := strings.TrimSpace(r.Header.Get(n)); v != "" {
			return v
		}
	}
	return ""
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
// Devices are recorded only when the client actually sends a usable device id
// (Happ / v2rayTun / Remnawave-compatible). Clash Meta / v2rayNG without HWID
// support will never appear in the device list — that is expected.
//
// limit <= 0: never deny; best-effort track.
// limit > 0: require x-hwid; deny new devices when at capacity.
func Enforce(db *gorm.DB, userID uint, limit int, info DeviceInfo, hdr http.Header) error {
	if db == nil || userID == 0 {
		return nil
	}
	active := limit > 0
	if active && hdr != nil {
		hdr.Set(HeaderActive, "true")
		hdr.Set(HeaderLimitFlag, "true")
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

	var existing models.HWIDDevice
	qerr := db.Unscoped().Where("proxy_user_id = ? AND hwid = ?", userID, info.HWID).First(&existing).Error
	if qerr == nil {
		if err := touchDevice(db, &existing, info, now); err != nil {
			log.Printf("hwid: touch user=%d hwid=%q: %v", userID, info.HWID, err)
		} else {
			log.Printf("hwid: seen user=%d hwid=%q os=%q model=%q", userID, info.HWID, info.DeviceOS, info.DeviceModel)
		}
		return nil
	}
	if !errors.Is(qerr, gorm.ErrRecordNotFound) {
		log.Printf("hwid: lookup user=%d: %v", userID, qerr)
		if !active {
			return nil
		}
		return nil
	}

	if active {
		var count int64
		if err := db.Model(&models.HWIDDevice{}).Where("proxy_user_id = ?", userID).Count(&count).Error; err != nil {
			log.Printf("hwid: count user=%d: %v", userID, err)
			return nil
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
	// Upsert-friendly create: on unique race, treat as known.
	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "proxy_user_id"}, {Name: "hwid"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"device_os", "ver_os", "device_model", "user_agent", "last_seen_at", "deleted_at", "updated_at",
		}),
	}).Create(&row).Error
	if err != nil {
		// Fallback without OnConflict (older SQLite / driver quirks).
		if err2 := db.Create(&row).Error; err2 != nil {
			if isUniqueConflict(err2) {
				var raced models.HWIDDevice
				if e3 := db.Unscoped().Where("proxy_user_id = ? AND hwid = ?", userID, info.HWID).First(&raced).Error; e3 == nil {
					_ = touchDevice(db, &raced, info, now)
					log.Printf("hwid: race-resolved user=%d hwid=%q", userID, info.HWID)
					return nil
				}
			}
			log.Printf("hwid: create failed user=%d hwid=%q: %v (onconflict: %v)", userID, info.HWID, err2, err)
			return nil
		}
	}
	log.Printf("hwid: registered user=%d hwid=%q os=%q model=%q", userID, info.HWID, info.DeviceOS, info.DeviceModel)
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
