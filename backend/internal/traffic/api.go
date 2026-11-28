package traffic

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

type Handler struct {
	service   *Service
	collector *Collector
	db        *gorm.DB
}

func NewHandler(service *Service, collector *Collector, db *gorm.DB) *Handler {
	return &Handler{service: service, collector: collector, db: db}
}

// RegisterRoutes mounts the traffic endpoints under the given group,
// following the same RegisterRoutes(rg) convention used by the
// user/node/subscription packages.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/status", h.Status)
	rg.GET("/users", h.Users)
	rg.GET("/connections", h.Connections)
}

// Status returns the current global traffic snapshot (rate + cumulative
// totals + active connection count), as last collected by the scheduler.
func (h *Handler) Status(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusOK, Snapshot{})
		return
	}
	c.JSON(http.StatusOK, h.service.Current())
}

// Users returns per-proxy-user traffic data.
func (h *Handler) Users(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, []UserTraffic{})
		return
	}
	var users []models.ProxyUser
	if err := h.db.Order("id desc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]UserTraffic, 0, len(users))
	for _, u := range users {
		out = append(out, UserTraffic{
			UserID: u.ID, Username: u.Username,
			UploadBytes: u.UploadBytes, DownloadBytes: u.DownloadBytes,
			Online: u.Online, TrafficUsed: u.TrafficUsed, TrafficLimit: u.TrafficLimit,
			ExpireTime: u.ExpireTime, LastSeen: u.LastSeen,
			Blocked: !user.IsCredentialActive(u),
		})
	}
	c.JSON(http.StatusOK, out)
}

// Connections returns the most recently collected, listener/user-mapped
// connection list.
func (h *Handler) Connections(c *gin.Context) {
	if h.collector == nil {
		c.JSON(http.StatusOK, gin.H{"connections": []ConnectionView{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"connections": h.collector.CurrentConnections()})
}
