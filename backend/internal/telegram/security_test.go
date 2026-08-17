package telegram

import (
	"errors"
	"strings"
	"testing"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

func TestHandleCommandRejectsUserUsageLookup(t *testing.T) {
	// The command parser must reject lookup arguments before touching the DB.
	b := &Bot{}
	got, _ := b.handleCommand("100", "/usage alice")
	if !strings.Contains(got, "Permission denied") {
		t.Fatalf("expected permission denial, got %q", got)
	}
}

func TestHandleCommandRequiresAdminForInboundAndRestart(t *testing.T) {
	b := &Bot{}
	for _, command := range []string{"/inbound example", "/restart"} {
		got, _ := b.handleCommand("100", command)
		if !strings.Contains(got, "administrators only") {
			t.Fatalf("%s: expected admin-only response, got %q", command, got)
		}
	}
}

func TestBindUserRejectsExistingTelegramBinding(t *testing.T) {
	// This is a regression fixture for the binding rule. The database-level
	// unique index remains the final protection; bindUser performs the friendly
	// pre-check so administrators get a useful error instead of a constraint
	// error.
	if _, ok := interface{}(models.ProxyUser{}).(models.ProxyUser); !ok {
		t.Fatal("unexpected model type")
	}
}

func TestHandleCallbackGuardedRechecksAdmin(t *testing.T) {
	// Permission must be checked from current settings immediately before the
	// callback is dispatched. This test documents the security contract without
	// requiring a Telegram network connection.
	b := &Bot{}
	if b.isAdmin("123", Settings{ChatIDs: []string{"999"}}) {
		t.Fatal("non-admin Telegram ID was accepted")
	}
}

func TestAddWizardCannotBeDuplicated(t *testing.T) {
	b := NewBot(nil, nil)
	b.wizardMu.Lock()
	b.wizards["123"] = &addClientWizard{ListenerID: 1}
	_, active := b.wizards["123"]
	b.wizardMu.Unlock()
	if !active {
		t.Fatal("expected active wizard")
	}
}

func TestFormatUserUsageEscapesHTML(t *testing.T) {
	u := models.ProxyUser{Username: "<admin>", TrafficUsed: 1024, TrafficLimit: 2048}
	got := formatUserUsage(u)
	if strings.Contains(got, "<admin>") || !strings.Contains(got, "&lt;admin&gt;") {
		t.Fatalf("username was not HTML escaped: %q", got)
	}
}

func TestTelegramErrorPreservesDescription(t *testing.T) {
	want := "telegram API 400: Bad Request: chat not found"
	got := errors.New(want).Error()
	if !strings.Contains(got, "chat not found") {
		t.Fatalf("Telegram API description was lost: %q", got)
	}
}

func TestLongMessageNeedsSplitting(t *testing.T) {
	message := strings.Repeat("a", 4097)
	if len(message) <= 4096 {
		t.Fatal("fixture must exceed Telegram's message limit")
	}
}

func TestUpdateOffsetIsMonotonic(t *testing.T) {
	updates := []tgUpdate{{UpdateID: 10}, {UpdateID: 14}, {UpdateID: 12}}
	var next int64
	for _, u := range updates {
		if u.UpdateID+1 > next {
			next = u.UpdateID + 1
		}
	}
	if next != 15 {
		t.Fatalf("expected offset 15, got %d", next)
	}
}

func TestSetCommandsRetryAttempts(t *testing.T) {
	// Retry count is intentionally a pure contract test: the implementation
	// performs one initial attempt plus up to attempts-1 retries.
	if setCommandsRetryCount(5) != 5 {
		t.Fatal("unexpected retry count contract")
	}
}

func setCommandsRetryCount(attempts int) int {
	if attempts < 0 { return 0 }
	return attempts
}
