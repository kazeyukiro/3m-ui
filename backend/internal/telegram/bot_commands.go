package telegram

import (
	"fmt"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
)

func (b *Bot) handleCommand(text string) string {
	text = strings.TrimSpace(text)
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
		return helpText()
	case "status", "状态":
		return b.cmdStatus()
	case "users", "用户":
		return b.cmdUsers()
	case "online", "在线":
		return b.cmdOnline()
	case "listeners", "nodes", "节点":
		return b.cmdListeners()
	case "traffic", "流量":
		return b.cmdTraffic()
	case "restart", "重启":
		return b.cmdRestart()
	case "deldepleted", "清理":
		return b.cmdDelDepleted()
	case "search", "查找":
		q := ""
		if len(parts) > 1 {
			q = strings.Join(parts[1:], " ")
		}
		return b.cmdSearch(q)
	case "backup", "备份":
		return b.cmdBackup()
	default:
		return "未知指令。发送 /help 查看可用命令。"
	}
}

func helpText() string {
	return strings.TrimSpace(`
🤖 <b>3m-ui Bot</b>
/status — 核心与面板概览
/users — 代理用户列表（含封禁）
/online — 当前在线用户
/listeners — 入站节点列表
/traffic — 流量快照
/restart — 重启 Mihomo 核心
/deldepleted — 清理到期/超额用户
/search &lt;关键字&gt; — 按用户名/备注搜索
/backup — 备份提示
/help — 本帮助
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
	if b.db != nil {
		_ = b.db.Model(&models.ProxyUser{}).Count(&userCount).Error
		var users []models.ProxyUser
		_ = b.db.Find(&users).Error
		for _, u := range users {
			if !user.IsCredentialActive(u) {
				blocked++
			}
			if u.Online {
				online++
			}
		}
		_ = b.db.Model(&models.Listener{}).Count(&listeners).Error
	}
	core := "stopped"
	if running {
		core = "running"
	}
	return fmt.Sprintf(
		"📊 <b>Status</b>\ncore: <code>%s</code>\nversion: <code>%s</code>\npid: <code>%d</code>\nusers: %d (online %d, blocked %d)\nlisteners: %d",
		core, escapeHTML(version), pid, userCount, online, blocked, listeners,
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
	_ = b.db.Order("traffic_used desc").Limit(15).Find(&users).Error
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
