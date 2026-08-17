package telegram

import (
	"errors"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:telegram-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.PanelSetting{}, &models.ProxyUser{}); err != nil { t.Fatal(err) }
	return db
}

func TestUsageRejectsLookupForNormalUser(t *testing.T) {
	db := testDB(t)
	if err := SaveSettings(db, Settings{Enabled: true, BotToken: "token", ChatIDs: []string{"999"}}); err != nil { t.Fatal(err) }
	b := NewBot(db, nil, nil)
	got, _ := b.handleCommand("100", "/usage alice")
	if !strings.Contains(got, "Permission denied") { t.Fatalf("expected denial, got %q", got) }
}

func TestInboundAndRestartAreAdminOnly(t *testing.T) {
	db := testDB(t)
	if err := SaveSettings(db, Settings{Enabled: true, BotToken: "token", ChatIDs: []string{"999"}}); err != nil { t.Fatal(err) }
	b := NewBot(db, nil, nil)
	for _, command := range []string{"/inbound example", "/restart"} {
		got, _ := b.handleCommand("100", command)
		if !strings.Contains(got, "administrators only") { t.Fatalf("%s: got %q", command, got) }
	}
}

func TestBindUserRejectsExistingTelegramBinding(t *testing.T) {
	db := testDB(t)
	id := int64(12345)
	first := models.ProxyUser{Username: "first", UUID: "u1", TelegramID: &id}
	second := models.ProxyUser{Username: "second", UUID: "u2"}
	if err := db.Create(&first).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&second).Error; err != nil { t.Fatal(err) }
	b := NewBot(db, nil, nil)
	got := b.bindUser("second", "12345")
	if !strings.Contains(got, "already bound") { t.Fatalf("expected duplicate binding rejection, got %q", got) }
}

func TestBindUserRejectsAlreadyBoundClient(t *testing.T) {
	db := testDB(t)
	id := int64(111)
	first := models.ProxyUser{Username: "first", UUID: "u1"}
	second := models.ProxyUser{Username: "second", UUID: "u2", TelegramID: &id}
	if err := db.Create(&first).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&second).Error; err != nil { t.Fatal(err) }
	b := NewBot(db, nil, nil)
	got := b.bindUser("second", "222")
	if !strings.Contains(got, "already bound") { t.Fatalf("expected existing-client rejection, got %q", got) }
}

func TestWizardStateIsSinglePerChat(t *testing.T) {
	b := NewBot(nil, nil, nil)
	b.wizardMu.Lock()
	b.wizards["123"] = &addClientWizard{ListenerID: 1}
	_, active := b.wizards["123"]
	b.wizardMu.Unlock()
	if !active { t.Fatal("expected active wizard") }
}

func TestHTMLIsEscapedInUsage(t *testing.T) {
	u := models.ProxyUser{Username: "<admin>", TrafficUsed: 1024, TrafficLimit: 2048}
	got := formatUserUsage(u)
	if strings.Contains(got, "<admin>") || !strings.Contains(got, "&lt;admin&gt;") { t.Fatalf("username was not escaped: %q", got) }
}

func TestSplitTelegramHTMLNeverExceedsLimit(t *testing.T) {
	text := "<b>header</b>\n" + strings.Repeat("测试内容 ", 1500)
	chunks := splitTelegramHTML(text, 4096)
	if len(chunks) < 2 { t.Fatalf("expected message to be split") }
	for i, chunk := range chunks {
		if len([]rune(chunk)) > 4096 { t.Fatalf("chunk %d exceeds Telegram limit: %d", i, len([]rune(chunk))) }
	}
	if !strings.HasPrefix(chunks[0], "<b>header</b>") { t.Fatalf("unexpected first chunk") }
}

func TestSplitTelegramHTMLReopensActiveTag(t *testing.T) {
	text := "<b>" + strings.Repeat("x", 5000) + "</b>"
	chunks := splitTelegramHTML(text, 4096)
	if len(chunks) < 2 { t.Fatal("expected split") }
	for _, chunk := range chunks {
		if !strings.HasPrefix(chunk, "<b>") { t.Fatalf("chunk does not reopen b tag") }
		if !strings.HasSuffix(chunk, "</b>") { t.Fatalf("chunk does not close b tag") }
	}
}

func TestParseUpdatesPreservesTelegramError(t *testing.T) {
	_, _, err := parseUpdates([]byte(`{"ok":false,"description":"Conflict: terminated by other getUpdates request"}`), 7)
	if err == nil || !strings.Contains(err.Error(), "Conflict: terminated by other getUpdates request") { t.Fatalf("Telegram error description was lost: %v", err) }
}

func TestParseUpdatesAdvancesOffsetToHighestUpdate(t *testing.T) {
	raw := []byte(`{"ok":true,"result":[{"update_id":10},{"update_id":14},{"update_id":12}]}`)
	updates, next, err := parseUpdates(raw, 7)
	if err != nil { t.Fatal(err) }
	if len(updates) != 3 || next != 15 { t.Fatalf("expected 3 updates and offset 15, got %d/%d", len(updates), next) }
}

func TestUpdateOffsetPersistsAcrossLoads(t *testing.T) {
	db := testDB(t)
	if err := SaveUpdateOffset(db, 1234); err != nil { t.Fatal(err) }
	if got := LoadUpdateOffset(db); got != 1234 { t.Fatalf("expected 1234, got %d", got) }
	if err := SaveUpdateOffset(db, 5678); err != nil { t.Fatal(err) }
	if got := LoadUpdateOffset(db); got != 5678 { t.Fatalf("expected 5678, got %d", got) }
}

func TestSetCommandsRetryRetriesAndStopsOnSuccess(t *testing.T) {
	calls := 0
	err := setCommandsWithRetryFn(func() error {
		calls++
		if calls < 3 { return errors.New("temporary Telegram API failure") }
		return nil
	}, 5)
	if err != nil { t.Fatal(err) }
	if calls != 3 { t.Fatalf("expected 3 attempts, got %d", calls) }
}

func TestSetCommandsRetryReturnsLastError(t *testing.T) {
	calls := 0
	want := errors.New("Telegram API unavailable")
	err := setCommandsWithRetryFn(func() error { calls++; return want }, 3)
	if !errors.Is(err, want) { t.Fatalf("expected last error, got %v", err) }
	if calls != 3 { t.Fatalf("expected 3 attempts, got %d", calls) }
}
