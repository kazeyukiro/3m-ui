package database

import (
	"net/url"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenReadOnly is for local maintenance commands. It must never create a
// database, migrate its schema, or modify the running application's settings.
func OpenReadOnly(path string) (*gorm.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro"}
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: sqliteDriverName, DSN: pragmaDSN(uri.String())}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	// Maintenance commands run a handful of queries; one warm connection is
	// enough and keeps the memory these commands add negligible.
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(2)
		sqlDB.SetMaxIdleConns(1)
	}
	return db, nil
}
