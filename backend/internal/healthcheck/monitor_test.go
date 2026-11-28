package healthcheck

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// --- test doubles ---

type stubProber struct {
	mu     sync.Mutex
	errs   []error // consumed in order; last error repeats
	result error
}

func (p *stubProber) Probe(ctx context.Context, s Settings) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.errs) > 0 {
		e := p.errs[0]
		p.errs = p.errs[1:]
		p.result = e
		return e
	}
	return p.result
}

type stubCore struct {
	mu        sync.Mutex
	running   bool
	restarts  int
	restartFn func() error
}

func (c *stubCore) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}
func (c *stubCore) Restart() error {
	c.mu.Lock()
	c.restarts++
	fn := c.restartFn
	c.mu.Unlock()
	if fn != nil {
		return fn()
	}
	return nil
}
func (c *stubCore) restartCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.restarts
}
func (c *stubCore) setRunning(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = v
}

func enabledSettings() Settings {
	s := DefaultSettings()
	s.Enabled = true
	s.URL = "https://probe.example/ok"
	return s
}

// --- probe behavior (real HTTP) ---

func TestHTTPProberSuccessAndStatuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/200":
			w.WriteHeader(http.StatusOK)
		case "/404":
			w.WriteHeader(http.StatusNotFound)
		case "/500":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	for _, tc := range []struct {
		path string
		ok   bool
	}{
		{"/200", true},
		{"/404", true}, // 4xx does not mean a dead tunnel
		{"/500", false},
	} {
		s := DefaultSettings()
		s.URL = srv.URL + tc.path
		err := httpProber{}.Probe(context.Background(), s)
		if tc.ok && err != nil {
			t.Errorf("%s: expected success, got %v", tc.path, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected failure, got nil", tc.path)
		}
	}
}

func TestHTTPProberRejectsBadProxyScheme(t *testing.T) {
	s := DefaultSettings()
	s.ProxyURL = "ftp://127.0.0.1:21"
	err := httpProber{}.Probe(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for unsupported proxy scheme")
	}
	var use *unsupportedProxySchemeError
	if !errors.As(err, &use) {
		t.Fatalf("expected unsupportedProxySchemeError, got %T", err)
	}
}

func TestHTTPProberRespectsTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := DefaultSettings()
	s.URL = srv.URL
	s.Timeout = 1 // clamped to >=1s by Normalize, so call prober directly with tiny context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := (httpProber{}).Probe(ctx, s); err == nil {
		t.Fatal("expected timeout error")
	}
}

// --- state machine ---

func TestDisabledSettingsNeverProbeOrRestart(t *testing.T) {
	prober := &stubProber{errs: []error{errors.New("fail"), errors.New("fail"), errors.New("fail")}}
	core := &stubCore{running: true}
	m := NewMonitor(prober, core)
	m.Apply(DefaultSettings()) // disabled

	for i := 0; i < 5; i++ {
		m.tick()
	}
	if core.restartCount() != 0 {
		t.Fatalf("disabled monitor restarted core %d times", core.restartCount())
	}
	if m.Status().ConsecutiveFailures != 0 {
		t.Fatalf("disabled monitor counted failures: %d", m.Status().ConsecutiveFailures)
	}
}

func TestCoreNotRunningSkipsProbe(t *testing.T) {
	prober := &stubProber{result: errors.New("fail")}
	core := &stubCore{running: false}
	m := NewMonitor(prober, core)
	s := enabledSettings()
	s.Failures = 2
	m.Apply(s)

	m.tick()
	m.tick()
	m.tick()
	if core.restartCount() != 0 {
		t.Fatalf("core not running but restarted %d times", core.restartCount())
	}
}

func TestConsecutiveFailuresTriggerRestart(t *testing.T) {
	prober := &stubProber{result: errors.New("probe failed")}
	core := &stubCore{running: true}
	m := NewMonitor(prober, core)
	s := enabledSettings()
	s.Failures = 3
	s.Cooldown = 1
	m.Apply(s)

	m.tick() // 1/3
	m.tick() // 2/3
	if core.restartCount() != 0 {
		t.Fatalf("restarted too early at %d", core.restartCount())
	}
	m.tick() // 3/3 -> restart
	if core.restartCount() != 1 {
		t.Fatalf("expected 1 restart, got %d", core.restartCount())
	}
	st := m.Status()
	if st.TotalRestarts != 1 || st.ConsecutiveFailures != 0 {
		t.Fatalf("bad status after restart: %+v", st)
	}
	if st.LastRestart == "" || st.LastError == "" {
		t.Fatalf("status missing last_restart/last_error: %+v", st)
	}
}

func TestSuccessResetsFailureCounter(t *testing.T) {
	prober := &stubProber{errs: []error{errors.New("fail"), errors.New("fail"), nil}}
	core := &stubCore{running: true}
	m := NewMonitor(prober, core)
	s := enabledSettings()
	s.Failures = 3
	m.Apply(s)

	m.tick() // 1
	m.tick() // 2
	m.tick() // success -> reset
	if m.Status().ConsecutiveFailures != 0 {
		t.Fatalf("failures not reset after success: %d", m.Status().ConsecutiveFailures)
	}
	if core.restartCount() != 0 {
		t.Fatalf("unexpected restart: %d", core.restartCount())
	}
}

func TestCooldownSuppressesRestart(t *testing.T) {
	prober := &stubProber{result: errors.New("fail")}
	core := &stubCore{running: true}
	m := NewMonitor(prober, core)
	s := enabledSettings()
	s.Failures = 1
	s.Cooldown = 3600 // long cooldown
	m.Apply(s)

	m.tick() // restart #1, cooldown starts
	m.tick() // still in cooldown -> suppressed
	m.tick() // suppressed
	if core.restartCount() != 1 {
		t.Fatalf("cooldown failed: restarts = %d, want 1", core.restartCount())
	}
}

func TestApplyResetsFailuresOnSettingsChange(t *testing.T) {
	prober := &stubProber{result: errors.New("fail")}
	core := &stubCore{running: true}
	m := NewMonitor(prober, core)
	s := enabledSettings()
	s.Failures = 5
	m.Apply(s)

	m.tick()
	m.tick()
	if m.Status().ConsecutiveFailures != 2 {
		t.Fatalf("expected 2 failures, got %d", m.Status().ConsecutiveFailures)
	}
	// Changing settings resets the counter so an edited threshold cannot
	// immediately restart the core.
	s.Failures = 5
	s.Interval = 45
	m.Apply(s)
	if m.Status().ConsecutiveFailures != 0 {
		t.Fatalf("Apply did not reset failures: %d", m.Status().ConsecutiveFailures)
	}
}

func TestRestartErrorDoesNotCrashLoop(t *testing.T) {
	prober := &stubProber{result: errors.New("fail")}
	core := &stubCore{running: true, restartFn: func() error { return errors.New("restart failed") }}
	m := NewMonitor(prober, core)
	s := enabledSettings()
	s.Failures = 1
	s.Cooldown = 3600
	m.Apply(s)

	m.tick()
	if m.Status().LastError == "" || m.Status().LastError[:7] != "restart" {
		t.Fatalf("expected restart error recorded, got %q", m.Status().LastError)
	}
	if m.Status().TotalRestarts != 0 {
		t.Fatalf("failed restart must not increment total: %d", m.Status().TotalRestarts)
	}
	// A failed restart must not set a cooldown window; the next tick retries.
	core.restartFn = nil
	m.tick()
	if core.restartCount() != 2 {
		t.Fatalf("expected retry after failed restart, got %d restarts", core.restartCount())
	}
}

func TestSettingsSnapshotIsCopy(t *testing.T) {
	m := NewMonitor(&stubProber{}, &stubCore{running: true})
	s := enabledSettings()
	m.Apply(s)
	got := m.Settings()
	got.Enabled = false
	if !m.Settings().Enabled {
		t.Fatal("Settings() must return a copy, not the internal struct")
	}
}
