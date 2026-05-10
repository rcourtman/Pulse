package ai

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// reportFindingsLimit caps how many findings are passed to the report
// narrator. Reports are retrospective summaries, so a long tail of findings
// adds prompt cost without changing the conclusion the narrator should
// reach. Sorted by severity then DetectedAt before truncation.
const reportFindingsLimit = 25

// Compile-time assertion the Service satisfies reporting.FindingsProvider.
var _ reporting.FindingsProvider = (*Service)(nil)

// FindingsForReport implements reporting.FindingsProvider. It returns
// Patrol findings whose resource matches resourceID and whose lifecycle
// overlaps the [start, end) window: any finding either detected inside
// the window or detected before but still active during it. Returning an
// empty slice when patrol is unavailable or no findings exist is intentional
// — the engine treats absence as "no patrol activity" rather than an error.
func (s *Service) FindingsForReport(ctx context.Context, resourceID string, start, end time.Time) []reporting.FindingSummary {
	if ctx != nil && ctx.Err() != nil {
		return nil
	}
	patrol := s.GetPatrolService()
	if patrol == nil {
		return nil
	}

	candidates := patrol.GetFindingsForResource(resourceID)
	if len(candidates) == 0 {
		return nil
	}

	out := make([]reporting.FindingSummary, 0, len(candidates))
	for _, f := range candidates {
		if f == nil {
			continue
		}
		if !findingOverlapsWindow(f, start, end) {
			continue
		}
		out = append(out, reporting.FindingSummary{
			ID:             f.ID,
			Severity:       string(f.Severity),
			Category:       string(f.Category),
			Title:          f.Title,
			Description:    f.Description,
			Recommendation: f.Recommendation,
			DetectedAt:     f.DetectedAt,
			Resolved:       f.IsResolved(),
		})
	}

	if len(out) > reportFindingsLimit {
		out = out[:reportFindingsLimit]
	}
	return out
}

// findingOverlapsWindow reports whether a finding's lifecycle intersects
// the [start, end) window used by a report. A finding overlaps when it was
// detected before end and (if resolved) resolved at or after start.
func findingOverlapsWindow(f *Finding, start, end time.Time) bool {
	if f == nil {
		return false
	}
	if !end.IsZero() && !f.DetectedAt.IsZero() && f.DetectedAt.After(end) {
		return false
	}
	if f.IsResolved() && f.ResolvedAt != nil && !start.IsZero() {
		if f.ResolvedAt.Before(start) {
			return false
		}
	}
	return true
}
