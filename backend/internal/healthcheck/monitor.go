package healthcheck

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Prober performs a single tunnel health probe. It is an interface so tests
// can inject deterministic failures without relying on real network access.
type Prober interface {
	Probe(ctx context.Context, s Settings) error
}

// httpProber probes the configured URL with an optional outbound proxy.
// A probe succeeds when the endpoint returns any response with a status code
// below 500. Network errors, TLS errors and timeouts are failures. Status
// codes >= 500 are treated as failures because a healthy tunnel that reaches
// a broken origin should not count as "up", but 4xx (e.g. a 404 on a changed
// probe path) does not indicate a dead tunnel and is deliberately accepted.
type httpProber struct{}

func (httpProber) Probe(ctx context.Context, s Settings) error {
	transport := http.DefaultTransport
	if s.ProxyURL != "" {
		proxyURL, err := url.Parse(s.ProxyURL)
		if err != nil {
			return err
		}
		scheme := strings.ToLower(proxyURL.Scheme)
		switch scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return &unsupportedProxySchemeError{scheme: scheme}
		}
		transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Drain a bounded amount so the connection can be reused; never copy an
	// unbounded body (a probe URL could return a large payload).
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 500 {
		return &badStatusError{code: resp.StatusCode}
	}
	return nil
}

type unsupportedProxySchemeError struct{ scheme string }

func (e *unsupportedProxySchemeError) Error() string {
	return "unsupported health check proxy scheme: " + e.scheme
}

type badStatusError struct{ code int }

func (e *badStatusError) Error() string {
	return "health check returned status " + http.StatusText(e.code)
}

// coreState reports whether the Mihomo core is currently expected to run.
// The monitor only probes and restarts a core that is supposed to be up, so it
// never starts a core the admin intentionally stopped.
type coreState interface {
	Running() bool
	Restart() error
}

// Monitor is a lightweight watchdog that periodically probes a URL through
// the tunnel and restarts the core after a configurable number of consecutive
// failures, respecting a cooldown between restarts.
//
// It starts disabled by default and only becomes active once Enable(true) is
// called (or LoadFromDB finds a persisted enabled setting). Settings can be
// changed at runtime via Apply without restarting the panel.
type Monitor struct {
	mu       sync.Mutex
	settings Settings

	prober Prober
	core   coreState

	startOnce sync.Once
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// restartMu serializes restarts independently of the settings mutex so a
	// slow Restart never blocks status reads.
	restartMu sync.Mutex

	// probeNow lets a settings change or a manual test trigger an immediate
	// probe without waiting for the next ticker interval. Buffered size 1 so a
	// signal is never dropped while a probe is in flight.
	probeNow chan struct{}

	consecutiveFailures int
	lastRestart         time.Time
	lastProbe           time.Time
	lastError           string
	totalRestarts       uint64
}

func NewMonitor(prober Prober, core coreState) *Monitor {
	if prober == nil {
		prober = httpProber{}
	}
	return &Monitor{
		settings: DefaultSettings(),
		prober:   prober,
		core:     core,
		stopCh:   make(chan struct{}),
		probeNow: make(chan struct{}, 1),
	}
}

// Apply replaces the active settings atomically. A non-nil change resets the
// failure counter so a threshold edit cannot immediately restart the core.
func (m *Monitor) Apply(s Settings) {
	m.mu.Lock()
	prev := m.settings
	m.settings = s.Normalize()
	m.consecutiveFailures = 0
	m.mu.Unlock()
	if prev.Interval != m.settings.Interval {
		m.signalProbe()
	}
}

// Settings returns a copy of the active settings.
func (m *Monitor) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

// Status is a snapshot exposed to the UI for observability.
type Status struct {
	Enabled             bool   `json:"enabled"`
	Interval            int    `json:"interval"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	TotalRestarts       uint64 `json:"total_restarts"`
	LastProbe           string `json:"last_probe"`
	LastRestart         string `json:"last_restart"`
	LastError           string `json:"last_error"`
}

func (m *Monitor) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := Status{
		Enabled:             m.settings.Enabled,
		Interval:            m.settings.Interval,
		ConsecutiveFailures: m.consecutiveFailures,
		TotalRestarts:       m.totalRestarts,
		LastError:           m.lastError,
	}
	if !m.lastProbe.IsZero() {
		st.LastProbe = m.lastProbe.Format(time.RFC3339)
	}
	if !m.lastRestart.IsZero() {
		st.LastRestart = m.lastRestart.Format(time.RFC3339)
	}
	return st
}

func (m *Monitor) signalProbe() {
	select {
	case m.probeNow <- struct{}{}:
	default:
	}
}

// Start launches the watchdog goroutine exactly once. It is safe to call
// multiple times.
func (m *Monitor) Start() {
	m.startOnce.Do(func() {
		m.wg.Add(1)
		go m.loop()
	})
}

// Stop terminates the watchdog goroutine.
func (m *Monitor) Stop() {
	select {
	case <-m.stopCh:
		return
	default:
	}
	close(m.stopCh)
	m.wg.Wait()
}

func (m *Monitor) loop() {
	defer m.wg.Done()
	for {
		m.mu.Lock()
		enabled := m.settings.Enabled
		interval := m.settings.intervalDuration()
		m.mu.Unlock()

		if !enabled {
			// When disabled, wake briefly to notice Enable/Apply or Stop.
			select {
			case <-m.stopCh:
				return
			case <-m.probeNow:
				continue
			case <-time.After(500 * time.Millisecond):
				continue
			}
		}

		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			m.tick()
		case <-m.probeNow:
			stopTimer(timer)
		case <-m.stopCh:
			stopTimer(timer)
			return
		}
	}
}

// tick performs one probe and decides whether a restart is warranted.
func (m *Monitor) tick() {
	m.mu.Lock()
	s := m.settings
	if !s.Enabled {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// Never probe or restart a core the admin intentionally stopped.
	if m.core == nil || !m.core.Running() {
		m.mu.Lock()
		m.consecutiveFailures = 0
		m.mu.Unlock()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeoutDuration())
	err := m.prober.Probe(ctx, s)
	cancel()

	now := time.Now()
	m.mu.Lock()
	m.lastProbe = now
	if err == nil {
		m.consecutiveFailures = 0
		m.lastError = ""
		m.mu.Unlock()
		return
	}
	m.consecutiveFailures++
	failures := m.consecutiveFailures
	m.lastError = err.Error()
	cooldownUntil := m.lastRestart.Add(s.cooldownDuration())
	inCooldown := now.Before(cooldownUntil)
	m.mu.Unlock()

	log.Printf("healthcheck: probe failed (%d/%d): %v", failures, s.Failures, err)
	if failures < s.Failures {
		return
	}
	if inCooldown {
		log.Printf("healthcheck: restart suppressed by cooldown (until %s)", cooldownUntil.Format(time.RFC3339))
		return
	}

	// Restart through the service so all existing guards (core-update busy,
	// config validation, lifecycle serialization) are respected. Only one
	// restart may be in flight; the mutex is checked again after acquiring to
	// avoid a duplicate when a probe raced with a manual restart.
	m.restartMu.Lock()
	defer m.restartMu.Unlock()
	m.mu.Lock()
	if m.consecutiveFailures < s.Failures || time.Now().Before(m.lastRestart.Add(s.cooldownDuration())) {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	log.Printf("healthcheck: %d consecutive failures; restarting Mihomo core", failures)
	if restartErr := m.core.Restart(); restartErr != nil {
		log.Printf("healthcheck: core restart failed: %v", restartErr)
		m.mu.Lock()
		m.lastError = "restart failed: " + restartErr.Error()
		m.mu.Unlock()
		return
	}
	m.mu.Lock()
	m.lastRestart = time.Now()
	m.totalRestarts++
	m.consecutiveFailures = 0
	m.mu.Unlock()
	log.Printf("healthcheck: core restarted (total %d); cooldown %ds", m.totalRestarts, s.Cooldown)
}

func stopTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}
