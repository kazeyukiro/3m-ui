package database

import (
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: sqliteDriverName,
		DSN:        dbPath,
	}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sqlite database: %w", err)
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.Listener{},
		&models.ListenerUser{},
		&models.Subscription{},
		&models.AccessToken{},
		&models.Config{},
		&models.ProxyUser{},
		&models.TrafficRecord{},
		&models.PanelSetting{},
		&models.RemoteServer{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run database auto-migration: %w", err)
	}

	GlobalDB = db
	return db, nil
}
