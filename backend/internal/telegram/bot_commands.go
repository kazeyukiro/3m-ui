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
func (b *Bot) handleCommand(ctx commandContext) string {
	text := strings.TrimSpace(ctx.Text)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return helpText()
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
			return permDeniedUserOnly
		}
		return b.cmdStatus()
	case "users", "用户":
		if !ctx.IsAdmin {
			return permDeniedUserOnly
		}
		return b.cmdUsers()
	case "online", "在线":
		if !ctx.IsAdmin {
			return permDeniedUserOnly
		}
		return b.cmdOnline()
	case "listeners", "nodes", "节点":
		if !ctx.IsAdmin {
			return permDeniedUserOnly
		}
		return b.cmdListeners()
	case "traffic", "流量":
		if !ctx.IsAdmin {
			return permDeniedUserOnly
		}
		return b.cmdTraffic()
	case "restart", "重启":
		if !ctx.IsAdmin {
			return permDeniedUserOnly
		}
		return b.cmdRestart()
	case "deldepleted", "清理":
		if !ctx.IsAdmin {
			return permDeniedUserOnly
		}
		return b.cmdDelDepleted()
	case "search", "查找":
		if !ctx.IsAdmin {
			return permDeniedUserOnly
		}
		q := ""
		if len(parts) > 1 {
			q = strings.Join(parts[1:], " ")
		}
		return b.cmdSearch(q)
	case "backup", "备份":
		if !ctx.IsAdmin {
			return permDeniedUserOnly
		}
		return b.cmdBackup()
	default:
		return "未知指令。发送 /help 查看可用命令。"
	}
}

// permDeniedUserOnly is the reply sent to non-admin chats that try to invoke
// an admin-only command. It tells them which commands they CAN use.
const permDeniedUserOnly = "⛔ 此命令仅管理员可用 / Admin-only command.\n可用命令 / Available: /id  /usage  /help"

// cmdStartHelp renders /start and /help. Admin chats receive the admin inline
// keyboard; bound users receive the user menu; unbound chats (which cannot
// reach this handler anyway) get the plain help text.
func (b *Bot) cmdStartHelp(ctx commandContext) string {
	help := helpText()
	if !ctx.IsAdmin && !ctx.IsBound {
		return help
	}
	if b.tgClient == nil {
		return help
	}
	chatID := fmtInt64(ctx.ChatID)
	if ctx.IsAdmin {
		kb := buildAdminMenu()
		if err := b.tgClient.sendWithKeyboard(chatID, help, kb); err != nil {
			// Fall back to plain text on API failure (e.g. keyboard rejected).
			return help
		}
		return ""
	}
	welcome := "👋 欢迎！点击下方按钮查看你的用量与订阅链接。\nWelcome! Tap a button below to view your usage & subscription."
	if err := b.tgClient.sendWithKeyboard(chatID, welcome, buildUserMenu()); err != nil {
		return welcome
	}
	return ""
}

// cmdID replies with the sender's Telegram user ID. Used by admins to discover
// their own ID for the chat_ids allowlist, and by users to share their ID with
// the admin for binding.
func (b *Bot) cmdID(ctx commandContext) string {
	return fmt.Sprintf("🆔 Your Telegram ID: <code>%d</code>", ctx.FromID)
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
			return "用法 / Usage: /usage &lt;关键字&gt; — 搜索代理用户 / search proxy users"
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
		return notBoundMessage(fromID)
	}
	u, err := b.userSvc.GetByTelegramID(fromID)
	if err != nil || u == nil {
		return notBoundMessage(fromID)
	}
	return formatUserUsage(u)
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
		return notBoundMessage(ctx.FromID)
	}
	u, err := b.userSvc.GetByTelegramID(ctx.FromID)
	if err != nil || u == nil {
		return notBoundMessage(ctx.FromID)
	}
	token := u.SubToken
	if strings.TrimSpace(token) == "" {
		if t, err := b.userSvc.EnsureSubToken(u.ID); err == nil {
			token = t
		}
	}
	if strings.TrimSpace(token) == "" {
		return "⚠️ 订阅链接生成失败, 请稍后重试 / Subscription token not available."
	}
	return fmt.Sprintf("🔗 <b>订阅链接 / Subscription URL</b>\n<code>%s</code>", buildSubURL(token))
}

func notBoundMessage(fromID int64) string {
	return fmt.Sprintf(
		"未绑定账户。请联系管理员将你的 Telegram ID (<code>%d</code>) 绑定到你的账户。\n/ Not bound. Ask admin to bind your Telegram ID.",
		fromID,
	)
}

// formatUserUsage builds the per-user usage report shown by /usage and the
// my_usage callback.
func formatUserUsage(u *models.ProxyUser) string {
	var bld strings.Builder
	bld.WriteString("📊 <b>账户用量 / My Usage</b>\n")
	bld.WriteString(fmt.Sprintf("用户 / User: <code>%s</code>\n", escapeHTML(u.Username)))
	used := formatBytes(u.TrafficUsed)
	limit := "∞"
	if u.TrafficLimit > 0 {
		limit = formatBytes(u.TrafficLimit)
	}
	bld.WriteString(fmt.Sprintf("流量 / Traffic: %s / %s\n", used, limit))
	bld.WriteString(fmt.Sprintf("上传 ↑ %s   下载 ↓ %s\n",
		formatBytes(u.UploadBytes), formatBytes(u.DownloadBytes)))
	if !u.ExpireTime.IsZero() {
		bld.WriteString(fmt.Sprintf("到期 / Expires: <code>%s</code>\n", u.ExpireTime.Format("2006-01-02 15:04")))
	} else {
		bld.WriteString("到期 / Expires: 永不过期 / Never\n")
	}
	online := "离线 / Offline"
	if u.Online {
		online = "在线 / Online 🟢"
	}
	bld.WriteString(fmt.Sprintf("状态 / Status: %s\n", online))
	if strings.TrimSpace(u.SubToken) != "" {
		bld.WriteString(fmt.Sprintf("订阅 / Subscription:\n<code>%s</code>\n", buildSubURL(u.SubToken)))
	} else {
		bld.WriteString("订阅 / Subscription: 未生成 / Not generated (点击 /click my_sub 生成)\n")
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
	return "未知操作 / Unknown action."
}

func helpText() string {
	return strings.TrimSpace(`
🤖 <b>3m-ui Bot</b>
/status — 核心与面板概览 / Panel & core overview
/users — 代理用户列表 / Proxy user list
/online — 当前在线用户 / Online users
/listeners — 入站节点列表 / Inbound listeners
/traffic — 流量快照 / Traffic snapshot
/restart — 重启 Mihomo 核心 / Restart core
/deldepleted — 清理到期/超额用户 / Delete depleted users
/search &lt;关键字&gt; — 按用户名/备注搜索 / Search by username/remark
/backup — 备份提示 / Backup hint
/id — 显示你的 Telegram ID / Show your Telegram ID
/usage — 查询用量 (用户) 或搜索 (管理员) / Usage (user) or search (admin)
/help — 本帮助 / This help
`)
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
	core := "stopped"
	if running {
		core = "running"
	}
	return fmt.Sprintf(
		"📊 <b>Status</b>\ncore: <code>%s</code>\nversion: <code>%s</code>\npid: <code>%d</code>\nusers: %d (online %d, blocked %d)\nlisteners: %d%s",
		core, escapeHTML(version), pid, userCount, online, blocked, listeners, dbWarn,
	)
}

func (b *Bot) cmdUsers() string {
	var users []models.ProxyUser
	if err := b.db.Order("id asc").Limit(40).Find(&users).Error; err != nil {
		return "读取用户失败: " + escapeHTML(err.Error())
	}
	if len(users) == 0 {
		return "暂无代理用户。"
	}
	var bld strings.Builder
	bld.WriteString("👥 <b>Users</b>\n")
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
		return "读取在线用户失败: " + escapeHTML(err.Error())
	}
	if len(users) == 0 {
		return "当前无在线用户。"
	}
	var bld strings.Builder
	bld.WriteString("🟢 <b>Online</b>\n")
	for _, u := range users {
		bld.WriteString(fmt.Sprintf("• <code>%s</code>\n", escapeHTML(u.Username)))
	}
	return bld.String()
}

func (b *Bot) cmdListeners() string {
	var list []models.Listener
	if err := b.db.Order("id asc").Limit(40).Find(&list).Error; err != nil {
		return "读取节点失败: " + escapeHTML(err.Error())
	}
	if len(list) == 0 {
		return "暂无节点。"
	}
	var bld strings.Builder
	bld.WriteString("📡 <b>Listeners</b>\n")
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
	bld.WriteString(fmt.Sprintf("📈 <b>Traffic</b> (top users)\napprox listed used sum: %s\n", formatBytes(total)))
	for _, u := range users {
		bld.WriteString(fmt.Sprintf("• <code>%s</code> ↑%s ↓%s\n",
			escapeHTML(u.Username), formatBytes(u.UploadBytes), formatBytes(u.DownloadBytes)))
	}
	if len(users) == 0 {
		bld.WriteString("暂无数据。")
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
		return "Mihomo 服务未初始化。"
	}
	if err := b.mihomo.RestartMihomo(); err != nil {
		return "重启失败: " + escapeHTML(err.Error())
	}
	return "✅ Mihomo 核心已重启。"
}

func (b *Bot) cmdDelDepleted() string {
	svc := user.NewService(b.db)
	n, err := svc.DeleteDepleted()
	if err != nil {
		return "清理失败: " + escapeHTML(err.Error())
	}
	return fmt.Sprintf("🧹 已删除 %d 个到期/超额用户。", n)
}

func (b *Bot) cmdSearch(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return "用法: /search &lt;用户名或备注关键字&gt;"
	}
	svc := user.NewService(b.db)
	users, err := svc.ListFiltered(user.ListFilter{Query: q})
	if err != nil {
		return "搜索失败: " + escapeHTML(err.Error())
	}
	if len(users) == 0 {
		return "未找到匹配用户。"
	}
	var bld strings.Builder
	bld.WriteString(fmt.Sprintf("🔍 <b>Search</b> <code>%s</code> (%d)\n", escapeHTML(q), len(users)))
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
	return strings.TrimSpace(`📦 <b>Backup</b>
请在面板「系统设置 → 备份」下载完整备份（SQLite + Mihomo 配置）。
Use panel Settings → Backup to download a full zip (database + Mihomo config).
API: <code>GET /api/v1/system/backup</code>`)
}
