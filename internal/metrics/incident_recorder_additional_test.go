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

func TestSaveToDiskWrapsMkdirError(t *testing.T) {
	dir := t.TempDir()
	blockingPath := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blockingPath, []byte("x"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	recorder := NewIncidentRecorder(IncidentRecorderConfig{})
	recorder.dataDir = blockingPath
	recorder.filePath = filepath.Join(blockingPath, "incident_windows.json")

	err := recorder.saveToDisk()
	if err == nil {
		t.Fatal("expected saveToDisk to fail when data dir is a file")
	}
	if !strings.Contains(err.Error(), "incident recorder save: ensure data directory") {
		t.Fatalf("expected contextual mkdir error, got: %v", err)
	}
}

func TestLoadFromDiskWrapsParseError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "incident_windows.json")
	if err := os.WriteFile(filePath, []byte("{"), 0600); err != nil {
		t.Fatalf("failed to seed invalid JSON: %v", err)
	}

	recorder := NewIncidentRecorder(IncidentRecorderConfig{})
	recorder.filePath = filePath

	err := recorder.loadFromDisk()
	if err == nil {
		t.Fatal("expected loadFromDisk to fail on malformed JSON")
	}
	if !strings.Contains(err.Error(), "incident recorder load: parse file") {
		t.Fatalf("expected contextual parse error, got: %v", err)
	}
}

func TestSaveToDiskRenameFailureCleansTempFile(t *testing.T) {
	dir := t.TempDir()
	recorder := NewIncidentRecorder(IncidentRecorderConfig{DataDir: dir})

	now := time.Now()
	recorder.completedWindows = []*IncidentWindow{
		{
			ID:      "window-1",
			EndTime: &now,
			Status:  IncidentWindowStatusComplete,
		},
	}

	if err := os.Mkdir(recorder.filePath, 0755); err != nil {
		t.Fatalf("failed to create destination directory at file path: %v", err)
	}

	tmpPath := recorder.filePath + ".tmp"
	err := recorder.saveToDisk()
	if err == nil {
		t.Fatal("expected saveToDisk to fail when rename target is a directory")
	}
	if !strings.Contains(err.Error(), "incident recorder save: commit temp file") {
		t.Fatalf("expected contextual rename error, got: %v", err)
	}
	if _, statErr := os.Stat(tmpPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected temp file cleanup after rename failure, stat err: %v", statErr)
	}
}
