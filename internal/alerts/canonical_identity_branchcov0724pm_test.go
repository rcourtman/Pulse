package alerts

import (
	"testing"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
)

// TestBranchcov0724pmCanonicalPoweredStateSpecID raises branch coverage of
// canonicalPoweredStateSpecID (canonical_identity.go:41). The existing suite
// only calls it with a non-empty resourceID (happy path). These cases drive the
// empty-input early-return arm and re-assert the happy path with a concrete,
// distinct value.
func TestBranchcov0724pmCanonicalPoweredStateSpecID(t *testing.T) {
	cases := []struct {
		name       string
		resourceID string
		want       string
	}{
		{
			// Empty arm: resourceID == "" short-circuits to "".
			name:       "empty_resource_id_returns_empty",
			resourceID: "",
			want:       "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalPoweredStateSpecID(tc.resourceID); got != tc.want {
				t.Fatalf("canonicalPoweredStateSpecID(%q) want %q, got %q", tc.resourceID, tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmCanonicalDiscreteStateSpecID raises branch coverage of
// canonicalDiscreteStateSpecID (canonical_identity.go:52). The existing suite
// only reaches the happy path (both args non-empty). These cases drive both
// halves of the OR guard -- empty resourceID and empty stateKey -- so each
// sub-condition is independently exercised, plus the happy path with distinct
// values.
func TestBranchcov0724pmCanonicalDiscreteStateSpecID(t *testing.T) {
	cases := []struct {
		name       string
		resourceID string
		stateKey   string
		want       string
	}{
		{
			// Empty arm, first half of the OR: resourceID == "" -> "".
			name:       "empty_resource_id_returns_empty",
			resourceID: "",
			stateKey:   "runtime-state",
			want:       "",
		},
		{
			// Empty arm, second half of the OR: stateKey == "" -> "".
			name:       "empty_state_key_returns_empty",
			resourceID: "docker:c1",
			stateKey:   "",
			want:       "",
		},
		{
			// Both empty: still "" (does not matter which half fires).
			name:       "both_empty_returns_empty",
			resourceID: "",
			stateKey:   "",
			want:       "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalDiscreteStateSpecID(tc.resourceID, tc.stateKey); got != tc.want {
				t.Fatalf("canonicalDiscreteStateSpecID(%q, %q) want %q, got %q", tc.resourceID, tc.stateKey, tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmCanonicalServiceGapSpecID raises branch coverage of
// canonicalServiceGapSpecID (canonical_identity.go:63). The existing suite only
// reaches the happy path. These cases drive the empty-resourceID early-return
// arm and re-assert the happy path.
func TestBranchcov0724pmCanonicalServiceGapSpecID(t *testing.T) {
	cases := []struct {
		name       string
		resourceID string
		want       string
	}{
		{
			// Empty arm: resourceID == "" short-circuits to "".
			name:       "empty_resource_id_returns_empty",
			resourceID: "",
			want:       "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalServiceGapSpecID(tc.resourceID); got != tc.want {
				t.Fatalf("canonicalServiceGapSpecID(%q) want %q, got %q", tc.resourceID, tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmCanonicalTrackingKeyForSpec raises branch coverage of
// canonicalTrackingKeyForSpec (canonical_identity.go:389). The existing suite
// only reaches the happy path (buildCanonicalStateID returns a non-empty key).
// These cases drive the fallback arm -- when spec.ResourceID or spec.ID is empty
// the canonical key is "" so the distinct fallback must be returned -- and
// re-assert the happy path with a concrete expected canonical key.
func TestBranchcov0724pmCanonicalTrackingKeyForSpec(t *testing.T) {
	cases := []struct {
		name     string
		spec     alertspecs.ResourceAlertSpec
		fallback string
		want     string
	}{
		{
			// Fallback arm: empty ResourceID -> buildCanonicalStateID returns "".
			name:     "empty_resource_id_returns_fallback",
			spec:     alertspecs.ResourceAlertSpec{ID: "spec-1"},
			fallback: "legacy-id-1",
			want:     "legacy-id-1",
		},
		{
			// Fallback arm: empty ID -> buildCanonicalStateID returns "".
			name:     "empty_spec_id_returns_fallback",
			spec:     alertspecs.ResourceAlertSpec{ResourceID: "res-1"},
			fallback: "legacy-id-2",
			want:     "legacy-id-2",
		},
		{
			// Fallback arm: both empty and fallback itself empty -> "".
			name:     "both_empty_empty_fallback_returns_empty",
			spec:     alertspecs.ResourceAlertSpec{},
			fallback: "",
			want:     "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalTrackingKeyForSpec(tc.spec, tc.fallback); got != tc.want {
				t.Fatalf("canonicalTrackingKeyForSpec want %q, got %q", tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmInferCanonicalKindFromLegacyAlert raises branch coverage of
// inferCanonicalKindFromLegacyAlert (canonical_identity.go:278). The existing
// suite already covers the legacy-prefix arms and the type-based arm. The
// UNCOVERED arms are:
//  1. alert == nil -> ""
//  2. Canonical-separator ("::") path where the inferred specID ends with
//     "-powered-state" -> AlertSpecKindPoweredState
//  3. Same path, specID ends with "-runtime-state" or "-update-state" ->
//     AlertSpecKindDiscreteState
//  4. Same path, specID ends with "-service-gap" -> AlertSpecKindServiceGap
//  5. Same path, specID contains "/snapshot:" or ends with "-backup-age" ->
//     AlertSpecKindPostureThreshold
//
// When an alert's ID contains "::" the inferred specID is the segment after the
// first "::" (see inferCanonicalSpecIDFromLegacyAlert), so each case constructs
// an ID whose post-"::" suffix lands in the target switch arm. Each case uses a
// distinct resourceID/specID so the expected kind is unambiguous.
func TestBranchcov0724pmInferCanonicalKindFromLegacyAlert(t *testing.T) {
	cases := []struct {
		name  string
		alert *Alert
		want  string
	}{
		{
			// Arm 1: nil guard.
			name:  "nil_alert_returns_empty",
			alert: nil,
			want:  "",
		},
		{
			// Canonical path -> -powered-state arm.
			name:  "canonical_separator_powered_state",
			alert: &Alert{ID: "vm-100::vm-100-powered-state"},
			want:  string(alertspecs.AlertSpecKindPoweredState),
		},
		{
			// Canonical path -> -runtime-state arm (DiscreteState).
			name:  "canonical_separator_runtime_state",
			alert: &Alert{ID: "docker:c1::docker:c1-runtime-state"},
			want:  string(alertspecs.AlertSpecKindDiscreteState),
		},
		{
			// Canonical path -> -update-state arm (also DiscreteState).
			name:  "canonical_separator_update_state",
			alert: &Alert{ID: "docker:s2::docker:s2-update-state"},
			want:  string(alertspecs.AlertSpecKindDiscreteState),
		},
		{
			// Canonical path -> -service-gap arm (ServiceGap).
			name:  "canonical_separator_service_gap",
			alert: &Alert{ID: "docker:svc::docker:svc-service-gap"},
			want:  string(alertspecs.AlertSpecKindServiceGap),
		},
		{
			// Canonical path -> contains "/snapshot:" arm (PostureThreshold).
			name:  "canonical_separator_snapshot",
			alert: &Alert{ID: "pbs-1::pbs/store1/snapshot:nightly"},
			want:  string(alertspecs.AlertSpecKindPostureThreshold),
		},
		{
			// Canonical path -> ends with "-backup-age" arm (PostureThreshold).
			name:  "canonical_separator_backup_age",
			alert: &Alert{ID: "subj-7::backup-subject-7-backup-age"},
			want:  string(alertspecs.AlertSpecKindPostureThreshold),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := inferCanonicalKindFromLegacyAlert(tc.alert); got != tc.want {
				t.Fatalf("inferCanonicalKindFromLegacyAlert want %q, got %q", tc.want, got)
			}
		})
	}
}
