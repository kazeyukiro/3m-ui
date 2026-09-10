package telegram

// inlineKeyboardButton represents a single button in an inline keyboard.
// Either CallbackData (button press → callback_query) or URL (open link) is set.
type inlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// inlineKeyboardMarkup wraps a 2D grid of buttons for Telegram's reply_markup.
type inlineKeyboardMarkup struct {
	InlineKeyboard [][]inlineKeyboardButton `json:"inline_keyboard"`
}

func buildAdminMenu(lang string) inlineKeyboardMarkup {
	lang = NormalizeLang(lang)
	return inlineKeyboardMarkup{
		InlineKeyboard: [][]inlineKeyboardButton{
			{
				{Text: Tr(lang, "btn_status"), CallbackData: "status"},
				{Text: Tr(lang, "btn_users"), CallbackData: "users"},
			},
			{
				{Text: Tr(lang, "btn_traffic"), CallbackData: "traffic"},
				{Text: Tr(lang, "btn_online"), CallbackData: "online"},
			},
			{
				{Text: Tr(lang, "btn_listeners"), CallbackData: "listeners"},
				{Text: Tr(lang, "btn_cleanup"), CallbackData: "deldepleted"},
			},
			{
				{Text: Tr(lang, "btn_backup"), CallbackData: "backup"},
				{Text: Tr(lang, "btn_restart"), CallbackData: "restart"},
			},
		},
	}
}

func buildUserMenu(lang string) inlineKeyboardMarkup {
	lang = NormalizeLang(lang)
	return inlineKeyboardMarkup{
		InlineKeyboard: [][]inlineKeyboardButton{
			{
				{Text: Tr(lang, "btn_my_usage"), CallbackData: "my_usage"},
				{Text: Tr(lang, "btn_my_sub"), CallbackData: "my_sub"},
			},
		},
	}
}
