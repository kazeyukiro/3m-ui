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
	"gorm.io/gorm"
)

type Bot struct {
	db     *gorm.DB
	mihomo *mihomo.Service

	mu             sync.Mutex
	stopCh         chan struct{}
	wg             sync.WaitGroup
	webhookCleared bool
}

func NewBot(db *gorm.DB, mihomoSvc *mihomo.Service) *Bot {
	return &Bot{db: db, mihomo: mihomoSvc, stopCh: make(chan struct{})}
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
			tgClient = NewClient(settings.BotToken, settings.ChatIDs)
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
		for _, u := range updates {
			msg := u.Message
			if msg == nil {
				continue
			}
			text := strings.TrimSpace(msg.Text)
			if text == "" {
				continue
			}
			chatID := strconv.FormatInt(msg.Chat.ID, 10)
			if !chatAllowed(allowed, chatID) {
				log.Printf("telegram: ignore from chat %s (allowlist=%v): %q",
					chatID, settings.ChatIDs, truncate(text, 40))
				continue
			}
			reply := b.handleCommand(text)
			if err := tgClient.sendOne(chatID, reply); err != nil {
				log.Printf("telegram: send HTML failed: %v", err)
				if err2 := tgClient.sendPlain(chatID, stripHTML(reply)); err2 != nil {
					log.Printf("telegram: send plain failed: %v", err2)
				}
			}
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
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int    `json:"message_id"`
		Text      string `json:"text"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
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
	q.Set("allowed_updates", `["message"]`)
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
