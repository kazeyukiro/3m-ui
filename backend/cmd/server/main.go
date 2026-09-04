package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kazeyukiro/3m-ui/backend/internal/app"
	"github.com/kazeyukiro/3m-ui/backend/internal/auth"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
)

// main accepts both the installer-provided THREE_M_UI_CONFIG environment
// variable and the documented --config/-c command-line option.
func main() {
	args := os.Args[1:]
	configPath := os.Getenv("THREE_M_UI_CONFIG")
	var cmd string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "version":
			println(versionString())
			return
		case "--config", "-c":
			if i+1 >= len(args) || args[i+1] == "" {
				log.Fatal("--config requires a path")
			}
			configPath = args[i+1]
			if err := os.Setenv("THREE_M_UI_CONFIG", configPath); err != nil {
				log.Fatalf("set config path: %v", err)
			}
			i++
		case "reset-admin":
			cmd = "reset-admin"
		default:
			if cmd == "" && !hasPrefixDash(args[i]) {
				cmd = args[i]
			} else if hasPrefixDash(args[i]) {
				log.Fatalf("unknown argument: %s", args[i])
			}
		}
	}

	if cmd == "reset-admin" {
		if err := runResetAdmin(configPath); err != nil {
			log.Fatal(err)
		}
		return
	}
	if cmd != "" {
		log.Fatalf("unknown command: %s", cmd)
	}

	if err := app.Run(frontendFiles); err != nil {
		log.Fatal(err)
	}
}

func hasPrefixDash(s string) bool {
	return len(s) > 0 && s[0] == '-'
}

func runResetAdmin(configPath string) error {
	if configPath == "" {
		configPath = "/etc/3m-ui/config.yaml"
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = filepath.Join("/var/lib/3m-ui", "3m-ui.db")
	}
	db, err := database.InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	// Keep initial default password semantics: admin / admin
	if err := auth.ResetAdminPassword(db, "admin"); err != nil {
		return err
	}
	fmt.Println("Administrator password reset to: admin")
	fmt.Println("You must change the password on next login.")
	return nil
}
