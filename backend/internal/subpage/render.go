package subpage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const (
	settingKeyThemeDir = "sub_theme_dir"
	settingKeyTitle    = "sub_title"
	settingKeySupport  = "sub_support_url"
	settingKeyAnnounce = "sub_announce"
	settingKeyWebPage  = "sub_web_page_url"
	settingKeyUpdates  = "sub_updates"
	settingKeyEncrypt  = "sub_encrypt"
)

// ViewModel is the data passed to custom / built-in subscription HTML templates
// (custom subscription page theme).
type ViewModel struct {
	Username      string
	Remark        string
	Enabled       bool
	Online        bool
	TrafficUsed   int64
	TrafficLimit  int64
	UploadBytes   int64
	DownloadBytes int64
	ExpireTime    string
	IPLimit       int
	SubURL        string
	SubJSONURL    string
	SubClashURL   string
	SubV2RayURL   string
	SubTitle      string
	SubSupportURL string
	IsOnline      bool
	Links         []string
}

// Settings holds subscription page branding options stored in PanelSetting.
type Settings struct {
	ThemeDir    string `json:"theme_dir"`
	Title       string `json:"title"`
	SupportURL  string `json:"support_url"`
	Announce    string `json:"announce"`
	WebPageURL  string `json:"web_page_url"`
	UpdateHours int    `json:"update_hours"` // Profile-Update-Interval
	Encrypt     bool   `json:"encrypt"`      // base64-encode raw URI list
}

func LoadPageSettings(db *gorm.DB) Settings {
	s := Settings{Title: "3m-ui Subscription", UpdateHours: 12, Encrypt: true}
	if db == nil {
		return s
	}
	s.ThemeDir = getSetting(db, settingKeyThemeDir)
	if t := getSetting(db, settingKeyTitle); t != "" {
		s.Title = t
	}
	s.SupportURL = getSetting(db, settingKeySupport)
	s.Announce = getSetting(db, settingKeyAnnounce)
	s.WebPageURL = getSetting(db, settingKeyWebPage)
	if v := getSetting(db, settingKeyUpdates); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			s.UpdateHours = n
		}
	}
	if v := getSetting(db, settingKeyEncrypt); v != "" {
		s.Encrypt = v == "1" || strings.EqualFold(v, "true")
	}
	return s
}

func SavePageSettings(db *gorm.DB, s Settings) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	if s.UpdateHours <= 0 {
		s.UpdateHours = 12
	}
	enc := "true"
	if !s.Encrypt {
		enc = "false"
	}
	themeDir := sanitizeThemeDir(s.ThemeDir)
	if strings.TrimSpace(s.ThemeDir) != "" && themeDir == "" {
		return fmt.Errorf("theme_dir must be empty or under an allowed theme root")
	}
	for _, kv := range []struct{ k, v string }{
		{settingKeyThemeDir, themeDir},
		{settingKeyTitle, strings.TrimSpace(s.Title)},
		{settingKeySupport, strings.TrimSpace(s.SupportURL)},
		{settingKeyAnnounce, strings.TrimSpace(s.Announce)},
		{settingKeyWebPage, strings.TrimSpace(s.WebPageURL)},
		{settingKeyUpdates, fmt.Sprintf("%d", s.UpdateHours)},
		{settingKeyEncrypt, enc},
	} {
		if err := upsertSetting(db, kv.k, kv.v); err != nil {
			return err
		}
	}
	return nil
}

func getSetting(db *gorm.DB, key string) string {
	var row models.PanelSetting
	if err := db.Where("key = ?", key).First(&row).Error; err != nil {
		return ""
	}
	return row.Value
}

func upsertSetting(db *gorm.DB, key, value string) error {
	var row models.PanelSetting
	err := db.Where("key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&models.PanelSetting{Key: key, Value: value}).Error
	}
	if err != nil {
		return err
	}
	row.Value = value
	return db.Save(&row).Error
}

// RenderHTML renders the subscription information page for a proxy user.
func RenderHTML(db *gorm.DB, pu models.ProxyUser, subBase string, links []string) ([]byte, error) {
	page := LoadPageSettings(db)
	expire := ""
	if !pu.ExpireTime.IsZero() {
		expire = pu.ExpireTime.UTC().Format(time.RFC3339)
	}
	base := strings.TrimSuffix(subBase, "/")
	vm := ViewModel{
		Username:      pu.Username,
		Remark:        pu.Remark,
		Enabled:       pu.Enabled,
		Online:        pu.Online,
		TrafficUsed:   pu.TrafficUsed,
		TrafficLimit:  pu.TrafficLimit,
		UploadBytes:   pu.UploadBytes,
		DownloadBytes: pu.DownloadBytes,
		ExpireTime:    expire,
		IPLimit:       pu.IPLimit,
		SubURL:        base,
		SubJSONURL:    base + "?target=singbox",
		SubClashURL:   base + "?target=clash",
		SubV2RayURL:   base + "?target=v2ray",
		SubTitle:      page.Title,
		SubSupportURL: page.SupportURL,
		IsOnline:      pu.Online,
		Links:         links,
	}

	tpl, err := loadTemplate(page.ThemeDir)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, vm); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// allowedThemeRoots enumerates the directories under which a custom subscription
// page theme may live. Path-traversal outside these roots is rejected so an
// attacker (or hijacked admin session) cannot point the HTML template loader at
// arbitrary files on the host (e.g. /etc/passwd, database files).
var allowedThemeRoots = []string{
	"/var/lib/3m-ui/themes",
	"/etc/3m-ui/themes",
	"/usr/local/share/3m-ui/themes",
}

// sanitizeThemeDir returns the cleaned absolute theme directory if it lives
// under one of the allowed roots, otherwise the empty string. An empty input
// is allowed (falls back to the built-in default template).
func sanitizeThemeDir(themeDir string) string {
	themeDir = strings.TrimSpace(themeDir)
	if themeDir == "" {
		return ""
	}
	clean := filepath.Clean(themeDir)
	if !filepath.IsAbs(clean) {
		return ""
	}
	for _, root := range allowedThemeRoots {
		rel, err := filepath.Rel(root, clean)
		if err != nil {
			continue
		}
		if rel == "." {
			return clean
		}
		if !strings.HasPrefix(rel, "..") && !strings.Contains(rel, string(filepath.Separator)+"..") {
			return clean
		}
	}
	return ""
}

func loadTemplate(themeDir string) (*template.Template, error) {
	themeDir = sanitizeThemeDir(themeDir)
	if themeDir != "" {
		for _, name := range []string{"index.html", "sub.html"} {
			path := filepath.Join(themeDir, name)
			if st, err := os.Stat(path); err == nil && !st.IsDir() {
				return template.ParseFiles(path)
			}
		}
	}
	return template.New("sub").Parse(defaultHTML)
}

// DefaultTemplate returns the built-in HTML for documentation / preview.
func DefaultTemplate() string {
	return defaultHTML
}

// ExportSettingsJSON is a convenience for API responses.
func ExportSettingsJSON(db *gorm.DB) ([]byte, error) {
	s := LoadPageSettings(db)
	return json.Marshal(s)
}

const defaultHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>{{if .SubTitle}}{{.SubTitle}}{{else}}Subscription{{end}}</title>
<style>
  :root { --bg:#0f1419; --card:#1a2332; --text:#e7ecf3; --muted:#8b9bb4; --accent:#3b82f6; --ok:#22c55e; --warn:#f59e0b; }
  * { box-sizing: border-box; }
  body { margin:0; font-family: system-ui,-apple-system,Segoe UI,Roboto,sans-serif; background:var(--bg); color:var(--text); min-height:100vh; }
  .wrap { max-width:640px; margin:0 auto; padding:24px 16px 48px; }
  h1 { font-size:1.4rem; margin:0 0 4px; }
  .sub { color:var(--muted); font-size:.9rem; margin-bottom:20px; }
  .card { background:var(--card); border-radius:12px; padding:16px 18px; margin-bottom:14px; }
  .row { display:flex; justify-content:space-between; gap:12px; padding:6px 0; font-size:.92rem; }
  .row span:first-child { color:var(--muted); }
  .badge { display:inline-block; padding:2px 8px; border-radius:999px; font-size:.75rem; background:#243044; }
  .badge.ok { background:rgba(34,197,94,.15); color:var(--ok); }
  .badge.warn { background:rgba(245,158,11,.15); color:var(--warn); }
  .links a { display:block; color:var(--accent); text-decoration:none; word-break:break-all; margin:8px 0; font-size:.85rem; }
  .links a:hover { text-decoration:underline; }
  .qr { text-align:center; margin-top:12px; }
  .qr img { border-radius:8px; background:#fff; padding:8px; }
  footer { margin-top:24px; text-align:center; color:var(--muted); font-size:.8rem; }
</style>
</head>
<body>
<div class="wrap">
  <h1>{{if .SubTitle}}{{.SubTitle}}{{else}}Subscription{{end}}</h1>
  <p class="sub">{{.Username}}{{if .Remark}} · {{.Remark}}{{end}}</p>

  <div class="card">
    <div class="row"><span>Status</span>
      <span>
        {{if .Enabled}}<span class="badge ok">Enabled</span>{{else}}<span class="badge warn">Disabled</span>{{end}}
        {{if .IsOnline}}<span class="badge ok">Online</span>{{else}}<span class="badge">Offline</span>{{end}}
      </span>
    </div>
    <div class="row"><span>Traffic</span>
      <span>{{.TrafficUsed}} / {{if gt .TrafficLimit 0}}{{.TrafficLimit}}{{else}}∞{{end}} bytes</span>
    </div>
    <div class="row"><span>Expire</span><span>{{if .ExpireTime}}{{.ExpireTime}}{{else}}Never{{end}}</span></div>
    <div class="row"><span>IP limit</span><span>{{if gt .IPLimit 0}}{{.IPLimit}}{{else}}∞{{end}}</span></div>
  </div>

  <div class="card links">
    <div class="row"><span>Formats</span></div>
    <a href="{{.SubURL}}">Mihomo / Clash (YAML)</a>
    <a href="{{.SubV2RayURL}}">V2Ray / Base64</a>
    <a href="{{.SubClashURL}}">Clash target</a>
    {{if .SubSupportURL}}<a href="{{.SubSupportURL}}" rel="noopener">Support</a>{{end}}
  </div>

  <footer>Powered by 3m-ui · refresh client subscription to pick up changes</footer>
</div>
</body>
</html>
`
