package metrics

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewIncidentRecorderLoadFromDiskInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "incident_windows.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}

	recorder := NewIncidentRecorder(IncidentRecorderConfig{DataDir: dir})
	if recorder == nil {
		t.Fatal("expected recorder")
	}
	if len(recorder.completedWindows) != 0 {
		t.Fatalf("expected no completed windows on invalid json, got %d", len(recorder.completedWindows))
	}
}

func TestIncidentRecorderStartStopIdempotentGuards(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		SampleInterval: 10 * time.Millisecond,
	})

	recorder.Start()
	firstStopCh := recorder.stopCh

	recorder.Start()
	if recorder.stopCh != firstStopCh {
		t.Fatal("expected second Start call to be a no-op while running")
	}

	recorder.Stop()
	if recorder.running {
		t.Fatal("expected recorder to be stopped")
	}

	// Should be a no-op and should not panic.
	recorder.Stop()
}

func TestRecordSampleNoProviderNoop(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{})
	recorder.recordSample()
}

func TestRecordSampleCoversActiveWindowBranches(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		PreIncidentWindow:      time.Second,
		PostIncidentWindow:     time.Second,
		MaxDataPointsPerWindow: 1,
		MaxWindows:             10,
		RetentionDuration:      time.Hour,
	})
	provider := &stubMetricsProvider{
		metricsByID: map[string]map[string]float64{
			"res-ok":        {"cpu": 1},
			"res-active-ok": {"cpu": 2},
		},
		ids: []string{"res-ok", "res-buffer-missing"},
	}
	recorder.SetMetricsProvider(provider)

	now := time.Now()
	past := now.Add(-time.Millisecond)
	recorder.activeWindows["skip-non-recording"] = &IncidentWindow{
		ID:         "skip-non-recording",
		ResourceID: "res-active-ok",
		Status:     IncidentWindowStatusComplete,
		EndTime:    &past,
	}
	recorder.activeWindows["expire-now"] = &IncidentWindow{
		ID:         "expire-now",
		ResourceID: "res-active-ok",
		Status:     IncidentWindowStatusRecording,
		EndTime:    &past,
	}
	recorder.activeWindows["truncate-now"] = &IncidentWindow{
		ID:         "truncate-now",
		ResourceID: "res-active-ok",
		Status:     IncidentWindowStatusRecording,
		DataPoints: []IncidentDataPoint{
			{Timestamp: now, Metrics: map[string]float64{"cpu": 7}},
		},
	}
	recorder.activeWindows["metrics-error"] = &IncidentWindow{
		ID:         "metrics-error",
		ResourceID: "res-active-missing",
		Status:     IncidentWindowStatusRecording,
	}

	recorder.preIncidentBuffer["res-ok"] = []IncidentDataPoint{
		{Timestamp: now.Add(-2 * time.Second), Metrics: map[string]float64{"cpu": 0.5}},
	}
	recorder.preIncidentBuffer["stale-resource"] = []IncidentDataPoint{
		{Timestamp: now, Metrics: map[string]float64{"cpu": 9}},
	}

	recorder.recordSample()

	if _, ok := recorder.activeWindows["expire-now"]; ok {
		t.Fatal("expected expired window to complete")
	}
	if _, ok := recorder.activeWindows["truncate-now"]; ok {
		t.Fatal("expected truncated window to complete")
	}

	metricsErrWindow, ok := recorder.activeWindows["metrics-error"]
	if !ok {
		t.Fatal("expected metrics-error window to remain active")
	}
	if len(metricsErrWindow.DataPoints) != 0 {
		t.Fatalf("expected metrics-error window to skip append, got %d points", len(metricsErrWindow.DataPoints))
	}

	if _, ok := recorder.preIncidentBuffer["stale-resource"]; ok {
		t.Fatal("expected stale pre-incident buffer to be removed")
	}
	if got := len(recorder.preIncidentBuffer["res-ok"]); got != 1 {
		t.Fatalf("expected pre-incident buffer trim to keep 1 point, got %d", got)
	}

	foundTruncated := false
	foundCompleted := false
	for _, w := range recorder.completedWindows {
		if w.ID == "truncate-now" && w.Status == IncidentWindowStatusTruncated {
			foundTruncated = true
		}
		if w.ID == "expire-now" && w.Status == IncidentWindowStatusComplete {
			foundCompleted = true
		}
	}
	if !foundTruncated {
		t.Fatal("expected truncated completed window")
	}
	if !foundCompleted {
		t.Fatal("expected completed expired window")
	}
}

func TestStartRecordingCopiesPreIncidentBuffer(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		PreIncidentWindow:  time.Minute,
		PostIncidentWindow: time.Minute,
	})
	recorder.preIncidentBuffer["res-1"] = []IncidentDataPoint{
		{Timestamp: time.Now().Add(-30 * time.Second), Metrics: map[string]float64{"cpu": 1}},
	}

	windowID := recorder.StartRecording("res-1", "db", "host", "alert", "alert-1")
	window := recorder.activeWindows[windowID]
	if window == nil {
		t.Fatalf("expected active window %s", windowID)
	}
	if len(window.DataPoints) != 1 {
		t.Fatalf("expected pre-incident points to be copied, got %d", len(window.DataPoints))
	}
}

func TestCompleteWindowNoopForNonRecordingStatus(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{})
	window := &IncidentWindow{
		ID:         "already-complete",
		ResourceID: "res-1",
		Status:     IncidentWindowStatusComplete,
	}
	recorder.activeWindows[window.ID] = window

	recorder.completeWindow(window)

	if len(recorder.completedWindows) != 0 {
		t.Fatalf("expected no completed windows to be appended, got %d", len(recorder.completedWindows))
	}
	if _, ok := recorder.activeWindows[window.ID]; !ok {
		t.Fatal("expected window to remain in active map when completion is skipped")
	}
}

func TestComputeSummaryNoDataPoints(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{})
	if summary := recorder.computeSummary(&IncidentWindow{}); summary != nil {
		t.Fatal("expected nil summary when there are no data points")
	}
}

func TestTrimCompletedWindowsEnforcesMaxWindows(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		MaxWindows:        2,
		RetentionDuration: time.Hour,
	})
	now := time.Now()
	recorder.completedWindows = []*IncidentWindow{
		{ID: "w1", EndTime: &now},
		{ID: "w2", EndTime: &now},
		{ID: "w3", EndTime: &now},
	}

	recorder.trimCompletedWindows()

	if len(recorder.completedWindows) != 2 {
		t.Fatalf("expected 2 windows after trim, got %d", len(recorder.completedWindows))
	}
	if recorder.completedWindows[0].ID != "w2" || recorder.completedWindows[1].ID != "w3" {
		t.Fatalf("expected newest windows to be retained, got %s and %s", recorder.completedWindows[0].ID, recorder.completedWindows[1].ID)
	}
}

func TestGetWindowActiveAndMissing(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{})
	recorder.activeWindows["active-1"] = &IncidentWindow{ID: "active-1", ResourceID: "res-1"}

	if got := recorder.GetWindow("active-1"); got == nil {
		t.Fatal("expected active window to be returned")
	}
	if got := recorder.GetWindow("does-not-exist"); got != nil {
		t.Fatal("expected nil for missing window")
	}
}

func TestStopHandlesSaveErrorPath(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		DataDir:           t.TempDir(),
		SampleInterval:    50 * time.Millisecond,
		PreIncidentWindow: 10 * time.Millisecond,
	})
	now := time.Now()
	recorder.completedWindows = []*IncidentWindow{
		{
			ID:      "nan-window",
			EndTime: &now,
			DataPoints: []IncidentDataPoint{
				{Timestamp: now, Metrics: map[string]float64{"cpu": math.NaN()}},
			},
		},
	}

	recorder.Start()
	recorder.Stop()
}

func TestSaveToDiskErrorPaths(t *testing.T) {
	t.Run("marshal error", func(t *testing.T) {
		recorder := NewIncidentRecorder(IncidentRecorderConfig{DataDir: t.TempDir()})
		now := time.Now()
		recorder.completedWindows = []*IncidentWindow{
			{
				ID:      "marshal-fail",
				EndTime: &now,
				DataPoints: []IncidentDataPoint{
					{Timestamp: now, Metrics: map[string]float64{"cpu": math.NaN()}},
				},
			},
		}

		if err := recorder.saveToDisk(); err == nil {
			t.Fatal("expected marshal error")
		}
	})

	t.Run("mkdir error", func(t *testing.T) {
		base := t.TempDir()
		fileAsDir := filepath.Join(base, "file-instead-of-dir")
		if err := os.WriteFile(fileAsDir, []byte("x"), 0600); err != nil {
			t.Fatalf("write setup file: %v", err)
		}

		recorder := NewIncidentRecorder(IncidentRecorderConfig{DataDir: fileAsDir})
		now := time.Now()
		recorder.completedWindows = []*IncidentWindow{{ID: "w1", EndTime: &now}}

		if err := recorder.saveToDisk(); err == nil {
			t.Fatal("expected mkdir error")
		}
	})

	t.Run("write temp file error", func(t *testing.T) {
		dir := t.TempDir()
		recorder := NewIncidentRecorder(IncidentRecorderConfig{DataDir: dir})
		now := time.Now()
		recorder.completedWindows = []*IncidentWindow{{ID: "w2", EndTime: &now}}
		recorder.filePath = filepath.Join(dir, "missing-subdir", "incident_windows.json")

		if err := recorder.saveToDisk(); err == nil {
			t.Fatal("expected write temp file error")
		}
	})
}

func TestLoadFromDiskErrorPaths(t *testing.T) {
	t.Run("empty file path", func(t *testing.T) {
		recorder := NewIncidentRecorder(IncidentRecorderConfig{})
		recorder.filePath = ""
		if err := recorder.loadFromDisk(); err != nil {
			t.Fatalf("expected nil error for empty file path, got %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		recorder := NewIncidentRecorder(IncidentRecorderConfig{DataDir: t.TempDir()})
		if err := recorder.loadFromDisk(); err != nil {
			t.Fatalf("expected nil error for missing file, got %v", err)
		}
	})

	t.Run("read error", func(t *testing.T) {
		recorder := NewIncidentRecorder(IncidentRecorderConfig{})
		recorder.filePath = t.TempDir()
		if err := recorder.loadFromDisk(); err == nil {
			t.Fatal("expected read error")
		}
	})

	t.Run("unmarshal error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "incident_windows.json")
		if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
			t.Fatalf("write invalid json: %v", err)
		}

		recorder := NewIncidentRecorder(IncidentRecorderConfig{})
		recorder.filePath = path
		if err := recorder.loadFromDisk(); err == nil {
			t.Fatal("expected unmarshal error")
		}
	})
}

func TestCopyWindowNilAndIntToStringEdges(t *testing.T) {
	if copyWindow(nil) != nil {
		t.Fatal("expected nil copy for nil input")
	}

	if got := intToString(0); got != "0" {
		t.Fatalf("expected 0, got %s", got)
	}
	if got := intToString(-42); got != "-42" {
		t.Fatalf("expected -42, got %s", got)
	}
}
