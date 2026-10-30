package router

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/acme"
	"github.com/kazeyukiro/3m-ui/backend/internal/auth"
	"github.com/kazeyukiro/3m-ui/backend/internal/buildinfo"
	"github.com/kazeyukiro/3m-ui/backend/internal/cluster"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/docs"
	"github.com/kazeyukiro/3m-ui/backend/internal/listener"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
	"github.com/kazeyukiro/3m-ui/backend/internal/subpage"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"github.com/kazeyukiro/3m-ui/backend/internal/telegram"
	"github.com/kazeyukiro/3m-ui/backend/internal/traffic"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

// WebPathHandler wraps an http.Handler so the panel is only reachable under
// webPath (e.g. /secret). Requests carrying the prefix are stripped to the bare
// path BEFORE the inner handler's route tree is matched — unlike a Gin middleware,
// which runs after matching and therefore cannot re-route — the bare prefix
// redirects to a trailing-slash URL, and everything else (except public health
// probes, client subscriptions, and the configured sub_path) returns 404 so the
// entry point is not discoverable by scanning the bare domain.
func WebPathHandler(webPath, subPath string, inner http.Handler) http.Handler {
	wp := config.NormalizeWebPath(webPath)
	if wp == "" {
		return inner
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == wp {
			http.Redirect(w, r, wp+"/", http.StatusMovedPermanently)
			return
		}
		if path == wp+"/" || strings.HasPrefix(path, wp+"/") {
			r.URL.Path = strings.TrimPrefix(path, wp)
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
			// Propagate the stripped panel base path so subscription pages and
			// their links/QR codes stay reachable under the hidden entry point
			// (e.g. /secret/api/v1/client/sub/:token?html=1 renders links that
			// include /secret/). The public /api/v1/client/ branch below does
			// not set this header, which is correct: those requests carry no
			// secret prefix.
			r.Header.Set("X-Web-Path", wp)
			inner.ServeHTTP(w, r)
			return
		}
		// Public, prefix-less endpoints stay reachable.
		if strings.HasPrefix(path, "/api/v1/client/") || strings.HasPrefix(path, "/api/client/") ||
			path == "/api/v1/health" || path == "/healthz" {
			inner.ServeHTTP(w, r)
			return
		}
		if sp := config.NormalizeSubPath(subPath); sp != "" {
			if path == sp || strings.HasPrefix(path, sp+"/") {
				inner.ServeHTTP(w, r)
				return
			}
		}
		http.NotFound(w, r)
	})
}

func SetupRouter(cfg *config.Config) *gin.Engine {
	return SetupRouterWithDeps(Deps{Config: cfg})
}

func SetupRouterWithDeps(d Deps) *gin.Engine {
	cfg := d.Config
	if cfg == nil {
		cfg = &config.Config{}
	}
	db := resolveDB(d)

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.Default()
	r.Use(SecurityHeaders())
	r.Use(CORSMiddleware(cfg.Security.CORSOrigins))

	// The optional panel base path (hide UI/API behind /secret) is enforced by
	// WebPathHandler, which wraps the engine before its route tree is matched.

	RegisterLegacySubscriptionRoutes(r, db, cfg)
	RegisterCustomSubPathRoutes(r, db, cfg)

	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/openapi.yaml", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.OpenAPI)
		})
		auth.NewHandler(db, cfg).RegisterRoutes(apiV1.Group("/auth"))

		apiV1.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":     "ok",
				"version":    buildinfo.Summary(),
				"git_commit": buildinfo.GitCommit,
				"build_time": buildinfo.BuildTime,
			})
		})

		RegisterPublicSubscriptionRoutes(apiV1, db, cfg)

		apiV1.Use(auth.RequireAuth(db, cfg.JWT.Secret))
		// Access-token CRUD (GET/POST/PUT/DELETE) and the per-node
		// /nodes/:id/client-access + /nodes/:id/uri endpoints below are NOT
		// dead code: they back the subscription system (router/subscription.go
		// resolves a bearer access token to a Listener and generates a client
		// config via converter.GenerateRawConfig) and the node URI export used by
		// the frontend's "copy link" buttons. Do not remove without grepping both
		// callers — a previous audit (R3-F5) incorrectly flagged them as unused.
		RegisterAccessTokenRoutes(apiV1, d)

		registerDashboardRoute(apiV1, d)
		registerPanelSettingsRoutes(apiV1, d)
		registerPanelServerRoutes(apiV1, cfg)
		protocol.RegisterRoutes(apiV1)

		system.NewHandler(d.systemService()).WithDB(db).WithWARPDelete(mihomoConfig.DeleteServerWARP).WithBackupPaths(cfg.Database.Path, cfg.Mihomo.Config).RegisterRoutes(apiV1.Group("/system"))
		registerMihomoRoutes(apiV1, d)

		user.NewHandler(d.userService()).RegisterRoutes(apiV1.Group("/users"))
		telegram.NewHandler(db).RegisterRoutes(apiV1.Group("/telegram"))
		acme.NewHandler(db).RegisterRoutes(apiV1.Group("/system"))
		subpage.NewHandler(db).RegisterRoutes(apiV1.Group("/system"))
		cluster.NewHandler(cluster.NewService(db)).RegisterRoutes(apiV1.Group("/cluster"))

		// Listener owns the CRUD/template/version endpoints. Node adds the
		// node-specific URI and client-access endpoints. Registering both full
		// route sets on the same group causes Gin to panic on duplicate paths.
		if d.listenerService() != nil {
			listenerHandler := listener.NewHandler(d.listenerService())
			listenerHandler.RegisterRoutes(apiV1.Group("/nodes"))
			listenerHandler.RegisterRoutes(apiV1.Group("/listeners"))
		}
		if d.nodeService() != nil {
			nodeHandler := node.NewHandler(d.nodeService(), d.userService(), db)
			nodeHandler.RegisterClientRoutes(apiV1.Group("/nodes"))
		}

		traffic.RegisterRoutes(
			apiV1.Group("/traffic"),
			traffic.NewHandler(d.trafficService(), d.trafficCollector(), db),
		)

		registerConfigRoutes(apiV1, d, cfg)
		registerRoutingRoutes(apiV1, db)
	}

	return r
}

var hashedFrontendAsset = regexp.MustCompile(`^assets/.+-[A-Za-z0-9_-]{8,}\.(js|css)$`)

type frontendAsset struct {
	data, compressed                                []byte
	contentType, etag, compressedETag, cacheControl string
}

// MountFrontend serves embedded assets independently of API/subscription routes.
// Compression happens once at startup, not on the request path or on disk.
// webPath is the active panel base path ("" when served from root); it is
// injected into index.html so the SPA can build asset/API URLs relative to it.
func MountFrontend(r *gin.Engine, frontendFS fs.FS, webPath string) {
	staticFS, err := fs.Sub(frontendFS, "web/dist")
	if err != nil {
		log.Printf("frontend assets unavailable: %v", err)
		return
	}
	assets := make(map[string]frontendAsset)
	err = fs.WalkDir(staticFS, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(staticFS, name)
		if err != nil {
			return err
		}
		assets[name] = buildFrontendAsset(name, data)
		return nil
	})
	if err != nil {
		log.Printf("frontend assets unavailable: %v", err)
		return
	}
	// Inject the active base path so the SPA knows where it is mounted. When the
	// panel is served from the root (empty webPath) no injection is needed.
	if idx, ok := assets["index.html"]; ok && webPath != "" {
		assets["index.html"] = buildFrontendAsset("index.html", injectWebPathMeta(idx.data, webPath))
	}
	if _, ok := assets["index.html"]; !ok {
		log.Printf("frontend index.html unavailable")
		return
	}
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	r.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if strings.HasPrefix(requestPath, "/api") || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
			c.Status(http.StatusNotFound)
			return
		}
		name := strings.TrimPrefix(requestPath, "/")
		asset, ok := assets[name]
		if !ok {
			// Missing chunks must never return cacheable HTML after a deployment.
			if strings.HasPrefix(requestPath, "/assets/") || requestPath == "/assets" {
				c.Header("Cache-Control", "no-store")
				c.Status(http.StatusNotFound)
				return
			}
			name, asset = "index.html", assets["index.html"]
		}
		data, etag := asset.data, asset.etag
		if asset.compressed != nil {
			c.Writer.Header().Add("Vary", "Accept-Encoding")
			// Keep byte ranges in the identity representation; ServeContent handles
			// Range, If-Range, conditional requests and HEAD consistently.
			if c.Request.Header.Get("Range") == "" && acceptsFrontendGzip(c.Request.Header.Values("Accept-Encoding")) {
				data = asset.compressed
				etag = asset.compressedETag
				c.Header("Content-Encoding", "gzip")
			}
		}
		c.Header("Content-Type", asset.contentType)
		c.Header("Cache-Control", asset.cacheControl)
		c.Header("ETag", etag)
		http.ServeContent(c.Writer, c.Request, name, time.Time{}, bytes.NewReader(data))
	})
}

// buildFrontendAsset pre-computes content type, ETag and optional gzip body
// (and its ETag) once at startup.
func buildFrontendAsset(name string, data []byte) frontendAsset {
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	asset := frontendAsset{
		data:         data,
		contentType:  contentType,
		etag:         fmt.Sprintf(`"%x"`, sha256.Sum256(data)),
		cacheControl: "no-cache",
	}
	if hashedFrontendAsset.MatchString(name) {
		asset.cacheControl = "public, max-age=31536000, immutable"
	}
	if len(data) >= 1024 && (strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "javascript") || strings.Contains(contentType, "json") || strings.Contains(contentType, "svg+xml")) {
		var buffer bytes.Buffer
		writer := gzip.NewWriter(&buffer)
		if _, err := writer.Write(data); err != nil {
			return asset
		}
		if err := writer.Close(); err != nil {
			return asset
		}
		if buffer.Len() < len(data) {
			asset.compressed = buffer.Bytes()
			asset.compressedETag = fmt.Sprintf(`"%x"`, sha256.Sum256(asset.compressed))
		}
	}
	return asset
}

// injectWebPathMeta embeds the active panel base path into index.html as a
// <meta> tag (not a <script>, which would be blocked by the page CSP) so the
// SPA can compute asset and API URLs relative to it. An empty webPath yields "".
func injectWebPathMeta(data []byte, webPath string) []byte {
	meta := fmt.Sprintf("<meta name=\"x-web-path\" content=%q>", webPath)
	s := string(data)
	if i := strings.Index(s, "</head>"); i >= 0 {
		return []byte(s[:i] + meta + s[i:])
	}
	return []byte(meta + s)
}

func acceptsFrontendGzip(values []string) bool {
	wildcard := false
	for _, item := range strings.Split(strings.Join(values, ","), ",") {
		parts := strings.Split(item, ";")
		coding := strings.ToLower(strings.TrimSpace(parts[0]))
		if coding != "gzip" && coding != "*" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if ok && strings.EqualFold(strings.TrimSpace(key), "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					quality = 0
				} else {
					quality = parsed
				}
			}
		}
		// An explicit refusal overrides the wildcard regardless of ordering.
		if coding == "gzip" {
			return quality > 0
		}
		wildcard = quality > 0
	}
	return wildcard
}
