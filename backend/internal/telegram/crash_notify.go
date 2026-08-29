package telegram

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// NotifyCrash fires when the mihomo core process exits unexpectedly (i.e. not
// via an admin-initiated Stop). It is wired through
// mihomo.Service.SetCrashHandler in app/container.go. Gated by
// settings.EventEnabled("crash"). The message notes that auto-restart will be
// attempted because waitProcess independently retries Start() after a crash —
// a successful restart is reported separately by the next status poll, while a
// failed restart surfaces via the mihomo logs API.
func NotifyCrash(db *gorm.DB, exitErr error) {
	if db == nil {
		return
	}
	client, settings, err := NewClientFromDB(db)
	if err != nil || client == nil || !settings.EventEnabled("crash") {
		return
	}
	errText := "unknown"
	if exitErr != nil {
		errText = exitErr.Error()
	}
	msg := fmt.Sprintf(
		"💥 <b>核心崩溃 / Core crashed</b>\n错误 / Error：<code>%s</code>\n时间 / Time：%s\nℹ️ 将尝试自动重启 / Auto-restart will be attempted",
		escapeHTML(errText), time.Now().Format("2006-01-02 15:04:05"),
	)
	if err := client.SendText(msg); err != nil {
		log.Printf("telegram: crash notification failed: %v", err)
	}
}
