package router

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
)

func registerMihomoRoutes(api *gin.RouterGroup, d Deps) {
	group := api.Group("/mihomo")
	group.GET("/releases", func(c *gin.Context) {
		releases, err := d.mihomoService().CoreReleases(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, releases)
	})
	group.GET("/update", func(c *gin.Context) { c.JSON(http.StatusOK, d.mihomoService().CoreUpdateStatus()) })
	begin := func(c *gin.Context, rollback bool) {
		var input struct {
			Version string `json:"version"`
		}
		if !rollback && c.ShouldBindJSON(&input) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "a version is required"})
			return
		}
		job, err := d.mihomoService().BeginCoreUpdate(input.Version, rollback)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, mihomo.ErrCoreUpdateBusy) {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, job)
	}
	group.POST("/update", func(c *gin.Context) { begin(c, false) })
	group.POST("/update/rollback", func(c *gin.Context) { begin(c, true) })

	group.GET("/status", func(c *gin.Context) {
		status, err := d.mihomoService().GetStatus()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, status)
	})
	group.POST("/start", func(c *gin.Context) {
		if err := d.mihomoService().StartMihomo(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Mihomo started"})
	})
	group.POST("/stop", func(c *gin.Context) {
		if err := d.mihomoService().StopMihomo(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Mihomo stopped"})
	})
	group.POST("/restart", func(c *gin.Context) {
		if err := d.mihomoService().RestartMihomo(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Mihomo restarted"})
	})
	group.GET("/logs", func(c *gin.Context) {
		logs, err := d.mihomoService().GetLogs()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, logs)
	})
}
