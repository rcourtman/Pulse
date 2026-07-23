package alerts

import (
	"reflect"
	"testing"
)

// TestBranchcov0723Am raises branch coverage of NormalizeAlertConfigAliases
// (the alerts-package facade at internal/alerts/config_facade.go:65, which
// delegates to internal/alerts/config/types.go:294).
//
// The function strips deprecated legacy resource-type keys from
// TimeThresholds and MetricTimeThresholds. Its branches are:
//   - nil config guard
//   - nil TimeThresholds (skip the first loop)
//   - per-key: CanonicalAlertResourceType(key) == "" or "all" -> continue (keep)
//   - per-key: isUnsupportedLegacyAlertResourceType(typeKey) -> delete, else keep
//   - len(MetricTimeThresholds) == 0 -> early return
//   - the same per-key arms on MetricTimeThresholds
//
// Each subtest is independent: it builds its own AlertConfig and asserts the
// concrete post-call map contents (and that the function never renames keys,
// only deletes).
func TestBranchcov0723Am(t *testing.T) {
	t.Run("nil_config_does_not_panic", func(t *testing.T) {
		// Drives the `config == nil` early-return guard.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NormalizeAlertConfigAliases(nil) panicked: %v", r)
			}
		}()
		NormalizeAlertConfigAliases(nil)
	})

	tests := []struct {
		name   string
		time   map[string]int
		metric map[string]map[string]int
		// Pointers so nil vs empty-non-nil map distinctions are explicit.
		wantTime   map[string]int
		wantMetric map[string]map[string]int
	}{
		{
			// TimeThresholds == nil exercises the `!= nil` skip; the non-empty
			// metric map forces the metric loop to run so we can observe the
			// function reached it (the supported "guest" key is retained).
			name:       "time_thresholds_nil_map_skipped_metric_loop_runs",
			time:       nil,
			metric:     map[string]map[string]int{"guest": {"cpu": 5}},
			wantTime:   nil,
			wantMetric: map[string]map[string]int{"guest": {"cpu": 5}},
		},
		{
			// Legacy keys are deleted; supported canonical keys are retained
			// with their original values. Both "host" and "docker_host" are in
			// the CanonicalizeLegacyResourceTypeAlias map, so both are caught by
			// the IsUnsupportedLegacyResourceTypeAlias arm before the local
			// switch is reached. The local-switch arm is exercised separately by
			// "docker" in the metric-threshold case below, which the alias map
			// does not recognise.
			name: "time_thresholds_strips_legacy_keeps_supported",
			time: map[string]int{
				"host":        1,
				"docker_host": 6,
				"guest":       2,
				"agent":       4,
			},
			metric:     nil,
			wantTime:   map[string]int{"guest": 2, "agent": 4},
			wantMetric: nil,
		},
		{
			// typeKey == "" (empty/whitespace raw key) and typeKey == "all"
			// both hit the `continue` arm and are preserved verbatim.
			name: "time_thresholds_preserves_all_blank_whitespace_keys",
			time: map[string]int{
				"all": 9,
				"":    8,
				"  ":  7,
			},
			metric:     nil,
			wantTime:   map[string]int{"all": 9, "": 8, "  ": 7},
			wantMetric: nil,
		},
		{
			// "kubernetes cluster" canonicalizes to "k8s-cluster" (supported),
			// so it is neither deleted nor renamed -- the function only strips,
			// it never rewrites the stored key. Asserts the original key survives.
			name:       "time_thresholds_canonicalized_type_not_renamed",
			time:       map[string]int{"kubernetes cluster": 11},
			metric:     nil,
			wantTime:   map[string]int{"kubernetes cluster": 11},
			wantMetric: nil,
		},
		{
			// MetricTimeThresholds legacy strip + supported retain. TimeThresholds
			// is nil, which also exercises the `len(metric) != 0` path (i.e. the
			// early-return arm is NOT taken here).
			name: "metric_strips_legacy_keeps_supported",
			time: nil,
			metric: map[string]map[string]int{
				"docker": {"cpu": 9},
				"node":   {"mem": 3},
			},
			wantTime: nil,
			wantMetric: map[string]map[string]int{
				"node": {"mem": 3},
			},
		},
		{
			// typeKey == "" and typeKey == "all" hit the metric-loop `continue`
			// arm and are preserved.
			name: "metric_preserves_all_blank_keys",
			time: nil,
			metric: map[string]map[string]int{
				"all": {"cpu": 1},
				"":    {"x": 2},
			},
			wantTime: nil,
			wantMetric: map[string]map[string]int{
				"all": {"cpu": 1},
				"":    {"x": 2},
			},
		},
		{
			// len(MetricTimeThresholds) == 0 (empty non-nil map) takes the
			// early return; meanwhile the TimeThresholds loop still ran and
			// stripped "host", proving the two paths are independent.
			name:       "metric_empty_map_returns_early",
			time:       map[string]int{"host": 1},
			metric:     map[string]map[string]int{},
			wantTime:   map[string]int{},
			wantMetric: map[string]map[string]int{},
		},
		{
			// Both maps populated with a legacy + supported pair; each is
			// reduced independently of the other.
			name: "both_maps_strip_legacy_independently",
			time: map[string]int{
				"host":  1,
				"guest": 2,
			},
			metric: map[string]map[string]int{
				"docker": {"cpu": 9},
				"node":   {"mem": 3},
			},
			wantTime: map[string]int{"guest": 2},
			wantMetric: map[string]map[string]int{
				"node": {"mem": 3},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := &AlertConfig{
				TimeThresholds:       tc.time,
				MetricTimeThresholds: tc.metric,
			}
			NormalizeAlertConfigAliases(cfg)

			if !reflect.DeepEqual(cfg.TimeThresholds, tc.wantTime) {
				t.Fatalf("TimeThresholds mismatch:\n got: %#v\nwant: %#v",
					cfg.TimeThresholds, tc.wantTime)
			}
			if !reflect.DeepEqual(cfg.MetricTimeThresholds, tc.wantMetric) {
				t.Fatalf("MetricTimeThresholds mismatch:\n got: %#v\nwant: %#v",
					cfg.MetricTimeThresholds, tc.wantMetric)
			}
		})
	}
}
