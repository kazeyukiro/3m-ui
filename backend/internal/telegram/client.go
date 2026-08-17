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
	return &Client{
		Token:   strings.TrimSpace(token),
		ChatIDs: chatIDs,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.Token != "" && len(c.ChatIDs) > 0
}

type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

func (c *Client) SendText(text string) error {
	if !c.Enabled() {
		return fmt.Errorf("telegram is not configured")
	}
	var last error
	ok := 0
	for _, chatID := range c.ChatIDs {
		if err := c.sendOne(chatID, text); err != nil {
			last = err
			continue
		}
		ok++
	}
	if ok == 0 {
		if last != nil {
			return last
		}
		return fmt.Errorf("no telegram chats delivered")
	}
	return nil
}

func (c *Client) sendPlain(chatID, text string) error {
	return c.sendMessage(chatID, text, "")
}

func (c *Client) sendOne(chatID, text string) error {
	return c.sendMessage(chatID, text, "HTML")
}

func (c *Client) sendMessage(chatID, text, parseMode string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.Token)
	reqBody := sendMessageRequest{ChatID: chatID, Text: text}
	if parseMode != "" {
		reqBody.ParseMode = parseMode
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram API %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
