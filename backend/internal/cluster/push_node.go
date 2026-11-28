package cluster

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// PushNodeInput clones a local listener onto a remote panel (optional dry-run).
type PushNodeInput struct {
	LocalNodeID uint   `json:"local_node_id"`
	DryRun      bool   `json:"dry_run"`
	NewName     string `json:"new_name"`
	NewPort     string `json:"new_port"`
}

// PushLocalNodeToRemote posts a local listener body to remote POST /api/v1/nodes.
func (s *Service) PushLocalNodeToRemote(remoteID uint, in PushNodeInput) (map[string]interface{}, error) {
	if in.LocalNodeID == 0 {
		return nil, fmt.Errorf("local_node_id is required")
	}
	var local models.Listener
	if err := s.db.First(&local, in.LocalNodeID).Error; err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.NewName)
	if name == "" {
		name = local.Name + "-remote"
	}
	port := strings.TrimSpace(in.NewPort)
	if port == "" {
		port = local.Port
	}
	// Config is stored as JSON text; pass through as string so remote Unmarshal works.
	payload := map[string]interface{}{
		"name":         name,
		"protocol":     local.Protocol,
		"port":         port,
		"bind_address": local.BindAddress,
		"listen":       local.Listen,
		"enabled":      false, // always create disabled for safety
		"udp":          local.UDP,
		"tls":          local.TLS,
		"config":       local.Config,
		"public_host":  local.PublicHost,
		"public_port":  local.PublicPort,
		"access_sni":   local.AccessSNI,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"dry_run":       in.DryRun,
		"remote_id":     remoteID,
		"local_node_id": in.LocalNodeID,
		"payload":       payload,
	}
	if in.DryRun {
		result["status"] = "dry-run"
		return result, nil
	}
	status, raw, err := s.ProxyRemote(remoteID, http.MethodPost, "/api/v1/nodes", body)
	if err != nil {
		return nil, err
	}
	result["http_status"] = status
	var remoteBody interface{}
	_ = json.Unmarshal(raw, &remoteBody)
	result["remote_response"] = remoteBody
	if status >= 300 {
		return result, fmt.Errorf("remote HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	result["status"] = "created"
	return result, nil
}
