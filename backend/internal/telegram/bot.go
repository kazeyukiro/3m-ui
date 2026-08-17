package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

type Bot struct {
	db *gorm.DB
	mihomo *mihomo.Service
	users *user.Service
	mu sync.Mutex
	stopCh chan struct{}
	wg sync.WaitGroup
	wizardMu sync.Mutex
	wizards map[string]*addClientWizard
}

type addClientWizard struct { ListenerID uint }

func NewBot(db *gorm.DB, mihomoSvc *mihomo.Service, userSvc *user.Service) *Bot {
	return &Bot{db: db, mihomo: mihomoSvc, users: userSvc, stopCh: make(chan struct{}), wizards: make(map[string]*addClientWizard)}
}

func (b *Bot) Start() {
	if b == nil { return }
	b.wg.Add(1)
	go func() { defer b.wg.Done(); b.loop() }()
	log.Printf("telegram: bot command loop started")
}

func (b *Bot) Stop() {
	if b == nil { return }
	b.mu.Lock()
	select { case <-b.stopCh: default: close(b.stopCh) }
	b.mu.Unlock()
	b.wg.Wait()
}

func (b *Bot) loop() {
	offset := LoadUpdateOffset(b.db)
	commandsToken := ""
	httpClient := &http.Client{Timeout: 45 * time.Second}
	for {
		select { case <-b.stopCh: return; default: }
		tgClient, settings, err := NewClientFromDB(b.db)
		if err != nil || tgClient == nil || !settings.Enabled {
			select { case <-b.stopCh: return; case <-time.After(15 * time.Second): }
			continue
		}
		if commandsToken != settings.BotToken {
			if err := setCommandsWithRetry(tgClient, 5); err != nil {
				log.Printf("telegram: set commands failed after retries: %v", err)
			} else { commandsToken = settings.BotToken }
		}
		updates, next, err := getUpdates(httpClient, settings.BotToken, offset, 25)
		if err != nil {
			log.Printf("telegram: getUpdates: %v", err)
			select { case <-b.stopCh: return; case <-time.After(5 * time.Second): }
			continue
		}
		for _, u := range updates {
			if u.CallbackQuery != nil {
				chatID := strconv.FormatInt(u.CallbackQuery.Message.Chat.ID, 10)
				current, loadErr := LoadSettings(b.db)
				if loadErr != nil { log.Printf("telegram: callback settings: %v", loadErr) } else if current.Enabled && current.BotToken == settings.BotToken {
					if err := b.handleCallbackGuarded(tgClient, current, chatID, u.CallbackQuery.ID, u.CallbackQuery.Data); err != nil { log.Printf("telegram: callback: %v", err) }
				}
			} else if u.Message != nil && strings.TrimSpace(u.Message.Text) != "" {
				chatID := strconv.FormatInt(u.Message.Chat.ID, 10)
				if b.handleWizardMessage(chatID, u.Message.Text) { continue }
				reply, markup := b.handleCommand(chatID, u.Message.Text)
				if err := tgClient.SendTo(chatID, reply, markup); err != nil { log.Printf("telegram: reply: %v", err) }
			}
			if u.UpdateID+1 > offset {
				offset = u.UpdateID + 1
				if err := SaveUpdateOffset(b.db, offset); err != nil { log.Printf("telegram: persist update offset %d: %v", offset, err) }
			}
		}
		if next > offset {
			offset = next
			if err := SaveUpdateOffset(b.db, offset); err != nil { log.Printf("telegram: persist batch offset %d: %v", offset, err) }
		}
	}
}

func (b *Bot) handleCallbackGuarded(c *Client, s Settings, chatID, callbackID, data string) error {
	current, err := LoadSettings(b.db)
	if err != nil { return err }
	if !current.Enabled || current.BotToken != s.BotToken { return c.AnswerCallback(callbackID, "Configuration changed / 配置已变更") }
	if strings.HasPrefix(data, "admin:") || strings.HasPrefix(data, "add:") {
		if !b.isAdmin(chatID, current) { return c.AnswerCallback(callbackID, "Permission denied / 无权限") }
	}
	if data == "admin:add" || strings.HasPrefix(data, "add:") {
		b.wizardMu.Lock(); _, active := b.wizards[chatID]; b.wizardMu.Unlock()
		if active { return c.AnswerCallback(callbackID, "Wizard already active / 向导已在进行中") }
	}
	return b.handleCallback(c, current, chatID, callbackID, data)
}

func setCommandsWithRetry(c *Client, attempts int) error { return setCommandsWithRetryFn(c.SetCommands, attempts) }
func setCommandsWithRetryFn(fn func() error, attempts int) error {
	if attempts <= 0 { return fmt.Errorf("telegram: invalid retry count %d", attempts) }
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil { return nil }
		if i+1 < attempts {
			delay := time.Duration(1<<i) * time.Second
			log.Printf("telegram: set commands attempt %d/%d failed: %v; retrying in %s", i+1, attempts, err, delay)
			time.Sleep(delay)
		}
	}
	return err
}

type tgUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message *struct { Text string `json:"text"`; Chat struct { ID int64 `json:"id"` } `json:"chat"` } `json:"message"`
	CallbackQuery *struct { ID string `json:"id"`; Data string `json:"data"`; Message struct { Chat struct { ID int64 `json:"id"` } `json:"chat"` } `json:"message"` } `json:"callback_query"`
}

type tgUpdatesResponse struct { OK bool `json:"ok"`; Result []tgUpdate `json:"result"`; Description string `json:"description"` }

func parseUpdates(raw []byte, offset int64) ([]tgUpdate, int64, error) {
	var parsed tgUpdatesResponse
	if err := json.Unmarshal(raw, &parsed); err != nil { return nil, offset, err }
	if !parsed.OK { return nil, offset, fmt.Errorf("telegram API: %s", parsed.Description) }
	next := offset
	for _, u := range parsed.Result { if u.UpdateID+1 > next { next = u.UpdateID+1 } }
	return parsed.Result, next, nil
}

func getUpdates(httpClient *http.Client, token string, offset int64, timeoutSec int) ([]tgUpdate, int64, error) {
	q := url.Values{}
	q.Set("timeout", strconv.Itoa(timeoutSec))
	q.Set("allowed_updates", `["message","callback_query"]`)
	if offset > 0 { q.Set("offset", strconv.FormatInt(offset, 10)) }
	resp, err := httpClient.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?%s", token, q.Encode()))
	if err != nil { return nil, offset, err }
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil { return nil, offset, err }
	if resp.StatusCode >= 300 { return nil, offset, fmt.Errorf("telegram API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw))) }
	return parseUpdates(raw, offset)
}
