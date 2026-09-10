package telegram

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/config"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

// handleCommand dispatches a /command from a Telegram message. The returned
// string is sent as the reply. An empty string means the handler already sent
// its reply (e.g. via sendWithKeyboard) and the loop should not send again.

func (b *Bot) lang() string {
	s, _ := LoadSettings(b.db)
	return NormalizeLang(s.Language)
}

func (b *Bot) handleCommand(ctx commandContext) string {
	text := strings.TrimSpace(ctx.Text)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return helpText(b.lang())
	}
	cmd := strings.ToLower(parts[0])
	if i := strings.IndexByte(cmd, '@'); i >= 0 {
		cmd = cmd[:i]
	}
	cmd = strings.TrimPrefix(cmd, "/")
	switch cmd {
	case "start", "help", "帮助":
		return b.cmdStartHelp(ctx)
	case "id":
		return b.cmdID(ctx)
	case "usage", "用量":
		return b.cmdUsage(ctx)
	case "status", "状态":
		if !ctx.IsAdmin {
			return Tr(b.lang(), "perm_denied")
		}
		return b.cmdStatus()
	case "users", "用户":
		if !ctx.IsAdmin {
			return Tr(b.lang(), "perm_denied")
		}
		return b.cmdUsers()
	case "online", "在线":
		if !ctx.IsAdmin {
			return Tr(b.lang(), "perm_denied")
		}
		return b.cmdOnline()
	case "listeners", "nodes", "节点":
		if !ctx.IsAdmin {
			return Tr(b.lang(), "perm_denied")
		}
		return b.cmdListeners()
	case "traffic", "流量":
		if !ctx.IsAdmin {
			return Tr(b.lang(), "perm_denied")
		}
		return b.cmdTraffic()
	case "restart", "重启":
		if !ctx.IsAdmin {
			return Tr(b.lang(), "perm_denied")
		}
		return b.cmdRestart()
	case "deldepleted", "清理":
		if !ctx.IsAdmin {
			return Tr(b.lang(), "perm_denied")
		}
		return b.cmdDelDepleted()
	case "search", "查找":
		if !ctx.IsAdmin {
			return Tr(b.lang(), "perm_denied")
		}
		q := ""
		if len(parts) > 1 {
			q = strings.Join(parts[1:], " ")
		}
		return b.cmdSearch(q)
	case "backup", "备份":
		if !ctx.IsAdmin {
			return Tr(b.lang(), "perm_denied")
		}
		return b.cmdBackup()
	default:
		return Tr(b.lang(), "unknown_cmd")
	}
}

// permDeniedUserOnly is the reply sent to non-admin chats that try to invoke
// an admin-only command. It tells them which commands they CAN use.
// cmdStartHelp renders /start and /help. Admin chats receive the admin inline
// keyboard; bound users receive the user menu; unbound chats (which cannot
// reach this handler anyway) get the plain help text.
func (b *Bot) cmdStartHelp(ctx commandContext) string {
	help := helpText(b.lang())
	if !ctx.IsAdmin && !ctx.IsBound {
		return help
	}
	if b.tgClient == nil {
		return help
	}
	chatID := fmtInt64(ctx.ChatID)
	if ctx.IsAdmin {
		kb := buildAdminMenu(b.lang())
		if err := b.tgClient.sendWithKeyboard(chatID, help, kb); err != nil {
			// Fall back to plain text on API failure (e.g. keyboard rejected).
			return help
		}
		return ""
	}
	welcome := Tr(b.lang(), "user_welcome")
	if err := b.tgClient.sendWithKeyboard(chatID, welcome, buildUserMenu(b.lang())); err != nil {
		return welcome
	}
	return ""
}

// cmdID replies with the sender's Telegram user ID. Used by admins to discover
// their own ID for the chat_ids allowlist, and by users to share their ID with
// the admin for binding.
func (b *Bot) cmdID(ctx commandContext) string {
	return Trf(b.lang(), "your_id", ctx.FromID)
}

// cmdUsage branches on caller role:
//   - admin: /usage <keyword> searches proxy users (alias of /search)
//   - bound user: /usage shows their own traffic / expiry / sub URL
func (b *Bot) cmdUsage(ctx commandContext) string {
	if ctx.IsAdmin {
		parts := strings.Fields(ctx.Text)
		q := ""
		if len(parts) > 1 {
			q = strings.Join(parts[1:], " ")
		}
		if strings.TrimSpace(q) == "" {
			return Tr(b.lang(), "usage_admin_hint")
		}
		return b.cmdSearch(q)
	}
	return b.userUsageMessage(ctx.FromID)
}

// userUsageMessage renders the bound user's traffic / expiry / online status
// and subscription URL. Returns a friendly "not bound" notice when the FromID
// is not linked to any ProxyUser.
func (b *Bot) userUsageMessage(fromID int64) string {
	if fromID == 0 || b.userSvc == nil {
		return notBoundMessage(b.lang(), fromID)
	}
	u, err := b.userSvc.GetByTelegramID(fromID)
	if err != nil || u == nil {
		return notBoundMessage(b.lang(), fromID)
	}
	return formatUserUsage(b.lang(), u)
}

// cmdMyUsage is the callback_query variant — always renders the bound user's
// usage regardless of admin role, since admins tapping the user menu expect
// their own bound-account view.
func (b *Bot) cmdMyUsage(ctx commandContext) string {
	return b.userUsageMessage(ctx.FromID)
}

// cmdMySub returns (and lazily ensures) the bound user's subscription URL.
func (b *Bot) cmdMySub(ctx commandContext) string {
	if ctx.FromID == 0 || b.userSvc == nil {
		return notBoundMessage(b.lang(), ctx.FromID)
	}
	u, err := b.userSvc.GetByTelegramID(ctx.FromID)
	if err != nil || u == nil {
		return notBoundMessage(b.lang(), ctx.FromID)
	}
	token := u.SubToken
	if strings.TrimSpace(token) == "" {
		if t, err := b.userSvc.EnsureSubToken(u.ID); err == nil {
			token = t
		}
	}
	if strings.TrimSpace(token) == "" {
		return Tr(b.lang(), "sub_fail")
	}
	return Trf(b.lang(), "sub_link", buildSubURL(token))
}

func notBoundMessage(lang string, fromID int64) string {
	return Trf(lang, "not_bound", fromID)
}

// formatUserUsage builds the per-user usage report shown by /usage and the
// my_usage callback.
func formatUserUsage(lang string, u *models.ProxyUser) string {
	var bld strings.Builder
	bld.WriteString(Tr(lang, "my_usage_title") + "\n")
	bld.WriteString(Trf(lang, "user_label", escapeHTML(u.Username)) + "\n")
	used := formatBytes(u.TrafficUsed)
	limit := "∞"
	if u.TrafficLimit > 0 {
		limit = formatBytes(u.TrafficLimit)
	}
	bld.WriteString(Trf(lang, "traffic_label", used, limit) + "\n")
	bld.WriteString(Trf(lang, "up_down", formatBytes(u.UploadBytes), formatBytes(u.DownloadBytes)) + "\n")
	if !u.ExpireTime.IsZero() {
		bld.WriteString(Trf(lang, "expires", u.ExpireTime.Format("2006-01-02 15:04")) + "\n")
	} else {
		bld.WriteString(Tr(lang, "expires_never") + "\n")
	}
	online := Tr(lang, "status_offline")
	if u.Online {
		online = Tr(lang, "status_online")
	}
	bld.WriteString(online + "\n")
	if strings.TrimSpace(u.SubToken) != "" {
		bld.WriteString(Trf(lang, "sub_line", buildSubURL(u.SubToken)) + "\n")
	} else {
		bld.WriteString(Tr(lang, "sub_missing") + "\n")
	}
	return bld.String()
}

// buildSubURL constructs a public client subscription URL from the configured
// server.public_url (env-overridden via ApplyEnvOverrides) or falls back to
// localhost so the bot can resolve a URL outside any HTTP request context.
func buildSubURL(token string) string {
	base := "http://localhost:8080"
	if config.GlobalConfig != nil && strings.TrimSpace(config.GlobalConfig.Server.PublicURL) != "" {
		base = strings.TrimRight(strings.TrimSpace(config.GlobalConfig.Server.PublicURL), "/")
	}
	return fmt.Sprintf("%s/api/v1/client/sub/%s", base, url.PathEscape(token))
}

func fmtInt64(n int64) string {
	return fmt.Sprintf("%d", n)
}

// handleCallback dispatches a callback_query.data string. Admin-only callbacks
// (status/users/...) are gated by IsAdmin; my_usage/my_sub work for any
// authorised chat (admin or bound user). Empty data → no-op.
func (b *Bot) handleCallback(ctx commandContext, data string) string {
	data = strings.TrimSpace(data)
	if data == "" {
		return ""
	}
	// Admin-only callbacks
	if ctx.IsAdmin {
		switch data {
		case "status":
			return b.cmdStatus()
		case "users":
			return b.cmdUsers()
		case "traffic":
			return b.cmdTraffic()
		case "online":
			return b.cmdOnline()
		case "listeners":
			return b.cmdListeners()
		case "deldepleted":
			return b.cmdDelDepleted()
		case "backup":
			return b.cmdBackup()
		case "restart":
			return b.cmdRestart()
		}
	}
	// User callbacks (also available to admins who happen to be bound)
	switch data {
	case "my_usage":
		return b.cmdMyUsage(ctx)
	case "my_sub":
		return b.cmdMySub(ctx)
	}
	return Tr(b.lang(), "unknown_action")
}

func helpText(lang string) string {
	return strings.TrimSpace(Tr(lang, "help"))
}

func (b *Bot) cmdStatus() string {
	running := false
	version := "-"
	pid := 0
	if b.mihomo != nil {
		st, err := b.mihomo.GetStatus()
		if err == nil && st != nil {
			running = st.Running
			version = st.Version
			pid = st.PID
		}
	}
	var userCount, blocked, online, listeners int64
	var dbWarn string
	if b.db != nil {
		if err := b.db.Model(&models.ProxyUser{}).Count(&userCount).Error; err != nil {
			dbWarn = fmt.Sprintf("\n⚠️ DB error: %s", escapeHTML(err.Error()))
		}
		var users []models.ProxyUser
		if err := b.db.Find(&users).Error; err != nil {
			dbWarn = fmt.Sprintf("\n⚠️ DB error: %s", escapeHTML(err.Error()))
		}
		for _, u := range users {
			if !user.IsCredentialActive(u) {
				blocked++
			}
			if u.Online {
				online++
			}
		}
		if err := b.db.Model(&models.Listener{}).Count(&listeners).Error; err != nil {
			dbWarn = fmt.Sprintf("\n⚠️ DB error: %s", escapeHTML(err.Error()))
		}
	}
	core := Tr(b.lang(), "core_stopped")
	if running {
		core = Tr(b.lang(), "core_running")
	}
	return fmt.Sprintf(
		Tr(b.lang(), "status_title"),
		core, escapeHTML(version), pid, userCount, online, blocked, listeners, dbWarn,
	)
}

func (b *Bot) cmdUsers() string {
	var users []models.ProxyUser
	if err := b.db.Order("id asc").Limit(40).Find(&users).Error; err != nil {
		return Trf(b.lang(), "users_fail", escapeHTML(err.Error()))
	}
	if len(users) == 0 {
		return Tr(b.lang(), "users_empty")
	}
	var bld strings.Builder
	bld.WriteString(Tr(b.lang(), "users_title") + "\n")
	for _, u := range users {
		flag := "✅"
		if !user.IsCredentialActive(u) {
			flag = "⛔"
		} else if u.Online {
			flag = "🟢"
		}
		used := formatBytes(u.TrafficUsed)
		limit := "∞"
		if u.TrafficLimit > 0 {
			limit = formatBytes(u.TrafficLimit)
		}
		bld.WriteString(fmt.Sprintf("%s <code>%s</code> %s/%s\n", flag, escapeHTML(u.Username), used, limit))
	}
	return bld.String()
}

func (b *Bot) cmdOnline() string {
	var users []models.ProxyUser
	if err := b.db.Where("online = ?", true).Order("id asc").Find(&users).Error; err != nil {
		return Trf(b.lang(), "users_fail", escapeHTML(err.Error()))
	}
	if len(users) == 0 {
		return Tr(b.lang(), "online_empty")
	}
	var bld strings.Builder
	bld.WriteString(Tr(b.lang(), "online_title") + "\n")
	for _, u := range users {
		bld.WriteString(fmt.Sprintf("• <code>%s</code>\n", escapeHTML(u.Username)))
	}
	return bld.String()
}

func (b *Bot) cmdListeners() string {
	var list []models.Listener
	if err := b.db.Order("id asc").Limit(40).Find(&list).Error; err != nil {
		return Trf(b.lang(), "users_fail", escapeHTML(err.Error()))
	}
	if len(list) == 0 {
		return Tr(b.lang(), "listeners_empty")
	}
	var bld strings.Builder
	bld.WriteString(Tr(b.lang(), "listeners_title") + "\n")
	for _, n := range list {
		en := "off"
		if n.Enabled {
			en = "on"
		}
		bld.WriteString(fmt.Sprintf("• <code>%s</code> %s:%s [%s]\n", escapeHTML(n.Name), n.Protocol, n.Port, en))
	}
	return bld.String()
}

func (b *Bot) cmdTraffic() string {
	var users []models.ProxyUser
	var dbWarn string
	if err := b.db.Order("traffic_used desc").Limit(15).Find(&users).Error; err != nil {
		dbWarn = fmt.Sprintf("\n⚠️ DB error: %s", escapeHTML(err.Error()))
	}
	var total int64
	for _, u := range users {
		total += u.TrafficUsed
	}
	var bld strings.Builder
	bld.WriteString(Trf(b.lang(), "traffic_title", formatBytes(total)))
	for _, u := range users {
		bld.WriteString(fmt.Sprintf("• <code>%s</code> ↑%s ↓%s\n",
			escapeHTML(u.Username), formatBytes(u.UploadBytes), formatBytes(u.DownloadBytes)))
	}
	if len(users) == 0 {
		bld.WriteString(Tr(b.lang(), "traffic_empty"))
	}
	if dbWarn != "" {
		bld.WriteString(dbWarn)
	}
	return bld.String()
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

func (b *Bot) cmdRestart() string {
	if b.mihomo == nil {
		return Tr(b.lang(), "mihomo_nil")
	}
	if err := b.mihomo.RestartMihomo(); err != nil {
		return Trf(b.lang(), "restart_fail", escapeHTML(err.Error()))
	}
	return Tr(b.lang(), "restart_ok")
}

func (b *Bot) cmdDelDepleted() string {
	svc := user.NewService(b.db)
	n, err := svc.DeleteDepleted()
	if err != nil {
		return Trf(b.lang(), "cleanup_fail", escapeHTML(err.Error()))
	}
	return Trf(b.lang(), "cleanup_ok", n)
}

func (b *Bot) cmdSearch(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return Tr(b.lang(), "search_usage")
	}
	svc := user.NewService(b.db)
	users, err := svc.ListFiltered(user.ListFilter{Query: q})
	if err != nil {
		return Trf(b.lang(), "search_fail", escapeHTML(err.Error()))
	}
	if len(users) == 0 {
		return Tr(b.lang(), "search_empty")
	}
	var bld strings.Builder
	bld.WriteString(Trf(b.lang(), "search_title", escapeHTML(q), len(users)))
	for i, u := range users {
		if i >= 20 {
			bld.WriteString("…\n")
			break
		}
		flag := "✅"
		if !user.IsCredentialActive(u) {
			flag = "⛔"
		} else if u.Online {
			flag = "🟢"
		}
		bld.WriteString(fmt.Sprintf("%s <code>%s</code> used=%s\n", flag, escapeHTML(u.Username), formatBytes(u.TrafficUsed)))
	}
	return bld.String()
}

func (b *Bot) cmdBackup() string {
	return strings.TrimSpace(Tr(b.lang(), "backup"))
}
