package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// Client is a thin wrapper around the Telegram Bot HTTP API.
// It is safe for concurrent use once constructed.
type Client struct {
	Token      string
	ChatIDs    []string
	Proxy      string
	APIServer  string
	HTTPClient *http.Client
}

// NewClient constructs a Client. If apiServer is empty the official
// https://api.telegram.org base is used. If proxyURL is set it must be an
// http(s):// or socks5:// URL; parse failures fall back to a direct connection
// with a logged warning.
func NewClient(token string, chatIDs []string, proxyURL, apiServer string) *Client {
	token = strings.TrimSpace(token)
	apiServer = strings.TrimSpace(apiServer)
	if apiServer == "" {
		apiServer = "https://api.telegram.org"
	}
	proxyURL = strings.TrimSpace(proxyURL)
	return &Client{
		Token:      token,
		ChatIDs:    chatIDs,
		Proxy:      proxyURL,
		APIServer:  apiServer,
		HTTPClient: buildHTTPClient(proxyURL),
	}
}

// buildHTTPClient returns an *http.Client optionally routed through proxyURL.
// Unsupported schemes or parse failures log a warning and fall back to direct.
func buildHTTPClient(proxyURL string) *http.Client {
	base := &http.Client{Timeout: 15 * time.Second}
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return base
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		log.Printf("telegram: parse proxy url %q failed: %v — falling back to direct", proxyURL, err)
		return base
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		base.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
		return base
	case "socks5", "socks5h":
		dialer, err := proxy.FromURL(u, proxy.Direct)
		if err != nil {
			log.Printf("telegram: socks5 dialer for %q failed: %v — falling back to direct", proxyURL, err)
			return base
		}
		transport := &http.Transport{}
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = cd.DialContext
		} else {
			transport.Dial = dialer.Dial
		}
		base.Transport = transport
		return base
	default:
		log.Printf("telegram: unsupported proxy scheme %q — falling back to direct", u.Scheme)
		return base
	}
}

// apiBase returns the trimmed Telegram API base URL (no trailing slash).
func (c *Client) apiBase() string {
	if c == nil {
		return "https://api.telegram.org"
	}
	base := strings.TrimRight(strings.TrimSpace(c.APIServer), "/")
	if base == "" {
		return "https://api.telegram.org"
	}
	return base
}

func (c *Client) Enabled() bool {
	return c != nil && c.Token != "" && len(c.ChatIDs) > 0
}

// BotInfo is the subset of Telegram getMe result exposed to the panel.
type BotInfo struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	ID        int64  `json:"id"`
}

// GetMe calls Telegram getMe and returns the bot identity.
func (c *Client) GetMe() (*BotInfo, error) {
	if c == nil || strings.TrimSpace(c.Token) == "" {
		return nil, fmt.Errorf("telegram bot token is empty")
	}
	endpoint := fmt.Sprintf("%s/bot%s/getMe", c.apiBase(), c.Token)
	resp, err := c.HTTPClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("telegram getMe: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("telegram getMe HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		OK          bool     `json:"ok"`
		Result      *BotInfo `json:"result"`
		Description string   `json:"description"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("telegram getMe decode: %w", err)
	}
	if !parsed.OK {
		if parsed.Description != "" {
			return nil, fmt.Errorf("telegram getMe: %s", parsed.Description)
		}
		return nil, fmt.Errorf("telegram getMe not ok")
	}
	if parsed.Result == nil {
		return nil, fmt.Errorf("telegram getMe returned empty result")
	}
	return parsed.Result, nil
}

// Validate calls Telegram getMe to verify the bot token is usable.
func (c *Client) Validate() error {
	_, err := c.GetMe()
	return err
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
	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", c.apiBase(), c.Token)
	reqBody := sendMessageRequest{ChatID: chatID, Text: text}
	if parseMode != "" {
		reqBody.ParseMode = parseMode
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
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

// sendWithKeyboard sends an HTML-formatted message with an inline keyboard
// attached (reply_markup). Used by /start, /help and menu rendering handlers.
func (c *Client) sendWithKeyboard(chatID, text string, keyboard inlineKeyboardMarkup) error {
	if c == nil || strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("telegram bot token is empty")
	}
	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", c.apiBase(), c.Token)
	payload := struct {
		ChatID      string               `json:"chat_id"`
		Text        string               `json:"text"`
		ParseMode   string               `json:"parse_mode"`
		ReplyMarkup inlineKeyboardMarkup `json:"reply_markup"`
	}{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
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
		return fmt.Errorf("telegram sendWithKeyboard %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// answerCallbackQuery clears the loading spinner shown on a callback button
// after the user taps it. Pass the callback_query.id from the Telegram update.
func (c *Client) answerCallbackQuery(callbackID string) error {
	if c == nil || strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("telegram bot token is empty")
	}
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		return fmt.Errorf("callback_query_id is required")
	}
	endpoint := fmt.Sprintf("%s/bot%s/answerCallbackQuery", c.apiBase(), c.Token)
	payload := struct {
		CallbackQueryID string `json:"callback_query_id"`
	}{
		CallbackQueryID: callbackID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
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
		return fmt.Errorf("telegram answerCallbackQuery %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// SendDocument uploads a file to a single chat via multipart sendDocument.
// The body is consumed fully. contentType may be empty (defaults to
// application/octet-stream).
func (c *Client) SendDocument(chatID, filename, contentType string, body io.Reader) error {
	if !c.Enabled() {
		return fmt.Errorf("telegram is not configured")
	}
	if strings.TrimSpace(filename) == "" {
		return fmt.Errorf("filename is required")
	}
	if body == nil {
		return fmt.Errorf("body is required")
	}
	endpoint := fmt.Sprintf("%s/bot%s/sendDocument", c.apiBase(), c.Token)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("chat_id", chatID); err != nil {
		return err
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="document"; filename="%s"`, escapeQuotes(filename)))
	if ct := strings.TrimSpace(contentType); ct != "" {
		h.Set("Content-Type", ct)
	} else {
		h.Set("Content-Type", "application/octet-stream")
	}
	part, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, body); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram sendDocument %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// escapeQuotes mirrors multipart.Writer.CreateFormFile's internal escaper so
// our custom content-type document part stays RFC 7578 compliant.
func escapeQuotes(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return r.Replace(s)
}

// BotCommand is a single entry of the Telegram bot command menu.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// SetMyCommands registers the command menu via the Telegram setMyCommands API.
// It returns the parsed upstream response.
func (c *Client) SetMyCommands(commands []BotCommand) (map[string]interface{}, error) {
	if c == nil || strings.TrimSpace(c.Token) == "" {
		return nil, fmt.Errorf("telegram bot token is empty")
	}
	endpoint := fmt.Sprintf("%s/bot%s/setMyCommands", c.apiBase(), c.Token)
	body, err := json.Marshal(struct {
		Commands []BotCommand `json:"commands"`
	}{Commands: commands})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("telegram setMyCommands decode: %w (body: %s)", err, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode >= 300 {
		return parsed, fmt.Errorf("telegram setMyCommands HTTP %d", resp.StatusCode)
	}
	return parsed, nil
}
