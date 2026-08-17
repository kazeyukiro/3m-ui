package user

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/credentials"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/security"
	"gorm.io/gorm"
)

type Service struct {
	db                 *gorm.DB
	credentialsChanged func() error
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) SetCredentialsChangedHandler(fn func() error) {
	s.credentialsChanged = fn
}

func (s *Service) notifyCredentialsChanged() error {
	if s.credentialsChanged == nil {
		return nil
	}
	return s.credentialsChanged()
}

type CreateInput struct {
	Username     string     `json:"username" binding:"required"`
	Password     string     `json:"password"`
	UUID         string     `json:"uuid"`
	TrafficLimit int64      `json:"traffic_limit"`
	IPLimit      int        `json:"ip_limit"`
	Remark       string     `json:"remark"`
	ExpireTime   *time.Time `json:"expire_time"`
	Enabled      *bool      `json:"enabled"`
}

type UpdateInput struct {
	Username     string     `json:"username"`
	Password     string     `json:"password"`
	UUID         string     `json:"uuid"`
	TrafficLimit *int64     `json:"traffic_limit"`
	IPLimit      *int       `json:"ip_limit"`
	Remark       *string    `json:"remark"`
	ExpireTime   *time.Time `json:"expire_time"`
	Enabled      *bool      `json:"enabled"`
}

type Credential struct{ Username, Password, UUID string }

func (s *Service) Create(in CreateInput) (*models.ProxyUser, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return nil, errors.New("username is required")
	}
	password := in.Password
	var err error
	if password == "" {
		password, err = randomToken(24)
		if err != nil {
			return nil, fmt.Errorf("generate proxy user password: %w", err)
		}
	}
	uuid := in.UUID
	if uuid == "" {
		var err error
		uuid, err = newUUID()
		if err != nil {
			return nil, err
		}
	}
	expire := time.Time{}
	if in.ExpireTime != nil {
		expire = in.ExpireTime.UTC()
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	encrypted, err := encryptPassword(password)
	if err != nil {
		return nil, err
	}
	subTok, _ := randomHex(16)
	u := &models.ProxyUser{
		Username:          username,
		PasswordEncrypted: encrypted,
		UUID:              uuid,
		TrafficLimit:      in.TrafficLimit,
		IPLimit:           max0(in.IPLimit),
		Remark:            strings.TrimSpace(in.Remark),
		ExpireTime:        expire,
		Enabled:           enabled,
		SubToken:          subTok,
	}
	if err := s.db.Create(u).Error; err != nil {
		return nil, fmt.Errorf("create proxy user: %w", err)
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return u, fmt.Errorf("proxy user created, but Mihomo configuration could not be updated: %w", err)
	}
	return u, nil
}

func (s *Service) GetAll() ([]models.ProxyUser, error) {
	var users []models.ProxyUser
	if err := s.db.Order("id desc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (s *Service) GetByID(id uint) (*models.ProxyUser, error) {
	var u models.ProxyUser
	if err := s.db.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) Update(id uint, in UpdateInput) (*models.ProxyUser, error) {
	u, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Username) != "" {
		u.Username = strings.TrimSpace(in.Username)
	}
	if in.Password != "" {
		u.PasswordEncrypted, err = encryptPassword(in.Password)
		if err != nil {
			return nil, err
		}
	}
	if in.UUID != "" {
		u.UUID = in.UUID
	}
	if in.TrafficLimit != nil {
		u.TrafficLimit = *in.TrafficLimit
	}
	if in.IPLimit != nil {
		u.IPLimit = max0(*in.IPLimit)
	}
	if in.Remark != nil {
		u.Remark = strings.TrimSpace(*in.Remark)
	}
	if in.ExpireTime != nil {
		u.ExpireTime = in.ExpireTime.UTC()
	}
	if in.Enabled != nil {
		u.Enabled = *in.Enabled
	}
	if err := s.db.Save(u).Error; err != nil {
		return nil, fmt.Errorf("update proxy user: %w", err)
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return u, fmt.Errorf("proxy user updated, but Mihomo configuration could not be updated: %w", err)
	}
	return u, nil
}

func (s *Service) Delete(id uint) error {
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("proxy_user_id = ?", id).Delete(&models.ListenerUser{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.ProxyUser{}, id).Error
	}); err != nil {
		return err
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return fmt.Errorf("proxy user deleted, but Mihomo configuration could not be updated: %w", err)
	}
	return nil
}

func (s *Service) BindListeners(userID uint, listenerIDs []uint) error {
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var user models.ProxyUser
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		desired := make([]uint, 0, len(listenerIDs))
		seen := map[uint]struct{}{}
		for _, id := range listenerIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			desired = append(desired, id)
		}
		if len(desired) > 0 {
			var count int64
			if err := tx.Model(&models.Listener{}).Where("id IN ?", desired).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(desired)) {
				return errors.New("one or more listener ids do not exist")
			}
			if err := tx.Where("proxy_user_id = ? AND listener_id NOT IN ?", userID, desired).Delete(&models.ListenerUser{}).Error; err != nil {
				return err
			}
		} else if err := tx.Where("proxy_user_id = ?", userID).Delete(&models.ListenerUser{}).Error; err != nil {
			return err
		}
		for _, listenerID := range desired {
			var binding models.ListenerUser
			result := tx.Unscoped().Where("listener_id = ? AND proxy_user_id = ?", listenerID, userID).Find(&binding)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				if binding.DeletedAt.Valid {
					if err := tx.Unscoped().Model(&binding).Update("deleted_at", nil).Error; err != nil {
						return err
					}
				}
				continue
			}
			if err := tx.Create(&models.ListenerUser{ListenerID: listenerID, ProxyUserID: userID}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := s.notifyCredentialsChanged(); err != nil {
		return fmt.Errorf("listener bindings updated, but Mihomo configuration could not be updated: %w", err)
	}
	return nil
}

func (s *Service) GetListeners(userID uint) ([]models.Listener, error) {
	var listeners []models.Listener
	err := s.db.Model(&models.Listener{}).Joins("JOIN listener_users ON listener_users.listener_id = listeners.id AND listener_users.deleted_at IS NULL").Where("listener_users.proxy_user_id = ?", userID).Order("listeners.id").Find(&listeners).Error
	return listeners, err
}

func IsCredentialActive(u models.ProxyUser) bool {
	now := time.Now()
	if !u.Enabled {
		return false
	}
	if !u.ExpireTime.IsZero() && !u.ExpireTime.After(now) {
		return false
	}
	if u.TrafficLimit > 0 && u.TrafficUsed >= u.TrafficLimit {
		return false
	}
	return true
}

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
	for _, row := range rows {
		boundUsers[row.ListenerID] = append(boundUsers[row.ListenerID], row.ProxyUserID)
	}
	for _, listener := range listeners {
		if ids, hasBindings := boundUsers[listener.ID]; hasBindings {
			for _, userID := range ids {
				u, err := s.GetByID(userID)
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					return nil, err
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

func safeMask(s string) string {
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

type SafeUser struct {
	ID            uint       `json:"id"`
	Username      string     `json:"username"`
	UUIDMasked    string     `json:"uuid_masked"`
	TrafficLimit  int64      `json:"traffic_limit"`
	TrafficUsed   int64      `json:"traffic_used"`
	UploadBytes   int64      `json:"upload_bytes"`
	DownloadBytes int64      `json:"download_bytes"`
	LastSeen      *time.Time `json:"last_seen"`
	Online        bool       `json:"online"`
	ExpireTime    time.Time  `json:"expire_time"`
	Enabled       bool       `json:"enabled"`
	Blocked       bool       `json:"blocked"`
	IPLimit       int        `json:"ip_limit"`
	Remark        string     `json:"remark"`
	SubToken      string     `json:"sub_token"`
}

func ToSafeUser(u *models.ProxyUser) SafeUser {
	return SafeUser{
		ID:            u.ID,
		Username:      u.Username,
		UUIDMasked:    safeMask(u.UUID),
		TrafficLimit:  u.TrafficLimit,
		TrafficUsed:   u.TrafficUsed,
		UploadBytes:   u.UploadBytes,
		DownloadBytes: u.DownloadBytes,
		LastSeen:      u.LastSeen,
		Online:        u.Online,
		ExpireTime:    u.ExpireTime,
		Enabled:       u.Enabled,
		Blocked:       !IsCredentialActive(*u),
		IPLimit:       u.IPLimit,
		Remark:        u.Remark,
		SubToken:      u.SubToken,
	}
}

func encryptPassword(plain string) (string, error)   { return security.Encrypt(plain) }
func decryptPassword(encoded string) (string, error) { return security.Decrypt(encoded) }

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
