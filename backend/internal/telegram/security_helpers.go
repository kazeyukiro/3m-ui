package telegram

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

func bindUserTransactional(db *gorm.DB, username, chatID string) string {
	id, err := strconv.ParseInt(strings.TrimSpace(chatID), 10, 64)
	if err != nil || id == 0 { return "Invalid Telegram ID / Telegram ID 无效。" }
	var message string
	err = db.Transaction(func(tx *gorm.DB) error {
		var u models.ProxyUser
		if err := tx.Where("username = ?", username).First(&u).Error; err != nil { return fmt.Errorf("client not found") }
		var existing models.ProxyUser
		if err := tx.Where("telegram_id = ? AND id <> ?", id, u.ID).First(&existing).Error; err == nil {
			message = fmt.Sprintf("Telegram ID is already bound to client / Telegram ID 已绑定到客户端 <code>%s</code>。", escapeHTML(existing.Username))
			return gorm.ErrDuplicatedKey
		} else if err != gorm.ErrRecordNotFound { return err }
		if u.TelegramID != nil && *u.TelegramID != id {
			message = fmt.Sprintf("Client is already bound to Telegram ID %d / 客户端已经绑定 Telegram ID <code>%d</code>。", *u.TelegramID, *u.TelegramID)
			return gorm.ErrDuplicatedKey
		}
		if err := tx.Model(&u).Update("telegram_id", id).Error; err != nil { return err }
		message = fmt.Sprintf("Client <code>%s</code> bound to Telegram ID <code>%d</code> / 已将客户端绑定到 Telegram ID。", escapeHTML(username), id)
		return nil
	})
	if err == nil { return message }
	if message != "" { return message }
	if strings.Contains(strings.ToLower(err.Error()), "client not found") { return "Client not found / 没有找到这个客户端。" }
	return "Binding failed / 绑定失败：" + escapeHTML(err.Error())
}
