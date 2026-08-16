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

// Bot runs a long-poll getUpdates loop and answers admin commands.
type Bot struct {
	db     *gorm.DB
	mihomo *mihomo.Service

	mu     sync.Mutex
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewBot(db *gorm.DB, mihomoSvc *mihomo.Service) *Bot {
	return &Bot{db: db, mihomo: mihomoSvc, stopCh: make(chan struct{})}
}

// Start begins the command loop in the background. Safe to call once.
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

// Stop signals the loop to exit.
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
	client := &http.Client{Timeout: 45 * time.Second}
	for {
		select {
		case <-b.stopCh:
			return
		default:
		}
		tgClient, settings, err := NewClientFromDB(b.db)
		if err != nil || tgClient == nil || !settings.Enabled {
			select {
			case <-b.stopCh:
				return
			case <-time.After(15 * time.Second):
			}
			continue
		}
		updates, next, err := getUpdates(client, settings.BotToken, offset, 25)
		if err != nil {
			log.Printf("telegram: getUpdates: %v", err)
			select {
			case <-b.stopCh:
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		if next > offset {
			offset = next
		}
		allowed := make(map[string]struct{}, len(settings.ChatIDs))
		for _, id := range settings.ChatIDs {
			allowed[id] = struct{}{}
		}
		for _, u := range updates {
			if u.Message == nil || strings.TrimSpace(u.Message.Text) == "" {
				continue
			}
			chatID := strconv.FormatInt(u.Message.Chat.ID, 10)
			if _, ok := allowed[chatID]; !ok {
				continue
			}
			reply := b.handleCommand(u.Message.Text)
			_ = tgClient.sendOne(chatID, reply)
		}
	}
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
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, offset, err
	}
	if resp.StatusCode >= 300 {
		return nil, offset, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		OK     bool       `json:"ok"`
		Result []tgUpdate `json:"result"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, offset, err
	}
	if !parsed.OK {
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
