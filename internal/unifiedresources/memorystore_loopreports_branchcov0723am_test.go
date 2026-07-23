package unifiedresources

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Branch-coverage tests for currently-0.0%-covered MemoryStore methods in
// internal/unifiedresources/loop_reports_store.go:
//   - MemoryStore.ListResourceOperatorStates
//   - MemoryStore.GetLoopReport
//   - MemoryStore.FindLoopReportByWindow
//   - MemoryStore.UpdateLoopReportUserOutcome
//
// Every subtest constructs its OWN MemoryStore so it passes when run alone via
// -run. The newLoopReport helper and the LoopReport / ResourceOperatorState
// types come from sibling _test.go / source files in this same package.

// branchcov0723amFullReport returns a LoopReport with EVERY optional field
// populated so a round-trip read can assert each one. Values are chosen
// already-trimmed, already-UTC, and unique so NormalizeLoopReport is a no-op
// for them and the expected post-store value can be written out by hand.
func branchcov0723amFullReport(id, scope string, windowStart, windowEnd time.Time, status LoopReportStatus) LoopReport {
	r := newLoopReport(id, scope, windowEnd, status)
	r.Goal = "verify recovery goal"
	r.WindowStartedAt = &windowStart
	r.LinkedFindingIDs = []string{"finding-1", "finding-2"}
	r.LinkedAlertIDs = []string{"alert-1"}
	r.LinkedActionIDs = []string{"action-1", "action-2", "action-3"}
	r.LinkedPatrolRunID = "patrol-run-42"
	r.Recommendation = "operator should verify cpu baseline"
	r.Evidence = LoopReportEvidence{
		OperatorStateSummary:          "maintenance window ended",
		ActiveCriticalAlerts:          1,
		ActiveWarningAlerts:           2,
		ActiveCriticalFindings:        0,
		ActiveWarningFindings:         3,
		FailedActionsSinceWindowStart: 1,
		MetricRecovery: &MetricRecoveryEvidence{
			MetricsObserved: []string{"cpu", "memory"},
			SamplesAfterEnd: 5,
			Trend:           "improving",
			Note:            "trending back to baseline",
		},
	}
	return r
}

// ---------------------------------------------------------------------------
// MemoryStore.ListResourceOperatorStates
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_MemListResourceOperatorStates(t *testing.T) {
	// emptyStore: the method must return a non-nil empty slice and no error.
	t.Run("empty_store_returns_empty_slice", func(t *testing.T) {
		store := NewMemoryStore()
		got, err := store.ListResourceOperatorStates()
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if got == nil {
			t.Fatal("got nil slice, want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("len=%d want 0", len(got))
		}
	})

	// severalStates: every seeded state must come back by its canonical id,
	// with its persisted scalar fields intact.
	t.Run("several_states_all_returned", func(t *testing.T) {
		store := NewMemoryStore()
		now := time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)
		seeded := []ResourceOperatorState{
			{CanonicalID: "vm:1", IntentionallyOffline: true, Criticality: CriticalityHigh, SetAt: now, SetBy: "alice"},
			{CanonicalID: "vm:2", NeverAutoRemediate: true, Note: "do not touch", SetAt: now, SetBy: "bob"},
			{CanonicalID: "vm:3", SetAt: now, SetBy: "carol"},
		}
		for _, s := range seeded {
			if err := store.SetResourceOperatorState(s); err != nil {
				t.Fatalf("seed %s: %v", s.CanonicalID, err)
			}
		}
		got, err := store.ListResourceOperatorStates()
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if len(got) != len(seeded) {
			t.Fatalf("len=%d want %d", len(got), len(seeded))
		}
		byID := make(map[string]ResourceOperatorState, len(got))
		for _, s := range got {
			byID[s.CanonicalID] = s
		}
		for _, want := range seeded {
			g, ok := byID[want.CanonicalID]
			if !ok {
				t.Fatalf("canonical id %q missing from result", want.CanonicalID)
			}
			if g.IntentionallyOffline != want.IntentionallyOffline {
				t.Fatalf("%q IntentionallyOffline=%v want %v", want.CanonicalID, g.IntentionallyOffline, want.IntentionallyOffline)
			}
			if g.NeverAutoRemediate != want.NeverAutoRemediate {
				t.Fatalf("%q NeverAutoRemediate=%v want %v", want.CanonicalID, g.NeverAutoRemediate, want.NeverAutoRemediate)
			}
			if g.Criticality != want.Criticality {
				t.Fatalf("%q Criticality=%q want %q", want.CanonicalID, g.Criticality, want.Criticality)
			}
			if g.Note != want.Note {
				t.Fatalf("%q Note=%q want %q", want.CanonicalID, g.Note, want.Note)
			}
			if g.SetBy != want.SetBy {
				t.Fatalf("%q SetBy=%q want %q", want.CanonicalID, g.SetBy, want.SetBy)
			}
		}
	})

	// resultIndependentOfInternalMap: mutating the returned slice / its
	// elements must not affect a subsequent listing (the method returns a
	// copy of each map value, not a live reference into the store).
	t.Run("result_independent_of_internal_map", func(t *testing.T) {
		store := NewMemoryStore()
		now := time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)
		if err := store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: "vm:1", SetAt: now, SetBy: "alice"}); err != nil {
			t.Fatalf("seed: %v", err)
		}
		first, err := store.ListResourceOperatorStates()
		if err != nil {
			t.Fatalf("first list: %v", err)
		}
		if len(first) != 1 {
			t.Fatalf("first len=%d want 1", len(first))
		}
		// Mutate the returned copy in every way a caller might: drop the
		// element, rewrite a scalar, and clear the slice header.
		originalID := first[0].CanonicalID
		first[0].CanonicalID = "MUTATED"
		first[0].IntentionallyOffline = true
		first = first[:0]

		second, err := store.ListResourceOperatorStates()
		if err != nil {
			t.Fatalf("second list: %v", err)
		}
		if len(second) != 1 {
			t.Fatalf("second len=%d want 1 (mutation leaked into store)", len(second))
		}
		if second[0].CanonicalID != originalID {
			t.Fatalf("CanonicalID=%q want %q (mutation leaked into store)", second[0].CanonicalID, originalID)
		}
		if second[0].IntentionallyOffline {
			t.Fatal("IntentionallyOffline=true want false (mutation leaked into store)")
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.GetLoopReport
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_MemGetLoopReport(t *testing.T) {
	// missingID: a non-empty id that was never recorded -> zero value,
	// found=false, no error.
	t.Run("missing_id_returns_zero_value", func(t *testing.T) {
		store := NewMemoryStore()
		got, found, err := store.GetLoopReport("never-recorded")
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if found {
			t.Fatal("found=true want false")
		}
		if got.ID != "" || got.Scope != "" || got.Status != "" {
			t.Fatalf("expected zero LoopReport, got %#v", got)
		}
	})

	// presentID: every field round-trips through record -> get.
	t.Run("present_id_round_trips_every_field", func(t *testing.T) {
		store := NewMemoryStore()
		windowStart := time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC)
		windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
		report := branchcov0723amFullReport("mv-rt-full", "vm:1", windowStart, windowEnd, LoopReportStatusNeedsReview)
		if err := store.RecordLoopReport(report); err != nil {
			t.Fatalf("record: %v", err)
		}
		got, found, err := store.GetLoopReport(report.ID)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if !found {
			t.Fatal("found=false want true")
		}
		// Scalar fields.
		if got.ID != "mv-rt-full" {
			t.Fatalf("ID=%q", got.ID)
		}
		if got.Type != LoopReportTypeMaintenanceVerification {
			t.Fatalf("Type=%q", got.Type)
		}
		if got.Scope != "vm:1" {
			t.Fatalf("Scope=%q", got.Scope)
		}
		if got.Trigger != "maintenance_window_end" {
			t.Fatalf("Trigger=%q", got.Trigger)
		}
		if got.Goal != "verify recovery goal" {
			t.Fatalf("Goal=%q", got.Goal)
		}
		if got.Status != LoopReportStatusNeedsReview {
			t.Fatalf("Status=%q", got.Status)
		}
		if got.LinkedPatrolRunID != "patrol-run-42" {
			t.Fatalf("LinkedPatrolRunID=%q", got.LinkedPatrolRunID)
		}
		if got.Recommendation != "operator should verify cpu baseline" {
			t.Fatalf("Recommendation=%q", got.Recommendation)
		}
		if got.UserOutcome != "" {
			t.Fatalf("UserOutcome=%q want empty (not reviewed)", got.UserOutcome)
		}
		if got.ReviewedBy != "" || got.ReviewNote != "" {
			t.Fatalf("review fields non-empty: by=%q note=%q", got.ReviewedBy, got.ReviewNote)
		}
		// Time fields.
		wantStarted := windowEnd.Add(time.Minute)
		if !got.StartedAt.Equal(wantStarted) {
			t.Fatalf("StartedAt=%v want %v", got.StartedAt, wantStarted)
		}
		if !got.CompletedAt.Equal(wantStarted) {
			t.Fatalf("CompletedAt=%v want %v", got.CompletedAt, wantStarted)
		}
		if got.WindowStartedAt == nil || !got.WindowStartedAt.Equal(windowStart) {
			t.Fatalf("WindowStartedAt=%v want %v", got.WindowStartedAt, windowStart)
		}
		if got.WindowEndedAt == nil || !got.WindowEndedAt.Equal(windowEnd) {
			t.Fatalf("WindowEndedAt=%v want %v", got.WindowEndedAt, windowEnd)
		}
		if got.ReviewedAt != nil {
			t.Fatalf("ReviewedAt=%v want nil", got.ReviewedAt)
		}
		// Slice fields.
		if !reflect.DeepEqual(got.LinkedFindingIDs, []string{"finding-1", "finding-2"}) {
			t.Fatalf("LinkedFindingIDs=%#v", got.LinkedFindingIDs)
		}
		if !reflect.DeepEqual(got.LinkedAlertIDs, []string{"alert-1"}) {
			t.Fatalf("LinkedAlertIDs=%#v", got.LinkedAlertIDs)
		}
		if !reflect.DeepEqual(got.LinkedActionIDs, []string{"action-1", "action-2", "action-3"}) {
			t.Fatalf("LinkedActionIDs=%#v", got.LinkedActionIDs)
		}
		// Evidence struct (including nested MetricRecovery).
		wantEvidence := LoopReportEvidence{
			OperatorStateSummary:          "maintenance window ended",
			ActiveCriticalAlerts:          1,
			ActiveWarningAlerts:           2,
			ActiveCriticalFindings:        0,
			ActiveWarningFindings:         3,
			FailedActionsSinceWindowStart: 1,
			MetricRecovery: &MetricRecoveryEvidence{
				MetricsObserved: []string{"cpu", "memory"},
				SamplesAfterEnd: 5,
				Trend:           "improving",
				Note:            "trending back to baseline",
			},
		}
		if !reflect.DeepEqual(got.Evidence, wantEvidence) {
			t.Fatalf("Evidence=%#v want %#v", got.Evidence, wantEvidence)
		}
	})

	// emptyID: empty and whitespace-only ids short-circuit before the lookup
	// (the trim path) and return found=false with no error.
	t.Run("empty_id_returns_not_found", func(t *testing.T) {
		store := NewMemoryStore()
		// Seed one report so a non-trimmed bug would actually find it.
		windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
		if err := store.RecordLoopReport(newLoopReport("mv-present", "vm:1", windowEnd, LoopReportStatusHealthy)); err != nil {
			t.Fatalf("seed: %v", err)
		}
		for _, id := range []string{"", "   ", "\t\n"} {
			got, found, err := store.GetLoopReport(id)
			if err != nil {
				t.Fatalf("id=%q err=%v want nil", id, err)
			}
			if found {
				t.Fatalf("id=%q found=true want false", id)
			}
			if got.ID != "" {
				t.Fatalf("id=%q got.ID=%q want empty", id, got.ID)
			}
		}
	})

	// whitespaceIDStillMatches: a recorded id with surrounding whitespace in
	// the lookup key still resolves (TrimSpace on the key path).
	t.Run("whitespace_id_still_matches", func(t *testing.T) {
		store := NewMemoryStore()
		windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
		report := newLoopReport("mv-ws", "vm:1", windowEnd, LoopReportStatusHealthy)
		if err := store.RecordLoopReport(report); err != nil {
			t.Fatalf("seed: %v", err)
		}
		got, found, err := store.GetLoopReport("  " + report.ID + "\t")
		if err != nil || !found {
			t.Fatalf("found=%v err=%v", found, err)
		}
		if got.ID != report.ID {
			t.Fatalf("ID=%q want %q", got.ID, report.ID)
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.FindLoopReportByWindow
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_MemFindLoopReportByWindow(t *testing.T) {
	windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)

	// noMatch: store holds reports but none match the triple -> not found.
	t.Run("no_match", func(t *testing.T) {
		store := NewMemoryStore()
		if err := store.RecordLoopReport(newLoopReport("mv-other", "vm:999", windowEnd, LoopReportStatusHealthy)); err != nil {
			t.Fatalf("seed: %v", err)
		}
		got, found, err := store.FindLoopReportByWindow(LoopReportTypeMaintenanceVerification, "vm:1", windowEnd)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if found {
			t.Fatal("found=true want false")
		}
		if got.ID != "" {
			t.Fatalf("got.ID=%q want empty", got.ID)
		}
	})

	// exactMatch: the full report for the matching triple comes back.
	t.Run("exact_match_returns_full_report", func(t *testing.T) {
		store := NewMemoryStore()
		windowStart := time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC)
		report := branchcov0723amFullReport("mv-exact", "vm:1", windowStart, windowEnd, LoopReportStatusNeedsReview)
		if err := store.RecordLoopReport(report); err != nil {
			t.Fatalf("seed: %v", err)
		}
		got, found, err := store.FindLoopReportByWindow(LoopReportTypeMaintenanceVerification, "vm:1", windowEnd)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if !found {
			t.Fatal("found=false want true")
		}
		if got.ID != report.ID || got.Scope != "vm:1" || got.Status != LoopReportStatusNeedsReview {
			t.Fatalf("got=%#v", got)
		}
		if got.WindowEndedAt == nil || !got.WindowEndedAt.Equal(windowEnd) {
			t.Fatalf("WindowEndedAt=%v want %v", got.WindowEndedAt, windowEnd)
		}
		if !reflect.DeepEqual(got.LinkedActionIDs, []string{"action-1", "action-2", "action-3"}) {
			t.Fatalf("LinkedActionIDs=%#v", got.LinkedActionIDs)
		}
	})

	// rightCanonicalIDWrongType: a report sharing scope + window but with a
	// different report type must not match. RecordLoopReport rejects unknown
	// types, so the different-type report is seeded directly under the lock.
	t.Run("right_canonical_id_wrong_type", func(t *testing.T) {
		store := NewMemoryStore()
		other := newLoopReport("mv-other-type", "vm:1", windowEnd, LoopReportStatusHealthy)
		other.Type = LoopReportType("other_loop")
		store.mu.Lock()
		store.loopReports[other.ID] = other
		store.mu.Unlock()
		got, found, err := store.FindLoopReportByWindow(LoopReportTypeMaintenanceVerification, "vm:1", windowEnd)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if found {
			t.Fatalf("found=true want false (matched by type wrongly); got=%#v", got)
		}
	})

	// rightTypeWrongCanonicalID: same type + window but a different scope must
	// not match.
	t.Run("right_type_wrong_canonical_id", func(t *testing.T) {
		store := NewMemoryStore()
		if err := store.RecordLoopReport(newLoopReport("mv-other-scope", "vm:999", windowEnd, LoopReportStatusHealthy)); err != nil {
			t.Fatalf("seed: %v", err)
		}
		got, found, err := store.FindLoopReportByWindow(LoopReportTypeMaintenanceVerification, "vm:1", windowEnd)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if found {
			t.Fatalf("found=true want false (matched by scope wrongly); got=%#v", got)
		}
	})

	// windowEndedAtDiffersByOneNanosecond: matching type+scope but a window
	// end that differs by exactly 1ns must NOT match — the lookup is exact,
	// not fuzzy.
	t.Run("window_ended_at_differs_by_one_nanosecond", func(t *testing.T) {
		store := NewMemoryStore()
		if err := store.RecordLoopReport(newLoopReport("mv-near", "vm:1", windowEnd, LoopReportStatusHealthy)); err != nil {
			t.Fatalf("seed: %v", err)
		}
		got, found, err := store.FindLoopReportByWindow(LoopReportTypeMaintenanceVerification, "vm:1", windowEnd.Add(time.Nanosecond))
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if found {
			t.Fatalf("found=true want false (matched a 1ns-off window); got=%#v", got)
		}
	})

	// matchedScopeTypeButNilWindowEndedAt: a report matching type+scope whose
	// WindowEndedAt is nil must be skipped (the explicit nil-check continue
	// arm), so no false match occurs. WindowEndedAt is legitimately nil here
	// because ValidateLoopReport does not require it.
	t.Run("matched_scope_type_but_nil_window_ended_at", func(t *testing.T) {
		store := NewMemoryStore()
		nilWindow := newLoopReport("mv-nil-window", "vm:1", windowEnd, LoopReportStatusHealthy)
		nilWindow.WindowEndedAt = nil
		if err := store.RecordLoopReport(nilWindow); err != nil {
			t.Fatalf("seed: %v", err)
		}
		got, found, err := store.FindLoopReportByWindow(LoopReportTypeMaintenanceVerification, "vm:1", windowEnd)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if found {
			t.Fatalf("found=true want false (nil-window report was matched); got=%#v", got)
		}
	})

	// guardEmptyCanonicalID: empty canonical id short-circuits the guard and
	// returns not found without scanning.
	t.Run("guard_empty_canonical_id", func(t *testing.T) {
		store := NewMemoryStore()
		got, found, err := store.FindLoopReportByWindow(LoopReportTypeMaintenanceVerification, "", windowEnd)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if found {
			t.Fatalf("found=true want false; got=%#v", got)
		}
	})

	// guardInvalidReportType: an unknown report type short-circuits the guard.
	t.Run("guard_invalid_report_type", func(t *testing.T) {
		store := NewMemoryStore()
		got, found, err := store.FindLoopReportByWindow(LoopReportType("bogus"), "vm:1", windowEnd)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if found {
			t.Fatalf("found=true want false; got=%#v", got)
		}
	})

	// guardZeroWindowEndedAt: a zero window end short-circuits the guard.
	t.Run("guard_zero_window_ended_at", func(t *testing.T) {
		store := NewMemoryStore()
		got, found, err := store.FindLoopReportByWindow(LoopReportTypeMaintenanceVerification, "vm:1", time.Time{})
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if found {
			t.Fatalf("found=true want false; got=%#v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.UpdateLoopReportUserOutcome
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_MemUpdateLoopReportUserOutcome(t *testing.T) {
	// emptyReportID: empty / whitespace id is rejected with ErrLoopReportInvalid
	// before any lookup.
	t.Run("empty_report_id_invalid", func(t *testing.T) {
		store := NewMemoryStore()
		err := store.UpdateLoopReportUserOutcome("  ", LoopReportUserOutcomeReviewed, "alice", "note", time.Now().UTC())
		if !errors.Is(err, ErrLoopReportInvalid) {
			t.Fatalf("err=%v want ErrLoopReportInvalid", err)
		}
		if !strings.Contains(err.Error(), "id is required") {
			t.Fatalf("err=%v want 'id is required'", err)
		}
	})

	// unknownOutcome: a value outside the known enum is rejected with
	// ErrLoopReportInvalid, distinct from the missing-id error.
	t.Run("unknown_outcome_invalid", func(t *testing.T) {
		store := NewMemoryStore()
		err := store.UpdateLoopReportUserOutcome("mv-x", LoopReportUserOutcome("bogus"), "alice", "note", time.Now().UTC())
		if !errors.Is(err, ErrLoopReportInvalid) {
			t.Fatalf("err=%v want ErrLoopReportInvalid", err)
		}
		if !strings.Contains(err.Error(), "unknown user outcome") {
			t.Fatalf("err=%v want 'unknown user outcome'", err)
		}
	})

	// unknownReportID: a valid id that was never recorded returns the concrete
	// ErrLoopReportNotFound sentinel, not a wrapped/derived error.
	t.Run("unknown_report_id_not_found", func(t *testing.T) {
		store := NewMemoryStore()
		err := store.UpdateLoopReportUserOutcome("mv-missing", LoopReportUserOutcomeReviewed, "alice", "note", time.Now().UTC())
		if !errors.Is(err, ErrLoopReportNotFound) {
			t.Fatalf("err=%v want ErrLoopReportNotFound", err)
		}
	})

	// happyAllFourFieldsRoundTrip: a successful update writes ALL four fields
	// (outcome, reviewedBy, note, reviewedAt) and leaves the immutable
	// status untouched. reviewedAt is supplied in a non-UTC zone and must be
	// stored as its UTC equivalent.
	t.Run("happy_all_four_fields_round_trip", func(t *testing.T) {
		store := NewMemoryStore()
		windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
		report := newLoopReport("mv-update-happy", "vm:1", windowEnd, LoopReportStatusNeedsReview)
		if err := store.RecordLoopReport(report); err != nil {
			t.Fatalf("seed: %v", err)
		}
		// Non-UTC offset: 2026-05-12 14:00 +02:00 == 12:00:00 UTC. Using a
		// non-UTC input exercises the reviewedAt.UTC() conversion arm.
		nonUTC := time.Date(2026, 5, 12, 14, 0, 0, 0, time.FixedZone("CEST", 2*3600))
		wantUTC := nonUTC.UTC()
		if err := store.UpdateLoopReportUserOutcome(report.ID, LoopReportUserOutcomeReviewed, "alice", "acknowledged", nonUTC); err != nil {
			t.Fatalf("update: %v", err)
		}
		got, _, err := store.GetLoopReport(report.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.UserOutcome != LoopReportUserOutcomeReviewed {
			t.Fatalf("UserOutcome=%q want %q", got.UserOutcome, LoopReportUserOutcomeReviewed)
		}
		if got.ReviewedBy != "alice" {
			t.Fatalf("ReviewedBy=%q want alice", got.ReviewedBy)
		}
		if got.ReviewNote != "acknowledged" {
			t.Fatalf("ReviewNote=%q want acknowledged", got.ReviewNote)
		}
		if got.ReviewedAt == nil || !got.ReviewedAt.Equal(wantUTC) {
			t.Fatalf("ReviewedAt=%v want %v (UTC)", got.ReviewedAt, wantUTC)
		}
		// Immutable fields must be unchanged.
		if got.Status != LoopReportStatusNeedsReview {
			t.Fatalf("Status=%q want %q (status must not be mutated by review)", got.Status, LoopReportStatusNeedsReview)
		}
	})

	// emptyReviewedByAndNoteStored: empty / whitespace reviewedBy and note are
	// NOT rejected — they are trimmed and stored as empty strings.
	t.Run("empty_reviewed_by_and_note_stored", func(t *testing.T) {
		store := NewMemoryStore()
		windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
		report := newLoopReport("mv-update-empty", "vm:1", windowEnd, LoopReportStatusNeedsReview)
		if err := store.RecordLoopReport(report); err != nil {
			t.Fatalf("seed: %v", err)
		}
		if err := store.UpdateLoopReportUserOutcome(report.ID, LoopReportUserOutcomeReviewed, "   ", "\t", time.Now().UTC()); err != nil {
			t.Fatalf("update with empty by/note: %v (expected stored, not rejected)", err)
		}
		got, _, err := store.GetLoopReport(report.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.ReviewedBy != "" {
			t.Fatalf("ReviewedBy=%q want empty (trimmed)", got.ReviewedBy)
		}
		if got.ReviewNote != "" {
			t.Fatalf("ReviewNote=%q want empty (trimmed)", got.ReviewNote)
		}
		if got.UserOutcome != LoopReportUserOutcomeReviewed {
			t.Fatalf("UserOutcome=%q want %q (outcome still written)", got.UserOutcome, LoopReportUserOutcomeReviewed)
		}
	})

	// zeroReviewedAtBackfilledToNow: a zero reviewedAt is backfilled to the
	// current UTC time (the IsZero() true arm) rather than being stored as
	// zero.
	t.Run("zero_reviewed_at_backfilled_to_now", func(t *testing.T) {
		store := NewMemoryStore()
		windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
		report := newLoopReport("mv-update-zero-at", "vm:1", windowEnd, LoopReportStatusNeedsReview)
		if err := store.RecordLoopReport(report); err != nil {
			t.Fatalf("seed: %v", err)
		}
		before := time.Now().UTC()
		if err := store.UpdateLoopReportUserOutcome(report.ID, LoopReportUserOutcomeReviewed, "alice", "note", time.Time{}); err != nil {
			t.Fatalf("update: %v", err)
		}
		after := time.Now().UTC()
		got, _, err := store.GetLoopReport(report.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.ReviewedAt == nil {
			t.Fatal("ReviewedAt=nil want non-nil (zero should be backfilled to now)")
		}
		stamped := got.ReviewedAt.UTC()
		if stamped.Before(before.Add(-2*time.Second)) || stamped.After(after.Add(2*time.Second)) {
			t.Fatalf("ReviewedAt=%v want within [%v, %v]", stamped, before, after)
		}
	})

	// updateTwiceSecondOverwrites: a second update fully overwrites the first
	// across all four writable fields (not merged).
	t.Run("update_twice_second_overwrites", func(t *testing.T) {
		store := NewMemoryStore()
		windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
		report := newLoopReport("mv-update-twice", "vm:1", windowEnd, LoopReportStatusNeedsReview)
		if err := store.RecordLoopReport(report); err != nil {
			t.Fatalf("seed: %v", err)
		}
		firstAt := time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC)
		if err := store.UpdateLoopReportUserOutcome(report.ID, LoopReportUserOutcomeReviewed, "alice", "first note", firstAt); err != nil {
			t.Fatalf("first update: %v", err)
		}
		secondAt := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
		if err := store.UpdateLoopReportUserOutcome(report.ID, "", "bob", "second note", secondAt); err != nil {
			t.Fatalf("second update: %v", err)
		}
		got, _, err := store.GetLoopReport(report.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		// All four fields must reflect the SECOND update.
		if got.UserOutcome != "" {
			t.Fatalf("UserOutcome=%q want empty (second update cleared it)", got.UserOutcome)
		}
		if got.ReviewedBy != "bob" {
			t.Fatalf("ReviewedBy=%q want bob (second update)", got.ReviewedBy)
		}
		if got.ReviewNote != "second note" {
			t.Fatalf("ReviewNote=%q want 'second note'", got.ReviewNote)
		}
		if got.ReviewedAt == nil || !got.ReviewedAt.Equal(secondAt) {
			t.Fatalf("ReviewedAt=%v want %v (second update)", got.ReviewedAt, secondAt)
		}
	})
}
