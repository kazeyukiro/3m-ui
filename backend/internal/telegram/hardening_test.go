package telegram

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSplitTelegramHTMLKeepsEntitiesIntact(t *testing.T) {
	text := strings.Repeat("A", 4080) + " &amp; " + strings.Repeat("B", 100)
	chunks := splitTelegramHTML(text, 4096)
	if len(chunks) < 2 { t.Fatal("expected message to be split") }
	for _, chunk := range chunks {
		if strings.Contains(chunk, "&am") && !strings.Contains(chunk, "&amp;") { t.Fatalf("HTML entity was split: %q", chunk) }
		if len([]rune(chunk)) > 4096 { t.Fatalf("chunk exceeds Telegram limit: %d", len([]rune(chunk))) }
	}
}

func TestPostWithRetryRetries429AndSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n < 3 { w.WriteHeader(http.StatusTooManyRequests); _, _ = w.Write([]byte(`{"ok":false,"description":"Too Many Requests"}`)); return }
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := NewClient("token", []string{"1"})
	c.HTTPClient = srv.Client()
	if err := c.postWithRetry(srv.URL, []byte(`{}`), "application/json"); err != nil { t.Fatalf("unexpected error: %v", err) }
	if got := atomic.LoadInt32(&calls); got != 3 { t.Fatalf("expected 3 calls, got %d", got) }
}

func TestPostWithRetryDoesNotRetry400(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: object expected as reply markup"}`))
	}))
	defer srv.Close()
	c := NewClient("token", []string{"1"})
	c.HTTPClient = srv.Client()
	err := c.postWithRetry(srv.URL, []byte(`{}`), "application/json")
	if err == nil || !strings.Contains(err.Error(), "object expected as reply markup") { t.Fatalf("expected Telegram API description, got %v", err) }
	if got := atomic.LoadInt32(&calls); got != 1 { t.Fatalf("expected no retry for 400, got %d calls", got) }
}

func TestSetCommandsRetryStopsOnSuccess(t *testing.T) {
	var calls int
	err := setCommandsWithRetryFn(func() error { calls++; if calls < 2 { return errors.New("temporary") }; return nil }, 5)
	if err != nil { t.Fatal(err) }
	if calls != 2 { t.Fatalf("expected 2 attempts, got %d", calls) }
}

func TestSetCommandsRetryReturnsLastError(t *testing.T) {
	var calls int
	err := setCommandsWithRetryFn(func() error { calls++; return errors.New("permanent") }, 2)
	if err == nil || err.Error() != "permanent" { t.Fatalf("unexpected error: %v", err) }
	if calls != 2 { t.Fatalf("expected 2 attempts, got %d", calls) }
}

func TestParseUpdatesPreservesOffsetOnTelegramError(t *testing.T) {
	updates, next, err := parseUpdates([]byte(`{"ok":false,"description":"Conflict: terminated by other getUpdates request"}`), 42)
	if err == nil || !strings.Contains(err.Error(), "terminated by other getUpdates") { t.Fatalf("unexpected error: %v", err) }
	if updates != nil || next != 42 { t.Fatalf("offset changed after failed update: next=%d", next) }
}

func TestParseUpdatesAdvancesToHighestUpdate(t *testing.T) {
	updates, next, err := parseUpdates([]byte(`{"ok":true,"result":[{"update_id":43},{"update_id":45}]}`), 42)
	if err != nil { t.Fatal(err) }
	if len(updates) != 2 || next != 46 { t.Fatalf("unexpected result: len=%d next=%d", len(updates), next) }
}

func TestPersistentDailyDigestAndWizardKeysAreDeterministic(t *testing.T) {
	if digestDatePrefix+"2026-08-17" != "telegram_daily_digest_2026-08-17" { t.Fatal("unexpected digest key") }
	if wizardPrefix+"123" != "telegram_wizard_123" { t.Fatal("unexpected wizard key") }
	_ = time.Now()
}
