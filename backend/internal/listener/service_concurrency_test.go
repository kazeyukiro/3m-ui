package listener

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database"
	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
)

type orderedConfigApplier struct {
	mu      sync.Mutex
	calls   int
	configs []string
	started chan struct{}
	resume  chan struct{}
}

func (a *orderedConfigApplier) ApplyConfig(content string) error {
	a.mu.Lock()
	a.calls++
	first := a.calls == 1
	a.mu.Unlock()
	if first {
		close(a.started)
		<-a.resume
	}
	a.mu.Lock()
	a.configs = append(a.configs, content)
	a.mu.Unlock()
	return nil
}

func (a *orderedConfigApplier) ApplyConfigDeferredRestart(content string) error {
	return a.ApplyConfig(content)
}

func TestRegenerationCannotOverwriteConcurrentListenerCreate(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	applier := &orderedConfigApplier{started: make(chan struct{}), resume: make(chan struct{})}
	resume := sync.OnceFunc(func() { close(applier.resume) })
	t.Cleanup(resume)
	svc := NewService(db, "unused.yaml", applier)
	regenerated := make(chan error, 1)
	go func() { regenerated <- svc.RegenerateConfig() }()
	select {
	case <-applier.started:
	case <-time.After(5 * time.Second):
		t.Fatal("configuration generation did not reach the applier")
	}
	listener := &models.Listener{Name: "new-listener", Protocol: "shadowsocks", Port: "19388", BindAddress: "127.0.0.1", Enabled: true, Config: `{"cipher":"aes-128-gcm","password":"test-password"}`}
	created := make(chan error, 1)
	go func() { created <- svc.Create(listener) }()
	var createErr error
	completedEarly := false
	select {
	case createErr = <-created:
		completedEarly = true
		t.Error("listener creation overtook the pending configuration application")
	case <-time.After(250 * time.Millisecond):
	}
	resume()
	select {
	case err := <-regenerated:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("configuration application did not finish")
	}
	if !completedEarly {
		select {
		case createErr = <-created:
		case <-time.After(5 * time.Second):
			t.Fatal("listener creation did not finish")
		}
	}
	if createErr != nil {
		t.Fatal(createErr)
	}
	if _, err := svc.GetByID(listener.ID); err != nil {
		t.Fatal(err)
	}
	applier.mu.Lock()
	defer applier.mu.Unlock()
	if len(applier.configs) != 2 || !strings.Contains(applier.configs[len(applier.configs)-1], listener.Name) {
		t.Fatal("the final configuration lost the successfully created listener")
	}
}
