package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	Token      string
	ChatIDs    []string
	HTTPClient *http.Client
}

func NewClient(token string, chatIDs []string) *Client {
	clean := make([]string, 0, len(chatIDs))
	for _, id := range chatIDs {
		if id = strings.TrimSpace(id); id != "" { clean = append(clean, id) }
	}
	return &Client{Token: strings.TrimSpace(token), ChatIDs: clean, HTTPClient: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) Enabled() bool { return c != nil && c.Token != "" && len(c.ChatIDs) > 0 }

type sendMessageRequest struct { ChatID string `json:"chat_id"`; Text string `json:"text"`; ParseMode string `json:"parse_mode,omitempty"` }
type telegramResponse struct { OK bool `json:"ok"`; Description string `json:"description"` }

func (c *Client) SendText(text string) error {
	if !c.Enabled() { return fmt.Errorf("telegram is not configured") }
	var last error
	ok := 0
	for _, chatID := range c.ChatIDs {
		if err := c.sendOne(chatID, text); err != nil { last = err; continue }
		ok++
	}
	if ok == 0 { if last != nil { return last }; return fmt.Errorf("no telegram chats delivered") }
	return nil
}

// Validate checks the bot token with Telegram without sending a message.
func (c *Client) Validate() error {
	if c == nil || c.Token == "" { return fmt.Errorf("telegram bot token is empty") }
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", c.Token)
	resp, err := c.HTTPClient.Get(endpoint)
	if err != nil { return err }
	defer resp.Body.Close()
	var parsed telegramResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&parsed); err != nil { return err }
	if resp.StatusCode >= 300 || !parsed.OK { return fmt.Errorf("telegram API: %s", parsed.Description) }
	return nil
}

func (c *Client) sendOne(chatID, text string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.Token)
	body, err := json.Marshal(sendMessageRequest{ChatID: chatID, Text: text, ParseMode: "HTML"})
	if err != nil { return err }
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var parsed telegramResponse
	if err := json.Unmarshal(raw, &parsed); err != nil { return fmt.Errorf("telegram API returned invalid response: %w", err) }
	if resp.StatusCode >= 300 || !parsed.OK { return fmt.Errorf("telegram API %d: %s", resp.StatusCode, parsed.Description) }
	return nil
}
