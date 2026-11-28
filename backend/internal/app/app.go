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
	"github.com/kazeyukiro/3m-ui/backend/internal/bootstrap"
	"github.com/kazeyukiro/3m-ui/backend/internal/certstore"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	dbconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/router"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
	"gorm.io/gorm"
)

// Run boots the application and serves the embedded frontend.
func Run(frontendFS fs.FS) error {
	configPath := defaultConfigPath()
	initialized, err := bootstrap.Initialize(configPath)
	if err != nil {
		return err
	}
	cfg, db := initialized.Config, initialized.DB
	initialized.PrintCredentials(os.Stdout)
	security.InitCredentialKey(cfg.Security.CredentialKey)
	container := NewContainer(db, cfg)

	// The website-audit blocklist feature was removed (routing rules cover
	// it); drop the inert settings row so stale data cannot linger.
	if err := db.Where("key = ?", "blocklist").Delete(&models.PanelSetting{}).Error; err != nil {
		log.Printf("[WARNING] drop removed blocklist setting: %v", err)
	}

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

	// Restore listener TLS PEMs from disk (and last mihomo config.yaml) before
	// generating core config — binary-only updates must not mint new identities.
	certstore.HydrateListenersFromDisk(db)
	if cfg.Mihomo.Config != "" {
		certstore.HydrateFromMihomoYAML(db, cfg.Mihomo.Config)
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
		if _, statErr := os.Stat(container.Mihomo.BinaryPath()); statErr == nil {
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
	router.MountFrontend(r, frontendFS, config.NormalizeWebPath(cfg.Server.WebPath))
	// WebPathHandler enforces the panel base path (e.g. /secret) before the
	// Gin route tree is matched, so stripped requests route correctly.
	handler := router.WebPathHandler(config.NormalizeWebPath(cfg.Server.WebPath), cfg.Server.SubPath, r)

	sslSettings, _ := acme.LoadSettings(db)
	addr := panelListenAddr(cfg.Server.Listen, cfg.Server.Port)
	publicURL, webPath := acme.PublicURLAndWebPath()
	httpURL := acme.EffectiveEntryURL(sslSettings, publicURL, webPath, addr, cfg.Server.Port, false)
	if sslSettings.Enabled {
		// "configured" until the HTTPS listener is actually bound, so the UI
		// never claims SSL is on just because the setting was saved.
		acme.SetRuntime(acme.RuntimeState{
			State:      "configured",
			Protocol:   "https",
			ListenAddr: sslSettings.ListenTLS,
			EntryURL:   acme.EffectiveEntryURL(sslSettings, publicURL, webPath, sslSettings.ListenTLS, cfg.Server.Port, true),
			Detail:     "SSL enabled in settings; HTTPS listener not started yet (restart required after changes)",
		})
		if err := serveWithSSL(handler, sslSettings, cfg.Server.Port); err != nil {
			// Fall back to plain HTTP so the panel remains reachable when TLS
			// bind / ACME fails (port in use, missing domain, permission, …).
			log.Printf("[WARNING] panel SSL failed (%v); falling back to HTTP on :%d", err, cfg.Server.Port)
			acme.SetRuntime(acme.RuntimeState{
				State:       "fallback",
				Protocol:    "http",
				ListenAddr:  addr,
				EntryURL:    httpURL,
				Detail:      "SSL failed; panel is running on plain HTTP",
				FallbackErr: err.Error(),
			})
		} else {
			return nil
		}
	} else {
		acme.SetRuntime(acme.RuntimeState{
			State:      "off",
			Protocol:   "http",
			ListenAddr: addr,
			EntryURL:   httpURL,
			Detail:     "HTTPS disabled; panel is running on plain HTTP",
		})
	}

	if cfg.Server.SubPort > 0 && cfg.Server.SubPort != cfg.Server.Port {
		go startSubscriptionServer(db, cfg)
	}

	if wp := config.NormalizeWebPath(cfg.Server.WebPath); wp != "" {
		log.Printf("3m-ui panel base path: %s (the panel UI/API is only reachable at /<host>%s/ and the bare domain returns 404)", wp, wp)
	}
	log.Printf("3m-ui listening on %s (IPv4/IPv6)", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}

// serveWithSSL starts HTTPS (Let's Encrypt / manual cert) and the optional
// HTTP-01 challenge / redirect listener.
//
// Ordering matters (issue #106): the HTTP-01 listener must be bound and
// serving BEFORE first-time certificate issuance. Otherwise the CA fetches
// /.well-known/acme-challenge/<token> while nothing is listening on :80 and
// the panel silently falls back to plain HTTP with "Connection refused".
func serveWithSSL(handler http.Handler, s acme.Settings, fallbackPort int) error {
	mgr, err := acme.NewManager(s)
	if err != nil {
		return fmt.Errorf("panel SSL: %w", err)
	}
	acme.LogHint(s)

	tlsAddr := s.ListenTLS
	if tlsAddr == "" {
		tlsAddr = fmt.Sprintf(":%d", fallbackPort)
	}
	httpAddr := s.ListenHTTP
	if httpAddr == "" {
		httpAddr = ":80"
	}
	publicURL, webPath := acme.PublicURLAndWebPath()

	redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respect the configured TLS port (and any external PublicURL) so a
		// custom :10087 listener does not redirect browsers to :443.
		http.Redirect(w, r, httpsRedirectTarget(publicURL, s, r, tlsAddr)+r.URL.RequestURI(), http.StatusMovedPermanently)
	})

	httpLn, httpSrv, err := bindThenEnsure(mgr, httpAddr, mgr.HTTPHandler(redirect), func(m *acme.Manager, _ string) error {
		return m.EnsureCertificate()
	})
	if err != nil {
		return err
	}
	if httpLn != nil {
		// Releases :80 when TLS startup fails and Run falls back to HTTP —
		// previously this goroutine leaked the port for the process lifetime.
		defer func() {
			_ = httpSrv.Close()
			_ = httpLn.Close()
		}()
		log.Printf("3m-ui ACME/HTTP listening on %s", httpAddr)
	}

	tlsCfg, err := mgr.TLSConfig()
	if err != nil {
		return fmt.Errorf("panel SSL tls config: %w", err)
	}
	srv := &http.Server{
		Addr:              tlsAddr,
		Handler:           handler,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: 10 * time.Second,
	}
	tlsLn, err := net.Listen("tcp", tlsAddr)
	if err != nil {
		return fmt.Errorf("panel SSL: TLS listen %s: %w", tlsAddr, err)
	}

	acme.SetRuntime(acme.RuntimeState{
		State:      "running",
		Protocol:   "https",
		ListenAddr: tlsAddr,
		EntryURL:   acme.EffectiveEntryURL(s, publicURL, webPath, tlsAddr, fallbackPort, true),
		Detail:     fmt.Sprintf("HTTPS on %s; HTTP-01 / redirect on %s", tlsAddr, httpAddr),
	})
	log.Printf("3m-ui HTTPS listening on %s", tlsAddr)
	if s.CertFile != "" && s.KeyFile != "" {
		return srv.ServeTLS(tlsLn, s.CertFile, s.KeyFile)
	}
	return srv.ServeTLS(tlsLn, "", "")
}

// prepareChallengeListener binds the HTTP-01 listener before any ACME
// issuance begins and returns it for lifecycle management. A bind failure is
// fatal only when a first-time HTTP-01 issuance needs the listener; DNS-01
// and manual certs can still serve HTTPS without :80.
func prepareChallengeListener(addr string, mgr *acme.Manager, h http.Handler) (net.Listener, *http.Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if mgr != nil && mgr.NeedsHTTP01Listener() {
			return nil, nil, fmt.Errorf("panel SSL: HTTP-01 listener %s unavailable: %w (challenge must be bound before issuance)", addr, err)
		}
		log.Printf("panel SSL: HTTP listener %s unavailable (%v); continuing without redirect", addr, err)
		return nil, nil, nil
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	ready := make(chan struct{})
	go func() {
		close(ready)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("panel HTTP listener: %v", err)
		}
	}()
	<-ready
	return ln, srv, nil
}

// bindThenEnsure is the ordering contract for first-time HTTP-01 issuance:
// bind (and verify) the challenge listener, tell the IP issuer the permanent
// listeners are up, and only then run certificate issuance. Issuance can take
// minutes and must never start against an unbound :80 (issue #106).
func bindThenEnsure(mgr *acme.Manager, addr string, h http.Handler, ensure func(*acme.Manager, string) error) (net.Listener, *http.Server, error) {
	ln, srv, err := prepareChallengeListener(addr, mgr, h)
	if err != nil {
		return nil, nil, err
	}
	boundAddr := ""
	if ln != nil {
		boundAddr = ln.Addr().String()
	}
	if mgr != nil {
		mgr.MarkChallengeBound()
	}
	if err := ensure(mgr, boundAddr); err != nil {
		if ln != nil {
			_ = srv.Close()
			_ = ln.Close()
		}
		return nil, nil, err
	}
	return ln, srv, nil
}

// httpsRedirectTarget builds the redirect base. PublicURL wins (NAT /
// reverse proxy with a different external port); otherwise the host comes
// from the configured domain (or request host) and the port from ListenTLS.
func httpsRedirectTarget(publicURL string, s acme.Settings, r *http.Request, tlsAddr string) string {
	if base := strings.TrimSpace(publicURL); base != "" {
		if !strings.Contains(base, "://") {
			base = "https://" + base
		} else if strings.HasPrefix(base, "http://") {
			base = "https://" + strings.TrimPrefix(base, "http://")
		}
		return strings.TrimRight(base, "/")
	}
	host := r.Host
	if s.Domain != "" {
		host = s.Domain
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	port := ""
	if _, p, err := net.SplitHostPort(tlsAddr); err == nil {
		port = p
	}
	if port == "" || port == "443" {
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		return "https://" + host
	}
	// JoinHostPort brackets IPv6 hosts itself; do not pre-bracket here.
	return "https://" + net.JoinHostPort(host, port)
}

func defaultConfigPath() string {
	return config.ConfigPath()
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

func startSubscriptionServer(db *gorm.DB, cfg *config.Config) {
	if db == nil || cfg == nil {
		return
	}
	engine := gin.New()
	engine.Use(gin.Recovery())
	router.RegisterPublicSubscriptionRoutes(engine.Group("/api/v1"), db, cfg)
	router.RegisterCustomSubPathRoutes(engine, db, cfg)
	router.RegisterLegacySubscriptionRoutes(engine, db, cfg)
	listen := cfg.Server.SubListen
	if listen == "" {
		listen = cfg.Server.Listen
	}
	addr := panelListenAddr(listen, cfg.Server.SubPort)
	log.Printf("3m-ui subscription listener on %s (path %s)", addr, config.SubscriptionBasePath(cfg))
	srv := &http.Server{Addr: addr, Handler: engine, ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("subscription server stopped: %v", err)
	}
}
