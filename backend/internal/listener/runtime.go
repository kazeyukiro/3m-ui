package listener

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	"gorm.io/gorm"
)

type runtimeInspector interface {
	ListenerRuntime([]models.Listener, bool) []mihomo.ListenerRuntime
}

func (s *Service) runtimeStatus(id uint, details bool) ([]mihomo.ListenerRuntime, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var listeners []models.Listener
	if id != 0 {
		var l models.Listener
		if err := s.db.First(&l, id).Error; err != nil {
			return nil, err
		}
		listeners = append(listeners, l)
	} else if err := s.db.Order("id desc").Find(&listeners).Error; err != nil {
		return nil, err
	}
	if inspector, ok := s.mihomoApply.(runtimeInspector); ok {
		return inspector.ListenerRuntime(listeners, details), nil
	}
	result := make([]mihomo.ListenerRuntime, 0, len(listeners))
	for _, l := range listeners {
		state, reason := "unknown", "inspection_unavailable"
		if !l.Enabled {
			state, reason = "disabled", "disabled"
		}
		result = append(result, mihomo.ListenerRuntime{ID: l.ID, State: state, Reason: reason, CheckedAt: time.Now().UTC(), Endpoints: []mihomo.RuntimeEndpoint{}})
	}
	return result, nil
}

func (h *Handler) RuntimeStatus(c *gin.Context) {
	statuses, err := h.svc.runtimeStatus(0, c.Query("summary") != "true")
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to read listener status"})
		return
	}
	c.JSON(200, statuses)
}

// CheckRuntime only observes local sockets. It neither restarts Mihomo nor
// connects to a user-supplied public host.
func (h *Handler) CheckRuntime(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if id == 0 {
		c.JSON(400, gin.H{"error": "invalid listener id"})
		return
	}
	statuses, err := h.svc.runtimeStatus(id, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "listener not found"})
		} else {
			c.JSON(500, gin.H{"error": "failed to read listener status"})
		}
		return
	}
	c.JSON(200, statuses[0])
}
