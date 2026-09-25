package listener

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/acme"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

func (s *Service) SaveVersion(listenerID uint, reason string) error {
	l, err := s.GetByID(listenerID)
	if err != nil {
		return err
	}
	var count int64
	if err := s.db.Model(&models.ListenerVersion{}).Where("listener_id = ?", listenerID).Count(&count).Error; err != nil {
		return err
	}
	data, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("marshal listener snapshot: %w", err)
	}
	return s.db.Create(&models.ListenerVersion{ListenerID: listenerID, Version: int(count) + 1, Reason: reason, Snapshot: string(data)}).Error
}
func (s *Service) ListVersions(listenerID uint) ([]models.ListenerVersion, error) {
	var versions []models.ListenerVersion
	err := s.db.Where("listener_id = ?", listenerID).Order("version desc").Find(&versions).Error
	return versions, err
}
func (s *Service) RollbackVersion(listenerID uint, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var v models.ListenerVersion
	if err := s.db.Where("listener_id = ? AND version = ?", listenerID, version).First(&v).Error; err != nil {
		return fmt.Errorf("listener version not found: %w", err)
	}
	var target models.Listener
	if err := json.Unmarshal([]byte(v.Snapshot), &target); err != nil {
		return fmt.Errorf("invalid listener snapshot: %w", err)
	}
	target.ID = listenerID
	if err := ValidateModel(&target); err != nil {
		return fmt.Errorf("rollback validation failed: %w", err)
	}
	if err := s.ensureEndpointAvailable(&target); err != nil {
		return err
	}
	if err := s.SaveVersion(listenerID, "before-rollback"); err != nil {
		return err
	}
	if err := s.db.Save(&target).Error; err != nil {
		return err
	}
	s.scheduleConfigApply()
	return nil
}
func (s *Service) Clone(id uint, name, port string) (*models.Listener, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var src models.Listener
	if err := s.db.First(&src, id).Error; err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("listener name is required")
	}
	src.ID = 0
	src.CreatedAt = time.Time{}
	src.UpdatedAt = time.Time{}
	src.DeletedAt = gorm.DeletedAt{}
	src.Name = name
	if strings.TrimSpace(port) != "" {
		src.Port = strings.TrimSpace(port)
	}
	if err := ValidateModel(&src); err != nil {
		return nil, err
	}
	if err := s.ensureEndpointAvailable(&src); err != nil {
		return nil, err
	}
	if err := s.db.Create(&src).Error; err != nil {
		return nil, err
	}
	if err := s.SaveVersion(src.ID, "clone"); err != nil {
		log.Printf("warning: save cloned listener history: %v", err)
	}
	s.scheduleConfigApply()
	return &src, nil
}
func (s *Service) BatchCreate(list []models.Listener) ([]models.Listener, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(list) == 0 {
		return nil, fmt.Errorf("no listeners supplied")
	}
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	created := make([]models.Listener, 0, len(list))
	for i := range list {
		// Batch-created listeners must go through the same autofill path as
		// single Create so UUIDs / passwords / REALITY keys are generated and
		// the config passes mihomo validation. Without this, mihomo rejects the
		// entire config and the whole batch fails.
		if err := AutofillListenerDefaults(&list[i]); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("autofill listener defaults: %w", err)
		}
		if err := ValidateModel(&list[i]); err != nil {
			tx.Rollback()
			return nil, err
		}
		// GORM's default scope excludes soft-deleted rows, but the UNIQUE(name)
		// index covers them. Use ensureUniqueName so soft-deleted rows with the
		// same name are renamed out of the way before Create hits the index.
		if err := s.ensureUniqueName(&list[i]); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Create(&list[i]).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		created = append(created, list[i])
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	if err := s.ensureBatchEndpointsAvailable(created); err != nil {
		for _, l := range created {
			if rollbackErr := s.db.Delete(&l).Error; rollbackErr != nil {
				return nil, fmt.Errorf("%v; rollback batch listener %d failed: %w", err, l.ID, rollbackErr)
			}
		}
		return nil, err
	}
	for _, l := range created {
		if versionErr := s.SaveVersion(l.ID, "batch-create"); versionErr != nil {
			log.Printf("warning: save batch listener history for %d: %v", l.ID, versionErr)
		}
	}
	s.scheduleConfigApply()
	return created, nil
}
func (s *Service) ensureBatchEndpointsAvailable(created []models.Listener) error {
	for i := range created {
		if err := s.ensureEndpointAvailable(&created[i]); err != nil {
			return err
		}
		for j := i + 1; j < len(created); j++ {
			if portsOverlap(created[i].Port, created[j].Port) && listenerAddressesConflict(firstListenerAddress(created[i]), firstListenerAddress(created[j])) {
				return fmt.Errorf("listeners %q and %q have conflicting endpoints", created[i].Name, created[j].Name)
			}
		}
	}
	return nil
}
func (s *Service) BatchSetEnabled(ids []uint, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(ids) == 0 {
		return fmt.Errorf("no listener ids supplied")
	}
	var previous []models.Listener
	if err := s.db.Where("id IN ?", ids).Find(&previous).Error; err != nil {
		return err
	}
	if len(previous) != len(ids) {
		return fmt.Errorf("one or more listeners were not found")
	}
	if enabled {
		candidates := make([]models.Listener, len(previous))
		copy(candidates, previous)
		for i := range candidates {
			candidates[i].Enabled = true
		}
		if err := s.ensureBatchEndpointsAvailable(candidates); err != nil {
			return err
		}
	}
	for _, l := range previous {
		if err := s.SaveVersion(l.ID, "before-batch-enabled"); err != nil {
			return fmt.Errorf("save listener %d history before batch enable: %w", l.ID, err)
		}
	}
	if err := s.db.Model(&models.Listener{}).Where("id IN ?", ids).Update("enabled", enabled).Error; err != nil {
		return err
	}
	s.scheduleConfigApply()
	return nil
}
func (s *Service) DiffVersion(listenerID uint, version int) (string, error) {
	var v models.ListenerVersion
	if err := s.db.Where("listener_id = ? AND version = ?", listenerID, version).First(&v).Error; err != nil {
		return "", err
	}
	current, err := s.GetByID(listenerID)
	if err != nil {
		return "", err
	}
	cur, _ := json.MarshalIndent(current, "", "  ")
	var old any
	if err := json.Unmarshal([]byte(v.Snapshot), &old); err != nil {
		return "", err
	}
	oldJSON, _ := json.MarshalIndent(old, "", "  ")
	return fmt.Sprintf("--- version/%d\n+++ current\n- %s\n+ %s", version, strings.TrimSpace(string(oldJSON)), strings.TrimSpace(string(cur))), nil
}
func (s *Service) CreateTemplate(t *models.ListenerTemplate) error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("template name is required")
	}
	if strings.TrimSpace(t.Protocol) == "" {
		return fmt.Errorf("template protocol is required")
	}
	probe := &models.Listener{Name: t.Name, Protocol: t.Protocol, Port: "1", Config: t.Config}
	if err := ValidateModel(probe); err != nil {
		return err
	}
	if err := s.db.Create(t).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("template name %q already exists", strings.TrimSpace(t.Name))
		}
		return err
	}
	return nil
}
func (s *Service) ListTemplates() ([]models.ListenerTemplate, error) {
	var out []models.ListenerTemplate
	err := s.db.Order("name asc").Find(&out).Error
	return out, err
}
func (s *Service) GetTemplate(id uint) (*models.ListenerTemplate, error) {
	var t models.ListenerTemplate
	if err := s.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}
func (s *Service) DeleteTemplate(id uint) error {
	return s.db.Unscoped().Delete(&models.ListenerTemplate{}, id).Error
}
func (s *Service) InstantiateTemplate(templateID uint, name, port string) (*models.Listener, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("listener name is required")
	}
	l := &models.Listener{Name: name, Protocol: t.Protocol, Port: strings.TrimSpace(port), BindAddress: "0.0.0.0", Enabled: true, Config: t.Config}
	if l.Port == "" {
		l.Port = "0"
	}
	if err := ValidateModel(l); err != nil {
		return nil, err
	}
	if err := s.ensureEndpointAvailable(l); err != nil {
		return nil, err
	}
	if err := s.db.Create(l).Error; err != nil {
		return nil, err
	}
	if err := s.SaveVersion(l.ID, "template"); err != nil {
		log.Printf("warning: save template listener history: %v", err)
	}
	s.scheduleConfigApply()
	return l, nil
}

// ApplyCertificateInput applies a TLS certificate pair to multiple listeners' config.
type ApplyCertificateInput struct {
	IDs          []uint `json:"ids"`
	Certificate  string `json:"certificate"`
	PrivateKey   string `json:"private_key"`
	CertFile     string `json:"cert_file"`
	KeyFile      string `json:"key_file"`
	FromPanelSSL bool   `json:"from_panel_ssl"`
}

// ApplyCertificateResult reports per-node outcomes for bulk certificate apply.
type ApplyCertificateResult struct {
	Updated []uint                    `json:"updated"`
	Failed  []ApplyCertificateFailure `json:"failed"`
}

// ApplyCertificateFailure is one listener that could not receive the certificate.
type ApplyCertificateFailure struct {
	ID    uint   `json:"id"`
	Name  string `json:"name,omitempty"`
	Error string `json:"error"`
}

// BatchApplyCertificate writes certificate + private-key into each listener's
// Config JSON (Mihomo listener fields) and reloads the core once.
func (s *Service) BatchApplyCertificate(in ApplyCertificateInput) (*ApplyCertificateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(in.IDs) == 0 {
		return nil, fmt.Errorf("no listener ids supplied")
	}
	certPEM, keyPEM, err := resolveCertificatePair(s, in)
	if err != nil {
		return nil, err
	}
	out := &ApplyCertificateResult{
		Updated: make([]uint, 0, len(in.IDs)),
		Failed:  make([]ApplyCertificateFailure, 0),
	}
	for _, id := range in.IDs {
		var l models.Listener
		if err := s.db.First(&l, id).Error; err != nil {
			out.Failed = append(out.Failed, ApplyCertificateFailure{ID: id, Error: "listener not found"})
			continue
		}
		if err := s.SaveVersion(l.ID, "before-batch-certificate"); err != nil {
			out.Failed = append(out.Failed, ApplyCertificateFailure{ID: id, Name: l.Name, Error: err.Error()})
			continue
		}
		cfg := map[string]interface{}{}
		raw := strings.TrimSpace(l.Config)
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				out.Failed = append(out.Failed, ApplyCertificateFailure{ID: id, Name: l.Name, Error: "invalid config JSON"})
				continue
			}
		}
		if cfg == nil {
			cfg = map[string]interface{}{}
		}
		cfg["certificate"] = certPEM
		cfg["private-key"] = keyPEM
		encoded, err := json.Marshal(cfg)
		if err != nil {
			out.Failed = append(out.Failed, ApplyCertificateFailure{ID: id, Name: l.Name, Error: err.Error()})
			continue
		}
		l.Config = string(encoded)
		if err := AutofillListenerDefaults(&l); err != nil {
			out.Failed = append(out.Failed, ApplyCertificateFailure{ID: id, Name: l.Name, Error: err.Error()})
			continue
		}
		if err := ValidateModel(&l); err != nil {
			out.Failed = append(out.Failed, ApplyCertificateFailure{ID: id, Name: l.Name, Error: err.Error()})
			continue
		}
		if err := s.db.Save(&l).Error; err != nil {
			out.Failed = append(out.Failed, ApplyCertificateFailure{ID: id, Name: l.Name, Error: err.Error()})
			continue
		}
		out.Updated = append(out.Updated, id)
	}
	if len(out.Updated) > 0 {
		s.scheduleConfigApply()
	}
	return out, nil
}

// sanitizeIPFilename converts an IP address to a filesystem-safe name.
// Mirrors the logic in acme/ip_issuer.go:sanitizeIPFilename.
func sanitizeIPFilename(ip string) string {
	ip = strings.ReplaceAll(ip, ":", "-")
	ip = strings.ReplaceAll(ip, ".", "-")
	return ip
}

func resolveCertificatePair(s *Service, in ApplyCertificateInput) (certPEM, keyPEM string, err error) {
	certPEM = strings.TrimSpace(in.Certificate)
	keyPEM = strings.TrimSpace(in.PrivateKey)
	certFile := strings.TrimSpace(in.CertFile)
	keyFile := strings.TrimSpace(in.KeyFile)

	if in.FromPanelSSL {
		st, loadErr := acme.LoadSettings(s.db)
		if loadErr != nil {
			return "", "", fmt.Errorf("load panel SSL settings: %w", loadErr)
		}
		certFile = strings.TrimSpace(st.CertFile)
		keyFile = strings.TrimSpace(st.KeyFile)
		if certFile == "" || keyFile == "" {
			// Panel is using ACME (autocert or IP cert), not manual PEM files.
			// Try to read the IP certificate PEM files from the ACME cache dir.
			// IP certs are stored as <cacheDir>/ip-<sanitized-ip>-cert.pem
			// and <cacheDir>/ip-<sanitized-ip>-key.pem (standard PEM format).
			// Domain ACME (autocert.DirCache) stores certs in Go binary format
			// (not PEM), so we can't extract PEM from there — fall through to
			// the "paste PEM" error below.
			if st.CacheDir != "" && st.Domain != "" {
				ipCert := filepath.Join(st.CacheDir, "ip-"+sanitizeIPFilename(st.Domain)+"-cert.pem")
				ipKey := filepath.Join(st.CacheDir, "ip-"+sanitizeIPFilename(st.Domain)+"-key.pem")
				if _, e := os.Stat(ipCert); e == nil {
					if _, e2 := os.Stat(ipKey); e2 == nil {
						certFile = ipCert
						keyFile = ipKey
					}
				}
			}
			if certFile == "" || keyFile == "" {
				// Try autocert DirCache: the file <cacheDir>/<domain>
				// contains a PEM block with PRIVATE KEY + CERTIFICATE
				// concatenated (autocert cacheGet format). Read it
				// and split into separate cert/key PEM.
				if st.CacheDir != "" && st.Domain != "" {
					autocertCachePath := filepath.Join(st.CacheDir, st.Domain)
					if data, e := os.ReadFile(autocertCachePath); e == nil {
						pemStr := string(data)
						if strings.Contains(pemStr, "PRIVATE KEY") && strings.Contains(pemStr, "CERTIFICATE") {
							// Extract private key block
							keyStart := strings.Index(pemStr, "-----BEGIN")
							keyEndMarker := "-----END"
							keyEndIdx := strings.Index(pemStr, keyEndMarker)
							if keyEndIdx > keyStart {
								// Find the newline after END
								restAfterEnd := pemStr[keyEndIdx:]
								newlineAfterEnd := strings.Index(restAfterEnd, "\n")
								if newlineAfterEnd > 0 {
									keyPEM = strings.TrimSpace(pemStr[keyStart : keyEndIdx+newlineAfterEnd])
								}
							}
							// Extract certificate block(s)
							certStart := strings.Index(pemStr, "-----BEGIN CERTIFICATE-----")
							if certStart > 0 {
								certPEM = strings.TrimSpace(pemStr[certStart:])
							}
							if certPEM != "" && keyPEM != "" {
								return certPEM, keyPEM, nil
							}
						}
					}
				}
				return "", "", fmt.Errorf("panel SSL uses ACME with domain %q. The autocert cache PEM was not found at %s/%s — the cert may not have been issued yet, or the panel was restarted before the first TLS handshake. Visit the panel over HTTPS once to trigger cert issuance, then retry. Alternatively, paste the certificate PEM below or use /etc/letsencrypt/live/<domain>/ paths.", st.Domain, st.CacheDir, st.Domain)
			}
		}
	}

	if certFile != "" || keyFile != "" {
		if certFile == "" || keyFile == "" {
			return "", "", fmt.Errorf("cert_file and key_file must both be set")
		}
		if !isAllowedTLSPath(certFile) || !isAllowedTLSPath(keyFile) {
			return "", "", fmt.Errorf("cert_file/key_file must be under an allowed TLS directory")
		}
		cb, rerr := os.ReadFile(filepath.Clean(certFile))
		if rerr != nil {
			return "", "", fmt.Errorf("read cert_file: %w", rerr)
		}
		kb, rerr := os.ReadFile(filepath.Clean(keyFile))
		if rerr != nil {
			return "", "", fmt.Errorf("read key_file: %w", rerr)
		}
		certPEM = string(cb)
		keyPEM = string(kb)
	}

	certPEM = strings.TrimSpace(certPEM)
	keyPEM = strings.TrimSpace(keyPEM)
	if certPEM == "" || keyPEM == "" {
		return "", "", fmt.Errorf("certificate and private_key are required (PEM text, files, or from_panel_ssl)")
	}
	if !strings.Contains(certPEM, "BEGIN CERTIFICATE") {
		return "", "", fmt.Errorf("certificate does not look like a PEM certificate")
	}
	if !strings.Contains(keyPEM, "PRIVATE KEY") {
		return "", "", fmt.Errorf("private_key does not look like a PEM private key")
	}
	return certPEM, keyPEM, nil
}

func isAllowedTLSPath(p string) bool {
	clean := filepath.Clean(p)
	prefixes := []string{
		"/etc/ssl/",
		"/etc/3m-ui/",
		"/var/lib/3m-ui/",
		"/etc/letsencrypt/",
		"/root/.acme.sh/",
		"/etc/nginx/ssl/",
		"/etc/caddy/certs/",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(clean, prefix) {
			return true
		}
	}
	return false
}
