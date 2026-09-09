package mihomo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	mihomoconfig "github.com/kazeyukiro/3m-ui/backend/internal/mihomo/config"
	"gopkg.in/yaml.v3"
)

const connectionResultTTL = time.Minute

var errListenerNotApplied = errors.New("listener is missing from the applied configuration")

type ConnectionStep struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}

type ConnectionCheck struct {
	State     string           `json:"state"`
	Reason    string           `json:"reason"`
	CheckedAt time.Time        `json:"checked_at"`
	ExpiresAt time.Time        `json:"expires_at"`
	Source    string           `json:"source"`
	Address   string           `json:"address,omitempty"`
	Target    string           `json:"target,omitempty"`
	DelayMS   int              `json:"delay_ms,omitempty"`
	Steps     []ConnectionStep `json:"steps"`
}

type cachedConnectionCheck struct {
	result          ConnectionCheck
	pid             int
	configHash      [32]byte
	listenerHash    [32]byte
	clientUnchanged func() bool
}

func connectionListenerHash(l models.Listener) [32]byte {
	data, _ := json.Marshal(l)
	return sha256.Sum256(data)
}

func (s *Service) cachedConnectionResult(l models.Listener, pid int, configHash [32]byte) *ConnectionCheck {
	id := l.ID
	s.connectionResultsMu.Lock()
	cached, ok := s.connectionResults[id]
	s.connectionResultsMu.Unlock()
	if !ok {
		return nil
	}
	if cached.pid != pid || cached.configHash != configHash || cached.listenerHash != connectionListenerHash(l) || !time.Now().Before(cached.result.ExpiresAt) ||
		(cached.clientUnchanged != nil && !cached.clientUnchanged()) {
		s.connectionResultsMu.Lock()
		// Validation must not evict a newer result stored in the meantime.
		if s.connectionResults[id].result.CheckedAt.Equal(cached.result.CheckedAt) {
			delete(s.connectionResults, id)
		}
		s.connectionResultsMu.Unlock()
		return nil
	}
	copy := cached.result
	return &copy
}

// CheckConnection runs an isolated client, never a reload of the serving core.
// The callback provides the same exported configuration used by clients. It is
// invoked under applyMu while the applied configuration is snapshotted.
func (s *Service) CheckConnection(ctx context.Context, l models.Listener, export func() (string, error), reuse bool) ListenerRuntime {
	if reuse && l.Enabled {
		result := s.ListenerRuntime([]models.Listener{l}, true)[0]
		if cached := result.ConnectionCheck; cached != nil && (cached.State == "available" || cached.State == "unavailable") {
			return result
		}
	}
	check := ConnectionCheck{State: "unknown", Source: "local", Steps: []ConnectionStep{}}
	finish := func(reason string) ListenerRuntime {
		// Observe sockets after the request so an earlier startup observation
		// cannot contradict the completed connection test.
		result := s.ListenerRuntime([]models.Listener{l}, true)[0]
		check.Reason = reason
		check.CheckedAt = time.Now().UTC()
		check.ExpiresAt = check.CheckedAt.Add(connectionResultTTL)
		result.CheckedAt = check.CheckedAt
		result.ConnectionCheck = &check
		return result
	}
	if !l.Enabled {
		check.State = "disabled"
		return finish("disabled")
	}
	if !s.connectionCheckMu.TryLock() {
		return finish("check_busy")
	}
	defer s.connectionCheckMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	s.applyMu.Lock()
	pid := s.runtimePID()
	content, readErr := s.cm.ReadConfig()
	clientYAML, exportErr := export()
	clientHash, exportOK := sha256.Sum256([]byte(clientYAML)), exportErr == nil
	// Re-read current node/access settings and active credentials. Only the
	// fingerprint and read callback are retained; no client secrets are cached.
	clientUnchanged := func() bool {
		current, err := export()
		return (err == nil) == exportOK && sha256.Sum256([]byte(current)) == clientHash
	}
	var binary string
	if s.pm != nil {
		s.pm.mu.Lock()
		binary = s.pm.binaryPath
		s.pm.mu.Unlock()
	}
	s.applyMu.Unlock()
	if readErr != nil {
		return finish("config_unavailable")
	}
	// Cache only results for the unchanged serving process and configuration.
	defer func() {
		s.applyMu.Lock()
		defer s.applyMu.Unlock()
		current, err := s.cm.ReadConfig()
		if ctx.Err() != nil || err != nil || current != content || s.runtimePID() != pid || !clientUnchanged() {
			check.State, check.Reason = "unknown", "configuration_changed"
			if ctx.Err() != nil {
				check.Reason = "check_interrupted"
			}
			return
		}
		s.connectionResultsMu.Lock()
		defer s.connectionResultsMu.Unlock()
		if s.connectionResults == nil {
			s.connectionResults = make(map[uint]cachedConnectionCheck)
		}
		// Bound the in-memory cache even when many nodes are checked/deleted.
		for id, entry := range s.connectionResults {
			if time.Now().After(entry.result.ExpiresAt) {
				delete(s.connectionResults, id)
			}
		}
		if len(s.connectionResults) >= 256 {
			clear(s.connectionResults)
		}
		s.connectionResults[l.ID] = cachedConnectionCheck{check, pid, sha256.Sum256([]byte(content)), connectionListenerHash(l), clientUnchanged}
	}()
	if pid == 0 {
		check.State = "unavailable"
		return finish("core_stopped")
	}
	if _, err := appliedListener(content, l.Name); err != nil {
		if errors.Is(err, errListenerNotApplied) {
			check.State = "unavailable"
			// A missing listener is a configuration problem, not evidence that
			// its port/address is unsupported. Do not run later client stages.
			reason := "config_not_applied"
			if err := mihomoconfig.ValidateListenerConfig(l.Protocol, l.Config); err != nil {
				reason = "listener_config_invalid"
				if strings.Contains(err.Error(), `field "sni" is not supported`) {
					reason = "listener_sni_unsupported"
				}
			}
			return finish(reason)
		}
		return finish("config_unavailable")
	}
	if exportErr != nil {
		check.Steps = append(check.Steps, ConnectionStep{"client_config", "unknown", "client_config_unavailable"})
		return finish("client_config_unavailable")
	}
	proxy, address, err := exportedClientProxy(clientYAML)
	if err != nil {
		check.Steps = append(check.Steps, ConnectionStep{"client_config", "unknown", "client_endpoint_unavailable"})
		return finish("client_endpoint_unavailable")
	}
	check.Address = address
	check.Steps = append(check.Steps, ConnectionStep{"client_config", "passed", "client_config_ready"})
	runClientCheck(ctx, binary, proxy, &check, []string{"https://www.gstatic.com/generate_204", "https://cp.cloudflare.com/generate_204"})
	return finish(check.Reason)
}

func appliedListener(content, name string) (runtimeListener, error) {
	listeners, err := runtimeListeners(content)
	if err != nil {
		return runtimeListener{}, err
	}
	for _, listener := range listeners {
		if listener.Name == name {
			return listener, nil
		}
	}
	return runtimeListener{}, errListenerNotApplied
}

// Preserve the exported endpoint and protocol options. Rewriting the endpoint
// to loopback would hide broken public addresses and missing port mappings.
func exportedClientProxy(clientYAML string) (map[string]any, string, error) {
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(clientYAML), &doc); err != nil || len(doc.Proxies) == 0 {
		return nil, "", fmt.Errorf("no proxy")
	}
	proxy := doc.Proxies[0]
	address, ok := proxy["server"].(string)
	if !ok || address == "" || address != strings.TrimSpace(address) || strings.ContainsAny(address, "/[] \t\r\n") {
		return nil, "", fmt.Errorf("invalid client server")
	}
	if ip := net.ParseIP(address); ip != nil {
		if ip.IsUnspecified() {
			return nil, "", fmt.Errorf("unspecified client server")
		}
	} else if strings.Contains(address, ":") {
		return nil, "", fmt.Errorf("invalid client server")
	}
	port := fmt.Sprint(proxy["port"])
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return nil, "", fmt.Errorf("a single client port is required")
	}
	// A dependent proxy is absent from this isolated profile. Reject it rather
	// than silently removing the dependency and testing a different route.
	if dialer, exists := proxy["dialer-proxy"]; exists && dialer != "" {
		return nil, "", fmt.Errorf("dependent proxy is unavailable")
	}
	proxy["name"] = "node-check"
	return proxy, net.JoinHostPort(address, strconv.Itoa(p)), nil
}

func runClientCheck(ctx context.Context, binary string, proxy map[string]any, check *ConnectionCheck, targets []string) {
	check.State, check.Reason = "unknown", "client_start_failed"
	ready := false
	defer func() {
		if !ready {
			check.Steps = append(check.Steps, ConnectionStep{"client_start", "unknown", "client_start_failed"})
		}
	}()
	if runtime.GOOS == "windows" || !isAllowedBinaryPath(binary) {
		return
	}
	dir, err := os.MkdirTemp("", "3m-node-check-")
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "api.sock")
	config := map[string]any{
		"external-controller": "", "external-controller-unix": socket,
		"mode": "rule", "log-level": "silent", "ipv6": true,
		"proxies": []map[string]any{proxy}, "rules": []string{"MATCH,node-check"},
		"dns": map[string]any{"enable": false}, "profile": map[string]any{"store-selected": false},
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return
	}
	processCtx, stop := context.WithCancel(ctx)
	defer stop()
	cmd := exec.CommandContext(processCtx, binary, "-d", dir, "-f", path)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	defer func() { stop(); <-done }()
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	readyCtx, readyCancel := context.WithTimeout(ctx, 4*time.Second)
	defer readyCancel()
	for readyCtx.Err() == nil {
		select {
		case <-done:
			return
		default:
		}
		req, _ := http.NewRequestWithContext(readyCtx, http.MethodGet, "http://local/version", nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-readyCtx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	if !ready {
		return
	}
	check.Steps = append(check.Steps, ConnectionStep{"client_start", "passed", "client_started"})
	target, delay, ok := testProxyTargets(ctx, client, targets)
	if ok {
		check.State, check.Reason, check.Target, check.DelayMS = "available", "proxy_request_passed", target, delay
		check.Steps = append(check.Steps, ConnectionStep{"proxy_request", "passed", "proxy_request_passed"})
		return
	}
	if ctx.Err() != nil {
		check.Reason = "check_interrupted"
		return
	}
	select {
	case <-done:
		check.Reason = "client_start_failed"
		return
	default:
	}
	check.State, check.Reason = "unavailable", "proxy_request_failed"
	check.Steps = append(check.Steps, ConnectionStep{"proxy_request", "failed", "proxy_request_failed"})
}

// Both targets use the same isolated proxy, with no direct fallback. Preserve
// each target's timeout but avoid paying it twice when the endpoint is blocked.
func testProxyTargets(ctx context.Context, client *http.Client, targets []string) (string, int, bool) {
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	type outcome struct {
		target string
		delay  int
		ok     bool
	}
	results := make(chan outcome, len(targets))
	for _, target := range targets {
		workers.Add(1)
		go func() {
			defer workers.Done()
			requestCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
			defer cancel()
			query := url.Values{"url": {target}, "timeout": {"5000"}, "expected": {"204"}}
			req, _ := http.NewRequestWithContext(requestCtx, http.MethodGet, "http://local/proxies/node-check/delay?"+query.Encode(), nil)
			resp, err := client.Do(req)
			var result struct {
				Delay *int `json:"delay"`
			}
			ok := false
			if err == nil {
				ok = resp.StatusCode == 200 && json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result) == nil && result.Delay != nil
				resp.Body.Close()
			}
			if ok {
				results <- outcome{target, *result.Delay, true}
			} else {
				results <- outcome{}
			}
		}()
	}
	for range targets {
		select {
		case <-ctx.Done():
			return "", 0, false
		case result := <-results:
			if result.ok && ctx.Err() == nil {
				return result.target, result.delay, true
			}
		}
	}
	return "", 0, false
}
