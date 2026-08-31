package app

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/acme"
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
	if created, username, _, err := auth.EnsureAdmin(db, cfg.Database.Path); err != nil {
		return fmt.Errorf("initialize administrator: %w", err)
	} else if created {
		log.Printf("initial administrator created: username=%s", username)
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

	// Configuration generation must not block the panel. A bad custom fragment
	// or listener can be fixed from the UI once HTTP is up.
	generatedConfig, err := container.ConfigEngine.GenerateFinalConfig()
	if err != nil {
		log.Printf("[WARNING] generate Mihomo configuration failed: %v; panel will start without applying core config — check listener configs (conflicting ports, invalid custom fragments, or missing VLESS+REALITY params) and re-apply from the UI", err)
		generatedConfig = ""
	}
	if container.Mihomo == nil {
		return fmt.Errorf("initialize Mihomo service: service is nil")
	}
	if generatedConfig != "" {
		if _, statErr := os.Stat(cfg.Mihomo.Binary); statErr == nil {
			// Soft-fail: Mihomo problems must not prevent the management panel
			// from starting. Operators can inspect logs and fix listeners/config
			// from the UI, then start/restart the core explicitly.
			if err := container.Mihomo.ApplyConfig(generatedConfig); err != nil {
				// ApplyConfig already rolled back any on-disk candidate on failure.
				// Do NOT re-write the failed config — that would undo the rollback
				// and leave a broken YAML for the next restart.
				log.Printf("[WARNING] apply Mihomo configuration failed: %v; panel started without core", err)
			} else {
				log.Printf("Mihomo core started successfully")
			}
		} else {
			manager := mihomo.NewConfigManager(cfg.Mihomo.Config)
			if err := manager.SaveConfig(generatedConfig); err != nil {
				log.Printf("[WARNING] save Mihomo configuration failed: %v", err)
			} else {
				log.Printf("[WARNING] Mihomo binary unavailable at %s; panel started without core", cfg.Mihomo.Binary)
			}
		}
	}

	r := router.SetupRouterWithDeps(container.RouterDeps())
	mountFrontend(r, frontendFS)

	sslSettings, _ := acme.LoadSettings(db)
	if sslSettings.Enabled {
		if err := serveWithSSL(r, sslSettings, cfg.Server.Port); err != nil {
			// Fall back to plain HTTP so the panel remains reachable when TLS
			// bind / ACME fails (port in use, missing domain, permission, …).
			log.Printf("[WARNING] panel SSL failed (%v); falling back to HTTP on :%d", err, cfg.Server.Port)
		} else {
			return nil
		}
	}

	addr := panelListenAddr(cfg.Server.Listen, cfg.Server.Port)
	log.Printf("3m-ui listening on %s (IPv4/IPv6)", addr)
	if err := r.Run(addr); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}

// serveWithSSL starts HTTPS (Let's Encrypt autocert or manual cert) and optional
// HTTP-01 challenge / redirect listener — panel SSL with HTTP-01.
func serveWithSSL(handler http.Handler, s acme.Settings, fallbackPort int) error {
	mgr, err := acme.NewManager(s)
	if err != nil {
		return fmt.Errorf("panel SSL: %w", err)
	}
	acme.LogHint(s)
	tlsCfg, err := mgr.TLSConfig()
	if err != nil {
		return fmt.Errorf("panel SSL tls config: %w", err)
	}
	tlsAddr := s.ListenTLS
	if tlsAddr == "" {
		tlsAddr = fmt.Sprintf(":%d", fallbackPort)
	}
	srv := &http.Server{
		Addr:              tlsAddr,
		Handler:           handler,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// HTTP-01 challenge + optional redirect to HTTPS.
	//
	// KNOWN LIMITATION (R2-2.1): this goroutine is intentionally NOT tracked by
	// the caller. http.ListenAndServe blocks until the listener errors out, and
	// serveWithSSL does not wait on it. When serveWithSSL returns (e.g. because
	// ListenAndServeTLS failed and the caller falls back to plain HTTP in Run),
	// this goroutine keeps holding :80 until the process exits or the listener
	// breaks. Wiring it into a context with cancellation + a tracked WaitGroup
	// is a larger refactor tracked separately; for now this comment documents
	// the leak so it is not silently reintroduced.
	httpAddr := s.ListenHTTP
	if httpAddr == "" {
		httpAddr = ":80"
	}
	go func() {
		redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if s.Domain != "" {
				host = s.Domain
			}
			http.Redirect(w, r, "https://"+host+r.URL.RequestURI(), http.StatusMovedPermanently)
		})
		h := mgr.HTTPHandler(redirect)
		log.Printf("3m-ui ACME/HTTP listening on %s", httpAddr)
		if err := http.ListenAndServe(httpAddr, h); err != nil {
			log.Printf("panel HTTP listener: %v", err)
		}
	}()
	log.Printf("3m-ui HTTPS listening on %s", tlsAddr)
	if s.CertFile != "" && s.KeyFile != "" {
		return srv.ListenAndServeTLS(s.CertFile, s.KeyFile)
	}
	// autocert: certificates obtained on first request for the configured domain.
	return srv.ListenAndServeTLS("", "")
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
		if strings.HasPrefix(path, "/api") {
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

// panelListenAddr builds a net listen address supporting dual-stack and IPv6.
// Empty / wildcard listen → ":port" (Go dual-stack on Linux).
// Concrete IPs use JoinHostPort so IPv6 is correctly bracketed.
func panelListenAddr(listen string, port int) string {
	listen = strings.TrimSpace(listen)
	if listen == "" || listen == "0.0.0.0" || listen == "::" || listen == "*" {
		return fmt.Sprintf(":%d", port)
	}
	return net.JoinHostPort(listen, strconv.Itoa(port))
}
