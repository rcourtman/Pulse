package specs

import (
	"math"
	"strings"
	"testing"
	"time"
)

// This file is a purpose-built branch-coverage test set (selectable via
// `-run "^TestBranchcov0725am"`) for four partially-covered functions whose
// uncovered arms the existing suite does not reach:
//
//   - ChangeThresholdSpec.Validate   (types.go:402)     baseline 57.1%
//   - BaselineAnomalySpec.Validate   (types.go:447)     baseline 58.8%
//   - metricTriggered                (evaluator.go:701) baseline 50.0%
//   - metricStillLatched             (evaluator.go:715) baseline 50.0%
//
// Conventions mirror the sibling in-package branch-coverage file
// evaluator_branchcov0724pm_test.go and types_test.go: stdlib `testing` only,
// table-driven subtests, t.Parallel(), t.Fatalf assertions on the concrete
// returned error / bool. No testify.
//
// The two Validate methods are reached by the existing suite only through the
// "happy path" accept case (ResourceAlertSpec.Validate -> payload.Validate) and,
// for ChangeThreshold, the warning-percent-without-delta arm; for
// BaselineAnomaly, the inverted quiet-delta arm. metricTriggered and
// metricStillLatched are never called by name in any test and are only reached
// indirectly through Evaluate, which always supplies a non-nil, validated spec —
// so their nil-spec and unknown-direction arms are entirely uncovered. These
// tests call all four functions directly to drive every remaining arm.
//
// Purity: every target is a pure function over its struct arguments; no network,
// daemon, database, or filesystem is touched.

// assertBranchcov0725amError centralises the "specific rejection reason"
// assertion used by both Validate tables: when wantErr is true the returned
// error must be non-nil and contain wantSubstr (asserting the specific error
// path); when wantErr is false the error must be nil (the accepting case).
func assertBranchcov0725amError(t *testing.T, err error, wantErr bool, wantSubstr string) {
	t.Helper()
	if wantErr {
		if err == nil {
			t.Fatalf("expected error containing %q, got nil", wantSubstr)
		}
		if !strings.Contains(err.Error(), wantSubstr) {
			t.Fatalf("expected error containing %q, got %q", wantSubstr, err.Error())
		}
		return
	}
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestBranchcov0725amChangeThresholdSpecValidate drives every distinct
// rejection reason in ChangeThresholdSpec.Validate (types.go:402) as its own
// case asserting the specific error, plus the accepting case. The existing
// suite already covers only the warning-percent-without-delta arm and the
// accept path; this adds the empty-metric, negative-window, non-finite,
// negative-percent, critical-percent-without-delta, inverted-percent,
// no-threshold, inverted-current and inverted-delta arms.
func TestBranchcov0725amChangeThresholdSpecValidate(t *testing.T) {
	t.Parallel()

	// A fully-valid change threshold used as the base for rejection cases; each
	// case below overrides only the field(s) relevant to the arm under test so
	// that every earlier guard passes and the targeted return is reached.
	positivePercentDelta := ChangeThresholdSpec{
		Metric:          "quarantine-spam",
		ReferenceWindow: 2 * time.Hour,
		WarningCurrent:  2000,
		CriticalCurrent: 5000,
		WarningDelta:    250,
		CriticalDelta:   500,
		WarningPercent:  25,
		CriticalPercent: 50,
	}

	cases := []struct {
		name       string
		spec       ChangeThresholdSpec
		wantErr    bool
		wantSubstr string
	}{
		// ---- accepting case (arm: return nil) ----
		{"accepts fully populated spec", positivePercentDelta, false, ""},

		// ---- empty metric (arm: "metric is required") ----
		{"empty metric rejected", ChangeThresholdSpec{Metric: "  ", WarningCurrent: 1}, true, "metric is required"},

		// ---- negative reference window (arm: "reference window must be zero or positive") ----
		{"negative reference window rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam", ReferenceWindow: -1 * time.Second, WarningCurrent: 1,
		}, true, "reference window must be zero or positive"},

		// ---- non-finite threshold (arm: "thresholds must be finite") ----
		// Exactly one field is non-finite so the big || fires deterministically.
		{"non-finite warning current rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam", WarningCurrent: math.NaN(), CriticalCurrent: 1,
		}, true, "thresholds must be finite"},
		{"infinite critical delta rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam", CriticalCurrent: 1, CriticalDelta: math.Inf(1),
		}, true, "thresholds must be finite"},

		// ---- negative percent (arm: "percent thresholds must not be negative") ----
		{"negative warning percent rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam", WarningCurrent: 1, WarningPercent: -5,
		}, true, "percent thresholds must not be negative"},
		{"negative critical percent rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam", CriticalCurrent: 1, CriticalPercent: -1,
		}, true, "percent thresholds must not be negative"},

		// ---- critical percent without critical delta (arm: "critical delta is required when critical percent is set") ----
		// WarningPercent is left at 0 so the warning guard does not fire first.
		{"critical percent without critical delta rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam", CriticalCurrent: 1, CriticalPercent: 50,
		}, true, "critical delta is required when critical percent is set"},

		// ---- inverted percent (arm: "critical percent must be greater than or equal to warning percent") ----
		// Both percents > 0 with matching deltas so the percent/delta guards pass.
		{"critical percent below warning percent rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam", CriticalCurrent: 1,
			WarningDelta: 250, CriticalDelta: 500, WarningPercent: 50, CriticalPercent: 25,
		}, true, "critical percent must be greater than or equal to warning percent"},

		// ---- no current or delta threshold (arm: "at least one current or delta threshold is required") ----
		// All currents/deltas zero and percents zero so every earlier guard passes.
		{"no current or delta threshold rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam",
		}, true, "at least one current or delta threshold is required"},

		// ---- inverted current (arm: "critical current must be greater than or equal to warning current") ----
		{"critical current below warning current rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam", WarningCurrent: 5000, CriticalCurrent: 2000,
		}, true, "critical current must be greater than or equal to warning current"},

		// ---- inverted delta (arm: "critical delta must be greater than or equal to warning delta") ----
		// Currents left at zero so the inverted-current guard is skipped.
		{"critical delta below warning delta rejected", ChangeThresholdSpec{
			Metric: "quarantine-spam", WarningDelta: 500, CriticalDelta: 250,
		}, true, "critical delta must be greater than or equal to warning delta"},

		// ---- boundary: equal current/delta pairs are accepted (not strictly inverted) ----
		{"equal warning and critical current accepted", ChangeThresholdSpec{
			Metric: "quarantine-spam", WarningCurrent: 2000, CriticalCurrent: 2000,
		}, false, ""},
		{"equal warning and critical delta accepted", ChangeThresholdSpec{
			Metric: "quarantine-spam", WarningDelta: 250, CriticalDelta: 250,
		}, false, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.spec.Validate()
			assertBranchcov0725amError(t, err, tc.wantErr, tc.wantSubstr)
		})
	}
}

// TestBranchcov0725amBaselineAnomalySpecValidate drives every distinct
// rejection reason in BaselineAnomalySpec.Validate (types.go:447) as its own
// case asserting the specific error, plus the accepting case. The existing
// suite already covers only the inverted-quiet-delta arm and the accept path;
// this adds the empty-metric, non-finite, negative-quiet-baseline,
// negative-ratio, non-positive-delta, inverted-ratio and inverted-delta arms.
func TestBranchcov0725amBaselineAnomalySpecValidate(t *testing.T) {
	t.Parallel()

	// A fully-valid baseline anomaly used as the base for rejection cases.
	valid := BaselineAnomalySpec{
		Metric:             "spamIn",
		QuietBaseline:      40,
		WarningRatio:       1.8,
		CriticalRatio:      2.5,
		WarningDelta:       150,
		CriticalDelta:      300,
		QuietWarningDelta:  60,
		QuietCriticalDelta: 120,
	}

	cases := []struct {
		name       string
		spec       BaselineAnomalySpec
		wantErr    bool
		wantSubstr string
	}{
		// ---- accepting case (arm: return nil) ----
		{"accepts fully populated spec", valid, false, ""},

		// ---- empty metric (arm: "metric is required") ----
		{"empty metric rejected", BaselineAnomalySpec{Metric: "\t", QuietBaseline: 1, WarningDelta: 1, CriticalDelta: 1, QuietWarningDelta: 1, QuietCriticalDelta: 1}, true, "metric is required"},

		// ---- non-finite threshold (arm: "thresholds must be finite") ----
		{"non-finite quiet baseline rejected", BaselineAnomalySpec{
			Metric: "spamIn", QuietBaseline: math.NaN(),
			WarningDelta: 1, CriticalDelta: 1, QuietWarningDelta: 1, QuietCriticalDelta: 1,
		}, true, "thresholds must be finite"},
		{"negative-infinite critical ratio rejected", BaselineAnomalySpec{
			Metric: "spamIn", QuietBaseline: 1, CriticalRatio: math.Inf(-1),
			WarningDelta: 1, CriticalDelta: 1, QuietWarningDelta: 1, QuietCriticalDelta: 1,
		}, true, "thresholds must be finite"},

		// ---- negative quiet baseline (arm: "quiet baseline must not be negative") ----
		{"negative quiet baseline rejected", BaselineAnomalySpec{
			Metric: "spamIn", QuietBaseline: -5,
			WarningDelta: 1, CriticalDelta: 1, QuietWarningDelta: 1, QuietCriticalDelta: 1,
		}, true, "quiet baseline must not be negative"},

		// ---- negative ratio (arm: "ratios must not be negative") ----
		{"negative warning ratio rejected", BaselineAnomalySpec{
			Metric: "spamIn", QuietBaseline: 40, WarningRatio: -1.8,
			WarningDelta: 1, CriticalDelta: 1, QuietWarningDelta: 1, QuietCriticalDelta: 1,
		}, true, "ratios must not be negative"},
		{"negative critical ratio rejected", BaselineAnomalySpec{
			Metric: "spamIn", QuietBaseline: 40, WarningRatio: 1.8, CriticalRatio: -2.5,
			WarningDelta: 1, CriticalDelta: 1, QuietWarningDelta: 1, QuietCriticalDelta: 1,
		}, true, "ratios must not be negative"},

		// ---- non-positive delta (arm: "delta thresholds must be positive") ----
		// Exactly one delta is zero (the rest positive) so the <= 0 guard fires.
		{"zero warning delta rejected", BaselineAnomalySpec{
			Metric: "spamIn", QuietBaseline: 40, WarningRatio: 1.8, CriticalRatio: 2.5,
			WarningDelta: 0, CriticalDelta: 300, QuietWarningDelta: 60, QuietCriticalDelta: 120,
		}, true, "delta thresholds must be positive"},

		// ---- inverted ratio (arm: "critical ratio must be greater than or equal to warning ratio") ----
		{"critical ratio below warning ratio rejected", BaselineAnomalySpec{
			Metric: "spamIn", QuietBaseline: 40, WarningRatio: 2.5, CriticalRatio: 1.8,
			WarningDelta: 150, CriticalDelta: 300, QuietWarningDelta: 60, QuietCriticalDelta: 120,
		}, true, "critical ratio must be greater than or equal to warning ratio"},

		// ---- inverted delta (arm: "critical delta must be greater than or equal to warning delta") ----
		{"critical delta below warning delta rejected", BaselineAnomalySpec{
			Metric: "spamIn", QuietBaseline: 40, WarningRatio: 1.8, CriticalRatio: 2.5,
			WarningDelta: 300, CriticalDelta: 150, QuietWarningDelta: 60, QuietCriticalDelta: 120,
		}, true, "critical delta must be greater than or equal to warning delta"},

		// ---- boundary: zero ratios are accepted (the ratio checks are skipped when ratios are 0) ----
		{"zero ratios with positive deltas accepted", BaselineAnomalySpec{
			Metric: "spamIn", QuietBaseline: 40, WarningRatio: 0, CriticalRatio: 0,
			WarningDelta: 150, CriticalDelta: 300, QuietWarningDelta: 60, QuietCriticalDelta: 120,
		}, false, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.spec.Validate()
			assertBranchcov0725amError(t, err, tc.wantErr, tc.wantSubstr)
		})
	}
}

// TestBranchcov0725amMetricTriggered drives every arm of metricTriggered
// (evaluator.go:701): the nil-spec early return, both valid directions with the
// metric value pinned at / just below / just above the trigger, and the
// unknown-direction default. Evaluate always feeds a non-nil validated spec, so
// only the direction arms (and only some of their outcomes) are covered today.
// The "latched" context is represented by repeating the at/just-above/just-below
// pinning with a spec that carries a Recovery pointer, proving metricTriggered
// is independent of latch state (it consults only Trigger/Direction).
func TestBranchcov0725amMetricTriggered(t *testing.T) {
	t.Parallel()

	above := func(recovery *float64) *MetricThresholdSpec {
		return &MetricThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Trigger: 90, Recovery: recovery}
	}
	below := func(recovery *float64) *MetricThresholdSpec {
		return &MetricThresholdSpec{Metric: "latency", Direction: ThresholdDirectionBelow, Trigger: 10, Recovery: recovery}
	}
	noRecovery := func() *float64 { return nil }

	cases := []struct {
		name     string
		spec     *MetricThresholdSpec
		observed float64
		want     bool
	}{
		// nil spec short-circuits to false regardless of the observed value.
		{"nil spec returns false", nil, 9999, false},

		// direction above: triggered is observed >= Trigger (boundary-inclusive).
		{"above at trigger returns true", above(noRecovery()), 90, true},
		{"above just above trigger returns true", above(noRecovery()), 90.5, true},
		{"above just below trigger returns false", above(noRecovery()), 89.999, false},

		// direction below: triggered is observed <= Trigger (boundary-inclusive).
		{"below at trigger returns true", below(noRecovery()), 10, true},
		{"below just below trigger returns true", below(noRecovery()), 9.5, true},
		{"below just above trigger returns false", below(noRecovery()), 10.001, false},

		// unknown direction falls through to the default false arm.
		{"unknown direction returns false", &MetricThresholdSpec{Metric: "cpu", Direction: "sideways", Trigger: 90}, 9999, false},
		{"empty direction returns false", &MetricThresholdSpec{Metric: "cpu", Direction: "", Trigger: 90}, 9999, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := metricTriggered(tc.spec, tc.observed)
			if got != tc.want {
				t.Fatalf("metricTriggered(observed=%v) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amMetricStillLatched drives every arm of metricStillLatched
// (evaluator.go:715): the nil-spec early return, the nil-Recovery early return,
// both valid directions with the metric pinned at / just below / just above the
// recovery threshold (note the comparison is strict, so observed == recovery is
// "recovered" = false), and the unknown-direction default. The "still latched"
// true outcome and the "recovered / unlatched" false outcome are both driven for
// each direction.
func TestBranchcov0725amMetricStillLatched(t *testing.T) {
	t.Parallel()

	aboveRecovery := func() *float64 { v := 85.0; return &v }
	belowRecovery := func() *float64 { v := 15.0; return &v }

	cases := []struct {
		name     string
		spec     *MetricThresholdSpec
		observed float64
		want     bool
	}{
		// nil spec short-circuits to false.
		{"nil spec returns false", nil, 9999, false},

		// nil Recovery short-circuits to false (no latch point defined).
		{"nil recovery returns false", &MetricThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Trigger: 90}, 9999, false},

		// direction above: still latched is observed > *Recovery (strict).
		{"above just above recovery returns true (latched)", &MetricThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Trigger: 90, Recovery: aboveRecovery()}, 86, true},
		{"above at recovery returns false (recovered)", &MetricThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Trigger: 90, Recovery: aboveRecovery()}, 85, false},
		{"above just below recovery returns false", &MetricThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Trigger: 90, Recovery: aboveRecovery()}, 84.999, false},

		// direction below: still latched is observed < *Recovery (strict).
		{"below just below recovery returns true (latched)", &MetricThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionBelow, Trigger: 10, Recovery: belowRecovery()}, 14, true},
		{"below at recovery returns false (recovered)", &MetricThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionBelow, Trigger: 10, Recovery: belowRecovery()}, 15, false},
		{"below just above recovery returns false", &MetricThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionBelow, Trigger: 10, Recovery: belowRecovery()}, 15.001, false},

		// unknown direction with a Recovery set still falls through to false.
		{"unknown direction returns false", &MetricThresholdSpec{Metric: "cpu", Direction: "sideways", Trigger: 90, Recovery: aboveRecovery()}, 9999, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := metricStillLatched(tc.spec, tc.observed)
			if got != tc.want {
				t.Fatalf("metricStillLatched(observed=%v) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}
