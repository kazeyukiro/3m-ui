package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	"github.com/kazeyukiro/3m-ui/backend/internal/traffic"
)

func registerDashboardRoute(api *gin.RouterGroup, d Deps) {
	api.GET("/dashboard", func(c *gin.Context) {
		var mihomoStatus *mihomo.StatusResponse
		if ms := d.mihomoService(); ms != nil {
			var err error
			mihomoStatus, err = ms.GetStatus()
			if err != nil {
				mihomoStatus = &mihomo.StatusResponse{Running: false, Version: "unknown", PID: 0, Uptime: "0s"}
			}
		} else {
			mihomoStatus = &mihomo.StatusResponse{Running: false, Version: "unknown", PID: 0, Uptime: "0s"}
		}

		var sysStatus interface{}
		if sysSvc := d.systemService(); sysSvc != nil {
			sysStatus = sysSvc.GetStatus()
		}

		db := resolveDB(d)
		var listenerTotal int64
		var listenerEnabled int64
		if db != nil {
			db.Model(&models.Listener{}).Count(&listenerTotal)
			db.Model(&models.Listener{}).Where("enabled = ?", true).Count(&listenerEnabled)
		}
		listenerDisabled := listenerTotal - listenerEnabled
		if listenerDisabled < 0 {
			listenerDisabled = 0
		}

		var trafficSnapshot traffic.Snapshot
		if ts := d.trafficService(); ts != nil {
			trafficSnapshot = ts.Current()
		}
		var onlineUsers, userTotal, userEnabled int64
		if db != nil {
			db.Model(&models.ProxyUser{}).Where("online = ?", true).Count(&onlineUsers)
			db.Model(&models.ProxyUser{}).Count(&userTotal)
			db.Model(&models.ProxyUser{}).Where("enabled = ?", true).Count(&userEnabled)
		}
		activeConnections := trafficSnapshot.Connections
		if col := d.trafficCollector(); col != nil {
			activeConnections = len(col.CurrentConnections())
		}

		c.JSON(http.StatusOK, gin.H{
			"mihomo": mihomoStatus,
			"system": sysStatus,
			"listeners": gin.H{
				"total":    listenerTotal,
				"enabled":  listenerEnabled,
				"disabled": listenerDisabled,
			},
			"users": gin.H{
				"total":   userTotal,
				"enabled": userEnabled,
				"online":  onlineUsers,
			},
			"traffic": gin.H{
				"uploadRate":        trafficSnapshot.UploadRate,
				"downloadRate":      trafficSnapshot.DownloadRate,
				"totalUpload":       trafficSnapshot.UploadBytes,
				"totalDownload":     trafficSnapshot.DownloadBytes,
				"onlineUsers":       onlineUsers,
				"activeConnections": activeConnections,
			},
		})
	})
}
