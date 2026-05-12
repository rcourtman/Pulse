package recovery

import (
	"sort"
	"strings"
	"time"
)

// BackupVerifyStaleWindow is the default lookback used to decide whether a
// successful backup has been verified recently enough. A successful backup
// without a verification-bearing point inside this window flips the rollup's
// VerifyIntent to stale. MVP keeps this as a package-level constant; a
// future configurable plumbing change will replace it.
const BackupVerifyStaleWindow = 7 * 24 * time.Hour

// ComputeVerifyIntentAt returns the verify intent and the newest verified
// timestamp for a subject, given the most recent successful attempt and the
// newest verification-bearing point seen on the rollup. The "now" parameter
// is injected so tests and the SQL rollup path stay deterministic.
//
// Semantics:
//   - verified : a verification-bearing point exists within window of now
//   - stale    : a successful backup exists, but no verification within window
//   - unknown  : no successful backup at all (or no info)
//
// LastVerifiedAt is returned regardless of intent (when known) so consumers
// can surface "last verified X days ago" alongside the badge.
func ComputeVerifyIntentAt(lastSuccessMs, lastVerifiedMs int64, now time.Time) (VerifyIntent, *time.Time) {
	windowMs := BackupVerifyStaleWindow.Milliseconds()
	nowMs := now.UTC().UnixMilli()

	var lastVerifiedAt *time.Time
	if lastVerifiedMs > 0 {
		t := time.UnixMilli(lastVerifiedMs).UTC()
		lastVerifiedAt = &t
	}

	if lastSuccessMs <= 0 {
		return VerifyIntentUnknown, lastVerifiedAt
	}

	if lastVerifiedMs > 0 && nowMs-lastVerifiedMs <= windowMs {
		return VerifyIntentVerified, lastVerifiedAt
	}

	return VerifyIntentStale, lastVerifiedAt
}

func displayForRollupPoint(p RecoveryPoint) *RecoveryPointDisplay {
	if p.Display != nil {
		display := *p.Display
		return &display
	}
	return DeriveIndex(p).ToDisplay()
}

// BuildRollupsFromPoints computes per-subject rollups from a set of recovery points.
// This mirrors the sqlite rollup semantics (timestamp selection + success window)
// so mock mode and in-memory consumers behave consistently with persisted stores.
func BuildRollupsFromPoints(points []RecoveryPoint) []ProtectionRollup {
	return BuildRollupsFromPointsAt(points, time.Now())
}

// BuildRollupsFromPointsAt is the now-injectable companion used by tests so
// VerifyIntent staleness is deterministic regardless of wall-clock drift.
func BuildRollupsFromPointsAt(points []RecoveryPoint, now time.Time) []ProtectionRollup {
	if len(points) == 0 {
		return []ProtectionRollup{}
	}

	type agg struct {
		subjectKey string

		latestTS      int64
		latestUpdated int64
		latestID      string
		latestOutcome Outcome

		lastAttemptMs  int64
		lastSuccessMs  int64
		lastVerifiedMs int64

		// Latest identity seen (ties resolved by latestTS/updated/id).
		subjectRID string
		subjectRef *ExternalRef
		display    *RecoveryPointDisplay

		providers map[Provider]struct{}
	}

	rollupTS := func(p RecoveryPoint) *time.Time {
		if p.CompletedAt != nil && !p.CompletedAt.IsZero() {
			t := p.CompletedAt.UTC()
			return &t
		}
		if p.StartedAt != nil && !p.StartedAt.IsZero() {
			t := p.StartedAt.UTC()
			return &t
		}
		return nil
	}

	// Best-effort stable tie-breaker to match store ordering.
	updatedMs := func(p RecoveryPoint) int64 {
		if v, ok := p.Details["updatedAtMs"].(int64); ok {
			return v
		}
		if v, ok := p.Details["updated_at_ms"].(int64); ok {
			return v
		}
		if v, ok := p.Details["updatedAtMs"].(float64); ok {
			return int64(v)
		}
		if v, ok := p.Details["updated_at_ms"].(float64); ok {
			return int64(v)
		}
		return 0
	}

	byKey := make(map[string]*agg, 64)
	for _, p := range points {
		subjectKey := strings.TrimSpace(SubjectKeyForPoint(p))
		if subjectKey == "" {
			continue
		}

		ts := rollupTS(p)
		if ts == nil || ts.IsZero() {
			continue
		}
		tsMs := ts.UnixMilli()

		a := byKey[subjectKey]
		if a == nil {
			a = &agg{
				subjectKey: subjectKey,
				latestTS:   tsMs,
				latestID:   strings.TrimSpace(p.ID),
				latestOutcome: func() Outcome {
					if strings.TrimSpace(string(p.Outcome)) == "" {
						return OutcomeUnknown
					}
					return p.Outcome
				}(),
				lastAttemptMs: tsMs,
				lastSuccessMs: 0,
				subjectRID:    strings.TrimSpace(p.SubjectResourceID),
				subjectRef:    p.SubjectRef,
				display:       displayForRollupPoint(p),
				providers:     make(map[Provider]struct{}, 2),
			}
			byKey[subjectKey] = a
		}

		a.providers[p.Provider] = struct{}{}

		if tsMs > a.lastAttemptMs {
			a.lastAttemptMs = tsMs
		}
		if p.Outcome == OutcomeSuccess && tsMs > a.lastSuccessMs {
			a.lastSuccessMs = tsMs
		}
		// Track the newest verification-bearing point on the rollup window so
		// the VerifyIntent computation downstream can grade the subject.
		if p.Verified != nil && *p.Verified && tsMs > a.lastVerifiedMs {
			a.lastVerifiedMs = tsMs
		}

		// Latest-point identity + outcome: choose the point with the greatest ts,
		// then updated, then id lexicographically.
		u := updatedMs(p)
		id := strings.TrimSpace(p.ID)
		if tsMs > a.latestTS || (tsMs == a.latestTS && (u > a.latestUpdated || (u == a.latestUpdated && id > a.latestID))) {
			a.latestTS = tsMs
			a.latestUpdated = u
			a.latestID = id
			if strings.TrimSpace(string(p.Outcome)) == "" {
				a.latestOutcome = OutcomeUnknown
			} else {
				a.latestOutcome = p.Outcome
			}
			a.subjectRID = strings.TrimSpace(p.SubjectResourceID)
			a.subjectRef = p.SubjectRef
			a.display = displayForRollupPoint(p)
		}
	}

	out := make([]ProtectionRollup, 0, len(byKey))
	for _, a := range byKey {
		var lastAttemptAt *time.Time
		if a.lastAttemptMs > 0 {
			t := time.UnixMilli(a.lastAttemptMs).UTC()
			lastAttemptAt = &t
		}
		var lastSuccessAt *time.Time
		if a.lastSuccessMs > 0 {
			t := time.UnixMilli(a.lastSuccessMs).UTC()
			lastSuccessAt = &t
		}

		providers := make([]Provider, 0, len(a.providers))
		for p := range a.providers {
			if strings.TrimSpace(string(p)) == "" {
				continue
			}
			providers = append(providers, p)
		}
		sort.Slice(providers, func(i, j int) bool { return string(providers[i]) < string(providers[j]) })

		verifyIntent, lastVerifiedAt := ComputeVerifyIntentAt(a.lastSuccessMs, a.lastVerifiedMs, now)

		out = append(out, ProtectionRollup{
			RollupID:          strings.TrimSpace(a.subjectKey),
			SubjectResourceID: a.subjectRID,
			SubjectRef:        a.subjectRef,
			Display:           a.display,
			LastAttemptAt:     lastAttemptAt,
			LastSuccessAt:     lastSuccessAt,
			LastOutcome:       a.latestOutcome,
			Providers:         providers,
			VerifyIntent:      verifyIntent,
			LastVerifiedAt:    lastVerifiedAt,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		a := out[i]
		b := out[j]

		ai := int64(0)
		if a.LastAttemptAt != nil {
			ai = a.LastAttemptAt.UTC().UnixMilli()
		}
		bi := int64(0)
		if b.LastAttemptAt != nil {
			bi = b.LastAttemptAt.UTC().UnixMilli()
		}
		if ai == 0 && bi != 0 {
			return false
		}
		if ai != 0 && bi == 0 {
			return true
		}
		if ai != bi {
			return ai > bi
		}
		return strings.TrimSpace(a.RollupID) < strings.TrimSpace(b.RollupID)
	})

	return out
}
