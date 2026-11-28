// Package bootstrap owns the shared first-run initialization for native and
// container installations. It never replaces an existing administrator.
package bootstrap

import (
	"fmt"
	"io"

	"github.com/kazeyukiro/3m-ui/backend/internal/auth"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"gorm.io/gorm"
)

type Result struct {
	Config   *config.Config
	DB       *gorm.DB
	Created  bool
	Username string
	Password string
}

func Initialize(path string) (*Result, error) {
	if err := config.EnsureConfig(path); err != nil {
		return nil, fmt.Errorf("initialize configuration: %w", err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	created, username, password, err := auth.EnsureAdmin(db, cfg.Database.Path)
	if err != nil {
		if sqlDB, closeErr := db.DB(); closeErr == nil {
			sqlDB.Close()
		}
		return nil, fmt.Errorf("initialize administrator: %w", err)
	}
	return &Result{Config: cfg, DB: db, Created: created, Username: username, Password: password}, nil
}

// PrintCredentials is deliberately limited to creation. Re-running init or
// restarting a container cannot reveal or reset an existing user's password.
func (r *Result) PrintCredentials(w io.Writer) {
	if r.Created {
		fmt.Fprintf(w, "Initial administrator: %s\nInitial password: %s\nChange this password on first login. It will not be displayed again.\n", r.Username, r.Password)
	}
}
