package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

type remoteNodeDTO struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Port       string `json:"port"`
	PublicHost string `json:"public_host"`
	Enabled    bool   `json:"enabled"`
}

type remoteURIExport struct {
	Name       string   `json:"name"`
	Protocol   string   `json:"protocol"`
	URI        string   `json:"uri"`
	URIs       []string `json:"uris"`
	ClientYAML string   `json:"client_yaml"`
}

// SyncRemoteNodes pulls node list + share exports from a remote panel and
// upserts RemoteNodeMirror rows for local subscription merge.
func (s *Service) SyncRemoteNodes(remoteID uint) ([]models.RemoteNodeMirror, error) {
	status, raw, err := s.ProxyRemote(remoteID, http.MethodGet, "/api/v1/nodes", nil)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("remote nodes HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	nodes, err := parseRemoteNodeList(raw)
	if err != nil {
		return nil, fmt.Errorf("parse remote nodes: %w", err)
	}

	var server models.RemoteServer
	if err := s.db.First(&server, remoteID).Error; err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	seen := make(map[uint]struct{}, len(nodes))
	out := make([]models.RemoteNodeMirror, 0, len(nodes))

	for _, n := range nodes {
		if n.ID == 0 {
			continue
		}
		seen[n.ID] = struct{}{}
		mirror := models.RemoteNodeMirror{
			RemoteServerID: remoteID,
			RemoteNodeID:   n.ID,
			Name:           strings.TrimSpace(n.Name),
			Protocol:       strings.TrimSpace(n.Protocol),
			Port:           strings.TrimSpace(n.Port),
			PublicHost:     strings.TrimSpace(n.PublicHost),
			Enabled:        n.Enabled,
			LastSyncAt:     &now,
		}
		if mirror.Name == "" {
			mirror.Name = fmt.Sprintf("remote-%d-%d", remoteID, n.ID)
		}

		ustatus, uraw, uerr := s.ProxyRemote(remoteID, http.MethodGet, fmt.Sprintf("/api/v1/nodes/%d/uri", n.ID), nil)
		if uerr != nil {
			mirror.LastError = truncateErr(uerr.Error())
		} else if ustatus >= 300 {
			mirror.LastError = fmt.Sprintf("uri HTTP %d: %s", ustatus, truncateErr(string(uraw)))
		} else {
			var exp remoteURIExport
			if json.Unmarshal(uraw, &exp) == nil {
				mirror.ShareURI = strings.TrimSpace(exp.URI)
				if len(exp.URIs) > 0 {
					if b, mErr := json.Marshal(exp.URIs); mErr == nil {
						mirror.ShareURIsJSON = string(b)
					}
				}
				mirror.ClientYAML = strings.TrimSpace(exp.ClientYAML)
				if mirror.Protocol == "" {
					mirror.Protocol = strings.TrimSpace(exp.Protocol)
				}
				if mirror.Name == "" && exp.Name != "" {
					mirror.Name = exp.Name
				}
				mirror.LastError = ""
			} else {
				mirror.LastError = "uri response is not JSON"
			}
		}

		var existing models.RemoteNodeMirror
		qerr := s.db.Where("remote_server_id = ? AND remote_node_id = ?", remoteID, n.ID).First(&existing).Error
		if qerr == nil {
			existing.Name = mirror.Name
			existing.Protocol = mirror.Protocol
			existing.Port = mirror.Port
			existing.PublicHost = mirror.PublicHost
			existing.Enabled = mirror.Enabled
			existing.ShareURI = mirror.ShareURI
			existing.ShareURIsJSON = mirror.ShareURIsJSON
			existing.ClientYAML = mirror.ClientYAML
			existing.LastSyncAt = mirror.LastSyncAt
			existing.LastError = mirror.LastError
			if err := s.db.Save(&existing).Error; err != nil {
				return nil, err
			}
			existing.RemoteServerName = server.Name
			out = append(out, existing)
			continue
		}
		if err := s.db.Create(&mirror).Error; err != nil {
			return nil, err
		}
		mirror.RemoteServerName = server.Name
		out = append(out, mirror)
	}

	var all []models.RemoteNodeMirror
	if err := s.db.Where("remote_server_id = ?", remoteID).Find(&all).Error; err == nil {
		for _, m := range all {
			if _, ok := seen[m.RemoteNodeID]; !ok && m.Enabled {
				_ = s.db.Model(&models.RemoteNodeMirror{}).Where("id = ?", m.ID).Updates(map[string]interface{}{
					"enabled":    false,
					"last_error": "removed on remote",
				}).Error
			}
		}
	}

	log.Printf("cluster: synced %d node mirror(s) from remote %d (%s)", len(out), remoteID, server.Name)
	return out, nil
}

func parseRemoteNodeList(raw []byte) ([]remoteNodeDTO, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	var arr []remoteNodeDTO
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var wrap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	for _, key := range []string{"data", "items", "nodes", "listeners"} {
		if v, ok := wrap[key]; ok {
			var inner []remoteNodeDTO
			if err := json.Unmarshal(v, &inner); err == nil {
				return inner, nil
			}
		}
	}
	return nil, fmt.Errorf("unexpected nodes payload")
}

// ListMirroredNodes returns cached remote nodes (optionally filtered by server).
func (s *Service) ListMirroredNodes(remoteServerID uint) ([]models.RemoteNodeMirror, error) {
	q := s.db.Model(&models.RemoteNodeMirror{}).Order("remote_server_id asc, name asc")
	if remoteServerID > 0 {
		q = q.Where("remote_server_id = ?", remoteServerID)
	}
	var rows []models.RemoteNodeMirror
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	var servers []models.RemoteServer
	_ = s.db.Find(&servers).Error
	nameByID := map[uint]string{}
	for _, srv := range servers {
		nameByID[srv.ID] = srv.Name
	}
	for i := range rows {
		rows[i].RemoteServerName = nameByID[rows[i].RemoteServerID]
	}
	return rows, nil
}

// BindUserRemoteNodes replaces remote-node bindings for a local proxy user.
func (s *Service) BindUserRemoteNodes(userID uint, mirrorIDs []uint) error {
	var user models.ProxyUser
	if err := s.db.First(&user, userID).Error; err != nil {
		return err
	}
	desired := uniqueUint(mirrorIDs)
	return s.db.Transaction(func(tx *gorm.DB) error {
		if len(desired) > 0 {
			var count int64
			if err := tx.Model(&models.RemoteNodeMirror{}).Where("id IN ?", desired).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(desired)) {
				return fmt.Errorf("one or more remote node mirror ids do not exist")
			}
			if err := tx.Where("proxy_user_id = ? AND remote_node_mirror_id NOT IN ?", userID, desired).
				Delete(&models.ProxyUserRemoteNode{}).Error; err != nil {
				return err
			}
		} else if err := tx.Where("proxy_user_id = ?", userID).Delete(&models.ProxyUserRemoteNode{}).Error; err != nil {
			return err
		}
		for _, mid := range desired {
			var binding models.ProxyUserRemoteNode
			res := tx.Unscoped().Where("proxy_user_id = ? AND remote_node_mirror_id = ?", userID, mid).Find(&binding)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				if binding.DeletedAt.Valid {
					if err := tx.Unscoped().Model(&binding).Update("deleted_at", nil).Error; err != nil {
						return err
					}
				}
				continue
			}
			if err := tx.Create(&models.ProxyUserRemoteNode{
				ProxyUserID:        userID,
				RemoteNodeMirrorID: mid,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListUserRemoteNodeIDs returns mirror IDs bound to a proxy user.
func (s *Service) ListUserRemoteNodeIDs(userID uint) ([]uint, error) {
	var rows []models.ProxyUserRemoteNode
	if err := s.db.Where("proxy_user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.RemoteNodeMirrorID)
	}
	return ids, nil
}

func uniqueUint(ids []uint) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
