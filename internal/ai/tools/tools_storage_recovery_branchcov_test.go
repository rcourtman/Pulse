package tools

import (
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// These tests raise branch coverage for the small pure helper functions in
// tools_storage.go that canonicalize recovery-point fields. They target the
// previously uncovered branches: nil Details/Display/SubjectRef fallbacks,
// the int/float64/json.Number arms of recoveryPointDetailInt, the Outcome
// switch arms in the status/error mappers, and the SubjectRef.Name/ID parse
// fallbacks in recoveryPointCanonicalVMID.

func TestRecoveryPointDetailString_BranchCov(t *testing.T) {
	tests := []struct {
		name  string
		point recovery.RecoveryPoint
		key   string
		want  string
	}{
		{
			name:  "nil_details_returns_empty",
			point: recovery.RecoveryPoint{},
			key:   "type",
			want:  "",
		},
		{
			name:  "missing_key_returns_empty",
			point: recovery.RecoveryPoint{Details: map[string]any{}},
			key:   "type",
			want:  "",
		},
		{
			name:  "nil_value_returns_empty",
			point: recovery.RecoveryPoint{Details: map[string]any{"type": nil}},
			key:   "type",
			want:  "",
		},
		{
			name:  "string_value_is_trimmed",
			point: recovery.RecoveryPoint{Details: map[string]any{"type": "  vm  "}},
			key:   "type",
			want:  "vm",
		},
		{
			name:  "non_string_value_returns_empty",
			point: recovery.RecoveryPoint{Details: map[string]any{"type": 42}},
			key:   "type",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := recoveryPointDetailString(tc.point, tc.key); got != tc.want {
				t.Fatalf("recoveryPointDetailString(%+v, %q) = %q, want %q", tc.point, tc.key, got, tc.want)
			}
		})
	}
}

func TestRecoveryPointDetailInt_BranchCov(t *testing.T) {
	tests := []struct {
		name  string
		point recovery.RecoveryPoint
		key   string
		want  int
	}{
		{
			name:  "nil_details_returns_zero",
			point: recovery.RecoveryPoint{},
			key:   "vmid",
			want:  0,
		},
		{
			name:  "missing_key_returns_zero",
			point: recovery.RecoveryPoint{Details: map[string]any{}},
			key:   "vmid",
			want:  0,
		},
		{
			name:  "nil_value_returns_zero",
			point: recovery.RecoveryPoint{Details: map[string]any{"vmid": nil}},
			key:   "vmid",
			want:  0,
		},
		{
			name:  "int_value_returned",
			point: recovery.RecoveryPoint{Details: map[string]any{"vmid": 101}},
			key:   "vmid",
			want:  101,
		},
		{
			name:  "int64_value_returned",
			point: recovery.RecoveryPoint{Details: map[string]any{"vmid": int64(202)}},
			key:   "vmid",
			want:  202,
		},
		{
			name:  "float64_value_truncated",
			point: recovery.RecoveryPoint{Details: map[string]any{"vmid": float64(303.99)}},
			key:   "vmid",
			want:  303,
		},
		{
			name:  "json_number_value_returned",
			point: recovery.RecoveryPoint{Details: map[string]any{"vmid": json.Number("404")}},
			key:   "vmid",
			want:  404,
		},
		{
			name:  "bool_value_falls_to_default_zero",
			point: recovery.RecoveryPoint{Details: map[string]any{"vmid": true}},
			key:   "vmid",
			want:  0,
		},
		{
			name:  "string_value_falls_to_default_zero",
			point: recovery.RecoveryPoint{Details: map[string]any{"vmid": "not-a-number"}},
			key:   "vmid",
			want:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := recoveryPointDetailInt(tc.point, tc.key); got != tc.want {
				t.Fatalf("recoveryPointDetailInt(%+v, %q) = %d, want %d", tc.point, tc.key, got, tc.want)
			}
		})
	}
}

func TestRecoveryPointCanonicalVMID_BranchCov(t *testing.T) {
	tests := []struct {
		name  string
		point recovery.RecoveryPoint
		want  int
	}{
		{
			name: "display_entity_id_label_used_when_valid",
			point: recovery.RecoveryPoint{
				Display: &recovery.RecoveryPointDisplay{EntityIDLabel: "100"},
			},
			want: 100,
		},
		{
			// parseRecoveryPointVMID rejects negative numbers, so even with a
			// non-empty EntityIDLabel we must fall through to SubjectRef.Name.
			name: "display_entity_id_label_negative_skips_to_subject_ref_name",
			point: recovery.RecoveryPoint{
				Display:    &recovery.RecoveryPointDisplay{EntityIDLabel: "-1"},
				SubjectRef: &recovery.ExternalRef{Name: "200"},
			},
			want: 200,
		},
		{
			// EntityIDLabel unparseable AND SubjectRef.Name parses: Name wins.
			name: "display_unparseable_falls_to_subject_ref_name",
			point: recovery.RecoveryPoint{
				Display:    &recovery.RecoveryPointDisplay{EntityIDLabel: "abc"},
				SubjectRef: &recovery.ExternalRef{Name: "300"},
			},
			want: 300,
		},
		{
			// SubjectRef.Name zero is rejected; SubjectRef.ID is the fallback.
			name: "subject_ref_name_zero_falls_to_subject_ref_id",
			point: recovery.RecoveryPoint{
				SubjectRef: &recovery.ExternalRef{Name: "0", ID: "400"},
			},
			want: 400,
		},
		{
			name: "subject_ref_name_unparseable_falls_to_subject_ref_id",
			point: recovery.RecoveryPoint{
				SubjectRef: &recovery.ExternalRef{Name: "node-1", ID: "500"},
			},
			want: 500,
		},
		{
			// No Display, no SubjectRef -> details["vmid"] terminal fallback.
			name: "no_display_no_subject_ref_falls_to_details",
			point: recovery.RecoveryPoint{
				Details: map[string]any{"vmid": 600},
			},
			want: 600,
		},
		{
			// SubjectRef present but neither Name nor ID parses; details wins.
			name: "subject_ref_unparseable_falls_to_details",
			point: recovery.RecoveryPoint{
				SubjectRef: &recovery.ExternalRef{Name: "abc", ID: "xyz"},
				Details:    map[string]any{"vmid": 700},
			},
			want: 700,
		},
		{
			name: "all_sources_empty_returns_zero",
			point: recovery.RecoveryPoint{
				Display:    &recovery.RecoveryPointDisplay{},
				SubjectRef: &recovery.ExternalRef{},
			},
			want: 0,
		},
		{
			name:  "no_sources_at_all_returns_zero",
			point: recovery.RecoveryPoint{},
			want:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := recoveryPointCanonicalVMID(tc.point); got != tc.want {
				t.Fatalf("recoveryPointCanonicalVMID(%+v) = %d, want %d", tc.point, got, tc.want)
			}
		})
	}
}

func TestRecoveryPointCanonicalTaskStatus_BranchCov(t *testing.T) {
	tests := []struct {
		name  string
		point recovery.RecoveryPoint
		want  string
	}{
		{
			// details["status"] short-circuits the Outcome switch entirely.
			name: "details_status_present_overrides_outcome",
			point: recovery.RecoveryPoint{
				Details: map[string]any{"status": "PAUSED"},
				Outcome: recovery.OutcomeFailed,
			},
			want: "PAUSED",
		},
		{
			// A whitespace-only status trims to empty, so we hit the switch.
			name: "details_status_whitespace_falls_to_outcome_switch",
			point: recovery.RecoveryPoint{
				Details: map[string]any{"status": "   "},
				Outcome: recovery.OutcomeRunning,
			},
			want: "RUNNING",
		},
		{
			name:  "outcome_success_maps_to_OK",
			point: recovery.RecoveryPoint{Outcome: recovery.OutcomeSuccess},
			want:  "OK",
		},
		{
			name:  "outcome_failed_maps_to_ERROR",
			point: recovery.RecoveryPoint{Outcome: recovery.OutcomeFailed},
			want:  "ERROR",
		},
		{
			name:  "outcome_warning_maps_to_WARNING",
			point: recovery.RecoveryPoint{Outcome: recovery.OutcomeWarning},
			want:  "WARNING",
		},
		{
			name:  "outcome_running_maps_to_RUNNING",
			point: recovery.RecoveryPoint{Outcome: recovery.OutcomeRunning},
			want:  "RUNNING",
		},
		{
			name:  "unknown_outcome_value_maps_to_UNKNOWN",
			point: recovery.RecoveryPoint{Outcome: recovery.Outcome("bogus")},
			want:  "UNKNOWN",
		},
		{
			name:  "empty_outcome_maps_to_UNKNOWN",
			point: recovery.RecoveryPoint{},
			want:  "UNKNOWN",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := recoveryPointCanonicalTaskStatus(tc.point); got != tc.want {
				t.Fatalf("recoveryPointCanonicalTaskStatus(%+v) = %q, want %q", tc.point, got, tc.want)
			}
		})
	}
}

func TestRecoveryPointCanonicalTaskError_BranchCov(t *testing.T) {
	tests := []struct {
		name  string
		point recovery.RecoveryPoint
		want  string
	}{
		{
			// details["error"] short-circuits regardless of Outcome.
			name: "details_error_present_overrides_outcome",
			point: recovery.RecoveryPoint{
				Details: map[string]any{"error": "  boom  "},
				Outcome: recovery.OutcomeSuccess,
			},
			want: "boom",
		},
		{
			name: "outcome_failed_uses_display_details_summary",
			point: recovery.RecoveryPoint{
				Outcome: recovery.OutcomeFailed,
				Display: &recovery.RecoveryPointDisplay{DetailsSummary: "  snapshot failed  "},
			},
			want: "snapshot failed",
		},
		{
			name: "outcome_warning_uses_display_details_summary",
			point: recovery.RecoveryPoint{
				Outcome: recovery.OutcomeWarning,
				Display: &recovery.RecoveryPointDisplay{DetailsSummary: "  partial  "},
			},
			want: "partial",
		},
		{
			// Success/Running outcomes ignore the summary entirely.
			name: "outcome_success_returns_empty_even_with_summary",
			point: recovery.RecoveryPoint{
				Outcome: recovery.OutcomeSuccess,
				Display: &recovery.RecoveryPointDisplay{DetailsSummary: "should-not-appear"},
			},
			want: "",
		},
		{
			name: "outcome_running_returns_empty_even_with_summary",
			point: recovery.RecoveryPoint{
				Outcome: recovery.OutcomeRunning,
				Display: &recovery.RecoveryPointDisplay{DetailsSummary: "should-not-appear"},
			},
			want: "",
		},
		{
			// nil Display hits the recoveryPointDisplayString guard.
			name:  "outcome_failed_with_nil_display_returns_empty",
			point: recovery.RecoveryPoint{Outcome: recovery.OutcomeFailed},
			want:  "",
		},
		{
			name:  "empty_outcome_returns_empty",
			point: recovery.RecoveryPoint{},
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := recoveryPointCanonicalTaskError(tc.point); got != tc.want {
				t.Fatalf("recoveryPointCanonicalTaskError(%+v) = %q, want %q", tc.point, got, tc.want)
			}
		})
	}
}
