package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	mihomoConfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
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
}
