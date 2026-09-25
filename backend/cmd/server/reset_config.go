package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/acme"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// resetConfigFlags selects which configuration layers to reset.
type resetConfigFlags struct {
	panel  bool // server.listen / server.port + SSL/ACME settings
	access bool // listener.public_host / access_sni / access_alpn (per-listener overrides)
	public bool // server.public_url (subscription / share host)
	all    bool // = panel + access + public
	yes    bool // skip interactive confirmation (for rescue scripts)
}

func parseResetConfigArgs(args []string) resetConfigFlags {
	var f resetConfigFlags
	for _, a := range args {
		switch a {
		case "--panel":
			f.panel = true
		case "--access":
			f.access = true
		case "--public":
			f.public = true
		case "--all":
			f.all = true
		case "--yes", "-y":
			f.yes = true
		case "--help", "-h":
			printResetConfigHelp()
			os.Exit(0)
		}
	}
	if f.all {
		f.panel = true
		f.access = true
		f.public = true
	}
	// Default scope when nothing specified: panel only (the most common
	// lockout scenario — port/listen/SSL misconfiguration).
	if !f.panel && !f.access && !f.public && !f.all {
		f.panel = true
	}
	return f
}

func printResetConfigHelp() {
	fmt.Println(`3m-ui reset-config — undo panel configuration changes via SSH

Resets configuration layers to safe defaults so you can regain web access
after a misconfiguration (wrong port, bound to 127.0.0.1, broken SSL, etc.).
The panel process must be stopped before running this command.

USAGE
    3m-ui reset-config [--panel] [--access] [--public] [--all] [--yes]

SCOPE FLAGS (default: --panel if none given)
    --panel    Reset server.listen / server.port + ACME/SSL settings
               → listen defaults to "" (= ":<port>"), port preserved,
                 SSL disabled so the panel starts plain HTTP on next boot.
    --access   Clear per-listener public_host / access_sni / access_alpn
               → nodes fall back to their bind address (still routable
                 if the bind address is publicly reachable).
    --public   Clear server.public_url (subscription / share host)
               → subscription links fall back to the request host.
    --all      Apply all three scopes above.

CONFIRMATION
    Interactive y/N prompt unless --yes / -y is passed.
    Recommended for rescue scripts: sudo 3m-ui reset-config --panel --yes

AFTER RESET
    Restart the panel:  systemctl restart 3m-ui
    Then browse to http://<server-ip>:<port>  (HTTP, not HTTPS)
    and re-apply your SSL / public_url / access_profile settings from the UI.

This command only modifies config.yaml + the panel_settings DB row +
the listeners table's public_host/access_sni/access_alpn columns.
It never deletes users, listeners, traffic records, or credentials.`)
}

func runResetConfig(configPath string, args []string) error {
	flags := parseResetConfigArgs(args)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Show what will be reset before asking for confirmation.
	fmt.Printf("Config:   %s\n", configPath)
	fmt.Printf("DB:       %s\n", cfg.Database.Path)
	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}
	fmt.Println()
	fmt.Println("Will reset:")
	if flags.panel {
		fmt.Printf("  [panel]  server.listen=%q → \"\" (binds :%d)\n", cfg.Server.Listen, port)
		fmt.Printf("           server.port=%d → preserved\n", port)
		fmt.Println("           ACME/SSL settings → disabled (panel boots plain HTTP)")
	}
	if flags.access {
		fmt.Println("  [access] listener.public_host / access_sni / access_alpn → cleared on ALL listeners")
	}
	if flags.public {
		fmt.Printf("  [public] server.public_url=%q → \"\" (subscription falls back to request host)\n", cfg.Server.PublicURL)
	}
	fmt.Println()

	if !flags.yes {
		fmt.Print("Proceed? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted. No changes made.")
			return nil
		}
	}

	// --- panel scope: rewrite config.yaml ---
	if flags.panel {
		// Clear server.listen so the panel binds to ":port" (dual-stack),
		// preserving the port the operator chose. config.UpdateServerFile
		// skips empty listen, so we delete the key directly.
		if err := config.ClearListenInFile(configPath); err != nil {
			return fmt.Errorf("clear server.listen: %w", err)
		}
		fmt.Println("✓ config.yaml: server.listen cleared (port preserved)")
	}

	// Open the DB for the ACME + listeners edits.
	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	// --- panel scope: disable ACME/SSL so panel boots plain HTTP ---
	if flags.panel {
		ssl, err := acme.LoadSettings(db)
		if err != nil {
			return fmt.Errorf("load SSL settings: %w", err)
		}
		if ssl.Enabled {
			ssl.Enabled = false
			// Keep Domain / CertFile / KeyFile so the operator can re-enable
			// from the UI after fixing the underlying issue. Only flip the
			// Enabled flag — that's the single switch the panel server reads
			// at boot to decide HTTP vs HTTPS.
			if err := acme.SaveSettings(db, ssl); err != nil {
				return fmt.Errorf("disable SSL: %w", err)
			}
			fmt.Println("✓ panel SSL disabled (re-enable from UI after fix)")
		} else {
			fmt.Println("· panel SSL already disabled — skipping")
		}
	}

	// --- access scope: clear per-listener overrides ---
	if flags.access {
		result := db.Model(&models.Listener{}).
			Where("public_host <> '' OR access_sni <> '' OR access_alpn <> ''").
			Updates(map[string]interface{}{
				"public_host": "",
				"access_sni":  "",
				"access_alpn": "",
			})
		if result.Error != nil {
			return fmt.Errorf("clear listener access overrides: %w", result.Error)
		}
		if result.RowsAffected > 0 {
			fmt.Printf("✓ cleared access_profile on %d listener(s)\n", result.RowsAffected)
		} else {
			fmt.Println("· no listener access_profile overrides to clear")
		}
	}

	// --- public scope: clear server.public_url ---
	if flags.public {
		if err := config.ClearPublicURLInFile(configPath); err != nil {
			return fmt.Errorf("clear public_url: %w", err)
		}
		if cfg.Server.PublicURL != "" {
			fmt.Println("✓ server.public_url cleared")
		} else {
			fmt.Println("· server.public_url already empty — skipping")
		}
	}

	fmt.Println()
	fmt.Println("Reset complete. Restart the panel:")
	fmt.Printf("    systemctl restart 3m-ui   (or your process manager)\n")
	fmt.Printf("Then open http://<server-ip>:%d  (HTTP, not HTTPS)\n", port)
	fmt.Println("and re-apply your settings from the web UI.")
	return nil
}
