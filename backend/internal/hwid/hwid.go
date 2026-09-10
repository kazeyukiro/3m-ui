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

// Remnawave v3+: /^[a-zA-Z0-9=-]{10,64}$/
var hwidPattern = regexp.MustCompile(`^[a-zA-Z0-9=\-]{10,64}$`)

var (
	ErrMaxDevices = errors.New("hwid device limit reached")
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

// Enforce registers or refreshes the device and returns whether the subscription
// body should be denied. limit<=0 means unlimited (still records known devices).
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
		}
		return nil
	}

	var existing models.HWIDDevice
	// UNIQUE(proxy_user_id, hwid) includes soft-deleted rows.
	err := db.Unscoped().Where("proxy_user_id = ? AND hwid = ?", userID, info.HWID).First(&existing).Error
	now := time.Now().UTC()
	if err == nil {
		existing.DeviceOS = firstNonEmpty(info.DeviceOS, existing.DeviceOS)
		existing.VerOS = firstNonEmpty(info.VerOS, existing.VerOS)
		existing.DeviceModel = firstNonEmpty(info.DeviceModel, existing.DeviceModel)
		existing.UserAgent = firstNonEmpty(info.UserAgent, existing.UserAgent)
		existing.LastSeenAt = now
		existing.DeletedAt = gorm.DeletedAt{}
		_ = db.Unscoped().Save(&existing).Error
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// New device
	if limit > 0 {
		var count int64
		if err := db.Model(&models.HWIDDevice{}).Where("proxy_user_id = ?", userID).Count(&count).Error; err != nil {
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
		DeviceOS:    info.DeviceOS,
		VerOS:       info.VerOS,
		DeviceModel: info.DeviceModel,
		UserAgent:   truncate(info.UserAgent, 512),
		LastSeenAt:  now,
	}
	return db.Create(&row).Error
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
