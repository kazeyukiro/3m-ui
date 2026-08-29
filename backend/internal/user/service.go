package user

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
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
	TelegramID   int64      `json:"telegram_id"`
	TelegramName string     `json:"telegram_name"`
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
	TelegramID   *int64     `json:"telegram_id"`
	TelegramName *string    `json:"telegram_name"`
}

type Credential struct{ Username, Password, UUID string }

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

// BindTelegram links a Telegram account (numeric chat/user ID + display name) to a proxy user.
func (s *Service) BindTelegram(userID uint, tgID int64, tgName string) error {
	if tgID == 0 {
		return errors.New("telegram id is required")
	}
	u, err := s.GetByID(userID)
	if err != nil {
		return err
	}
	u.TelegramID = tgID
	u.TelegramName = strings.TrimSpace(tgName)
	if err := s.db.Save(u).Error; err != nil {
		return fmt.Errorf("bind telegram: %w", err)
	}
	return nil
}

// UnbindTelegram removes the Telegram account link from a proxy user.
func (s *Service) UnbindTelegram(userID uint) error {
	u, err := s.GetByID(userID)
	if err != nil {
		return err
	}
	u.TelegramID = 0
	u.TelegramName = ""
	if err := s.db.Save(u).Error; err != nil {
		return fmt.Errorf("unbind telegram: %w", err)
	}
	return nil
}

// GetByTelegramID looks up a proxy user by their linked Telegram chat/user ID.
func (s *Service) GetByTelegramID(tgID int64) (*models.ProxyUser, error) {
	if tgID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var u models.ProxyUser
	if err := s.db.Where("telegram_id = ?", tgID).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}
