package specs

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestResourceAlertSpecValidateAcceptsSupportedKinds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec ResourceAlertSpec
	}{
		{
			name: "metric threshold",
			spec: ResourceAlertSpec{
				ID:           "vm-01-cpu-high",
				ResourceID:   "agent:node-01/vm:100",
				ResourceType: unifiedresources.ResourceTypeVM,
				Kind:         AlertSpecKindMetricThreshold,
				Severity:     AlertSeverityWarning,
				MetricThreshold: &MetricThresholdSpec{
					Metric:    "cpu",
					Direction: ThresholdDirectionAbove,
					Trigger:   90,
				},
			},
		},
		{
			name: "severity threshold",
			spec: ResourceAlertSpec{
				ID:           "pmg-01-queue-total",
				ResourceID:   "pmg-01",
				ResourceType: unifiedresources.ResourceTypePMG,
				Kind:         AlertSpecKindSeverityThreshold,
				Severity:     AlertSeverityWarning,
				SeverityThreshold: &SeverityThresholdSpec{
					Metric:    "queue-total",
					Direction: ThresholdDirectionAbove,
					Warning:   500,
					Critical:  1000,
				},
			},
		},
		{
			name: "change threshold",
			spec: ResourceAlertSpec{
				ID:           "pmg-01-quarantine-spam",
				ResourceID:   "pmg-01",
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
			},
		},
		{
			name: "baseline anomaly",
			spec: ResourceAlertSpec{
				ID:           "pmg-01-anomaly-spamIn",
				ResourceID:   "pmg-01",
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
			},
		},
		{
			name: "health assessment",
			spec: ResourceAlertSpec{
				ID:           "agent:host1/raid:md2-health",
				ResourceID:   "agent:host1/raid:md2",
				ResourceType: unifiedresources.ResourceTypeAgent,
				Kind:         AlertSpecKindHealthAssessment,
				Severity:     AlertSeverityWarning,
				HealthAssessment: &HealthAssessmentSpec{
					Signal: "host-raid",
					Codes:  []string{"raid_degraded", "raid_rebuilding"},
				},
			},
		},
		{
			name: "connectivity",
			spec: ResourceAlertSpec{
				ID:           "agent-01-heartbeat-lost",
				ResourceID:   "agent:node-01",
				ResourceType: unifiedresources.ResourceTypeAgent,
				Kind:         AlertSpecKindConnectivity,
				Severity:     AlertSeverityCritical,
				Connectivity: &ConnectivitySpec{
					Signal:    "heartbeat",
					LostAfter: 2 * time.Minute,
				},
			},
		},
		{
			name: "powered state",
			spec: ResourceAlertSpec{
				ID:           "vm-01-powered-off",
				ResourceID:   "agent:node-01/vm:100",
				ResourceType: unifiedresources.ResourceTypeVM,
				Kind:         AlertSpecKindPoweredState,
				Severity:     AlertSeverityWarning,
				PoweredState: &PoweredStateSpec{
					Expected: PowerStateOn,
				},
			},
		},
		{
			name: "provider incident",
			spec: ResourceAlertSpec{
				ID:           "tank-zfs-health",
				ResourceID:   "storage:tank",
				ResourceType: unifiedresources.ResourceTypeStorage,
				Kind:         AlertSpecKindProviderIncident,
				Severity:     AlertSeverityCritical,
				ProviderIncident: &ProviderIncidentSpec{
					Provider: "truenas",
					Codes:    []string{"truenas_volume_status"},
				},
			},
		},
		{
			name: "service gap",
			spec: ResourceAlertSpec{
				ID:           "pbs-archive-backup-gap",
				ResourceID:   "pbs:archive",
				ResourceType: unifiedresources.ResourceTypePBS,
				Kind:         AlertSpecKindServiceGap,
				Severity:     AlertSeverityCritical,
				ServiceGap: &ServiceGapSpec{
					Service:         "backup-run",
					WarningPercent:  10,
					CriticalPercent: 50,
				},
			},
		},
		{
			name: "resource incident rollup",
			spec: ResourceAlertSpec{
				ID:           "pbs-archive-rollup",
				ResourceID:   "pbs:archive",
				ResourceType: unifiedresources.ResourceTypePBS,
				Kind:         AlertSpecKindResourceIncidentRollup,
				Severity:     AlertSeverityCritical,
				ResourceIncidentRollup: &ResourceIncidentRollupSpec{
					Code:          "capacity_runway_low",
					IncidentCount: 2,
				},
			},
		},
		{
			name: "discrete state",
			spec: ResourceAlertSpec{
				ID:           "ceph-health-bad",
				ResourceID:   "ceph:cluster-a",
				ResourceType: unifiedresources.ResourceTypeCeph,
				Kind:         AlertSpecKindDiscreteState,
				Severity:     AlertSeverityWarning,
				DiscreteState: &DiscreteStateSpec{
					StateKey:      "health",
					TriggerStates: []string{"warning", "error"},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.spec.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestResourceAlertSpecValidateAllowsMetricThresholdWithoutRecovery(t *testing.T) {
	t.Parallel()

	spec := ResourceAlertSpec{
		ID:           "agent-01-cpu-hot",
		ResourceID:   "agent:node-01",
		ResourceType: unifiedresources.ResourceTypeAgent,
		Kind:         AlertSpecKindMetricThreshold,
		Severity:     AlertSeverityWarning,
		MetricThreshold: &MetricThresholdSpec{
			Metric:    "cpu",
			Direction: ThresholdDirectionAbove,
			Trigger:   85,
		},
	}

	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if spec.MetricThreshold.Recovery != nil {
		t.Fatalf("expected nil recovery threshold, got %v", *spec.MetricThreshold.Recovery)
	}
}

func TestResourceAlertSpecValidateAllowsNodeMigrationBridgeType(t *testing.T) {
	t.Parallel()

	spec := ResourceAlertSpec{
		ID:           "node-pve1-cpu",
		ResourceID:   "node/pve-1",
		ResourceType: unifiedresources.ResourceType("node"),
		Kind:         AlertSpecKindMetricThreshold,
		Severity:     AlertSeverityWarning,
		MetricThreshold: &MetricThresholdSpec{
			Metric:    "cpu",
			Direction: ThresholdDirectionAbove,
			Trigger:   85,
		},
	}

	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestResourceAlertSpecValidateAllowsAgentDiskMigrationBridgeType(t *testing.T) {
	t.Parallel()

	spec := ResourceAlertSpec{
		ID:           "agent:host1/disk:data-disk",
		ResourceID:   "agent:host1/disk:data",
		ResourceType: unifiedresources.ResourceType("agent-disk"),
		Kind:         AlertSpecKindMetricThreshold,
		Severity:     AlertSeverityWarning,
		MetricThreshold: &MetricThresholdSpec{
			Metric:    "disk",
			Direction: ThresholdDirectionAbove,
			Trigger:   85,
		},
	}

	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestResourceAlertSpecValidateAllowsDockerHostMigrationBridgeType(t *testing.T) {
	t.Parallel()

	spec := ResourceAlertSpec{
		ID:           "docker:host1-connectivity",
		ResourceID:   "docker:host1",
		ResourceType: unifiedresources.ResourceType("docker-host"),
		Kind:         AlertSpecKindConnectivity,
		Severity:     AlertSeverityCritical,
		Connectivity: &ConnectivitySpec{
			Signal:    "status",
			LostAfter: time.Second,
		},
	}

	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestResourceAlertSpecValidateRejectsPayloadKindMismatch(t *testing.T) {
	t.Parallel()

	spec := ResourceAlertSpec{
		ID:           "agent-01-heartbeat-lost",
		ResourceID:   "agent:node-01",
		ResourceType: unifiedresources.ResourceTypeAgent,
		Kind:         AlertSpecKindConnectivity,
		Severity:     AlertSeverityCritical,
		MetricThreshold: &MetricThresholdSpec{
			Metric:    "cpu",
			Direction: ThresholdDirectionAbove,
			Trigger:   90,
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "connectivity payload is required") {
		t.Fatalf("expected connectivity payload error, got %v", err)
	}
}

func TestMetricThresholdSpecValidateRejectsInvalidRecoveryDirection(t *testing.T) {
	t.Parallel()

	recovery := 92.0
	spec := MetricThresholdSpec{
		Metric:    "cpu",
		Direction: ThresholdDirectionAbove,
		Trigger:   90,
		Recovery:  &recovery,
	}

	err := spec.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "recovery must be below trigger") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSeverityThresholdSpecValidateRejectsInvertedAboveBands(t *testing.T) {
	t.Parallel()

	spec := SeverityThresholdSpec{
		Metric:    "queue-total",
		Direction: ThresholdDirectionAbove,
		Warning:   500,
		Critical:  400,
	}

	err := spec.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "critical must be greater than or equal to warning") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChangeThresholdSpecValidateRejectsPercentWithoutDelta(t *testing.T) {
	t.Parallel()

	spec := ChangeThresholdSpec{
		Metric:         "quarantine-spam",
		WarningPercent: 25,
	}

	err := spec.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "warning delta is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaselineAnomalySpecValidateRejectsInvertedQuietDelta(t *testing.T) {
	t.Parallel()

	spec := BaselineAnomalySpec{
		Metric:             "spamIn",
		QuietBaseline:      40,
		WarningRatio:       1.8,
		CriticalRatio:      2.5,
		WarningDelta:       150,
		CriticalDelta:      300,
		QuietWarningDelta:  80,
		QuietCriticalDelta: 60,
	}

	err := spec.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "quiet critical delta must be greater than or equal to quiet warning delta") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOverrideTargetRequiresExplicitResourceIDAndMatchesSpec(t *testing.T) {
	t.Parallel()

	spec := ResourceAlertSpec{
		ID:           "tank-zfs-health",
		ResourceID:   "storage:tank",
		ResourceType: unifiedresources.ResourceTypeStorage,
		Kind:         AlertSpecKindProviderIncident,
		Severity:     AlertSeverityCritical,
		ProviderIncident: &ProviderIncidentSpec{
			Provider: "truenas",
			Codes:    []string{"truenas_volume_status"},
		},
	}

	invalid := OverrideTarget{}
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected explicit resource id validation error")
	}

	target := OverrideTarget{
		ResourceID: "storage:tank",
		SpecID:     "tank-zfs-health",
		Kind:       AlertSpecKindProviderIncident,
	}
	if err := target.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !target.Matches(spec) {
		t.Fatal("expected override target to match spec")
	}

	target.SpecID = "other-spec"
	if target.Matches(spec) {
		t.Fatal("expected mismatched spec id to fail")
	}
}

func TestAlertTransitionValidateRequiresKindSpecificEvidence(t *testing.T) {
	t.Parallel()

	transition := AlertTransition{
		SpecID:     "tank-zfs-health",
		ResourceID: "storage:tank",
		Kind:       AlertSpecKindProviderIncident,
		From:       AlertStateClear,
		To:         AlertStateFiring,
		At:         time.Now().UTC(),
		Evidence: AlertEvidence{
			ObservedAt: time.Now().UTC(),
			ProviderIncident: &ProviderIncidentEvidence{
				Provider: "truenas",
				Code:     "truenas_volume_status",
				NativeID: "alert-1",
			},
		},
	}

	if err := transition.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	transition.Evidence.ProviderIncident = nil
	transition.Evidence.Connectivity = &ConnectivityEvidence{
		Signal:    "heartbeat",
		Connected: false,
	}

	err := transition.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "provider incident evidence is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
