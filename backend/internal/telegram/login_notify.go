package telegram

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// NotifyLogin sends an optional panel-login alert (3x-ui parity).
func NotifyLogin(db *gorm.DB, username, clientIP string) {
	if db == nil {
		return
	}
	client, settings, err := NewClientFromDB(db)
	if err != nil || client == nil || !settings.NotifyOnLogin {
		return
	}
	msg := fmt.Sprintf(
		"🔐 <b>面板登录 / Panel login</b>\n用户 / User：<code>%s</code>\nIP：<code>%s</code>\n时间 / Time：%s",
		escapeHTML(username), escapeHTML(clientIP), time.Now().Format("2006-01-02 15:04:05"),
	)
	if err := client.SendText(msg); err != nil {
		log.Printf("telegram: login notification failed: %v", err)
	}
}
