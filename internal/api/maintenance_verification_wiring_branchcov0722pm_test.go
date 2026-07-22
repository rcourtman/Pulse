package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/maintenancesentinel"
)

// This file raises branch/function coverage for the pure helpers in
// maintenance_verification_wiring.go and maintenance_verification.go.
// It targets ONLY:
//   - alertMatchesResource
//   - mapAlertLevel
//   - mapFindingSeverity
//   - canonicalToSourceID
//   - maintenanceVerificationOrgContext
//   - extractMaintenanceVerificationReportID
//
// Conventions mirror maintenance_verification_test.go (table-driven
// subtests, package-internal `package api`, t.Fatalf/t.Errorf idioms).

func TestBranchcov0722PM_AlertMatchesResource(t *testing.T) {
	// alertMatchesResource tries CanonicalState, then CanonicalSpecID,
	// then the legacy ResourceID, canonicalizing each (which only trims
	// whitespace today) before comparing to the supplied canonicalID.
	cases := []struct {
		name        string
		alert       alerts.Alert
		canonicalID string
		want        bool
	}{
		{
			name:        "matches_via_canonical_state_with_whitespace_trimmed",
			alert:       alerts.Alert{CanonicalState: "  vm:101  "},
			canonicalID: "vm:101",
			want:        true,
		},
		{
			// CanonicalState is present but does not match; the
			// CanonicalSpecID arm must catch it.
			name:        "matches_via_canonical_spec_id_when_state_mismatch",
			alert:       alerts.Alert{CanonicalState: "other", CanonicalSpecID: "ct:200"},
			canonicalID: "ct:200",
			want:        true,
		},
		{
			// Neither canonical field set: fall through to legacy
			// ResourceID.
			name:        "matches_via_legacy_resource_id_fallback",
			alert:       alerts.Alert{ResourceID: "node:pve"},
			canonicalID: "node:pve",
			want:        true,
		},
		{
			name:        "no_match_when_all_identity_fields_differ",
			alert:       alerts.Alert{CanonicalState: "a", CanonicalSpecID: "b", ResourceID: "c"},
			canonicalID: "vm:101",
			want:        false,
		},
		{
			name:        "no_match_when_alert_identity_empty_but_id_present",
			alert:       alerts.Alert{},
			canonicalID: "vm:101",
			want:        false,
		},
		{
			// Documented edge: empty canonicalID against an alert with
			// no identity canonicalizes all fields to "" which equals
			// the empty target. Callers in production always guard the
			// canonicalID to non-empty before invoking this helper, so
			// the empty==empty collision is not reachable in practice.
			name:        "empty_target_matches_empty_alert_identity",
			alert:       alerts.Alert{},
			canonicalID: "",
			want:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := alertMatchesResource(tc.alert, tc.canonicalID)
			if got != tc.want {
				t.Fatalf("alertMatchesResource(%+v, %q) = %v, want %v", tc.alert, tc.canonicalID, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PM_MapAlertLevel(t *testing.T) {
	// Exhaustive over the declared AlertLevel constants plus an
	// unknown value (default arm -> "").
	cases := []struct {
		name  string
		level alerts.AlertLevel
		want  maintenancesentinel.Severity
	}{
		{name: "critical", level: alerts.AlertLevelCritical, want: maintenancesentinel.SeverityCritical},
		{name: "warning", level: alerts.AlertLevelWarning, want: maintenancesentinel.SeverityWarning},
		{name: "unknown_falls_back_to_empty", level: alerts.AlertLevel("info"), want: maintenancesentinel.Severity("")},
		{name: "zero_value_falls_back_to_empty", level: alerts.AlertLevel(""), want: maintenancesentinel.Severity("")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapAlertLevel(tc.level)
			if got != tc.want {
				t.Fatalf("mapAlertLevel(%q) = %q, want %q", tc.level, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PM_MapFindingSeverity(t *testing.T) {
	// Exhaustive over every declared FindingSeverity constant plus an
	// unknown value. Only Critical and Warning map; everything else
	// falls through to the default "".
	cases := []struct {
		name     string
		severity ai.FindingSeverity
		want     maintenancesentinel.Severity
	}{
		{name: "critical", severity: ai.FindingSeverityCritical, want: maintenancesentinel.SeverityCritical},
		{name: "warning", severity: ai.FindingSeverityWarning, want: maintenancesentinel.SeverityWarning},
		{name: "info_defaults_to_empty", severity: ai.FindingSeverityInfo, want: maintenancesentinel.Severity("")},
		{name: "watch_defaults_to_empty", severity: ai.FindingSeverityWatch, want: maintenancesentinel.Severity("")},
		{name: "unknown_defaults_to_empty", severity: ai.FindingSeverity("bogus"), want: maintenancesentinel.Severity("")},
		{name: "zero_value_defaults_to_empty", severity: ai.FindingSeverity(""), want: maintenancesentinel.Severity("")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapFindingSeverity(tc.severity)
			if got != tc.want {
				t.Fatalf("mapFindingSeverity(%q) = %q, want %q", tc.severity, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PM_CanonicalToSourceID(t *testing.T) {
	// canonicalToSourceID maps a `kind:id` canonical id into the
	// metrics-history source-id form. Unknown kinds and inputs without
	// a colon yield "".
	cases := []struct {
		name        string
		canonicalID string
		want        string
	}{
		{name: "vm_to_qemu", canonicalID: "vm:101", want: "qemu/101"},
		{name: "ct_to_lxc", canonicalID: "ct:200", want: "lxc/200"},
		{name: "node_passes_through", canonicalID: "node:pve", want: "node/pve"},
		{name: "unknown_kind_returns_empty", canonicalID: "storage:local", want: ""},
		{name: "no_colon_returns_empty", canonicalID: "bareid", want: ""},
		{name: "empty_returns_empty", canonicalID: "", want: ""},
		{
			// SplitN(_, ":", 2) keeps everything after the first colon
			// as the id, so an id that itself contains a colon is
			// preserved verbatim in the source form.
			name:        "extra_colons_kept_in_id",
			canonicalID: "vm:101:extra",
			want:        "qemu/101:extra",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalToSourceID(tc.canonicalID)
			if got != tc.want {
				t.Fatalf("canonicalToSourceID(%q) = %q, want %q", tc.canonicalID, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PM_MaintenanceVerificationOrgContext(t *testing.T) {
	// maintenanceVerificationOrgContext stashes the org id under
	// OrgIDContextKey. The production reader GetOrgID reads that exact
	// key (defaulting to "default" when absent), so round-tripping
	// through GetOrgID proves the value is retrievable with the key
	// production code uses.
	cases := []struct {
		name  string
		orgID string
		want  string
	}{
		{name: "explicit_org_is_preserved", orgID: "tenant-a", want: "tenant-a"},
		{name: "empty_org_defaults", orgID: "", want: "default"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := maintenanceVerificationOrgContext(tc.orgID)
			if got := GetOrgID(ctx); got != tc.want {
				t.Fatalf("GetOrgID(maintenanceVerificationOrgContext(%q)) = %q, want %q", tc.orgID, got, tc.want)
			}
			// Belt-and-braces: confirm the value is a string stored
			// under OrgIDContextKey itself (not via some other key).
			if v, ok := ctx.Value(OrgIDContextKey).(string); !ok || v != tc.want {
				t.Fatalf("ctx.Value(OrgIDContextKey) = %q (ok=%v), want %q", v, ok, tc.want)
			}
		})
	}
}

func TestBranchcov0722PM_ExtractMaintenanceVerificationReportID(t *testing.T) {
	// extractMaintenanceVerificationReportID trims the route prefix and
	// a single trailing slash. It does NOT strip nested segments
	// (/review) or query strings, and it does NOT validate the prefix.
	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "simple_id", path: "/api/maintenance-verifications/rpt-123", want: "rpt-123"},
		{name: "trailing_slash_stripped", path: "/api/maintenance-verifications/rpt-123/", want: "rpt-123"},
		{
			// Nested path: the trailing-slash trim does not apply, and
			// /review is NOT stripped by this helper (that is the job
			// of extractMaintenanceVerificationReviewReportID), so the
			// full nested tail is returned.
			name: "nested_path_kept_verbatim",
			path: "/api/maintenance-verifications/rpt-123/review",
			want: "rpt-123/review",
		},
		{name: "empty_path", path: "", want: ""},
		{
			name: "query_like_junk_preserved",
			path: "/api/maintenance-verifications/rpt-123?foo=bar",
			want: "rpt-123?foo=bar",
		},
		{name: "surrounding_whitespace_trimmed", path: "/api/maintenance-verifications/  rpt-456  ", want: "rpt-456"},
		{
			// No matching prefix: TrimPrefix is a no-op, so the whole
			// input is returned (minus any trailing slash / whitespace).
			name: "non_matching_prefix_returned_as_is",
			path: "/api/resources/vm:101",
			want: "/api/resources/vm:101",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractMaintenanceVerificationReportID(tc.path)
			if got != tc.want {
				t.Fatalf("extractMaintenanceVerificationReportID(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
