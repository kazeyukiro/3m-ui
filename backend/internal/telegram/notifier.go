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

// Notifier watches credential state transitions and optionally sends a daily
// usage digest. The first observation only establishes the baseline so a
// restart does not spam alerts for users that were already blocked.
type Notifier struct {
	db *gorm.DB

	mu             sync.Mutex
	initialized    bool
	lastBlocked    map[uint]bool
	lastDigestDate string
}

func NewNotifier(db *gorm.DB) *Notifier {
	return &Notifier{db: db, lastBlocked: map[uint]bool{}}
}

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
	firstRun := !n.initialized
	n.initialized = true
	n.lastBlocked = current
	digestAlreadySent := n.lastDigestDate
	n.mu.Unlock()

	if !firstRun {
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
				"⛔ <b>用户已被阻止</b>\n用户：<code>%s</code>\n原因：%s\n时间：%s",
				escapeHTML(u.Username), reasonText(reason), time.Now().Format("2006-01-02 15:04:05"),
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
					"✅ <b>用户已恢复</b>\n用户：<code>%s</code>\n时间：%s",
					escapeHTML(u.Username), time.Now().Format("2006-01-02 15:04:05"),
				))
			}
		}
		for _, msg := range messages {
			if err := client.SendText(msg); err != nil {
				log.Printf("telegram: notify failed: %v", err)
			}
		}
	}

	if settings.NotifyDailyDigest {
		today := time.Now().Format("2006-01-02")
		if digestAlreadySent != today && time.Now().Hour() == 0 {
			if err := client.SendText(dailyDigest(users)); err != nil {
				log.Printf("telegram: daily digest failed: %v", err)
			} else {
				n.mu.Lock()
				n.lastDigestDate = today
				n.mu.Unlock()
			}
		}
	}
}

func dailyDigest(users []models.ProxyUser) string {
	var total, used, blocked int64
	for _, u := range users {
		total++
		used += u.TrafficUsed
		if !user.IsCredentialActive(u) {
			blocked++
		}
	}
	return fmt.Sprintf("📊 <b>3m-ui 每日摘要</b>\n用户数：%d\n已阻止：%d\n累计流量：%s\n时间：%s", total, blocked, formatBytes(used), time.Now().Format("2006-01-02 15:04:05"))
}

func reasonText(reason string) string {
	switch reason {
	case "disabled":
		return "用户已禁用"
	case "expired":
		return "已过期"
	case "traffic_limit":
		return "流量已用尽"
	default:
		return "凭据不可用"
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

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n) / 1024
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.2f %s", v, units[i])
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}
