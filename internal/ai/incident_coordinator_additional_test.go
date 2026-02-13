package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
)

func TestNewIncidentCoordinator_DefaultFallbacks(t *testing.T) {
	cfg := IncidentCoordinatorConfig{
		PreBuffer:     -1 * time.Second,
		PostDuration:  0,
		MaxConcurrent: 0,
	}

	coord := NewIncidentCoordinator(cfg)
	defaults := DefaultIncidentCoordinatorConfig()

	if coord.config.PreBuffer != defaults.PreBuffer {
		t.Fatalf("expected default pre-buffer %v, got %v", defaults.PreBuffer, coord.config.PreBuffer)
	}
	if coord.config.PostDuration != defaults.PostDuration {
		t.Fatalf("expected default post-duration %v, got %v", defaults.PostDuration, coord.config.PostDuration)
	}
	if coord.config.MaxConcurrent != defaults.MaxConcurrent {
		t.Fatalf("expected default max concurrent %d, got %d", defaults.MaxConcurrent, coord.config.MaxConcurrent)
	}
	if coord.activeIncidents == nil {
		t.Fatal("expected active incident map to be initialized")
	}
}

func TestIncidentCoordinator_OnAlertFired_RequiresRunning(t *testing.T) {
	coord := NewIncidentCoordinator(DefaultIncidentCoordinatorConfig())
	store := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	coord.SetIncidentStore(store)

	alert := &alerts.Alert{
		ID:           "alert-requires-running",
		ResourceID:   "resource-requires-running",
		ResourceName: "resource-requires-running",
	}

	coord.OnAlertFired(nil)
	coord.OnAlertFired(alert)

	if got := coord.GetActiveIncidentCount(); got != 0 {
		t.Fatalf("expected no active incidents while coordinator is stopped, got %d", got)
	}
	if got := len(store.ListIncidentsByResource(alert.ResourceID, 0)); got != 0 {
		t.Fatalf("expected no incident-store records while stopped, got %d", got)
	}

	coord.Start()
	coord.OnAlertFired(alert)

	if got := coord.GetActiveIncidentCount(); got != 1 {
		t.Fatalf("expected one active incident after start, got %d", got)
	}
	if got := len(store.ListIncidentsByResource(alert.ResourceID, 0)); got != 1 {
		t.Fatalf("expected one incident-store record after start, got %d", got)
	}
}

func TestIncidentCoordinator_OnAlertCleared_ImmediateCleanupWithoutRecorder(t *testing.T) {
	coord := NewIncidentCoordinator(DefaultIncidentCoordinatorConfig())
	store := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	coord.SetIncidentStore(store)
	coord.Start()

	alert := &alerts.Alert{
		ID:           "alert-no-recorder",
		ResourceID:   "resource-no-recorder",
		ResourceName: "resource-no-recorder",
	}

	coord.OnAlertFired(alert)
	if got := coord.GetActiveIncidentCount(); got != 1 {
		t.Fatalf("expected one active incident after fire, got %d", got)
	}

	coord.OnAlertCleared(nil)
	coord.OnAlertCleared(&alerts.Alert{ID: "missing"})
	if got := coord.GetActiveIncidentCount(); got != 1 {
		t.Fatalf("expected active incident to remain after nil/missing clears, got %d", got)
	}

	coord.OnAlertCleared(alert)
	if got := coord.GetActiveIncidentCount(); got != 0 {
		t.Fatalf("expected incident to be cleaned up immediately without recorder, got %d", got)
	}

	incidents := store.ListIncidentsByResource(alert.ResourceID, 0)
	if len(incidents) != 1 {
		t.Fatalf("expected exactly one stored incident, got %d", len(incidents))
	}
	if incidents[0].Status != memory.IncidentStatusResolved {
		t.Fatalf("expected incident status %q, got %q", memory.IncidentStatusResolved, incidents[0].Status)
	}
	if incidents[0].ClosedAt == nil {
		t.Fatal("expected incident ClosedAt to be set on clear")
	}
}

func TestIncidentCoordinator_OnAnomalyDetected_DuplicateAndCapacityAndStop(t *testing.T) {
	cfg := DefaultIncidentCoordinatorConfig()
	cfg.MaxConcurrent = 1
	cfg.PostDuration = time.Minute

	coord := NewIncidentCoordinator(cfg)

	recCfg := metrics.DefaultIncidentRecorderConfig()
	recCfg.SampleInterval = 10 * time.Millisecond
	recorder := metrics.NewIncidentRecorder(recCfg)
	recorder.SetMetricsProvider(&MockMetricsProvider{data: map[string]map[string]float64{
		"resource-anomaly": {"cpu": 95},
	}})
	recorder.Start()
	defer recorder.Stop()

	coord.SetRecorder(recorder)
	coord.Start()

	coord.OnAnomalyDetected("resource-anomaly", "host", "cpu", "critical")
	if got := coord.GetActiveIncidentCount(); got != 1 {
		t.Fatalf("expected one anomaly incident, got %d", got)
	}

	coord.OnAnomalyDetected("resource-anomaly", "host", "cpu", "critical")
	if got := coord.GetActiveIncidentCount(); got != 1 {
		t.Fatalf("expected duplicate anomaly to be ignored, got %d", got)
	}

	coord.OnAnomalyDetected("resource-anomaly", "host", "memory", "warning")
	if got := coord.GetActiveIncidentCount(); got != 1 {
		t.Fatalf("expected anomaly over capacity to be ignored, got %d", got)
	}

	coord.Stop()
	if got := coord.GetActiveIncidentCount(); got != 0 {
		t.Fatalf("expected active incidents to be cleared on stop, got %d", got)
	}

	coord.OnAnomalyDetected("resource-anomaly", "host", "cpu", "critical")
	if got := coord.GetActiveIncidentCount(); got != 0 {
		t.Fatalf("expected anomalies to be ignored while stopped, got %d", got)
	}
}
