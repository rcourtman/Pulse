package specs

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestEvaluateMetricThresholdTriggerClearAndReevaluation(t *testing.T) {
	recovery := 75.0
	spec := ResourceAlertSpec{
		ID:           "vm-101-cpu",
		ResourceID:   "pve1:node1:101",
		ResourceType: unifiedresources.ResourceTypeVM,
		Kind:         AlertSpecKindMetricThreshold,
		Severity:     AlertSeverityWarning,
		MetricThreshold: &MetricThresholdSpec{
			Metric:    "cpu",
			Direction: ThresholdDirectionAbove,
			Trigger:   80,
			Recovery:  &recovery,
		},
	}

	triggered, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC),
		MetricThreshold: &MetricThresholdEvidence{
			Metric:    "cpu",
			Direction: ThresholdDirectionAbove,
			Observed:  85,
			Trigger:   80,
			Recovery:  &recovery,
		},
	})
	if err != nil {
		t.Fatalf("trigger evaluation failed: %v", err)
	}
	if triggered.State.State != AlertStateFiring {
		t.Fatalf("triggered state = %q, want firing", triggered.State.State)
	}
	if triggered.Transition == nil || triggered.Transition.Kind != EvaluationTransitionActivated {
		t.Fatalf("trigger transition = %+v, want activated", triggered.Transition)
	}

	latched, err := Evaluate(spec, triggered.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 0, 5, 0, time.UTC),
		MetricThreshold: &MetricThresholdEvidence{
			Metric:    "cpu",
			Direction: ThresholdDirectionAbove,
			Observed:  77,
			Trigger:   80,
			Recovery:  &recovery,
		},
	})
	if err != nil {
		t.Fatalf("latched evaluation failed: %v", err)
	}
	if latched.State.State != AlertStateFiring {
		t.Fatalf("latched state = %q, want firing", latched.State.State)
	}
	if latched.Transition != nil {
		t.Fatalf("latched transition = %+v, want nil", latched.Transition)
	}

	cleared, err := Evaluate(spec, latched.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 0, 10, 0, time.UTC),
		MetricThreshold: &MetricThresholdEvidence{
			Metric:    "cpu",
			Direction: ThresholdDirectionAbove,
			Observed:  74,
			Trigger:   80,
			Recovery:  &recovery,
		},
	})
	if err != nil {
		t.Fatalf("clear evaluation failed: %v", err)
	}
	if cleared.State.State != AlertStateClear {
		t.Fatalf("cleared state = %q, want clear", cleared.State.State)
	}
	if cleared.Transition == nil || cleared.Transition.Kind != EvaluationTransitionRecovered {
		t.Fatalf("clear transition = %+v, want recovered", cleared.Transition)
	}

	updated := spec
	updated.MetricThreshold.Trigger = 90
	revaluated, err := Evaluate(updated, triggered.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 0, 15, 0, time.UTC),
		MetricThreshold: &MetricThresholdEvidence{
			Metric:    "cpu",
			Direction: ThresholdDirectionAbove,
			Observed:  85,
			Trigger:   90,
			Recovery:  &recovery,
		},
	})
	if err != nil {
		t.Fatalf("reevaluation failed: %v", err)
	}
	if revaluated.State.State != AlertStateClear {
		t.Fatalf("reevaluated state = %q, want clear", revaluated.State.State)
	}
	if revaluated.Transition == nil || revaluated.Transition.Kind != EvaluationTransitionReevaluated {
		t.Fatalf("reevaluated transition = %+v, want reevaluated", revaluated.Transition)
	}
}

func TestEvaluateSeverityThresholdEscalationAndRecovery(t *testing.T) {
	spec := ResourceAlertSpec{
		ID:           "pmg-queue-total",
		ResourceID:   "pmg-1",
		ResourceType: unifiedresources.ResourceTypePMG,
		Kind:         AlertSpecKindSeverityThreshold,
		Severity:     AlertSeverityWarning,
		SeverityThreshold: &SeverityThresholdSpec{
			Metric:    "queue-total",
			Direction: ThresholdDirectionAbove,
			Warning:   500,
			Critical:  1000,
		},
	}

	warning, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 30, 0, 0, time.UTC),
		SeverityThreshold: &SeverityThresholdEvidence{
			Metric:    "queue-total",
			Direction: ThresholdDirectionAbove,
			Observed:  700,
		},
	})
	if err != nil {
		t.Fatalf("warning evaluation failed: %v", err)
	}
	if warning.State.State != AlertStateFiring || warning.State.Severity != AlertSeverityWarning {
		t.Fatalf("warning state = %+v, want firing warning", warning.State)
	}
	if warning.Transition == nil || warning.Transition.Kind != EvaluationTransitionActivated {
		t.Fatalf("warning transition = %+v, want activated", warning.Transition)
	}

	critical, err := Evaluate(spec, warning.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 31, 0, 0, time.UTC),
		SeverityThreshold: &SeverityThresholdEvidence{
			Metric:    "queue-total",
			Direction: ThresholdDirectionAbove,
			Observed:  1200,
		},
	})
	if err != nil {
		t.Fatalf("critical evaluation failed: %v", err)
	}
	if critical.State.Severity != AlertSeverityCritical {
		t.Fatalf("critical severity = %q, want critical", critical.State.Severity)
	}
	if critical.Transition == nil || critical.Transition.Kind != EvaluationTransitionSeverityChanged {
		t.Fatalf("critical transition = %+v, want severity-changed", critical.Transition)
	}

	recovered, err := Evaluate(spec, critical.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 32, 0, 0, time.UTC),
		SeverityThreshold: &SeverityThresholdEvidence{
			Metric:    "queue-total",
			Direction: ThresholdDirectionAbove,
			Observed:  200,
		},
	})
	if err != nil {
		t.Fatalf("recovery evaluation failed: %v", err)
	}
	if recovered.State.State != AlertStateClear {
		t.Fatalf("recovered state = %q, want clear", recovered.State.State)
	}
	if recovered.Transition == nil || recovered.Transition.Kind != EvaluationTransitionRecovered {
		t.Fatalf("recovered transition = %+v, want recovered", recovered.Transition)
	}
}

func TestEvaluateSeverityThresholdHysteresisLatch(t *testing.T) {
	recovery := 85.0
	spec := ResourceAlertSpec{
		ID:           "docker:host/container-memory-limit",
		ResourceID:   "docker:host/container",
		ResourceType: unifiedresources.ResourceTypeAppContainer,
		Kind:         AlertSpecKindSeverityThreshold,
		Severity:     AlertSeverityWarning,
		SeverityThreshold: &SeverityThresholdSpec{
			Metric:    "memory-limit-percent",
			Direction: ThresholdDirectionAbove,
			Warning:   90,
			Critical:  95,
			Recovery:  &recovery,
		},
	}

	critical, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 40, 0, 0, time.UTC),
		SeverityThreshold: &SeverityThresholdEvidence{
			Metric:    "memory-limit-percent",
			Direction: ThresholdDirectionAbove,
			Observed:  96,
		},
	})
	if err != nil {
		t.Fatalf("critical evaluation failed: %v", err)
	}
	if critical.State.Severity != AlertSeverityCritical {
		t.Fatalf("critical severity = %q, want critical", critical.State.Severity)
	}

	latched, err := Evaluate(spec, critical.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 41, 0, 0, time.UTC),
		SeverityThreshold: &SeverityThresholdEvidence{
			Metric:    "memory-limit-percent",
			Direction: ThresholdDirectionAbove,
			Observed:  88,
		},
	})
	if err != nil {
		t.Fatalf("latched evaluation failed: %v", err)
	}
	if latched.State.State != AlertStateFiring {
		t.Fatalf("latched state = %q, want firing", latched.State.State)
	}
	if latched.State.Severity != AlertSeverityCritical {
		t.Fatalf("latched severity = %q, want critical", latched.State.Severity)
	}
	if latched.Transition != nil {
		t.Fatalf("latched transition = %+v, want nil", latched.Transition)
	}

	cleared, err := Evaluate(spec, latched.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 9, 42, 0, 0, time.UTC),
		SeverityThreshold: &SeverityThresholdEvidence{
			Metric:    "memory-limit-percent",
			Direction: ThresholdDirectionAbove,
			Observed:  84,
		},
	})
	if err != nil {
		t.Fatalf("clear evaluation failed: %v", err)
	}
	if cleared.State.State != AlertStateClear {
		t.Fatalf("cleared state = %q, want clear", cleared.State.State)
	}
	if cleared.Transition == nil || cleared.Transition.Kind != EvaluationTransitionRecovered {
		t.Fatalf("cleared transition = %+v, want recovered", cleared.Transition)
	}
}

func TestEvaluateChangeThresholdAbsoluteAndGrowth(t *testing.T) {
	spec := ResourceAlertSpec{
		ID:           "pmg-quarantine-spam",
		ResourceID:   "pmg-1",
		ResourceType: unifiedresources.ResourceTypePMG,
		Kind:         AlertSpecKindChangeThreshold,
		Severity:     AlertSeverityWarning,
		ChangeThreshold: &ChangeThresholdSpec{
			Metric:          "quarantine-spam",
			ReferenceWindow: 2 * time.Hour,
			WarningCurrent:  2000,
			CriticalCurrent: 5000,
			WarningDelta:    250,
			CriticalDelta:   500,
			WarningPercent:  25,
			CriticalPercent: 50,
		},
	}

	absolute, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC),
		ChangeThreshold: &ChangeThresholdEvidence{
			Metric:   "quarantine-spam",
			Observed: 2500,
		},
	})
	if err != nil {
		t.Fatalf("absolute evaluation failed: %v", err)
	}
	if absolute.State.State != AlertStateFiring || absolute.State.Severity != AlertSeverityWarning {
		t.Fatalf("absolute state = %+v, want firing warning", absolute.State)
	}
	if absolute.State.Reason != "change-threshold-current-warning" {
		t.Fatalf("absolute reason = %q, want current warning", absolute.State.Reason)
	}

	previous := 1000.0
	growth, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 10, 5, 0, 0, time.UTC),
		ChangeThreshold: &ChangeThresholdEvidence{
			Metric:           "quarantine-spam",
			Observed:         1600,
			PreviousObserved: &previous,
		},
	})
	if err != nil {
		t.Fatalf("growth evaluation failed: %v", err)
	}
	if growth.State.State != AlertStateFiring || growth.State.Severity != AlertSeverityCritical {
		t.Fatalf("growth state = %+v, want firing critical", growth.State)
	}
	if growth.State.Reason != "change-threshold-growth-critical" {
		t.Fatalf("growth reason = %q, want growth critical", growth.State.Reason)
	}
}

func TestEvaluateBaselineAnomalyNormalAndQuietSite(t *testing.T) {
	spec := ResourceAlertSpec{
		ID:           "pmg-anomaly-spamIn",
		ResourceID:   "pmg-1",
		ResourceType: unifiedresources.ResourceTypePMG,
		Kind:         AlertSpecKindBaselineAnomaly,
		Severity:     AlertSeverityWarning,
		BaselineAnomaly: &BaselineAnomalySpec{
			Metric:             "spamIn",
			QuietBaseline:      40,
			WarningRatio:       1.8,
			CriticalRatio:      2.5,
			WarningDelta:       150,
			CriticalDelta:      300,
			QuietWarningDelta:  60,
			QuietCriticalDelta: 120,
		},
	}

	normal, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 10, 10, 0, 0, time.UTC),
		BaselineAnomaly: &BaselineAnomalyEvidence{
			Metric:   "spamIn",
			Observed: 420,
			Baseline: 100,
		},
	})
	if err != nil {
		t.Fatalf("normal evaluation failed: %v", err)
	}
	if normal.State.State != AlertStateFiring || normal.State.Severity != AlertSeverityCritical {
		t.Fatalf("normal state = %+v, want firing critical", normal.State)
	}
	if normal.State.Reason != "baseline-anomaly-critical" {
		t.Fatalf("normal reason = %q, want baseline-anomaly-critical", normal.State.Reason)
	}

	quiet, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 10, 11, 0, 0, time.UTC),
		BaselineAnomaly: &BaselineAnomalyEvidence{
			Metric:   "spamIn",
			Observed: 140,
			Baseline: 10,
		},
	})
	if err != nil {
		t.Fatalf("quiet evaluation failed: %v", err)
	}
	if quiet.State.State != AlertStateFiring || quiet.State.Severity != AlertSeverityCritical {
		t.Fatalf("quiet state = %+v, want firing critical", quiet.State)
	}
	if quiet.State.Reason != "baseline-anomaly-quiet-critical" {
		t.Fatalf("quiet reason = %q, want baseline-anomaly-quiet-critical", quiet.State.Reason)
	}
}

func TestEvaluateConnectivityConfirmationAndRecovery(t *testing.T) {
	spec := ResourceAlertSpec{
		ID:           "node-pve1-connectivity",
		ResourceID:   "node/pve-1",
		ResourceType: unifiedresources.ResourceTypeAgent,
		Kind:         AlertSpecKindConnectivity,
		Severity:     AlertSeverityCritical,
		Connectivity: &ConnectivitySpec{
			Signal:    "heartbeat",
			LostAfter: 30 * time.Second,
		},
	}

	first, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC),
		Connectivity: &ConnectivityEvidence{
			Signal:    "heartbeat",
			Connected: false,
		},
	})
	if err != nil {
		t.Fatalf("first evaluation failed: %v", err)
	}
	if first.State.State != AlertStatePending || first.State.ConsecutiveMatches != 1 {
		t.Fatalf("first state = %+v, want pending with one confirmation", first.State)
	}

	second, err := Evaluate(spec, first.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 10, 0, 5, 0, time.UTC),
		Connectivity: &ConnectivityEvidence{
			Signal:    "heartbeat",
			Connected: false,
		},
	})
	if err != nil {
		t.Fatalf("second evaluation failed: %v", err)
	}
	if second.State.State != AlertStatePending || second.State.ConsecutiveMatches != 2 {
		t.Fatalf("second state = %+v, want pending with two confirmations", second.State)
	}

	third, err := Evaluate(spec, second.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 10, 0, 10, 0, time.UTC),
		Connectivity: &ConnectivityEvidence{
			Signal:    "heartbeat",
			Connected: false,
		},
	})
	if err != nil {
		t.Fatalf("third evaluation failed: %v", err)
	}
	if third.State.State != AlertStateFiring {
		t.Fatalf("third state = %q, want firing", third.State.State)
	}
	if third.Transition == nil || third.Transition.Kind != EvaluationTransitionActivated {
		t.Fatalf("third transition = %+v, want activated", third.Transition)
	}

	recovered, err := Evaluate(spec, third.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 10, 0, 15, 0, time.UTC),
		Connectivity: &ConnectivityEvidence{
			Signal:    "heartbeat",
			Connected: true,
		},
	})
	if err != nil {
		t.Fatalf("recovery evaluation failed: %v", err)
	}
	if recovered.State.State != AlertStateClear {
		t.Fatalf("recovered state = %q, want clear", recovered.State.State)
	}
	if recovered.Transition == nil || recovered.Transition.Kind != EvaluationTransitionRecovered {
		t.Fatalf("recovered transition = %+v, want recovered", recovered.Transition)
	}
}

func TestEvaluatePoweredStateSuppressionAndDisable(t *testing.T) {
	spec := ResourceAlertSpec{
		ID:                         "vm-101-powered-off",
		ResourceID:                 "pve1:node1:101",
		ResourceType:               unifiedresources.ResourceTypeVM,
		Kind:                       AlertSpecKindPoweredState,
		Severity:                   AlertSeverityWarning,
		SuppressOnConnectivityLoss: true,
		PoweredState: &PoweredStateSpec{
			Expected: PowerStateOn,
		},
	}

	first, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt:      time.Date(2026, 3, 10, 11, 0, 0, 0, time.UTC),
		ParentConnected: boolPtr(true),
		PoweredState: &PoweredStateEvidence{
			Expected: PowerStateOn,
			Observed: PowerStateOff,
		},
	})
	if err != nil {
		t.Fatalf("first evaluation failed: %v", err)
	}
	if first.State.State != AlertStatePending {
		t.Fatalf("first state = %q, want pending", first.State.State)
	}

	active, err := Evaluate(spec, first.State, AlertEvidence{
		ObservedAt:      time.Date(2026, 3, 10, 11, 0, 5, 0, time.UTC),
		ParentConnected: boolPtr(true),
		PoweredState: &PoweredStateEvidence{
			Expected: PowerStateOn,
			Observed: PowerStateOff,
		},
	})
	if err != nil {
		t.Fatalf("active evaluation failed: %v", err)
	}
	if active.State.State != AlertStateFiring {
		t.Fatalf("active state = %q, want firing", active.State.State)
	}

	suppressed, err := Evaluate(spec, active.State, AlertEvidence{
		ObservedAt:      time.Date(2026, 3, 10, 11, 0, 10, 0, time.UTC),
		ParentConnected: boolPtr(false),
		PoweredState: &PoweredStateEvidence{
			Expected: PowerStateOn,
			Observed: PowerStateOff,
		},
	})
	if err != nil {
		t.Fatalf("suppressed evaluation failed: %v", err)
	}
	if suppressed.State.State != AlertStateSuppressed {
		t.Fatalf("suppressed state = %q, want suppressed", suppressed.State.State)
	}
	if suppressed.Transition == nil || suppressed.Transition.Kind != EvaluationTransitionSuppressed {
		t.Fatalf("suppressed transition = %+v, want suppressed", suppressed.Transition)
	}

	disabledSpec := spec
	disabledSpec.Disabled = true
	disabled, err := Evaluate(disabledSpec, active.State, AlertEvidence{
		ObservedAt:      time.Date(2026, 3, 10, 11, 0, 15, 0, time.UTC),
		ParentConnected: boolPtr(true),
		PoweredState: &PoweredStateEvidence{
			Expected: PowerStateOn,
			Observed: PowerStateOff,
		},
	})
	if err != nil {
		t.Fatalf("disabled evaluation failed: %v", err)
	}
	if disabled.State.State != AlertStateClear {
		t.Fatalf("disabled state = %q, want clear", disabled.State.State)
	}
	if disabled.Transition == nil || disabled.Transition.Kind != EvaluationTransitionDisabled {
		t.Fatalf("disabled transition = %+v, want disabled", disabled.Transition)
	}
}

func TestEvaluateServiceGapSeverityChanges(t *testing.T) {
	spec := ResourceAlertSpec{
		ID:           "docker-service-gap",
		ResourceID:   "docker:host-1/service:web",
		ResourceType: unifiedresources.ResourceTypeDockerService,
		Kind:         AlertSpecKindServiceGap,
		Severity:     AlertSeverityWarning,
		ServiceGap: &ServiceGapSpec{
			Service:         "web",
			WarningPercent:  10,
			CriticalPercent: 50,
		},
	}

	warning, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
		ServiceGap: &ServiceGapEvidence{
			Service: "web",
			Desired: 10,
			Running: 8,
		},
	})
	if err != nil {
		t.Fatalf("warning evaluation failed: %v", err)
	}
	if warning.State.State != AlertStateFiring || warning.State.Severity != AlertSeverityWarning {
		t.Fatalf("warning state = %+v, want firing warning", warning.State)
	}

	critical, err := Evaluate(spec, warning.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 12, 0, 5, 0, time.UTC),
		ServiceGap: &ServiceGapEvidence{
			Service: "web",
			Desired: 10,
			Running: 4,
		},
	})
	if err != nil {
		t.Fatalf("critical evaluation failed: %v", err)
	}
	if critical.State.Severity != AlertSeverityCritical {
		t.Fatalf("critical severity = %q, want critical", critical.State.Severity)
	}
	if critical.Transition == nil || critical.Transition.Kind != EvaluationTransitionSeverityChanged {
		t.Fatalf("critical transition = %+v, want severity-changed", critical.Transition)
	}

	recovered, err := Evaluate(spec, critical.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 12, 0, 10, 0, time.UTC),
		ServiceGap: &ServiceGapEvidence{
			Service: "web",
			Desired: 10,
			Running: 10,
		},
	})
	if err != nil {
		t.Fatalf("recovery evaluation failed: %v", err)
	}
	if recovered.State.State != AlertStateClear {
		t.Fatalf("recovered state = %q, want clear", recovered.State.State)
	}
	if recovered.Transition == nil || recovered.Transition.Kind != EvaluationTransitionRecovered {
		t.Fatalf("recovered transition = %+v, want recovered", recovered.Transition)
	}
}

func TestEvaluateDiscreteStatePreservesStableSpecIdentityAcrossMutableFields(t *testing.T) {
	base := ResourceAlertSpec{
		ID:           "docker-api-runtime-state",
		ResourceID:   "docker:host-1/container:api",
		ResourceType: unifiedresources.ResourceTypeAppContainer,
		Kind:         AlertSpecKindDiscreteState,
		Severity:     AlertSeverityWarning,
		DiscreteState: &DiscreteStateSpec{
			StateKey:      "runtime-state",
			TriggerStates: []string{"paused"},
		},
	}

	initial, err := Evaluate(base, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 13, 0, 0, 0, time.UTC),
		DiscreteState: &DiscreteStateEvidence{
			StateKey: "runtime-state",
			Observed: "paused",
		},
	})
	if err != nil {
		t.Fatalf("initial evaluation failed: %v", err)
	}
	if initial.State.State != AlertStateFiring || initial.State.Severity != AlertSeverityWarning {
		t.Fatalf("initial state = %+v, want firing warning", initial.State)
	}

	mutated := base
	mutated.ConfirmationsRequired = 4
	mutated.SuppressOnConnectivityLoss = true
	mutated.Severity = AlertSeverityCritical

	continued, err := Evaluate(mutated, initial.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 13, 0, 5, 0, time.UTC),
		DiscreteState: &DiscreteStateEvidence{
			StateKey: "runtime-state",
			Observed: "paused",
		},
	})
	if err != nil {
		t.Fatalf("continued evaluation failed: %v", err)
	}
	if continued.State.SpecID != mutated.ID {
		t.Fatalf("continued spec id = %q, want %q", continued.State.SpecID, mutated.ID)
	}
	if continued.State.State != AlertStateFiring || continued.State.Severity != AlertSeverityCritical {
		t.Fatalf("continued state = %+v, want firing critical", continued.State)
	}
	if continued.Transition == nil || continued.Transition.Kind != EvaluationTransitionSeverityChanged {
		t.Fatalf("continued transition = %+v, want severity-changed", continued.Transition)
	}
}

func TestEvaluateHealthAssessmentEscalationAndRecovery(t *testing.T) {
	spec := ResourceAlertSpec{
		ID:           "agent:host1/raid:md2-health",
		ResourceID:   "agent:host1/raid:md2",
		ResourceType: unifiedresources.ResourceTypeAgent,
		Kind:         AlertSpecKindHealthAssessment,
		Severity:     AlertSeverityWarning,
		HealthAssessment: &HealthAssessmentSpec{
			Signal: "host-raid",
			Codes:  []string{"raid_degraded", "raid_rebuilding"},
		},
	}

	warning, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 13, 10, 0, 0, time.UTC),
		HealthAssessment: &HealthAssessmentEvidence{
			Signal:   "host-raid",
			Severity: AlertSeverityWarning,
			Codes:    []string{"raid_rebuilding"},
		},
	})
	if err != nil {
		t.Fatalf("warning evaluation failed: %v", err)
	}
	if warning.State.State != AlertStateFiring || warning.State.Severity != AlertSeverityWarning {
		t.Fatalf("warning state = %+v, want firing warning", warning.State)
	}

	critical, err := Evaluate(spec, warning.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 13, 11, 0, 0, time.UTC),
		HealthAssessment: &HealthAssessmentEvidence{
			Signal:   "host-raid",
			Severity: AlertSeverityCritical,
			Codes:    []string{"raid_degraded"},
		},
	})
	if err != nil {
		t.Fatalf("critical evaluation failed: %v", err)
	}
	if critical.State.Severity != AlertSeverityCritical {
		t.Fatalf("critical severity = %q, want critical", critical.State.Severity)
	}
	if critical.Transition == nil || critical.Transition.Kind != EvaluationTransitionSeverityChanged {
		t.Fatalf("critical transition = %+v, want severity-changed", critical.Transition)
	}

	recovered, err := Evaluate(spec, critical.State, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 13, 12, 0, 0, time.UTC),
		HealthAssessment: &HealthAssessmentEvidence{
			Signal: "host-raid",
		},
	})
	if err != nil {
		t.Fatalf("recovery evaluation failed: %v", err)
	}
	if recovered.State.State != AlertStateClear {
		t.Fatalf("recovered state = %q, want clear", recovered.State.State)
	}
	if recovered.Transition == nil || recovered.Transition.Kind != EvaluationTransitionRecovered {
		t.Fatalf("recovered transition = %+v, want recovered", recovered.Transition)
	}
}

func TestEvaluatePostureThresholdAgeAndSize(t *testing.T) {
	spec := ResourceAlertSpec{
		ID:           "inst:node:100-snapshot-weekly",
		ResourceID:   "inst:node:100",
		ResourceType: unifiedresources.ResourceTypeVM,
		Kind:         AlertSpecKindPostureThreshold,
		Severity:     AlertSeverityWarning,
		PostureThreshold: &PostureThresholdSpec{
			AgeMetric:    "snapshot-age-days",
			WarningAge:   7,
			CriticalAge:  14,
			SizeMetric:   "snapshot-size-gib",
			WarningSize:  50,
			CriticalSize: 100,
		},
	}

	size := 120.0
	critical, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 13, 20, 0, 0, time.UTC),
		PostureThreshold: &PostureThresholdEvidence{
			AgeMetric:  "snapshot-age-days",
			AgeValue:   2,
			SizeMetric: "snapshot-size-gib",
			SizeValue:  &size,
		},
	})
	if err != nil {
		t.Fatalf("critical evaluation failed: %v", err)
	}
	if critical.State.State != AlertStateFiring || critical.State.Severity != AlertSeverityCritical {
		t.Fatalf("critical state = %+v, want firing critical", critical.State)
	}
	if critical.State.Reason != "posture-threshold-size-critical" {
		t.Fatalf("critical reason = %q, want size critical", critical.State.Reason)
	}

	warning, err := Evaluate(spec, EvaluatorState{}, AlertEvidence{
		ObservedAt: time.Date(2026, 3, 10, 13, 21, 0, 0, time.UTC),
		PostureThreshold: &PostureThresholdEvidence{
			AgeMetric:  "snapshot-age-days",
			AgeValue:   10,
			SizeMetric: "snapshot-size-gib",
			SizeValue:  &[]float64{20}[0],
		},
	})
	if err != nil {
		t.Fatalf("warning evaluation failed: %v", err)
	}
	if warning.State.State != AlertStateFiring || warning.State.Severity != AlertSeverityWarning {
		t.Fatalf("warning state = %+v, want firing warning", warning.State)
	}
	if warning.State.Reason != "posture-threshold-age-warning" {
		t.Fatalf("warning reason = %q, want age warning", warning.State.Reason)
	}
}
