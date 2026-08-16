package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// Notifier watches blocked-user transitions and pushes Telegram alerts.
type Notifier struct {
	db *gorm.DB

	mu          sync.Mutex
	lastBlocked map[uint]bool
}

func NewNotifier(db *gorm.DB) *Notifier {
	return &Notifier{db: db, lastBlocked: map[uint]bool{}}
}

// CheckAndNotify compares current blocked set with the previous tick.
func (n *Notifier) CheckAndNotify() {
	if n == nil || n.db == nil {
		return
	}
	client, settings, err := NewClientFromDB(n.db)
	if err != nil || client == nil {
		return
	}

	var users []models.ProxyUser
	if err := n.db.Find(&users).Error; err != nil {
		return
	}

	current := make(map[uint]bool, len(users))
	blockedNow := make([]models.ProxyUser, 0)
	for _, u := range users {
		if !user.IsCredentialActive(u) {
			current[u.ID] = true
			blockedNow = append(blockedNow, u)
		}
	}

	n.mu.Lock()
	prev := n.lastBlocked
	n.lastBlocked = current
	n.mu.Unlock()

	var messages []string
	for _, u := range blockedNow {
		if prev[u.ID] {
			continue
		}
		reason := blockReason(u)
		if reason == "expired" && !settings.NotifyOnExpiry {
			continue
		}
		if reason != "expired" && !settings.NotifyOnBlock {
			continue
		}
		messages = append(messages, fmt.Sprintf(
			"⛔ <b>User blocked</b>\nuser: <code>%s</code>\nreason: %s\ntime: %s",
			escapeHTML(u.Username), reason, time.Now().UTC().Format(time.RFC3339),
		))
	}
	if settings.NotifyOnUnblock {
		for id := range prev {
			if current[id] {
				continue
			}
			var u models.ProxyUser
			if err := n.db.First(&u, id).Error; err != nil {
				continue
			}
			messages = append(messages, fmt.Sprintf(
				"✅ <b>User unblocked</b>\nuser: <code>%s</code>\ntime: %s",
				escapeHTML(u.Username), time.Now().UTC().Format(time.RFC3339),
			))
		}
	}

	for _, msg := range messages {
		if err := client.SendText(msg); err != nil {
			log.Printf("telegram: notify failed: %v", err)
		}
	}
}

func blockReason(u models.ProxyUser) string {
	now := time.Now()
	if !u.Enabled {
		return "disabled"
	}
	if !u.ExpireTime.IsZero() && !u.ExpireTime.After(now) {
		return "expired"
	}
	if u.TrafficLimit > 0 && u.TrafficUsed >= u.TrafficLimit {
		return "traffic_limit"
	}
	return "blocked"
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
