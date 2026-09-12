package subpage

import (
	"bytes"
	"encoding/base64"
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
// URIItem is one share link shown on the subscription HTML page.
type URIItem struct {
	Protocol string
	URI      string
}

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
	TrafficPct    float64 // 0–100 for progress bar; 0 when unlimited / unknown
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
	URIItems      []URIItem
	// SubQRDataURI is a data:image/png;base64,... QR of SubURL, generated locally (no external API).
	// template.URL prevents html/template from percent-encoding the base64 payload in src="...".
	SubQRDataURI template.URL
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
	pct := 0.0
	if pu.TrafficLimit > 0 {
		pct = float64(pu.TrafficUsed) * 100 / float64(pu.TrafficLimit)
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
		}
	}
	uriItems := make([]URIItem, 0, len(links))
	for _, link := range links {
		uriItems = append(uriItems, URIItem{Protocol: protocolLabel(link), URI: link})
	}
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
		TrafficPct:    pct,
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
		URIItems:      uriItems,
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
	return fmt.Sprintf("%.2f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func formatBytesLimit(n int64) string {
	if n <= 0 {
		return "∞"
	}
	return formatBytes(n)
}

// localSubQRDataURI encodes the subscription URL as a PNG data-URI using an
// in-process QR library — no third-party HTTP API.
func localSubQRDataURI(subURL string) template.URL {
	subURL = strings.TrimSpace(subURL)
	if subURL == "" {
		return ""
	}
	// Medium is denser; Long is safer for longer public_url paths.
	png, err := qrcode.Encode(subURL, qrcode.Medium, 256)
	if err != nil || len(png) == 0 {
		return ""
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))
}

func protocolLabel(uri string) string {
	uri = strings.TrimSpace(uri)
	if i := strings.Index(uri, "://"); i > 0 {
		scheme := strings.ToLower(uri[:i])
		switch scheme {
		case "hysteria2", "hy2":
			return "HY2"
		case "hysteria":
			return "HY"
		case "anytls":
			return "AnyTLS"
		case "ss", "shadowsocks":
			return "SS"
		case "vmess":
			return "VMess"
		case "vless":
			return "VLESS"
		case "trojan":
			return "Trojan"
		case "tuic":
			return "TUIC"
		case "wireguard":
			return "WG"
		case "snell":
			return "Snell"
		case "mieru":
			return "Mieru"
		default:
			if len(scheme) > 10 {
				scheme = scheme[:10]
			}
			return strings.ToUpper(scheme)
		}
	}
	return "URI"
}

// DefaultTemplate returns the built-in HTML for documentation / preview.
func DefaultTemplate() string {
	return defaultHTML
}

const defaultHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <meta name="color-scheme" content="light dark">
  <title>{{if .SubTitle}}{{.SubTitle}}{{else}}3m-ui Subscription{{end}}</title>
  <style>
    :root {
      --bg: #0f172a;
      --card-bg: #1e293b;
      --card-border: #334155;
      --text-main: #f8fafc;
      --text-muted: #94a3b8;
      --primary: #3b82f6;
      --primary-hover: #2563eb;
      --primary-light: rgba(59, 130, 246, 0.1);
      --success: #10b981;
      --success-light: rgba(16, 185, 129, 0.12);
      --danger: #ef4444;
      --danger-light: rgba(239, 68, 68, 0.12);
      --radius: 12px;
      --code-bg: #0f172a;
    }
    @media (prefers-color-scheme: light) {
      :root {
        --bg: #f8fafc;
        --card-bg: #ffffff;
        --card-border: #e2e8f0;
        --text-main: #0f172a;
        --text-muted: #64748b;
        --primary: #2563eb;
        --primary-hover: #1d4ed8;
        --primary-light: #eff6ff;
        --success: #059669;
        --success-light: #ecfdf5;
        --danger: #dc2626;
        --danger-light: #fef2f2;
        --code-bg: #f1f5f9;
      }
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "PingFang SC", "Noto Sans SC", sans-serif;
      background-color: var(--bg);
      color: var(--text-main);
      min-height: 100vh;
      display: flex;
      justify-content: center;
      align-items: flex-start;
      padding: 32px 16px 64px;
      line-height: 1.5;
    }
    .wrap { width: 100%; max-width: 520px; display: flex; flex-direction: column; gap: 20px; }
    .header {
      display: flex; align-items: center; justify-content: space-between; gap: 12px;
      background: var(--card-bg); border: 1px solid var(--card-border);
      padding: 16px 20px; border-radius: var(--radius);
    }
    .brand { display: flex; align-items: center; gap: 12px; min-width: 0; }
    .logo {
      width: 42px; height: 42px; background: var(--primary); color: #fff;
      font-weight: 800; font-size: 1.15rem; border-radius: 10px;
      display: grid; place-items: center; letter-spacing: -0.5px; flex-shrink: 0;
    }
    .brand-text h1 { font-size: 1.05rem; font-weight: 700; line-height: 1.2; }
    .brand-text .user { font-size: 0.82rem; color: var(--text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .header-right { display: flex; flex-direction: column; align-items: flex-end; gap: 8px; flex-shrink: 0; }
    .lang-switch { display: flex; gap: 4px; }
    .lang-btn {
      font-size: 0.72rem; font-weight: 600; padding: 3px 8px; border-radius: 6px;
      border: 1px solid var(--card-border); background: transparent; color: var(--text-muted); cursor: pointer;
    }
    .lang-btn.active { background: var(--primary-light); border-color: var(--primary); color: var(--primary); }
    .status-group { display: flex; gap: 6px; flex-wrap: wrap; justify-content: flex-end; }
    .badge {
      font-size: 0.75rem; font-weight: 600; padding: 4px 10px; border-radius: 999px;
      display: inline-flex; align-items: center; gap: 5px;
    }
    .badge.ok { background: var(--success-light); color: var(--success); }
    .badge.off { background: var(--danger-light); color: var(--danger); }
    .badge-dot { width: 6px; height: 6px; border-radius: 50%; background-color: currentColor; }
    .card {
      background: var(--card-bg); border: 1px solid var(--card-border);
      border-radius: var(--radius); padding: 20px;
    }
    .card-title {
      font-size: 0.85rem; font-weight: 700; text-transform: uppercase;
      letter-spacing: 0.05em; color: var(--text-muted); margin-bottom: 16px;
    }
    .announce {
      margin-bottom: 14px; padding: 10px 12px; border-radius: 8px;
      background: var(--primary-light); color: var(--primary); font-size: 0.88rem;
    }
    .traffic-box { margin-bottom: 18px; }
    .traffic-info {
      display: flex; justify-content: space-between; font-size: 0.9rem; margin-bottom: 8px; gap: 8px;
    }
    .traffic-info .val { font-weight: 600; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
    .progress-bar { height: 8px; background: var(--code-bg); border-radius: 999px; overflow: hidden; }
    .progress-fill { height: 100%; background: var(--primary); border-radius: 999px; transition: width 0.3s ease; }
    .grid-info {
      display: grid; grid-template-columns: 1fr 1fr; gap: 12px;
      padding-top: 12px; border-top: 1px dashed var(--card-border);
    }
    .info-item { display: flex; flex-direction: column; gap: 2px; }
    .info-item .lbl { font-size: 0.78rem; color: var(--text-muted); }
    .info-item .val { font-size: 0.92rem; font-weight: 600; }
    .import-grid { display: flex; flex-direction: column; align-items: center; gap: 20px; }
    @media (min-width: 480px) {
      .import-grid { flex-direction: row; align-items: flex-start; }
    }
    .qr-box { text-align: center; flex-shrink: 0; }
    .qr-box img {
      width: 150px; height: 150px; border-radius: 8px; background: #fff;
      padding: 8px; border: 1px solid var(--card-border);
    }
    .qr-box .hint { font-size: 0.75rem; color: var(--text-muted); margin-top: 6px; max-width: 150px; margin-left: auto; margin-right: auto; }
    .actions { flex: 1; width: 100%; display: flex; flex-direction: column; gap: 10px; }
    .btn {
      display: inline-flex; align-items: center; justify-content: center;
      padding: 10px 16px; font-size: 0.88rem; font-weight: 600; border-radius: 8px;
      text-decoration: none; cursor: pointer; border: 1px solid transparent;
      transition: all 0.15s ease; gap: 6px; font-family: inherit; width: 100%;
    }
    .btn-primary { background: var(--primary); color: #ffffff; }
    .btn-primary:hover { background: var(--primary-hover); }
    .btn-outline {
      background: transparent; border-color: var(--card-border); color: var(--text-main);
    }
    .btn-outline:hover { background: var(--primary-light); border-color: var(--primary); color: var(--primary); }
    .links-grid { display: grid; grid-template-columns: 1fr; gap: 8px; }
    @media (min-width: 400px) {
      .links-grid { grid-template-columns: 1fr; }
    }
    .uri-list { display: flex; flex-direction: column; gap: 10px; }
    .uri-item {
      background: var(--code-bg); border: 1px solid var(--card-border); border-radius: 8px;
      padding: 10px 12px; display: flex; align-items: center; justify-content: space-between; gap: 12px;
    }
    .uri-info { display: flex; align-items: center; gap: 8px; overflow: hidden; flex: 1; min-width: 0; }
    .protocol-tag {
      font-size: 0.7rem; font-weight: 700; padding: 2px 6px; border-radius: 4px;
      background: var(--primary-light); color: var(--primary); text-transform: uppercase; flex-shrink: 0;
    }
    .uri-text {
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 0.78rem; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
    }
    .btn-sm { padding: 4px 10px; font-size: 0.75rem; border-radius: 6px; flex-shrink: 0; width: auto; }
    footer { text-align: center; color: var(--text-muted); font-size: 0.8rem; margin-top: 8px; }
    .toast {
      position: fixed; top: 20px; left: 50%; transform: translateX(-50%) translateY(-20px);
      background: var(--text-main); color: var(--bg); padding: 8px 18px; border-radius: 999px;
      font-size: 0.85rem; font-weight: 600; opacity: 0; pointer-events: none;
      transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
      box-shadow: 0 4px 12px rgba(0,0,0,0.15); z-index: 100;
    }
    .toast.show { opacity: 1; transform: translateX(-50%) translateY(0); }
  </style>
</head>
<body>
<div class="wrap">
  <header class="header">
    <div class="brand">
      <div class="logo">3m</div>
      <div class="brand-text">
        <h1 data-i18n="title">{{if .SubTitle}}{{.SubTitle}}{{else}}3m-ui Subscription{{end}}</h1>
        <div class="user"><span data-i18n="user">User</span>: {{.Username}}</div>
      </div>
    </div>
    <div class="header-right">
      <div class="lang-switch">
        <button type="button" class="lang-btn" data-lang="zh" id="lang-zh">中文</button>
        <button type="button" class="lang-btn" data-lang="en" id="lang-en">EN</button>
      </div>
      <div class="status-group">
        {{if .Enabled}}
        <span class="badge ok"><i class="badge-dot"></i><span data-i18n="enabled">Enabled</span></span>
        {{else}}
        <span class="badge off"><i class="badge-dot"></i><span data-i18n="disabled">Disabled</span></span>
        {{end}}
      </div>
    </div>
  </header>

  <section class="card">
    <div class="card-title" data-i18n="account">Account</div>
    {{if .Announce}}<div class="announce">{{.Announce}}</div>{{end}}
    <div class="traffic-box">
      <div class="traffic-info">
        <span style="color: var(--text-muted);" data-i18n="trafficUsed">Traffic used</span>
        <span class="val">{{.TrafficUsedH}} / {{.TrafficLimitH}}</span>
      </div>
      <div class="progress-bar">
        <div class="progress-fill" style="width: {{printf "%.2f" .TrafficPct}}%;"></div>
      </div>
    </div>
    <div class="grid-info">
      <div class="info-item">
        <span class="lbl" data-i18n="expire">Expire</span>
        <span class="val">{{if .ExpireTime}}{{.ExpireTime}}{{else}}<span data-i18n="never">Never</span>{{end}}</span>
      </div>
      <div class="info-item">
        <span class="lbl" data-i18n="ipLimit">IP limit</span>
        <span class="val">{{if gt .IPLimit 0}}{{.IPLimit}}{{else}}<span data-i18n="unlimited">Unlimited</span>{{end}}</span>
      </div>
    </div>
  </section>

  <section class="card">
    <div class="card-title" data-i18n="import">Quick import</div>
    <div class="import-grid">
      {{if .SubQRDataURI}}
      <div class="qr-box">
        <img src="{{.SubQRDataURI}}" width="150" height="150" alt="Subscription QR">
        <p class="hint" data-i18n="qrHint">Scan to import</p>
      </div>
      {{end}}
      <div class="actions">
        <div class="links-grid">
          <button type="button" class="btn btn-primary copy-link-btn" data-url="{{.SubURL}}" data-i18n="copyMihomo">Clash / Mihomo</button>
          <button type="button" class="btn btn-outline copy-link-btn" data-url="{{.SubJSONURL}}" data-i18n="copySingbox">Sing-box</button>
          <button type="button" class="btn btn-outline copy-link-btn" data-url="{{.SubV2RayURL}}" data-i18n="copyV2ray">V2Ray / Base64</button>
          {{if .SubSupportURL}}
          <a class="btn btn-outline" href="{{.SubSupportURL}}" rel="noopener" data-i18n="support">Support</a>
          {{end}}
        </div>
      </div>
    </div>
  </section>

  {{if .URIItems}}
  <section class="card">
    <div class="card-title" data-i18n="nodes">Node links (URIs)</div>
    <div class="uri-list">
      {{range .URIItems}}
      <div class="uri-item">
        <div class="uri-info">
          <span class="protocol-tag">{{.Protocol}}</span>
          <span class="uri-text">{{.URI}}</span>
        </div>
        <button type="button" class="btn btn-outline btn-sm copy-uri-btn" data-uri="{{.URI}}" data-i18n="copy">Copy</button>
      </div>
      {{end}}
    </div>
  </section>
  {{else if .Links}}
  <section class="card">
    <div class="card-title" data-i18n="nodes">Node links (URIs)</div>
    <div class="uri-list">
      {{range .Links}}
      <div class="uri-item">
        <div class="uri-info">
          <span class="uri-text">{{.}}</span>
        </div>
        <button type="button" class="btn btn-outline btn-sm copy-uri-btn" data-uri="{{.}}" data-i18n="copy">Copy</button>
      </div>
      {{end}}
    </div>
  </section>
  {{end}}

  <footer>Powered by 3m-ui</footer>
</div>
<div class="toast" id="toast"></div>
<script>
(function () {
  var I18N = {
    zh: {
      titleFallback: "3m-ui 订阅",
      user: "用户",
      enabled: "已启用",
      disabled: "已禁用",
      account: "账号资源",
      trafficUsed: "已用流量",
      expire: "到期时间",
      never: "永久有效",
      ipLimit: "同时在线 IP 限制",
      unlimited: "无限制",
      import: "快捷导入",
      qrHint: "扫码导入订阅（默认 Mihomo / Clash）",
      copyMihomo: "Clash / Mihomo",
      copySingbox: "Sing-box",
      copyV2ray: "V2Ray / Base64",
      support: "支持",
      nodes: "节点链接 (URIs)",
      copy: "复制",
      copied: "已复制到剪贴板",
      copyFail: "复制失败，请手动复制"
    },
    en: {
      titleFallback: "3m-ui Subscription",
      user: "User",
      enabled: "Enabled",
      disabled: "Disabled",
      account: "Account",
      trafficUsed: "Traffic used",
      expire: "Expire",
      never: "Never",
      ipLimit: "IP limit",
      unlimited: "Unlimited",
      import: "Quick import",
      qrHint: "Scan to import (Mihomo / Clash by default)",
      copyMihomo: "Clash / Mihomo",
      copySingbox: "Sing-box",
      copyV2ray: "V2Ray / Base64",
      support: "Support",
      nodes: "Node links (URIs)",
      copy: "Copy",
      copied: "Copied to clipboard",
      copyFail: "Copy failed — please copy manually"
    }
  };

  function detectLang() {
    try {
      var q = new URLSearchParams(location.search).get("lang");
      if (q === "zh" || q === "en") return q;
    } catch (e) {}
    try {
      var saved = localStorage.getItem("3m-ui-sub-lang");
      if (saved === "zh" || saved === "en") return saved;
    } catch (e) {}
    var nav = (navigator.language || navigator.userLanguage || "en").toLowerCase();
    return nav.indexOf("zh") === 0 ? "zh" : "en";
  }

  var lang = detectLang();

  function applyLang(l) {
    lang = l === "zh" ? "zh" : "en";
    try { localStorage.setItem("3m-ui-sub-lang", lang); } catch (e) {}
    var dict = I18N[lang];
    document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
    document.querySelectorAll("[data-i18n]").forEach(function (el) {
      var key = el.getAttribute("data-i18n");
      if (dict[key]) el.textContent = dict[key];
    });
    document.getElementById("lang-zh").classList.toggle("active", lang === "zh");
    document.getElementById("lang-en").classList.toggle("active", lang === "en");
  }

  function showToast(text) {
    var toast = document.getElementById("toast");
    toast.textContent = text;
    toast.classList.add("show");
    setTimeout(function () { toast.classList.remove("show"); }, 2000);
  }

  function copyText(text) {
    var ok = I18N[lang].copied;
    var fail = I18N[lang].copyFail;
    if (!text) { showToast(fail); return; }
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(function () { showToast(ok); }).catch(function () { fallback(text, ok, fail); });
    } else {
      fallback(text, ok, fail);
    }
  }

  function fallback(text, ok, fail) {
    var ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand("copy");
      showToast(ok);
    } catch (e) {
      showToast(fail);
    }
    document.body.removeChild(ta);
  }

  document.getElementById("lang-zh").addEventListener("click", function () { applyLang("zh"); });
  document.getElementById("lang-en").addEventListener("click", function () { applyLang("en"); });

  document.querySelectorAll(".copy-link-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      copyText(this.getAttribute("data-url") || "");
    });
  });
  document.querySelectorAll(".copy-uri-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      copyText(this.getAttribute("data-uri") || "");
    });
  });

  applyLang(lang);
})();
</script>
</body>
</html>
`
