package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"gorm.io/gorm"
)

func registerRoutingRoutes(api *gin.RouterGroup, db *gorm.DB) {
	group := api.Group("/config")
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

	group.POST("/routing/inject-warp", func(c *gin.Context) {
		var body struct {
			Mode     string `json:"mode"`
			RuleMode string `json:"rule_mode"`
			Name     string `json:"name"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Mode == "" {
			body.Mode = "wireguard"
		}
		res, err := system.RegisterWARP()
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		yamlStr := res.YAML
		name := body.Name
		if name == "" {
			name = "WARP-OUT"
		}
		if body.Mode == "masque" {
			if res.MasqueYAML == "" {
				c.JSON(http.StatusBadGateway, gin.H{"error": "MASQUE YAML empty"})
				return
			}
			yamlStr = res.MasqueYAML
			if body.Name == "" {
				name = "WARP-Masque-OUT"
			}
		}
		cfg, err := mihomoConfig.InjectWARPProxy(db, yamlStr, name, body.RuleMode)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"name":    name,
			"rules":   cfg.Rules,
			"proxies": cfg.Proxies,
			"groups":  cfg.Groups,
		})
	})
}

