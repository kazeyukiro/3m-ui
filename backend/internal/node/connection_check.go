package node

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/converter"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mui"
	"github.com/kazeyukiro/3m-ui/backend/internal/netutil"
	"github.com/kazeyukiro/3m-ui/backend/internal/protocol"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

func (h *Handler) CheckConnection(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
		return
	}
	l, err := h.svc.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listener not found"})
		return
	}
	if h.svc.mihomo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "connection check unavailable"})
		return
	}
	result := h.svc.mihomo.CheckConnection(c.Request.Context(), *l, func() (string, error) {
		current, err := h.svc.GetByID(uint(id))
		if err != nil {
			return "", err
		}
		return h.svc.connectionClientYAML(*current)
	}, c.Query("reuse") == "true")
	c.JSON(http.StatusOK, result)
}

// Use the same exporter order and access-profile fallbacks as share export.
// The checker never creates credentials, changes bindings, or trusts a request
// Host as a probe target. The runner preserves the exported endpoint.
func (s *Service) connectionClientYAML(l models.Listener) (string, error) {
	byListener, err := user.NewService(s.db).ExistingCredentialsByListener()
	if err != nil {
		return "", err
	}
	credentials := byListener[l.ID]
	if _, bound := byListener[l.ID]; bound && len(credentials) == 0 {
		return "", fmt.Errorf("no active credentials")
	}
	ap := protocol.LoadAccessProfile(s.db)
	if strings.TrimSpace(l.PublicPort) == "" {
		l.PublicPort = ap.PublicPort
	}
	if strings.TrimSpace(l.AccessSNI) == "" {
		l.AccessSNI = ap.SNI
	}
	if strings.TrimSpace(l.ClientFingerprint) == "" {
		l.ClientFingerprint = ap.ClientFingerprint
	}
	if strings.TrimSpace(l.AccessALPN) == "" {
		l.AccessALPN = strings.Join(ap.ALPN, ",")
	}
	host := netutil.NormalizeHost(l.PublicHost)
	if host == "" {
		host = netutil.NormalizeHost(ap.PublicHost)
	}
	if host == "" && config.GlobalConfig != nil {
		host = netutil.NormalizeHost(config.GlobalConfig.Server.PublicURL)
	}
	if host == "" {
		return "", fmt.Errorf("client access host is not configured")
	}
	muiCreds := make([]mui.Cred, 0, len(credentials))
	creds := make([]protocol.UserCred, 0, len(credentials))
	for _, c := range credentials {
		muiCreds = append(muiCreds, mui.Cred{Username: c.Username, Password: c.Password, UUID: c.UUID})
		creds = append(creds, protocol.UserCred{Username: c.Username, Password: c.Password, UUID: c.UUID})
	}
	if shares, err := mui.BuildShares(l, host, muiCreds); err == nil && len(shares) > 0 {
		if len(shares[0].ClientYAML) > 0 {
			return string(shares[0].ClientYAML), nil
		}
		return "", fmt.Errorf("client YAML unavailable")
	}
	if shares, err := protocol.ExportShares(l, host, creds); err == nil && len(shares) > 0 {
		if shares[0].ClientYAML != "" {
			return shares[0].ClientYAML, nil
		}
		return "", fmt.Errorf("client YAML unavailable")
	}
	return converter.ExportClientYAML(l, host, credentials)
}
