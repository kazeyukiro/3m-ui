package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func registerPanelSettingsRoutes(api *gin.RouterGroup, d Deps) {
	api.GET("/panel-settings", func(c *gin.Context) {
		db := resolveDB(d)
		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
			return
		}
		var rows []models.PanelSetting
		_ = db.Find(&rows).Error
		out := map[string]string{}
		for _, r := range rows {
			if strings.Contains(strings.ToLower(r.Key), "token") || strings.Contains(strings.ToLower(r.Key), "secret") {
				continue
			}
			out[r.Key] = r.Value
		}
		c.JSON(http.StatusOK, out)
	})
	api.PUT("/panel-settings", func(c *gin.Context) {
		db := resolveDB(d)
		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
			return
		}
		var body map[string]string
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		for k, v := range body {
			k = strings.TrimSpace(k)
			if k == "" || strings.Contains(strings.ToLower(k), "token") {
				continue
			}
			var row models.PanelSetting
			err := db.Where("key = ?", k).First(&row).Error
			if err != nil {
				_ = db.Create(&models.PanelSetting{Key: k, Value: v}).Error
			} else {
				row.Value = v
				_ = db.Save(&row).Error
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
