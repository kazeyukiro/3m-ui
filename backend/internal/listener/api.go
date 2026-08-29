package listener

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.ListListeners)
	rg.POST("", h.CreateListener)
	rg.POST("/generate", h.GenerateMaterial)
	rg.GET("/templates", h.ListTemplates)
	rg.POST("/templates", h.CreateTemplate)
	rg.GET("/templates/:id", h.GetTemplate)
	rg.DELETE("/templates/:id", h.DeleteTemplate)
	rg.POST("/templates/:id/instantiate", h.InstantiateTemplate)
	rg.POST("/batch", h.BatchCreate)
	rg.POST("/batch/enabled", h.BatchEnabled)
	rg.GET("/:id", h.GetListener)
	rg.PUT("/:id", h.UpdateListener)
	rg.DELETE("/:id", h.DeleteListener)
	rg.POST("/:id/reload", h.ReloadListener)
	rg.POST("/:id/clone", h.CloneListener)
	rg.GET("/:id/versions", h.ListVersions)
	rg.GET("/:id/versions/:version/diff", h.DiffVersion)
	rg.POST("/:id/versions/:version/rollback", h.RollbackVersion)
}
func parseID(c *gin.Context, key string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(key), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(n), true
}
func (h *Handler) ListListeners(c *gin.Context) {
	list, err := h.svc.GetAll()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// search across inbound name / protocol / port / bind.
	q := strings.ToLower(strings.TrimSpace(c.Query("q")))
	if q != "" {
		filtered := make([]models.Listener, 0, len(list))
		for _, l := range list {
			hay := strings.ToLower(l.Name + " " + l.Protocol + " " + l.Port + " " + l.BindAddress + " " + l.PublicHost)
			if strings.Contains(hay, q) {
				filtered = append(filtered, l)
			}
		}
		list = filtered
	}
	if en := c.Query("enabled"); en == "true" || en == "1" {
		out := make([]models.Listener, 0)
		for _, l := range list {
			if l.Enabled {
				out = append(out, l)
			}
		}
		list = out
	} else if en == "false" || en == "0" {
		out := make([]models.Listener, 0)
		for _, l := range list {
			if !l.Enabled {
				out = append(out, l)
			}
		}
		list = out
	}
	c.JSON(200, list)
}
func (h *Handler) CreateListener(c *gin.Context) {
	var l models.Listener
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.Create(&l); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, l)
}
func (h *Handler) BatchCreate(c *gin.Context) {
	var list []models.Listener
	if err := c.ShouldBindJSON(&list); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	out, err := h.svc.BatchCreate(list)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, out)
}
func (h *Handler) GetListener(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	l, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, l)
}
func (h *Handler) UpdateListener(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var l models.Listener
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	l.ID = id
	if err := h.svc.Update(&l); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, l)
}
func (h *Handler) DeleteListener(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}
func (h *Handler) ReloadListener(c *gin.Context) {
	if _, ok := parseID(c, "id"); !ok {
		return
	}
	if err := h.svc.RegenerateConfig(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}
func (h *Handler) CloneListener(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
		Port string `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	l, err := h.svc.Clone(id, req.Name, req.Port)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, l)
}
func (h *Handler) ListVersions(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	v, err := h.svc.ListVersions(id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, v)
}
func (h *Handler) DiffVersion(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	v, err := strconv.Atoi(c.Param("version"))
	if err != nil || v < 1 {
		c.JSON(400, gin.H{"error": "invalid version"})
		return
	}
	d, err := h.svc.DiffVersion(id, v)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.Data(200, "text/plain; charset=utf-8", []byte(d))
}
func (h *Handler) RollbackVersion(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	v, err := strconv.Atoi(c.Param("version"))
	if err != nil || v < 1 {
		c.JSON(400, gin.H{"error": "invalid version"})
		return
	}
	if err := h.svc.RollbackVersion(id, v); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}
func (h *Handler) BatchEnabled(c *gin.Context) {
	var req struct {
		IDs     []uint `json:"ids"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.BatchSetEnabled(req.IDs, req.Enabled); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}
func (h *Handler) ListTemplates(c *gin.Context) {
	t, err := h.svc.ListTemplates()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, t)
}
func (h *Handler) GetTemplate(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	t, err := h.svc.GetTemplate(id)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, t)
}
func (h *Handler) CreateTemplate(c *gin.Context) {
	var t models.ListenerTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.CreateTemplate(&t); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, t)
}
func (h *Handler) DeleteTemplate(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteTemplate(id); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}
func (h *Handler) InstantiateTemplate(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
		Port string `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	l, err := h.svc.InstantiateTemplate(id, req.Name, req.Port)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, l)
}

func (h *Handler) GenerateMaterial(c *gin.Context) {
	var body struct {
		Kind   string `json:"kind"`
		Cipher string `json:"cipher"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := GenerateMaterial(body.Kind, body.Cipher)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}
