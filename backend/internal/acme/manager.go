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

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"golang.org/x/crypto/acme/autocert"
	"gorm.io/gorm"
)

const settingKey = "panel_ssl"

// Settings controls panel HTTPS via Let's Encrypt (autocert) or manual cert files.
type Settings struct {
	Enabled    bool   `json:"enabled"`
	Domain     string `json:"domain"`
	Email      string `json:"email"`
	CacheDir   string `json:"cache_dir"`
	CertFile   string `json:"cert_file"`   // optional manual cert (PEM)
	KeyFile    string `json:"key_file"`    // optional manual key (PEM)
	ListenHTTP string `json:"listen_http"` // e.g. ":80" for ACME HTTP-01 + redirect
	ListenTLS  string `json:"listen_tls"`  // e.g. ":443"
}

func DefaultSettings() Settings {
	return Settings{
		CacheDir:   "/var/lib/3m-ui/acme",
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

// Manager wraps autocert or manual TLS for the panel listener.
type Manager struct {
	mu       sync.Mutex
	settings Settings
	manager  *autocert.Manager
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
		// Manual PEM pair — no autocert. Restrict to allowlisted directories so
		// the loader cannot be pointed at arbitrary sensitive files.
		if !isAllowedCertKeyPath(s.CertFile) || !isAllowedCertKeyPath(s.KeyFile) {
			return fmt.Errorf("panel SSL: cert_file/key_file must be under an allowed directory")
		}
		return nil
	}
	if s.Domain == "" {
		return fmt.Errorf("panel SSL: domain is required for Let's Encrypt")
	}
	if err := os.MkdirAll(s.CacheDir, 0o700); err != nil {
		return fmt.Errorf("panel SSL: create cache dir: %w", err)
	}
	hostPolicy := autocert.HostWhitelist(s.Domain)
	m.manager = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: hostPolicy,
		Cache:      autocert.DirCache(s.CacheDir),
		Email:      s.Email,
	}
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
			return nil, fmt.Errorf("load manual cert: %w", err)
		}
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}, nil
	}
	if m.manager == nil {
		if err := m.configure(); err != nil {
			return nil, err
		}
	}
	return m.manager.TLSConfig(), nil
}

// HTTPHandler returns the ACME HTTP-01 challenge handler (nil if not using autocert).
func (m *Manager) HTTPHandler(fallback http.Handler) http.Handler {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.manager == nil {
		return fallback
	}
	return m.manager.HTTPHandler(fallback)
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
	m.manager = nil
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
		"enabled":     s.Enabled,
		"domain":      s.Domain,
		"email":       s.Email,
		"cache_dir":   s.CacheDir,
		"cert_file":   s.CertFile,
		"key_file":    s.KeyFile,
		"listen_http": s.ListenHTTP,
		"listen_tls":  s.ListenTLS,
		"mode":        modeLabel(s, manual),
		"has_cache":   hasCache,
		"cert_path":   filepath.Join(s.CacheDir, s.Domain),
	}
}

func modeLabel(s Settings, manual bool) string {
	if !s.Enabled {
		return "disabled"
	}
	if manual {
		return "manual"
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
	log.Printf("panel SSL: Let's Encrypt for %s (HTTP %s → TLS %s, cache %s)",
		s.Domain, s.ListenHTTP, s.ListenTLS, s.CacheDir)
}
