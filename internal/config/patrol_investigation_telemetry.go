package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

const patrolInvestigationTelemetryFile = "patrol_investigation_telemetry.json"
const patrolInvestigationTelemetryLimit = 100000

// Only these fields reach the independent local ledger. Keys are SHA-256 of
// tenant + run identity, used locally for replay and lifecycle reconciliation.
// Neither keys nor timestamps are exported. There is no customer text here.
type PatrolInvestigationMeasurement struct {
	CompletedAt time.Time                              `json:"completed_at"`
	Outcome     string                                 `json:"outcome"`
	Handoff     aicontracts.InvestigationHandoffReason `json:"handoff,omitempty"`
}

type patrolInvestigationLedger struct {
	Version       int                                       `json:"version"`
	ObservedSince time.Time                                 `json:"observed_since"`
	Runs          map[string]PatrolInvestigationMeasurement `json:"runs"`
}

type PatrolInvestigationTelemetrySummary struct {
	Available       bool
	ObservationDays int
	Runs            int
	Outcomes        map[string]int
	Handoffs        map[aicontracts.InvestigationHandoffReason]int
}

// All ConfigPersistence instances for this installation share serialization
// and failed writes. A failed observation stays pending and makes coverage
// unavailable until all pending observations have been durably written.
type patrolInvestigationLedgerState struct {
	sync.Mutex
	pending     map[string]PatrolInvestigationMeasurement
	writeFailed bool
}

var patrolInvestigationLedgerStates sync.Map

func (c *ConfigPersistence) patrolInvestigationLedgerPath() string {
	return filepath.Join(c.SharedInstallationDataDir(), patrolInvestigationTelemetryFile)
}

func (c *ConfigPersistence) patrolInvestigationLedgerState() *patrolInvestigationLedgerState {
	state, _ := patrolInvestigationLedgerStates.LoadOrStore(c.patrolInvestigationLedgerPath(), &patrolInvestigationLedgerState{pending: map[string]PatrolInvestigationMeasurement{}})
	return state.(*patrolInvestigationLedgerState)
}

func canonicalInvestigationMeasurementOutcome(outcome aicontracts.InvestigationOutcome, status aicontracts.InvestigationStatus) string {
	switch outcome {
	case aicontracts.OutcomeResolved, aicontracts.OutcomeFixQueued, aicontracts.OutcomeFixExecuted,
		aicontracts.OutcomeFixFailed, aicontracts.OutcomeFixRejected, aicontracts.OutcomeFixVerified,
		aicontracts.OutcomeFixVerificationFailed, aicontracts.OutcomeFixVerificationUnknown,
		aicontracts.OutcomeNeedsAttention, aicontracts.OutcomeCannotFix, aicontracts.OutcomeTimedOut,
		aicontracts.OutcomeUnclassified, aicontracts.OutcomeInterrupted:
		return string(outcome)
	default:
		if status == aicontracts.InvestigationStatusFailed {
			return "failed"
		}
		return string(aicontracts.OutcomeUnclassified)
	}
}

func (c *ConfigPersistence) loadPatrolInvestigationLedgerLocked() (*patrolInvestigationLedger, error) {
	info, err := c.fs.Stat(c.patrolInvestigationLedgerPath())
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 32<<20 {
		return nil, fmt.Errorf("invalid investigation telemetry file")
	}
	raw, err := c.fs.ReadFile(c.patrolInvestigationLedgerPath())
	if err != nil {
		return nil, err
	}
	var ledger patrolInvestigationLedger
	if err := json.Unmarshal(raw, &ledger); err != nil {
		return nil, err
	}
	if ledger.Version != 1 || ledger.ObservedSince.IsZero() || ledger.Runs == nil || len(ledger.Runs) > patrolInvestigationTelemetryLimit {
		return nil, fmt.Errorf("invalid investigation telemetry ledger")
	}
	for key, run := range ledger.Runs {
		decoded, err := hex.DecodeString(key)
		handoff := run.Outcome == string(aicontracts.OutcomeNeedsAttention) || run.Outcome == string(aicontracts.OutcomeCannotFix)
		if err != nil || len(decoded) != sha256.Size || run.CompletedAt.IsZero() ||
			run.CompletedAt.Before(ledger.ObservedSince) ||
			(run.Outcome != "failed" && canonicalInvestigationMeasurementOutcome(aicontracts.InvestigationOutcome(run.Outcome), "") != run.Outcome) ||
			(handoff && !run.Handoff.Valid()) || (!handoff && run.Handoff != "") {
			return nil, fmt.Errorf("invalid investigation measurement")
		}
	}
	return &ledger, nil
}

func (c *ConfigPersistence) savePatrolInvestigationLedgerLocked(ledger *patrolInvestigationLedger) error {
	raw, err := json.Marshal(ledger)
	if err != nil {
		return err
	}
	// Measurements are independent runtime history, not importable settings.
	// An unrelated configuration rollback must not discard observations.
	return writeConfigFileAtomic(c.fs, c.patrolInvestigationLedgerPath(), raw, 0600)
}

// EnsurePatrolInvestigationTelemetry begins a new, explicitly partial window.
// It never backfills pre-upgrade sessions from a capped history or findings.
// Existing corrupt/unavailable history must not become a healthy empty ledger.
func (c *ConfigPersistence) EnsurePatrolInvestigationTelemetry(now time.Time) error {
	state := c.patrolInvestigationLedgerState()
	state.Lock()
	defer state.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.loadPatrolInvestigationLedgerLocked(); !os.IsNotExist(err) {
		return err
	}
	return c.savePatrolInvestigationLedgerLocked(&patrolInvestigationLedger{Version: 1, ObservedSince: now.UTC(), Runs: map[string]PatrolInvestigationMeasurement{}})
}

// RecordPatrolInvestigationMeasurement upserts one closed run, regardless of
// whether its finding still exists. Replays and later action verification
// replace its current outcome rather than incrementing a second counter.
func (c *ConfigPersistence) RecordPatrolInvestigationMeasurement(orgID string, session *aicontracts.InvestigationSession, now time.Time) error {
	return c.RecordPatrolInvestigationMeasurements(orgID, []*aicontracts.InvestigationSession{session}, now)
}

// RecordPatrolInvestigationMeasurements replays startup history with one read
// and atomic write, rather than rewriting the full ledger for every session.
func (c *ConfigPersistence) RecordPatrolInvestigationMeasurements(orgID string, sessions []*aicontracts.InvestigationSession, now time.Time) (retErr error) {
	state := c.patrolInvestigationLedgerState()
	state.Lock()
	defer state.Unlock()
	defer func() {
		if retErr != nil {
			state.writeFailed = true
		}
	}()
	for _, session := range sessions {
		if session == nil || session.ID == "" || session.CompletedAt == nil || session.CompletedAt.IsZero() ||
			(session.Status != aicontracts.InvestigationStatusCompleted && session.Status != aicontracts.InvestigationStatusFailed && session.Status != aicontracts.InvestigationStatusNeedsAttention) {
			continue
		}
		outcome := canonicalInvestigationMeasurementOutcome(session.Outcome, session.Status)
		reason := session.HandoffReason
		if outcome == string(aicontracts.OutcomeNeedsAttention) || outcome == string(aicontracts.OutcomeCannotFix) {
			if session.Action != nil && session.Action.State == "expired" {
				reason = aicontracts.HandoffActionExpired
			} else if session.CommandsDisabled != nil {
				reason = aicontracts.HandoffCommandsDisabled
			} else if session.GuestActionsUnavailable != nil {
				reason = aicontracts.HandoffGuestActionsUnavailable
			}
			if !reason.Valid() {
				reason = aicontracts.HandoffUnspecified
			}
		} else {
			reason = ""
		}
		key := sha256.Sum256([]byte(orgID + "\x00" + session.ID))
		state.pending[hex.EncodeToString(key[:])] = PatrolInvestigationMeasurement{CompletedAt: session.CompletedAt.UTC(), Outcome: outcome, Handoff: reason}
	}
	if len(state.pending) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ledger, err := c.loadPatrolInvestigationLedgerLocked()
	if err != nil {
		return err
	}
	cutoff := now.UTC().Add(-30 * 24 * time.Hour)
	changed := false
	for key, run := range ledger.Runs {
		if run.CompletedAt.Before(cutoff) {
			delete(ledger.Runs, key)
			changed = true
		}
	}
	for key, run := range state.pending {
		if existing, ok := ledger.Runs[key]; ok {
			run.CompletedAt = existing.CompletedAt
		}
		if !run.CompletedAt.Before(ledger.ObservedSince) && !run.CompletedAt.Before(cutoff) && !run.CompletedAt.After(now) {
			previous, exists := ledger.Runs[key]
			changed = changed || !exists || !previous.CompletedAt.Equal(run.CompletedAt) || previous.Outcome != run.Outcome || previous.Handoff != run.Handoff
			ledger.Runs[key] = run
		}
	}
	if len(ledger.Runs) > patrolInvestigationTelemetryLimit {
		return fmt.Errorf("investigation measurement capacity exceeded")
	}
	if changed || state.writeFailed {
		if err := c.savePatrolInvestigationLedgerLocked(ledger); err != nil {
			return err
		}
	}
	clear(state.pending)
	state.writeFailed = false
	return nil
}

// PatrolInvestigationTelemetry reports only a complete successful ledger read.
// Observation days are elapsed age, not uptime; a new window may be zero days.
func (c *ConfigPersistence) PatrolInvestigationTelemetry(now time.Time) PatrolInvestigationTelemetrySummary {
	result := PatrolInvestigationTelemetrySummary{}
	state := c.patrolInvestigationLedgerState()
	state.Lock()
	defer state.Unlock()
	if len(state.pending) > 0 {
		return result
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ledger, err := c.loadPatrolInvestigationLedgerLocked()
	if err != nil || ledger.ObservedSince.After(now) {
		return result
	}
	// Lazy retention runs even on a read with no subsequent investigations.
	cutoff := now.UTC().Add(-30 * 24 * time.Hour)
	pruned := false
	for key, run := range ledger.Runs {
		if run.CompletedAt.Before(cutoff) {
			delete(ledger.Runs, key)
			pruned = true
		}
	}
	if pruned {
		if err := c.savePatrolInvestigationLedgerLocked(ledger); err != nil {
			return result
		}
	}
	result.Available = true
	result.ObservationDays = min(30, int(now.Sub(ledger.ObservedSince).Hours()/24))
	result.Outcomes = map[string]int{}
	result.Handoffs = map[aicontracts.InvestigationHandoffReason]int{}
	for _, run := range ledger.Runs {
		if run.CompletedAt.Before(cutoff) || run.CompletedAt.After(now) {
			continue
		}
		result.Runs++
		result.Outcomes[run.Outcome]++
		if run.Handoff != "" {
			result.Handoffs[run.Handoff]++
		}
	}
	return result
}
