package alerts

import (
	"testing"
)

// TestBranchcov0724pmCanonicalAckKey raises branch coverage of canonicalAckKey
// (ack_state.go:5, baseline 80%). The existing suite reaches the canonical-
// state-present arm and the fallback-with-identity-absent arm, but never the
// nil-alert early return. These cases drive the nil guard and re-assert the
// other two arms with concrete, distinct expected values.
func TestBranchcov0724pmCanonicalAckKey(t *testing.T) {
	cases := []struct {
		name     string
		alert    *Alert
		fallback string
		want     string
	}{
		{
			// UNCOVERED arm: nil alert -> fallback returned verbatim.
			name:     "nil_alert_returns_fallback",
			alert:    nil,
			fallback: "public-id-1",
			want:     "public-id-1",
		},
		{
			// Canonical state present on the alert wins over the fallback.
			name:     "canonical_state_wins_over_fallback",
			alert:    &Alert{CanonicalState: "r1::s1"},
			fallback: "should-not-be-used",
			want:     "r1::s1",
		},
		{
			// No canonical identity resolvable -> fallback.
			name:     "no_identity_returns_fallback",
			alert:    &Alert{ID: "legacy-only"},
			fallback: "legacy-only",
			want:     "legacy-only",
		},
		{
			// No identity and empty fallback -> empty string.
			name:     "no_identity_empty_fallback_returns_empty",
			alert:    &Alert{},
			fallback: "",
			want:     "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalAckKey(tc.alert, tc.fallback); got != tc.want {
				t.Fatalf("canonicalAckKey want %q, got %q", tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmAlertMetadataResourceType raises branch coverage of
// alertMetadataResourceType (active_cleanup.go:219, baseline 80%). The
// existing suite covers the nil-alert/nil-metadata guard and the
// resourceType-present-as-string arm. The UNCOVERED arm is the final
// `return ""` when the key is absent or holds a non-string value. These cases
// drive that arm and re-assert the trim/lowercase behaviour of the string arm.
func TestBranchcov0724pmAlertMetadataResourceType(t *testing.T) {
	cases := []struct {
		name  string
		alert *Alert
		want  string
	}{
		{
			// Nil alert guard -> "".
			name:  "nil_alert_returns_empty",
			alert: nil,
			want:  "",
		},
		{
			// Nil metadata guard -> "".
			name:  "nil_metadata_returns_empty",
			alert: &Alert{Metadata: nil},
			want:  "",
		},
		{
			// UNCOVERED arm: metadata present but key absent -> "".
			name:  "missing_resource_type_key_returns_empty",
			alert: &Alert{Metadata: map[string]interface{}{"other": 1}},
			want:  "",
		},
		{
			// UNCOVERED arm (same return): key present but not a string -> "".
			name:  "non_string_resource_type_returns_empty",
			alert: &Alert{Metadata: map[string]interface{}{"resourceType": 42}},
			want:  "",
		},
		{
			// String value is trimmed and lower-cased.
			name:  "string_value_trimmed_and_lowercased",
			alert: &Alert{Metadata: map[string]interface{}{"resourceType": "  Docker  "}},
			want:  "docker",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := alertMetadataResourceType(tc.alert); got != tc.want {
				t.Fatalf("alertMetadataResourceType want %q, got %q", tc.want, got)
			}
		})
	}
}
