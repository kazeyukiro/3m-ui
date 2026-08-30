package listener

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	var previous models.Listener
	if err := s.db.First(&previous, listenerID).Error; err != nil {
		return err
	}
	if err := s.db.Save(&target).Error; err != nil {
		return err
	}
	if err := s.regenerateConfigLocked(); err != nil {
		if rollbackErr := s.db.Save(&previous).Error; rollbackErr != nil {
			return fmt.Errorf("%v; rollback listener failed: %w", err, rollbackErr)
		}
		if regenerateErr := s.regenerateConfigLocked(); regenerateErr != nil {
			return fmt.Errorf("%v; restored listener but failed to regenerate previous configuration: %w", err, regenerateErr)
		}
		return err
	}
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
	if err := s.regenerateConfigLocked(); err != nil {
		if rollbackErr := s.db.Delete(&src).Error; rollbackErr != nil {
			return nil, fmt.Errorf("%v; rollback cloned listener failed: %w", err, rollbackErr)
		}
		return nil, err
	}
	if err := s.SaveVersion(src.ID, "clone"); err != nil {
		if rollbackErr := s.db.Delete(&src).Error; rollbackErr != nil {
			return nil, fmt.Errorf("save cloned listener history: %v; rollback failed: %w", err, rollbackErr)
		}
		if regenerateErr := s.regenerateConfigLocked(); regenerateErr != nil {
			return nil, fmt.Errorf("save cloned listener history: %v; listener rolled back but previous configuration regeneration failed: %w", err, regenerateErr)
		}
		return nil, fmt.Errorf("save cloned listener history: %w", err)
	}
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
		if err := ValidateModel(&list[i]); err != nil {
			tx.Rollback()
			return nil, err
		}
		var n int64
		if err := tx.Model(&models.Listener{}).Where("name = ?", list[i].Name).Count(&n).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if n > 0 {
			tx.Rollback()
			return nil, fmt.Errorf("listener name %q already exists", list[i].Name)
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
	if err := s.regenerateConfigLocked(); err != nil {
		for _, l := range created {
			if rollbackErr := s.db.Delete(&l).Error; rollbackErr != nil {
				return nil, fmt.Errorf("%v; rollback batch listener %d failed: %w", err, l.ID, rollbackErr)
			}
		}
		if regenerateErr := s.regenerateConfigLocked(); regenerateErr != nil {
			return nil, fmt.Errorf("%v; batch rollback completed but previous configuration regeneration failed: %w", err, regenerateErr)
		}
		return nil, err
	}
	for _, l := range created {
		if versionErr := s.SaveVersion(l.ID, "batch-create"); versionErr != nil {
			for _, createdListener := range created {
				if rollbackErr := s.db.Delete(&createdListener).Error; rollbackErr != nil {
					return nil, fmt.Errorf("save batch listener history: %v; rollback listener %d failed: %w", versionErr, createdListener.ID, rollbackErr)
				}
			}
			if regenerateErr := s.regenerateConfigLocked(); regenerateErr != nil {
				return nil, fmt.Errorf("save batch listener history: %v; batch rolled back but previous configuration regeneration failed: %w", versionErr, regenerateErr)
			}
			return nil, fmt.Errorf("save batch listener history: %w", versionErr)
		}
	}
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
	if err := s.regenerateConfigLocked(); err != nil {
		for i := range previous {
			if rollbackErr := s.db.Save(&previous[i]).Error; rollbackErr != nil {
				return fmt.Errorf("%v; rollback listener %d failed: %w", err, previous[i].ID, rollbackErr)
			}
		}
		if regenerateErr := s.regenerateConfigLocked(); regenerateErr != nil {
			return fmt.Errorf("%v; listener state restored but previous configuration regeneration failed: %w", err, regenerateErr)
		}
		return err
	}
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
	return s.db.Create(t).Error
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
	return s.db.Delete(&models.ListenerTemplate{}, id).Error
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
	if err := s.regenerateConfigLocked(); err != nil {
		if rollbackErr := s.db.Delete(l).Error; rollbackErr != nil {
			return nil, fmt.Errorf("%v; rollback instantiated listener failed: %w", err, rollbackErr)
		}
		return nil, err
	}
	if err := s.SaveVersion(l.ID, "template"); err != nil {
		if rollbackErr := s.db.Delete(l).Error; rollbackErr != nil {
			return nil, fmt.Errorf("save template listener history: %v; rollback failed: %w", err, rollbackErr)
		}
		if regenerateErr := s.regenerateConfigLocked(); regenerateErr != nil {
			return nil, fmt.Errorf("save template listener history: %v; listener rolled back but previous configuration regeneration failed: %w", err, regenerateErr)
		}
		return nil, fmt.Errorf("save template listener history: %w", err)
	}
	return l, nil
}
