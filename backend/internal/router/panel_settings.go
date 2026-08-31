package router

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

func registerPanelSettingsRoutes(api *gin.RouterGroup, d Deps) {
	api.GET("/panel-settings", func(c *gin.Context) {
		db := resolveDB(d)
		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
			return
		}
		var rows []models.PanelSetting
		if err := db.Find(&rows).Error; err != nil {
			log.Printf("panel-settings list failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		out := map[string]string{}
		for _, r := range rows {
			if isSensitiveSettingKey(r.Key) {
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
		// Apply all settings in a single transaction so a partial failure (e.g.
		// a DB error mid-loop) rolls back every prior create/update and leaves
		// the persisted settings in a consistent state rather than half-applied.
		txErr := db.Transaction(func(tx *gorm.DB) error {
			for k, v := range body {
				k = strings.TrimSpace(k)
				if isSensitiveSettingKey(k) {
					continue
				}
				var row models.PanelSetting
				err := tx.Where("key = ?", k).First(&row).Error
				if err != nil {
					if createErr := tx.Create(&models.PanelSetting{Key: k, Value: v}).Error; createErr != nil {
						log.Printf("panel-settings create %q failed: %v", k, createErr)
						return createErr
					}
				} else {
					row.Value = v
					if saveErr := tx.Save(&row).Error; saveErr != nil {
						log.Printf("panel-settings save %q failed: %v", k, saveErr)
						return saveErr
					}
				}
			}
			return nil
		})
		if txErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

// isSensitiveSettingKey reports whether a panel setting key holds a secret that
// must never be returned in bulk reads nor written through the generic PUT
// endpoint. The match is case-insensitive substring on a curated keyword list
// plus a small set of exact keys whose values are inherently sensitive.
func isSensitiveSettingKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return true
	}
	switch k {
	case "telegram", "panel_ssl":
		return true
	}
	for _, s := range []string{"token", "secret", "credential", "password", "private", "bot", "key", "auth"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}
