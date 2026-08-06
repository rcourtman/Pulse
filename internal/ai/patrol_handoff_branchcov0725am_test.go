package ai

import (
	"strings"
	"testing"
)

// This file raises statement coverage of four partially-covered pure helpers
// in patrol_assistant_handoff.go:
//
//   - formatPatrolRunDurationMs      (line 214)  -> pinned at every unit
//                                                  boundary + zero/negative.
//   - patrolRunRuntimeFailureSummary (line 119)  -> empty / summary-only /
//                                                  detail-only / combined /
//                                                  summary==detail / count
//                                                  arms, with truncation.
//   - patrolRunScopeSummary          (line 269)  -> empty / single / multi /
//                                                  types / scoped-only /
//                                                  Effective-vs-Scope nil.
//   - patrolRunCoverageSummary       (line 229)  -> empty / partial-of-scope /
//                                                  full scoped / unscoped /
//                                                  breakdown / nil-vs-empty.
//
// Every assertion pins the exact returned string so a behaviour change is
// caught. No existing test in this package calls these functions directly, so
// nothing here duplicates existing coverage.

// TestBranchcov0725amFormatPatrolRunDurationMs pins each unit boundary the
// formatter switches on. There is NO separate hours arm in the source: any
// duration >= 60s collapses to whole minutes, so one hour renders as "60m".
// That is exercised by the one_hour_renders_as_minutes case and called out in
// the report as a source observation (not a bug).
func TestBranchcov0725amFormatPatrolRunDurationMs(t *testing.T) {
	cases := []struct {
		name string
		ms   int64
		want string
	}{
		// ms <= 0 arm (covers both zero and the negative boundary).
		{name: "zero_returns_empty", ms: 0, want: ""},
		{name: "negative_returns_empty", ms: -1, want: ""},

		// Sub-second arm: 0 < ms < 1000.
		{name: "one_millisecond", ms: 1, want: "1ms"},
		{name: "just_below_one_second", ms: 999, want: "999ms"},

		// Seconds arm: ms rounds up to 1..59 whole seconds.
		{name: "exactly_one_second_rounds_to_1s", ms: 1000, want: "1s"},
		{name: "just_above_one_second_still_1s", ms: 1001, want: "1s"},
		{name: "last_value_rendering_seconds", ms: 59499, want: "59s"}, // (59499+500)/1000 = 59

		// Minutes arm (seconds >= 60): rounds seconds to whole minutes.
		{name: "first_value_rendering_minutes", ms: 59500, want: "1m"}, // (60000/1000=60)->(60+30)/60=1
		{name: "rounds_down_to_1m", ms: 89499, want: "1m"},             // (89+30)/60 = 1
		{name: "rounds_up_to_2m", ms: 89500, want: "2m"},               // (90+30)/60 = 2

		// Hours: no dedicated arm exists; one hour renders as "60m".
		{name: "one_hour_renders_as_minutes", ms: 3600000, want: "60m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatPatrolRunDurationMs(tc.ms)
			if got != tc.want {
				t.Fatalf("formatPatrolRunDurationMs(%d) = %q, want %q", tc.ms, got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amPatrolRunRuntimeFailureSummary drives every return arm
// of patrolRunRuntimeFailureSummary with inputs that survive
// redaction/summarization deterministically, asserting the exact string.
func TestBranchcov0725amPatrolRunRuntimeFailureSummary(t *testing.T) {
	longDetail := strings.Repeat("a", 300) // falls through summarize to default redaction (no keyword match, no secrets)

	cases := []struct {
		name string
		run  PatrolRunRecord
		want string
	}{
		{
			// Empty / no-data arm: nothing to report.
			name: "no_errors_returns_empty",
			run:  PatrolRunRecord{},
			want: "",
		},
		{
			// Combined arm: distinct non-empty summary and detail ->
			// "summary: detail".
			name: "summary_and_distinct_detail_joined",
			run: PatrolRunRecord{
				ErrorSummary: "Patrol run hit a snag",
				ErrorDetail:  "rate limit exceeded",
			},
			want: "Patrol run hit a snag: Provider rate limit reached. Wait for capacity or adjust provider limits before retrying.",
		},
		{
			// summary == detail arm: when summary and the summarized detail
			// are identical, the joined form is suppressed and the bare
			// summary is returned (no redundant duplication).
			name: "summary_equals_detail_returns_bare_summary",
			run: PatrolRunRecord{
				ErrorSummary: "upstream returned unexpected eof",
				ErrorDetail:  "upstream returned unexpected eof",
			},
			want: "upstream returned unexpected eof",
		},
		{
			// Summary-only arm: detail empty -> return summary verbatim.
			name: "summary_only_no_detail",
			run: PatrolRunRecord{
				ErrorSummary: "Patrol hit a snag",
			},
			want: "Patrol hit a snag",
		},
		{
			// Detail-only arm: summary empty -> return summarized detail.
			name: "detail_only_no_summary",
			run: PatrolRunRecord{
				ErrorDetail: "rate limit exceeded",
			},
			want: "Provider rate limit reached. Wait for capacity or adjust provider limits before retrying.",
		},
		{
			// Combined arm with detail longer than the 260-char cap ->
			// detail is truncated to "<257 chars>...".
			name: "combined_detail_truncated_at_260",
			run: PatrolRunRecord{
				ErrorSummary: "Boom",
				ErrorDetail:  longDetail,
			},
			want: "Boom: " + longDetail[:257] + "...",
		},
		{
			// ErrorCount arm, singular: no summary/detail, count == 1.
			name: "error_count_one_singular",
			run: PatrolRunRecord{
				ErrorCount: 1,
			},
			want: "1 Patrol runtime error recorded",
		},
		{
			// ErrorCount arm, plural: no summary/detail, count != 1.
			name: "error_count_many_plural",
			run: PatrolRunRecord{
				ErrorCount: 3,
			},
			want: "3 Patrol runtime errors recorded",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := patrolRunRuntimeFailureSummary(tc.run)
			if got != tc.want {
				t.Fatalf("patrolRunRuntimeFailureSummary = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amPatrolRunScopeSummary drives the empty, single-item,
// multi-item, types-only, scoped-only, and Effective-vs-Scope precedence arms
// of patrolRunScopeSummary.
func TestBranchcov0725amPatrolRunScopeSummary(t *testing.T) {
	cases := []struct {
		name string
		run  PatrolRunRecord
		want string
	}{
		{
			// Empty / no-data arm.
			name: "no_scope_returns_empty",
			run:  PatrolRunRecord{},
			want: "",
		},
		{
			// Single-item effective scope (suffix "").
			name: "single_effective_resource",
			run: PatrolRunRecord{
				EffectiveScopeResourceIDs: []string{"vm-1"},
			},
			want: "Scoped to 1 resource",
		},
		{
			// Multi-item effective scope (suffix "s").
			name: "multiple_effective_resources",
			run: PatrolRunRecord{
				EffectiveScopeResourceIDs: []string{"vm-1", "vm-2", "vm-3"},
			},
			want: "Scoped to 3 resources",
		},
		{
			// Effective nil -> falls back to ScopeResourceIDs (single).
			name: "effective_nil_falls_back_to_scope_single",
			run: PatrolRunRecord{
				ScopeResourceIDs: []string{"node-1"},
			},
			want: "Scoped to 1 resource",
		},
		{
			// Effective non-nil empty slice takes precedence over a populated
			// ScopeResourceIDs: scopedResourceCount becomes 0 so the types arm
			// is consulted instead. Pins the nil-vs-empty distinction.
			name: "effective_empty_overrides_populated_scope",
			run: PatrolRunRecord{
				EffectiveScopeResourceIDs: []string{},
				ScopeResourceIDs:          []string{"a", "b"},
				ScopeResourceTypes:        []string{"vm", "node"},
			},
			want: "Scoped to vm, node",
		},
		{
			// Types-only arm, single type.
			name: "types_only_single",
			run: PatrolRunRecord{
				ScopeResourceTypes: []string{"pbs"},
			},
			want: "Scoped to pbs",
		},
		{
			// Types-only arm, multi type.
			name: "types_only_multi",
			run: PatrolRunRecord{
				ScopeResourceTypes: []string{"vm", "node"},
			},
			want: "Scoped to vm, node",
		},
		{
			// ids take precedence over types when both are set.
			name: "ids_take_precedence_over_types",
			run: PatrolRunRecord{
				EffectiveScopeResourceIDs: []string{"vm-1"},
				ScopeResourceTypes:        []string{"vm"},
			},
			want: "Scoped to 1 resource",
		},
		{
			// Scoped-only arm: Type == "scoped", no ids/types.
			name: "type_scoped_label",
			run: PatrolRunRecord{
				Type: "scoped",
			},
			want: "Scoped",
		},
		{
			// EqualFold: Type is matched case-insensitively after trim.
			name: "type_scoped_case_insensitive",
			run: PatrolRunRecord{
				Type: "  SCOPED ",
			},
			want: "Scoped",
		},
		{
			// Non-scoped type with no ids/types -> empty.
			name: "non_scoped_type_returns_empty",
			run: PatrolRunRecord{
				Type: "patrol",
			},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := patrolRunScopeSummary(tc.run)
			if got != tc.want {
				t.Fatalf("patrolRunScopeSummary = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amPatrolRunCoverageSummary drives the empty, partial-of-
// scope, full-scoped (singular + plural), unscoped (singular + plural), and
// breakdown arms of patrolRunCoverageSummary, including the subtle
// nil-vs-empty scope distinction and the scoped-but-zero-checked fall-through.
func TestBranchcov0725amPatrolRunCoverageSummary(t *testing.T) {
	cases := []struct {
		name string
		run  PatrolRunRecord
		want string
	}{
		{
			// Empty / no-data arm: nothing checked, no scope, no breakdown.
			name: "nothing_checked_returns_empty",
			run:  PatrolRunRecord{},
			want: "",
		},
		{
			// Partial-of-scope arm: resourcesChecked > 0 and < scope count.
			name: "partial_of_scope",
			run: PatrolRunRecord{
				EffectiveScopeResourceIDs: []string{"a", "b", "c"},
				ResourcesChecked:          1,
			},
			want: "Checked 1 of 3 scoped resources",
		},
		{
			// Full scoped arm, singular: checked == scope == 1.
			name: "full_scoped_singular",
			run: PatrolRunRecord{
				EffectiveScopeResourceIDs: []string{"a"},
				ResourcesChecked:          1,
			},
			want: "Checked 1 scoped resource",
		},
		{
			// Full scoped arm, plural: checked == scope == 2.
			name: "full_scoped_plural",
			run: PatrolRunRecord{
				EffectiveScopeResourceIDs: []string{"a", "b"},
				ResourcesChecked:          2,
			},
			want: "Checked 2 scoped resources",
		},
		{
			// Effective nil -> scope count falls back to ScopeResourceIDs.
			name: "partial_of_scope_via_legacy_scope_ids",
			run: PatrolRunRecord{
				ScopeResourceIDs: []string{"a", "b", "c"},
				ResourcesChecked: 1,
			},
			want: "Checked 1 of 3 scoped resources",
		},
		{
			// Effective non-nil empty overrides a populated ScopeResourceIDs:
			// scope count becomes 0 so the unscoped "Checked N resource[s]"
			// arm fires instead. Pins nil-vs-empty for coverage.
			name: "effective_empty_treated_as_unscoped",
			run: PatrolRunRecord{
				EffectiveScopeResourceIDs: []string{},
				ScopeResourceIDs:          []string{"a", "b"},
				ResourcesChecked:          1,
			},
			want: "Checked 1 resource",
		},
		{
			// Unscoped singular: no scope, checked == 1.
			name: "unscoped_singular",
			run: PatrolRunRecord{
				ResourcesChecked: 1,
			},
			want: "Checked 1 resource",
		},
		{
			// Unscoped plural: no scope, checked == 2.
			name: "unscoped_plural",
			run: PatrolRunRecord{
				ResourcesChecked: 2,
			},
			want: "Checked 2 resources",
		},
		{
			// Breakdown arm, single category.
			name: "breakdown_single",
			run: PatrolRunRecord{
				NodesChecked: 2,
			},
			want: "2 nodes",
		},
		{
			// Breakdown arm, multi category (joined with "; ").
			name: "breakdown_multi",
			run: PatrolRunRecord{
				NodesChecked:      2,
				GuestsChecked:     3,
				DockerChecked:     1,
				StorageChecked:    4,
				HostsChecked:      5,
				TrueNASChecked:    1,
				KubernetesChecked: 6,
			},
			want: "2 nodes; 3 VMs; 1 containers; 4 storage resources; 5 agents; 1 TrueNAS systems; 6 Kubernetes resources",
		},
		{
			// Subtle fall-through: scope is populated but resourcesChecked
			// is 0, so neither scoped inner-if fires and the breakdown join
			// is returned instead. Scoped zero-checked runs do NOT report
			// "Checked 0 of N"; see report for the source observation.
			name: "scoped_zero_checked_falls_through_to_breakdown",
			run: PatrolRunRecord{
				EffectiveScopeResourceIDs: []string{"a", "b"},
				ResourcesChecked:          0,
				NodesChecked:              1,
			},
			want: "1 nodes",
		},
		{
			// Negative ResourcesChecked is clamped to 0 via max(); with no
			// breakdown it yields the empty join.
			name: "negative_resources_checked_clamped_to_zero",
			run: PatrolRunRecord{
				ResourcesChecked: -5,
			},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := patrolRunCoverageSummary(tc.run)
			if got != tc.want {
				t.Fatalf("patrolRunCoverageSummary = %q, want %q", got, tc.want)
			}
		})
	}
}
