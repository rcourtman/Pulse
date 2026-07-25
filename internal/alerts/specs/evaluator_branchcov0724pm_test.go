package specs

import (
	"testing"
	"time"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run "^TestBranchcov0724pm"`) for the alert-spec matching predicates in
// evaluator.go that the existing evaluator_test.go suite only reaches
// indirectly (and partially) through Evaluate. The functions are unexported, so
// these tests live in-package and call the predicates directly, which lets them
// exercise arms that Evaluate's validation/dispatch makes hard to reach:
//
//   - matches                            (evaluator.go:435)  baseline 47.9%
//   - matchesSeverityThreshold           (evaluator.go:518)  baseline 50.0%
//   - severityThresholdStillLatched      (evaluator.go:547)  baseline 62.5%
//   - matchesChangeThreshold             (evaluator.go:565)  baseline 58.8%
//   - matchesBaselineAnomaly             (evaluator.go:598)  baseline 63.2%
//   - matchesHealthAssessment            (evaluator.go:636)  baseline 75.0%
//   - matchesPostureThreshold            (evaluator.go:658)  baseline 66.7%
//
// Conventions match sibling in-package tests in this directory (see
// rollup_evidence_branchcov0723am_test.go and evaluator_test.go): stdlib
// `testing` only, table-driven subtests, t.Fatalf assertions on concrete
// expected (matched, severity, reason) triples, no testify.
//
// Purity: every target is a pure function over its struct arguments; no
// network, daemon, database, or filesystem is touched.

// matchTriple is the (matched, severity, reason) shape returned by every
// matches* predicate, used to keep table assertions uniform and tautology-free.
type matchTriple struct {
	matched  bool
	severity AlertSeverity
	reason   string
}

func assertMatchTriple(t *testing.T, got, want matchTriple) {
	t.Helper()
	if got.matched != want.matched || got.severity != want.severity || got.reason != want.reason {
		t.Fatalf("got (matched=%v severity=%q reason=%q), want (matched=%v severity=%q reason=%q)",
			got.matched, got.severity, got.reason, want.matched, want.severity, want.reason)
	}
}

// TestBranchcov0724pmMatches drives the matches() dispatcher (evaluator.go:435)
// across every spec kind, focusing on the arms the existing suite does not
// reach when going through Evaluate: the nil/missing-evidence early return of
// each payload kind, the unknown-kind default arm, the ProviderIncident and
// ResourceIncidentRollup kinds (entirely uncovered), the ServiceGap duration
// fallback + negative-missing clamp, and the no-match arms of DiscreteState and
// Connectivity/PoweredState.
func TestBranchcov0724pmMatches(t *testing.T) {
	// Shared, fully-populated payloads used by both the "matched" and
	// "nil-evidence" variants of each kind so the only variable is the
	// evidence/spec presence.
	sevSpec := &SeverityThresholdSpec{Metric: "queue", Direction: ThresholdDirectionAbove, Warning: 500, Critical: 1000}
	changeSpec := &ChangeThresholdSpec{Metric: "spam", WarningCurrent: 2000, CriticalCurrent: 5000, WarningDelta: 250, CriticalDelta: 500, WarningPercent: 25, CriticalPercent: 50}
	baselineSpec := &BaselineAnomalySpec{Metric: "spamIn", QuietBaseline: 40, WarningRatio: 1.8, CriticalRatio: 2.5, WarningDelta: 150, CriticalDelta: 300, QuietWarningDelta: 60, QuietCriticalDelta: 120}
	healthSpec := &HealthAssessmentSpec{Signal: "host-raid", Codes: []string{"raid_degraded"}}
	postureSpec := &PostureThresholdSpec{AgeMetric: "age", WarningAge: 7, CriticalAge: 14, SizeMetric: "size", WarningSize: 50, CriticalSize: 100}
	discreteSpec := &DiscreteStateSpec{StateKey: "state", TriggerStates: []string{"paused"}}
	svcGapPercentSpec := &ServiceGapSpec{Service: "web", WarningPercent: 10, CriticalPercent: 50, GapAfter: 5 * time.Minute}
	svcGapDurationSpec := &ServiceGapSpec{Service: "web", GapAfter: 5 * time.Minute}
	providerSpec := &ProviderIncidentSpec{Provider: "aws", Codes: []string{"aws_ec2_degraded"}, NativeIDs: []string{"inc-1"}}
	rollupSpec := &ResourceIncidentRollupSpec{Code: "cap_low", IncidentCount: 3}

	cases := []struct {
		name     string
		spec     ResourceAlertSpec
		evidence AlertEvidence
		want     matchTriple
	}{
		// ---- nil / missing-evidence early returns (one per payload kind) ----
		{
			name:     "severity threshold nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindSeverityThreshold, Severity: AlertSeverityWarning, SeverityThreshold: sevSpec},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "severity threshold nil spec payload returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindSeverityThreshold, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{SeverityThreshold: &SeverityThresholdEvidence{Metric: "queue", Direction: ThresholdDirectionAbove, Observed: 700}},
			want:     matchTriple{false, "", ""},
		},
		{
			// SeverityThreshold is normally dispatched to evaluateSeverityThreshold
			// by Evaluate and never reaches matches(); calling matches() directly
			// with both payloads present exercises the delegation return to
			// matchesSeverityThreshold.
			name:     "severity threshold delegated to matchesSeverityThreshold",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindSeverityThreshold, Severity: AlertSeverityWarning, SeverityThreshold: sevSpec},
			evidence: AlertEvidence{SeverityThreshold: &SeverityThresholdEvidence{Metric: "queue", Direction: ThresholdDirectionAbove, Observed: 1200}},
			want:     matchTriple{true, AlertSeverityCritical, "severity-threshold-critical"},
		},
		{
			name:     "change threshold nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindChangeThreshold, Severity: AlertSeverityWarning, ChangeThreshold: changeSpec},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "change threshold nil spec payload returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindChangeThreshold, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{ChangeThreshold: &ChangeThresholdEvidence{Metric: "spam", Observed: 2500}},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "baseline anomaly nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindBaselineAnomaly, Severity: AlertSeverityWarning, BaselineAnomaly: baselineSpec},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "baseline anomaly nil spec payload returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindBaselineAnomaly, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{BaselineAnomaly: &BaselineAnomalyEvidence{Metric: "spamIn", Observed: 420, Baseline: 100}},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "health assessment nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindHealthAssessment, Severity: AlertSeverityWarning, HealthAssessment: healthSpec},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "health assessment nil spec payload returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindHealthAssessment, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{HealthAssessment: &HealthAssessmentEvidence{Signal: "host-raid", Severity: AlertSeverityWarning, Codes: []string{"raid_degraded"}}},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "posture threshold nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindPostureThreshold, Severity: AlertSeverityWarning, PostureThreshold: postureSpec},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "posture threshold nil spec payload returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindPostureThreshold, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{PostureThreshold: &PostureThresholdEvidence{AgeMetric: "age", AgeValue: 20}},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "discrete state nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindDiscreteState, Severity: AlertSeverityWarning, DiscreteState: discreteSpec},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "discrete state nil spec payload returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindDiscreteState, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{DiscreteState: &DiscreteStateEvidence{StateKey: "state", Observed: "paused"}},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "service gap nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindServiceGap, Severity: AlertSeverityWarning, ServiceGap: svcGapPercentSpec},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "service gap nil spec payload returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindServiceGap, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{ServiceGap: &ServiceGapEvidence{Service: "web", Desired: 10, Running: 8}},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "provider incident nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindProviderIncident, Severity: AlertSeverityWarning, ProviderIncident: providerSpec},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "provider incident nil spec payload returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindProviderIncident, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{ProviderIncident: &ProviderIncidentEvidence{Provider: "aws", Code: "aws_ec2_degraded"}},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "resource incident rollup nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindResourceIncidentRollup, Severity: AlertSeverityWarning, ResourceIncidentRollup: rollupSpec},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "resource incident rollup nil spec payload returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindResourceIncidentRollup, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{ResourceIncidentRollup: &ResourceIncidentRollupEvidence{Code: "cap_low", IncidentCount: 3}},
			want:     matchTriple{false, "", ""},
		},

		// ---- Connectivity: nil evidence vs disconnected vs connected ----
		{
			name:     "connectivity nil evidence returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindConnectivity, Severity: AlertSeverityCritical},
			evidence: AlertEvidence{},
			want:     matchTriple{false, AlertSeverityCritical, "connectivity-lost"},
		},
		{
			name:     "connectivity connected returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindConnectivity, Severity: AlertSeverityCritical},
			evidence: AlertEvidence{Connectivity: &ConnectivityEvidence{Signal: "heartbeat", Connected: true}},
			want:     matchTriple{false, AlertSeverityCritical, "connectivity-lost"},
		},
		{
			name:     "connectivity disconnected returns true critical",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindConnectivity, Severity: AlertSeverityCritical},
			evidence: AlertEvidence{Connectivity: &ConnectivityEvidence{Signal: "heartbeat", Connected: false}},
			want:     matchTriple{true, AlertSeverityCritical, "connectivity-lost"},
		},

		// ---- PoweredState: nil evidence vs match vs match ----
		{
			name:     "powered state nil evidence returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindPoweredState, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			name:     "powered state matching observed equals expected returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindPoweredState, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{PoweredState: &PoweredStateEvidence{Expected: PowerStateOn, Observed: PowerStateOn}},
			want:     matchTriple{false, AlertSeverityWarning, "powered-state-mismatch"},
		},
		{
			name:     "powered state mismatch returns true",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindPoweredState, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{PoweredState: &PoweredStateEvidence{Expected: PowerStateOn, Observed: PowerStateOff}},
			want:     matchTriple{true, AlertSeverityWarning, "powered-state-mismatch"},
		},

		// ---- DiscreteState: in-set vs not-in-set ----
		{
			name:     "discrete state observed not in trigger set returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindDiscreteState, Severity: AlertSeverityWarning, DiscreteState: discreteSpec},
			evidence: AlertEvidence{DiscreteState: &DiscreteStateEvidence{StateKey: "state", Observed: "running"}},
			want:     matchTriple{false, AlertSeverityWarning, "discrete-state-match"},
		},
		{
			name:     "discrete state observed in trigger set returns true",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindDiscreteState, Severity: AlertSeverityWarning, DiscreteState: discreteSpec},
			evidence: AlertEvidence{DiscreteState: &DiscreteStateEvidence{StateKey: "state", Observed: "paused"}},
			want:     matchTriple{true, AlertSeverityWarning, "discrete-state-match"},
		},

		// ---- ServiceGap: percent critical/warning/normal + duration + clamp ----
		{
			name:     "service gap critical percent returns critical",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindServiceGap, Severity: AlertSeverityWarning, ServiceGap: svcGapPercentSpec},
			evidence: AlertEvidence{ServiceGap: &ServiceGapEvidence{Service: "web", Desired: 10, Running: 4}},
			want:     matchTriple{true, AlertSeverityCritical, "service-gap-critical"},
		},
		{
			name:     "service gap warning percent returns warning",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindServiceGap, Severity: AlertSeverityWarning, ServiceGap: svcGapPercentSpec},
			evidence: AlertEvidence{ServiceGap: &ServiceGapEvidence{Service: "web", Desired: 10, Running: 8}},
			want:     matchTriple{true, AlertSeverityWarning, "service-gap-warning"},
		},
		{
			name:     "service gap percent below thresholds returns normal",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindServiceGap, Severity: AlertSeverityWarning, ServiceGap: svcGapPercentSpec},
			evidence: AlertEvidence{ServiceGap: &ServiceGapEvidence{Service: "web", Desired: 10, Running: 10}},
			want:     matchTriple{false, "", "service-gap-normal"},
		},
		{
			// Running exceeds Desired: missing goes negative and is clamped to 0,
			// yielding percent 0 -> normal. Exercises the `if missing < 0` arm.
			name:     "service gap running exceeds desired clamps missing to zero",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindServiceGap, Severity: AlertSeverityWarning, ServiceGap: svcGapPercentSpec},
			evidence: AlertEvidence{ServiceGap: &ServiceGapEvidence{Service: "web", Desired: 10, Running: 12}},
			want:     matchTriple{false, "", "service-gap-normal"},
		},
		{
			// Desired == 0 falls through to the duration branch; MissingFor past
			// GapAfter with GapAfter > 0 -> true at spec severity.
			name:     "service gap duration beyond gapAfter returns true",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindServiceGap, Severity: AlertSeverityWarning, ServiceGap: svcGapDurationSpec},
			evidence: AlertEvidence{ServiceGap: &ServiceGapEvidence{Service: "web", Desired: 0, MissingFor: 6 * time.Minute}},
			want:     matchTriple{true, AlertSeverityWarning, "service-gap-duration"},
		},
		{
			// Duration branch, MissingFor below GapAfter -> false.
			name:     "service gap duration below gapAfter returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindServiceGap, Severity: AlertSeverityWarning, ServiceGap: svcGapDurationSpec},
			evidence: AlertEvidence{ServiceGap: &ServiceGapEvidence{Service: "web", Desired: 0, MissingFor: 1 * time.Minute}},
			want:     matchTriple{false, AlertSeverityWarning, "service-gap-duration"},
		},
		{
			// Duration branch, GapAfter == 0 short-circuits to false even when
			// MissingFor is positive (covers the `&& spec.ServiceGap.GapAfter > 0`
			// right operand of the conjunction).
			name:     "service gap duration with zero gapAfter returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindServiceGap, Severity: AlertSeverityWarning, ServiceGap: &ServiceGapSpec{Service: "web"}},
			evidence: AlertEvidence{ServiceGap: &ServiceGapEvidence{Service: "web", Desired: 0, MissingFor: time.Hour}},
			want:     matchTriple{false, AlertSeverityWarning, "service-gap-duration"},
		},

		// ---- ProviderIncident: provider/code/native-id mismatch + happy ----
		{
			name:     "provider incident provider mismatch returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindProviderIncident, Severity: AlertSeverityWarning, ProviderIncident: providerSpec},
			evidence: AlertEvidence{ProviderIncident: &ProviderIncidentEvidence{Provider: "gcp", Code: "aws_ec2_degraded"}},
			want:     matchTriple{false, "", "provider-mismatch"},
		},
		{
			name:     "provider incident code mismatch returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindProviderIncident, Severity: AlertSeverityWarning, ProviderIncident: providerSpec},
			evidence: AlertEvidence{ProviderIncident: &ProviderIncidentEvidence{Provider: "aws", Code: "other_code", NativeID: "inc-1"}},
			want:     matchTriple{false, "", "provider-code-mismatch"},
		},
		{
			name:     "provider incident native id mismatch returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindProviderIncident, Severity: AlertSeverityWarning, ProviderIncident: providerSpec},
			evidence: AlertEvidence{ProviderIncident: &ProviderIncidentEvidence{Provider: "aws", Code: "aws_ec2_degraded", NativeID: "other"}},
			want:     matchTriple{false, "", "provider-native-id-mismatch"},
		},
		{
			// Provider matches, code is in the allowed set, no native-id filter
			// restriction violated -> incident reported at spec severity.
			name:     "provider incident happy path returns true",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindProviderIncident, Severity: AlertSeverityCritical, ProviderIncident: providerSpec},
			evidence: AlertEvidence{ProviderIncident: &ProviderIncidentEvidence{Provider: "aws", Code: "aws_ec2_degraded", NativeID: "inc-1"}},
			want:     matchTriple{true, AlertSeverityCritical, "provider-incident"},
		},
		{
			// No Codes filter and no NativeIDs filter: only the provider needs to
			// match (both `len(...) > 0` guards are false).
			name:     "provider incident no filters matches on provider only",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindProviderIncident, Severity: AlertSeverityCritical, ProviderIncident: &ProviderIncidentSpec{Provider: "aws", NativeIDs: []string{"inc-1"}}},
			evidence: AlertEvidence{ProviderIncident: &ProviderIncidentEvidence{Provider: "aws", NativeID: "inc-1", Code: "anything"}},
			want:     matchTriple{true, AlertSeverityCritical, "provider-incident"},
		},

		// ---- ResourceIncidentRollup: count/code mismatch + happy ----
		{
			name:     "resource incident rollup zero count returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindResourceIncidentRollup, Severity: AlertSeverityWarning, ResourceIncidentRollup: rollupSpec},
			evidence: AlertEvidence{ResourceIncidentRollup: &ResourceIncidentRollupEvidence{Code: "cap_low", IncidentCount: 0}},
			want:     matchTriple{false, AlertSeverityWarning, "resource-incident-rollup"},
		},
		{
			name:     "resource incident rollup code mismatch returns false",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindResourceIncidentRollup, Severity: AlertSeverityWarning, ResourceIncidentRollup: rollupSpec},
			evidence: AlertEvidence{ResourceIncidentRollup: &ResourceIncidentRollupEvidence{Code: "other", IncidentCount: 5}},
			want:     matchTriple{false, AlertSeverityWarning, "resource-incident-rollup"},
		},
		{
			name:     "resource incident rollup happy path returns true",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindResourceIncidentRollup, Severity: AlertSeverityCritical, ResourceIncidentRollup: rollupSpec},
			evidence: AlertEvidence{ResourceIncidentRollup: &ResourceIncidentRollupEvidence{Code: "cap_low", IncidentCount: 2}},
			want:     matchTriple{true, AlertSeverityCritical, "resource-incident-rollup"},
		},

		// ---- default / unknown kind ----
		{
			name:     "unknown kind returns false empty",
			spec:     ResourceAlertSpec{Kind: AlertSpecKind("nope"), Severity: AlertSeverityWarning},
			evidence: AlertEvidence{},
			want:     matchTriple{false, "", ""},
		},
		{
			// MetricThreshold is intentionally NOT a case in matches() (it is
			// dispatched to evaluateMetricThreshold by Evaluate, never matches()),
			// so it must fall through to the default arm and return false.
			name:     "metric threshold kind falls through to default",
			spec:     ResourceAlertSpec{Kind: AlertSpecKindMetricThreshold, Severity: AlertSeverityWarning},
			evidence: AlertEvidence{MetricThreshold: &MetricThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 95, Trigger: 80}},
			want:     matchTriple{false, "", ""},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			matched, severity, reason := matches(tc.spec, tc.evidence)
			assertMatchTriple(t, matchTriple{matched, severity, reason}, tc.want)
		})
	}
}

// TestBranchcov0724pmMatchesSeverityThreshold covers every branch of
// matchesSeverityThreshold (evaluator.go:518). The existing suite only reaches
// the ThresholdDirectionAbove arm (critical/warning/normal) and only the metric-
// and direction-matching path; this adds the direction/metric mismatch early
// return, the entire ThresholdDirectionBelow arm, and the unknown-direction
// default arm.
func TestBranchcov0724pmMatchesSeverityThreshold(t *testing.T) {
	spec := SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Warning: 80, Critical: 90}

	cases := []struct {
		name     string
		spec     SeverityThresholdSpec
		evidence SeverityThresholdEvidence
		want     matchTriple
	}{
		// Direction mismatch short-circuits before any threshold comparison.
		{"direction mismatch returns false empty",
			spec, SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionBelow, Observed: 95}, matchTriple{false, "", ""}},
		// Metric mismatch (direction equal) short-circuits via the || operand.
		{"metric mismatch returns false empty",
			spec, SeverityThresholdEvidence{Metric: "mem", Direction: ThresholdDirectionAbove, Observed: 95}, matchTriple{false, "", ""}},

		// ---- Above direction: at / above critical, warning, normal ----
		{"above at critical boundary",
			spec, SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 90}, matchTriple{true, AlertSeverityCritical, "severity-threshold-critical"}},
		{"above above critical",
			spec, SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 99}, matchTriple{true, AlertSeverityCritical, "severity-threshold-critical"}},
		{"above just below critical hits warning",
			spec, SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 89}, matchTriple{true, AlertSeverityWarning, "severity-threshold-warning"}},
		{"above at warning boundary",
			spec, SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 80}, matchTriple{true, AlertSeverityWarning, "severity-threshold-warning"}},
		{"above below warning is normal",
			spec, SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 79}, matchTriple{false, "", "severity-threshold-normal"}},
		// Critical threshold disabled (<=0) -> only warning is consulted.
		{"above with no critical warning only",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Warning: 80},
			SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 85}, matchTriple{true, AlertSeverityWarning, "severity-threshold-warning"}},
		// Both thresholds disabled (<=0) -> default normal within Above.
		{"above with no thresholds is normal",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove},
			SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 999}, matchTriple{false, "", "severity-threshold-normal"}},

		// ---- Below direction: at / below critical, warning, normal ----
		{"below at critical boundary",
			SeverityThresholdSpec{Metric: "disk", Direction: ThresholdDirectionBelow, Warning: 20, Critical: 10},
			SeverityThresholdEvidence{Metric: "disk", Direction: ThresholdDirectionBelow, Observed: 10}, matchTriple{true, AlertSeverityCritical, "severity-threshold-critical"}},
		{"below under critical",
			SeverityThresholdSpec{Metric: "disk", Direction: ThresholdDirectionBelow, Warning: 20, Critical: 10},
			SeverityThresholdEvidence{Metric: "disk", Direction: ThresholdDirectionBelow, Observed: 5}, matchTriple{true, AlertSeverityCritical, "severity-threshold-critical"}},
		{"below just above critical hits warning",
			SeverityThresholdSpec{Metric: "disk", Direction: ThresholdDirectionBelow, Warning: 20, Critical: 10},
			SeverityThresholdEvidence{Metric: "disk", Direction: ThresholdDirectionBelow, Observed: 11}, matchTriple{true, AlertSeverityWarning, "severity-threshold-warning"}},
		{"below at warning boundary",
			SeverityThresholdSpec{Metric: "disk", Direction: ThresholdDirectionBelow, Warning: 20, Critical: 10},
			SeverityThresholdEvidence{Metric: "disk", Direction: ThresholdDirectionBelow, Observed: 20}, matchTriple{true, AlertSeverityWarning, "severity-threshold-warning"}},
		{"below above warning is normal",
			SeverityThresholdSpec{Metric: "disk", Direction: ThresholdDirectionBelow, Warning: 20, Critical: 10},
			SeverityThresholdEvidence{Metric: "disk", Direction: ThresholdDirectionBelow, Observed: 21}, matchTriple{false, "", "severity-threshold-normal"}},
		// Below with no critical -> warning only.
		{"below with no critical warning only",
			SeverityThresholdSpec{Metric: "disk", Direction: ThresholdDirectionBelow, Warning: 20},
			SeverityThresholdEvidence{Metric: "disk", Direction: ThresholdDirectionBelow, Observed: 15}, matchTriple{true, AlertSeverityWarning, "severity-threshold-warning"}},

		// ---- Unknown direction default arm ----
		{"unknown direction returns false empty",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirection("sideways"), Warning: 80, Critical: 90},
			SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirection("sideways"), Observed: 95}, matchTriple{false, "", ""}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			matched, severity, reason := matchesSeverityThreshold(tc.spec, tc.evidence)
			assertMatchTriple(t, matchTriple{matched, severity, reason}, tc.want)
		})
	}
}

// TestBranchcov0724pmSeverityThresholdStillLatched covers every branch of
// severityThresholdStillLatched (evaluator.go:547). The existing suite only
// reaches the Above arm of the switch; this adds the nil-Recovery early return,
// the direction/metric mismatch return, the Below arm (latched and unlatched),
// and the unknown-direction default arm.
func TestBranchcov0724pmSeverityThresholdStillLatched(t *testing.T) {
	aboveRecovery := 85.0
	belowRecovery := 15.0

	cases := []struct {
		name     string
		spec     SeverityThresholdSpec
		evidence SeverityThresholdEvidence
		want     bool
	}{
		// nil Recovery -> immediate false (no hysteresis configured).
		{"nil recovery returns false",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Warning: 80, Critical: 90},
			SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 88}, false},

		// Direction mismatch -> false.
		{"direction mismatch returns false",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Warning: 80, Critical: 90, Recovery: &aboveRecovery},
			SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionBelow, Observed: 88}, false},
		// Metric mismatch -> false.
		{"metric mismatch returns false",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Warning: 80, Critical: 90, Recovery: &aboveRecovery},
			SeverityThresholdEvidence{Metric: "mem", Direction: ThresholdDirectionAbove, Observed: 88}, false},

		// Above: observed >= recovery latches.
		{"above observed at recovery latches",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Warning: 80, Critical: 90, Recovery: &aboveRecovery},
			SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 85}, true},
		{"above observed above recovery latches",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Warning: 80, Critical: 90, Recovery: &aboveRecovery},
			SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 88}, true},
		{"above observed below recovery unlatches",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirectionAbove, Warning: 80, Critical: 90, Recovery: &aboveRecovery},
			SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirectionAbove, Observed: 84}, false},

		// Below: observed <= recovery latches.
		{"below observed at recovery latches",
			SeverityThresholdSpec{Metric: "disk", Direction: ThresholdDirectionBelow, Warning: 20, Critical: 10, Recovery: &belowRecovery},
			SeverityThresholdEvidence{Metric: "disk", Direction: ThresholdDirectionBelow, Observed: 15}, true},
		{"below observed under recovery latches",
			SeverityThresholdSpec{Metric: "disk", Direction: ThresholdDirectionBelow, Warning: 20, Critical: 10, Recovery: &belowRecovery},
			SeverityThresholdEvidence{Metric: "disk", Direction: ThresholdDirectionBelow, Observed: 12}, true},
		{"below observed above recovery unlatches",
			SeverityThresholdSpec{Metric: "disk", Direction: ThresholdDirectionBelow, Warning: 20, Critical: 10, Recovery: &belowRecovery},
			SeverityThresholdEvidence{Metric: "disk", Direction: ThresholdDirectionBelow, Observed: 16}, false},

		// Unknown direction -> default false.
		{"unknown direction returns false",
			SeverityThresholdSpec{Metric: "cpu", Direction: ThresholdDirection("sideways"), Warning: 80, Critical: 90, Recovery: &aboveRecovery},
			SeverityThresholdEvidence{Metric: "cpu", Direction: ThresholdDirection("sideways"), Observed: 88}, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := severityThresholdStillLatched(tc.spec, tc.evidence); got != tc.want {
				t.Fatalf("severityThresholdStillLatched() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBranchcov0724pmMatchesChangeThreshold covers every return arm of
// matchesChangeThreshold (evaluator.go:565). The existing suite reaches the
// warning-current and growth-critical arms; this adds the metric mismatch, the
// critical-current arm, the nil/non-positive PreviousObserved early normal, the
// growth-warning arm, the percent-gate-skip arms (CriticalPercent/WarningPercent
// <= 0), the percent-not-met fall-throughs, and the final normal return.
func TestBranchcov0724pmMatchesChangeThreshold(t *testing.T) {
	fullSpec := ChangeThresholdSpec{
		Metric:          "spam",
		WarningCurrent:  2000,
		CriticalCurrent: 5000,
		WarningDelta:    250,
		CriticalDelta:   500,
		WarningPercent:  25,
		CriticalPercent: 50,
	}
	prev1000 := 1000.0
	prev2000 := 2000.0
	prev100 := 100.0
	prev0 := 0.0

	cases := []struct {
		name     string
		spec     ChangeThresholdSpec
		evidence ChangeThresholdEvidence
		want     matchTriple
	}{
		// Metric mismatch short-circuits.
		{"metric mismatch returns false empty",
			fullSpec, ChangeThresholdEvidence{Metric: "virus", Observed: 2500}, matchTriple{false, "", ""}},

		// Absolute current thresholds.
		{"critical current at boundary",
			fullSpec, ChangeThresholdEvidence{Metric: "spam", Observed: 5000}, matchTriple{true, AlertSeverityCritical, "change-threshold-current-critical"}},
		{"warning current at boundary",
			fullSpec, ChangeThresholdEvidence{Metric: "spam", Observed: 2000}, matchTriple{true, AlertSeverityWarning, "change-threshold-current-warning"}},

		// PreviousObserved absent or non-positive -> normal (both || operands).
		// Observed is kept below the current thresholds so the current checks do
		// not short-circuit before the PreviousObserved guard runs.
		{"nil previous observed returns normal",
			fullSpec, ChangeThresholdEvidence{Metric: "spam", Observed: 100}, matchTriple{false, "", "change-threshold-normal"}},
		{"zero previous observed returns normal",
			fullSpec, ChangeThresholdEvidence{Metric: "spam", Observed: 100, PreviousObserved: &prev0}, matchTriple{false, "", "change-threshold-normal"}},
		{"negative previous observed returns normal",
			fullSpec, ChangeThresholdEvidence{Metric: "spam", Observed: 100, PreviousObserved: &[]float64{-5}[0]}, matchTriple{false, "", "change-threshold-normal"}},

		// Growth delta + percent met.
		{"growth critical delta and percent met",
			fullSpec, ChangeThresholdEvidence{Metric: "spam", Observed: 1600, PreviousObserved: &prev1000}, matchTriple{true, AlertSeverityCritical, "change-threshold-growth-critical"}},
		{"growth warning delta and percent met",
			fullSpec, ChangeThresholdEvidence{Metric: "spam", Observed: 1300, PreviousObserved: &prev1000}, matchTriple{true, AlertSeverityWarning, "change-threshold-growth-warning"}},

		// Delta met but percent gate not met -> falls through to next check / normal.
		// Uses dedicated specs with no current thresholds so the current guards
		// cannot mask the percent-gate behaviour.
		{"critical delta met but percent below gate falls to warning",
			ChangeThresholdSpec{Metric: "spam", WarningDelta: 250, CriticalDelta: 500, WarningPercent: 25, CriticalPercent: 50},
			ChangeThresholdEvidence{Metric: "spam", Observed: 2500, PreviousObserved: &prev2000}, matchTriple{true, AlertSeverityWarning, "change-threshold-growth-warning"}},
		{"warning delta met but percent below gate falls to normal",
			ChangeThresholdSpec{Metric: "spam", WarningDelta: 250, WarningPercent: 25},
			ChangeThresholdEvidence{Metric: "spam", Observed: 2250, PreviousObserved: &prev2000}, matchTriple{false, "", "change-threshold-normal"}},
		{"no delta met returns normal",
			fullSpec, ChangeThresholdEvidence{Metric: "spam", Observed: 1050, PreviousObserved: &prev1000}, matchTriple{false, "", "change-threshold-normal"}},

		// Percent gate disabled (<=0) -> any delta meeting the threshold fires.
		{"critical with no percent gate fires on delta only",
			ChangeThresholdSpec{Metric: "spam", CriticalDelta: 500, CriticalPercent: 0},
			ChangeThresholdEvidence{Metric: "spam", Observed: 600, PreviousObserved: &prev100}, matchTriple{true, AlertSeverityCritical, "change-threshold-growth-critical"}},
		{"warning with no percent gate fires on delta only",
			ChangeThresholdSpec{Metric: "spam", WarningDelta: 50, WarningPercent: 0},
			ChangeThresholdEvidence{Metric: "spam", Observed: 200, PreviousObserved: &prev100}, matchTriple{true, AlertSeverityWarning, "change-threshold-growth-warning"}},

		// Small previous observed to exercise large percent swings without
		// tripping any current threshold.
		{"large percent growth but delta under thresholds returns normal",
			fullSpec, ChangeThresholdEvidence{Metric: "spam", Observed: 130, PreviousObserved: &prev100}, matchTriple{false, "", "change-threshold-normal"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			matched, severity, reason := matchesChangeThreshold(tc.spec, tc.evidence)
			assertMatchTriple(t, matchTriple{matched, severity, reason}, tc.want)
		})
	}
}

// TestBranchcov0724pmMatchesBaselineAnomaly covers every return arm of
// matchesBaselineAnomaly (evaluator.go:598). The existing suite reaches the
// quiet-critical and normal-baseline critical arms; this adds the metric
// mismatch, the baseline==0 clamp, the quiet-warning and quiet-normal arms, the
// baseline<=0 arm (reachable only when QuietBaseline==0), and the normal-baseline
// warning and normal arms.
func TestBranchcov0724pmMatchesBaselineAnomaly(t *testing.T) {
	spec := BaselineAnomalySpec{
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
		name     string
		spec     BaselineAnomalySpec
		evidence BaselineAnomalyEvidence
		want     matchTriple
	}{
		// Metric mismatch short-circuits.
		{"metric mismatch returns false empty",
			spec, BaselineAnomalyEvidence{Metric: "virus", Observed: 420, Baseline: 100}, matchTriple{false, "", ""}},

		// ---- Quiet site (baseline < QuietBaseline) ----
		{"quiet critical delta",
			spec, BaselineAnomalyEvidence{Metric: "spamIn", Observed: 140, Baseline: 10}, matchTriple{true, AlertSeverityCritical, "baseline-anomaly-quiet-critical"}},
		{"quiet warning delta",
			spec, BaselineAnomalyEvidence{Metric: "spamIn", Observed: 80, Baseline: 10}, matchTriple{true, AlertSeverityWarning, "baseline-anomaly-quiet-warning"}},
		{"quiet below warning delta is normal",
			spec, BaselineAnomalyEvidence{Metric: "spamIn", Observed: 50, Baseline: 10}, matchTriple{false, "", "baseline-anomaly-normal"}},

		// baseline == 0 with observed > 0 is clamped to 1, then treated as quiet.
		// delta = observed - 1; pick observed so delta meets quiet warning.
		{"baseline zero clamps to one and enters quiet branch",
			spec, BaselineAnomalyEvidence{Metric: "spamIn", Observed: 61, Baseline: 0}, matchTriple{true, AlertSeverityWarning, "baseline-anomaly-quiet-warning"}},

		// ---- Normal site (baseline >= QuietBaseline) ----
		{"normal critical ratio and delta",
			spec, BaselineAnomalyEvidence{Metric: "spamIn", Observed: 420, Baseline: 100}, matchTriple{true, AlertSeverityCritical, "baseline-anomaly-critical"}},
		{"normal warning ratio and delta",
			spec, BaselineAnomalyEvidence{Metric: "spamIn", Observed: 250, Baseline: 100}, matchTriple{true, AlertSeverityWarning, "baseline-anomaly-warning"}},
		{"normal ratio below thresholds is normal",
			spec, BaselineAnomalyEvidence{Metric: "spamIn", Observed: 150, Baseline: 100}, matchTriple{false, "", "baseline-anomaly-normal"}},
		// Ratio meets critical but delta below critical; ratio also meets warning
		// but delta below warning -> normal (proves the `&& delta >=` is required).
		{"ratio high but delta below thresholds is normal",
			spec, BaselineAnomalyEvidence{Metric: "spamIn", Observed: 240, Baseline: 100}, matchTriple{false, "", "baseline-anomaly-normal"}},

		// QuietBaseline == 0 with baseline 0 and observed 0: the quiet branch is
		// skipped (0 < 0 is false) and the `baseline <= 0` arm fires.
		{"zero baseline zero observed with zero quiet threshold returns normal",
			BaselineAnomalySpec{Metric: "spamIn", QuietBaseline: 0, QuietWarningDelta: 60, QuietCriticalDelta: 120, WarningDelta: 150, CriticalDelta: 300, WarningRatio: 1.8, CriticalRatio: 2.5},
			BaselineAnomalyEvidence{Metric: "spamIn", Observed: 0, Baseline: 0}, matchTriple{false, "", "baseline-anomaly-normal"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			matched, severity, reason := matchesBaselineAnomaly(tc.spec, tc.evidence)
			assertMatchTriple(t, matchTriple{matched, severity, reason}, tc.want)
		})
	}
}

// TestBranchcov0724pmMatchesHealthAssessment covers every return arm of
// matchesHealthAssessment (evaluator.go:636). The existing suite reaches the
// match arm and the empty-codes/severity normal arm; this adds the signal-
// mismatch arm, the empty-spec-codes wildcard match arm, and the no-overlap
// normal arm.
func TestBranchcov0724pmMatchesHealthAssessment(t *testing.T) {
	spec := HealthAssessmentSpec{Signal: "host-raid", Codes: []string{"raid_degraded", "raid_rebuilding"}}

	cases := []struct {
		name     string
		spec     HealthAssessmentSpec
		evidence HealthAssessmentEvidence
		want     matchTriple
	}{
		// Signal mismatch short-circuits with its own reason.
		{"signal mismatch returns false with mismatch reason",
			spec, HealthAssessmentEvidence{Signal: "host-disk", Severity: AlertSeverityWarning, Codes: []string{"raid_degraded"}},
			matchTriple{false, "", "health-assessment-signal-mismatch"}},

		// Empty observed codes -> normal.
		{"empty evidence codes returns normal",
			spec, HealthAssessmentEvidence{Signal: "host-raid", Severity: AlertSeverityWarning}, matchTriple{false, "", "health-assessment-normal"}},
		// Codes present but severity empty -> normal. (Reachable by direct call;
		// Evaluate's evidence validation forbids codes-without-severity, so this
		// arm is otherwise unreachable through the public path.)
		{"codes present but empty severity returns normal",
			spec, HealthAssessmentEvidence{Signal: "host-raid", Codes: []string{"raid_degraded"}}, matchTriple{false, "", "health-assessment-normal"}},

		// Spec with no code filter -> any evidence code/severity matches.
		{"empty spec codes matches any evidence severity",
			HealthAssessmentSpec{Signal: "host-raid"},
			HealthAssessmentEvidence{Signal: "host-raid", Severity: AlertSeverityCritical, Codes: []string{"anything"}}, matchTriple{true, AlertSeverityCritical, "health-assessment-match"}},

		// Overlap present -> match at evidence severity.
		{"observed code in spec set matches",
			spec, HealthAssessmentEvidence{Signal: "host-raid", Severity: AlertSeverityWarning, Codes: []string{"raid_rebuilding"}}, matchTriple{true, AlertSeverityWarning, "health-assessment-match"}},
		// First observed code misses but a later one hits -> still matches.
		{"later observed code in spec set matches",
			spec, HealthAssessmentEvidence{Signal: "host-raid", Severity: AlertSeverityCritical, Codes: []string{"raid_unknown", "raid_degraded"}}, matchTriple{true, AlertSeverityCritical, "health-assessment-match"}},
		// No overlap -> normal.
		{"no observed code in spec set returns normal",
			spec, HealthAssessmentEvidence{Signal: "host-raid", Severity: AlertSeverityWarning, Codes: []string{"disk_worn", "psu_fail"}}, matchTriple{false, "", "health-assessment-normal"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			matched, severity, reason := matchesHealthAssessment(tc.spec, tc.evidence)
			assertMatchTriple(t, matchTriple{matched, severity, reason}, tc.want)
		})
	}
}

// TestBranchcov0724pmMatchesPostureThreshold covers every return arm of
// matchesPostureThreshold (evaluator.go:658). The existing suite reaches the
// size-critical and age-warning arms; this adds the age/size metric-mismatch
// returns, the size-missing return, every severity combination in the final
// switch (age+size critical, age-only critical, age+size warning, size-only
// warning, default normal), and the per-dimension disabled-metric guards.
func TestBranchcov0724pmMatchesPostureThreshold(t *testing.T) {
	fullSpec := PostureThresholdSpec{AgeMetric: "snapshot-age-days", WarningAge: 7, CriticalAge: 14, SizeMetric: "snapshot-size-gib", WarningSize: 50, CriticalSize: 100}
	small := 20.0
	large := 120.0
	mid := 60.0

	cases := []struct {
		name     string
		spec     PostureThresholdSpec
		evidence PostureThresholdEvidence
		want     matchTriple
	}{
		// ---- Metric mismatches (each dimension independently) ----
		{"age metric mismatch returns false",
			fullSpec, PostureThresholdEvidence{AgeMetric: "wrong-age", AgeValue: 20, SizeMetric: "snapshot-size-gib", SizeValue: &small},
			matchTriple{false, "", "posture-threshold-age-metric-mismatch"}},
		{"size metric mismatch returns false",
			fullSpec, PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 20, SizeMetric: "wrong-size", SizeValue: &small},
			matchTriple{false, "", "posture-threshold-size-metric-mismatch"}},
		{"size metric set but size value nil returns false",
			fullSpec, PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 20, SizeMetric: "snapshot-size-gib", SizeValue: nil},
			matchTriple{false, "", "posture-threshold-size-missing"}},

		// ---- Final switch: every severity combination ----
		{"age and size both critical",
			fullSpec, PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 20, SizeMetric: "snapshot-size-gib", SizeValue: &large},
			matchTriple{true, AlertSeverityCritical, "posture-threshold-critical"}},
		{"age critical size not",
			fullSpec, PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 20, SizeMetric: "snapshot-size-gib", SizeValue: &small},
			matchTriple{true, AlertSeverityCritical, "posture-threshold-age-critical"}},
		{"size critical age not",
			fullSpec, PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 2, SizeMetric: "snapshot-size-gib", SizeValue: &large},
			matchTriple{true, AlertSeverityCritical, "posture-threshold-size-critical"}},
		{"age and size both warning",
			fullSpec, PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 10, SizeMetric: "snapshot-size-gib", SizeValue: &mid},
			matchTriple{true, AlertSeverityWarning, "posture-threshold-warning"}},
		{"age warning size not",
			fullSpec, PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 10, SizeMetric: "snapshot-size-gib", SizeValue: &small},
			matchTriple{true, AlertSeverityWarning, "posture-threshold-age-warning"}},
		{"size warning age not",
			fullSpec, PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 2, SizeMetric: "snapshot-size-gib", SizeValue: &mid},
			matchTriple{true, AlertSeverityWarning, "posture-threshold-size-warning"}},
		{"neither age nor size breach is normal",
			fullSpec, PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 2, SizeMetric: "snapshot-size-gib", SizeValue: &small},
			matchTriple{false, "", "posture-threshold-normal"}},

		// ---- Per-dimension disabled guards ----
		// Age metric empty -> age dimension skipped entirely; only size evaluated.
		{"only size dimension configured size warning",
			PostureThresholdSpec{SizeMetric: "snapshot-size-gib", WarningSize: 50, CriticalSize: 100},
			PostureThresholdEvidence{SizeMetric: "snapshot-size-gib", SizeValue: &mid}, matchTriple{true, AlertSeverityWarning, "posture-threshold-size-warning"}},
		// Size metric empty -> size dimension skipped; only age evaluated.
		{"only age dimension configured age critical",
			PostureThresholdSpec{AgeMetric: "snapshot-age-days", WarningAge: 7, CriticalAge: 14},
			PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 20}, matchTriple{true, AlertSeverityCritical, "posture-threshold-age-critical"}},
		// Size metric empty -> the size branch is not entered, so a nil SizeValue
		// must NOT trigger the size-missing return.
		{"no size metric tolerates nil size value",
			PostureThresholdSpec{AgeMetric: "snapshot-age-days", WarningAge: 7, CriticalAge: 14},
			PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 3, SizeValue: nil}, matchTriple{false, "", "posture-threshold-normal"}},
		// Critical threshold disabled (<=0) -> warning is consulted for that dim.
		{"age warning when critical age disabled",
			PostureThresholdSpec{AgeMetric: "snapshot-age-days", WarningAge: 7},
			PostureThresholdEvidence{AgeMetric: "snapshot-age-days", AgeValue: 10}, matchTriple{true, AlertSeverityWarning, "posture-threshold-age-warning"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			matched, severity, reason := matchesPostureThreshold(tc.spec, tc.evidence)
			assertMatchTriple(t, matchTriple{matched, severity, reason}, tc.want)
		})
	}
}
