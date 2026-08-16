package node

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gorm.io/gorm"
)

type Service struct {
	db         *gorm.DB
	configPath string
	mihomo     *mihomo.Service
}

// NewService constructs a node/listener service.
// Optional mihomoSvc is used for hot-reload after config regeneration.
func NewService(db *gorm.DB, configPath string, mihomoSvc *mihomo.Service) *Service {
	return &Service{db: db, configPath: configPath, mihomo: mihomoSvc}
}

func (s *Service) Create(l *models.Listener) error {
	if err := ValidateNode(l); err != nil {
		return err
	}

	if err := s.db.Create(l).Error; err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	if err := s.RegenerateConfig(); err != nil {
		if deleteErr := s.db.Delete(&models.Listener{}, l.ID).Error; deleteErr != nil {
			return fmt.Errorf("failed to regenerate config: %v; rollback node %d failed: %w", err, l.ID, deleteErr)
		}
		return fmt.Errorf("failed to regenerate config: %w", err)
	}

	return nil
}

func (s *Service) GetAll() ([]models.Listener, error) {
	var list []models.Listener
	if err := s.db.Find(&list).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %w", err)
	}
	return list, nil
}

func (s *Service) GetByID(id uint) (*models.Listener, error) {
	var l models.Listener
	if err := s.db.First(&l, id).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch node by id %d: %w", id, err)
	}
	return &l, nil
}

func (s *Service) Update(l *models.Listener) error {
	if err := ValidateNode(l); err != nil {
		return err
	}

	var previous models.Listener
	if err := s.db.First(&previous, l.ID).Error; err != nil {
		return fmt.Errorf("failed to load node %d: %w", l.ID, err)
	}

	if err := s.db.Save(l).Error; err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}

	if err := s.RegenerateConfig(); err != nil {
		if restoreErr := s.db.Save(&previous).Error; restoreErr != nil {
			return fmt.Errorf("failed to regenerate config: %v; rollback node %d failed: %w", err, l.ID, restoreErr)
		}
		return fmt.Errorf("failed to regenerate config: %w", err)
	}

	return nil
}

func (s *Service) Delete(id uint) error {
	var previous models.Listener
	if err := s.db.First(&previous, id).Error; err != nil {
		return fmt.Errorf("failed to load node %d: %w", id, err)
	}

	// Soft-delete join bindings first so credential regeneration stays consistent.
	if err := s.db.Where("listener_id = ?", id).Delete(&models.ListenerUser{}).Error; err != nil {
		return fmt.Errorf("failed to delete node bindings: %w", err)
	}
	if err := s.db.Delete(&models.Listener{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	if err := s.RegenerateConfig(); err != nil {
		// Best-effort restore so a config write failure does not leave a half-deleted node.
		_ = s.db.Unscoped().Model(&models.Listener{}).Where("id = ?", id).Update("deleted_at", nil).Error
		_ = s.db.Unscoped().Model(&models.ListenerUser{}).Where("listener_id = ?", id).Update("deleted_at", nil).Error
		return fmt.Errorf("failed to regenerate config after delete: %w", err)
	}
	return nil
}

// RegenerateConfig regenerates the complete Mihomo configuration through the
// Config Engine. Node services never write a partial listeners-only YAML file.
func (s *Service) RegenerateConfig() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("node service is not initialized")
	}
	engine := mihomoConfig.NewConfigEngine(s.db)
	yamlContent, err := engine.GenerateFinalConfig()
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	if err := os.WriteFile(s.configPath, []byte(yamlContent), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	if err := os.Chmod(s.configPath, 0600); err != nil {
		return fmt.Errorf("failed to secure config file: %w", err)
	}

	tmpl := mihomoConfig.GetDefaultTemplate()
	controllerURL := "http://" + tmpl.ExternalController
	if err := mihomo.NewExternalControllerAPI(controllerURL, tmpl.Secret).ReloadConfig(map[string]interface{}{
		"path": s.configPath,
	}); err != nil {
		log.Printf("node: mihomo hot reload skipped (core unreachable): %v", err)
	}

	return nil
}

// TriggerReload forces manual config regeneration and triggers hot reloading
func (s *Service) TriggerReload(id uint) error {
	return s.RegenerateConfig()
}
