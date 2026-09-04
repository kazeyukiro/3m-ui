package cluster

import (
	"errors"
	"log"
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
	rg.POST("/health-all", h.HealthAll)
	rg.GET("/mirrored-nodes", h.ListMirroredNodes)
	rg.PUT("/:id", h.Update)
	rg.DELETE("/:id", h.Delete)
	rg.POST("/:id/health", h.Health)
	rg.GET("/:id/dashboard", h.RemoteDashboard)
	rg.GET("/:id/users", h.RemoteUsers)
	rg.GET("/:id/nodes", h.RemoteNodes)
	rg.POST("/:id/sync-nodes", h.SyncNodes)
	rg.POST("/:id/nodes", h.RemoteCreateNode)
	rg.PUT("/:id/nodes/:nodeId", h.RemoteUpdateNode)
	rg.DELETE("/:id/nodes/:nodeId", h.RemoteDeleteNode)
	rg.POST("/:id/mihomo/start", h.RemoteStart)
	rg.POST("/:id/mihomo/stop", h.RemoteStop)
	rg.POST("/:id/mihomo/restart", h.RemoteRestart)
	rg.POST("/:id/proxy", h.Proxy)
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
		log.Printf("cluster list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
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
		log.Printf("cluster health check failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *Handler) HealthAll(c *gin.Context) {
	rows, err := h.svc.HealthCheckAll()
	if err != nil {
		log.Printf("cluster health-check-all failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) RemoteNodes(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	raw, err := h.svc.FetchRemoteNodes(id)
	if err != nil {
		writeRemoteErr(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

func (h *Handler) RemoteDashboard(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	raw, err := h.svc.FetchDashboard(id)
	if err != nil {
		writeRemoteErr(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

func (h *Handler) RemoteUsers(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	raw, err := h.svc.FetchUsers(id)
	if err != nil {
		writeRemoteErr(c, err)
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
		writeRemoteErr(c, err)
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
		writeRemoteErr(c, err)
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
		writeRemoteErr(c, err)
		return
	}
	if len(raw) == 0 {
		c.JSON(status, gin.H{"status": "ok"})
		return
	}
	c.Data(status, "application/json; charset=utf-8", raw)
}

func (h *Handler) RemoteStart(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	raw, err := h.svc.StartCore(id)
	if err != nil {
		writeRemoteErr(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

func (h *Handler) RemoteStop(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	raw, err := h.svc.StopCore(id)
	if err != nil {
		writeRemoteErr(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

func (h *Handler) RemoteRestart(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	raw, err := h.svc.RestartCore(id)
	if err != nil {
		writeRemoteErr(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

// Proxy is a restricted generic forwarder (allowlisted paths only).
func (h *Handler) Proxy(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		Body   string `json:"body"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "method and path required"})
		return
	}
	method := body.Method
	if method == "" {
		method = http.MethodGet
	}
	var payload []byte
	if body.Body != "" {
		payload = []byte(body.Body)
	}
	status, raw, err := h.svc.ProxyRemote(id, method, body.Path, payload)
	if err != nil {
		writeRemoteErr(c, err)
		return
	}
	c.Data(status, "application/json; charset=utf-8", raw)
}

func writeRemoteErr(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	log.Printf("cluster remote operation failed: %v", err)
	c.JSON(http.StatusBadGateway, gin.H{"error": "upstream cluster operation failed"})
}


func (h *Handler) SyncNodes(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	rows, err := h.svc.SyncRemoteNodes(id)
	if err != nil {
		writeRemoteErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *Handler) ListMirroredNodes(c *gin.Context) {
	var remoteID uint
	if v := c.Query("remote_server_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			remoteID = uint(n)
		}
	}
	rows, err := h.svc.ListMirroredNodes(remoteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, rows)
}
