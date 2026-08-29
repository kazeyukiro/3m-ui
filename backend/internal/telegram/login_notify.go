package telegram

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// NotifyLogin sends an optional panel-login (success) alert. It fires only when
// the broad NotifyOnLogin toggle is on AND the fine-grained "login" event is
// allowlisted via EnabledEvents. The two gates together preserve the existing
// admin-controlled silence of success spam while adding the per-event veto
// introduced by tg-1's EventEnabled helper.
func NotifyLogin(db *gorm.DB, username, clientIP string) {
	if db == nil {
		return
	}
	client, settings, err := NewClientFromDB(db)
	if err != nil || client == nil || !settings.NotifyOnLogin || !settings.EventEnabled("login") {
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

// NotifyLoginFailed sends an alert on a failed panel-login attempt. Unlike
// NotifyLogin, this is gated solely by EventEnabled("login") and does NOT
// require NotifyOnLogin — a failed login is operationally more interesting
// than a successful one, so an admin may want failure alerts even when the
// chatty success path is disabled. The single shared event gate still keeps
// the class fully mute-able when desired.
func NotifyLoginFailed(db *gorm.DB, username, clientIP string) {
	if db == nil {
		return
	}
	client, settings, err := NewClientFromDB(db)
	if err != nil || client == nil || !settings.EventEnabled("login") {
		return
	}
	msg := fmt.Sprintf(
		"❌ <b>登录失败 / Login failed</b>\n用户 / User：<code>%s</code>\nIP：<code>%s</code>\n时间 / Time：%s",
		escapeHTML(username), escapeHTML(clientIP), time.Now().Format("2006-01-02 15:04:05"),
	)
	if err := client.SendText(msg); err != nil {
		log.Printf("telegram: login failed notification failed: %v", err)
	}
}
