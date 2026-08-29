package telegram

// inlineKeyboardButton represents a single button in an inline keyboard.
// Either CallbackData (button press → callback_query) or URL (open link) is set.
type inlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// inlineKeyboardMarkup wraps a 2D grid of buttons for Telegram's reply_markup.
// Each inner slice is a visual row.
type inlineKeyboardMarkup struct {
	InlineKeyboard [][]inlineKeyboardButton `json:"inline_keyboard"`
}

// buildAdminMenu returns the inline keyboard shown to admin chats (after /start
// or /help). Buttons trigger the corresponding callback_query handlers.
func buildAdminMenu() inlineKeyboardMarkup {
	return inlineKeyboardMarkup{
		InlineKeyboard: [][]inlineKeyboardButton{
			{
				{Text: "📊 状态", CallbackData: "status"},
				{Text: "👥 用户", CallbackData: "users"},
			},
			{
				{Text: "📈 流量", CallbackData: "traffic"},
				{Text: "🟢 在线", CallbackData: "online"},
			},
			{
				{Text: "📡 节点", CallbackData: "listeners"},
				{Text: "🧹 清理", CallbackData: "deldepleted"},
			},
			{
				{Text: "📦 备份", CallbackData: "backup"},
				{Text: "🔄 重启", CallbackData: "restart"},
			},
		},
	}
}

// buildUserMenu returns the inline keyboard shown to chats bound to a proxy
// user via TelegramID (admin-independent, access-controlled by GetByTelegramID).
func buildUserMenu() inlineKeyboardMarkup {
	return inlineKeyboardMarkup{
		InlineKeyboard: [][]inlineKeyboardButton{
			{
				{Text: "📊 我的用量", CallbackData: "my_usage"},
				{Text: "🔗 订阅链接", CallbackData: "my_sub"},
			},
		},
	}
}
