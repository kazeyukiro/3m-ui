package acme

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const settingKey = "panel_ssl"

// Settings controls panel HTTPS via Let's Encrypt (acmez) or manual cert files.
type Settings struct {
	Enabled     bool   `json:"enabled"`
	Domain      string `json:"domain"`
	Email       string `json:"email"`
	CacheDir    string `json:"cache_dir"`
	CertFile    string `json:"cert_file"`   // optional manual cert (PEM)
	KeyFile     string `json:"key_file"`    // optional manual key (PEM)
	ListenHTTP  string `json:"listen_http"` // e.g. ":80" for ACME HTTP-01 + redirect
	ListenTLS   string `json:"listen_tls"`  // e.g. ":443"
	Challenge   string `json:"challenge,omitempty"`
	DNSProvider string `json:"dns_provider,omitempty"`
	DNSToken    string `json:"dns_token,omitempty"`
	DNSZone     string `json:"dns_zone,omitempty"`
}

func DefaultSettings() Settings {
	cacheDir := "/var/lib/3m-ui/acme"
	if cfg := config.GlobalConfig; cfg != nil && cfg.Database.Path != "" {
		cacheDir = filepath.Join(filepath.Dir(cfg.Database.Path), "acme")
	}
	return Settings{
		CacheDir:   cacheDir,
		ListenHTTP: ":80",
		ListenTLS:  ":443",
	}
}

// allowedCertKeyPrefixes constrains where manual cert_file / key_file may
// reside. This blocks an administrator mistake (or a compromised admin
// session) from pointing the panel TLS loader at arbitrary files such as
// /etc/shadow or other secrets — only well-known TLS directories are
// permitted.
var allowedCertKeyPrefixes = []string{
	"/etc/ssl/",
	"/etc/3m-ui/",
	"/var/lib/3m-ui/",
	"/etc/letsencrypt/",
	"/root/.acme.sh/",
	"/etc/nginx/ssl/",
	"/etc/caddy/certs/",
}

func isAllowedCertKeyPath(p string) bool {
	clean := filepath.Clean(p)
	for _, prefix := range allowedCertKeyPrefixes {
		if strings.HasPrefix(clean, prefix) {
			return true
		}
	}
	return false
}

func LoadSettings(db *gorm.DB) (Settings, error) {
	s := DefaultSettings()
	if db == nil {
		return s, nil
	}
	var row models.PanelSetting
	if err := db.Where("key = ?", settingKey).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s, nil
		}
		return s, err
	}
	if strings.TrimSpace(row.Value) == "" {
		return s, nil
	}
	if err := json.Unmarshal([]byte(row.Value), &s); err != nil {
		return DefaultSettings(), err
	}
	s.Domain = strings.TrimSpace(s.Domain)
	s.Email = strings.TrimSpace(s.Email)
	s.CacheDir = strings.TrimSpace(s.CacheDir)
	s.Challenge = strings.TrimSpace(s.Challenge)
	s.DNSProvider = strings.TrimSpace(s.DNSProvider)
	s.DNSToken = strings.TrimSpace(s.DNSToken)
	s.DNSZone = strings.TrimSpace(s.DNSZone)
	if s.CacheDir == "" {
		s.CacheDir = DefaultSettings().CacheDir
	}
	if s.ListenHTTP == "" {
		s.ListenHTTP = ":80"
	}
	if s.ListenTLS == "" {
		s.ListenTLS = ":443"
	}
	return s, nil
}

func SaveSettings(db *gorm.DB, s Settings) error {
	s.Domain = strings.TrimSpace(s.Domain)
	s.Email = strings.TrimSpace(s.Email)
	s.CacheDir = strings.TrimSpace(s.CacheDir)
	if s.CacheDir == "" {
		s.CacheDir = DefaultSettings().CacheDir
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var row models.PanelSetting
	err = db.Where("key = ?", settingKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&models.PanelSetting{Key: settingKey, Value: string(raw)}).Error
	}
	if err != nil {
		return err
	}
	row.Value = string(raw)
	return db.Save(&row).Error
}

// Manager wraps acmez issuers or manual TLS for the panel listener.
type Manager struct {
	mu         sync.Mutex
	settings   Settings
	domainHTTP *domainHTTPIssuer
	ipIssuer   *ipIssuer
	dnsIssuer  *dnsIssuer
}

func NewManager(s Settings) (*Manager, error) {
	m := &Manager{settings: s}
	if !s.Enabled {
		return m, nil
	}
	if err := m.configure(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) configure() error {
	s := m.settings
	if s.CertFile != "" && s.KeyFile != "" {
		// Manual PEM pair — no acmez. Restrict to allowlisted directories so
		// the loader cannot be pointed at arbitrary sensitive files.
		if !isAllowedCertKeyPath(s.CertFile) || !isAllowedCertKeyPath(s.KeyFile) {
			return fmt.Errorf("panel SSL: cert_file/key_file must be under an allowed directory")
		}
		return nil
	}
	if m.ipIssuer != nil {
		m.ipIssuer.close()
		m.ipIssuer = nil
	}
	if m.dnsIssuer != nil {
		m.dnsIssuer.Close()
		m.dnsIssuer = nil
	}
	if m.domainHTTP != nil {
		m.domainHTTP.Close()
		m.domainHTTP = nil
	}
	if s.Domain == "" {
		return fmt.Errorf("panel SSL: domain or public IP is required for Let's Encrypt")
	}
	if err := os.MkdirAll(s.CacheDir, 0o700); err != nil {
		return fmt.Errorf("panel SSL: create cache dir: %w", err)
	}
	if IsIPHost(s.Domain) {
		if NeedsDNS01(s) {
			return fmt.Errorf("panel SSL: DNS-01 cannot be used with an IP address")
		}
		iss, err := newIPIssuer(s.Domain, s.Email, s.CacheDir)
		if err != nil {
			return err
		}
		m.ipIssuer = iss
		return nil
	}
	if NeedsDNS01(s) {
		iss, err := newDNSIssuer(s)
		if err != nil {
			return err
		}
		m.dnsIssuer = iss
		return nil
	}
	if IsWildcardDomain(s.Domain) {
		return fmt.Errorf("panel SSL: wildcard domains require DNS-01 (set challenge=dns-01 and a DNS API token)")
	}
	iss, err := newDomainHTTPIssuer(s.Domain, s.Email, s.CacheDir)
	if err != nil {
		return err
	}
	m.domainHTTP = iss
	return nil
}

// TLSConfig returns a *tls.Config suitable for https.Server, or nil if SSL is disabled.
func (m *Manager) TLSConfig() (*tls.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.settings
	if !s.Enabled {
		return nil, nil
	}
	if s.CertFile != "" && s.KeyFile != "" {
		if !isAllowedCertKeyPath(s.CertFile) || !isAllowedCertKeyPath(s.KeyFile) {
			return nil, fmt.Errorf("panel SSL: cert_file/key_file must be under an allowed directory")
		}
		cert, err := tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("panel SSL: load cert: %w", err)
		}
		return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, nil
	}
	if m.ipIssuer != nil {
		return &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: m.ipIssuer.GetCertificate,
		}, nil
	}
	if m.dnsIssuer != nil {
		return &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: m.dnsIssuer.GetCertificate,
		}, nil
	}
	if m.domainHTTP != nil {
		return &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: m.domainHTTP.GetCertificate,
		}, nil
	}
	if err := m.configure(); err != nil {
		return nil, err
	}
	if m.ipIssuer != nil {
		return &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: m.ipIssuer.GetCertificate,
		}, nil
	}
	if m.dnsIssuer != nil {
		return &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: m.dnsIssuer.GetCertificate,
		}, nil
	}
	if m.domainHTTP != nil {
		return &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: m.domainHTTP.GetCertificate,
		}, nil
	}
	return nil, fmt.Errorf("panel SSL: no certificate issuer configured")
}

func (m *Manager) HTTPHandler(fallback http.Handler) http.Handler {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ipIssuer != nil {
		return m.ipIssuer.httpHandler(fallback)
	}
	if m.domainHTTP != nil {
		return m.domainHTTP.httpHandler(fallback)
	}
	return fallback
}

// MarkChallengeBound tells the IP issuer permanent :80/:443 answer ACME challenges.
func (m *Manager) MarkChallengeBound() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ipIssuer != nil {
		m.ipIssuer.MarkChallengeBound()
	}
}

func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

func (m *Manager) Update(s Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings = s
	if m.domainHTTP != nil {
		m.domainHTTP.Close()
		m.domainHTTP = nil
	}
	if m.ipIssuer != nil {
		m.ipIssuer.close()
		m.ipIssuer = nil
	}
	if m.dnsIssuer != nil {
		m.dnsIssuer.Close()
		m.dnsIssuer = nil
	}
	if !s.Enabled {
		return nil
	}
	return m.configure()
}

// Status reports current SSL configuration for the panel UI.
func Status(db *gorm.DB) map[string]interface{} {
	s, _ := LoadSettings(db)
	hasCache := false
	if s.CacheDir != "" {
		entries, err := os.ReadDir(s.CacheDir)
		hasCache = err == nil && len(entries) > 0
	}
	manual := s.CertFile != "" && s.KeyFile != ""
	return map[string]interface{}{
		"enabled":       s.Enabled,
		"domain":        s.Domain,
		"email":         s.Email,
		"cache_dir":     s.CacheDir,
		"cert_file":     s.CertFile,
		"key_file":      s.KeyFile,
		"listen_http":   s.ListenHTTP,
		"listen_tls":    s.ListenTLS,
		"challenge":     s.Challenge,
		"dns_provider":  s.DNSProvider,
		"dns_zone":      s.DNSZone,
		"has_dns_token": strings.TrimSpace(s.DNSToken) != "",
		"mode":          modeLabel(s, manual),
		"is_ip":         IsIPHost(s.Domain),
		"is_wildcard":   IsWildcardDomain(s.Domain),
		"has_cache":     hasCache,
		"cert_path":     domainCertPath(s),
		"ip_profile":    ipCertProfile,
		"ip_note":       "IP certs use Let's Encrypt shortlived (~6 days); auto-renew when <48h remain (checked every 6h). Via acmez; validation HTTP-01 (:80) or TLS-ALPN-01 (:443).",
		"domain_note":   "Domain HTTP-01/DNS-01 certs auto-renew when fewer than 15 days remain (checked every 6h). Engine: acmez.",
		"dns_note":      "Wildcard (*.example.com) and DNS-01 need a DNS API token (Cloudflare Zone.DNS Edit). Apex is included on wildcard certs.",
		"engine":        "acmez",
	}
}

func modeLabel(s Settings, manual bool) string {
	if !s.Enabled {
		return "disabled"
	}
	if manual {
		return "manual"
	}
	if IsIPHost(s.Domain) {
		return "letsencrypt-ip"
	}
	if NeedsDNS01(s) {
		return "letsencrypt-dns01"
	}
	return "letsencrypt"
}

// LogHint prints a one-line hint after enabling SSL.
func LogHint(s Settings) {
	if !s.Enabled {
		return
	}
	if s.CertFile != "" {
		log.Printf("panel SSL: manual cert %s (listen %s)", s.CertFile, s.ListenTLS)
		return
	}
	if IsIPHost(s.Domain) {
		log.Printf("panel SSL: Let's Encrypt IP (shortlived) for %s (HTTP-01 %s / TLS-ALPN-01 %s, cache %s)",
			s.Domain, s.ListenHTTP, s.ListenTLS, s.CacheDir)
		return
	}
	if NeedsDNS01(s) {
		log.Printf("panel SSL: Let's Encrypt DNS-01 for %s (provider=%s, cache %s)",
			s.Domain, s.DNSProvider, s.CacheDir)
		return
	}
	log.Printf("panel SSL: Let's Encrypt for %s (HTTP %s → TLS %s, cache %s)",
		s.Domain, s.ListenHTTP, s.ListenTLS, s.CacheDir)
}

func domainCertPath(s Settings) string {
	if s.CertFile != "" {
		return s.CertFile
	}
	d := strings.TrimSpace(s.Domain)
	if d == "" {
		return s.CacheDir
	}
	if IsIPHost(d) {
		return filepath.Join(s.CacheDir, "ip-"+strings.ReplaceAll(normalizeIPHost(d), ":", "_")+"-cert.pem")
	}
	if NeedsDNS01(s) || IsWildcardDomain(d) {
		safe := strings.ReplaceAll(d, "*", "_wildcard_")
		return filepath.Join(s.CacheDir, "dns-"+safe+".crt")
	}
	return filepath.Join(s.CacheDir, "http-"+strings.ToLower(d)+".crt")
}
