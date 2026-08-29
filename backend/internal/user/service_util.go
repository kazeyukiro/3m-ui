package user

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
)

func safeMask(s string) string {
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

type SafeUser struct {
	ID            uint       `json:"id"`
	Username      string     `json:"username"`
	UUIDMasked    string     `json:"uuid_masked"`
	TrafficLimit  int64      `json:"traffic_limit"`
	TrafficUsed   int64      `json:"traffic_used"`
	UploadBytes   int64      `json:"upload_bytes"`
	DownloadBytes int64      `json:"download_bytes"`
	LastSeen      *time.Time `json:"last_seen"`
	Online        bool       `json:"online"`
	ExpireTime    time.Time  `json:"expire_time"`
	Enabled       bool       `json:"enabled"`
	Blocked       bool       `json:"blocked"`
	IPLimit       int        `json:"ip_limit"`
	Remark        string     `json:"remark"`
	SubToken      string     `json:"sub_token"`
	TelegramID    int64      `json:"telegram_id"`
	TelegramName  string     `json:"telegram_name"`
}

func ToSafeUser(u *models.ProxyUser) SafeUser {
	return SafeUser{
		ID:            u.ID,
		Username:      u.Username,
		UUIDMasked:    safeMask(u.UUID),
		TrafficLimit:  u.TrafficLimit,
		TrafficUsed:   u.TrafficUsed,
		UploadBytes:   u.UploadBytes,
		DownloadBytes: u.DownloadBytes,
		LastSeen:      u.LastSeen,
		Online:        u.Online,
		ExpireTime:    u.ExpireTime,
		Enabled:       u.Enabled,
		Blocked:       !IsCredentialActive(*u),
		IPLimit:       u.IPLimit,
		Remark:        u.Remark,
		SubToken:      u.SubToken,
		TelegramID:    u.TelegramID,
		TelegramName:  u.TelegramName,
	}
}

func encryptPassword(plain string) (string, error)   { return security.Encrypt(plain) }
func decryptPassword(encoded string) (string, error) { return security.Decrypt(encoded) }

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
