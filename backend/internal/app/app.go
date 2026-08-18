package app

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/auth"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	dbconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/router"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
)

// Run boots the application and serves the embedded frontend.
func Run(frontendFS fs.FS) error {
	configPath := defaultConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	db, err := database.InitDB(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	if created, username, password, err := auth.EnsureAdmin(db, cfg.Database.Path); err != nil {
		return fmt.Errorf("initialize administrator: %w", err)
	} else if created {
		log.Printf("initial administrator created: username=%s password=%s (change it immediately)", username, password)
	}
	security.InitCredentialKey(cfg.Security.CredentialKey)
	container := NewContainer(db, cfg)

	dbconfig.CredentialProvider = func() (map[uint][]dbconfig.Credential, error) {
		if container.User == nil {
			return map[uint][]dbconfig.Credential{}, nil
		}
		provided, err := container.User.ActiveCredentialsByListener()
		if err != nil {
			return nil, err
		}
		result := make(map[uint][]dbconfig.Credential, len(provided))

		var bindings []models.ListenerUser
		if err := db.Where("deleted_at IS NULL").Find(&bindings).Error; err != nil {
			return nil, err
		}
		for _, binding := range bindings {
			result[binding.ListenerID] = []dbconfig.Credential{}
		}

		for listenerID, credentials := range provided {
			converted := make([]dbconfig.Credential, 0, len(credentials))
			for _, credential := range credentials {
				converted = append(converted, dbconfig.Credential{Username: credential.Username, Password: credential.Password, UUID: credential.UUID})
			}
			result[listenerID] = converted
		}
		return result, nil
	}

	generatedConfig, err := container.ConfigEngine.GenerateFinalConfig()
	if err != nil {
		return fmt.Errorf("generate Mihomo configuration: %w", err)
	}
	if container.Mihomo == nil {
		return fmt.Errorf("initialize Mihomo service: service is nil")
	}
	if _, statErr := os.Stat(cfg.Mihomo.Binary); statErr == nil {
		if err := container.Mihomo.SaveConfig(generatedConfig); err != nil {
			return fmt.Errorf("validate Mihomo configuration: %w", err)
		}
		if err := container.Mihomo.StartMihomo(); err != nil {
			return fmt.Errorf("start Mihomo core: %w", err)
		}
		log.Printf("Mihomo core started successfully")
	} else {
		manager := mihomo.NewConfigManager(cfg.Mihomo.Config)
		if err := manager.SaveConfig(generatedConfig); err != nil {
			return fmt.Errorf("save Mihomo configuration: %w", err)
		}
		log.Printf("warning: Mihomo binary unavailable at %s; panel started without core", cfg.Mihomo.Binary)
	}

	r := router.SetupRouterWithDeps(container.RouterDeps())
	mountFrontend(r, frontendFS)
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("3m-ui listening on %s", addr)
	if err := r.Run(addr); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}

func defaultConfigPath() string {
	if value := os.Getenv("THREE_M_UI_CONFIG"); value != "" {
		return value
	}
	for _, candidate := range []string{"/etc/3m-ui/config.yaml", "config/config.yaml", "backend/config/config.yaml"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "config/config.yaml"
}

func mountFrontend(r *gin.Engine, frontendFS fs.FS) {
	staticFS, err := fs.Sub(frontendFS, "web/dist")
	if err != nil {
		log.Printf("frontend assets unavailable: %v", err)
		return
	}
	fileServer := http.FileServer(http.FS(staticFS))
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) >= 4 && path[:4] == "/api" {
			c.Status(http.StatusNotFound)
			return
		}
		if path == "/" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", mustReadFile(staticFS, "index.html"))
			return
		}
		f, err := staticFS.Open(path[1:])
		if err == nil {
			defer f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", mustReadFile(staticFS, "index.html"))
	})
}

func mustReadFile(fsys fs.FS, name string) []byte {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		log.Printf("read frontend %s failed: %v", name, err)
		return []byte("3m-ui frontend unavailable")
	}
	return data
}
