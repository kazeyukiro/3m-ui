package protocol

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts capability endpoints under the given API group.
func RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/capabilities", func(c *gin.Context) {
		c.JSON(http.StatusOK, DefaultManifest())
	})
}
