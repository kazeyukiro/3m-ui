package subpage

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"
)

const (
	settingKeyThemeDir = "sub_theme_dir"
	settingKeyTitle    = "sub_title"
	settingKeySupport  = "sub_support_url"
	settingKeyAnnounce = "sub_announce"
	settingKeyNotices  = "sub_notices"
	settingKeyWebPage  = "sub_web_page_url"
	settingKeyUpdates  = "sub_updates"
	settingKeyEncrypt  = "sub_encrypt"
)

// Notice size caps. The notice list is operator-authored panel content, but a
// textarea paste should never be able to bloat every subscription response.
const (
	maxNotices        = 20
	maxNoticeDateLen  = 24
	maxNoticeTitleLen = 120
	maxNoticeDescLen  = 500
)

// ViewModel is the data passed to custom / built-in subscription HTML templates
// (custom subscription page theme).
// URIItem is one share link shown on the subscription HTML page.
type URIItem struct {
	Protocol string
	URI      string
	// Name is the human label carried in the URI fragment (e.g. "#HK-01"),
	// URL-decoded. Empty when the link has no fragment.
	Name string
	// Meta is a short transport/security label derived from the URI query
	// (e.g. "gRPC · TLS"). Empty when nothing can be derived.
	Meta string
	// ProtocolKey is a stable lowercase protocol id for CSS styling
	// (vless / vmess / trojan / hy2 / ss / tuic / wg / …).
	ProtocolKey string
	// Index is the 1-based position of the node in the list.
	Index int
}

// NoticeItem is one announcement rendered in the dedicated notice block of the
// subscription page. Date is free-form (e.g. "09-15"); pinned items render
// expanded by default and carry a "置顶" tag.
type NoticeItem struct {
	Date   string `json:"date"`
	Title  string `json:"title"`
	Desc   string `json:"desc"`
	Pinned bool   `json:"pinned"`
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
	// PageTitle is the hero/brand title: the operator's sub_title when
	// customized, otherwise the built-in "3m-ui 订阅".
	PageTitle string
	IsOnline  bool
	Links     []string
	URIItems  []URIItem
	// SubQRDataURI is a data:image/png;base64,... QR of SubURL, generated locally (no external API).
	// template.URL prevents html/template from percent-encoding the base64 payload in src="...".
	SubQRDataURI template.URL
	// ---- Fields below were added for the mobile-first page redesign.
	// They are additive only: custom themes built on older ViewModels keep working.
	RemainBytesH   string // remaining traffic; "" when unlimited
	IsUnlimited    bool   // traffic limit not set
	UploadBytesH   string
	DownloadBytesH string
	HasExpiry      bool // ExpireTime is set
	IsExpired      bool // ExpireTime is in the past
	DaysLeft       int  // whole days until expiry; <= 0 when expired
	NodeCount      int  // number of node links
	Notices        []NoticeItem
	// ClashDeepLink / SingboxDeepLink are app deep links with the subscription
	// URL pre-encoded (clash://install-config?url=…, sing-box://import-remote?url=…).
	ClashDeepLink   template.URL
	SingboxDeepLink template.URL
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
	// Notices is the raw editor text for the dedicated notice block: one
	// notice per line, "[*]date|title|desc" ("*" prefix = pinned, "#" = comment).
	Notices string `json:"notices"`
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
	s.Notices = getSetting(db, settingKeyNotices)
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
		{settingKeyNotices, formatNotices(parseNotices(s.Notices))},
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
	for i, link := range links {
		uriItems = append(uriItems, URIItem{
			Protocol:    protocolLabel(link),
			URI:         link,
			Name:        uriFragmentName(link),
			Meta:        uriMeta(link),
			ProtocolKey: protocolKey(link),
			Index:       i + 1,
		})
	}
	unlimited := pu.TrafficLimit <= 0
	remain := ""
	if !unlimited {
		r := pu.TrafficLimit - pu.TrafficUsed
		if r < 0 {
			r = 0
		}
		remain = formatBytes(r)
	}
	hasExpiry := !pu.ExpireTime.IsZero()
	daysLeft, isExpired := 0, false
	if hasExpiry {
		daysLeft = int(time.Until(pu.ExpireTime).Hours() / 24)
		isExpired = !pu.ExpireTime.After(time.Now())
	}
	notices := parseNotices(page.Notices)
	if len(notices) == 0 {
		// Legacy fallback: a bare announce string renders as one pinned notice.
		if an := strings.TrimSpace(page.Announce); an != "" {
			notices = []NoticeItem{{Title: clipField(an, maxNoticeTitleLen), Pinned: true}}
		}
	}
	pageTitle := strings.TrimSpace(page.Title)
	if pageTitle == "" || pageTitle == "3m-ui Subscription" {
		pageTitle = "3m-ui 订阅"
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
		PageTitle:     pageTitle,
		SubSupportURL: page.SupportURL,
		Announce:      page.Announce,
		IsOnline:      pu.Online,
		Links:         links,
		URIItems:      uriItems,
		SubQRDataURI:  localSubQRDataURI(base),
		// Mobile-first redesign additions.
		RemainBytesH:    remain,
		IsUnlimited:     unlimited,
		UploadBytesH:    formatBytes(pu.UploadBytes),
		DownloadBytesH:  formatBytes(pu.DownloadBytes),
		HasExpiry:       hasExpiry,
		IsExpired:       isExpired,
		DaysLeft:        daysLeft,
		NodeCount:       len(links),
		Notices:         notices,
		ClashDeepLink:   template.URL("clash://install-config?url=" + url.QueryEscape(base+"?target=clash")),
		SingboxDeepLink: template.URL("sing-box://import-remote?url=" + url.QueryEscape(base+"?target=singbox")),
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

// parseNotices decodes the operator-authored notice editor text into items.
// Line format: "[*]date|title|desc" — "*" prefix marks a pinned notice, "#"
// starts a comment line, blank lines are ignored. Values are length-capped so
// a stray paste cannot bloat every subscription response.
func parseNotices(raw string) []NoticeItem {
	var out []NoticeItem
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pinned := false
		if strings.HasPrefix(line, "*") {
			pinned = true
			line = strings.TrimSpace(line[1:])
		}
		parts := strings.SplitN(line, "|", 3)
		it := NoticeItem{Pinned: pinned}
		switch len(parts) {
		case 3:
			it.Date, it.Title, it.Desc = parts[0], parts[1], parts[2]
		case 2:
			it.Date, it.Title = parts[0], parts[1]
		default:
			it.Title = parts[0]
		}
		it.Date = clipField(it.Date, maxNoticeDateLen)
		it.Title = clipField(it.Title, maxNoticeTitleLen)
		it.Desc = clipField(it.Desc, maxNoticeDescLen)
		if it.Title == "" && it.Desc == "" {
			continue
		}
		if it.Title == "" {
			it.Title, it.Desc = it.Desc, ""
		}
		out = append(out, it)
		if len(out) >= maxNotices {
			break
		}
	}
	return out
}

// formatNotices is the inverse of parseNotices (canonical editor text).
func formatNotices(items []NoticeItem) string {
	var lines []string
	for _, it := range items {
		it.Date = clipField(it.Date, maxNoticeDateLen)
		it.Title = clipField(it.Title, maxNoticeTitleLen)
		it.Desc = clipField(it.Desc, maxNoticeDescLen)
		if it.Title == "" && it.Desc == "" {
			continue
		}
		if it.Title == "" {
			it.Title, it.Desc = it.Desc, ""
		}
		var b strings.Builder
		if it.Pinned {
			b.WriteString("*")
		}
		b.WriteString(it.Date)
		b.WriteString("|")
		b.WriteString(it.Title)
		if it.Desc != "" {
			b.WriteString("|")
			b.WriteString(it.Desc)
		}
		lines = append(lines, b.String())
		if len(lines) >= maxNotices {
			break
		}
	}
	return strings.Join(lines, "\n")
}

// clipField trims, collapses newlines (they would break the line format) and
// caps the length of one notice field on rune boundaries.
func clipField(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

// uriFragmentName extracts the human label from a share link fragment
// (e.g. "vless://…#HK-01" → "HK-01").
func uriFragmentName(uri string) string {
	if i := strings.Index(uri, "#"); i >= 0 && i+1 < len(uri) {
		frag := uri[i+1:]
		if decoded, err := url.QueryUnescape(frag); err == nil {
			return strings.TrimSpace(decoded)
		}
		return strings.TrimSpace(frag)
	}
	return ""
}

// uriMeta derives a short transport · security label from a share link query
// (e.g. "gRPC · TLS"). Returns "" when nothing can be derived.
func uriMeta(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "hysteria2", "hy2", "hysteria", "tuic":
		return "QUIC"
	case "anytls":
		return "TLS"
	case "wireguard":
		return "WireGuard"
	}
	q := u.Query()
	transport := strings.ToLower(q.Get("type"))
	security := strings.ToLower(q.Get("security"))
	label := ""
	switch transport {
	case "grpc":
		label = "gRPC"
	case "ws", "websocket":
		label = "WebSocket"
	case "http", "h2":
		label = "HTTP/2"
	case "httpupgrade":
		label = "HTTPUpgrade"
	case "quic":
		label = "QUIC"
	case "xhttp", "splithttp":
		label = "XHTTP"
	case "tcp":
		label = "TCP"
	case "kcp":
		label = "mKCP"
	}
	if security != "" && security != "none" && security != "allowInsecure" {
		sec := strings.ToUpper(security)
		if sec == "REALITY" {
			sec = "REALITY"
		}
		if label != "" {
			label += " · " + sec
		} else {
			label = sec
		}
	}
	return label
}

// protocolKey maps a share-link scheme to a stable lowercase id for CSS.
func protocolKey(uri string) string {
	uri = strings.TrimSpace(uri)
	if i := strings.Index(uri, "://"); i > 0 {
		switch strings.ToLower(uri[:i]) {
		case "hysteria2", "hy2":
			return "hy2"
		case "hysteria":
			return "hysteria"
		case "vmess":
			return "vmess"
		case "vless":
			return "vless"
		case "trojan":
			return "trojan"
		case "ss", "shadowsocks":
			return "ss"
		case "tuic":
			return "tuic"
		case "wireguard":
			return "wg"
		case "anytls":
			return "anytls"
		case "snell":
			return "snell"
		case "mieru":
			return "mieru"
		}
	}
	return "other"
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
  <meta name="format-detection" content="telephone=no">
  <meta name="referrer" content="no-referrer">
  <meta name="color-scheme" content="dark">
  <title>{{if .SubTitle}}{{.SubTitle}}{{else}}3m-ui Subscription{{end}}</title>
  <style>
    :root {
      --bg: #0e1420;
      --card: #161f30;
      --card-2: #1b263b;
      --stroke: #26324a;
      --text: #eef2f9;
      --muted: #8b9bb4;
      --faint: #5c6c86;
      --accent: #f59e0b;
      --accent-soft: rgba(245, 158, 11, .12);
      --green: #22c55e;
      --red: #ef4444;
      --radius: 20px;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; -webkit-tap-highlight-color: transparent; }
    html { -webkit-text-size-adjust: 100%; text-size-adjust: 100%; width: 100%; overflow-x: hidden; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "PingFang SC", "Noto Sans SC", sans-serif;
      /* Solid fallbacks first — some SFA/SFM WebViews resolve CSS variables poorly. */
      background: #0e1420;
      background: var(--bg, #0e1420);
      color: #eef2f9;
      color: var(--text, #eef2f9);
      min-height: 100vh;
      line-height: 1.5;
      width: 100%;
      max-width: 100vw;
      overflow-x: hidden;
      display: -webkit-box;
      display: -webkit-flex;
      display: flex;
      -webkit-box-pack: center;
      justify-content: center;
      padding: max(18px, env(safe-area-inset-top, 0px)) max(14px, env(safe-area-inset-right, 0px)) max(30px, env(safe-area-inset-bottom, 0px)) max(14px, env(safe-area-inset-left, 0px));
      word-wrap: break-word;
      overflow-wrap: anywhere;
    }
    .wrap {
      width: 100%;
      max-width: 430px;
      min-width: 0;
      display: -webkit-box;
      display: -webkit-flex;
      display: flex;
      -webkit-box-orient: vertical;
      -webkit-flex-direction: column;
      flex-direction: column;
      gap: 13px;
    }

    /* ===== hero ===== */
    .hero { text-align: center; padding: 10px 0 4px; }
    .hero .mark {
      width: 54px; height: 54px; margin: 0 auto 12px; border-radius: 17px;
      background: #f59e0b;
      background: -webkit-linear-gradient(160deg, #f59e0b, #ea580c);
      background: linear-gradient(160deg, #f59e0b, #ea580c);
      display: -webkit-box;
      display: -webkit-flex;
      display: flex;
      -webkit-box-align: center;
      -webkit-flex-align: center;
      align-items: center;
      -webkit-box-pack: center;
      -webkit-justify-content: center;
      justify-content: center;
      -webkit-box-shadow: 0 10px 30px rgba(245, 158, 11, .35);
      box-shadow: 0 10px 30px rgba(245, 158, 11, .35);
    }
    .hero .mark svg { width: 28px; height: 28px; }
    .hero h1 { font-size: 1.3rem; font-weight: 800; letter-spacing: .3px; }
    .hero .user { font-size: .82rem; color: #8b9bb4; color: var(--muted, #8b9bb4); margin-top: 3px; }
    .lang-switch { display: -webkit-box; display: -webkit-flex; display: flex; -webkit-box-pack: center; -webkit-justify-content: center; justify-content: center; gap: 6px; margin-top: 10px; }
    .lang-btn {
      font-family: inherit; font-size: .68rem; font-weight: 700; padding: 3px 12px; border-radius: 999px;
      border: 1px solid #26324a; border: 1px solid var(--stroke, #26324a);
      background: transparent; color: #8b9bb4; color: var(--muted, #8b9bb4); cursor: pointer;
      -webkit-appearance: none; appearance: none;
    }
    .lang-btn.active { border-color: #f59e0b; border-color: var(--accent, #f59e0b); color: #f59e0b; color: var(--accent, #f59e0b); }

    /* ===== 公告（独立区块） ===== */
    .notice {
      background: #161f30;
      background: var(--card, #161f30);
      border: 1px solid rgba(245, 158, 11, .30);
      border-radius: 20px;
      border-radius: var(--radius, 20px);
      padding: 13px 16px 11px;
      text-align: left;
    }
    .notice-hd { display: -webkit-box; display: -webkit-flex; display: flex; -webkit-box-align: center; -webkit-flex-align: center; align-items: center; gap: 9px; }
    .notice-hd .ic {
      width: 26px; height: 26px; border-radius: 8px;
      background: rgba(245, 158, 11, .12);
      background: var(--accent-soft, rgba(245, 158, 11, .12));
      display: -webkit-box; display: -webkit-flex; display: flex;
      -webkit-box-align: center; -webkit-flex-align: center; align-items: center;
      -webkit-box-pack: center; -webkit-justify-content: center; justify-content: center;
      color: #f59e0b; color: var(--accent, #f59e0b); flex-shrink: 0;
    }
    .notice-hd .ic svg { width: 14px; height: 14px; }
    .notice-hd .t { -webkit-box-flex: 1; -webkit-flex: 1; flex: 1; font-size: .88rem; font-weight: 800; }
    .notice-hd .n {
      font-size: .68rem; font-weight: 700; color: #f59e0b; color: var(--accent, #f59e0b);
      background: rgba(245, 158, 11, .12); background: var(--accent-soft, rgba(245, 158, 11, .12));
      padding: 2px 9px; border-radius: 999px;
    }
    .notice-item { display: -webkit-box; display: -webkit-flex; display: flex; gap: 10px; padding: 10px 0 3px; margin-top: 8px; border-top: 1px dashed rgba(245, 158, 11, .22); cursor: pointer; }
    .notice-item .date { font-size: .68rem; font-weight: 700; color: #f59e0b; color: var(--accent, #f59e0b); font-variant-numeric: tabular-nums; padding-top: 3px; flex-shrink: 0; width: 46px; overflow-wrap: anywhere; }
    .notice-item .body { -webkit-box-flex: 1; -webkit-flex: 1; flex: 1; min-width: 0; }
    .notice-item .ttl { font-size: .82rem; font-weight: 700; display: -webkit-box; display: -webkit-flex; display: flex; -webkit-flex-wrap: wrap; flex-wrap: wrap; -webkit-box-align: center; -webkit-flex-align: center; align-items: center; gap: 7px; }
    .notice-item .ttl .pin { font-size: .58rem; color: #1a1205; background: #f59e0b; background: var(--accent, #f59e0b); padding: 2px 6px; border-radius: 4px; font-weight: 800; }
    .notice-item .desc { font-size: .74rem; color: #8b9bb4; color: var(--muted, #8b9bb4); margin-top: 4px; display: none; }
    .notice-item.open .desc { display: block; }
    .notice-item .chev { align-self: center; color: #5c6c86; color: var(--faint, #5c6c86); font-size: .68rem; -webkit-transition: -webkit-transform .25s; transition: transform .25s; flex-shrink: 0; }
    .notice-item.open .chev { -webkit-transform: rotate(180deg); transform: rotate(180deg); }

    /* ===== 二维码 ===== */
    .qr-card {
      background: #161f30;
      background: var(--card, #161f30);
      border: 1px solid #26324a;
      border: 1px solid var(--stroke, #26324a);
      border-radius: 20px;
      border-radius: var(--radius, 20px);
      padding: 22px 18px 18px;
      text-align: center;
    }
    .qr-card .frame {
      width: 208px; height: 208px; margin: 0 auto; background: #fff;
      border-radius: 18px; padding: 12px;
      -webkit-box-shadow: 0 12px 40px rgba(0, 0, 0, .4);
      box-shadow: 0 12px 40px rgba(0, 0, 0, .4);
    }
    .qr-card .frame img { width: 100%; height: 100%; display: block; border-radius: 8px; }
    .qr-card .cap { margin-top: 14px; font-size: .92rem; font-weight: 700; }
    .qr-card .cap small { display: block; font-size: .72rem; color: #5c6c86; color: var(--faint, #5c6c86); font-weight: 500; margin-top: 3px; }

    /* ===== 导入按钮 ===== */
    .import-btn {
      display: -webkit-box;
      display: -webkit-flex;
      display: flex;
      -webkit-box-align: center;
      -webkit-flex-align: center;
      align-items: center;
      gap: 14px;
      width: 100%;
      background: #f59e0b;
      background: -webkit-linear-gradient(135deg, #f59e0b, #ea580c);
      background: linear-gradient(135deg, #f59e0b, #ea580c);
      color: #fff;
      border: none;
      border-radius: 18px;
      padding: 17px 18px;
      cursor: pointer;
      font-family: inherit;
      font-size: 1.02rem;
      font-weight: 800;
      text-align: left;
      text-decoration: none;
      -webkit-box-shadow: 0 10px 30px rgba(234, 88, 12, .35);
      box-shadow: 0 10px 30px rgba(234, 88, 12, .35);
      -webkit-transition: -webkit-transform .12s ease;
      transition: transform .12s ease;
      -webkit-appearance: none;
      appearance: none;
    }
    .import-btn:active { -webkit-transform: scale(.97); transform: scale(.97); }
    .import-btn .ic {
      width: 44px; height: 44px; border-radius: 13px; background: rgba(255, 255, 255, .18);
      display: -webkit-box; display: -webkit-flex; display: flex;
      -webkit-box-align: center; -webkit-flex-align: center; align-items: center;
      -webkit-box-pack: center; -webkit-justify-content: center; justify-content: center;
      flex-shrink: 0;
    }
    .import-btn .ic svg { width: 22px; height: 22px; }
    .import-btn .tx { -webkit-box-flex: 1; -webkit-flex: 1; flex: 1; min-width: 0; }
    .import-btn .tx small { display: block; font-size: .7rem; font-weight: 500; opacity: .85; margin-top: 1px; }
    .import-btn .go { font-size: 1.3rem; opacity: .8; }

    .mini-row { display: -webkit-box; display: -webkit-flex; display: flex; gap: 10px; }
    .mini-btn {
      -webkit-box-flex: 1;
      -webkit-flex: 1;
      flex: 1;
      display: -webkit-box;
      display: -webkit-flex;
      display: flex;
      -webkit-box-align: center;
      -webkit-flex-align: center;
      align-items: center;
      -webkit-box-pack: center;
      -webkit-justify-content: center;
      justify-content: center;
      gap: 8px;
      background: #161f30;
      background: var(--card, #161f30);
      border: 1px solid #26324a;
      border: 1px solid var(--stroke, #26324a);
      color: #eef2f9;
      color: var(--text, #eef2f9);
      border-radius: 14px;
      padding: 14px 10px;
      font-family: inherit;
      font-size: .86rem;
      font-weight: 700;
      cursor: pointer;
      text-decoration: none;
      -webkit-transition: background .15s;
      transition: background .15s;
      -webkit-appearance: none;
      appearance: none;
    }
    .mini-btn:active { -webkit-transform: scale(.97); transform: scale(.97); background: #1b263b; background: var(--card-2, #1b263b); }
    .mini-btn svg { width: 17px; height: 17px; color: #f59e0b; color: var(--accent, #f59e0b); flex-shrink: 0; }

    /* ===== 账号摘要 ===== */
    details summary { list-style: none; cursor: pointer; }
    details summary::-webkit-details-marker { display: none; }
    .acct {
      background: #161f30;
      background: var(--card, #161f30);
      border: 1px solid #26324a;
      border: 1px solid var(--stroke, #26324a);
      border-radius: 20px;
      border-radius: var(--radius, 20px);
      overflow: hidden;
      text-align: left;
    }
    .acct > summary { padding: 15px 17px; display: -webkit-box; display: -webkit-flex; display: flex; -webkit-box-align: center; -webkit-flex-align: center; align-items: center; gap: 11px; }
    .acct > summary .ttl { -webkit-box-flex: 1; -webkit-flex: 1; flex: 1; font-size: .9rem; font-weight: 800; }
    .acct > summary .chev { color: #5c6c86; color: var(--faint, #5c6c86); -webkit-transition: -webkit-transform .25s; transition: transform .25s; font-size: .8rem; }
    .acct[open] > summary .chev { -webkit-transform: rotate(180deg); transform: rotate(180deg); }
    .acct .body { padding: 0 17px 16px; }
    .bar-lab { display: -webkit-box; display: -webkit-flex; display: flex; -webkit-box-pack: justify; -webkit-justify-content: space-between; justify-content: space-between; font-size: .72rem; color: #5c6c86; color: var(--faint, #5c6c86); margin-bottom: 6px; }
    .bar { height: 9px; background: #0e1420; background: var(--bg, #0e1420); border-radius: 999px; overflow: hidden; margin-bottom: 12px; }
    .bar-fill {
      height: 100%;
      background: #f59e0b;
      background: -webkit-linear-gradient(90deg, #f59e0b, #ea580c);
      background: linear-gradient(90deg, #f59e0b, #ea580c);
      border-radius: 999px;
      -webkit-transition: width .9s cubic-bezier(.22, 1, .36, 1);
      transition: width .9s cubic-bezier(.22, 1, .36, 1);
    }
    .kv { display: -webkit-box; display: -webkit-flex; display: flex; -webkit-box-pack: justify; -webkit-justify-content: space-between; justify-content: space-between; -webkit-box-align: center; -webkit-flex-align: center; align-items: center; gap: 10px; padding: 10px 0; border-bottom: 1px solid #26324a; border-bottom: 1px solid var(--stroke, #26324a); font-size: .84rem; }
    .kv:last-child { border-bottom: none; }
    .kv .k { color: #8b9bb4; color: var(--muted, #8b9bb4); flex-shrink: 0; }
    .kv .v { font-weight: 700; font-variant-numeric: tabular-nums; text-align: right; min-width: 0; overflow-wrap: anywhere; }
    .chip-row { display: -webkit-box; display: -webkit-flex; display: flex; -webkit-flex-wrap: wrap; flex-wrap: wrap; gap: 7px; margin-top: 10px; }
    .chip { font-size: .7rem; font-weight: 700; padding: 4px 10px; border-radius: 999px; background: rgba(245, 158, 11, .12); background: var(--accent-soft, rgba(245, 158, 11, .12)); color: #f59e0b; color: var(--accent, #f59e0b); }
    .chip.g { background: rgba(34, 197, 94, .1); color: #22c55e; color: var(--green, #22c55e); }
    .chip.warn { background: rgba(245, 158, 11, .16); }
    .chip.bad { background: rgba(239, 68, 68, .12); color: #ef4444; color: var(--red, #ef4444); }
    .acct-actions { display: -webkit-box; display: -webkit-flex; display: flex; -webkit-flex-wrap: wrap; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
    .acct-actions button, .acct-actions a {
      -webkit-box-flex: 1;
      -webkit-flex: 1 1 auto;
      flex: 1 1 auto;
      font-family: inherit;
      font-size: .76rem;
      font-weight: 700;
      padding: 9px 10px;
      border-radius: 10px;
      border: 1px solid #26324a;
      border: 1px solid var(--stroke, #26324a);
      background: #0e1420;
      background: var(--bg, #0e1420);
      color: #8b9bb4;
      color: var(--muted, #8b9bb4);
      cursor: pointer;
      text-decoration: none;
      text-align: center;
      -webkit-appearance: none;
      appearance: none;
    }
    .acct-actions button:hover, .acct-actions a:hover { color: #f59e0b; color: var(--accent, #f59e0b); border-color: #f59e0b; border-color: var(--accent, #f59e0b); }

    /* ===== 节点 ===== */
    .nodes {
      background: #161f30;
      background: var(--card, #161f30);
      border: 1px solid #26324a;
      border: 1px solid var(--stroke, #26324a);
      border-radius: 20px;
      border-radius: var(--radius, 20px);
      padding: 15px 17px;
      text-align: left;
    }
    .nodes .hd { display: -webkit-box; display: -webkit-flex; display: flex; -webkit-box-pack: justify; -webkit-justify-content: space-between; justify-content: space-between; -webkit-box-align: center; -webkit-flex-align: center; align-items: center; gap: 8px; margin-bottom: 11px; }
    .nodes .hd .t { font-size: .9rem; font-weight: 800; }
    .nodes .hd .n { font-size: .72rem; color: #5c6c86; color: var(--faint, #5c6c86); font-weight: 600; }
    .nodes .hd button { font-family: inherit; font-size: .72rem; font-weight: 700; color: #f59e0b; color: var(--accent, #f59e0b); background: none; border: none; cursor: pointer; padding: 4px 8px; border-radius: 7px; }
    .nodes .hd button:hover { background: rgba(245, 158, 11, .12); background: var(--accent-soft, rgba(245, 158, 11, .12)); }
    .n-row { display: -webkit-box; display: -webkit-flex; display: flex; -webkit-flex-wrap: wrap; flex-wrap: wrap; -webkit-box-align: center; -webkit-flex-align: center; align-items: center; gap: 10px; padding: 11px 2px; border-bottom: 1px solid #26324a; border-bottom: 1px solid var(--stroke, #26324a); cursor: pointer; }
    .n-row:last-child { border-bottom: none; }
    .n-row .nm { -webkit-box-flex: 1; -webkit-flex: 1; flex: 1; min-width: 0; font-size: .85rem; font-weight: 700; }
    .n-row .nm small { display: block; font-size: .68rem; color: #5c6c86; color: var(--faint, #5c6c86); font-weight: 500; }
    .n-row .cp {
      width: 34px; height: 34px; border-radius: 10px;
      border: 1px solid #26324a; border: 1px solid var(--stroke, #26324a);
      background: #0e1420; background: var(--bg, #0e1420);
      color: #8b9bb4; color: var(--muted, #8b9bb4);
      display: -webkit-box; display: -webkit-flex; display: flex;
      -webkit-box-align: center; -webkit-flex-align: center; align-items: center;
      -webkit-box-pack: center; -webkit-justify-content: center; justify-content: center;
      cursor: pointer; flex-shrink: 0;
      -webkit-appearance: none; appearance: none;
    }
    .n-row .cp:active { -webkit-transform: scale(.92); transform: scale(.92); color: #f59e0b; color: var(--accent, #f59e0b); border-color: #f59e0b; border-color: var(--accent, #f59e0b); }
    .n-row .cp svg { width: 14px; height: 14px; }
    .n-row .raw { display: none; width: 100%; margin-top: 8px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: .66rem; color: #5c6c86; color: var(--faint, #5c6c86); word-break: break-all; -webkit-user-select: text; user-select: text; }
    .n-row.open .raw { display: block; }
    .proto {
      font-size: .62rem; font-weight: 800; padding: 3px 7px; border-radius: 5px; flex-shrink: 0;
      background: rgba(245, 158, 11, .12); background: var(--accent-soft, rgba(245, 158, 11, .12));
      color: #f59e0b; color: var(--accent, #f59e0b);
    }
    .proto.vless { background: rgba(56, 189, 248, .14); color: #38bdf8; }
    .proto.vmess { background: rgba(167, 139, 250, .14); color: #a78bfa; }
    .proto.trojan { background: rgba(248, 113, 113, .14); color: #f87171; }
    .proto.hy2 { background: rgba(251, 146, 60, .14); color: #fb923c; }
    .proto.ss { background: rgba(45, 212, 191, .14); color: #2dd4bf; }
    .proto.tuic { background: rgba(232, 121, 249, .14); color: #e879f9; }
    .proto.wg { background: rgba(148, 163, 184, .14); color: #94a3b8; }
    .proto.anytls { background: rgba(52, 211, 153, .14); color: #34d399; }
    .proto.hysteria, .proto.snell, .proto.mieru, .proto.other { background: rgba(148, 163, 184, .12); color: #94a3b8; }

    footer { text-align: center; color: #5c6c86; color: var(--faint, #5c6c86); font-size: .7rem; padding: 2px 0; }

    .toast {
      position: fixed; left: 50%; bottom: 40px;
      -webkit-transform: translateX(-50%) translateY(12px);
      transform: translateX(-50%) translateY(12px);
      background: #fff; color: #0e1420; padding: 10px 20px; border-radius: 999px;
      font-size: .84rem; font-weight: 800; opacity: 0; pointer-events: none;
      -webkit-transition: all .25s cubic-bezier(.16, 1, .3, 1);
      transition: all .25s cubic-bezier(.16, 1, .3, 1);
      z-index: 99;
      -webkit-box-shadow: 0 10px 30px rgba(0, 0, 0, .4);
      box-shadow: 0 10px 30px rgba(0, 0, 0, .4);
      max-width: 86vw;
      text-align: center;
    }
    .toast.show { opacity: 1; -webkit-transform: translateX(-50%) translateY(0); transform: translateX(-50%) translateY(0); }
  </style>
</head>
<body>
<div class="wrap">

  <header class="hero">
    <div class="mark">
      <svg viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2 4.5 13H11l-1 9 8.5-11H12l1-9z"/></svg>
    </div>
    <h1>{{.PageTitle}}</h1>
    <div class="user">{{.Username}}</div>
    <div class="lang-switch">
      <button type="button" class="lang-btn" data-lang="zh" id="lang-zh">中文</button>
      <button type="button" class="lang-btn" data-lang="en" id="lang-en">EN</button>
    </div>
  </header>

  {{if .Notices}}
  <section class="notice">
    <div class="notice-hd">
      <span class="ic"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 11v2a1 1 0 0 0 1 1h3l4 4V6L7 10H4a1 1 0 0 0-1 1z"/><path d="M15 8.5a4 4 0 0 1 0 7"/><path d="M18 5.5a8 8 0 0 1 0 13"/></svg></span>
      <span class="t" data-i18n="notice">公告</span>
      <span class="n" data-i18n="noticeCount" data-n="{{len .Notices}}">{{len .Notices}} 条</span>
    </div>
    {{range .Notices}}
    <div class="notice-item{{if .Pinned}} open{{end}}">
      {{if .Date}}<span class="date">{{.Date}}</span>{{end}}
      <div class="body">
        <div class="ttl">{{.Title}}{{if .Pinned}}<span class="pin" data-i18n="pinned">置顶</span>{{end}}</div>
        {{if .Desc}}<div class="desc">{{.Desc}}</div>{{end}}
      </div>
      <span class="chev">&#9660;</span>
    </div>
    {{end}}
  </section>
  {{end}}

  {{if .SubQRDataURI}}
  <section class="qr-card">
    <div class="frame"><img src="{{.SubQRDataURI}}" width="208" height="208" alt="Subscription QR"></div>
    <div class="cap">
      <span data-i18n="scanImport">扫码导入订阅</span>
      <small><span data-i18n="scanSupport">支持 Mihomo / Clash / sing-box / Shadowrocket</span></small>
    </div>
  </section>
  {{end}}

  <a class="import-btn" href="{{.ClashDeepLink}}" data-deeplink="Clash / Mihomo">
    <span class="ic"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M13 2 4.5 13H11l-1 9 8.5-11H12l1-9z"/></svg></span>
    <span class="tx">
      <span data-i18n="importClash">一键导入 Clash / Mihomo</span>
      <small><span data-i18n="importClashSub">点击自动唤起客户端</span></small>
    </span>
    <span class="go">&#8250;</span>
  </a>

  <div class="mini-row">
    <a class="mini-btn" href="{{.SingboxDeepLink}}" data-deeplink="Sing-box">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><rect x="4" y="4" width="16" height="16" rx="3"/><path d="M9 12h6M12 9v6"/></svg>
      <span data-i18n="singbox">Sing-box</span>
    </a>
    <button type="button" class="mini-btn" id="copySub" data-link="{{.SubURL}}">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/></svg>
      <span data-i18n="copySub">复制链接</span>
    </button>
  </div>

  <details class="acct" open>
    <summary>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#f59e0b" stroke-width="2.2" stroke-linecap="round"><path d="M3 3v18h18"/><path d="m7 14 4-4 4 3 5-6"/></svg>
      <span class="ttl" data-i18n="account">账号资源</span>
      <span class="chev">&#9660;</span>
    </summary>
    <div class="body">
      <div class="bar-lab">
        <span><span data-i18n="trafficUsed">已用流量</span> {{.TrafficUsedH}}</span>
        <span>{{printf "%.1f" .TrafficPct}}%</span>
      </div>
      <div class="bar"><div class="bar-fill" id="barFill" data-pct="{{printf "%.2f" .TrafficPct}}" style="width: {{printf "%.2f" .TrafficPct}}%;"></div></div>
      <div class="kv"><span class="k" data-i18n="remain">剩余流量</span><span class="v">{{if .IsUnlimited}}<span data-i18n="unlimited">不限量</span>{{else}}{{.RemainBytesH}}{{end}}</span></div>
      <div class="kv"><span class="k" data-i18n="expire">到期时间</span><span class="v">{{if .ExpireTime}}{{.ExpireTime}}{{else}}<span data-i18n="never">永久有效</span>{{end}}</span></div>
      <div class="kv"><span class="k" data-i18n="updown">上传 / 下载</span><span class="v">{{.UploadBytesH}} / {{.DownloadBytesH}}</span></div>
      <div class="kv"><span class="k" data-i18n="ipLimit">同时在线 IP 限制</span><span class="v">{{if gt .IPLimit 0}}{{.IPLimit}}{{else}}<span data-i18n="unlimited">不限量</span>{{end}}</span></div>
      {{if .HasExpiry}}
      <div class="chip-row">
        <span class="chip{{if .IsExpired}} bad{{else if le .DaysLeft 30}} warn{{else}} g{{end}}" id="daysChip" data-days="{{.DaysLeft}}" data-expired="{{if .IsExpired}}1{{else}}0{{end}}"></span>
      </div>
      {{end}}
      <div class="acct-actions">
        <button type="button" id="copyV2ray" data-link="{{.SubV2RayURL}}"><span data-i18n="copyV2ray">复制 V2Ray / Base64 链接</span></button>
        {{if .SubSupportURL}}<a href="{{.SubSupportURL}}" rel="noopener"><span data-i18n="support">技术支持</span></a>{{end}}
      </div>
    </div>
  </details>

  {{if .URIItems}}
  <section class="nodes">
    <div class="hd">
      <span class="t" data-i18n="nodes">节点</span>
      <span class="n" data-i18n="nodeHint" data-n="{{.NodeCount}}">{{.NodeCount}} 个 · 点击展开</span>
      <button type="button" id="copyAll" data-i18n="copyAll">全部复制</button>
    </div>
    {{range .URIItems}}
    <div class="n-row">
      <span class="proto {{.ProtocolKey}}">{{.Protocol}}</span>
      <span class="nm">{{if .Name}}{{.Name}}{{else}}节点 {{.Index}}{{end}}{{if .Meta}}<small>{{.Meta}}</small>{{end}}</span>
      <button type="button" class="cp" data-link="{{.URI}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/></svg></button>
      <div class="raw">{{.URI}}</div>
    </div>
    {{end}}
  </section>
  {{end}}

  <footer>Powered by 3m-ui</footer>
</div>
<div class="toast" id="toast"></div>
<script>
(function () {
  var I18N = {
    zh: {
      account: "账号资源",
      trafficUsed: "已用流量",
      remain: "剩余流量",
      unlimited: "不限量",
      expire: "到期时间",
      never: "永久有效",
      updown: "上传 / 下载",
      ipLimit: "同时在线 IP 限制",
      daysLeft: "剩余 {n} 天",
      expired: "已到期",
      notice: "公告",
      noticeCount: "{n} 条",
      pinned: "置顶",
      scanImport: "扫码导入订阅",
      scanSupport: "支持 Mihomo / Clash / sing-box / Shadowrocket",
      importClash: "一键导入 Clash / Mihomo",
      importClashSub: "点击自动唤起客户端",
      singbox: "Sing-box",
      copySub: "复制链接",
      copyV2ray: "复制 V2Ray / Base64 链接",
      support: "技术支持",
      nodes: "节点",
      nodeHint: "{n} 个 · 点击展开",
      copyAll: "全部复制",
      copied: "已复制到剪贴板",
      copyFail: "复制失败，请手动复制",
      subCopied: "订阅链接已复制",
      v2rayCopied: "V2Ray / Base64 链接已复制",
      allCopied: "已复制全部 {n} 个节点",
      nodeCopied: "节点链接已复制",
      importing: "正在唤起 {app}；未响应请点「复制链接」手动导入"
    },
    en: {
      account: "Account",
      trafficUsed: "Traffic used",
      remain: "Remaining",
      unlimited: "Unlimited",
      expire: "Expire",
      never: "Never",
      updown: "Upload / Download",
      ipLimit: "IP limit",
      daysLeft: "{n} days left",
      expired: "Expired",
      notice: "Announcements",
      noticeCount: "{n}",
      pinned: "Pinned",
      scanImport: "Scan to import",
      scanSupport: "Mihomo / Clash / sing-box / Shadowrocket",
      importClash: "One-tap import Clash / Mihomo",
      importClashSub: "Opens your client automatically",
      singbox: "Sing-box",
      copySub: "Copy link",
      copyV2ray: "Copy V2Ray / Base64 link",
      support: "Support",
      nodes: "Nodes",
      nodeHint: "{n} nodes · tap to expand",
      copyAll: "Copy all",
      copied: "Copied to clipboard",
      copyFail: "Copy failed — please copy manually",
      subCopied: "Subscription link copied",
      v2rayCopied: "V2Ray / Base64 link copied",
      allCopied: "Copied all {n} nodes",
      nodeCopied: "Node link copied",
      importing: "Launching {app} — if nothing happens, copy the link manually"
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
  var toast = document.getElementById("toast"), toastTimer;

  function showToast(text) {
    toast.textContent = text;
    toast.classList.add("show");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { toast.classList.remove("show"); }, 2400);
  }

  function fillPlaceholders(el, text) {
    return text
      .split("{n}").join(el.getAttribute("data-n") || "")
      .split("{app}").join(el.getAttribute("data-app") || "");
  }

  function renderDaysChip() {
    var chip = document.getElementById("daysChip");
    if (!chip) return;
    var dict = I18N[lang];
    var expired = chip.getAttribute("data-expired") === "1";
    var days = parseInt(chip.getAttribute("data-days") || "0", 10) || 0;
    chip.textContent = expired ? dict.expired : dict.daysLeft.split("{n}").join(String(days));
  }

  function applyLang(l) {
    lang = l === "zh" ? "zh" : "en";
    try { localStorage.setItem("3m-ui-sub-lang", lang); } catch (e) {}
    var dict = I18N[lang];
    document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
    document.querySelectorAll("[data-i18n]").forEach(function (el) {
      var key = el.getAttribute("data-i18n");
      if (dict[key]) el.textContent = fillPlaceholders(el, dict[key]);
    });
    document.getElementById("lang-zh").classList.toggle("active", lang === "zh");
    document.getElementById("lang-en").classList.toggle("active", lang === "en");
    renderDaysChip();
  }

  function fallbackCopy(text, ok, fail) {
    var ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.setAttribute("aria-hidden", "true");
    // iOS / WebView: keep in viewport, avoid display:none (breaks select).
    ta.style.position = "fixed";
    ta.style.top = "0";
    ta.style.left = "0";
    ta.style.width = "2em";
    ta.style.height = "2em";
    ta.style.padding = "0";
    ta.style.margin = "0";
    ta.style.border = "none";
    ta.style.outline = "none";
    ta.style.boxShadow = "none";
    ta.style.background = "transparent";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    try { ta.setSelectionRange(0, text.length); } catch (e) {}
    var success = false;
    try {
      success = document.execCommand("copy");
    } catch (e) {
      success = false;
    }
    document.body.removeChild(ta);
    showToast(success ? ok : fail);
  }

  function copyText(text, okMsg) {
    var ok = okMsg || I18N[lang].copied;
    var fail = I18N[lang].copyFail;
    text = (text || "").replace(/^\s+|\s+$/g, "");
    if (!text) { showToast(fail); return; }
    // Prefer async Clipboard API on secure contexts; always have a sync fallback
    // for HTTP panel access, older WebViews, and some in-app browsers.
    if (navigator.clipboard && typeof navigator.clipboard.writeText === "function" && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(function () {
        showToast(ok);
      }).catch(function () {
        fallbackCopy(text, ok, fail);
      });
      return;
    }
    fallbackCopy(text, ok, fail);
  }

  // Deep-link import buttons: show guidance; the browser handles the scheme.
  document.querySelectorAll("a[data-deeplink]").forEach(function (a) {
    a.addEventListener("click", function () {
      showToast(I18N[lang].importing.split("{app}").join(a.getAttribute("data-deeplink") || ""));
    });
  });

  // Notice items expand / collapse.
  document.querySelectorAll(".notice-item").forEach(function (n) {
    n.addEventListener("click", function () { n.classList.toggle("open"); });
  });

  // Node rows: tap to expand the raw URI; copy button copies it.
  document.querySelectorAll(".n-row").forEach(function (n) {
    n.addEventListener("click", function (e) {
      if (e.target.closest && e.target.closest(".cp")) return;
      n.classList.toggle("open");
    });
    var cp = n.querySelector(".cp");
    if (cp) {
      cp.addEventListener("click", function () {
        copyText(cp.getAttribute("data-link") || "", I18N[lang].nodeCopied);
      });
    }
  });

  document.getElementById("copySub").addEventListener("click", function () {
    copyText(this.getAttribute("data-link") || "", I18N[lang].subCopied);
  });
  document.getElementById("copyV2ray").addEventListener("click", function () {
    copyText(this.getAttribute("data-link") || "", I18N[lang].v2rayCopied);
  });
  document.getElementById("copyAll").addEventListener("click", function () {
    var uris = [];
    document.querySelectorAll(".n-row .raw").forEach(function (r) {
      var t = (r.textContent || "").replace(/^\s+|\s+$/g, "");
      if (t) uris.push(t);
    });
    copyText(uris.join("\n"), I18N[lang].allCopied.split("{n}").join(String(uris.length)));
  });

  document.getElementById("lang-zh").addEventListener("click", function () { applyLang("zh"); });
  document.getElementById("lang-en").addEventListener("click", function () { applyLang("en"); });

  // Traffic bar: server renders the final width (works without JS); JS replays
  // the fill animation from zero on capable clients.
  var bar = document.getElementById("barFill");
  if (bar) {
    var pct = parseFloat(bar.getAttribute("data-pct")) || 0;
    bar.style.width = "0%";
    requestAnimationFrame(function () {
      requestAnimationFrame(function () { bar.style.width = pct + "%"; });
    });
  }

  applyLang(lang);
})();
</script>
</body>
</html>
`
