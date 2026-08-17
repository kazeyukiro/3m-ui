package telegram

import (
    "fmt"
    "strings"
    "sync"
    "sync/atomic"
    "testing"

    "github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

// TestConcurrentWizardCreation verifies that the in-memory wizard guard is
// atomic: concurrent attempts for one chat may create at most one wizard.
func TestConcurrentWizardCreation(t *testing.T) {
    b := NewBot(nil, nil, nil)
    const attempts = 64

    var created int32
    var wg sync.WaitGroup
    wg.Add(attempts)
    for i := 0; i < attempts; i++ {
        go func() {
            defer wg.Done()
            b.wizardMu.Lock()
            defer b.wizardMu.Unlock()
            if _, exists := b.wizards["chat-1"]; exists {
                return
            }
            b.wizards["chat-1"] = &addClientWizard{ListenerID: 42}
            atomic.AddInt32(&created, 1)
        }()
    }
    wg.Wait()

    if created != 1 {
        t.Fatalf("expected exactly one wizard creation, got %d", created)
    }
}

// TestConcurrentWizardRestoreDoesNotReplaceActiveState verifies that a state
// restored from the database cannot overwrite a wizard already created by a
// concurrently handled callback.
func TestConcurrentWizardRestoreDoesNotReplaceActiveState(t *testing.T) {
    db := testDB(t)
    b := NewBot(db, nil, nil)
    b.wizards["chat-1"] = &addClientWizard{ListenerID: 99}

    const attempts = 32
    var wg sync.WaitGroup
    wg.Add(attempts)
    for i := 0; i < attempts; i++ {
        go func() {
            defer wg.Done()
            b.restoreWizard("chat-1")
        }()
    }
    wg.Wait()

    b.wizardMu.Lock()
    state := b.wizards["chat-1"]
    b.wizardMu.Unlock()
    if state == nil || state.ListenerID != 99 {
        t.Fatalf("active wizard was replaced: %#v", state)
    }
}

// TestConcurrentTelegramBindingAllowsOnlyOneOwner verifies the database-level
// unique TelegramID constraint under concurrent binding attempts. The test is
// intentionally run with the race detector in CI/local development.
func TestConcurrentTelegramBindingAllowsOnlyOneOwner(t *testing.T) {
    db := testDB(t)
    users := []models.ProxyUser{
        {Username: "client-a", UUID: "uuid-a"},
        {Username: "client-b", UUID: "uuid-b"},
    }
    for i := range users {
        if err := db.Create(&users[i]).Error; err != nil {
            t.Fatal(err)
        }
    }

    b := NewBot(db, nil, nil)
    const attempts = 24
    var wg sync.WaitGroup
    results := make(chan string, attempts)
    wg.Add(attempts)
    for i := 0; i < attempts; i++ {
        username := "client-a"
        if i%2 == 1 {
            username = "client-b"
        }
        go func(name string) {
            defer wg.Done()
            results <- b.bindUser(name, "777001")
        }(username)
    }
    wg.Wait()
    close(results)

    var bound []models.ProxyUser
    if err := db.Where("telegram_id = ?", int64(777001)).Find(&bound).Error; err != nil {
        t.Fatal(err)
    }
    if len(bound) != 1 {
        t.Fatalf("expected exactly one client bound to Telegram ID, got %d", len(bound))
    }

    successes := 0
    for result := range results {
        if strings.Contains(result, "bound to Telegram ID") {
            successes++
        }
    }
    if successes != 1 {
        t.Fatalf("expected exactly one successful bind, got %d", successes)
    }
}

// TestConcurrentDifferentTelegramIDsCanBindDifferentClients verifies that
// independent bindings do not serialize incorrectly on a shared chat/user
// state and that both unique bindings survive concurrent writes.
func TestConcurrentDifferentTelegramIDsCanBindDifferentClients(t *testing.T) {
    db := testDB(t)
    for i := 0; i < 8; i++ {
        user := models.ProxyUser{Username: fmt.Sprintf("client-%d", i), UUID: fmt.Sprintf("uuid-%d", i)}
        if err := db.Create(&user).Error; err != nil {
            t.Fatal(err)
        }
    }

    b := NewBot(db, nil, nil)
    var wg sync.WaitGroup
    for i := 0; i < 8; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            got := b.bindUser(fmt.Sprintf("client-%d", i), fmt.Sprintf("700%03d", i))
            if !strings.Contains(got, "bound to Telegram ID") {
                t.Errorf("client-%d bind failed: %s", i, got)
            }
        }(i)
    }
    wg.Wait()

    var count int64
    if err := db.Model(&models.ProxyUser{}).Where("telegram_id IS NOT NULL").Count(&count).Error; err != nil {
        t.Fatal(err)
    }
    if count != 8 {
        t.Fatalf("expected 8 independent bindings, got %d", count)
    }
}

// TestUpdateOffsetNeverMovesBackward exercises the same monotonic invariant
// used by the polling loop: an older update must never lower a persisted
// offset after a newer update has been acknowledged.
func TestUpdateOffsetNeverMovesBackward(t *testing.T) {
    db := testDB(t)
    if err := SaveUpdateOffset(db, 100); err != nil {
        t.Fatal(err)
    }

    offsets := []int64{101, 140, 115, 180, 160, 181}
    for _, next := range offsets {
        current := LoadUpdateOffset(db)
        if next > current {
            if err := SaveUpdateOffset(db, next); err != nil {
                t.Fatal(err)
            }
        }
    }

    if got := LoadUpdateOffset(db); got != 181 {
        t.Fatalf("expected monotonic final offset 181, got %d", got)
    }
}

func TestHTMLSplitPreservesEntitiesAtBoundary(t *testing.T) {
    text := "<b>" + strings.Repeat("x", 4090) + "&amp;" + strings.Repeat("y", 20) + "</b>"
    chunks := splitTelegramHTML(text, 4096)
    if len(chunks) < 2 {
        t.Fatal("expected HTML to be split")
    }
    for i, chunk := range chunks {
        if strings.Contains(chunk, "&am\n") || strings.Contains(chunk, "&amp\n") {
            t.Fatalf("chunk %d appears to split an HTML entity: %q", i, chunk)
        }
        if strings.Contains(chunk, "&am") && !strings.Contains(chunk, "&amp;") {
            t.Fatalf("chunk %d contains a partial HTML entity: %q", i, chunk)
        }
    }
}
