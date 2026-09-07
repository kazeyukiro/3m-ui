// Package realityscan finds candidate REALITY targets from this node's network.
// Passing a probe is a compatibility check, not a guarantee against fallback abuse.
package realityscan

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"sync"
	"time"
)

const (
	batchSize    = 10
	maxProbes    = 30
	concurrency  = 5
	minEligible  = 3
	probeTimeout = 5 * time.Second
	scanTimeout  = 35 * time.Second
)

var ErrBusy = errors.New("a REALITY target scan is already running on this node")

type Result struct {
	Target     string   `json:"target"`
	ServerName string   `json:"server_name"`
	LatencyMS  int      `json:"latency_ms"`
	Eligible   bool     `json:"eligible"`
	Reason     string   `json:"reason,omitempty"`
	Evidence   Evidence `json:"evidence"`
}

// Evidence records what a probe observed, without inferring that a shared
// service is safe merely because its TLS handshake succeeded.
type Evidence struct {
	CNAME              string `json:"cname,omitempty"`
	IP                 string `json:"ip,omitempty"`
	TLSVersion         string `json:"tls_version,omitempty"`
	ALPN               string `json:"alpn,omitempty"`
	KeyExchange        string `json:"key_exchange,omitempty"`
	CertificateValid   bool   `json:"certificate_valid"`
	CertificateExpires string `json:"certificate_expires,omitempty"`
	HTTPStatus         int    `json:"http_status,omitempty"`
	Server             string `json:"server,omitempty"`
	Redirect           string `json:"redirect,omitempty"`
	Cloudflare         bool   `json:"cloudflare"`
	DeliverySignal     string `json:"delivery_signal,omitempty"`
}

type Response struct {
	Selected   *Result   `json:"selected"`
	Candidates []Result  `json:"candidates"`
	Scanned    int       `json:"scanned"`
	CheckedAt  time.Time `json:"checked_at"`
}

type Scanner struct {
	mu         sync.Mutex
	candidates []string
	probe      func(context.Context, string) Result
}

func New() *Scanner {
	catalog, err := loadCatalog(candidateData)
	if err != nil {
		panic("invalid embedded REALITY candidate catalog: " + err.Error())
	}
	return &Scanner{candidates: automaticHosts(catalog), probe: probeTarget}
}

// Scan samples without replacement. Only one scan per node runs at a time, so
// repeated requests cannot multiply the outbound connection concurrency.
func (s *Scanner) Scan(ctx context.Context) (*Response, error) {
	if !s.mu.TryLock() {
		return nil, ErrBusy
	}
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()
	pool := slices.Clone(s.candidates)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	pool = pool[:min(len(pool), maxProbes)]
	out := &Response{Candidates: []Result{}}
	for start := 0; start < len(pool); start += batchSize {
		batch := pool[start:min(start+batchSize, len(pool))]
		results := make([]Result, len(batch))
		var wg sync.WaitGroup
		sem := make(chan struct{}, concurrency)
		for i, host := range batch {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				wg.Wait()
				return nil, ctx.Err()
			}
			wg.Add(1)
			go func(i int, host string) {
				defer wg.Done()
				defer func() { <-sem }()
				probeCtx, stop := context.WithTimeout(ctx, probeTimeout)
				defer stop()
				results[i] = s.probe(probeCtx, host)
			}(i, host)
		}
		wg.Wait()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out.Scanned += len(batch)
		out.Candidates = append(out.Candidates, results...)
		if len(eligibleResults(out.Candidates)) >= minEligible {
			break
		}
	}
	eligible := eligibleResults(out.Candidates)
	// Shuffle the acceptable subset rather than always choosing the fastest site.
	rand.Shuffle(len(eligible), func(i, j int) { eligible[i], eligible[j] = eligible[j], eligible[i] })
	for i := range out.Candidates {
		if out.Candidates[i].Eligible && !slices.ContainsFunc(eligible, func(r Result) bool { return r.Target == out.Candidates[i].Target }) {
			out.Candidates[i].Eligible = false
			out.Candidates[i].Reason = "latency_outside_window"
		}
	}
	if len(eligible) > 0 {
		out.Selected = &eligible[0]
	}
	out.CheckedAt = time.Now().UTC()
	return out, nil
}

// Accept up to 3x the fastest handshake, with a 300ms floor and 1500ms ceiling.
func eligibleResults(results []Result) []Result {
	fastest := 1501
	for _, r := range results {
		if r.Eligible {
			fastest = min(fastest, r.LatencyMS)
		}
	}
	limit := min(1500, max(300, fastest*3))
	out := []Result{}
	for _, r := range results {
		if r.Eligible && r.LatencyMS <= limit {
			out = append(out, r)
		}
	}
	return out
}
