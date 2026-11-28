package mihomo

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	mihomoconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gorm.io/gorm"
)

func RegisterOutboundConnectionRoute(group *gin.RouterGroup, db *gorm.DB, svc *Service) {
	group.POST("/server-outbounds/connection-check", func(c *gin.Context) {
		name := c.Query("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name query parameter is required"})
			return
		}
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Mihomo service is unavailable"})
			return
		}
		result, err := svc.CheckOutboundConnection(c.Request.Context(), db, name)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, mihomoconfig.ErrOutboundNotFound) {
				status = http.StatusNotFound
			} else if errors.Is(err, mihomoconfig.ErrInvalidOutbound) {
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	})
}
