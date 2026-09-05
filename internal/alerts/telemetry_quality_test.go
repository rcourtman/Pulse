package alerts

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func TestCalculateAlertQualitySnapshotBucketsAndOutcomes(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	resolvedAt := now.Add(-time.Hour)
	resolved := Alert{
		ID:             "private-alert-id",
		CanonicalState: "private-resource|cpu",
		Level:          AlertLevelCritical,
		StartTime:      resolvedAt.Add(-24 * time.Hour),
		AckTime:        alertQualityTimePtr(resolvedAt.Add(-time.Hour)),
		OperationalRecord: &operationaltrust.OperationalRecord{
			State:           operationaltrust.OperationalResolved,
			FirstObservedAt: resolvedAt.Add(-24 * time.Hour),
			ResolvedAt:      &resolvedAt,
		},
		Transitions: []operationaltrust.LifecycleTransition{
			{At: resolvedAt.Add(-2 * time.Hour), Cause: operationaltrust.TransitionSuppression},
			{At: resolvedAt, Cause: operationaltrust.TransitionRecoveryEvidence},
		},
	}
	repeat := resolved
	repeat.StartTime = resolved.StartTime.Add(-48 * time.Hour)
	repeatRecord := resolved.OperationalRecord.Clone()
	repeat.OperationalRecord = &repeatRecord
	repeat.OperationalRecord.FirstObservedAt = repeat.StartTime
	repeatResolvedAt := repeat.StartTime.Add(10 * time.Minute)
	repeat.OperationalRecord.ResolvedAt = &repeatResolvedAt
	repeat.Transitions = nil

	snapshot := CalculateAlertQualitySnapshot(
		[]Alert{resolved, resolved, repeat},
		[]Alert{
			{Level: AlertLevelInfo, StartTime: now.Add(-time.Hour + time.Nanosecond)},
			{Level: AlertLevelWarning, StartTime: now.Add(-time.Hour)},
			{Level: AlertLevelCritical, StartTime: now.Add(-24 * time.Hour)},
			{Level: AlertLevelCritical, StartTime: now.Add(-7 * 24 * time.Hour)},
		},
		now.Add(-30*24*time.Hour),
		now,
	)

	if snapshot.Fired30d != 2 || snapshot.FiredCritical30d != 2 || snapshot.Resolved30d != 2 || snapshot.ResolvedCritical30d != 2 {
		t.Fatalf("lifecycle counts = %+v", snapshot)
	}
	if snapshot.Acknowledged30d != 2 || snapshot.RepeatOccurrences30d != 1 {
		t.Fatalf("ack/repeat counts = %+v", snapshot)
	}
	if snapshot.SnoozedOccurrences30d != 1 || snapshot.ResolvedWhileSnoozed30d != 1 {
		t.Fatalf("snooze outcomes = %+v", snapshot)
	}
	if snapshot.ResolutionUnder15m30d != 1 || snapshot.Resolution1d7d30d != 1 {
		t.Fatalf("resolution buckets = %+v", snapshot)
	}
	if snapshot.ActiveAgeUnder1h != 1 || snapshot.ActiveAge1h24h != 1 || snapshot.ActiveAge1d7d != 1 || snapshot.ActiveAge7dPlus != 1 {
		t.Fatalf("active age boundaries = %+v", snapshot)
	}
}

func TestCalculateAlertQualitySnapshotUnsnoozePreventsEffectiveResolution(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	resolvedAt := now.Add(-time.Hour)
	alert := Alert{
		ID:        "alert-a",
		Level:     AlertLevelWarning,
		StartTime: now.Add(-2 * time.Hour),
		OperationalRecord: &operationaltrust.OperationalRecord{
			State:           operationaltrust.OperationalResolved,
			FirstObservedAt: now.Add(-2 * time.Hour),
			ResolvedAt:      &resolvedAt,
		},
		Transitions: []operationaltrust.LifecycleTransition{
			{At: now.Add(-110 * time.Minute), Cause: operationaltrust.TransitionSuppression},
			{At: now.Add(-100 * time.Minute), Cause: operationaltrust.TransitionSuppressionExpired},
			{At: resolvedAt, Cause: operationaltrust.TransitionRecoveryEvidence},
		},
	}
	snapshot := CalculateAlertQualitySnapshot([]Alert{alert}, nil, now.Add(-30*24*time.Hour), now)
	if snapshot.SnoozedOccurrences30d != 1 || snapshot.ResolvedWhileSnoozed30d != 0 {
		t.Fatalf("snooze outcome = %+v", snapshot)
	}
}

func TestAlertQualityAggregationDoesNotCreateCrossTenantRepeats(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	privateLocalIdentity := "resource-42|cpu"
	tenantOccurrence := func(startedAt time.Time) Alert {
		return Alert{
			CanonicalState: privateLocalIdentity,
			Level:          AlertLevelWarning,
			StartTime:      startedAt,
		}
	}
	one := CalculateAlertQualitySnapshot(
		[]Alert{tenantOccurrence(now.Add(-time.Hour))}, nil, now.Add(-30*24*time.Hour), now,
	)
	two := CalculateAlertQualitySnapshot(
		[]Alert{tenantOccurrence(now.Add(-2 * time.Hour))}, nil, now.Add(-30*24*time.Hour), now,
	)
	one.Add(two)
	if one.Fired30d != 2 || one.RepeatOccurrences30d != 0 {
		t.Fatalf("cross-tenant aggregate = %+v", one)
	}
}

func TestAlertQualityTelemetrySnapshotReportsAdoptionAndPersistenceHealth(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManagerWithDataDir(dataDir, WithDurableAlertStore())
	defer manager.Stop()
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = ActivationActive
	config.FlappingEnabled = true
	manager.UpdateConfig(config)
	intent := manager.GetIntentPolicies()
	grace := 30
	intent.Defaults[string(AlertIntentSignalDefault)] = AlertIntentRule{GraceSeconds: &grace}
	if _, err := manager.UpdateIntentPolicies(intent); err != nil {
		t.Fatalf("update intent policies: %v", err)
	}
	marker := filepath.Join(dataDir, "alerts", activeStateDegradedMarker)
	if err := os.WriteFile(marker, []byte("degraded"), 0o600); err != nil {
		t.Fatalf("write degraded marker: %v", err)
	}

	snapshot := manager.AlertQualityTelemetrySnapshot(time.Now().UTC())
	if snapshot.ManagerTenants != 1 || snapshot.DeliveryActiveTenants != 1 ||
		snapshot.FlappingEnabledTenants != 1 || snapshot.IntentPolicyTenants != 1 ||
		snapshot.EventHistoryHealthyTenants != 1 || snapshot.ActiveStateHealthyTenants != 1 ||
		snapshot.ActiveStateDegradedTenants != 1 {
		t.Fatalf("adoption/persistence snapshot = %+v", snapshot)
	}
}

func TestAlertQualityTelemetrySnapshotRespectsHistoryReset(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	manager.historyManager.AddAlert(Alert{
		ID:        "private-id",
		Level:     AlertLevelWarning,
		StartTime: time.Now().Add(-time.Hour),
	})
	if got := manager.AlertQualityTelemetrySnapshot(time.Now().UTC()).Fired30d; got != 1 {
		t.Fatalf("fired before reset = %d, want 1", got)
	}
	if err := manager.ClearAlertHistory(); err != nil {
		t.Fatalf("clear alert history: %v", err)
	}
	if got := manager.AlertQualityTelemetrySnapshot(time.Now().UTC()).Fired30d; got != 0 {
		t.Fatalf("fired after reset = %d, want 0", got)
	}
}

func TestAlertQualityResolutionDurationBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	durations := []time.Duration{
		15*time.Minute - time.Nanosecond,
		15 * time.Minute,
		time.Hour,
		24 * time.Hour,
		7 * 24 * time.Hour,
	}
	history := make([]Alert, 0, len(durations))
	for index, duration := range durations {
		resolvedAt := now.Add(-time.Duration(index) * time.Minute)
		history = append(history, Alert{
			ID:        string(rune('a' + index)),
			Level:     AlertLevelWarning,
			StartTime: resolvedAt.Add(-duration),
			OperationalRecord: &operationaltrust.OperationalRecord{
				State:           operationaltrust.OperationalResolved,
				FirstObservedAt: resolvedAt.Add(-duration),
				ResolvedAt:      &resolvedAt,
			},
		})
	}
	snapshot := CalculateAlertQualitySnapshot(history, nil, now.Add(-30*24*time.Hour), now)
	if snapshot.ResolutionUnder15m30d != 1 || snapshot.Resolution15m1h30d != 1 ||
		snapshot.Resolution1h24h30d != 1 || snapshot.Resolution1d7d30d != 1 ||
		snapshot.Resolution7dPlus30d != 1 {
		t.Fatalf("resolution boundary buckets = %+v", snapshot)
	}
}

func alertQualityTimePtr(value time.Time) *time.Time { return &value }

func TestPBSMissingMetricsDoNotResolve(t *testing.T) {
	m := newUnifiedEvalParityManager(t)
	m.UpdateConfig(AlertConfig{Enabled: true, PBSDefaults: ThresholdConfig{
		CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
		Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
	}})
	disableTestTimeThresholds(m)
	resolved := make(chan string, 8)
	m.SetResolvedCallback(func(id string) { resolved <- id })
	p := models.PBSInstance{ID: "pbs-missing", Name: "backup", Status: "online", CPU: 95, Memory: 95}
	m.CheckPBS(p)
	if len(m.GetActiveAlerts()) != 2 {
		t.Fatal("expected two high utilisation alerts")
	}
	p.CPU, p.Memory = 0, 0
	p.NodeMetricsUnavailable = true
	for range 5 {
		m.CheckPBS(p)
	}
	if len(m.GetActiveAlerts()) != 2 {
		t.Fatal("missing metrics falsely resolved high utilisation")
	}
	select {
	case id := <-resolved:
		t.Fatalf("false recovery callback %s", id)
	case <-time.After(50 * time.Millisecond):
	}
	p.NodeMetricsUnavailable = false
	for range 5 {
		m.CheckPBS(p)
	}
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("measured zero did not recover")
	}
	for range 2 {
		select {
		case <-resolved:
		case <-time.After(time.Second):
			t.Fatal("missing genuine recovery callback")
		}
	}
}

// Availability must gate utilisation evidence, not explicit policy or the
// independent connectivity lifecycle. Seed real incidents rather than map entries.
func TestPBSMissingMetricsPreservePolicyAndOutagePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name        string
		configure   func(*AlertConfig)
		status      string
		health      string
		wantMetrics int
		wantOffline bool
	}{
		{name: "partial failure", status: "online", wantMetrics: 2},
		{name: "connectivity disabled", status: "online", wantMetrics: 2,
			configure: func(c *AlertConfig) { c.DisableAllPBSOffline = true }},
		{name: "all PBS disabled", status: "online",
			configure: func(c *AlertConfig) { c.DisableAllPBS = true }},
		{name: "resource disabled", status: "online",
			configure: func(c *AlertConfig) { c.Overrides = map[string]ThresholdConfig{"pbs-precedence": {Disabled: true}} }},
		{name: "full outage", status: "offline", wantOffline: true},
		{name: "unhealthy connection", status: "online", health: "unhealthy", wantOffline: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newUnifiedEvalParityManager(t)
			m.UpdateConfig(AlertConfig{Enabled: true, PBSDefaults: ThresholdConfig{
				CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			}})
			disableTestTimeThresholds(m)
			p := models.PBSInstance{ID: "pbs-precedence", Name: "backup", Status: "online", CPU: 95, Memory: 95}
			m.CheckPBS(p)
			if len(m.GetActiveAlerts()) != 2 {
				t.Fatal("expected two utilisation incidents")
			}
			// Apply policy at the observation boundary without UpdateConfig's
			// independent reevaluation clearing incidents before CheckPBS runs.
			if tc.configure != nil {
				m.mu.Lock()
				tc.configure(&m.config)
				m.mu.Unlock()
			}
			p.NodeMetricsUnavailable = true
			p.CPU, p.Memory = 0, 0
			p.Status, p.ConnectionHealth = tc.status, tc.health
			for range 5 {
				m.CheckPBS(p)
			}
			metrics, offline := 0, false
			for _, a := range m.GetActiveAlerts() {
				switch a.Type {
				case "cpu", "memory":
					metrics++
				case "offline":
					offline = true
				default:
					t.Fatalf("unexpected alert type %q", a.Type)
				}
			}
			if metrics != tc.wantMetrics || offline != tc.wantOffline {
				t.Fatalf("metrics=%d offline=%v, want metrics=%d offline=%v", metrics, offline, tc.wantMetrics, tc.wantOffline)
			}
		})
	}
}
