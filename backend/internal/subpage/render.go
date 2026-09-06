package subpage

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/skip2/go-qrcode"
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
	TrafficUsedH  string
	TrafficLimitH string
	ExpireTime    string
	IPLimit       int
	SubURL        string
	SubJSONURL    string
	SubClashURL   string
	SubV2RayURL   string
	SubTitle      string
	SubSupportURL string
	Announce      string
	IsOnline      bool
	Links         []string
	// SubQRDataURI is a data:image/png;base64,... QR of SubURL, generated locally (no external API).
	SubQRDataURI string
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
		TrafficUsedH:  formatBytes(pu.TrafficUsed),
		TrafficLimitH: formatBytesLimit(pu.TrafficLimit),
		ExpireTime:    expire,
		IPLimit:       pu.IPLimit,
		SubURL:        base,
		SubJSONURL:    base + "?target=singbox",
		SubClashURL:   base + "?target=clash",
		SubV2RayURL:   base + "?target=v2ray",
		SubTitle:      page.Title,
		SubSupportURL: page.SupportURL,
		Announce:      page.Announce,
		IsOnline:      pu.Online,
		Links:         links,
		SubQRDataURI:  localSubQRDataURI(base),
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


func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(n)/float64(div), "KMGTPE"[exp])const defaultHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"/>
<meta name="color-scheme" content="dark light"/>
<title>{{if .SubTitle}}{{.SubTitle}}{{else}}3m-ui Subscription{{end}}</title>
<style>
  :root {
    --bg0: #0b1220;
    --bg1: #121a2b;
    --card: rgba(255,255,255,.06);
    --card-border: rgba(255,255,255,.10);
    --text: #e8eef9;
    --muted: #93a0b8;
    --accent: #3b82f6;
    --accent2: #60a5fa;
    --ok: #34d399;
    --warn: #fbbf24;
    --danger: #f87171;
    --radius: 16px;
    --shadow: 0 12px 40px rgba(0,0,0,.35);
    font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, "PingFang SC", "Noto Sans SC", "Microsoft YaHei", sans-serif;
  }
  @media (prefers-color-scheme: light) {
    :root {
      --bg0: #f4f7fc;
      --bg1: #ffffff;
      --card: #ffffff;
      --card-border: #e5eaf3;
      --text: #0f172a;
      --muted: #64748b;
      --shadow: 0 10px 30px rgba(15,23,42,.08);
    }
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; min-height: 100vh;
    color: var(--text);
    background:
      radial-gradient(1200px 600px at 10% -10%, rgba(59,130,246,.25), transparent 55%),
      radial-gradient(900px 500px at 100% 0%, rgba(96,165,250,.18), transparent 50%),
      linear-gradient(180deg, var(--bg0), var(--bg1));
  }
  .wrap { max-width: 560px; margin: 0 auto; padding: 28px 18px 48px; }
  header { text-align: center; margin-bottom: 20px; }
  .logo {
    width: 64px; height: 64px; margin: 0 auto 12px;
    border-radius: 16px;
    background: linear-gradient(135deg, var(--accent), #1d4ed8);
    display: grid; place-items: center;
    box-shadow: 0 8px 24px rgba(59,130,246,.35);
    font-weight: 800; font-size: 1.1rem; color: #fff; letter-spacing: .02em;
  }
  h1 { margin: 0 0 6px; font-size: 1.45rem; font-weight: 700; letter-spacing: -.02em; }
  .sub { margin: 0; color: var(--muted); font-size: .95rem; }
  .announce {
    margin: 16px 0 0;
    padding: 12px 14px;
    border-radius: 12px;
    background: rgba(59,130,246,.12);
    border: 1px solid rgba(59,130,246,.25);
    color: var(--text);
    font-size: .9rem;
    line-height: 1.5;
  }
  .card {
    background: var(--card);
    border: 1px solid var(--card-border);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
    padding: 16px 18px;
    margin-top: 16px;
    backdrop-filter: blur(8px);
  }
  .card h2 {
    margin: 0 0 12px;
    font-size: .78rem;
    text-transform: uppercase;
    letter-spacing: .08em;
    color: var(--muted);
    font-weight: 600;
  }
  .row {
    display: flex; justify-content: space-between; gap: 12px; align-items: center;
    padding: 10px 0;
    border-bottom: 1px solid var(--card-border);
    font-size: .95rem;
  }
  .row:last-child { border-bottom: 0; padding-bottom: 0; }
  .row > span:first-child { color: var(--muted); flex-shrink: 0; }
  .row > span:last-child { text-align: right; word-break: break-all; }
  .badge {
    display: inline-block; padding: 2px 10px; border-radius: 999px;
    font-size: .75rem; font-weight: 600;
    background: rgba(148,163,184,.2); color: var(--muted);
  }
  .badge.ok { background: rgba(52,211,153,.18); color: var(--ok); }
  .badge.warn { background: rgba(251,191,36,.18); color: var(--warn); }
  .badge.off { background: rgba(248,113,113,.15); color: var(--danger); }
  .links a, .btn {
    display: block; width: 100%; text-align: center; text-decoration: none;
    margin-top: 10px; padding: 12px 14px; border-radius: 12px;
    font-weight: 600; font-size: .92rem; cursor: pointer; border: 0;
    color: #fff; background: linear-gradient(135deg, var(--accent), #2563eb);
    box-shadow: 0 6px 16px rgba(37,99,235,.28);
  }
  .links a.secondary, .btn.secondary {
    background: transparent; color: var(--text);
    border: 1px solid var(--card-border);
    box-shadow: none;
  }
  .qr-box { text-align: center; padding: 8px 0 4px; }
  .qr-box img {
    width: 180px; height: 180px; border-radius: 12px;
    background: #fff; padding: 10px;
    box-shadow: 0 4px 16px rgba(0,0,0,.12);
  }
  .hint { margin-top: 10px; font-size: .8rem; color: var(--muted); text-align: center; line-height: 1.4; }
  .uri {
    margin-top: 8px; padding: 10px 12px; border-radius: 10px;
    background: rgba(0,0,0,.2); font-size: .72rem; word-break: break-all;
    color: var(--muted); max-height: 4.5em; overflow: auto;
  }
  footer { margin-top: 28px; text-align: center; color: var(--muted); font-size: .78rem; }
  .toast {
    position: fixed; left: 50%; bottom: 24px; transform: translateX(-50%) translateY(20px);
    opacity: 0; pointer-events: none; transition: .2s ease;
    background: #0f172a; color: #fff; padding: 10px 16px; border-radius: 999px;
    font-size: .85rem; z-index: 20;
  }
  .toast.show { opacity: 1; transform: translateX(-50%) translateY(0); }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <div class="logo">3m</div>
    <h1>{{if .SubTitle}}{{.SubTitle}}{{else}}Subscription{{end}}</h1>
    <p class="sub">{{.Username}}{{if .Remark}} · {{.Remark}}{{end}}</p>
    {{if .Announce}}<div class="announce">{{.Announce}}</div>{{end}}
  </header>

  <div class="card">
    <h2>Account</h2>
    <div class="row"><span>Status</span>
      <span>
        {{if .Enabled}}<span class="badge ok">Enabled</span>{{else}}<span class="badge off">Disabled</span>{{end}}
        {{if .IsOnline}}<span class="badge ok">Online</span>{{else}}<span class="badge">Offline</span>{{end}}
      </span>
    </div>
    <div class="row"><span>Traffic</span>
      <span>{{.TrafficUsedH}} / {{.TrafficLimitH}}</span>
    </div>
    <div class="row"><span>Expire</span><span>{{if .ExpireTime}}{{.ExpireTime}}{{else}}Never{{end}}</span></div>
    <div class="row"><span>IP limit</span><span>{{if gt .IPLimit 0}}{{.IPLimit}}{{else}}∞{{end}}</span></div>
  </div>

  <div class="card">
    <h2>Import</h2>
    {{if .SubQRDataURI}}
    <div class="qr-box">
      <img src="{{.SubQRDataURI}}" width="180" height="180" alt="Subscription QR"/>
      <p class="hint">Scan to import the subscription URL (Mihomo / Clash by default)</p>
    </div>
    {{end}}
    <div class="links">
      <a href="{{.SubURL}}">Mihomo / Clash (YAML)</a>
      <a class="secondary" href="{{.SubV2RayURL}}">V2Ray / Base64</a>
      <a class="secondary" href="{{.SubJSONURL}}">Sing-box (JSON)</a>
      <a class="secondary" href="{{.SubClashURL}}">Clash (?target=clash)</a>
      <button type="button" class="btn secondary" id="copy-sub" data-url="{{.SubURL}}">Copy subscription URL</button>
      {{if .SubSupportURL}}<a class="secondary" href="{{.SubSupportURL}}" rel="noopener">Support</a>{{end}}
    </div>
  </div>

  {{if .Links}}
  <div class="card">
    <h2>Node URIs</h2>
    {{range .Links}}
    <div class="uri">{{.}}</div>
    {{end}}
  </div>
  {{end}}

  <footer>Powered by 3m-ui</footer>
</div>
<div class="toast" id="toast">Copied</div>
<script>
function copyText(t) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(t).then(showToast).catch(function(){ fallback(t); });
  } else { fallback(t); }
}
function fallback(t) {
  var a = document.createElement('textarea');
  a.value = t; document.body.appendChild(a); a.select();
  try { document.execCommand('copy'); showToast(); } catch (e) {}
  document.body.removeChild(a);
}
function showToast() {
  var el = document.getElementById('toast');
  el.classList.add('show');
  setTimeout(function(){ el.classList.remove('show'); }, 1600);
}
document.getElementById('copy-sub') && document.getElementById('copy-sub').addEventListener('click', function() {
  copyText(this.getAttribute('data-url') || '');
});
</script>
</body>
</html>

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
