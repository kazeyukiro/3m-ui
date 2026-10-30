package router

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	mihomo "github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gorm.io/gorm"
)

func registerRoutingRoutes(api *gin.RouterGroup, db *gorm.DB) {
	group := api.Group("/config")
	mihomoConfig.RegisterServerOutboundRoutes(group, db)
	group.GET("/groups", func(c *gin.Context) {
		visual, err := mihomoConfig.GetVisualConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, visual.Groups)
	})
	group.PUT("/groups", func(c *gin.Context) {
		var groups []mihomoConfig.GroupEntry
		if err := c.ShouldBindJSON(&groups); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		visual, err := mihomoConfig.GetVisualConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		visual.Groups = groups
		if err = mihomoConfig.SaveVisualConfig(db, visual); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, visual.Groups)
	})
	group.GET("/rules", func(c *gin.Context) {
		visual, err := mihomoConfig.GetVisualConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, visual.Rules)
	})
	group.PUT("/rules", func(c *gin.Context) {
		var rules []string
		if err := c.ShouldBindJSON(&rules); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		visual, err := mihomoConfig.GetVisualConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		visual.Rules = rules
		if err = mihomoConfig.SaveVisualConfig(db, visual); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, visual.Rules)
	})

	// Server-side egress (serving Mihomo), distinct from client subscription visual-config.
	group.GET("/server-routing", func(c *gin.Context) {
		sr, err := mihomoConfig.GetServerRouting(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sr)
	})
	group.PUT("/server-routing", func(c *gin.Context) {
		var sr mihomoConfig.ServerRoutingConfig
		if err := c.ShouldBindJSON(&sr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := mihomoConfig.SaveServerRouting(db, sr); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		saved, err := mihomoConfig.GetServerRouting(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, saved)
	})

	// Batch-import parse endpoint. Both scopes (client subscription rules
	// and server egress rules) share the same mihomo rule syntax, so one
	// parser serves both; parsing never persists anything — the editor
	// previews the normalized lines and appends them on confirm.
	group.POST("/rules/parse", func(c *gin.Context) {
		var req struct {
			Text string `json:"text"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, mihomoConfig.ParseRulesText(req.Text))
	})

	// Rule providers (rule-set) — stored in server-routing; hot-update via Mihomo API.
	group.GET("/rule-providers/status", func(c *gin.Context) {
		controller := strings.TrimSpace(os.Getenv("THREE_M_UI_MIHOMO_CONTROLLER"))
		if controller == "" {
			controller = "127.0.0.1:9090"
		}
		base := "http://" + controller
		api := mihomo.NewExternalControllerAPI(base, mihomoConfig.ControllerSecret())
		data, err := api.ListRuleProviders()
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "providers": map[string]interface{}{}})
			return
		}
		c.JSON(http.StatusOK, data)
	})
	group.PUT("/rule-providers/:name/update", func(c *gin.Context) {
		name := strings.TrimSpace(c.Param("name"))
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
			return
		}
		controller := strings.TrimSpace(os.Getenv("THREE_M_UI_MIHOMO_CONTROLLER"))
		if controller == "" {
			controller = "127.0.0.1:9090"
		}
		base := "http://" + controller
		api := mihomo.NewExternalControllerAPI(base, mihomoConfig.ControllerSecret())
		if err := api.UpdateRuleProvider(name); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "name": name})
	})

}
