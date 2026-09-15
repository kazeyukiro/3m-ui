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

// Create returns after the DB write while a slow ApplyConfig may still be running.
// The final applied config must still include the newly created listener.
func TestConcurrentCreateDoesNotLoseListenerInFinalConfig(t *testing.T) {
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
	if err := svc.Create(listener); err != nil {
		t.Fatal(err)
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
	// Flush the debounced apply scheduled by Create so the final config includes the new node.
	svc.FlushConfigApplyForTest()
	applier.mu.Lock()
	defer applier.mu.Unlock()
	if len(applier.configs) == 0 {
		t.Fatal("expected at least one applied config")
	}
	last := applier.configs[len(applier.configs)-1]
	if !strings.Contains(last, "new-listener") {
		t.Fatalf("final config missing new listener: %s", last)
	}
}
