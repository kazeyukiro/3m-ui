package subpage

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/subscription-page", h.Get)
	rg.PUT("/subscription-page", h.Put)
	rg.GET("/subscription-page/default-template", h.DefaultTemplate)
}

func (h *Handler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, LoadPageSettings(h.db))
}

func (h *Handler) Put(c *gin.Context) {
	var in Settings
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := SavePageSettings(h.db, in); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, LoadPageSettings(h.db))
}

func (h *Handler) DefaultTemplate(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(DefaultTemplate()))
}
