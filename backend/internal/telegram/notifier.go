package telegram

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

type Notifier struct {
	db          *gorm.DB
	mu          sync.Mutex
	initialized bool
	lastBlocked map[uint]bool
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
	now := time.Now()
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
	n.mu.Unlock()
	if !firstRun {
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
			msg := fmt.Sprintf("⛔ <b>用户已被阻止 / User blocked</b>\n用户 / User：<code>%s</code>\n原因 / Reason：%s\n时间 / Time：%s", escapeHTML(u.Username), reasonText(reason), now.Format("2006-01-02 15:04:05"))
			if err := client.SendText(msg); err != nil {
				log.Printf("telegram: block notification failed: %v", err)
			}
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
				msg := fmt.Sprintf("✅ <b>用户已恢复 / User restored</b>\n用户 / User：<code>%s</code>\n时间 / Time：%s", escapeHTML(u.Username), now.Format("2006-01-02 15:04:05"))
				if err := client.SendText(msg); err != nil {
					log.Printf("telegram: unblock notification failed: %v", err)
				}
			}
		}
	}
	n.emitThresholdWarnings(client, settings, users, now)
	n.emitCPUWarning(client, settings)
	n.emitMemoryWarning(client, settings)
	if settings.NotifyDailyDigest {
		today := now.Format("2006-01-02")
		if !dailyDigestSent(n.db, today) {
			if err := client.SendText(dailyDigest(users, now)); err != nil {
				log.Printf("telegram: daily digest failed: %v", err)
			} else if err := markDailyDigestSent(n.db, today); err != nil {
				log.Printf("telegram: persist daily digest state failed: %v", err)
			}
		}
	}
}

func dailyDigest(users []models.ProxyUser, now time.Time) string {
	var total, used, blocked int64
	for _, u := range users {
		total++
		used += u.TrafficUsed
		if !user.IsCredentialActive(u) {
			blocked++
		}
	}
	return fmt.Sprintf("📊 <b>3m-ui 每日摘要 / Daily Summary</b>\n用户数 / Users：%d\n已阻止 / Blocked：%d\n累计流量 / Traffic：%s\n时间 / Time：%s", total, blocked, formatBytes(used), now.Format("2006-01-02 15:04:05"))
}

func reasonText(reason string) string {
	switch reason {
	case "disabled":
		return "用户已禁用 / Disabled"
	case "expired":
		return "已过期 / Expired"
	case "traffic_limit":
		return "流量已用尽 / Traffic limit reached"
	default:
		return "凭据不可用 / Credentials unavailable"
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
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}

func (n *Notifier) emitThresholdWarnings(client *Client, settings Settings, users []models.ProxyUser, now time.Time) {
	if client == nil {
		return
	}
	warnPct := settings.TrafficWarnPct
	if warnPct <= 0 || warnPct > 100 {
		warnPct = 80
	}
	warnHours := settings.ExpiryWarnHours
	if warnHours <= 0 {
		warnHours = 72
	}
	// gigabyte is the unit used by the TrafficWarnGB absolute-remaining check.
	const gigabyte int64 = 1024 * 1024 * 1024
	for _, u := range users {
		if !u.Enabled {
			continue
		}
		// Existing percent-based traffic warning (default 80%).
		if settings.NotifyOnTraffic && u.TrafficLimit > 0 {
			pct := float64(u.TrafficUsed) * 100 / float64(u.TrafficLimit)
			if pct >= float64(warnPct) && pct < 100 {
				key := fmt.Sprintf("tg_warn_traffic_%d_%s", u.ID, now.Format("2006-01-02"))
				if !settingExists(n.db, key) {
					msg := fmt.Sprintf("⚠️ <b>流量预警 / Traffic warning</b>\n用户 / User：<code>%s</code>\n已用 / Used：%s / %s (%.0f%%)\n阈值 / Threshold：%d%%\n时间 / Time：%s",
						escapeHTML(u.Username), formatBytes(u.TrafficUsed), formatBytes(u.TrafficLimit), pct, warnPct, now.Format("2006-01-02 15:04:05"))
					if err := client.SendText(msg); err != nil {
						log.Printf("telegram: traffic warn failed: %v", err)
					} else {
						_ = markSetting(n.db, key, "1")
					}
				}
			}
		}
		// Secondary traffic check: absolute-GB reminder. Fires when the
		// remaining traffic drops below TrafficWarnGB regardless of the
		// percent threshold — useful for users with very large quotas who
		// would otherwise never trip the 80% gate until it is too late.
		// Gated by EventEnabled("traffic") and deduped per user per day.
		if settings.TrafficWarnGB > 0 && settings.EventEnabled("traffic") && u.TrafficLimit > 0 {
			remaining := u.TrafficLimit - u.TrafficUsed
			if remaining < int64(settings.TrafficWarnGB)*gigabyte {
				key := fmt.Sprintf("tg_warn_traffic_gb_%d_%s", u.ID, now.Format("2006-01-02"))
				if !settingExists(n.db, key) {
					msg := fmt.Sprintf("📦 <b>流量低 / Low traffic</b>\n用户 / User：<code>%s</code>\n剩余 / Remaining：%s / %s\n阈值 / Threshold：%dGB\n时间 / Time：%s",
						escapeHTML(u.Username), formatBytes(remaining), formatBytes(u.TrafficLimit), settings.TrafficWarnGB, now.Format("2006-01-02 15:04:05"))
					if err := client.SendText(msg); err != nil {
						log.Printf("telegram: traffic GB warn failed: %v", err)
					} else {
						_ = markSetting(n.db, key, "1")
					}
				}
			}
		}
		// Existing hour-based expiry warning (default 72h).
		if settings.NotifyOnExpiry && !u.ExpireTime.IsZero() && u.ExpireTime.After(now) {
			until := u.ExpireTime.Sub(now)
			if until <= time.Duration(warnHours)*time.Hour {
				key := fmt.Sprintf("tg_warn_expire_%d_%s", u.ID, u.ExpireTime.Format("2006-01-02"))
				if !settingExists(n.db, key) {
					msg := fmt.Sprintf("⏰ <b>到期预警 / Expiry warning</b>\n用户 / User：<code>%s</code>\n到期 / Expires：%s\n剩余 / Left：%s\n时间 / Time：%s",
						escapeHTML(u.Username), u.ExpireTime.Format("2006-01-02 15:04"), until.Round(time.Hour).String(), now.Format("2006-01-02 15:04:05"))
					if err := client.SendText(msg); err != nil {
						log.Printf("telegram: expiry warn failed: %v", err)
					} else {
						_ = markSetting(n.db, key, "1")
					}
				}
			}
		}
		// Secondary expiry check: day-granular reminder. Fires N days before
		// expiry (ExpiryWarnDays, 0 = disabled). This is broader than the
		// 72h-hourly window — useful for catching upcoming renewals a week
		// or two ahead. Gated by EventEnabled("expiry") and deduped per
		// user per expiry-date (so a re-warn only fires if expiry changes).
		if settings.ExpiryWarnDays > 0 && settings.EventEnabled("expiry") && !u.ExpireTime.IsZero() && u.ExpireTime.After(now) {
			daysLeft := int(u.ExpireTime.Sub(now).Hours() / 24)
			if daysLeft <= settings.ExpiryWarnDays {
				key := fmt.Sprintf("tg_warn_expire_days_%d_%s", u.ID, u.ExpireTime.Format("2006-01-02"))
				if !settingExists(n.db, key) {
					msg := fmt.Sprintf("📅 <b>到期提醒 / Expiry reminder</b>\n用户 / User：<code>%s</code>\n到期 / Expires：%s\n剩余天数 / Days left：%d\n时间 / Time：%s",
						escapeHTML(u.Username), u.ExpireTime.Format("2006-01-02 15:04"), daysLeft, now.Format("2006-01-02 15:04:05"))
					if err := client.SendText(msg); err != nil {
						log.Printf("telegram: expiry days warn failed: %v", err)
					} else {
						_ = markSetting(n.db, key, "1")
					}
				}
			}
		}
	}
}

func settingExists(db *gorm.DB, key string) bool {
	if db == nil || key == "" {
		return false
	}
	var row models.PanelSetting
	return db.Where("key = ?", key).First(&row).Error == nil
}

func markSetting(db *gorm.DB, key, value string) error {
	if db == nil || key == "" {
		return nil
	}
	var row models.PanelSetting
	err := db.Where("key = ?", key).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return db.Create(&models.PanelSetting{Key: key, Value: value}).Error
	}
	if err != nil {
		return err
	}
	row.Value = value
	return db.Save(&row).Error
}

func (n *Notifier) emitCPUWarning(client *Client, settings Settings) {
	if client == nil || !settings.NotifyOnCPU || settings.CPUWarnPct <= 0 {
		return
	}
	if !settings.EventEnabled("cpu") {
		return
	}
	stats := systemStatsCPU()
	if stats < float64(settings.CPUWarnPct) {
		return
	}
	key := "telegram_cpu_warn_last"
	var row models.PanelSetting
	if err := n.db.Where("key = ?", key).First(&row).Error; err == nil {
		if ts, err := time.Parse(time.RFC3339, row.Value); err == nil && time.Since(ts) < time.Hour {
			return
		}
	}
	msg := fmt.Sprintf("🔥 <b>CPU high</b>\nusage: <code>%.1f%%</code> (threshold %d%%)", stats, settings.CPUWarnPct)
	if err := client.SendText(msg); err != nil {
		log.Printf("telegram: cpu warning failed: %v", err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var existing models.PanelSetting
	err := n.db.Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_ = n.db.Create(&models.PanelSetting{Key: key, Value: now}).Error
	} else if err == nil {
		existing.Value = now
		_ = n.db.Save(&existing).Error
	}
}

// emitMemoryWarning sends a high-memory alert when the host's used-percent
// crosses the configured threshold. Settings has no dedicated memory threshold
// field, so we deliberately reuse CPUWarnPct as the general "resource warn %"
// gate — one slider in the panel UI that means "warn me when CPU or RAM
// exceeds N%". Gated separately by EventEnabled("memory") so an admin can
// mute the memory class while keeping CPU alerts. The dedup key
// "telegram_memory_warn_last" carries a 1-hour cooldown identical to the CPU
// path so a sustained memory spike does not spam the chat on every scheduler
// tick.
func (n *Notifier) emitMemoryWarning(client *Client, settings Settings) {
	if client == nil || !settings.EventEnabled("memory") || settings.CPUWarnPct <= 0 {
		return
	}
	stats, err := memoryPercentSample()
	if err != nil || stats < float64(settings.CPUWarnPct) {
		return
	}
	key := "telegram_memory_warn_last"
	var row models.PanelSetting
	if err := n.db.Where("key = ?", key).First(&row).Error; err == nil {
		if ts, err := time.Parse(time.RFC3339, row.Value); err == nil && time.Since(ts) < time.Hour {
			return
		}
	}
	msg := fmt.Sprintf("🧠 <b>Memory high</b>\nusage: <code>%.1f%%</code> (threshold %d%%)", stats, settings.CPUWarnPct)
	if err := client.SendText(msg); err != nil {
		log.Printf("telegram: memory warning failed: %v", err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var existing models.PanelSetting
	err = n.db.Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_ = n.db.Create(&models.PanelSetting{Key: key, Value: now}).Error
	} else if err == nil {
		existing.Value = now
		_ = n.db.Save(&existing).Error
	}
}

func systemStatsCPU() float64 {
	percents, err := cpuPercentSample()
	if err != nil || len(percents) == 0 {
		return 0
	}
	return percents[0]
}
