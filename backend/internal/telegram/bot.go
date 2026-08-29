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

// commandContext carries everything a command or callback handler needs to
// reply: the originating chat / sender / message IDs plus access flags so
// handlers can branch on admin vs bound-user identity.
type commandContext struct {
	Text    string // raw message text (empty for callback_query)
	ChatID  int64
	FromID  int64
	MsgID   int64
	IsAdmin bool // ChatID is in settings.ChatIDs (admin allowlist)
	IsBound bool // FromID matches a ProxyUser.TelegramID (bound user)
}

type Bot struct {
	db      *gorm.DB
	mihomo  *mihomo.Service
	userSvc *user.Service

	mu             sync.Mutex
	stopCh         chan struct{}
	wg             sync.WaitGroup
	webhookCleared bool

	// tgClient is the long-poll Telegram client reused by handlers to send
	// messages with keyboards. It is set by loop() on each iteration and only
	// read/written by the single loop goroutine, so no extra locking is needed.
	tgClient *Client
}

func NewBot(db *gorm.DB, mihomoSvc *mihomo.Service, userSvc *user.Service) *Bot {
	return &Bot{db: db, mihomo: mihomoSvc, userSvc: userSvc, stopCh: make(chan struct{})}
}

func (b *Bot) Start() {
	if b == nil {
		return
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.loop()
	}()
	log.Printf("telegram: bot command loop started")
}

func (b *Bot) Stop() {
	if b == nil {
		return
	}
	b.mu.Lock()
	select {
	case <-b.stopCh:
	default:
		close(b.stopCh)
	}
	b.mu.Unlock()
	b.wg.Wait()
}

func (b *Bot) loop() {
	var offset int64
	client := &http.Client{Timeout: 50 * time.Second}
	for {
		select {
		case <-b.stopCh:
			return
		default:
		}
		tgClient, settings, err := NewClientFromDB(b.db)
		if err != nil {
			log.Printf("telegram: load settings: %v", err)
			if !sleepOrStop(b.stopCh, 15*time.Second) {
				return
			}
			continue
		}
		if !settings.Enabled {
			if !sleepOrStop(b.stopCh, 20*time.Second) {
				return
			}
			continue
		}
		if strings.TrimSpace(settings.BotToken) == "" {
			log.Printf("telegram: enabled but bot token is empty — set token in panel Settings")
			if !sleepOrStop(b.stopCh, 20*time.Second) {
				return
			}
			continue
		}
		if len(settings.ChatIDs) == 0 {
			log.Printf("telegram: enabled but chat allowlist is empty — add Chat ID(s) in panel Settings")
			if !sleepOrStop(b.stopCh, 20*time.Second) {
				return
			}
			continue
		}
		if tgClient == nil {
			tgClient = NewClient(settings.BotToken, settings.ChatIDs, settings.ProxyURL, settings.APIServer)
		}

		if !b.webhookCleared {
			if err := deleteWebhook(client, settings.BotToken); err != nil {
				log.Printf("telegram: deleteWebhook: %v", err)
			} else {
				b.webhookCleared = true
				log.Printf("telegram: webhook cleared, long-poll ready")
			}
		}

		updates, next, err := getUpdates(client, settings.BotToken, offset, 30)
		if err != nil {
			log.Printf("telegram: getUpdates: %v", err)
			if !sleepOrStop(b.stopCh, 5*time.Second) {
				return
			}
			continue
		}
		if next > offset {
			offset = next
		}

		allowed := buildAllowedChats(settings.ChatIDs)
		b.tgClient = tgClient
		for _, u := range updates {
			if msg := u.Message; msg != nil {
				b.handleMessageUpdate(allowed, tgClient, msg)
				continue
			}
			if cb := u.CallbackQuery; cb != nil {
				b.handleCallbackUpdate(allowed, tgClient, cb)
			}
		}
	}
}

// tgMessage is the shape of the Telegram `message` field used by the bot loop.
// It is inlined into tgUpdate so JSON decoding works without a separate type.
type tgMessage struct {
	MessageID int    `json:"message_id"`
	Text      string `json:"text"`
	From      struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}

// tgCallbackQuery is the shape of Telegram's callback_query field.
type tgCallbackQuery struct {
	ID   string `json:"id"`
	Data string `json:"data"`
	From struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Message *struct {
		MessageID int64 `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

// handleMessageUpdate dispatches an incoming text message. Access control:
// admin chats (settings.ChatIDs) always pass; other chats pass only when the
// sender's FromID matches a bound ProxyUser.TelegramID. Non-text and
// unauthorised messages are dropped silently with a debug log line.
func (b *Bot) handleMessageUpdate(allowed map[string]struct{}, tgClient *Client, msg *tgMessage) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}
	chatID := strconv.FormatInt(msg.Chat.ID, 10)
	isAdmin := chatAllowed(allowed, chatID)
	isBound := false
	if !isAdmin && b.userSvc != nil {
		if u, err := b.userSvc.GetByTelegramID(msg.From.ID); err == nil && u != nil {
			isBound = true
		}
	}
	if !isAdmin && !isBound {
		log.Printf("telegram: ignore from chat %s (not admin, not bound): %q",
			chatID, truncate(text, 40))
		return
	}
	ctx := commandContext{
		Text:    text,
		ChatID:  msg.Chat.ID,
		FromID:  msg.From.ID,
		MsgID:   int64(msg.MessageID),
		IsAdmin: isAdmin,
		IsBound: isBound,
	}
	reply := b.handleCommand(ctx)
	if reply == "" {
		return // handler already sent the message (e.g. with keyboard)
	}
	if err := tgClient.sendOne(chatID, reply); err != nil {
		log.Printf("telegram: send HTML failed: %v", err)
		if err2 := tgClient.sendPlain(chatID, stripHTML(reply)); err2 != nil {
			log.Printf("telegram: send plain failed: %v", err2)
		}
	}
}

// handleCallbackUpdate dispatches a callback_query from an inline keyboard tap.
// Access control mirrors handleMessageUpdate: admin chats or bound users only.
// The spinner is always cleared via answerCallbackQuery before dispatch.
func (b *Bot) handleCallbackUpdate(allowed map[string]struct{}, tgClient *Client, cb *tgCallbackQuery) {
	if cb == nil || cb.Message == nil {
		_ = tgClient.answerCallbackQuery(cb.ID)
		return
	}
	chatID := strconv.FormatInt(cb.Message.Chat.ID, 10)
	isAdmin := chatAllowed(allowed, chatID)
	isBound := false
	if !isAdmin && b.userSvc != nil {
		if u, err := b.userSvc.GetByTelegramID(cb.From.ID); err == nil && u != nil {
			isBound = true
		}
	}
	// Always answer the callback to clear the loading spinner, even on rejection.
	if err := tgClient.answerCallbackQuery(cb.ID); err != nil {
		log.Printf("telegram: answerCallbackQuery failed: %v", err)
	}
	if !isAdmin && !isBound {
		log.Printf("telegram: ignore callback from chat %s (not admin, not bound)", chatID)
		return
	}
	ctx := commandContext{
		ChatID:  cb.Message.Chat.ID,
		FromID:  cb.From.ID,
		MsgID:   cb.Message.MessageID,
		IsAdmin: isAdmin,
		IsBound: isBound,
	}
	reply := b.handleCallback(ctx, cb.Data)
	if reply == "" {
		return
	}
	if err := tgClient.sendOne(chatID, reply); err != nil {
		log.Printf("telegram: callback reply failed: %v", err)
		if err2 := tgClient.sendPlain(chatID, stripHTML(reply)); err2 != nil {
			log.Printf("telegram: send plain failed: %v", err2)
		}
	}
}

func sleepOrStop(stopCh <-chan struct{}, d time.Duration) bool {
	select {
	case <-stopCh:
		return false
	case <-time.After(d):
		return true
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func stripHTML(s string) string {
	r := strings.NewReplacer(
		"<b>", "", "</b>", "",
		"<code>", "", "</code>", "",
		"&lt;", "<", "&gt;", ">", "&amp;", "&",
	)
	return r.Replace(s)
}

func normalizeChatID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "+")
	id = strings.TrimPrefix(id, "@")
	return id
}

func buildAllowedChats(ids []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		n := normalizeChatID(id)
		if n != "" {
			allowed[n] = struct{}{}
		}
	}
	return allowed
}

func chatAllowed(allowed map[string]struct{}, chatID string) bool {
	_, ok := allowed[normalizeChatID(chatID)]
	return ok
}

type tgUpdate struct {
	UpdateID int64      `json:"update_id"`
	Message  *tgMessage `json:"message"`
	// CallbackQuery is populated when the user taps an inline keyboard button.
	CallbackQuery *tgCallbackQuery `json:"callback_query"`
}

func deleteWebhook(httpClient *http.Client, token string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook?drop_pending_updates=false", token)
	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func getUpdates(httpClient *http.Client, token string, offset int64, timeoutSec int) ([]tgUpdate, int64, error) {
	q := url.Values{}
	q.Set("timeout", strconv.Itoa(timeoutSec))
	q.Set("allowed_updates", `["message","callback_query"]`)
	if offset > 0 {
		q.Set("offset", strconv.FormatInt(offset, 10))
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?%s", token, q.Encode())
	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return nil, offset, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, offset, err
	}
	if resp.StatusCode >= 300 {
		return nil, offset, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		OK          bool       `json:"ok"`
		Result      []tgUpdate `json:"result"`
		Description string     `json:"description"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, offset, err
	}
	if !parsed.OK {
		if parsed.Description != "" {
			return nil, offset, fmt.Errorf("telegram getUpdates not ok: %s", parsed.Description)
		}
		return nil, offset, fmt.Errorf("telegram getUpdates not ok")
	}
	next := offset
	for _, u := range parsed.Result {
		if u.UpdateID+1 > next {
			next = u.UpdateID + 1
		}
	}
	return parsed.Result, next, nil
}
