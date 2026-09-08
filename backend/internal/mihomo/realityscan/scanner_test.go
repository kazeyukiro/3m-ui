package realityscan

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCandidatePool(t *testing.T) {
	pool := New().candidates
	if len(pool) == 0 {
		t.Fatal("the shipped pool has no reviewed automatic candidates")
	}
	catalog, err := loadCatalog(candidateData)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) <= len(pool) {
		t.Fatal("expected quarantined review records outside the automatic pool")
	}
	seen := map[string]bool{}
	for _, host := range pool {
		if seen[host] || strings.ContainsAny(host, "/:@ \t") || !strings.Contains(host, ".") {
			t.Fatalf("invalid/duplicate hostname %q", host)
		}
		seen[host] = true
	}
}

// Algorithm tests use synthetic hosts so changing the reviewed pool does not
// require weakening its selection policy or retaining unused real domains.
func testScanner() *Scanner {
	hosts := make([]string, 40)
	for i := range hosts {
		hosts[i] = fmt.Sprintf("candidate-%d.example.com", i)
	}
	return &Scanner{candidates: hosts}
}

func TestSamplingStopsAfterEnoughEligibleTargets(t *testing.T) {
	s := testScanner()
	var calls atomic.Int32
	s.probe = func(_ context.Context, host string) Result {
		n := calls.Add(1)
		return Result{Target: host + ":443", ServerName: host, Eligible: n > 10, LatencyMS: 80}
	}
	response, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if response.Scanned != 20 || calls.Load() != 20 || response.Selected == nil {
		t.Fatalf("unexpected scan: %+v", response)
	}
	if !response.Selected.Eligible {
		t.Fatal("selected a rejected target")
	}
	if !slices.Contains(response.Candidates, *response.Selected) {
		t.Fatal("selection not in probed candidates")
	}
}

func TestFailedScanIsBoundedAndHasNoFallback(t *testing.T) {
	s := testScanner()
	var mu sync.Mutex
	seen := map[string]bool{}
	s.probe = func(_ context.Context, host string) Result {
		mu.Lock()
		defer mu.Unlock()
		if seen[host] {
			t.Errorf("duplicate probe: %s", host)
		}
		seen[host] = true
		return Result{Target: host + ":443", Reason: "tls13_required"}
	}
	r, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Scanned != 30 || len(seen) != 30 || r.Selected != nil {
		t.Fatalf("unexpected scan: %+v", r)
	}
}

func TestConcurrencyCancellationAndBusy(t *testing.T) {
	s := testScanner()
	var active atomic.Int32
	started := make(chan struct{}, concurrency)
	s.probe = func(ctx context.Context, host string) Result {
		if active.Add(1) > concurrency {
			t.Error("too many concurrent probes")
		}
		defer active.Add(-1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return Result{Target: host}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := s.Scan(ctx); done <- err }()
	for range concurrency {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	if _, err := s.Scan(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected busy, got %v", err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop scan")
	}
	if active.Load() != 0 {
		t.Fatal("probe leaked after cancellation")
	}
	s.probe = func(_ context.Context, host string) Result {
		return Result{Target: host, Eligible: true, LatencyMS: 10}
	}
	if _, err := s.Scan(context.Background()); err != nil {
		t.Fatalf("lock not released: %v", err)
	}
}

func TestLatencyWindow(t *testing.T) {
	results := []Result{{Target: "fast", Eligible: true, LatencyMS: 80}, {Target: "acceptable", Eligible: true, LatencyMS: 250}, {Target: "slow", Eligible: true, LatencyMS: 600}, {Target: "failed", LatencyMS: 1}}
	got := eligibleResults(results)
	if len(got) != 2 || got[1].Target != "acceptable" {
		t.Fatalf("unexpected eligible set: %+v", got)
	}
	if got := eligibleResults([]Result{{Eligible: true, LatencyMS: 1501}}); len(got) != 0 {
		t.Fatal("accepted target beyond latency ceiling")
	}
}

func TestAddressFiltering(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1", "169.254.169.254", "100.64.0.1", "0.1.2.3", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "ff02::1", "192.0.2.1", "64:ff9b::7f00:1"} {
		t.Run(ip, func(t *testing.T) {
			if publicIP(netip.MustParseAddr(ip)) {
				t.Fatal("accepted non-public address")
			}
		})
	}
	for _, ip := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !publicIP(netip.MustParseAddr(ip)) {
			t.Fatalf("rejected public address %s", ip)
		}
	}
}

func TestSmallAndEmptyPools(t *testing.T) {
	for _, size := range []int{0, 1, 3} {
		s := testScanner()
		s.candidates = s.candidates[:size]
		s.probe = func(_ context.Context, host string) Result {
			return Result{Target: host, Eligible: true, LatencyMS: 80}
		}
		r, err := s.Scan(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if r.Scanned != size || len(r.Candidates) != size || (r.Selected != nil) != (size > 0) {
			t.Fatalf("size %d: %+v", size, r)
		}
	}
}
