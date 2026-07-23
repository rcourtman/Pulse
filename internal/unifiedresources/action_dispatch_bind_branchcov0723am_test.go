package unifiedresources

import (
	"strings"
	"testing"
	"time"
)

// Branch/function coverage tests for three previously-uncovered (0.0%) PURE
// functions:
//   - (ActionDispatchAttempt).HasOperationBinding() bool            [action_dispatch.go:45]
//   - BindActionDispatchAttempt(ActionDispatchAttempt, ActionDispatchBinding) (ActionDispatchAttempt, error)
//                                                                    [action_dispatch.go:132]
//   - EmptyResourcePolicyPostureSummary() *ResourcePolicyPostureSummary [policy_posture.go:74]
//
// Each subtest drives a concrete branch/return path and asserts the concrete
// output value or error. No source file or pre-existing test is modified.
//
// Conventions (package clause, table-driven subtests, in-package construction
// of inputs, t.Fatalf/t.Errorf assertions) mirror the sibling
// action_dispatch_store_test.go and the recent *_branchcov*_test.go files in
// this directory.

// validDispatchBase returns a known-good ActionDispatchAttempt built through
// the public constructor so every field is canonical; callers copy it and
// mutate the specific field each failure arm needs.
func validDispatchBase(t *testing.T, actionID string, now time.Time) ActionDispatchAttempt {
	t.Helper()
	a, err := NewActionDispatchAttempt(actionID, now)
	if err != nil {
		t.Fatalf("NewActionDispatchAttempt(%q) unexpected error: %v", actionID, err)
	}
	return a
}

// ---------------------------------------------------------------------------
// HasOperationBinding
// ---------------------------------------------------------------------------

// TestBranchcov0723Am_HasOperationBinding drives both arms of every
// short-circuited conditional in HasOperationBinding (four conditions, eight
// arms) plus the strings.TrimSpace behaviour on the three string fields.
func TestBranchcov0723Am_HasOperationBinding(t *testing.T) {
	cases := []struct {
		name string
		a    ActionDispatchAttempt
		want bool
	}{
		{
			// First condition's false arm: OperationKind trims to empty,
			// short-circuits before any other condition is evaluated.
			name: "ZeroValueReturnsFalse",
			a:    ActionDispatchAttempt{},
			want: false,
		},
		{
			// First condition's true arm + second condition's false arm:
			// OperationKind is set, OperationVersion is zero.
			name: "OnlyOperationKindSetShortCircuitsBeforeVersion",
			a:    ActionDispatchAttempt{OperationKind: "patch"},
			want: false,
		},
		{
			// Second condition's true arm + third condition's false arm.
			name: "KindAndVersionSetShortCircuitsBeforeDigest",
			a:    ActionDispatchAttempt{OperationKind: "patch", OperationVersion: 3},
			want: false,
		},
		{
			// Third condition's true arm + fourth condition's false arm.
			name: "KindVersionDigestSetShortCircuitsBeforeAgentID",
			a:    ActionDispatchAttempt{OperationKind: "patch", OperationVersion: 3, RequestDigest: "sha256:abc"},
			want: false,
		},
		{
			// All four conditions' true arms -> the only path returning true.
			name: "AllFieldsSetReturnsTrue",
			a: ActionDispatchAttempt{
				OperationKind:    "patch",
				OperationVersion: 3,
				RequestDigest:    "sha256:abc",
				AgentID:          "agent-7",
			},
			want: true,
		},
		{
			// Drives the strings.TrimSpace call on OperationKind: a
			// whitespace-only OperationKind with every other field valid
			// must still return false. Without TrimSpace the first
			// condition would be true and the function would return true.
			name: "WhitespaceOperationKindTrimsToFalseDespiteOthersValid",
			a: ActionDispatchAttempt{
				OperationKind:    "   ",
				OperationVersion: 3,
				RequestDigest:    "sha256:abc",
				AgentID:          "agent-7",
			},
			want: false,
		},
		{
			// Drives the strings.TrimSpace call on RequestDigest.
			name: "WhitespaceRequestDigestTrimsToFalseDespiteOthersValid",
			a: ActionDispatchAttempt{
				OperationKind:    "patch",
				OperationVersion: 3,
				RequestDigest:    "\t ",
				AgentID:          "agent-7",
			},
			want: false,
		},
		{
			// Drives the strings.TrimSpace call on AgentID.
			name: "WhitespaceAgentIDTrimsToFalseDespiteOthersValid",
			a: ActionDispatchAttempt{
				OperationKind:    "patch",
				OperationVersion: 3,
				RequestDigest:    "sha256:abc",
				AgentID:          " ",
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.HasOperationBinding(); got != tc.want {
				t.Fatalf("HasOperationBinding() = %v, want %v (attempt=%+v)", got, tc.want, tc.a)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BindActionDispatchAttempt
// ---------------------------------------------------------------------------

// TestBranchcov0723Am_BindActionDispatchAttempt covers every validation
// failure arm surfaced through BindActionDispatchAttempt (which delegates to
// NormalizeActionDispatchAttempt), the success path asserting every field the
// binding writes, value-semantics (input is not mutated), rebinding
// (overwrite vs error), and empty/whitespace identifiers in the binding.
func TestBranchcov0723Am_BindActionDispatchAttempt(t *testing.T) {
	now := time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)

	fullBinding := ActionDispatchBinding{
		OperationKind:    "patch",
		OperationVersion: 3,
		RequestDigest:    "sha256:full",
		AgentID:          "agent-7",
	}

	t.Run("Success/WritesAllBindingFieldsAndPreservesAttemptIdentity", func(t *testing.T) {
		base := validDispatchBase(t, "act-success", now)
		// Base carries no binding to prove the bind writes the fields.
		if base.HasOperationBinding() {
			t.Fatalf("precondition: base must have no binding, got %+v", base)
		}

		got, err := BindActionDispatchAttempt(base, fullBinding)
		if err != nil {
			t.Fatalf("BindActionDispatchAttempt unexpected error: %v", err)
		}
		// Every field the binding writes must land on the returned attempt.
		if got.OperationKind != fullBinding.OperationKind {
			t.Errorf("OperationKind = %q, want %q", got.OperationKind, fullBinding.OperationKind)
		}
		if got.OperationVersion != fullBinding.OperationVersion {
			t.Errorf("OperationVersion = %d, want %d", got.OperationVersion, fullBinding.OperationVersion)
		}
		if got.RequestDigest != fullBinding.RequestDigest {
			t.Errorf("RequestDigest = %q, want %q", got.RequestDigest, fullBinding.RequestDigest)
		}
		if got.AgentID != fullBinding.AgentID {
			t.Errorf("AgentID = %q, want %q", got.AgentID, fullBinding.AgentID)
		}
		// The binding is now complete, so HasOperationBinding must agree.
		if !got.HasOperationBinding() {
			t.Fatalf("expected HasOperationBinding() true after successful bind, got %+v", got)
		}
		// Identity / lifecycle fields the bind must NOT touch are preserved.
		if got.ID != base.ID {
			t.Errorf("ID = %q, want %q (bind must not change identity)", got.ID, base.ID)
		}
		if got.ActionID != base.ActionID {
			t.Errorf("ActionID = %q, want %q", got.ActionID, base.ActionID)
		}
		if got.State != base.State {
			t.Errorf("State = %q, want %q", got.State, base.State)
		}
		if !got.CreatedAt.Equal(base.CreatedAt) {
			t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, base.CreatedAt)
		}
	})

	t.Run("Success/InputAttemptIsNotMutatedValueSemantics", func(t *testing.T) {
		base := validDispatchBase(t, "act-valuesemantics", now)

		got, err := BindActionDispatchAttempt(base, fullBinding)
		if err != nil {
			t.Fatalf("BindActionDispatchAttempt unexpected error: %v", err)
		}
		// The returned attempt must actually differ (proving the bind ran).
		if got == base {
			t.Fatalf("returned attempt identical to input; bind did not write binding fields")
		}
		if !got.HasOperationBinding() || base.HasOperationBinding() {
			t.Fatalf("value-semantics drift: got.HasOperationBinding=%v base.HasOperationBinding=%v", got.HasOperationBinding(), base.HasOperationBinding())
		}
	})

	t.Run("Success/RebindingOverwritesPreviousBindingWithoutError", func(t *testing.T) {
		// First bind establishes an "old" binding on the attempt.
		base := validDispatchBase(t, "act-rebind", now)
		first, err := BindActionDispatchAttempt(base, ActionDispatchBinding{
			OperationKind: "create", OperationVersion: 1,
			RequestDigest: "sha256:old", AgentID: "agent-old",
		})
		if err != nil {
			t.Fatalf("first bind unexpected error: %v", err)
		}
		if first.AgentID != "agent-old" {
			t.Fatalf("precondition: first bind did not write AgentID, got %q", first.AgentID)
		}

		// Re-binding with a new binding must overwrite cleanly (no error,
		// no "already bound" rejection) and the result carries the new
		// fields, not the old ones.
		rebound, err := BindActionDispatchAttempt(first, fullBinding)
		if err != nil {
			t.Fatalf("rebind returned error (expected overwrite): %v", err)
		}
		if rebound.OperationKind != fullBinding.OperationKind ||
			rebound.OperationVersion != fullBinding.OperationVersion ||
			rebound.RequestDigest != fullBinding.RequestDigest ||
			rebound.AgentID != fullBinding.AgentID {
			t.Fatalf("rebind did not overwrite every field, got %+v", rebound)
		}
		// The previous binding value must not linger anywhere.
		if rebound.AgentID == "agent-old" {
			t.Fatalf("rebind left stale AgentID from previous binding: %+v", rebound)
		}
	})

	t.Run("Failure/EmptyActionID", func(t *testing.T) {
		// ActionID=="" short-circuits before the ID/State/CreatedAt checks.
		attempt := ActionDispatchAttempt{
			ActionID: "", State: ActionDispatchQueued, CreatedAt: now,
		}
		_, err := BindActionDispatchAttempt(attempt, fullBinding)
		if err == nil {
			t.Fatal("expected error for empty ActionID, got nil")
		}
		if !strings.Contains(err.Error(), "action dispatch action id required") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "action dispatch action id required")
		}
	})

	t.Run("Failure/MismatchedAttemptID", func(t *testing.T) {
		// ActionID is valid but ID is not the canonical
		// ActionDispatchAttemptID(ActionID) form.
		attempt := ActionDispatchAttempt{
			ID: "wrong.dispatch.1", ActionID: "act-mismatch",
			State: ActionDispatchQueued, CreatedAt: now,
		}
		_, err := BindActionDispatchAttempt(attempt, fullBinding)
		if err == nil {
			t.Fatal("expected error for mismatched ID, got nil")
		}
		if !strings.Contains(err.Error(), "does not match action") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "does not match action")
		}
	})

	t.Run("Failure/UnsupportedState", func(t *testing.T) {
		// Valid identity + CreatedAt, but State is not one of the
		// supported ActionDispatchState constants.
		attempt := ActionDispatchAttempt{
			ActionID: "act-badstate", State: ActionDispatchState("bogus"),
			CreatedAt: now,
		}
		_, err := BindActionDispatchAttempt(attempt, fullBinding)
		if err == nil {
			t.Fatal("expected error for unsupported state, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported action dispatch state") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "unsupported action dispatch state")
		}
	})

	t.Run("Failure/ZeroCreatedAt", func(t *testing.T) {
		// Valid identity + valid state, but CreatedAt is the zero Time.
		attempt := ActionDispatchAttempt{
			ActionID: "act-nocreated", State: ActionDispatchQueued,
		}
		_, err := BindActionDispatchAttempt(attempt, fullBinding)
		if err == nil {
			t.Fatal("expected error for zero CreatedAt, got nil")
		}
		if !strings.Contains(err.Error(), "createdAt required") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "createdAt required")
		}
	})

	t.Run("Failure/NegativeDispatchCount", func(t *testing.T) {
		base := validDispatchBase(t, "act-negativecount", now)
		base.DispatchCount = -1
		_, err := BindActionDispatchAttempt(base, fullBinding)
		if err == nil {
			t.Fatal("expected error for negative DispatchCount, got nil")
		}
		if !strings.Contains(err.Error(), "action dispatch count cannot be negative") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "action dispatch count cannot be negative")
		}
	})

	t.Run("Failure/IncompleteBindingWithOnlyOperationKind", func(t *testing.T) {
		// Binding sets OperationKind but leaves OperationVersion at zero:
		// Normalize sees bound=true and incomplete=true and rejects.
		base := validDispatchBase(t, "act-incomplete", now)
		partial := ActionDispatchBinding{
			OperationKind: "patch",
			// OperationVersion intentionally zero.
			RequestDigest: "sha256:x",
			AgentID:       "agent-x",
		}
		_, err := BindActionDispatchAttempt(base, partial)
		if err == nil {
			t.Fatal("expected error for incomplete binding, got nil")
		}
		if !strings.Contains(err.Error(), "action dispatch operation binding is incomplete") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "action dispatch operation binding is incomplete")
		}
	})

	t.Run("EmptyAndWhitespaceBinding/AllWhitespaceTrimsToUnboundOnValidAttempt", func(t *testing.T) {
		// Every binding field is whitespace/zero. After BindActionDispatchAttempt
		// assigns them, Normalize trims the strings to empty; with all four
		// empty, `bound` is false and the incomplete check is skipped, so the
		// call succeeds and the result has no binding.
		base := validDispatchBase(t, "act-emptybinding", now)
		whitespaceBinding := ActionDispatchBinding{
			OperationKind:    "  ",
			OperationVersion: 0,
			RequestDigest:    "\t",
			AgentID:          " ",
		}
		got, err := BindActionDispatchAttempt(base, whitespaceBinding)
		if err != nil {
			t.Fatalf("expected success for all-whitespace binding on valid attempt, got error: %v", err)
		}
		if got.OperationKind != "" || got.RequestDigest != "" || got.AgentID != "" || got.OperationVersion != 0 {
			t.Fatalf("expected trimmed-to-empty binding fields, got %+v", got)
		}
		if got.HasOperationBinding() {
			t.Fatalf("expected HasOperationBinding() false after whitespace binding, got %+v", got)
		}
	})

	t.Run("EmptyAndWhitespaceBinding/WhitespaceAgentIDTriggersIncompleteAfterTrim", func(t *testing.T) {
		// AgentID is whitespace-only while the other three fields are valid:
		// after assignment the attempt is "bound" (kind/version/digest set),
		// but Normalize trims AgentID to empty, making it incomplete, so the
		// call is rejected. This proves the trim happens before the
		// completeness check.
		base := validDispatchBase(t, "act-wsagent", now)
		wsAgentBinding := ActionDispatchBinding{
			OperationKind:    "patch",
			OperationVersion: 3,
			RequestDigest:    "sha256:x",
			AgentID:          "   ",
		}
		_, err := BindActionDispatchAttempt(base, wsAgentBinding)
		if err == nil {
			t.Fatal("expected incomplete-binding error after trimming whitespace AgentID, got nil")
		}
		if !strings.Contains(err.Error(), "action dispatch operation binding is incomplete") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "action dispatch operation binding is incomplete")
		}
	})
}

// ---------------------------------------------------------------------------
// EmptyResourcePolicyPostureSummary
// ---------------------------------------------------------------------------

// TestBranchcov0723Am_EmptyResourcePolicyPostureSummary covers the canonical
// empty-contract constructor: every field is zero with non-nil empty maps
// (NormalizeCollections replaces nil maps with allocated empty ones), and two
// calls return independent pointers whose internal maps do not alias.
func TestBranchcov0723Am_EmptyResourcePolicyPostureSummary(t *testing.T) {
	t.Run("ReturnsZeroTotalWithNonNullEmptyMaps", func(t *testing.T) {
		got := EmptyResourcePolicyPostureSummary()
		if got == nil {
			t.Fatal("expected non-nil ResourcePolicyPostureSummary, got nil")
		}
		if got.TotalResources != 0 {
			t.Fatalf("TotalResources = %d, want 0", got.TotalResources)
		}
		// NormalizeCollections must allocate empty maps for each nil map;
		// asserting non-nil + len 0 proves every conditional's "set to empty"
		// arm ran.
		if got.SensitivityCounts == nil {
			t.Fatal("expected non-nil SensitivityCounts map, got nil")
		}
		if len(got.SensitivityCounts) != 0 {
			t.Fatalf("len(SensitivityCounts) = %d, want 0", len(got.SensitivityCounts))
		}
		if got.RoutingCounts == nil {
			t.Fatal("expected non-nil RoutingCounts map, got nil")
		}
		if len(got.RoutingCounts) != 0 {
			t.Fatalf("len(RoutingCounts) = %d, want 0", len(got.RoutingCounts))
		}
		if got.RedactionCounts == nil {
			t.Fatal("expected non-nil RedactionCounts map, got nil")
		}
		if len(got.RedactionCounts) != 0 {
			t.Fatalf("len(RedactionCounts) = %d, want 0", len(got.RedactionCounts))
		}
	})

	t.Run("TwoCallsReturnDistinctPointers", func(t *testing.T) {
		first := EmptyResourcePolicyPostureSummary()
		second := EmptyResourcePolicyPostureSummary()
		// Each call constructs a brand-new struct (&ResourcePolicyPostureSummary{})
		// before normalising, so the returned pointers must differ.
		if first == second {
			t.Fatalf("two calls returned the same pointer %p; constructor must allocate per call", first)
		}
	})

	t.Run("MapsAreIndependentAcrossCalls", func(t *testing.T) {
		// Mutating the maps returned by one call must not affect the maps
		// returned by another call — proves the maps themselves do not alias.
		first := EmptyResourcePolicyPostureSummary()
		second := EmptyResourcePolicyPostureSummary()

		first.SensitivityCounts[ResourceSensitivityPublic] = 42
		first.RoutingCounts[ResourceRoutingScopeLocalOnly] = 7
		first.RedactionCounts[ResourceRedactionHostname] = 99

		if got := second.SensitivityCounts[ResourceSensitivityPublic]; got != 0 {
			t.Fatalf("SensitivityCounts aliasing detected: second call sees %d after mutating first", got)
		}
		if got := second.RoutingCounts[ResourceRoutingScopeLocalOnly]; got != 0 {
			t.Fatalf("RoutingCounts aliasing detected: second call sees %d after mutating first", got)
		}
		if got := second.RedactionCounts[ResourceRedactionHostname]; got != 0 {
			t.Fatalf("RedactionCounts aliasing detected: second call sees %d after mutating first", got)
		}
		// The pointer's own TotalResources field is also independent.
		first.TotalResources = 1234
		if second.TotalResources != 0 {
			t.Fatalf("TotalResources aliasing detected: second call sees %d after mutating first", second.TotalResources)
		}
	})
}
