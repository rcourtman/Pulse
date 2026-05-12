package maintenancesentinel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// Providers bundles the slim, package-defined interfaces the sentinel
// needs from the rest of the system. The caller (router wiring) is
// responsible for adapting alerts/findings/actions/metrics into these
// shapes. Keeps this package free of `internal/alerts`, `internal/ai`,
// and `internal/monitoring` imports so there are no cycles.
type Providers struct {
	// Stores resolves the ResourceStore for the given org id. The
	// sentinel runs against the default org for MVP; tenant-scoped
	// scheduling is left to a future change.
	Stores func(orgID string) (unified.ResourceStore, error)
	// ActiveAlerts returns active alerts for a resource at this moment.
	ActiveAlerts func(orgID, canonicalID string) []AlertSummary
	// ActiveFindings returns active Patrol findings for a resource.
	ActiveFindings func(orgID, canonicalID string) []FindingSummary
	// RecentActions returns action audit records for the resource
	// since the supplied lower bound. The sentinel itself filters for
	// failed state after the call returns.
	RecentActions func(orgID, canonicalID string, since time.Time) []ActionSummary
	// PostWindowMetricSamples returns metric samples observed after
	// the maintenance window closed for the resource. The boolean
	// reports whether a metric source was available for the
	// resource — distinct from "available but empty".
	PostWindowMetricSamples func(orgID, canonicalID string, windowEnd, now time.Time) ([]MetricSample, bool)
	// Now is the clock; tests inject a fixed clock. Defaults to
	// time.Now.
	Now func() time.Time
}

// Sentinel watches operator state for maintenance-window-end events
// and writes Maintenance Verification Reports.
type Sentinel struct {
	orgID         string
	providers     Providers
	tick          time.Duration
	lookbackLimit time.Duration

	mu sync.Mutex
}

// Config configures a Sentinel instance.
type Config struct {
	// OrgID is the tenant the sentinel scans. MVP runs the default
	// org only — multi-tenant scheduling is deferred.
	OrgID string
	// Tick controls the sweep cadence. Defaults to one minute.
	Tick time.Duration
	// LookbackLimit prevents the sentinel from generating reports
	// for ancient windows on first start (e.g. after a long
	// downtime). Window-end events older than `now - LookbackLimit`
	// are ignored. Default: 7 days.
	LookbackLimit time.Duration
}

// New returns a Sentinel ready to start. Providers is required.
func New(cfg Config, providers Providers) (*Sentinel, error) {
	if providers.Stores == nil {
		return nil, errors.New("maintenancesentinel: Providers.Stores is required")
	}
	tick := cfg.Tick
	if tick <= 0 {
		tick = time.Minute
	}
	lookback := cfg.LookbackLimit
	if lookback <= 0 {
		lookback = 7 * 24 * time.Hour
	}
	org := cfg.OrgID
	if org == "" {
		org = "default"
	}
	return &Sentinel{
		orgID:         org,
		providers:     providers,
		tick:          tick,
		lookbackLimit: lookback,
	}, nil
}

// Start runs the sentinel loop in a goroutine and returns. The loop
// stops when ctx is canceled. Safe to call once per Sentinel.
func (s *Sentinel) Start(ctx context.Context) {
	go s.run(ctx)
}

func (s *Sentinel) run(ctx context.Context) {
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()
	// Run one sweep immediately so a newly-ended window doesn't have
	// to wait a full tick for a report.
	s.tickOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("maintenance-verification sentinel stopped")
			return
		case <-ticker.C:
			s.tickOnce(ctx)
		}
	}
}

func (s *Sentinel) tickOnce(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, err := s.providers.Stores(s.orgID)
	if err != nil {
		log.Debug().Err(err).Msg("maintenance-verification sentinel: get store")
		return
	}
	states, err := store.ListResourceOperatorStates()
	if err != nil {
		log.Debug().Err(err).Msg("maintenance-verification sentinel: list operator states")
		return
	}

	now := s.now()
	cutoff := now.Add(-s.lookbackLimit)

	for _, state := range states {
		if ctx.Err() != nil {
			return
		}
		if state.MaintenanceStartAt == nil || state.MaintenanceEndAt == nil {
			continue
		}
		if !state.MaintenanceEndAt.Before(now) && !state.MaintenanceEndAt.Equal(now) {
			// Window hasn't ended yet.
			continue
		}
		if state.MaintenanceEndAt.Before(cutoff) {
			// Window ended too long ago — don't backfill ancient events.
			continue
		}
		s.evaluateForState(state, store, now)
	}
}

func (s *Sentinel) evaluateForState(state unified.ResourceOperatorState, store unified.ResourceStore, now time.Time) {
	canonicalID := unified.CanonicalResourceID(state.CanonicalID)
	if canonicalID == "" || state.MaintenanceEndAt == nil {
		return
	}
	if _, exists, err := store.FindLoopReportByWindow(unified.LoopReportTypeMaintenanceVerification, canonicalID, *state.MaintenanceEndAt); err != nil {
		log.Debug().Err(err).Str("resource", canonicalID).Msg("maintenance-verification sentinel: dedupe lookup")
		return
	} else if exists {
		// Already wrote a report for this (resource, window-end).
		return
	}

	inputs := s.buildInputs(state, now)
	report := EvaluateVerification(inputs)
	if err := store.RecordLoopReport(report); err != nil {
		// A unique-constraint conflict (race against a parallel
		// tick) is benign — another writer wrote the same window.
		log.Debug().Err(err).Str("resource", canonicalID).Msg("maintenance-verification sentinel: record report")
		return
	}
	log.Info().
		Str("resource", canonicalID).
		Str("status", string(report.Status)).
		Str("report_id", report.ID).
		Msg("maintenance verification report written")
}

// buildInputs gathers the deterministic input bundle for the
// evaluator. Provider closures may be nil — the inputs simply carry
// zero values in that case.
func (s *Sentinel) buildInputs(state unified.ResourceOperatorState, now time.Time) VerificationInputs {
	canonicalID := unified.CanonicalResourceID(state.CanonicalID)
	inputs := VerificationInputs{
		ResourceID:    canonicalID,
		OperatorState: cloneOperatorState(state),
		Now:           now,
	}
	if state.MaintenanceStartAt != nil {
		inputs.WindowStartedAt = state.MaintenanceStartAt.UTC()
	}
	if state.MaintenanceEndAt != nil {
		inputs.WindowEndedAt = state.MaintenanceEndAt.UTC()
	}
	if s.providers.ActiveAlerts != nil {
		inputs.ActiveAlerts = s.providers.ActiveAlerts(s.orgID, canonicalID)
	}
	if s.providers.ActiveFindings != nil {
		inputs.ActiveFindings = s.providers.ActiveFindings(s.orgID, canonicalID)
	}
	if s.providers.RecentActions != nil {
		windowStart := inputs.WindowStartedAt
		if windowStart.IsZero() {
			windowStart = now.Add(-24 * time.Hour)
		}
		inputs.RecentActions = s.providers.RecentActions(s.orgID, canonicalID, windowStart)
	}
	if s.providers.PostWindowMetricSamples != nil {
		samples, available := s.providers.PostWindowMetricSamples(s.orgID, canonicalID, inputs.WindowEndedAt, now)
		inputs.PostWindowMetricSamples = samples
		inputs.MetricSourceAvailable = available
	}
	return inputs
}

func cloneOperatorState(s unified.ResourceOperatorState) *unified.ResourceOperatorState {
	clone := s
	if s.MaintenanceStartAt != nil {
		t := *s.MaintenanceStartAt
		clone.MaintenanceStartAt = &t
	}
	if s.MaintenanceEndAt != nil {
		t := *s.MaintenanceEndAt
		clone.MaintenanceEndAt = &t
	}
	return &clone
}

func (s *Sentinel) now() time.Time {
	if s.providers.Now != nil {
		return s.providers.Now().UTC()
	}
	return time.Now().UTC()
}

// EvaluateOnce runs the deterministic verification for the supplied
// resource exactly once, immediately, writing a fresh report. Used by
// the "rerun verification" handler so an operator can re-evaluate
// without waiting for the next tick.
//
// If the existing report for the (resource, window-end) triple is
// found, this writes a *new* report with a -rerun-N suffix so the
// review history is preserved.
func (s *Sentinel) EvaluateOnce(ctx context.Context, canonicalID string) (unified.LoopReport, error) {
	if ctx.Err() != nil {
		return unified.LoopReport{}, ctx.Err()
	}
	canonicalID = unified.CanonicalResourceID(canonicalID)
	if canonicalID == "" {
		return unified.LoopReport{}, fmt.Errorf("maintenancesentinel: canonical id is required")
	}
	store, err := s.providers.Stores(s.orgID)
	if err != nil {
		return unified.LoopReport{}, err
	}
	state, found, err := store.GetResourceOperatorState(canonicalID)
	if err != nil {
		return unified.LoopReport{}, err
	}
	if !found {
		return unified.LoopReport{}, fmt.Errorf("maintenancesentinel: no operator state for %q", canonicalID)
	}
	if state.MaintenanceStartAt == nil || state.MaintenanceEndAt == nil {
		return unified.LoopReport{}, fmt.Errorf("maintenancesentinel: resource %q has no maintenance window to verify", canonicalID)
	}
	now := s.now()
	inputs := s.buildInputs(state, now)
	report := EvaluateVerification(inputs)
	report.ID = uniqueRerunID(store, report.ID)
	if err := store.RecordLoopReport(report); err != nil {
		return unified.LoopReport{}, err
	}
	return report, nil
}

func uniqueRerunID(store unified.ResourceStore, base string) string {
	if _, exists, err := store.GetLoopReport(base); err == nil && !exists {
		return base
	}
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-rerun-%d", base, i)
		if _, exists, err := store.GetLoopReport(candidate); err == nil && !exists {
			return candidate
		}
	}
	return fmt.Sprintf("%s-rerun-%d", base, time.Now().UnixNano())
}
