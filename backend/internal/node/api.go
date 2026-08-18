package node

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// Handler serves node/listener HTTP endpoints with injected dependencies.
type Handler struct {
	svc  *Service
	user *user.Service
	db   *gorm.DB
}

// NewHandler constructs a node HTTP handler.
func NewHandler(svc *Service, userSvc *user.Service, db *gorm.DB) *Handler {
	return &Handler{svc: svc, user: userSvc, db: db}
}

// RegisterRoutes registers the node CRUD routes. Kept for callers that use
// the node package directly; the main router uses the listener handler for
// shared CRUD routes and RegisterClientRoutes for node-specific endpoints.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.ListNodes)
	rg.POST("", h.CreateNode)
	rg.GET("/:id", h.GetNode)
	rg.GET("/:id/uri", h.ExportNodeURI)
	rg.PUT("/:id", h.UpdateNode)
	rg.DELETE("/:id", h.DeleteNode)
	rg.POST("/:id/reload", h.ReloadNode)
	rg.POST("/:id/client-access", h.CreateClientAccess)
}

// RegisterClientRoutes registers only routes that are unique to the node
// handler. The listener handler already owns shared CRUD/reload endpoints.
func (h *Handler) RegisterClientRoutes(rg *gin.RouterGroup) {
	rg.GET("/:id/uri", h.ExportNodeURI)
	rg.POST("/:id/client-access", h.CreateClientAccess)
}

func (h *Handler) ListNodes(c *gin.Context) {
	list, err := h.svc.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) CreateNode(c *gin.Context) {
	var l models.Listener
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ValidateNode(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.Create(&l); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, l)
}
