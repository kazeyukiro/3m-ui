package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/acme"
	"github.com/kazeyukiro/3m-ui/backend/internal/auth"
	"github.com/kazeyukiro/3m-ui/backend/internal/cluster"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/docs"
	"github.com/kazeyukiro/3m-ui/backend/internal/listener"
	"github.com/kazeyukiro/3m-ui/backend/internal/node"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
	"github.com/kazeyukiro/3m-ui/backend/internal/subpage"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"github.com/kazeyukiro/3m-ui/backend/internal/telegram"
	"github.com/kazeyukiro/3m-ui/backend/internal/traffic"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

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
	r.Use(CORSMiddleware(cfg.Security.CORSOrigins))

	RegisterLegacySubscriptionRoutes(r, db, cfg)

	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/openapi.yaml", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.OpenAPI)
		})
		auth.NewHandler(db, cfg).RegisterRoutes(apiV1.Group("/auth"))

		apiV1.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		RegisterPublicSubscriptionRoutes(apiV1, db, cfg)

		apiV1.Use(auth.RequireAuth(db, cfg.JWT.Secret))
		RegisterAccessTokenRoutes(apiV1, d)

		registerDashboardRoute(apiV1, d)
		registerPanelSettingsRoutes(apiV1, d)
		registerPanelServerRoutes(apiV1, cfg)
		protocol.RegisterRoutes(apiV1)

		system.NewHandler(d.systemService()).WithBackupPaths(cfg.Database.Path, cfg.Mihomo.Config).RegisterRoutes(apiV1.Group("/system"))
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
