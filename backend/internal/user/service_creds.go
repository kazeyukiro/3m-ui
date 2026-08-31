package user

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/credentials"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func (s *Service) ActiveCredentialsByListener() (map[uint][]Credential, error) {
	result := make(map[uint][]Credential)
	var listeners []models.Listener
	if err := s.db.Where("enabled = ?", true).Find(&listeners).Error; err != nil {
		return nil, err
	}
	var rows []struct {
		ListenerID  uint
		ProxyUserID uint
	}
	if err := s.db.Model(&models.ListenerUser{}).
		Select("listener_id, proxy_user_id").
		Where("deleted_at IS NULL").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	boundUsers := make(map[uint][]uint)
	userIDSet := make(map[uint]struct{})
	for _, row := range rows {
		boundUsers[row.ListenerID] = append(boundUsers[row.ListenerID], row.ProxyUserID)
		userIDSet[row.ProxyUserID] = struct{}{}
	}
	// Batch-load every bound proxy user in a single query instead of issuing
	// one SELECT per (listener, user) pair. The previous N+1 pattern made
	// ActiveCredentialsByListener scale poorly with the number of bindings
	// and was visible in profiles of config regeneration on large panels.
	userIDs := make([]uint, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}
	usersByID := make(map[uint]*models.ProxyUser, len(userIDs))
	if len(userIDs) > 0 {
		var users []models.ProxyUser
		if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, err
		}
		for i := range users {
			usersByID[users[i].ID] = &users[i]
		}
	}
	for _, listener := range listeners {
		if ids, hasBindings := boundUsers[listener.ID]; hasBindings {
			for _, userID := range ids {
				u, ok := usersByID[userID]
				if !ok {
					// Binding row referenced a proxy user that no longer exists
					// (soft-deleted or hard-deleted out from under us). Skip it
					// instead of failing the whole regeneration.
					continue
				}
				if !IsCredentialActive(*u) {
					continue
				}
				password, err := decryptPassword(u.PasswordEncrypted)
				if err != nil {
					return nil, fmt.Errorf("decrypt proxy user %d password: %w", u.ID, err)
				}
				result[listener.ID] = append(result[listener.ID], Credential{
					Username: u.Username,
					Password: password,
					UUID:     u.UUID,
				})
			}
			continue
		}
		before := listener.Config
		if err := credentials.EnsureListenerCredentials(&listener); err != nil {
			return nil, fmt.Errorf("prepare listener %q credentials: %w", listener.Name, err)
		}
		if listener.Config != before {
			if err := s.db.Model(&models.Listener{}).Where("id = ?", listener.ID).Update("config", listener.Config).Error; err != nil {
				return nil, fmt.Errorf("save generated credentials for listener %q: %w", listener.Name, err)
			}
		}
		if creds := credentialsFromListenerConfig(listener.Protocol, listener.Config); len(creds) > 0 {
			result[listener.ID] = creds
		}
	}
	return result, nil
}

func credentialsFromListenerConfig(protocol, raw string) []Credential {
	var cfg map[string]interface{}
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &cfg) != nil {
		return nil
	}
	users, ok := cfg["users"]
	if !ok {
		return nil
	}
	result := []Credential{}
	switch protocol {
	case "anytls", "hysteria2", "mieru", "tuic":
		if m, ok := users.(map[string]interface{}); ok {
			for username, value := range m {
				result = append(result, Credential{Username: username, Password: fmt.Sprint(value), UUID: username})
			}
		}
	default:
		if list, ok := users.([]interface{}); ok {
			for _, value := range list {
				row, ok := value.(map[string]interface{})
				if !ok {
					continue
				}
				result = append(result, Credential{Username: fmt.Sprint(row["username"]), Password: fmt.Sprint(row["password"]), UUID: fmt.Sprint(row["uuid"])})
			}
		}
	}
	return result
}
