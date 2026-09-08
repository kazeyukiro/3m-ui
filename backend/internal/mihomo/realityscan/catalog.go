package realityscan

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

//go:embed candidates.json
var candidateData []byte

type Candidate struct {
	Hostname    string          `json:"hostname"`
	Sources     []string        `json:"sources"`
	Rationale   string          `json:"rationale"`
	ServiceKind string          `json:"service_kind"`
	AutoSelect  bool            `json:"auto_select"`
	Review      CandidateReview `json:"review"`
}

type CandidateReview struct {
	CheckedAt          time.Time `json:"checked_at"`
	VantagePoint       string    `json:"vantage_point"`
	HostingObservation string    `json:"hosting_observation"`
	KnownRisks         []string  `json:"known_risks"`
	Decision           string    `json:"decision"`
	Probe              Result    `json:"probe"`
}

// The embedded catalog is reviewed configuration, not untrusted request input.
// Fail clearly on malformed records instead of silently losing metadata/gates.
func loadCatalog(data []byte) ([]Candidate, error) {
	var catalog []Candidate
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("trailing catalog data")
	}
	seen := map[string]bool{}
	for _, candidate := range catalog {
		if !validHostname(candidate.Hostname) || seen[candidate.Hostname] {
			return nil, fmt.Errorf("invalid or duplicate hostname %q", candidate.Hostname)
		}
		seen[candidate.Hostname] = true
		if len(candidate.Sources) == 0 || strings.TrimSpace(candidate.Rationale) == "" {
			return nil, fmt.Errorf("%s has no provenance", candidate.Hostname)
		}
		for _, source := range candidate.Sources {
			u, err := url.Parse(source)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
				return nil, fmt.Errorf("invalid source for %s", candidate.Hostname)
			}
		}
		switch candidate.ServiceKind {
		case "website", "download", "shared_endpoint":
		default:
			return nil, fmt.Errorf("unknown service kind for %s", candidate.Hostname)
		}
		r := candidate.Review
		if r.CheckedAt.IsZero() || strings.TrimSpace(r.VantagePoint) == "" || strings.TrimSpace(r.HostingObservation) == "" || strings.TrimSpace(r.Decision) == "" || r.KnownRisks == nil {
			return nil, fmt.Errorf("incomplete review for %s", candidate.Hostname)
		}
		if r.Probe.Target != net.JoinHostPort(candidate.Hostname, "443") || r.Probe.ServerName != candidate.Hostname {
			return nil, fmt.Errorf("review target mismatch for %s", candidate.Hostname)
		}
		if candidate.AutoSelect {
			e := r.Probe.Evidence
			if candidate.ServiceKind != "website" || !r.Probe.Eligible || r.Probe.Reason != "" || !e.CertificateValid || e.TLSVersion != "TLS 1.3" || e.ALPN != "h2" || (e.KeyExchange != "X25519" && e.KeyExchange != "X25519MLKEM768") || e.HTTPStatus < 200 || e.HTTPStatus >= 300 || e.Cloudflare || e.DeliverySignal != "" || e.CNAME == "" || sharedDeliveryCNAME(e.CNAME) {
				return nil, fmt.Errorf("%s is enabled without a qualifying review", candidate.Hostname)
			}
		}
	}
	return catalog, nil
}

func automaticHosts(catalog []Candidate) []string {
	var hosts []string
	for _, candidate := range catalog {
		if candidate.AutoSelect {
			hosts = append(hosts, candidate.Hostname)
		}
	}
	return hosts
}

func validHostname(host string) bool {
	if len(host) > 253 || net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	return true
}
