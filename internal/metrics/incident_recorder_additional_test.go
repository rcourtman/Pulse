package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type countingProvider struct {
	ids     []string
	metrics map[string]map[string]float64
	calls   int32
}

func (c *countingProvider) GetCurrentMetrics(resourceID string) (map[string]float64, error) {
	atomic.AddInt32(&c.calls, 1)
	metrics, ok := c.metrics[resourceID]
	if !ok {
		return nil, errNoMetrics(resourceID)
	}
	copied := make(map[string]float64, len(metrics))
	for k, v := range metrics {
		copied[k] = v
	}
	return copied, nil
}

func (c *countingProvider) GetMonitoredResourceIDs() []string {
	return append([]string{}, c.ids...)
}

func waitForCalls(t *testing.T, provider *countingProvider, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&provider.calls) > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expected provider to be called")
}

func TestDefaultIncidentRecorderConfig(t *testing.T) {
	cfg := DefaultIncidentRecorderConfig()
	if cfg.SampleInterval == 0 || cfg.PreIncidentWindow == 0 || cfg.PostIncidentWindow == 0 {
		t.Fatalf("default config should be non-zero, got %+v", cfg)
	}
	if cfg.MaxDataPointsPerWindow == 0 || cfg.MaxWindows == 0 || cfg.RetentionDuration == 0 {
		t.Fatalf("default config should be non-zero, got %+v", cfg)
	}
}

func TestIncidentRecorderStartStop(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		SampleInterval:         5 * time.Millisecond,
		PreIncidentWindow:      10 * time.Millisecond,
		PostIncidentWindow:     10 * time.Millisecond,
		MaxDataPointsPerWindow: 5,
	})

	provider := &countingProvider{
		ids: []string{"res-1"},
		metrics: map[string]map[string]float64{
			"res-1": {"cpu": 1},
		},
	}
	recorder.SetMetricsProvider(provider)

	recorder.Start()
	waitForCalls(t, provider, 200*time.Millisecond)
	recorder.Stop()

	if recorder.running {
		t.Fatalf("expected recorder to be stopped")
	}
}

func TestGetWindowsForResource(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{})

	active := &IncidentWindow{ID: "active-1", ResourceID: "res-1"}
	recorder.activeWindows["active-1"] = active

	got := recorder.GetWindowsForResource("res-1", 0)
	if len(got) != 1 || got[0].ID != "active-1" {
		t.Fatalf("expected active window, got %+v", got)
	}

	recorder.activeWindows = map[string]*IncidentWindow{}
	recorder.completedWindows = []*IncidentWindow{
		{ID: "old", ResourceID: "res-1"},
		{ID: "new", ResourceID: "res-1"},
	}

	limited := recorder.GetWindowsForResource("res-1", 1)
	if len(limited) != 1 || limited[0].ID != "new" {
		t.Fatalf("expected most recent completed window, got %+v", limited)
	}
}

func TestRecordSampleSkipsPreIncidentBufferOnMetricsError(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		PreIncidentWindow:      time.Minute,
		PostIncidentWindow:     time.Minute,
		MaxDataPointsPerWindow: 10,
	})

	provider := &stubMetricsProvider{
		metricsByID: map[string]map[string]float64{
			"res-ok": {"cpu": 1},
		},
		ids: []string{"res-ok", "res-missing"},
	}
	recorder.SetMetricsProvider(provider)

	windowID := recorder.StartRecording("res-ok", "db", "host", "alert", "alert-1")
	recorder.recordSample()

	window := recorder.activeWindows[windowID]
	if window == nil {
		t.Fatalf("expected active window %s", windowID)
	}
	if len(window.DataPoints) != 1 {
		t.Fatalf("expected active window sample to be captured, got %d", len(window.DataPoints))
	}
	if len(recorder.preIncidentBuffer["res-ok"]) == 0 {
		t.Fatalf("expected pre-incident buffer for res-ok")
	}
	if _, ok := recorder.preIncidentBuffer["res-missing"]; ok {
		t.Fatalf("expected no pre-incident buffer for res-missing when metrics collection fails")
	}
}
