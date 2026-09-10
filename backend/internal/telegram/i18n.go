package telegram

import (
	"fmt"
	"strings"
)

// Supported bot language codes (aligned with the panel's 18 locales).
var SupportedBotLangs = []string{
	"en", "zh-CN", "zh-TW", "ja", "ko", "es", "fr", "de", "ru",
	"pt-BR", "vi", "id", "th", "tr", "ar", "hi", "pl", "uk",
}

// NormalizeLang maps panel / settings values onto SupportedBotLangs.
func NormalizeLang(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "en"
	}
	low := strings.ToLower(strings.ReplaceAll(s, "_", "-"))
	switch low {
	case "zh", "zh-cn", "cn", "chinese":
		return "zh-CN"
	case "zh-tw", "zh-hk", "tw", "hk":
		return "zh-TW"
	case "pt", "pt-br", "pt_br":
		return "pt-BR"
	case "jp":
		return "ja"
	case "ua":
		return "uk"
	}
	for _, code := range SupportedBotLangs {
		if strings.EqualFold(code, s) || strings.EqualFold(code, low) {
			return code
		}
	}
	// Prefix match e.g. en-US → en
	for _, code := range SupportedBotLangs {
		if strings.HasPrefix(low, strings.ToLower(code)+"-") || strings.HasPrefix(low, strings.ToLower(code)+"_") {
			return code
		}
	}
	return "en"
}

// Tr returns a localized string for key. Falls back to English, then the key.
func Tr(lang, key string) string {
	lang = NormalizeLang(lang)
	if m, ok := botMessages[lang]; ok {
		if v, ok := m[key]; ok && v != "" {
			return v
		}
	}
	if m, ok := botMessages["en"]; ok {
		if v, ok := m[key]; ok && v != "" {
			return v
		}
	}
	return key
}

// Trf is Tr + fmt.Sprintf.
func Trf(lang, key string, args ...any) string {
	return fmt.Sprintf(Tr(lang, key), args...)
}

// botMessages is lang → key → text. Protocol/command names stay in Latin script.
var botMessages = map[string]map[string]string{
	"en": {
		"help": `🤖 <b>3m-ui Telegram Bot</b>
/id — Show your Telegram ID
/usage — My usage (user) or search users (admin)
/status — Panel & core status (admin)
/users /online /listeners /traffic — Admin lists
/restart — Restart Mihomo (admin)
/deldepleted — Remove expired/over-quota users (admin)
/search &lt;q&gt; — Search users (admin)
/backup — Backup hint (admin)
/help — This help`,
		"unknown_cmd":      "Unknown command. Send /help for the list.",
		"perm_denied":      "⛔ Admin-only command.\nAvailable: /id  /usage  /help",
		"your_id":          "🆔 Your Telegram ID: <code>%d</code>",
		"usage_admin_hint": "Usage: /usage &lt;keyword&gt; — search proxy users",
		"sub_fail":         "⚠️ Could not generate subscription link. Try again later.",
		"sub_link":         "🔗 <b>Subscription URL</b>\n<code>%s</code>",
		"not_bound":        "Not bound. Ask an admin to link Telegram ID <code>%d</code> to your account.",
		"my_usage_title":   "📊 <b>My Usage</b>",
		"user_label":       "User: <code>%s</code>",
		"traffic_label":    "Traffic: %s / %s",
		"up_down":          "↑ %s   ↓ %s",
		"expires":          "Expires: <code>%s</code>",
		"expires_never":    "Expires: Never",
		"status_online":    "Status: Online 🟢",
		"status_offline":   "Status: Offline",
		"sub_line":         "Subscription:\n<code>%s</code>",
		"sub_missing":      "Subscription: not generated yet",
		"unknown_action":   "Unknown action.",
		"status_title":     "📊 <b>Status</b>\ncore: <code>%s</code>\nversion: <code>%s</code>\npid: <code>%d</code>\nusers: %d (online %d, blocked %d)\nlisteners: %d%s",
		"core_running":     "running",
		"core_stopped":     "stopped",
		"users_fail":       "Failed to load users: %s",
		"users_empty":      "No proxy users.",
		"users_title":      "👥 <b>Users</b>",
		"online_title":     "🟢 <b>Online</b>",
		"online_empty":     "No online users.",
		"listeners_title":  "📡 <b>Listeners</b>",
		"listeners_empty":  "No listeners.",
		"traffic_title":    "📈 <b>Traffic</b> (top users)\napprox listed used sum: %s\n",
		"traffic_empty":    "No data.",
		"mihomo_nil":       "Mihomo service is not initialized.",
		"restart_fail":     "Restart failed: %s",
		"restart_ok":       "✅ Mihomo core restarted.",
		"cleanup_fail":     "Cleanup failed: %s",
		"cleanup_ok":       "🧹 Removed %d expired/over-quota user(s).",
		"search_usage":     "Usage: /search &lt;username or remark&gt;",
		"search_fail":      "Search failed: %s",
		"search_empty":     "No matching users.",
		"search_title":     "🔍 <b>Search</b> <code>%s</code> (%d)\n",
		"backup":           "📦 <b>Backup</b>\nDownload a full backup in the panel: Settings → Backup (SQLite + Mihomo config).\nAPI: <code>GET /api/v1/system/backup</code>",
		"btn_status":       "📊 Status",
		"btn_users":        "👥 Users",
		"btn_traffic":      "📈 Traffic",
		"btn_online":       "🟢 Online",
		"btn_listeners":    "📡 Nodes",
		"btn_cleanup":      "🧹 Cleanup",
		"btn_backup":       "📦 Backup",
		"btn_restart":      "🔄 Restart",
		"btn_my_usage":     "📊 My usage",
		"btn_my_sub":       "🔗 Subscription",
		"cmd_start":        "Start / help",
		"cmd_id":           "Show Telegram ID",
		"cmd_usage":        "Usage or search users",
		"cmd_status":       "Panel status",
		"cmd_users":        "List users",
		"cmd_online":       "Online users",
		"cmd_listeners":    "Inbound listeners",
		"cmd_traffic":      "Traffic summary",
		"cmd_restart":      "Restart Mihomo",
		"cmd_deldepleted":  "Remove depleted users",
		"cmd_search":       "Search users",
		"cmd_backup":       "Backup hint",
		"test_msg":         "🔔 <b>3m-ui</b> Telegram test — connection OK.",
	},
	"zh-CN": {
		"help": `🤖 <b>3m-ui Telegram 机器人</b>
/id — 显示你的 Telegram ID
/usage — 查询用量（用户）或搜索用户（管理员）
/status — 面板与核心状态（管理员）
/users /online /listeners /traffic — 管理列表
/restart — 重启 Mihomo（管理员）
/deldepleted — 清理到期/超额用户（管理员）
/search &lt;关键字&gt; — 搜索用户（管理员）
/backup — 备份说明（管理员）
/help — 本帮助`,
		"unknown_cmd": "未知指令。发送 /help 查看可用命令。", "perm_denied": "⛔ 此命令仅管理员可用。\n可用: /id  /usage  /help",
		"your_id": "🆔 你的 Telegram ID: <code>%d</code>", "usage_admin_hint": "用法: /usage &lt;关键字&gt; — 搜索代理用户",
		"sub_fail": "⚠️ 订阅链接生成失败，请稍后重试。", "sub_link": "🔗 <b>订阅链接</b>\n<code>%s</code>",
		"not_bound":      "未绑定账户。请联系管理员将 Telegram ID <code>%d</code> 绑定到你的账户。",
		"my_usage_title": "📊 <b>账户用量</b>", "user_label": "用户: <code>%s</code>", "traffic_label": "流量: %s / %s",
		"up_down": "上传 ↑ %s   下载 ↓ %s", "expires": "到期: <code>%s</code>", "expires_never": "到期: 永不过期",
		"status_online": "状态: 在线 🟢", "status_offline": "状态: 离线", "sub_line": "订阅:\n<code>%s</code>", "sub_missing": "订阅: 未生成",
		"unknown_action": "未知操作。",
		"status_title":   "📊 <b>状态</b>\n核心: <code>%s</code>\n版本: <code>%s</code>\npid: <code>%d</code>\n用户: %d（在线 %d，不可用 %d）\n节点: %d%s",
		"core_running":   "运行中", "core_stopped": "已停止",
		"users_fail": "读取用户失败: %s", "users_empty": "暂无代理用户。", "users_title": "👥 <b>用户</b>",
		"online_title": "🟢 <b>在线</b>", "online_empty": "暂无在线用户。",
		"listeners_title": "📡 <b>节点</b>", "listeners_empty": "暂无节点。",
		"traffic_title": "📈 <b>流量</b>（用量前列）\n列表合计约: %s\n", "traffic_empty": "暂无数据。",
		"mihomo_nil": "Mihomo 服务未初始化。", "restart_fail": "重启失败: %s", "restart_ok": "✅ Mihomo 核心已重启。",
		"cleanup_fail": "清理失败: %s", "cleanup_ok": "🧹 已删除 %d 个到期/超额用户。",
		"search_usage": "用法: /search &lt;用户名或备注&gt;", "search_fail": "搜索失败: %s", "search_empty": "未找到匹配用户。",
		"search_title": "🔍 <b>搜索</b> <code>%s</code>（%d）\n",
		"backup":       "📦 <b>备份</b>\n请在面板「系统设置 → 备份」下载完整备份（SQLite + Mihomo 配置）。\nAPI: <code>GET /api/v1/system/backup</code>",
		"btn_status":   "📊 状态", "btn_users": "👥 用户", "btn_traffic": "📈 流量", "btn_online": "🟢 在线",
		"btn_listeners": "📡 节点", "btn_cleanup": "🧹 清理", "btn_backup": "📦 备份", "btn_restart": "🔄 重启",
		"btn_my_usage": "📊 我的用量", "btn_my_sub": "🔗 订阅链接",
		"cmd_start": "开始 / 帮助", "cmd_id": "显示 Telegram ID", "cmd_usage": "用量或搜索用户", "cmd_status": "面板状态",
		"cmd_users": "用户列表", "cmd_online": "在线用户", "cmd_listeners": "入站节点", "cmd_traffic": "流量摘要",
		"cmd_restart": "重启 Mihomo", "cmd_deldepleted": "清理失效用户", "cmd_search": "搜索用户", "cmd_backup": "备份说明",
		"test_msg":     "🔔 <b>3m-ui</b> Telegram 测试消息 — 连接正常。",
		"user_welcome": "👋 欢迎！点击下方按钮查看用量与订阅链接。",
	},
	"zh-TW": {
		"help": `🤖 <b>3m-ui Telegram 機器人</b>
/id — 顯示 Telegram ID
/usage — 查詢用量或搜尋使用者
/status — 面板與核心狀態（管理員）
/help — 本說明`,
		"unknown_cmd": "未知指令。傳送 /help 查看可用命令。", "perm_denied": "⛔ 此命令僅管理員可用。\n可用: /id  /usage  /help",
		"your_id": "🆔 你的 Telegram ID: <code>%d</code>", "usage_admin_hint": "用法: /usage &lt;關鍵字&gt;",
		"sub_fail": "⚠️ 訂閱連結產生失敗。", "sub_link": "🔗 <b>訂閱連結</b>\n<code>%s</code>",
		"not_bound":      "未綁定帳戶。請管理員綁定 Telegram ID <code>%d</code>。",
		"my_usage_title": "📊 <b>帳戶用量</b>", "user_label": "使用者: <code>%s</code>", "traffic_label": "流量: %s / %s",
		"up_down": "上傳 ↑ %s   下載 ↓ %s", "expires": "到期: <code>%s</code>", "expires_never": "到期: 永不過期",
		"status_online": "狀態: 在線 🟢", "status_offline": "狀態: 離線", "sub_line": "訂閱:\n<code>%s</code>", "sub_missing": "訂閱: 未產生",
		"unknown_action": "未知操作。", "status_title": "📊 <b>狀態</b>\n核心: <code>%s</code>\n版本: <code>%s</code>\npid: <code>%d</code>\n使用者: %d（在線 %d，不可用 %d）\n節點: %d%s",
		"core_running": "運行中", "core_stopped": "已停止", "users_fail": "讀取使用者失敗: %s", "users_empty": "暫無代理使用者。", "users_title": "👥 <b>使用者</b>",
		"online_title": "🟢 <b>在線</b>", "online_empty": "暫無在線使用者。", "listeners_title": "📡 <b>節點</b>", "listeners_empty": "暫無節點。",
		"traffic_title": "📈 <b>流量</b>\n列表合計約: %s\n", "traffic_empty": "暫無資料。",
		"mihomo_nil": "Mihomo 服務未初始化。", "restart_fail": "重啟失敗: %s", "restart_ok": "✅ Mihomo 核心已重啟。",
		"cleanup_fail": "清理失敗: %s", "cleanup_ok": "🧹 已刪除 %d 個到期/超額使用者。",
		"search_usage": "用法: /search &lt;使用者名稱或備註&gt;", "search_fail": "搜尋失敗: %s", "search_empty": "未找到相符使用者。",
		"search_title": "🔍 <b>搜尋</b> <code>%s</code>（%d）\n",
		"backup":       "📦 <b>備份</b>\n請在面板「系統設定 → 備份」下載完整備份。\nAPI: <code>GET /api/v1/system/backup</code>",
		"btn_status":   "📊 狀態", "btn_users": "👥 使用者", "btn_traffic": "📈 流量", "btn_online": "🟢 在線",
		"btn_listeners": "📡 節點", "btn_cleanup": "🧹 清理", "btn_backup": "📦 備份", "btn_restart": "🔄 重啟",
		"btn_my_usage": "📊 我的用量", "btn_my_sub": "🔗 訂閱連結",
		"cmd_start": "開始 / 說明", "cmd_id": "顯示 Telegram ID", "cmd_usage": "用量或搜尋", "cmd_status": "面板狀態",
		"cmd_users": "使用者列表", "cmd_online": "在線使用者", "cmd_listeners": "入站節點", "cmd_traffic": "流量摘要",
		"cmd_restart": "重啟 Mihomo", "cmd_deldepleted": "清理失效使用者", "cmd_search": "搜尋使用者", "cmd_backup": "備份說明",
		"test_msg":     "🔔 <b>3m-ui</b> Telegram 測試 — 連線正常。",
		"user_welcome": "👋 歡迎！點下方按鈕查看用量與訂閱連結。",
	},
}

func init() {
	// Seed remaining languages from English, then overlay high-traffic keys.
	overlays := map[string]map[string]string{
		"ja": {
			"unknown_cmd": "不明なコマンドです。/help で一覧を表示します。", "perm_denied": "⛔ 管理者専用コマンドです。\n利用可: /id  /usage  /help",
			"your_id": "🆔 Telegram ID: <code>%d</code>", "sub_fail": "⚠️ 購読リンクを生成できませんでした。", "sub_link": "🔗 <b>購読 URL</b>\n<code>%s</code>",
			"not_bound":      "未連携です。管理者に Telegram ID <code>%d</code> の紐付けを依頼してください。",
			"my_usage_title": "📊 <b>利用状況</b>", "user_label": "ユーザー: <code>%s</code>", "traffic_label": "トラフィック: %s / %s",
			"expires_never": "期限: なし", "status_online": "状態: オンライン 🟢", "status_offline": "状態: オフライン",
			"unknown_action": "不明な操作です。", "core_running": "稼働中", "core_stopped": "停止",
			"users_empty": "プロキシユーザーがありません。", "users_title": "👥 <b>ユーザー</b>",
			"online_title": "🟢 <b>オンライン</b>", "online_empty": "オンラインのユーザーはいません。",
			"listeners_title": "📡 <b>ノード</b>", "listeners_empty": "ノードがありません。",
			"traffic_empty": "データがありません。", "restart_ok": "✅ Mihomo を再起動しました。",
			"cleanup_ok": "🧹 期限切れ/超過ユーザーを %d 件削除しました。", "search_empty": "一致するユーザーがありません。",
			"btn_status": "📊 状態", "btn_users": "👥 ユーザー", "btn_traffic": "📈 トラフィック", "btn_online": "🟢 オンライン",
			"btn_listeners": "📡 ノード", "btn_cleanup": "🧹 整理", "btn_backup": "📦 バックアップ", "btn_restart": "🔄 再起動",
			"btn_my_usage": "📊 利用状況", "btn_my_sub": "🔗 購読", "test_msg": "🔔 <b>3m-ui</b> Telegram テスト — 接続 OK。",
			"cmd_start": "開始 / ヘルプ", "cmd_status": "パネル状態", "cmd_restart": "Mihomo 再起動",
		},
		"ko": {
			"unknown_cmd": "알 수 없는 명령입니다. /help 를 보내세요.", "perm_denied": "⛔ 관리자 전용 명령입니다.\n사용 가능: /id  /usage  /help",
			"your_id": "🆔 Telegram ID: <code>%d</code>", "sub_fail": "⚠️ 구독 링크를 만들 수 없습니다.", "sub_link": "🔗 <b>구독 URL</b>\n<code>%s</code>",
			"not_bound":      "연결되지 않았습니다. 관리자에게 Telegram ID <code>%d</code> 연결을 요청하세요.",
			"my_usage_title": "📊 <b>사용량</b>", "core_running": "실행 중", "core_stopped": "중지",
			"users_title": "👥 <b>사용자</b>", "online_title": "🟢 <b>온라인</b>", "listeners_title": "📡 <b>노드</b>",
			"restart_ok": "✅ Mihomo 코어가 재시작되었습니다.", "cleanup_ok": "🧹 만료/초과 사용자 %d명을 삭제했습니다.",
			"btn_status": "📊 상태", "btn_users": "👥 사용자", "btn_traffic": "📈 트래픽", "btn_online": "🟢 온라인",
			"btn_listeners": "📡 노드", "btn_cleanup": "🧹 정리", "btn_backup": "📦 백업", "btn_restart": "🔄 재시작",
			"btn_my_usage": "📊 내 사용량", "btn_my_sub": "🔗 구독", "test_msg": "🔔 <b>3m-ui</b> Telegram 테스트 — 연결 OK.",
		},
		"es": {
			"unknown_cmd": "Comando desconocido. Envía /help.", "perm_denied": "⛔ Solo administradores.\nDisponible: /id  /usage  /help",
			"your_id": "🆔 Tu ID de Telegram: <code>%d</code>", "sub_fail": "⚠️ No se pudo generar el enlace de suscripción.",
			"sub_link": "🔗 <b>URL de suscripción</b>\n<code>%s</code>", "not_bound": "No vinculado. Pide al admin enlazar el ID <code>%d</code>.",
			"my_usage_title": "📊 <b>Mi uso</b>", "core_running": "en ejecución", "core_stopped": "detenido",
			"users_title": "👥 <b>Usuarios</b>", "online_title": "🟢 <b>En línea</b>", "listeners_title": "📡 <b>Nodos</b>",
			"restart_ok": "✅ Núcleo Mihomo reiniciado.", "cleanup_ok": "🧹 Se eliminaron %d usuario(s) caducados/sobre cuota.",
			"btn_status": "📊 Estado", "btn_users": "👥 Usuarios", "btn_traffic": "📈 Tráfico", "btn_online": "🟢 En línea",
			"btn_listeners": "📡 Nodos", "btn_cleanup": "🧹 Limpieza", "btn_backup": "📦 Copia", "btn_restart": "🔄 Reiniciar",
			"btn_my_usage": "📊 Mi uso", "btn_my_sub": "🔗 Suscripción", "test_msg": "🔔 <b>3m-ui</b> prueba Telegram — conexión OK.",
		},
		"fr": {
			"unknown_cmd": "Commande inconnue. Envoyez /help.", "perm_denied": "⛔ Réservé aux admins.\nDispo: /id  /usage  /help",
			"your_id": "🆔 Votre ID Telegram: <code>%d</code>", "sub_fail": "⚠️ Impossible de générer le lien d'abonnement.",
			"sub_link": "🔗 <b>URL d'abonnement</b>\n<code>%s</code>", "my_usage_title": "📊 <b>Mon usage</b>",
			"core_running": "en cours", "core_stopped": "arrêté", "restart_ok": "✅ Noyau Mihomo redémarré.",
			"btn_status": "📊 État", "btn_users": "👥 Utilisateurs", "btn_traffic": "📈 Trafic", "btn_online": "🟢 En ligne",
			"btn_listeners": "📡 Nœuds", "btn_cleanup": "🧹 Nettoyage", "btn_backup": "📦 Sauvegarde", "btn_restart": "🔄 Redémarrer",
			"btn_my_usage": "📊 Mon usage", "btn_my_sub": "🔗 Abonnement", "test_msg": "🔔 <b>3m-ui</b> test Telegram — connexion OK.",
		},
		"de": {
			"unknown_cmd": "Unbekannter Befehl. Sende /help.", "perm_denied": "⛔ Nur für Admins.\nVerfügbar: /id  /usage  /help",
			"your_id": "🆔 Deine Telegram-ID: <code>%d</code>", "sub_fail": "⚠️ Abo-Link konnte nicht erzeugt werden.",
			"sub_link": "🔗 <b>Abo-URL</b>\n<code>%s</code>", "my_usage_title": "📊 <b>Mein Verbrauch</b>",
			"core_running": "läuft", "core_stopped": "gestoppt", "restart_ok": "✅ Mihomo-Core neu gestartet.",
			"btn_status": "📊 Status", "btn_users": "👥 Benutzer", "btn_traffic": "📈 Traffic", "btn_online": "🟢 Online",
			"btn_listeners": "📡 Knoten", "btn_cleanup": "🧹 Aufräumen", "btn_backup": "📦 Backup", "btn_restart": "🔄 Neustart",
			"btn_my_usage": "📊 Verbrauch", "btn_my_sub": "🔗 Abo", "test_msg": "🔔 <b>3m-ui</b> Telegram-Test — Verbindung OK.",
		},
		"ru": {
			"unknown_cmd": "Неизвестная команда. Отправьте /help.", "perm_denied": "⛔ Только для админов.\nДоступно: /id  /usage  /help",
			"your_id": "🆔 Ваш Telegram ID: <code>%d</code>", "sub_fail": "⚠️ Не удалось создать ссылку подписки.",
			"sub_link": "🔗 <b>URL подписки</b>\n<code>%s</code>", "my_usage_title": "📊 <b>Мой трафик</b>",
			"core_running": "работает", "core_stopped": "остановлен", "restart_ok": "✅ Ядро Mihomo перезапущено.",
			"btn_status": "📊 Статус", "btn_users": "👥 Пользователи", "btn_traffic": "📈 Трафик", "btn_online": "🟢 Онлайн",
			"btn_listeners": "📡 Узлы", "btn_cleanup": "🧹 Очистка", "btn_backup": "📦 Бэкап", "btn_restart": "🔄 Перезапуск",
			"btn_my_usage": "📊 Мой трафик", "btn_my_sub": "🔗 Подписка", "test_msg": "🔔 <b>3m-ui</b> тест Telegram — соединение OK.",
		},
		"pt-BR": {
			"unknown_cmd": "Comando desconhecido. Envie /help.", "perm_denied": "⛔ Somente admin.\nDisponível: /id  /usage  /help",
			"your_id": "🆔 Seu ID do Telegram: <code>%d</code>", "sub_fail": "⚠️ Não foi possível gerar o link de assinatura.",
			"sub_link": "🔗 <b>URL de assinatura</b>\n<code>%s</code>", "my_usage_title": "📊 <b>Meu uso</b>",
			"core_running": "em execução", "core_stopped": "parado", "restart_ok": "✅ Núcleo Mihomo reiniciado.",
			"btn_status": "📊 Status", "btn_users": "👥 Usuários", "btn_traffic": "📈 Tráfego", "btn_online": "🟢 Online",
			"btn_listeners": "📡 Nós", "btn_cleanup": "🧹 Limpeza", "btn_backup": "📦 Backup", "btn_restart": "🔄 Reiniciar",
			"btn_my_usage": "📊 Meu uso", "btn_my_sub": "🔗 Assinatura", "test_msg": "🔔 <b>3m-ui</b> teste Telegram — conexão OK.",
		},
		"vi": {
			"unknown_cmd": "Lệnh không rõ. Gửi /help.", "perm_denied": "⛔ Chỉ admin.\nCó thể dùng: /id  /usage  /help",
			"your_id": "🆔 Telegram ID: <code>%d</code>", "sub_fail": "⚠️ Không tạo được link subscription.",
			"sub_link": "🔗 <b>URL subscription</b>\n<code>%s</code>", "my_usage_title": "📊 <b>Lưu lượng của tôi</b>",
			"core_running": "đang chạy", "core_stopped": "đã dừng", "restart_ok": "✅ Đã khởi động lại Mihomo.",
			"btn_status": "📊 Trạng thái", "btn_users": "👥 Người dùng", "btn_traffic": "📈 Lưu lượng", "btn_online": "🟢 Online",
			"btn_listeners": "📡 Node", "btn_cleanup": "🧹 Dọn", "btn_backup": "📦 Sao lưu", "btn_restart": "🔄 Khởi động lại",
			"btn_my_usage": "📊 Lưu lượng", "btn_my_sub": "🔗 Subscription", "test_msg": "🔔 <b>3m-ui</b> thử Telegram — kết nối OK.",
		},
		"id": {
			"unknown_cmd": "Perintah tidak dikenal. Kirim /help.", "perm_denied": "⛔ Hanya admin.\nTersedia: /id  /usage  /help",
			"your_id": "🆔 ID Telegram Anda: <code>%d</code>", "sub_fail": "⚠️ Gagal membuat tautan langganan.",
			"sub_link": "🔗 <b>URL langganan</b>\n<code>%s</code>", "my_usage_title": "📊 <b>Pemakaian saya</b>",
			"core_running": "berjalan", "core_stopped": "berhenti", "restart_ok": "✅ Core Mihomo dimulai ulang.",
			"btn_status": "📊 Status", "btn_users": "👥 Pengguna", "btn_traffic": "📈 Lalu lintas", "btn_online": "🟢 Online",
			"btn_listeners": "📡 Node", "btn_cleanup": "🧹 Bersihkan", "btn_backup": "📦 Cadangan", "btn_restart": "🔄 Mulai ulang",
			"btn_my_usage": "📊 Pemakaian", "btn_my_sub": "🔗 Langganan", "test_msg": "🔔 <b>3m-ui</b> tes Telegram — koneksi OK.",
		},
		"th": {
			"unknown_cmd": "คำสั่งไม่รู้จัก ส่ง /help", "perm_denied": "⛔ สำหรับแอดมินเท่านั้น\nใช้ได้: /id  /usage  /help",
			"your_id": "🆔 Telegram ID: <code>%d</code>", "sub_fail": "⚠️ สร้างลิงก์สมัครไม่สำเร็จ",
			"sub_link": "🔗 <b>URL สมัครสมาชิก</b>\n<code>%s</code>", "my_usage_title": "📊 <b>การใช้งานของฉัน</b>",
			"core_running": "กำลังทำงาน", "core_stopped": "หยุดแล้ว", "restart_ok": "✅ รีสตาร์ท Mihomo แล้ว",
			"btn_status": "📊 สถานะ", "btn_users": "👥 ผู้ใช้", "btn_traffic": "📈 ทราฟฟิก", "btn_online": "🟢 ออนไลน์",
			"btn_listeners": "📡 โหนด", "btn_cleanup": "🧹 ล้าง", "btn_backup": "📦 สำรอง", "btn_restart": "🔄 รีสตาร์ท",
			"btn_my_usage": "📊 การใช้งาน", "btn_my_sub": "🔗 สมัครสมาชิก", "test_msg": "🔔 <b>3m-ui</b> ทดสอบ Telegram — เชื่อมต่อ OK",
		},
		"tr": {
			"unknown_cmd": "Bilinmeyen komut. /help gönderin.", "perm_denied": "⛔ Yalnızca yönetici.\nKullanılabilir: /id  /usage  /help",
			"your_id": "🆔 Telegram ID: <code>%d</code>", "sub_fail": "⚠️ Abonelik bağlantısı oluşturulamadı.",
			"sub_link": "🔗 <b>Abonelik URL</b>\n<code>%s</code>", "my_usage_title": "📊 <b>Kullanımım</b>",
			"core_running": "çalışıyor", "core_stopped": "durdu", "restart_ok": "✅ Mihomo çekirdeği yeniden başlatıldı.",
			"btn_status": "📊 Durum", "btn_users": "👥 Kullanıcılar", "btn_traffic": "📈 Trafik", "btn_online": "🟢 Çevrimiçi",
			"btn_listeners": "📡 Düğümler", "btn_cleanup": "🧹 Temizle", "btn_backup": "📦 Yedek", "btn_restart": "🔄 Yeniden başlat",
			"btn_my_usage": "📊 Kullanım", "btn_my_sub": "🔗 Abonelik", "test_msg": "🔔 <b>3m-ui</b> Telegram testi — bağlantı OK.",
		},
		"ar": {
			"unknown_cmd": "أمر غير معروف. أرسل /help.", "perm_denied": "⛔ للمشرفين فقط.\nمتاح: /id  /usage  /help",
			"your_id": "🆔 معرّف Telegram: <code>%d</code>", "sub_fail": "⚠️ تعذّر إنشاء رابط الاشتراك.",
			"sub_link": "🔗 <b>رابط الاشتراك</b>\n<code>%s</code>", "my_usage_title": "📊 <b>استهلاكي</b>",
			"core_running": "يعمل", "core_stopped": "متوقف", "restart_ok": "✅ أُعيد تشغيل نواة Mihomo.",
			"btn_status": "📊 الحالة", "btn_users": "👥 المستخدمون", "btn_traffic": "📈 المرور", "btn_online": "🟢 متصل",
			"btn_listeners": "📡 العقد", "btn_cleanup": "🧹 تنظيف", "btn_backup": "📦 نسخة احتياطية", "btn_restart": "🔄 إعادة تشغيل",
			"btn_my_usage": "📊 استهلاكي", "btn_my_sub": "🔗 الاشتراك", "test_msg": "🔔 <b>3m-ui</b> اختبار Telegram — الاتصال OK.",
		},
		"hi": {
			"unknown_cmd": "अज्ञात कमांड। /help भेजें।", "perm_denied": "⛔ केवल एडमिन।\nउपलब्ध: /id  /usage  /help",
			"your_id": "🆔 आपका Telegram ID: <code>%d</code>", "sub_fail": "⚠️ सब्सक्रिप्शन लिंक नहीं बना।",
			"sub_link": "🔗 <b>सब्सक्रिप्शन URL</b>\n<code>%s</code>", "my_usage_title": "📊 <b>मेरा उपयोग</b>",
			"core_running": "चल रहा", "core_stopped": "रुका", "restart_ok": "✅ Mihomo कोर पुनः आरंभ।",
			"btn_status": "📊 स्थिति", "btn_users": "👥 उपयोगकर्ता", "btn_traffic": "📈 ट्रैफ़िक", "btn_online": "🟢 ऑनलाइन",
			"btn_listeners": "📡 नोड्स", "btn_cleanup": "🧹 सफ़ाई", "btn_backup": "📦 बैकअप", "btn_restart": "🔄 रीस्टार्ट",
			"btn_my_usage": "📊 उपयोग", "btn_my_sub": "🔗 सब्सक्रिप्शन", "test_msg": "🔔 <b>3m-ui</b> Telegram परीक्षण — कनेक्शन OK।",
		},
		"pl": {
			"unknown_cmd": "Nieznana komenda. Wyślij /help.", "perm_denied": "⛔ Tylko admin.\nDostępne: /id  /usage  /help",
			"your_id": "🆔 Twój Telegram ID: <code>%d</code>", "sub_fail": "⚠️ Nie udało się utworzyć linku subskrypcji.",
			"sub_link": "🔗 <b>URL subskrypcji</b>\n<code>%s</code>", "my_usage_title": "📊 <b>Moje użycie</b>",
			"core_running": "działa", "core_stopped": "zatrzymany", "restart_ok": "✅ Rdzeń Mihomo zrestartowany.",
			"btn_status": "📊 Status", "btn_users": "👥 Użytkownicy", "btn_traffic": "📈 Ruch", "btn_online": "🟢 Online",
			"btn_listeners": "📡 Węzły", "btn_cleanup": "🧹 Czyszczenie", "btn_backup": "📦 Kopia", "btn_restart": "🔄 Restart",
			"btn_my_usage": "📊 Użycie", "btn_my_sub": "🔗 Subskrypcja", "test_msg": "🔔 <b>3m-ui</b> test Telegram — połączenie OK.",
		},
		"uk": {
			"unknown_cmd": "Невідома команда. Надішліть /help.", "perm_denied": "⛔ Лише для адмінів.\nДоступно: /id  /usage  /help",
			"your_id": "🆔 Ваш Telegram ID: <code>%d</code>", "sub_fail": "⚠️ Не вдалося створити посилання підписки.",
			"sub_link": "🔗 <b>URL підписки</b>\n<code>%s</code>", "my_usage_title": "📊 <b>Мій трафік</b>",
			"core_running": "працює", "core_stopped": "зупинено", "restart_ok": "✅ Ядро Mihomo перезапущено.",
			"btn_status": "📊 Статус", "btn_users": "👥 Користувачі", "btn_traffic": "📈 Трафік", "btn_online": "🟢 Онлайн",
			"btn_listeners": "📡 Вузли", "btn_cleanup": "🧹 Очищення", "btn_backup": "📦 Бекап", "btn_restart": "🔄 Перезапуск",
			"btn_my_usage": "📊 Трафік", "btn_my_sub": "🔗 Підписка", "test_msg": "🔔 <b>3m-ui</b> тест Telegram — з'єднання OK.",
		},
	}
	en := botMessages["en"]
	for _, code := range SupportedBotLangs {
		if code == "en" || code == "zh-CN" || code == "zh-TW" {
			continue
		}
		m := make(map[string]string, len(en)+8)
		for k, v := range en {
			m[k] = v
		}
		if ov, ok := overlays[code]; ok {
			for k, v := range ov {
				m[k] = v
			}
		}
		botMessages[code] = m
	}
}
