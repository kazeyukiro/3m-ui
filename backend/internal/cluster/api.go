package cluster

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.List)
	rg.POST("", h.Create)
	rg.PUT("/:id", h.Update)
	rg.DELETE("/:id", h.Delete)
	rg.POST("/:id/health", h.Health)
	rg.GET("/:id/nodes", h.RemoteNodes)
	rg.POST("/:id/nodes", h.RemoteCreateNode)
	rg.PUT("/:id/nodes/:nodeId", h.RemoteUpdateNode)
	rg.DELETE("/:id/nodes/:nodeId", h.RemoteDeleteNode)
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(id), true
}

func (h *Handler) List(c *gin.Context) {
	rows, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) Create(c *gin.Context) {
	var in CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := h.svc.Create(in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, row)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in UpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := h.svc.Update(id, in)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) Health(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	row, err := h.svc.HealthCheck(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *Handler) RemoteNodes(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	raw, err := h.svc.FetchRemoteNodes(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

func (h *Handler) RemoteCreateNode(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	status, raw, err := h.svc.ProxyRemote(id, http.MethodPost, "/api/v1/nodes", body)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(status, "application/json; charset=utf-8", raw)
}

func (h *Handler) RemoteUpdateNode(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	nodeID := c.Param("nodeId")
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	status, raw, err := h.svc.ProxyRemote(id, http.MethodPut, "/api/v1/nodes/"+nodeID, body)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(status, "application/json; charset=utf-8", raw)
}

func (h *Handler) RemoteDeleteNode(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	nodeID := c.Param("nodeId")
	status, raw, err := h.svc.ProxyRemote(id, http.MethodDelete, "/api/v1/nodes/"+nodeID, nil)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if len(raw) == 0 {
		c.JSON(status, gin.H{"status": "ok"})
		return
	}
	c.Data(status, "application/json; charset=utf-8", raw)
}
