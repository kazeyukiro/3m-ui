package user

import (
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// QuickCreateInput is the minimal payload for one-click user creation.
type QuickCreateInput struct {
	// Username optional; empty → auto "u" + random hex.
	Username string `json:"username"`
	// BindAllListeners attaches every local listener to the new user.
	BindAllListeners bool   `json:"bind_all_listeners"`
	Remark           string `json:"remark"`
}

// QuickCreateResult returns the safe user plus one-time plaintext secrets.
type QuickCreateResult struct {
	User     SafeUser `json:"user"`
	Password string   `json:"password"`
	UUID     string   `json:"uuid"`
}

// QuickCreate builds a working proxy user with generated credentials.
func (s *Service) QuickCreate(in QuickCreateInput) (*QuickCreateResult, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		hex, err := randomHex(4)
		if err != nil {
			return nil, fmt.Errorf("generate username: %w", err)
		}
		username = "u" + hex
		// rare collision: retry a few times
		for i := 0; i < 5; i++ {
			var n int64
			if err := s.db.Model(&models.ProxyUser{}).Where("username = ?", username).Count(&n).Error; err != nil {
				return nil, err
			}
			if n == 0 {
				break
			}
			hex, err = randomHex(4)
			if err != nil {
				return nil, err
			}
			username = "u" + hex
		}
	}
	password, err := randomToken(18)
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}
	uuid, err := newUUID()
	if err != nil {
		return nil, err
	}
	u, err := s.Create(CreateInput{
		Username: username,
		Password: password,
		UUID:     uuid,
		Remark:   strings.TrimSpace(in.Remark),
	})
	if err != nil {
		return nil, err
	}
	if in.BindAllListeners {
		var ids []uint
		if err := s.db.Model(&models.Listener{}).Pluck("id", &ids).Error; err != nil {
			return nil, fmt.Errorf("user created but list listeners failed: %w", err)
		}
		if len(ids) > 0 {
			if err := s.BindListeners(u.ID, ids); err != nil {
				return nil, fmt.Errorf("user created but bind listeners failed: %w", err)
			}
		}
	}
	return &QuickCreateResult{
		User:     ToSafeUser(u),
		Password: password,
		UUID:     uuid,
	}, nil
}
