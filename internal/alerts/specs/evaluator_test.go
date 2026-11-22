package specs

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestValidateEvidence(t *testing.T) {
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	size := 12.5

	validCases := []struct {
		name     string
		kind     AlertSpecKind
		evidence AlertEvidence
	}{
		{
			name: "metric-threshold",
			kind: AlertSpecKindMetricThreshold,
			evidence: AlertEvidence{
				ObservedAt: now,
				MetricThreshold: &MetricThresholdEvidence{
					Metric:    "cpu",
					Direction: ThresholdDirectionAbove,
					Observed:  85,
					Trigger:   80,
				},
			},
		},
		{
			name: "severity-threshold",
			kind: AlertSpecKindSeverityThreshold,
			evidence: AlertEvidence{
				ObservedAt: now,
				SeverityThreshold: &SeverityThresholdEvidence{
					Metric:    "queue_total",
					Direction: ThresholdDirectionAbove,
					Observed:  600,
				},
			},
		},
		{
			name: "change-threshold",
			kind: AlertSpecKindChangeThreshold,
			evidence: AlertEvidence{
				ObservedAt: now,
				ChangeThreshold: &ChangeThresholdEvidence{
					Metric:   "quarantine_spam",
					Observed: 2200,
				},
			},
		},
		{
			name: "baseline-anomaly",
			kind: AlertSpecKindBaselineAnomaly,
			evidence: AlertEvidence{
				ObservedAt: now,
				BaselineAnomaly: &BaselineAnomalyEvidence{
					Metric:   "spam_in",
					Observed: 300,
					Baseline: 50,
				},
			},
		},
		{
			name: "health-assessment",
			kind: AlertSpecKindHealthAssessment,
			evidence: AlertEvidence{
				ObservedAt: now,
				HealthAssessment: &HealthAssessmentEvidence{
					Signal:   "zfs_pool_health",
					Severity: AlertSeverityCritical,
					Codes:    []string{"degraded"},
				},
			},
		},
		{
			name: "posture-threshold",
			kind: AlertSpecKindPostureThreshold,
			evidence: AlertEvidence{
				ObservedAt: now,
				PostureThreshold: &PostureThresholdEvidence{
					AgeMetric:  "snapshot_age_days",
					AgeValue:   10,
					SizeMetric: "snapshot_size_gib",
					SizeValue:  &size,
				},
			},
		},
		{
			name: "connectivity",
			kind: AlertSpecKindConnectivity,
			evidence: AlertEvidence{
				ObservedAt:   now,
				Connectivity: &ConnectivityEvidence{Signal: "status", Connected: false},
			},
		},
		{
			name: "powered-state",
			kind: AlertSpecKindPoweredState,
			evidence: AlertEvidence{
				ObservedAt:   now,
				PoweredState: &PoweredStateEvidence{Observed: PowerStateOff, Expected: PowerStateOn},
			},
		},
		{
			name: "discrete-state",
			kind: AlertSpecKindDiscreteState,
			evidence: AlertEvidence{
				ObservedAt:    now,
				DiscreteState: &DiscreteStateEvidence{StateKey: "health", Observed: "unhealthy"},
			},
		},
		{
			name: "service-gap",
			kind: AlertSpecKindServiceGap,
			evidence: AlertEvidence{
				ObservedAt: now,
				ServiceGap: &ServiceGapEvidence{Service: "api", Desired: 4, Running: 2},
			},
		},
		{
			name: "provider-incident",
			kind: AlertSpecKindProviderIncident,
			evidence: AlertEvidence{
				ObservedAt: now,
				ProviderIncident: &ProviderIncidentEvidence{
					Provider: "truenas",
					Code:     "pool_degraded",
					NativeID: "alert-1",
				},
			},
		},
		{
			name: "resource-incident-rollup",
			kind: AlertSpecKindResourceIncidentRollup,
			evidence: AlertEvidence{
				ObservedAt: now,
				ResourceIncidentRollup: &ResourceIncidentRollupEvidence{
					Code:          "pool_degraded",
					IncidentCount: 1,
				},
			},
		},
	}

	for _, tc := range validCases {
		tc := tc
		t.Run("valid/"+tc.name, func(t *testing.T) {
			if err := ValidateEvidence(tc.kind, tc.evidence); err != nil {
				t.Fatalf("ValidateEvidence(%q) unexpected error: %v", tc.kind, err)
			}
		})
		t.Run("missing-payload/"+tc.name, func(t *testing.T) {
			if err := ValidateEvidence(tc.kind, AlertEvidence{ObservedAt: now}); err == nil {
				t.Fatalf("ValidateEvidence(%q) with empty evidence = nil, want error", tc.kind)
			}
		})
	}
}

func TestMatchPredicates(t *testing.T) {
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	prev := 1000.0
	size := 40.0

	t.Run("connectivity", func(t *testing.T) {
		spec := ResourceAlertSpec{
			ID:           "node-1-connectivity",
			ResourceID:   "node/pve-1",
			ResourceType: unifiedresources.ResourceTypeAgent,
			Kind:         AlertSpecKindConnectivity,
			Severity:     AlertSeverityCritical,
			Connectivity: &ConnectivitySpec{Signal: "status", LostAfter: time.Second},
		}
		matched, sev, reason := Match(spec, AlertEvidence{
			ObservedAt:   now,
			Connectivity: &ConnectivityEvidence{Signal: "status", Connected: false},
		})
		if !matched || sev != AlertSeverityCritical || reason != "connectivity-lost" {
			t.Fatalf("offline match = (%v, %q, %q), want (true, critical, connectivity-lost)", matched, sev, reason)
		}
		matched, _, _ = Match(spec, AlertEvidence{
			ObservedAt:   now,
			Connectivity: &ConnectivityEvidence{Signal: "status", Connected: true},
		})
		if matched {
			t.Fatalf("online connectivity matched = true, want false")
		}
	})

	t.Run("powered-state", func(t *testing.T) {
		spec := ResourceAlertSpec{
			ID:           "vm-101-powered",
			ResourceID:   "pve1:node1:101",
			ResourceType: unifiedresources.ResourceTypeVM,
			Kind:         AlertSpecKindPoweredState,
			Severity:     AlertSeverityWarning,
			PoweredState: &PoweredStateSpec{Expected: PowerStateOn},
		}
		matched, sev, reason := Match(spec, AlertEvidence{
			ObservedAt:   now,
			PoweredState: &PoweredStateEvidence{Observed: PowerStateOff, Expected: PowerStateOn},
		})
		if !matched || sev != AlertSeverityWarning || reason != "powered-state-mismatch" {
			t.Fatalf("powered-off match = (%v, %q, %q), want (true, warning, powered-state-mismatch)", matched, sev, reason)
		}
		matched, _, _ = Match(spec, AlertEvidence{
			ObservedAt:   now,
			PoweredState: &PoweredStateEvidence{Observed: PowerStateOn, Expected: PowerStateOn},
		})
		if matched {
			t.Fatalf("powered-on match = true, want false")
		}
	})

	t.Run("discrete-state", func(t *testing.T) {
		spec := ResourceAlertSpec{
			ID:            "docker-health",
			ResourceID:    "docker:h1/c1",
			ResourceType:  unifiedresources.ResourceTypeAppContainer,
			Kind:          AlertSpecKindDiscreteState,
			Severity:      AlertSeverityWarning,
			DiscreteState: &DiscreteStateSpec{StateKey: "health", TriggerStates: []string{"unhealthy"}},
		}
		matched, sev, reason := Match(spec, AlertEvidence{
			ObservedAt:    now,
			DiscreteState: &DiscreteStateEvidence{StateKey: "health", Observed: "unhealthy"},
		})
		if !matched || sev != AlertSeverityWarning || reason != "discrete-state-match" {
			t.Fatalf("unhealthy match = (%v, %q, %q), want (true, warning, discrete-state-match)", matched, sev, reason)
		}
		matched, _, _ = Match(spec, AlertEvidence{
			ObservedAt:    now,
			DiscreteState: &DiscreteStateEvidence{StateKey: "health", Observed: "healthy"},
		})
		if matched {
			t.Fatalf("healthy match = true, want false")
		}
	})

	t.Run("severity-threshold", func(t *testing.T) {
		spec := ResourceAlertSpec{
			ID:           "pmg-queue",
			ResourceID:   "pmg-1",
			ResourceType: unifiedresources.ResourceTypePMG,
			Kind:         AlertSpecKindSeverityThreshold,
			Severity:     AlertSeverityWarning,
			SeverityThreshold: &SeverityThresholdSpec{
				Metric:    "queue_total",
				Direction: ThresholdDirectionAbove,
				Warning:   500,
				Critical:  1000,
			},
		}
		matched, sev, reason := Match(spec, AlertEvidence{
			ObservedAt: now,
			SeverityThreshold: &SeverityThresholdEvidence{
				Metric:    "queue_total",
				Direction: ThresholdDirectionAbove,
				Observed:  600,
			},
		})
		if !matched || sev != AlertSeverityWarning || reason != "severity-threshold-warning" {
			t.Fatalf("warning match = (%v, %q, %q)", matched, sev, reason)
		}
		matched, sev, reason = Match(spec, AlertEvidence{
			ObservedAt: now,
			SeverityThreshold: &SeverityThresholdEvidence{
				Metric:    "queue_total",
				Direction: ThresholdDirectionAbove,
				Observed:  1200,
			},
		})
		if !matched || sev != AlertSeverityCritical || reason != "severity-threshold-critical" {
			t.Fatalf("critical match = (%v, %q, %q)", matched, sev, reason)
		}
	})

	t.Run("change-threshold", func(t *testing.T) {
		spec := ResourceAlertSpec{
			ID:           "pmg-quarantine",
			ResourceID:   "pmg-1",
			ResourceType: unifiedresources.ResourceTypePMG,
			Kind:         AlertSpecKindChangeThreshold,
			Severity:     AlertSeverityWarning,
			ChangeThreshold: &ChangeThresholdSpec{
				Metric:          "quarantine_spam",
				WarningCurrent:  2000,
				CriticalCurrent: 5000,
				WarningDelta:    250,
				CriticalDelta:   500,
				WarningPercent:  25,
				CriticalPercent: 50,
			},
		}
		matched, sev, reason := Match(spec, AlertEvidence{
			ObservedAt: now,
			ChangeThreshold: &ChangeThresholdEvidence{
				Metric:           "quarantine_spam",
				Observed:         1600,
				PreviousObserved: &prev,
			},
		})
		if !matched || sev != AlertSeverityCritical || reason != "change-threshold-growth-critical" {
			t.Fatalf("growth critical match = (%v, %q, %q)", matched, sev, reason)
		}
	})

	t.Run("baseline-anomaly", func(t *testing.T) {
		spec := ResourceAlertSpec{
			ID:           "pmg-spam-anomaly",
			ResourceID:   "pmg-1",
			ResourceType: unifiedresources.ResourceTypePMG,
			Kind:         AlertSpecKindBaselineAnomaly,
			Severity:     AlertSeverityWarning,
			BaselineAnomaly: &BaselineAnomalySpec{
				Metric:             "spam_in",
				QuietBaseline:      40,
				WarningRatio:       1.8,
				CriticalRatio:      2.5,
				WarningDelta:       150,
				CriticalDelta:      300,
				QuietWarningDelta:  60,
				QuietCriticalDelta: 120,
			},
		}
		matched, sev, reason := Match(spec, AlertEvidence{
			ObservedAt: now,
			BaselineAnomaly: &BaselineAnomalyEvidence{
				Metric:   "spam_in",
				Observed: 450,
				Baseline: 100,
			},
		})
		if !matched || sev != AlertSeverityCritical || reason != "baseline-anomaly-critical" {
			t.Fatalf("anomaly critical match = (%v, %q, %q)", matched, sev, reason)
		}
	})

	t.Run("health-assessment", func(t *testing.T) {
		spec := ResourceAlertSpec{
			ID:           "zfs-health",
			ResourceID:   "storage-1",
			ResourceType: unifiedresources.ResourceTypeStorage,
			Kind:         AlertSpecKindHealthAssessment,
			Severity:     AlertSeverityWarning,
			HealthAssessment: &HealthAssessmentSpec{
				Signal: "zfs_pool_health",
				Codes:  []string{"degraded", "faulted"},
			},
		}
		matched, sev, reason := Match(spec, AlertEvidence{
			ObservedAt: now,
			HealthAssessment: &HealthAssessmentEvidence{
				Signal:   "zfs_pool_health",
				Severity: AlertSeverityCritical,
				Codes:    []string{"faulted"},
			},
		})
		if !matched || sev != AlertSeverityCritical || reason != "health-assessment-match" {
			t.Fatalf("health assessment match = (%v, %q, %q)", matched, sev, reason)
		}
	})

	t.Run("posture-threshold", func(t *testing.T) {
		spec := ResourceAlertSpec{
			ID:           "snapshot-posture",
			ResourceID:   "pve1:node1:101",
			ResourceType: unifiedresources.ResourceTypeVM,
			Kind:         AlertSpecKindPostureThreshold,
			Severity:     AlertSeverityWarning,
			PostureThreshold: &PostureThresholdSpec{
				AgeMetric:    "snapshot_age_days",
				WarningAge:   7,
				CriticalAge:  14,
				SizeMetric:   "snapshot_size_gib",
				WarningSize:  20,
				CriticalSize: 50,
			},
		}
		matched, sev, reason := Match(spec, AlertEvidence{
			ObservedAt: now,
			PostureThreshold: &PostureThresholdEvidence{
				AgeMetric:  "snapshot_age_days",
				AgeValue:   15,
				SizeMetric: "snapshot_size_gib",
				SizeValue:  &size,
			},
		})
		if !matched || sev != AlertSeverityCritical || reason != "posture-threshold-age-critical" {
			t.Fatalf("posture critical match = (%v, %q, %q)", matched, sev, reason)
		}
	})

	t.Run("service-gap", func(t *testing.T) {
		spec := ResourceAlertSpec{
			ID:           "svc-gap",
			ResourceID:   "docker:h1/svc:api",
			ResourceType: unifiedresources.ResourceTypeAppContainer,
			Kind:         AlertSpecKindServiceGap,
			Severity:     AlertSeverityWarning,
			ServiceGap: &ServiceGapSpec{
				Service:         "api",
				WarningPercent:  10,
				CriticalPercent: 50,
			},
		}
		matched, sev, reason := Match(spec, AlertEvidence{
			ObservedAt: now,
			ServiceGap: &ServiceGapEvidence{Service: "api", Desired: 4, Running: 1},
		})
		if !matched || sev != AlertSeverityCritical || reason != "service-gap-critical" {
			t.Fatalf("service-gap match = (%v, %q, %q)", matched, sev, reason)
		}
	})
}
