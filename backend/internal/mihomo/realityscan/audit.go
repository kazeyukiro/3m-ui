package realityscan

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AuditObservation struct {
	CheckedAt time.Time `json:"checked_at"`
	Result    Result    `json:"result"`
}

type AuditReport struct {
	VantagePoint string             `json:"vantage_point"`
	Observations []AuditObservation `json:"observations"`
}

// Audit reviews every catalog entry, including disabled entries. It never
// promotes a candidate or writes the catalog. This maintenance operation is
// only exposed by the explicit CLI, not by the automatic scan API.
func Audit(ctx context.Context, vantagePoint string) (*AuditReport, error) {
	if strings.TrimSpace(vantagePoint) == "" {
		return nil, fmt.Errorf("describe the probe's vantage point")
	}
	catalog, err := loadCatalog(candidateData)
	if err != nil {
		return nil, err
	}
	report := &AuditReport{VantagePoint: vantagePoint, Observations: []AuditObservation{}}
	for _, candidate := range catalog {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		r := probeTarget(probeCtx, candidate.Hostname)
		cancel()
		report.Observations = append(report.Observations, AuditObservation{CheckedAt: time.Now().UTC(), Result: r})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return report, nil
}
