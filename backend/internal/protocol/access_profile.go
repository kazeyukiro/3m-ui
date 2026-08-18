package protocol

import (
	"strings"

	"gorm.io/gorm"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// AccessProfile is the public-facing endpoint used when generating share links / client YAML.
// Values are stored as panel settings keys with prefix "access_profile.".
type AccessProfile struct {
	PublicHost        string   `json:"public_host"`
	PublicPort        string   `json:"public_port"`
	SNI               string   `json:"sni"`
	ClientFingerprint string   `json:"client_fingerprint"`
	ALPN              []string `json:"alpn"`
}

const (
	KeyPublicHost        = "access_profile.public_host"
	KeyPublicPort        = "access_profile.public_port"
	KeySNI               = "access_profile.sni"
	KeyClientFingerprint = "access_profile.client_fingerprint"
	KeyALPN              = "access_profile.alpn"
)

func LoadAccessProfile(db *gorm.DB) AccessProfile {
	ap := AccessProfile{ClientFingerprint: "chrome"}
	if db == nil {
		return ap
	}
	var rows []models.PanelSetting
	_ = db.Where("key LIKE ?", "access_profile.%").Find(&rows).Error
	m := map[string]string{}
	for _, r := range rows {
		m[r.Key] = r.Value
	}
	ap.PublicHost = strings.TrimSpace(m[KeyPublicHost])
	ap.PublicPort = strings.TrimSpace(m[KeyPublicPort])
	ap.SNI = strings.TrimSpace(m[KeySNI])
	if v := strings.TrimSpace(m[KeyClientFingerprint]); v != "" {
		ap.ClientFingerprint = v
	}
	if v := strings.TrimSpace(m[KeyALPN]); v != "" {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				ap.ALPN = append(ap.ALPN, p)
			}
		}
	}
	return ap
}
