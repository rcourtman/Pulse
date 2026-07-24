package alerts

import (
	"testing"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
)

// TestBranchcov0724pmMetricClearThreshold raises branch coverage of
// metricClearThreshold (canonical_metric.go:36). Its arms, in evaluation
// order, are:
//  1. threshold != nil && threshold.Clear > 0  -> return threshold.Clear
//  2. spec != nil && spec.Recovery != nil      -> return *spec.Recovery
//  3. spec != nil                              -> return spec.Trigger
//  4. (both nil / fell through)                -> return 0
//
// The existing suite only reaches arm 3 through the live metric-evaluation
// path, so 28.6%. These cases drive every arm plus the hysteresis boundary
// (Clear == 0 must fall through to the spec) and confirm precedence between
// the three sources. Each threshold/spec source carries a distinct value so
// the expected result pins exactly which arm produced it.
func TestBranchcov0724pmMetricClearThreshold(t *testing.T) {
	recoveryVal := 41.0
	cases := []struct {
		name      string
		spec      *alertspecs.MetricThresholdSpec
		threshold *HysteresisThreshold
		want      float64
	}{
		{
			// Arm 1: positive Clear wins outright, even when a spec with its own
			// recovery/trigger is present (proves the threshold short-circuits).
			name:      "positive_clear_wins_over_spec_recovery_and_trigger",
			threshold: &HysteresisThreshold{Trigger: 95, Clear: 12.5},
			spec:      &alertspecs.MetricThresholdSpec{Trigger: 90, Recovery: &recoveryVal},
			want:      12.5,
		},
		{
			// Boundary of `Clear > 0`: a zero Clear must NOT take arm 1, so the
			// spec recovery is used instead.
			name:      "zero_clear_falls_through_to_spec_recovery",
			threshold: &HysteresisThreshold{Trigger: 95, Clear: 0},
			spec:      &alertspecs.MetricThresholdSpec{Trigger: 90, Recovery: &recoveryVal},
			want:      41.0,
		},
		{
			// Negative Clear also fails `> 0` and falls through to spec recovery.
			name:      "negative_clear_falls_through_to_spec_recovery",
			threshold: &HysteresisThreshold{Trigger: 95, Clear: -1},
			spec:      &alertspecs.MetricThresholdSpec{Trigger: 90, Recovery: &recoveryVal},
			want:      41.0,
		},
		{
			// Arm 2 with nil threshold: spec recovery is returned.
			name: "nil_threshold_uses_spec_recovery",
			spec: &alertspecs.MetricThresholdSpec{Trigger: 90, Recovery: &recoveryVal},
			want: 41.0,
		},
		{
			// Arm 3: nil threshold, no recovery -> spec trigger.
			name: "nil_threshold_no_recovery_uses_trigger",
			spec: &alertspecs.MetricThresholdSpec{Trigger: 90},
			want: 90,
		},
		{
			// Arm 3 reached via fall-through: zero Clear, no recovery -> trigger.
			name:      "zero_clear_no_recovery_uses_trigger",
			threshold: &HysteresisThreshold{Trigger: 95, Clear: 0},
			spec:      &alertspecs.MetricThresholdSpec{Trigger: 88},
			want:      88,
		},
		{
			// Arm 4: both nil -> 0.
			name: "both_nil_returns_zero",
			want: 0,
		},
		{
			// Arm 4 via fall-through: threshold present but Clear <= 0 and no
			// spec at all -> 0 (proves the final return is reached).
			name:      "zero_clear_nil_spec_returns_zero",
			threshold: &HysteresisThreshold{Trigger: 95, Clear: 0},
			spec:      nil,
			want:      0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := metricClearThreshold(tc.spec, tc.threshold)
			if got != tc.want {
				t.Fatalf("metricClearThreshold want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmResourceTypeLabel raises branch coverage of
// resourceTypeLabel (canonical_metric.go:49). The existing suite covers the
// common metric types via the live path (~50%); these cases drive every named
// switch arm plus the empty/whitespace "Resource" arm and -- critically --
// assert that the default arm returns the ORIGINAL (un-trimmed) input, which
// is an observable distinction from the trimmed cases.
func TestBranchcov0724pmResourceTypeLabel(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"agent_disk", "agent-disk", "Disk"},
		{"agent", "agent", "Agent"},
		{"node", "node", "Node"},
		{"guest", "guest", "Guest"},
		{"storage", "storage", "Storage"},
		{"pbs", "pbs", "PBS"},
		{"pmg", "pmg", "PMG"},
		{"empty_is_resource", "", "Resource"},
		// Whitespace is trimmed before the switch, so whitespace-only collapses
		// to "" and yields "Resource".
		{"whitespace_only_is_resource", "   ", "Resource"},
		// Surrounding whitespace is trimmed, then matched.
		{"padded_agent_matches", "  agent  ", "Agent"},
		// Default arm: unknown type returned verbatim, NOT trimmed.
		{"unknown_returned_as_is", "weird-type", "weird-type"},
		// The un-trimmed behaviour of the default arm is observable when the
		// input carries surrounding whitespace that a named case would have
		// trimmed -- this pins that the default does not re-trim.
		{"unknown_padded_returned_untrimmed", "  weird-type  ", "  weird-type  "},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := resourceTypeLabel(tc.in); got != tc.want {
				t.Fatalf("resourceTypeLabel(%q) want %q, got %q", tc.in, tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmAlertspecsMetricTriggered raises branch coverage of
// alertspecsMetricTriggered (canonical_metric.go:355). The existing suite only
// exercises the "above" direction via live evaluation (~50%). These cases
// drive: nil spec, the "above" boundary (>=), the "below" direction and its
// boundary (<=), and the default/invalid-direction arm.
func TestBranchcov0724pmAlertspecsMetricTriggered(t *testing.T) {
	cases := []struct {
		name     string
		spec     *alertspecs.MetricThresholdSpec
		observed float64
		want     bool
	}{
		{"nil_spec_is_false", nil, 999, false},
		{
			// Above: strictly over trigger -> true.
			"above_over_trigger",
			&alertspecs.MetricThresholdSpec{Metric: "cpu", Direction: alertspecs.ThresholdDirectionAbove, Trigger: 90},
			91, true,
		},
		{
			// Boundary of `observed >= Trigger`: equality triggers.
			"above_equal_trigger_triggers",
			&alertspecs.MetricThresholdSpec{Metric: "cpu", Direction: alertspecs.ThresholdDirectionAbove, Trigger: 90},
			90, true,
		},
		{
			"above_under_trigger_does_not_trigger",
			&alertspecs.MetricThresholdSpec{Metric: "cpu", Direction: alertspecs.ThresholdDirectionAbove, Trigger: 90},
			89.9, false,
		},
		{
			// Below: strictly under trigger -> true.
			"below_under_trigger",
			&alertspecs.MetricThresholdSpec{Metric: "cpu", Direction: alertspecs.ThresholdDirectionBelow, Trigger: 10},
			9, true,
		},
		{
			// Boundary of `observed <= Trigger`: equality triggers.
			"below_equal_trigger_triggers",
			&alertspecs.MetricThresholdSpec{Metric: "cpu", Direction: alertspecs.ThresholdDirectionBelow, Trigger: 10},
			10, true,
		},
		{
			"below_over_trigger_does_not_trigger",
			&alertspecs.MetricThresholdSpec{Metric: "cpu", Direction: alertspecs.ThresholdDirectionBelow, Trigger: 10},
			10.1, false,
		},
		{
			// Default arm: an unrecognised direction never triggers.
			"invalid_direction_is_false",
			&alertspecs.MetricThresholdSpec{Metric: "cpu", Direction: "sideways", Trigger: 1},
			5, false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := alertspecsMetricTriggered(tc.spec, tc.observed); got != tc.want {
				t.Fatalf("alertspecsMetricTriggered want %v, got %v", tc.want, got)
			}
		})
	}
}
