package models

type User struct {
	BaseModel
	Username           string `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash       string `gorm:"not null" json:"-"`
	Role               string `gorm:"not null" json:"role"`
	MustChangePassword bool   `gorm:"not null;default:false" json:"must_change_password"`
	SessionVersion     uint   `gorm:"not null;default:1" json:"-"`
	// TOTPSecret is base32-encoded shared secret; empty means 2FA not configured.
	TOTPSecret string `gorm:"size:64;default:''" json:"-"`
	// TOTPEnabled requires a valid TOTP code after password on login.
	TOTPEnabled bool `gorm:"not null;default:false" json:"totp_enabled"`
	// GithubID is the numeric GitHub user id as string; empty = not linked.
	GithubID string `gorm:"size:32;index" json:"github_id,omitempty"`
	// GithubLogin is the GitHub username at last successful OAuth (display/audit).
	GithubLogin string `gorm:"size:64;default:''" json:"github_login,omitempty"`
}
