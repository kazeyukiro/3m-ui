package listener

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	dbconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gorm.io/gorm"
)

type Service struct {
	db          *gorm.DB
	configPath  string
	mihomoApply interface{ ApplyConfig(string) error }
	mu          sync.Mutex
}

func NewService(db *gorm.DB, configPath string, mihomoApply interface{ ApplyConfig(string) error }) *Service {
	return &Service{db: db, configPath: configPath, mihomoApply: mihomoApply}
}

func (s *Service) Create(l *models.Listener) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := AutofillListenerDefaults(l); err != nil {
		return fmt.Errorf("autofill listener defaults: %w", err)
	}
	if err := ValidateModel(l); err != nil {
		return err
	}
	if err := s.ensureEndpointAvailable(l); err != nil {
		return err
	}
	if err := s.db.Create(l).Error; err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	if err := s.regenerateConfigLocked(); err != nil {
		// Hard-delete: soft-delete would keep UNIQUE(name) occupied and block retries.
		if rollbackErr := s.db.Unscoped().Delete(&models.Listener{}, l.ID).Error; rollbackErr != nil {
			return fmt.Errorf("%v; rollback newly created listener failed: %w", err, rollbackErr)
		}
		return err
	}
	if err := s.SaveVersion(l.ID, "create"); err != nil {
		// Version history is part of the create contract: do not report a
		// failed create while leaving a listener that the caller cannot account for.
		if rollbackErr := s.db.Unscoped().Delete(&models.Listener{}, l.ID).Error; rollbackErr != nil {
			return fmt.Errorf("save listener history: %v; rollback created listener failed: %w", err, rollbackErr)
		}
		if regenerateErr := s.regenerateConfigLocked(); regenerateErr != nil {
			return fmt.Errorf("save listener history: %v; listener rolled back but previous configuration regeneration failed: %w", err, regenerateErr)
		}
		return fmt.Errorf("save listener history: %w", err)
	}
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
	if err := s.regenerateConfigLocked(); err != nil {
		if rollbackErr := s.db.Save(&previous).Error; rollbackErr != nil {
			return fmt.Errorf("%v; rollback listener failed: %w", err, rollbackErr)
		}
		if regenerateErr := s.regenerateConfigLocked(); regenerateErr != nil {
			return fmt.Errorf("%v; listener restored but previous configuration regeneration failed: %w", err, regenerateErr)
		}
		return err
	}
	return nil
}

func (s *Service) Delete(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var previous models.Listener
	if err := s.db.First(&previous, id).Error; err != nil {
		return fmt.Errorf("failed to fetch listener before delete: %w", err)
	}
	if err := s.SaveVersion(id, "before-delete"); err != nil {
		return fmt.Errorf("save listener history: %w", err)
	}

	var bindings []models.ListenerUser
	if err := s.db.Where("listener_id = ?", id).Find(&bindings).Error; err != nil {
		return fmt.Errorf("failed to fetch listener bindings: %w", err)
	}
	if err := s.db.Where("listener_id = ?", id).Delete(&models.ListenerUser{}).Error; err != nil {
		return fmt.Errorf("failed to delete listener bindings: %w", err)
	}
	if err := s.db.Delete(&models.Listener{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete listener: %w", err)
	}
	// Soft-delete keeps the row; rename so UNIQUE(name) can be reused immediately.
	freed := fmt.Sprintf("%s__deleted_%d", previous.Name, id)
	_ = s.db.Unscoped().Model(&models.Listener{}).Where("id = ?", id).Update("name", freed).Error
	if err := s.regenerateConfigLocked(); err != nil {
		if rollbackErr := s.db.Unscoped().Save(&previous).Error; rollbackErr != nil {
			return fmt.Errorf("%v; rollback deleted listener failed: %w", err, rollbackErr)
		}
		if len(bindings) > 0 {
			if restoreErr := s.db.Unscoped().Save(&bindings).Error; restoreErr != nil {
				return fmt.Errorf("%v; rollback listener bindings failed: %w", err, restoreErr)
			}
		}
		if regenerateErr := s.regenerateConfigLocked(); regenerateErr != nil {
			return fmt.Errorf("%v; listener and bindings restored but previous configuration regeneration failed: %w", err, regenerateErr)
		}
		return err
	}
	return nil
}

func (s *Service) ensureUniqueName(candidate *models.Listener) error {
	name := strings.TrimSpace(candidate.Name)
	if name == "" {
		return fmt.Errorf("listener name is required")
	}
	// Only live rows should block create/update. Soft-deleted rows still occupy
	// the SQLite UNIQUE index, so reclaim those names first.
	var active int64
	q := s.db.Model(&models.Listener{}).Where("name = ?", name)
	if candidate.ID != 0 {
		q = q.Where("id <> ?", candidate.ID)
	}
	if err := q.Count(&active).Error; err != nil {
		return fmt.Errorf("check listener name uniqueness: %w", err)
	}
	if active > 0 {
		return fmt.Errorf("listener name %q already exists", name)
	}
	var soft []models.Listener
	sq := s.db.Unscoped().Where("name = ? AND deleted_at IS NOT NULL", name)
	if candidate.ID != 0 {
		sq = sq.Where("id <> ?", candidate.ID)
	}
	if err := sq.Find(&soft).Error; err != nil {
		return fmt.Errorf("check soft-deleted listener names: %w", err)
	}
	for _, row := range soft {
		newName := fmt.Sprintf("%s__deleted_%d", name, row.ID)
		if err := s.db.Unscoped().Model(&models.Listener{}).Where("id = ?", row.ID).Update("name", newName).Error; err != nil {
			return fmt.Errorf("reclaim soft-deleted name %q: %w", name, err)
		}
	}
	return nil
}

func (s *Service) ensureEndpointAvailable(candidate *models.Listener) error {
	if err := s.ensureUniqueName(candidate); err != nil {
		return err
	}
	var listeners []models.Listener
	if err := s.db.Where("enabled = ? AND id <> ?", true, candidate.ID).Find(&listeners).Error; err != nil {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.regenerateConfigLocked()
}

func (s *Service) regenerateConfigLocked() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("listener service not initialized")
	}
	engine := dbconfig.NewConfigEngine(s.db)
	yamlContent, err := engine.GenerateFinalConfig()
	if err != nil {
		return fmt.Errorf("generate Mihomo configuration: %w", err)
	}
	if s.mihomoApply != nil {
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
