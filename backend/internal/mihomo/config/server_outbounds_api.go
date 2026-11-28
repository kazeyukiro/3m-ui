package config

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func outboundAPIError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, ErrInvalidOutbound) {
		status = http.StatusBadRequest
	}
	if errors.Is(err, ErrOutboundNotFound) {
		status = http.StatusNotFound
	}
	if errors.Is(err, ErrOutboundReferenced) {
		status = http.StatusConflict
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

// RegisterServerOutboundRoutes is registered beneath the authenticated /config group.
func RegisterServerOutboundRoutes(group *gin.RouterGroup, db *gorm.DB) {
	group.GET("/server-outbounds", func(c *gin.Context) {
		entries, err := ListServerOutbounds(db)
		if err != nil {
			outboundAPIError(c, err)
			return
		}
		c.JSON(http.StatusOK, entries)
	})
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		group.Handle(method, "/server-outbounds", func(c *gin.Context) {
			var body struct {
				Proxy   ProxyEntry `json:"proxy"`
				Enabled *bool      `json:"enabled" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			entry := ServerOutbound{Proxy: body.Proxy, Enabled: *body.Enabled}
			previous := ""
			if c.Request.Method == http.MethodPut {
				previous = c.Query("name")
				if previous == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "name query parameter is required"})
					return
				}
			}
			if err := SaveServerOutbound(db, previous, entry); err != nil {
				outboundAPIError(c, err)
				return
			}
			status := http.StatusOK
			if c.Request.Method == http.MethodPost {
				status = http.StatusCreated
			}
			c.JSON(status, entry)
		})
	}
	group.DELETE("/server-outbounds", func(c *gin.Context) {
		if err := DeleteServerOutbound(db, c.Query("name")); err != nil {
			outboundAPIError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	group.POST("/server-outbounds/enabled", func(c *gin.Context) {
		var body struct {
			Names   []string `json:"names" binding:"required,min=1"`
			Enabled *bool    `json:"enabled" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := SetServerOutboundsEnabled(db, body.Names, *body.Enabled); err != nil {
			outboundAPIError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	group.PATCH("/server-routing", func(c *gin.Context) {
		var update ServerRoutingUpdate
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := UpdateServerRouting(db, update); err != nil {
			outboundAPIError(c, err)
			return
		}
		sr, err := GetServerRouting(db)
		if err != nil {
			outboundAPIError(c, err)
			return
		}
		c.JSON(http.StatusOK, sr)
	})
}
