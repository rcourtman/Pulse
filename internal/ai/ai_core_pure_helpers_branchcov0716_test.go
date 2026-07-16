package ai

import (
	"strconv"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// TestBranchCovFindingGetSetLoopState pins the real behaviour of the
// Finding.GetLoopState / Finding.SetLoopState pair (findings.go). SetLoopState
// is a plain field assignment with no validation and no derivation side-effect
// (it deliberately does NOT call syncLoopState/deriveLoopState), so the test
// asserts: the zero value, exact round-trip, overwrite semantics, explicit
// clearing, and that arbitrary/unknown values pass through unnormalised.
func TestBranchCovFindingGetSetLoopState(t *testing.T) {
	// Zero-value Finding: GetLoopState returns the empty default.
	zero := &Finding{}
	if got := zero.GetLoopState(); got != "" {
		t.Fatalf("zero-value GetLoopState = %q, want empty", got)
	}

	// SetLoopState mutates the underlying field; GetLoopState observes the
	// exact value with no transformation.
	f := &Finding{}
	f.SetLoopState(string(FindingLoopStateInvestigating))
	if got := f.GetLoopState(); got != string(FindingLoopStateInvestigating) {
		t.Fatalf("after SetLoopState(investigating): GetLoopState = %q, want %q",
			got, FindingLoopStateInvestigating)
	}

	// SetLoopState overwrites a prior value (no append/merge).
	f.SetLoopState(string(FindingLoopStateResolved))
	if got := f.GetLoopState(); got != string(FindingLoopStateResolved) {
		t.Fatalf("after overwrite: GetLoopState = %q, want %q", got, FindingLoopStateResolved)
	}

	// SetLoopState("") explicitly clears the stored value.
	f.SetLoopState("")
	if got := f.GetLoopState(); got != "" {
		t.Fatalf("after SetLoopState(\"\"): GetLoopState = %q, want empty", got)
	}

	// Arbitrary/unknown string round-trips verbatim (no validation, no
	// re-derivation through deriveLoopState).
	const arbitrary = "  not-a-known-state  "
	f.SetLoopState(arbitrary)
	if got := f.GetLoopState(); got != arbitrary {
		t.Fatalf("arbitrary round-trip: GetLoopState = %q, want %q", got, arbitrary)
	}

	// The mutation is observable on the same struct via the exported field too,
	// confirming Get/Set operate on the single LoopState field.
	if f.LoopState != arbitrary {
		t.Fatalf("struct field LoopState = %q, want %q", f.LoopState, arbitrary)
	}
}

// TestBranchCovInvestigationRecordStatusIsTerminal exercises every arm of the
// switch in investigationRecordStatusIsTerminal (investigation_records.go):
// the three grouped terminal cases and the default fall-through, including the
// zero-value status and a status outside the known enum set.
func TestBranchCovInvestigationRecordStatusIsTerminal(t *testing.T) {
	cases := []struct {
		name   string
		status aicontracts.InvestigationStatus
		want   bool
	}{
		// Explicit terminal arms.
		{"completed_is_terminal", aicontracts.InvestigationStatusCompleted, true},
		{"failed_is_terminal", aicontracts.InvestigationStatusFailed, true},
		{"needs_attention_is_terminal", aicontracts.InvestigationStatusNeedsAttention, true},
		// Explicit non-terminal statuses -> default arm.
		{"pending_not_terminal", aicontracts.InvestigationStatusPending, false},
		{"running_not_terminal", aicontracts.InvestigationStatusRunning, false},
		// Default arm: zero-value status.
		{"empty_status_not_terminal", aicontracts.InvestigationStatus(""), false},
		// Default arm: status outside the known enum set.
		{"unknown_status_not_terminal", aicontracts.InvestigationStatus("frobnicated"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := investigationRecordStatusIsTerminal(tc.status)
			if got != tc.want {
				t.Fatalf("investigationRecordStatusIsTerminal(%q) = %v, want %v",
					tc.status, got, tc.want)
			}
		})
	}
}

// TestBranchCovExtractIncidentResourceIdentifier exercises every reachable
// branch of extractIncidentResourceIdentifier (service.go): the trimmed
// targetID early-return (and its precedence over ctx), the whitespace-only /
// empty targetID fall-through, the nil-ctx guard, each of the three context
// keys in iteration order, type-assertion failure on a non-string value, the
// non-empty-after-trim guard, and the empty-result paths.
func TestBranchCovExtractIncidentResourceIdentifier(t *testing.T) {
	cases := []struct {
		name     string
		targetID string
		ctx      map[string]interface{}
		want     string
	}{
		{
			// Branch: targetID non-empty after trim -> returned trimmed; ctx ignored.
			name:     "target_id_present_ignores_ctx",
			targetID: "  node/pve-1/qemu/100  ",
			ctx:      map[string]interface{}{"resourceID": "should-be-ignored"},
			want:     "node/pve-1/qemu/100",
		},
		{
			// Branch: targetID whitespace-only -> trimmed to "", falls through to ctx.
			name:     "whitespace_only_target_id_falls_through_to_ctx",
			targetID: "   \t\n  ",
			ctx:      map[string]interface{}{"resourceID": "from-ctx"},
			want:     "from-ctx",
		},
		{
			// Branch: empty targetID + nil ctx -> "".
			name:     "empty_target_id_nil_ctx_returns_empty",
			targetID: "",
			ctx:      nil,
			want:     "",
		},
		{
			// Branch: ctx["resourceID"] hit, value trimmed on output.
			name:     "resourceID_key_value_trimmed",
			targetID: "",
			ctx:      map[string]interface{}{"resourceID": "  vm-42  "},
			want:     "vm-42",
		},
		{
			// Branch: ctx["resourceID"] present but whitespace-only -> skipped,
			// falls through to next key in iteration order.
			name:     "resourceID_whitespace_only_skipped_falls_to_resource_id",
			targetID: "",
			ctx: map[string]interface{}{
				"resourceID":  "   ",
				"resource_id": "snake-case-wins",
			},
			want: "snake-case-wins",
		},
		{
			// Branch: ctx["resourceID"] is a non-string type -> type assertion
			// fails, skipped, falls through to "resource_id".
			name:     "resourceID_non_string_skipped_falls_to_resource_id",
			targetID: "",
			ctx: map[string]interface{}{
				"resourceID":  12345,
				"resource_id": "int-coerced-fallthrough",
			},
			want: "int-coerced-fallthrough",
		},
		{
			// Branch: only the snake_case key present.
			name:     "resource_id_key_hit",
			targetID: "",
			ctx:      map[string]interface{}{"resource_id": "  storage/pool-0  "},
			want:     "storage/pool-0",
		},
		{
			// Branch: only the camelCase resourceId key present (last in order).
			name:     "resourceId_key_hit",
			targetID: "",
			ctx:      map[string]interface{}{"resourceId": "ct-7"},
			want:     "ct-7",
		},
		{
			// Branch: precedence — resourceID wins over resource_id and resourceId.
			name:     "resourceID_precedence_over_resource_id_and_resourceId",
			targetID: "",
			ctx: map[string]interface{}{
				"resourceID":  "first",
				"resource_id": "second",
				"resourceId":  "third",
			},
			want: "first",
		},
		{
			// Branch: resource_id wins over resourceId.
			name:     "resource_id_precedence_over_resourceId",
			targetID: "",
			ctx: map[string]interface{}{
				"resource_id": "second",
				"resourceId":  "third",
			},
			want: "second",
		},
		{
			// Branch: none of the known keys present -> "".
			name:     "no_known_keys_returns_empty",
			targetID: "",
			ctx:      map[string]interface{}{"unrelated": "x", "count": 3},
			want:     "",
		},
		{
			// Branch: all known keys present but empty/whitespace -> "".
			name:     "all_known_keys_empty_returns_empty",
			targetID: "",
			ctx: map[string]interface{}{
				"resourceID":  "",
				"resource_id": "  ",
				"resourceId":  "",
			},
			want: "",
		},
		{
			// Branch: empty targetID + empty (non-nil) ctx -> "".
			name:     "empty_target_id_empty_ctx_returns_empty",
			targetID: "",
			ctx:      map[string]interface{}{},
			want:     "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractIncidentResourceIdentifier(tc.targetID, tc.ctx)
			if got != tc.want {
				t.Fatalf("extractIncidentResourceIdentifier(%q, ctx) = %q, want %q",
					tc.targetID, got, tc.want)
			}
		})
	}
}

// TestBranchCovAppendSMARTInt64Issue exercises every reachable branch of
// appendSMARTInt64Issue (patrol_ai.go) and the value filter it delegates to
// (appendSMARTInt64ValueIssue): the nil-pointer early return, the delegated
// value<=0 drop for zero and negative, the positive-value append with exact
// "label=%d" formatting, accumulation/ordering, and appending into a nil slice.
func TestBranchCovAppendSMARTInt64Issue(t *testing.T) {
	int64ptr := func(v int64) *int64 { return &v }

	// Branch: value == nil -> early return, parts unchanged.
	t.Run("nil_value_is_noop", func(t *testing.T) {
		parts := []string{"existing"}
		appendSMARTInt64Issue(&parts, "reallocated sectors", nil)
		if len(parts) != 1 || parts[0] != "existing" {
			t.Fatalf("nil value mutated parts; got %v", parts)
		}
	})

	// Branch: value points to 0 -> delegated filter drops it.
	t.Run("zero_value_is_noop", func(t *testing.T) {
		parts := []string{}
		appendSMARTInt64Issue(&parts, "pending sectors", int64ptr(0))
		if len(parts) != 0 {
			t.Fatalf("zero value should not append; got %v", parts)
		}
	})

	// Branch: value points to negative -> delegated filter drops it.
	t.Run("negative_value_is_noop", func(t *testing.T) {
		parts := []string{}
		appendSMARTInt64Issue(&parts, "offline uncorrectable", int64ptr(-5))
		if len(parts) != 0 {
			t.Fatalf("negative value should not append; got %v", parts)
		}
	})

	// Branch: value points to positive -> appends "label=%d".
	t.Run("positive_value_appends", func(t *testing.T) {
		parts := []string{}
		appendSMARTInt64Issue(&parts, "UDMA CRC errors", int64ptr(7))
		if len(parts) != 1 || parts[0] != "UDMA CRC errors=7" {
			t.Fatalf("positive append: got %v", parts)
		}
	})

	// Branch: repeated calls accumulate in call order, with nil/zero skipped.
	t.Run("accumulates_in_order_skipping_nil_and_zero", func(t *testing.T) {
		parts := []string{}
		appendSMARTInt64Issue(&parts, "reallocated sectors", int64ptr(2))
		appendSMARTInt64Issue(&parts, "media errors", int64ptr(9))
		appendSMARTInt64Issue(&parts, "pending sectors", nil)               // skipped (nil)
		appendSMARTInt64Issue(&parts, "offline uncorrectable", int64ptr(0)) // skipped (<=0)
		want := []string{"reallocated sectors=2", "media errors=9"}
		if len(parts) != len(want) {
			t.Fatalf("accumulate length: got %v want %v", parts, want)
		}
		for i := range want {
			if parts[i] != want[i] {
				t.Fatalf("accumulate[%d]: got %q want %q (full: %v)", i, parts[i], want[i], parts)
			}
		}
	})

	// Branch: appending into a nil []string (parts points to nil) exercises
	// append's grow-from-nil path through the helper.
	t.Run("appends_into_nil_slice", func(t *testing.T) {
		var parts []string
		appendSMARTInt64Issue(&parts, "media errors", int64ptr(3))
		if len(parts) != 1 || parts[0] != "media errors=3" {
			t.Fatalf("nil-slice append: got %v", parts)
		}
	})

	// Formatting: a large positive int64 renders with %d (no overflow/truncation).
	t.Run("large_value_formatted_as_decimal", func(t *testing.T) {
		parts := []string{}
		const big int64 = 1<<62 + 1
		appendSMARTInt64Issue(&parts, "media errors", int64ptr(big))
		want := "media errors=" + strconv.FormatInt(big, 10)
		if len(parts) != 1 || parts[0] != want {
			t.Fatalf("large value formatting: got %v want [%q]", parts, want)
		}
	})
}
