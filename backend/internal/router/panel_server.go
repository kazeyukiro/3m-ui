package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
)

// registerPanelServerRoutes exposes panel bind/port/public_url for NAT deployments.
func registerPanelServerRoutes(api *gin.RouterGroup, cfg *config.Config) {
	api.GET("/system/panel-server", func(c *gin.Context) {
		if cfg == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "config unavailable"})
			return
		}
		path := config.ConfigPath()
		c.JSON(http.StatusOK, gin.H{
			"port":        cfg.Server.Port,
			"listen":      cfg.Server.Listen,
			"public_url":  cfg.Server.PublicURL,
			"config_path": path,
			"hint":        "Changing port/listen writes config.yaml; restart 3m-ui to apply. Env: THREE_M_UI_PORT, THREE_M_UI_LISTEN, THREE_M_UI_PUBLIC_URL.",
			"env": gin.H{
				"port":       []string{"THREE_M_UI_PORT", "PANEL_PORT"},
				"listen":     []string{"THREE_M_UI_LISTEN", "PANEL_LISTEN"},
				"public_url": []string{"THREE_M_UI_PUBLIC_URL", "PUBLIC_URL"},
			},
		})
	})

	api.PUT("/system/panel-server", func(c *gin.Context) {
		if cfg == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "config unavailable"})
			return
		}
		var body struct {
			Port      *int    `json:"port"`
			Listen    *string `json:"listen"`
			PublicURL *string `json:"public_url"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		port := cfg.Server.Port
		if body.Port != nil {
			if *body.Port < 1 || *body.Port > 65535 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "port must be 1-65535"})
				return
			}
			port = *body.Port
		}
		listen := cfg.Server.Listen
		if body.Listen != nil {
			listen = strings.TrimSpace(*body.Listen)
		}
		publicURL := cfg.Server.PublicURL
		setPublic := body.PublicURL != nil
		if setPublic {
			publicURL = strings.TrimSpace(*body.PublicURL)
		}

		path := config.ConfigPath()
		if err := config.UpdateServerFile(path, port, listen, publicURL, setPublic); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		cfg.Server.Port = port
		cfg.Server.Listen = listen
		if setPublic {
			cfg.Server.PublicURL = publicURL
		}
		config.GlobalConfig = cfg

		c.JSON(http.StatusOK, gin.H{
			"status":           "ok",
			"port":             port,
			"listen":           listen,
			"public_url":       cfg.Server.PublicURL,
			"config_path":      path,
			"restart_required": true,
			"message":          "Saved. Restart 3m-ui (systemctl restart 3m-ui / rc-service 3m-ui restart) to apply a new panel port or listen address.",
		})
	})
}
