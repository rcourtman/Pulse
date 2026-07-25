package unifiedresources

import (
	"strings"
	"testing"
	"time"
)

// Branch/function coverage tests for the three PURE helpers in
// action_dispatch.go that the existing suite reaches only partially:
//   - ActionDispatchAttemptID(string) string           [action_dispatch.go:65]  was 75.0%
//   - NewActionDispatchAttempt(string, time.Time) (...) [action_dispatch.go:73]  was 75.0%
//   - NormalizeActionDispatchReceipt(ActionDispatchReceipt) (ActionDispatchReceipt, error)
//                                                       [action_dispatch.go:140] was 72.7%
//
// Each subtest drives a concrete branch/return path the existing tests miss:
// the empty/whitespace identifier arm of ActionDispatchAttemptID, the
// zero-time arm of NewActionDispatchAttempt, and the identity-error,
// absent-optional-field and zero-timestamp arms of NormalizeActionDispatchReceipt,
// plus determinism and idempotence.
//
// Conventions (package clause, table-driven subtests, in-package construction
// of inputs, t.Fatalf/t.Errorf assertions) mirror the sibling
// action_dispatch_bind_branchcov0723am_test.go. No source file or pre-existing
// test is modified; the SQLite-backed store methods in action_dispatch_store.go
// are explicitly out of scope.

// ---------------------------------------------------------------------------
// ActionDispatchAttemptID  [action_dispatch.go:65]
// ---------------------------------------------------------------------------

// TestBranchcov0724pmActionDispatchAttemptID drives both arms of the if/else
// (empty-after-trim returns ""; otherwise returns actionID+".dispatch.1"),
// exercises strings.TrimSpace on the input, and pins determinism plus input
// sensitivity.
func TestBranchcov0724pmActionDispatchAttemptID(t *testing.T) {
	cases := []struct {
		name     string
		actionID string
		want     string
	}{
		{
			// Empty arm of `if actionID == ""`: the bare zero input.
			name:     "EmptyReturnsEmpty",
			actionID: "",
			want:     "",
		},
		{
			// Empty arm reached via TrimSpace: whitespace-only input must
			// collapse to "" and hit the early return, not the suffix path.
			name:     "WhitespaceOnlyTrimsToEmpty",
			actionID: "   \t\n",
			want:     "",
		},
		{
			// Non-empty arm: concrete canonical id for a real action id.
			name:     "NonEmptyAppendsDispatchSuffix",
			actionID: "vm:7",
			want:     "vm:7.dispatch.1",
		},
		{
			// Non-empty arm reached after TrimSpace: surrounding whitespace
			// is stripped before the suffix is appended, proving the trim
			// runs unconditionally and feeds the suffix path.
			name:     "WhitespacePaddedIsTrimmedBeforeSuffix",
			actionID: "  vm:7\t",
			want:     "vm:7.dispatch.1",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := ActionDispatchAttemptID(tc.actionID); got != tc.want {
				t.Fatalf("ActionDispatchAttemptID(%q) = %q, want %q", tc.actionID, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewActionDispatchAttempt  [action_dispatch.go:73]
// ---------------------------------------------------------------------------

// TestBranchcov0724pmNewActionDispatchAttempt drives both arms of the
// `if now.IsZero()` conditional: the zero-time arm (now is replaced with
// time.Now().UTC()) and the non-zero arm (now is converted to UTC). It also
// pins the generated id and the action-id trim.
func TestBranchcov0724pmNewActionDispatchAttempt(t *testing.T) {
	t.Run("ZeroNowReplacedWithCurrentUTCTime", func(t *testing.T) {
		// The zero-time arm: passing time.Time{} must NOT leave CreatedAt at
		// the zero value; the constructor substitutes time.Now().UTC().
		before := time.Now().UTC().Add(-time.Second)
		got, err := NewActionDispatchAttempt("act-zeronow", time.Time{})
		if err != nil {
			t.Fatalf("NewActionDispatchAttempt with zero now unexpected error: %v", err)
		}
		after := time.Now().UTC().Add(time.Second)

		if got.CreatedAt.IsZero() {
			t.Fatal("CreatedAt is zero; zero-time arm did not substitute time.Now().UTC()")
		}
		if got.CreatedAt.Location() != time.UTC {
			t.Fatalf("CreatedAt location = %v, want UTC", got.CreatedAt.Location())
		}
		// The substituted instant must fall in the [before, after] window
		// around the real clock — a concrete, non-tautological check.
		if got.CreatedAt.Before(before) || got.CreatedAt.After(after) {
			t.Fatalf("CreatedAt = %v, expected within [%v, %v]", got.CreatedAt, before, after)
		}
		// UpdatedAt mirrors CreatedAt in the constructor.
		if !got.UpdatedAt.Equal(got.CreatedAt) {
			t.Fatalf("UpdatedAt = %v, want equal to CreatedAt %v", got.UpdatedAt, got.CreatedAt)
		}
		// The generated id is the canonical dispatch id for this action.
		if got.ID != "act-zeronow.dispatch.1" {
			t.Fatalf("ID = %q, want act-zeronow.dispatch.1", got.ID)
		}
		if got.State != ActionDispatchQueued {
			t.Fatalf("State = %q, want %q", got.State, ActionDispatchQueued)
		}
	})

	t.Run("NonZeroNowIsConvertedToUTC", func(t *testing.T) {
		// The non-zero arm: a non-UTC wall-clock must be normalised to UTC
		// without changing the instant.
		zone := time.FixedZone("PST", -8*3600)
		local := time.Date(2026, 7, 24, 9, 30, 0, 0, zone) // 09:30 PST == 17:30 UTC
		got, err := NewActionDispatchAttempt("act-utcnorm", local)
		if err != nil {
			t.Fatalf("NewActionDispatchAttempt unexpected error: %v", err)
		}
		wantUTC := local.UTC()
		if !got.CreatedAt.Equal(wantUTC) {
			t.Fatalf("CreatedAt = %v, want %v (UTC of input)", got.CreatedAt, wantUTC)
		}
		if got.CreatedAt.Location() != time.UTC {
			t.Fatalf("CreatedAt location = %v, want UTC", got.CreatedAt.Location())
		}
		if got.CreatedAt.Hour() != 17 {
			t.Fatalf("CreatedAt hour = %d, want 17 (09:30 PST -> 17:30 UTC)", got.CreatedAt.Hour())
		}
	})

	t.Run("WhitespaceActionIDIsTrimmed", func(t *testing.T) {
		got, err := NewActionDispatchAttempt("  act-trim  ", time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("NewActionDispatchAttempt unexpected error: %v", err)
		}
		if got.ActionID != "act-trim" {
			t.Fatalf("ActionID = %q, want act-trim (trimmed)", got.ActionID)
		}
		if got.ID != "act-trim.dispatch.1" {
			t.Fatalf("ID = %q, want act-trim.dispatch.1", got.ID)
		}
	})

	t.Run("EmptyActionIDWithZeroNowErrorsAfterClockSubstitution", func(t *testing.T) {
		// Zero now still triggers the clock substitution, but the empty
		// action id then fails inside NormalizeActionDispatchAttempt. This
		// proves the zero-time arm runs even on the error path.
		_, err := NewActionDispatchAttempt("", time.Time{})
		if err == nil {
			t.Fatal("expected error for empty action id, got nil")
		}
		if !strings.Contains(err.Error(), "action dispatch action id required") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "action dispatch action id required")
		}
	})
}

// ---------------------------------------------------------------------------
// NormalizeActionDispatchReceipt  [action_dispatch.go:140]
// ---------------------------------------------------------------------------

// TestBranchcov0724pmNormalizeActionDispatchReceipt covers every arm the
// existing store-backed tests miss: both identity-invalid error conditions
// (empty/mismatched), the absent-optional-field defaults (empty
// TransportRequestID -> AttemptID; zero ReceivedAt -> time.Now().UTC()), the
// UTC conversion of a supplied non-zero ReceivedAt, whitespace trimming, and
// idempotence of a second normalization.
func TestBranchcov0724pmNormalizeActionDispatchReceipt(t *testing.T) {
	t.Run("Error/EmptyActionIDIsInvalid", func(t *testing.T) {
		// First clause of the identity guard: ActionID trims to empty.
		_, err := NormalizeActionDispatchReceipt(ActionDispatchReceipt{
			AttemptID: "x.dispatch.1", // value irrelevant; empty ActionID short-circuits
		})
		if err == nil {
			t.Fatal("expected error for empty ActionID, got nil")
		}
		if !strings.Contains(err.Error(), "action dispatch receipt identity is invalid") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "action dispatch receipt identity is invalid")
		}
	})

	t.Run("Error/WhitespaceActionIDTrimsToEmpty", func(t *testing.T) {
		_, err := NormalizeActionDispatchReceipt(ActionDispatchReceipt{
			ActionID: "   ", AttemptID: "x.dispatch.1",
		})
		if err == nil {
			t.Fatal("expected error for whitespace-only ActionID, got nil")
		}
		if !strings.Contains(err.Error(), "action dispatch receipt identity is invalid") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "action dispatch receipt identity is invalid")
		}
	})

	t.Run("Error/MismatchedAttemptIDIsInvalid", func(t *testing.T) {
		// Second clause: ActionID is valid but AttemptID is not its
		// canonical ActionDispatchAttemptID form.
		_, err := NormalizeActionDispatchReceipt(ActionDispatchReceipt{
			ActionID: "act-mismatch", AttemptID: "not-the-canonical-id",
		})
		if err == nil {
			t.Fatal("expected error for mismatched AttemptID, got nil")
		}
		if !strings.Contains(err.Error(), "action dispatch receipt identity is invalid") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "action dispatch receipt identity is invalid")
		}
	})

	t.Run("Error/EmptyAttemptIDWithValidActionIDIsMismatch", func(t *testing.T) {
		// Empty AttemptID != ActionDispatchAttemptID(ActionID), so the
		// mismatch clause fires (not the empty-ActionID clause).
		_, err := NormalizeActionDispatchReceipt(ActionDispatchReceipt{
			ActionID: "act-noattempt", AttemptID: "",
		})
		if err == nil {
			t.Fatal("expected error for empty AttemptID, got nil")
		}
		if !strings.Contains(err.Error(), "action dispatch receipt identity is invalid") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "action dispatch receipt identity is invalid")
		}
	})

	t.Run("Success/AbsentOptionalFieldsDefaulted", func(t *testing.T) {
		// Both optional fields absent: TransportRequestID empty -> AttemptID,
		// ReceivedAt zero -> time.Now().UTC().
		before := time.Now().UTC().Add(-time.Second)
		got, err := NormalizeActionDispatchReceipt(ActionDispatchReceipt{
			ActionID: "act-absent", AttemptID: ActionDispatchAttemptID("act-absent"),
			// TransportRequestID and ReceivedAt intentionally zero.
		})
		if err != nil {
			t.Fatalf("NormalizeActionDispatchReceipt unexpected error: %v", err)
		}
		after := time.Now().UTC().Add(time.Second)

		// Empty TransportRequestID must be defaulted to the AttemptID.
		if got.TransportRequestID != got.AttemptID {
			t.Fatalf("TransportRequestID = %q, want defaulted to AttemptID %q", got.TransportRequestID, got.AttemptID)
		}
		if got.TransportRequestID != "act-absent.dispatch.1" {
			t.Fatalf("TransportRequestID = %q, want act-absent.dispatch.1", got.TransportRequestID)
		}
		// Zero ReceivedAt must be replaced with the current UTC instant.
		if got.ReceivedAt.IsZero() {
			t.Fatal("ReceivedAt is zero; zero-time arm did not substitute time.Now().UTC()")
		}
		if got.ReceivedAt.Location() != time.UTC {
			t.Fatalf("ReceivedAt location = %v, want UTC", got.ReceivedAt.Location())
		}
		if got.ReceivedAt.Before(before) || got.ReceivedAt.After(after) {
			t.Fatalf("ReceivedAt = %v, expected within [%v, %v]", got.ReceivedAt, before, after)
		}
	})

	t.Run("Success/SuppliedFieldsPreservedAndConvertedToUTC", func(t *testing.T) {
		// Non-empty TransportRequestID is kept; non-zero ReceivedAt in a
		// non-UTC zone is converted to UTC without changing the instant.
		zone := time.FixedZone("JST", 9*3600)
		local := time.Date(2026, 7, 24, 23, 0, 0, 0, zone) // 23:00 JST == 14:00 UTC
		got, err := NormalizeActionDispatchReceipt(ActionDispatchReceipt{
			ActionID:           "act-supplied",
			AttemptID:          ActionDispatchAttemptID("act-supplied"),
			TransportRequestID: "transport-xyz",
			ReceivedAt:         local,
		})
		if err != nil {
			t.Fatalf("NormalizeActionDispatchReceipt unexpected error: %v", err)
		}
		if got.TransportRequestID != "transport-xyz" {
			t.Fatalf("TransportRequestID = %q, want transport-xyz (preserved)", got.TransportRequestID)
		}
		wantUTC := local.UTC()
		if !got.ReceivedAt.Equal(wantUTC) {
			t.Fatalf("ReceivedAt = %v, want %v (UTC of input)", got.ReceivedAt, wantUTC)
		}
		if got.ReceivedAt.Location() != time.UTC {
			t.Fatalf("ReceivedAt location = %v, want UTC", got.ReceivedAt.Location())
		}
		if got.ReceivedAt.Hour() != 14 {
			t.Fatalf("ReceivedAt hour = %d, want 14 (23:00 JST -> 14:00 UTC)", got.ReceivedAt.Hour())
		}
	})

	t.Run("Success/WhitespaceFieldsTrimmedAndStillValid", func(t *testing.T) {
		// All three string fields are padded; after trim the identity still
		// matches and the transport id is preserved (trimmed).
		got, err := NormalizeActionDispatchReceipt(ActionDispatchReceipt{
			ActionID:           "  act-ws  ",
			AttemptID:          "\tact-ws.dispatch.1\t",
			TransportRequestID: " transport-ws ",
			ReceivedAt:         time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("NormalizeActionDispatchReceipt unexpected error: %v", err)
		}
		if got.ActionID != "act-ws" {
			t.Fatalf("ActionID = %q, want act-ws (trimmed)", got.ActionID)
		}
		if got.AttemptID != "act-ws.dispatch.1" {
			t.Fatalf("AttemptID = %q, want act-ws.dispatch.1 (trimmed)", got.AttemptID)
		}
		if got.TransportRequestID != "transport-ws" {
			t.Fatalf("TransportRequestID = %q, want transport-ws (trimmed)", got.TransportRequestID)
		}
	})

	t.Run("Idempotent/AlreadyNormalizedReceiptPassesThroughUnchanged", func(t *testing.T) {
		// A receipt that is already canonical must normalize to an identical
		// value: TransportRequestID already set runs the keep arm, ReceivedAt
		// already set runs the UTC arm (a no-op on an already-UTC instant).
		fixedReceived := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		once, err := NormalizeActionDispatchReceipt(ActionDispatchReceipt{
			ActionID:           "act-idem",
			AttemptID:          ActionDispatchAttemptID("act-idem"),
			TransportRequestID: "transport-idem",
			ReceivedAt:         fixedReceived,
		})
		if err != nil {
			t.Fatalf("first normalize unexpected error: %v", err)
		}
		// Normalizing the already-normalized receipt again must not drift.
		twice, err := NormalizeActionDispatchReceipt(once)
		if err != nil {
			t.Fatalf("second normalize unexpected error: %v", err)
		}
		if twice != once {
			t.Fatalf("idempotence broken: second normalization drifted\nonce = %+v\ntwice= %+v", once, twice)
		}
	})

	t.Run("Idempotent/AbsentFieldsStabilizeOnSecondNormalization", func(t *testing.T) {
		// First normalization defaults the absent optional fields; the
		// second normalization (now over a fully-populated receipt) must be
		// a fixed point, proving the defaults are themselves canonical.
		once, err := NormalizeActionDispatchReceipt(ActionDispatchReceipt{
			ActionID:  "act-stable",
			AttemptID: ActionDispatchAttemptID("act-stable"),
			// both optional fields absent on first pass
		})
		if err != nil {
			t.Fatalf("first normalize unexpected error: %v", err)
		}
		twice, err := NormalizeActionDispatchReceipt(once)
		if err != nil {
			t.Fatalf("second normalize unexpected error: %v", err)
		}
		if twice != once {
			t.Fatalf("idempotence broken after defaulting:\nonce = %+v\ntwice= %+v", once, twice)
		}
	})
}
