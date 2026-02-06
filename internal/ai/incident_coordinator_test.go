package ai

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
)

// MockMetricsProvider for the real IncidentRecorder
type MockMetricsProvider struct {
	mu   sync.Mutex
	data map[string]map[string]float64
}

func (m *MockMetricsProvider) GetCurrentMetrics(resourceID string) (map[string]float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if val, ok := m.data[resourceID]; ok {
		return val, nil
	}
	return map[string]float64{"cpu": 10.0}, nil
}

func (m *MockMetricsProvider) GetMonitoredResourceIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}

func TestIncidentCoordinator_Lifecycle(t *testing.T) {
	cfg := DefaultIncidentCoordinatorConfig()
	coord := NewIncidentCoordinator(cfg)

	if coord.running {
		t.Error("Coordinator should not be running initially")
	}

	coord.Start()
	if !coord.running {
		t.Error("Coordinator should be running after Start()")
	}

	// Double start should be safe
	coord.Start()

	coord.Stop()
	if coord.running {
		t.Error("Coordinator should not be running after Stop()")
	}

	// Double stop should be safe
	coord.Stop()
}

func TestIncidentCoordinator_OnAlertFired(t *testing.T) {
	cfg := DefaultIncidentCoordinatorConfig()
	coord := NewIncidentCoordinator(cfg)

	// Create real recorder with mock provider
	recCfg := metrics.DefaultIncidentRecorderConfig()
	recCfg.SampleInterval = 50 * time.Millisecond // fast sampling
	recorder := metrics.NewIncidentRecorder(recCfg)
	provider := &MockMetricsProvider{data: make(map[string]map[string]float64)}
	recorder.SetMetricsProvider(provider)
	recorder.Start() // Recorder must be started
	defer recorder.Stop()

	// Inject recorder into coordinator
	coord.SetRecorder(recorder)
	coord.Start()

	alert := &alerts.Alert{
		ID:         "alert-1",
		ResourceID: "res-1",
	}

	coord.OnAlertFired(alert)

	if coord.GetActiveIncidentCount() != 1 {
		t.Errorf("Expected 1 active incident, got %d", coord.GetActiveIncidentCount())
	}

	// Verify recorder has active window
	wid := coord.GetRecordingWindowID("alert-1")
	if wid == "" {
		t.Error("Expected valid window ID")
	}

	// Fire same alert again - should be ignored
	coord.OnAlertFired(alert)
	if coord.GetActiveIncidentCount() != 1 {
		t.Errorf("Expected count to remain 1, got %d", coord.GetActiveIncidentCount())
	}

	// Fire another alert
	alert2 := &alerts.Alert{
		ID:         "alert-2",
		ResourceID: "res-2",
	}
	coord.OnAlertFired(alert2)
	if coord.GetActiveIncidentCount() != 2 {
		t.Errorf("Expected 2 active incidents, got %d", coord.GetActiveIncidentCount())
	}
}

func TestIncidentCoordinator_OnAlertCleared(t *testing.T) {
	cfg := DefaultIncidentCoordinatorConfig()
	cfg.PostDuration = 50 * time.Millisecond // fast for testing
	coord := NewIncidentCoordinator(cfg)

	// Create real recorder
	recCfg := metrics.DefaultIncidentRecorderConfig()
	recorder := metrics.NewIncidentRecorder(recCfg)
	provider := &MockMetricsProvider{data: make(map[string]map[string]float64)}
	recorder.SetMetricsProvider(provider)
	recorder.Start()
	defer recorder.Stop()

	coord.SetRecorder(recorder)
	coord.Start()

	alert := &alerts.Alert{ID: "alert-1", ResourceID: "res-1"}
	coord.OnAlertFired(alert)

	if coord.GetActiveIncidentCount() != 1 {
		t.Fatal("Failed to start incident")
	}

	// Clear alert
	coord.OnAlertCleared(alert)

	// Since we have a recorder and postDuration is 50ms, it should NOT be removed immediately
	if coord.GetActiveIncidentCount() != 1 {
		t.Error("Incident should NOT be removed immediately when recorder is active")
	}

	// Wait for post duration
	time.Sleep(100 * time.Millisecond)

	// Now it should be removed (via time.AfterFunc callback)
	if coord.GetActiveIncidentCount() != 0 {
		t.Errorf("Incident should be removed after post duration, count=%d", coord.GetActiveIncidentCount())
	}
}

func TestIncidentCoordinator_MaxConcurrent(t *testing.T) {
	cfg := DefaultIncidentCoordinatorConfig()
	cfg.MaxConcurrent = 1
	coord := NewIncidentCoordinator(cfg)
	coord.Start()

	coord.OnAlertFired(&alerts.Alert{ID: "alert-1", ResourceID: "res-1"})
	if coord.GetActiveIncidentCount() != 1 {
		t.Fatal("Should accept first incident")
	}

	coord.OnAlertFired(&alerts.Alert{ID: "alert-2", ResourceID: "res-2"})
	if coord.GetActiveIncidentCount() != 1 {
		t.Error("Should ignore second incident due to cap")
	}
}

func TestIncidentCoordinator_OnAnomalyDetected(t *testing.T) {
	cfg := DefaultIncidentCoordinatorConfig()
	coord := NewIncidentCoordinator(cfg)

	recCfg := metrics.DefaultIncidentRecorderConfig()
	recorder := metrics.NewIncidentRecorder(recCfg)
	provider := &MockMetricsProvider{data: make(map[string]map[string]float64)}
	recorder.SetMetricsProvider(provider)
	recorder.Start()
	defer recorder.Stop()

	coord.SetRecorder(recorder)
	coord.Start()

	// Start anomaly recording
	coord.OnAnomalyDetected("res-1", "host", "cpu", "critical")

	if coord.GetActiveIncidentCount() != 1 {
		t.Errorf("Should start incident for anomaly, got %d", coord.GetActiveIncidentCount())
	}

	// Anomaly ID format check (internal detail, but verify implicitly via count)
}

func TestIncidentCoordinator_GetRecordingWindowID(t *testing.T) {
	cfg := DefaultIncidentCoordinatorConfig()
	coord := NewIncidentCoordinator(cfg)

	recCfg := metrics.DefaultIncidentRecorderConfig()
	recorder := metrics.NewIncidentRecorder(recCfg)
	recorder.Start()
	defer recorder.Stop()

	coord.SetRecorder(recorder)
	coord.Start()

	alert := &alerts.Alert{ID: "alert-1", ResourceID: "res-1"}
	coord.OnAlertFired(alert)

	// With recorder, windowID should be present (start with 'iw-')
	wid := coord.GetRecordingWindowID("alert-1")
	if wid == "" {
		t.Error("Expected valid window ID")
	}

	widMissing := coord.GetRecordingWindowID("missing")
	if widMissing != "" {
		t.Error("Expected empty window ID for missing alert")
	}
}
