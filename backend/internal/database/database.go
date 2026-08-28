package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var GlobalDB *gorm.DB

func InitDB(dbPath string) (*gorm.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}
	// The database contains password hashes, proxy credentials and subscription
	// tokens. Do not leave it readable by other local users.
	_ = os.Chmod(dir, 0700)

	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: sqliteDriverName, DSN: dbPath}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sqlite database: %w", err)
	}
	if err := os.Chmod(dbPath, 0600); err != nil {
		return nil, fmt.Errorf("failed to secure database file: %w", err)
	}

	// Resolve collisions before unique indexes are applied by AutoMigrate.
	if err := dedupeListenerNames(db); err != nil {
		return nil, fmt.Errorf("dedupe listener names: %w", err)
	}
	if err := fillEmptySubTokens(db); err != nil {
		return nil, fmt.Errorf("fill empty sub tokens: %w", err)
	}

	err = db.AutoMigrate(
		&models.User{}, &models.Listener{}, &models.ListenerUser{}, &models.ListenerVersion{},
		&models.ListenerTemplate{}, &models.Subscription{}, &models.AccessToken{}, &models.Config{},
		&models.ProxyUser{}, &models.TrafficRecord{}, &models.PanelSetting{}, &models.RemoteServer{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run database auto-migration: %w", err)
	}

	GlobalDB = db
	return db, nil
}

// dedupeListenerNames renames later duplicates so AutoMigrate can add uniqueIndex on name.
func dedupeListenerNames(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.Listener{}) {
		return nil
	}
	var rows []models.Listener
	if err := db.Order("id asc").Find(&rows).Error; err != nil {
		return err
	}
	seen := map[string]uint{}
	for _, row := range rows {
		name := row.Name
		if name == "" {
			name = fmt.Sprintf("listener-%d", row.ID)
			if err := db.Model(&models.Listener{}).Where("id = ?", row.ID).Update("name", name).Error; err != nil {
				return err
			}
			seen[name] = row.ID
			continue
		}
		if prev, ok := seen[name]; ok && prev != row.ID {
			newName := fmt.Sprintf("%s-%d", name, row.ID)
			if err := db.Model(&models.Listener{}).Where("id = ?", row.ID).Update("name", newName).Error; err != nil {
				return err
			}
			seen[newName] = row.ID
			continue
		}
		seen[name] = row.ID
	}
	return nil
}

// fillEmptySubTokens assigns unique tokens so uniqueIndex on sub_token can be created.
func fillEmptySubTokens(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.ProxyUser{}) {
		return nil
	}
	var rows []models.ProxyUser
	if err := db.Where("sub_token = ? OR sub_token IS NULL", "").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		tok, err := randomHexToken(16)
		if err != nil {
			return err
		}
		if err := db.Model(&models.ProxyUser{}).Where("id = ?", row.ID).Update("sub_token", tok).Error; err != nil {
			return err
		}
	}
	return nil
}

func randomHexToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
