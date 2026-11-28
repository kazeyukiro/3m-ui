package realityscan

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func reviewedCandidate(t *testing.T) Candidate {
	t.Helper()
	catalog, err := loadCatalog(candidateData)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range catalog {
		if candidate.AutoSelect {
			return candidate
		}
	}
	t.Fatal("no reviewed candidate fixture")
	return Candidate{}
}

func TestCatalogRejectsIncompleteOrUnsafeApproval(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Candidate)
	}{
		{"missing source", func(c *Candidate) { c.Sources = nil }},
		{"invalid source", func(c *Candidate) { c.Sources = []string{"http://example.com"} }},
		{"missing rationale", func(c *Candidate) { c.Rationale = "" }},
		{"missing review date", func(c *Candidate) { c.Review.CheckedAt = time.Time{} }},
		{"missing vantage", func(c *Candidate) { c.Review.VantagePoint = "" }},
		{"missing risk record", func(c *Candidate) { c.Review.KnownRisks = nil }},
		{"wrong hostname", func(c *Candidate) { c.Hostname = "127.0.0.1" }},
		{"URL instead of hostname", func(c *Candidate) { c.Hostname = "https://example.com" }},
		{"wrong evidence target", func(c *Candidate) { c.Review.Probe.Target = "other.example.com:443" }},
		{"failed probe", func(c *Candidate) { c.Review.Probe.Eligible = false }},
		{"untrusted certificate", func(c *Candidate) { c.Review.Probe.Evidence.CertificateValid = false }},
		{"no DNS observation", func(c *Candidate) { c.Review.Probe.Evidence.CNAME = "" }},
		{"shared delivery alias", func(c *Candidate) { c.Review.Probe.Evidence.CNAME = "edge.fastly.net." }},
		{"shared delivery headers", func(c *Candidate) { c.Review.Probe.Evidence.DeliverySignal = "cloudfront" }},
		{"bulk download service", func(c *Candidate) { c.ServiceKind = "download" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := reviewedCandidate(t)
			tc.change(&candidate)
			data, err := json.Marshal([]Candidate{candidate})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := loadCatalog(data); err == nil {
				t.Fatal("accepted invalid approved candidate")
			}
		})
	}
	if _, err := loadCatalog([]byte(`[{"autoSelect":true}]`)); err == nil {
		t.Fatal("accepted misspelled approval field")
	}
	candidate := reviewedCandidate(t)
	data, _ := json.Marshal([]Candidate{candidate, candidate})
	if _, err := loadCatalog(data); err == nil {
		t.Fatal("accepted duplicate hostname")
	}
}

func TestDisabledCandidateNeverProbedEvenWithPassingHistory(t *testing.T) {
	allowed := reviewedCandidate(t)
	disabled := allowed
	disabled.Hostname = "disabled.example.com"
	disabled.Review.Probe.Target = disabled.Hostname + ":443"
	disabled.Review.Probe.ServerName = disabled.Hostname
	disabled.AutoSelect = false
	data, _ := json.Marshal([]Candidate{allowed, disabled})
	catalog, err := loadCatalog(data)
	if err != nil {
		t.Fatal(err)
	}
	s := &Scanner{candidates: automaticHosts(catalog), probe: func(_ context.Context, host string) Result {
		if host != allowed.Hostname {
			t.Errorf("probed disabled target %s", host)
		}
		return Result{Target: host + ":443", Eligible: true, LatencyMS: 10}
	}}
	r, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Scanned != 1 || r.Selected == nil || r.Selected.Target != allowed.Hostname+":443" {
		t.Fatalf("unexpected selection: %+v", r)
	}
}

func TestAuditRequiresVantageAndHonorsCancellation(t *testing.T) {
	if _, err := Audit(context.Background(), ""); err == nil {
		t.Fatal("accepted audit without vantage")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Audit(ctx, "test, no network"); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
