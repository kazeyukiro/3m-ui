package listener

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	dbconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gorm.io/gorm"
)

type Service struct {
	db          *gorm.DB
	configPath  string
	mihomoApply interface{ ApplyConfig(string) error }
}

func NewService(db *gorm.DB, configPath string, mihomoApply interface{ ApplyConfig(string) error }) *Service {
	return &Service{db: db, configPath: configPath, mihomoApply: mihomoApply}
}

func (s *Service) Create(l *models.Listener) error {
	if err := ValidateModel(l); err != nil { return err }
	if err := s.db.Create(l).Error; err != nil { return fmt.Errorf("failed to create listener: %w", err) }
	if err := s.RegenerateConfig(); err != nil { _ = s.db.Delete(&models.Listener{}, l.ID).Error; return err }
	return nil
}

func (s *Service) GetAll() ([]models.Listener, error) {
	var list []models.Listener
	if err := s.db.Order("id desc").Find(&list).Error; err != nil { return nil, fmt.Errorf("failed to fetch listeners: %w", err) }
	return list, nil
}

func (s *Service) GetByID(id uint) (*models.Listener, error) {
	var l models.Listener
	if err := s.db.First(&l, id).Error; err != nil { return nil, fmt.Errorf("failed to fetch listener by id %d: %w", id, err) }
	return &l, nil
}

func (s *Service) Update(l *models.Listener) error {
	if err := ValidateModel(l); err != nil { return err }
	var previous models.Listener
	if err := s.db.First(&previous, l.ID).Error; err != nil { return fmt.Errorf("failed to load previous listener: %w", err) }
	if err := s.db.Save(l).Error; err != nil { return fmt.Errorf("failed to update listener: %w", err) }
	if err := s.RegenerateConfig(); err != nil {
		if rollbackErr := s.db.Save(&previous).Error; rollbackErr != nil { return fmt.Errorf("%v; rollback listener failed: %w", err, rollbackErr) }
		_ = s.RegenerateConfig()
		return err
	}
	return nil
}

func (s *Service) Delete(id uint) error {
	var previous models.Listener
	if err := s.db.First(&previous, id).Error; err != nil { return fmt.Errorf("failed to fetch listener before delete: %w", err) }
	if err := s.db.Delete(&models.Listener{}, id).Error; err != nil { return fmt.Errorf("failed to delete listener: %w", err) }
	if err := s.RegenerateConfig(); err != nil {
		if rollbackErr := s.db.Unscoped().Save(&previous).Error; rollbackErr != nil { return fmt.Errorf("%v; rollback deleted listener failed: %w", err, rollbackErr) }
		_ = s.RegenerateConfig()
		return err
	}
	return nil
}

func (s *Service) RegenerateConfig() error {
	if s == nil || s.db == nil { return fmt.Errorf("listener service not initialized") }
	engine := dbconfig.NewConfigEngine(s.db)
	yamlContent, err := engine.GenerateFinalConfig()
	if err != nil { return fmt.Errorf("generate Mihomo configuration: %w", err) }
	if s.mihomoApply != nil { return s.mihomoApply.ApplyConfig(yamlContent) }
	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0750); err != nil { return fmt.Errorf("create config directory: %w", err) }
	tmp, err := os.CreateTemp(dir, ".config.yaml.tmp-*")
	if err != nil { return fmt.Errorf("create temporary config: %w", err) }
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil { _ = tmp.Close(); return err }
	if _, err := tmp.WriteString(yamlContent); err != nil { _ = tmp.Close(); return err }
	if err := tmp.Sync(); err != nil { _ = tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	if err := os.Rename(tmpName, s.configPath); err != nil { return fmt.Errorf("replace Mihomo config: %w", err) }
	return nil
}

func (s *Service) TriggerReload(_ uint) error { return s.RegenerateConfig() }
