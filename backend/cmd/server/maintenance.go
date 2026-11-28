package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/acme"
	"github.com/kazeyukiro/3m-ui/backend/internal/certstore"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
)

func readSSLSettings(cfg *config.Config) (acme.Settings, error) {
	db, err := database.OpenReadOnly(cfg.Database.Path)
	if err != nil {
		return acme.Settings{}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return acme.Settings{}, err
	}
	defer sqlDB.Close()
	return acme.LoadSettings(db)
}

func probeAddress(listen string, defaultPort int) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		host, port = listen, strconv.Itoa(defaultPort)
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	} else if host == "::" {
		host = "::1"
	}
	return net.JoinHostPort(host, port)
}

func runHealthcheck(path string) error {
	cfg, err := config.LoadConfig(path)
	if err != nil {
		return err
	}
	ssl, err := readSSLSettings(cfg)
	if err != nil {
		return fmt.Errorf("read panel settings: %w", err)
	}
	addresses := []string{"http://" + probeAddress(cfg.Server.Listen, cfg.Server.Port)}
	if ssl.Enabled {
		addresses = append([]string{"https://" + probeAddress(ssl.ListenTLS, cfg.Server.Port)}, addresses...)
	}
	client := &http.Client{
		Timeout: 2 * time.Second,
		// This local readiness probe validates the application's response, not
		// public certificate trust. ACME certificates usually name a domain,
		// while the connection goes directly to the configured local listener.
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: ssl.Domain}}, // #nosec G402
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	for _, address := range addresses {
		response, err := client.Get(address + "/api/v1/health")
		if err != nil {
			continue
		}
		var body struct {
			Status string `json:"status"`
		}
		err = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&body)
		response.Body.Close()
		if response.StatusCode == http.StatusOK && err == nil && body.Status == "ok" {
			return nil
		}
	}
	return fmt.Errorf("panel health endpoint did not respond successfully")
}

// printStoragePaths reports paths only, without reading out any credentials.
// Maintenance scripts use the candidate binary before changing the install,
// so even an older installation can be inventoried without a DB migration.
func printStoragePaths(path string, out io.Writer) error {
	cfg, err := config.LoadConfig(path)
	if err != nil {
		return err
	}
	ssl, err := readSSLSettings(cfg)
	if err != nil {
		return err
	}
	paths := []string{path, cfg.Database.Path, cfg.Database.Path + "-wal", cfg.Database.Path + "-shm", certstore.Dir()}
	if cfg.Mihomo.Config != "" {
		paths = append(paths, cfg.Mihomo.Config, filepath.Dir(cfg.Mihomo.Config))
	}
	paths = append(paths, ssl.CacheDir, ssl.CertFile, ssl.KeyFile)
	return printResolvedStoragePaths(paths, out)
}

func printResolvedStoragePaths(paths []string, out io.Writer) error {
	seen := make(map[string]bool)
	for index := 0; index < len(paths); index++ {
		item := paths[index]
		if item == "" {
			continue
		}
		abs, err := filepath.Abs(item)
		if err != nil {
			return err
		}
		// Include the target of e.g. Let's Encrypt live certificate symlinks.
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, name := range []string{abs, resolved} {
			if name == "" || seen[name] {
				continue
			}
			if strings.ContainsAny(name, "\r\n") {
				return fmt.Errorf("storage path contains a newline")
			}
			seen[name] = true
			if _, err := fmt.Fprintln(out, name); err != nil {
				return err
			}
		}
		// tar preserves symlinks, so inventory links below certificate/core
		// directories as well as the configured paths themselves. Do not follow
		// nested directory links: their contents cannot be assumed to be part of
		// the snapshot, even when the directory target is under a managed root.
		if resolved == "" {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			continue
		}
		if err := filepath.WalkDir(resolved, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink == 0 {
				return nil
			}
			target, err := os.Stat(name)
			if err != nil {
				return fmt.Errorf("inspect storage symlink %s: %w", name, err)
			}
			if target.IsDir() {
				return fmt.Errorf("automatic snapshots do not support nested directory symlinks: %s", name)
			}
			paths = append(paths, name)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
