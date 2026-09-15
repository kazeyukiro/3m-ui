package listener

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	dbconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gorm.io/gorm"
)

type Service struct {
	db          *gorm.DB
	configPath  string
	mihomoApply interface {
		ApplyConfig(string) error
	}
	mu sync.Mutex

	// Async ApplyConfig (same idea as user.Service credentials sync): Create/Update/Delete
	// must not block the HTTP response on Mihomo restart, or the browser reports a false
	// "Cannot reach the panel API" while the DB write already succeeded.
	applySchedMu sync.Mutex
	applyTimer   *time.Timer
	applyPending bool
}

func NewService(db *gorm.DB, configPath string, mihomoApply interface {
	ApplyConfig(string) error
}) *Service {
	return &Service{db: db, configPath: configPath, mihomoApply: mihomoApply}
}

// scheduleConfigApply debounces Mihomo config regeneration after listener mutations.
// Always returns immediately to the HTTP layer; failures are logged, not rolled back.
func (s *Service) scheduleConfigApply() {
	if s == nil {
		return
	}
	s.applySchedMu.Lock()
	defer s.applySchedMu.Unlock()
	s.applyPending = true
	if s.applyTimer != nil {
		s.applyTimer.Stop()
	}
	s.applyTimer = time.AfterFunc(400*time.Millisecond, s.flushConfigApply)
}

func (s *Service) flushConfigApply() {
	if s == nil {
		return
	}
	s.applySchedMu.Lock()
	run := s.applyPending
	s.applyPending = false
	s.applySchedMu.Unlock()
	if !run {
		return
	}
	// Generate under the mutation lock, ApplyConfig outside — ApplyConfig may
	// restart Mihomo and block for a long time; holding s.mu would deadlock
	// concurrent Create/Update/Delete (and the concurrency test).
	s.mu.Lock()
	yamlContent, err := s.generateConfigYAMLLocked()
	s.mu.Unlock()
	if err != nil {
		log.Printf("warning: Mihomo config generate after listener change failed: %v", err)
		return
	}
	if err := s.applyGeneratedYAML(yamlContent); err != nil {
		log.Printf("warning: Mihomo config reload after listener change failed: %v", err)
	}
}

// FlushConfigApplyForTest runs a pending async apply immediately (tests only).
func (s *Service) FlushConfigApplyForTest() {
	if s == nil {
		return
	}
	s.applySchedMu.Lock()
	if s.applyTimer != nil {
		s.applyTimer.Stop()
		s.applyTimer = nil
	}
	s.applyPending = true
	s.applySchedMu.Unlock()
	s.flushConfigApply()
}

func (s *Service) Create(l *models.Listener) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	autoPort := false
	if l != nil {
		port := strings.TrimSpace(l.Port)
		if port == "" || port == "0" {
			autoPort = true
			p, err := s.allocateFreePort()
			if err != nil {
				return err
			}
			l.Port = p
		}
	}
	if err := AutofillListenerDefaults(l); err != nil {
		return fmt.Errorf("autofill listener defaults: %w", err)
	}
	if err := ValidateModel(l); err != nil {
		return err
	}
	if err := s.ensureEndpointAvailable(l); err != nil {
		// Retry with a new free port when allocation raced or form reused a port.
		msg := strings.ToLower(err.Error())
		if autoPort || strings.Contains(msg, "conflicts with existing") {
			var last error = err
			var tried []int
			if cur, e := strconv.Atoi(strings.TrimSpace(l.Port)); e == nil && cur > 0 {
				tried = append(tried, cur)
			}
			for attempt := 0; attempt < 8; attempt++ {
				p, aerr := s.allocateFreePort(tried...)
				if aerr != nil {
					return last
				}
				if n, e := strconv.Atoi(p); e == nil {
					tried = append(tried, n)
				}
				l.Port = p
				if last = s.ensureEndpointAvailable(l); last == nil {
					break
				}
				if cur, e := strconv.Atoi(strings.TrimSpace(l.Port)); e == nil && cur > 0 {
					tried = append(tried, cur)
				}
			}
			if last != nil {
				return last
			}
		} else {
			return err
		}
	}
	if err := s.db.Create(l).Error; err != nil {
		// Concurrent create or leftover unique row: reclaim soft-deleted and retry once.
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "unique") || strings.Contains(msg, "constraint") || strings.Contains(msg, "duplicate") {
			_ = s.ensureUniqueName(l)
			if err2 := s.db.Create(l).Error; err2 != nil {
				return fmt.Errorf("listener name %q already exists", strings.TrimSpace(l.Name))
			}
		} else {
			return fmt.Errorf("failed to create listener: %w", err)
		}
	}
	// Persist history best-effort; do not fail the create if version write fails.
	if err := s.SaveVersion(l.ID, "create"); err != nil {
		log.Printf("warning: save listener create history: %v", err)
	}
	s.scheduleConfigApply()
	return nil
}

func (s *Service) GetAll() ([]models.Listener, error) {
	var list []models.Listener
	if err := s.db.Order("id desc").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch listeners: %w", err)
	}
	return list, nil
}

func (s *Service) GetByID(id uint) (*models.Listener, error) {
	var l models.Listener
	if err := s.db.First(&l, id).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch listener by id %d: %w", id, err)
	}
	return &l, nil
}

func (s *Service) Update(l *models.Listener) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := AutofillListenerDefaults(l); err != nil {
		return fmt.Errorf("autofill listener defaults: %w", err)
	}
	if err := ValidateModel(l); err != nil {
		return err
	}
	var previous models.Listener
	if err := s.db.First(&previous, l.ID).Error; err != nil {
		return fmt.Errorf("failed to load previous listener: %w", err)
	}
	if err := s.ensureEndpointAvailable(l); err != nil {
		return err
	}
	if err := s.SaveVersion(previous.ID, "before-update"); err != nil {
		return fmt.Errorf("save listener history: %w", err)
	}
	if err := s.db.Save(l).Error; err != nil {
		return fmt.Errorf("failed to update listener: %w", err)
	}
	s.scheduleConfigApply()
	return nil
}

func (s *Service) Delete(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var previous models.Listener
	if err := s.db.First(&previous, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Idempotent: UI double-click / stale list after a successful delete.
			return nil
		}
		return fmt.Errorf("failed to fetch listener before delete: %w", err)
	}
	if err := s.SaveVersion(id, "before-delete"); err != nil {
		return fmt.Errorf("save listener history: %w", err)
	}

	// Free UNIQUE(name) before soft-delete so create-after-delete cannot race.
	freed := fmt.Sprintf("%s__deleted_%d", previous.Name, id)
	if err := s.db.Model(&models.Listener{}).Where("id = ?", id).Update("name", freed).Error; err != nil {
		return fmt.Errorf("free listener name before delete: %w", err)
	}
	if err := s.db.Where("listener_id = ?", id).Delete(&models.ListenerUser{}).Error; err != nil {
		_ = s.db.Model(&models.Listener{}).Where("id = ?", id).Update("name", previous.Name).Error
		return fmt.Errorf("failed to delete listener bindings: %w", err)
	}
	if err := s.db.Delete(&models.Listener{}, id).Error; err != nil {
		_ = s.db.Model(&models.Listener{}).Where("id = ?", id).Update("name", previous.Name).Error
		return fmt.Errorf("failed to delete listener: %w", err)
	}
	s.scheduleConfigApply()
	return nil
}

func (s *Service) ensureUniqueName(candidate *models.Listener) error {
	name := strings.TrimSpace(candidate.Name)
	if name == "" {
		return fmt.Errorf("listener name is required")
	}
	// SQLite UNIQUE(name) covers soft-deleted rows. Reclaim every non-live row
	// that still holds this name (hard-delete the soft-deleted tombstone).
	var rows []models.Listener
	q := s.db.Unscoped().Where("name = ?", name)
	if candidate.ID != 0 {
		q = q.Where("id <> ?", candidate.ID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return fmt.Errorf("check listener name uniqueness: %w", err)
	}
	for _, row := range rows {
		if row.DeletedAt.Valid {
			if err := s.db.Unscoped().Delete(&models.Listener{}, row.ID).Error; err != nil {
				newName := fmt.Sprintf("%s__deleted_%d_%d", name, row.ID, time.Now().UnixNano())
				if renErr := s.db.Unscoped().Model(&models.Listener{}).Where("id = ?", row.ID).Update("name", newName).Error; renErr != nil {
					return fmt.Errorf("reclaim soft-deleted name %q: %v; rename: %w", name, err, renErr)
				}
			}
			continue
		}
		return fmt.Errorf("listener name %q already exists", name)
	}
	return nil
}

func (s *Service) ensureEndpointAvailable(candidate *models.Listener) error {
	if err := s.ensureUniqueName(candidate); err != nil {
		return err
	}
	// Check every live (non-soft-deleted) listener: a disabled node still owns its
	// port in the panel DB and would conflict as soon as it is re-enabled.
	var listeners []models.Listener
	q := s.db.Where("id <> ?", candidate.ID)
	if candidate.ID == 0 {
		q = s.db
	}
	if err := q.Find(&listeners).Error; err != nil {
		return fmt.Errorf("check listener endpoint conflicts: %w", err)
	}
	for _, existing := range listeners {
		if !portsOverlap(candidate.Port, existing.Port) {
			continue
		}
		if listenerAddressesConflict(firstListenerAddress(*candidate), firstListenerAddress(existing)) {
			return fmt.Errorf("listener %q conflicts with existing listener %q on %s:%s", candidate.Name, existing.Name, firstListenerAddress(existing), existing.Port)
		}
	}
	return nil
}

func (s *Service) RegenerateConfig() error {
	// Snapshot YAML under the mutation lock, then ApplyConfig without holding
	// s.mu so listener CRUD is not blocked for the duration of Mihomo restart.
	s.mu.Lock()
	yamlContent, err := s.generateConfigYAMLLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.applyGeneratedYAML(yamlContent)
}

// regenerateConfigLocked generates and applies while the caller holds s.mu.
// Prefer RegenerateConfig / flushConfigApply for production paths so ApplyConfig
// does not keep the mutation lock. Kept for any remaining call sites that already hold mu.
func (s *Service) regenerateConfigLocked() error {
	yamlContent, err := s.generateConfigYAMLLocked()
	if err != nil {
		return err
	}
	return s.applyGeneratedYAML(yamlContent)
}

func (s *Service) generateConfigYAMLLocked() (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("listener service not initialized")
	}
	engine := dbconfig.NewConfigEngine(s.db)
	yamlContent, err := engine.GenerateFinalConfig()
	if err != nil {
		return "", fmt.Errorf("generate Mihomo configuration: %w", err)
	}
	return yamlContent, nil
}

func (s *Service) applyGeneratedYAML(yamlContent string) error {
	if s.mihomoApply != nil {
		// Report activation failures while CRUD can still restore the database.
		return s.mihomoApply.ApplyConfig(yamlContent)
	}
	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config.yaml.tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(yamlContent); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.configPath); err != nil {
		return fmt.Errorf("replace Mihomo config: %w", err)
	}
	return nil
}

func (s *Service) TriggerReload(_ uint) error { return s.RegenerateConfig() }
