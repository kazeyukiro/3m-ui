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
	// Best-effort: file may not exist yet on brand-new installs until first write.
	if _, statErr := os.Stat(dbPath); statErr == nil {
		_ = os.Chmod(dbPath, 0600)
	}

	// Resolve collisions (including soft-deleted rows) before unique indexes.
	if err := dedupeListenerNames(db); err != nil {
		return nil, fmt.Errorf("dedupe listener names: %w", err)
	}
	if err := ensureUniqueSubTokens(db); err != nil {
		return nil, fmt.Errorf("ensure unique sub tokens: %w", err)
	}

	err = db.AutoMigrate(
		&models.User{}, &models.Listener{}, &models.ListenerUser{}, &models.ListenerVersion{},
		&models.ListenerTemplate{}, &models.Subscription{}, &models.AccessToken{}, &models.Config{},
		&models.ProxyUser{}, &models.TrafficRecord{}, &models.PanelSetting{}, &models.RemoteServer{},
	)
	if err != nil {
		// One more cleanup pass then retry — soft-deleted collisions are the usual cause.
		_ = dedupeListenerNames(db)
		_ = ensureUniqueSubTokens(db)
		if retryErr := db.AutoMigrate(
			&models.User{}, &models.Listener{}, &models.ListenerUser{}, &models.ListenerVersion{},
			&models.ListenerTemplate{}, &models.Subscription{}, &models.AccessToken{}, &models.Config{},
			&models.ProxyUser{}, &models.TrafficRecord{}, &models.PanelSetting{}, &models.RemoteServer{},
		); retryErr != nil {
			return nil, fmt.Errorf("failed to run database auto-migration: %w (after dedupe retry)", retryErr)
		}
	}

	GlobalDB = db
	return db, nil
}

// dedupeListenerNames renames later duplicates (including soft-deleted) so
// AutoMigrate can create uniqueIndex on name without failing panel boot.
func dedupeListenerNames(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.Listener{}) {
		return nil
	}
	var rows []models.Listener
	// Must include soft-deleted rows: UNIQUE indexes cover the whole table.
	if err := db.Unscoped().Order("id asc").Find(&rows).Error; err != nil {
		return err
	}
	seen := map[string]uint{}
	for _, row := range rows {
		name := row.Name
		if name == "" {
			name = fmt.Sprintf("listener-%d", row.ID)
			if err := db.Unscoped().Model(&models.Listener{}).Where("id = ?", row.ID).Update("name", name).Error; err != nil {
				return err
			}
			seen[name] = row.ID
			continue
		}
		if prev, ok := seen[name]; ok && prev != row.ID {
			newName := fmt.Sprintf("%s-%d", name, row.ID)
			if err := db.Unscoped().Model(&models.Listener{}).Where("id = ?", row.ID).Update("name", newName).Error; err != nil {
				return err
			}
			seen[newName] = row.ID
			continue
		}
		seen[name] = row.ID
	}
	return nil
}

// ensureUniqueSubTokens fills empty tokens and renames duplicate tokens across
// all rows (including soft-deleted) so uniqueIndex on sub_token can be created.
func ensureUniqueSubTokens(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.ProxyUser{}) {
		return nil
	}
	var rows []models.ProxyUser
	if err := db.Unscoped().Order("id asc").Find(&rows).Error; err != nil {
		return err
	}
	seen := map[string]uint{}
	for _, row := range rows {
		tok := row.SubToken
		needNew := tok == ""
		if !needNew {
			if prev, ok := seen[tok]; ok && prev != row.ID {
				needNew = true
			}
		}
		if needNew {
			var err error
			for attempt := 0; attempt < 8; attempt++ {
				tok, err = randomHexToken(16)
				if err != nil {
					return err
				}
				if _, clash := seen[tok]; !clash {
					break
				}
			}
			if err := db.Unscoped().Model(&models.ProxyUser{}).Where("id = ?", row.ID).Update("sub_token", tok).Error; err != nil {
				return err
			}
		}
		seen[tok] = row.ID
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
