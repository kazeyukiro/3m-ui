package router

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
)

func registerConfigRoutes(api *gin.RouterGroup, d Deps, cfg *config.Config) {
	db := resolveDB(d)
	group := api.Group("/config")

	group.GET("/proxies", func(c *gin.Context) {
		visual, err := mihomoConfig.GetVisualConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, visual.Proxies)
	})
	group.POST("/proxies", func(c *gin.Context) {
		var proxy mihomoConfig.ProxyEntry
		if err := c.ShouldBindJSON(&proxy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if proxy.Name == "" || proxy.Type == "" || proxy.Server == "" || proxy.Port == "" || proxy.Port == "0" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "名称、协议、服务器和有效端口不能为空"})
			return
		}
		visual, err := mihomoConfig.GetVisualConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		visual.Proxies = append(visual.Proxies, proxy)
		if err = mihomoConfig.SaveVisualConfig(db, visual); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, proxy)
	})
	group.PUT("/proxies/:index", func(c *gin.Context) {
		idx, err := strconv.Atoi(c.Param("index"))
		if err != nil || idx < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效节点索引"})
			return
		}
		var proxy mihomoConfig.ProxyEntry
		if err = c.ShouldBindJSON(&proxy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		visual, err := mihomoConfig.GetVisualConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if idx >= len(visual.Proxies) {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		visual.Proxies[idx] = proxy
		if err = mihomoConfig.SaveVisualConfig(db, visual); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, proxy)
	})
	group.DELETE("/proxies/:index", func(c *gin.Context) {
		idx, err := strconv.Atoi(c.Param("index"))
		if err != nil || idx < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效节点索引"})
			return
		}
		visual, err := mihomoConfig.GetVisualConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if idx >= len(visual.Proxies) {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		visual.Proxies = append(visual.Proxies[:idx], visual.Proxies[idx+1:]...)
		if err = mihomoConfig.SaveVisualConfig(db, visual); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	group.GET("/visual", func(c *gin.Context) {
		visual, err := mihomoConfig.GetVisualConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, visual)
	})
	group.POST("/visual", func(c *gin.Context) {
		var visual mihomoConfig.VisualConfig
		if err := c.ShouldBindJSON(&visual); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid visual configuration: " + err.Error()})
			return
		}
		if err := mihomoConfig.SaveVisualConfig(db, visual); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Visual configuration saved"})
	})
	group.GET("", func(c *gin.Context) {
		engine := mihomoConfig.NewConfigEngine(db)
		yamlStr, err := engine.GenerateFinalConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"config": yamlStr})
	})

	// /generate only builds YAML from DB (preview). It does NOT apply to disk/core.
	// Use POST /config/validate then POST /config/apply for a safe publish path.
	group.POST("/generate", func(c *gin.Context) {
		engine := mihomoConfig.NewConfigEngine(db)
		yamlStr, err := engine.GenerateFinalConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "config": yamlStr, "message": "Configuration generated (not applied)"})
	})
	group.POST("/preview", func(c *gin.Context) {
		var req struct {
			Config string `json:"config"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		if err := mihomoConfig.ValidateConfigYAML(req.Config); err != nil {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"valid": true, "config": req.Config})
	})
	group.POST("/validate", func(c *gin.Context) {
		var req struct {
			Config string `json:"config"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		if err := mihomoConfig.ValidateConfigYAML(req.Config); err != nil {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"valid": true})
	})
	group.POST("/apply", func(c *gin.Context) {
		var req struct {
			Config string `json:"config"`
		}
		_ = c.ShouldBindJSON(&req)
		yamlStr := strings.TrimSpace(req.Config)
		if yamlStr == "" {
			engine := mihomoConfig.NewConfigEngine(db)
			var err error
			yamlStr, err = engine.GenerateFinalConfig()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		if err := d.mihomoService().ApplyConfig(yamlStr); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Configuration validated and applied successfully"})
	})
	group.POST("/rollback", func(c *gin.Context) {
		if err := d.mihomoService().RollbackConfig(); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Previous Mihomo configuration restored"})
	})
	group.GET("/download", func(c *gin.Context) {
		engine := mihomoConfig.NewConfigEngine(db)
		yamlStr, err := engine.GenerateFinalConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", "attachment; filename=config.yaml")
		c.Data(http.StatusOK, "application/yaml", []byte(yamlStr))
	})

	_ = cfg
}
