package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type stubMetricsProvider struct {
	metricsByID map[string]map[string]float64
	ids         []string
}

func (s *stubMetricsProvider) GetCurrentMetrics(resourceID string) (map[string]float64, error) {
	metrics, ok := s.metricsByID[resourceID]
	if !ok {
		return nil, errNoMetrics(resourceID)
	}
	copied := make(map[string]float64, len(metrics))
	for k, v := range metrics {
		copied[k] = v
	}
	return copied, nil
}

func (s *stubMetricsProvider) GetMonitoredResourceIDs() []string {
	return append([]string{}, s.ids...)
}

type errNoMetrics string

func (e errNoMetrics) Error() string {
	return "no metrics for " + string(e)
}

func TestNewIncidentRecorderDefaults(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{})

	if recorder.config.SampleInterval != 5*time.Second {
		t.Fatalf("expected default sample interval, got %s", recorder.config.SampleInterval)
	}
	if recorder.config.PreIncidentWindow != 5*time.Minute {
		t.Fatalf("expected default pre-incident window, got %s", recorder.config.PreIncidentWindow)
	}
	if recorder.config.PostIncidentWindow != 10*time.Minute {
		t.Fatalf("expected default post-incident window, got %s", recorder.config.PostIncidentWindow)
	}
	if recorder.config.MaxDataPointsPerWindow != 500 {
		t.Fatalf("expected default max data points, got %d", recorder.config.MaxDataPointsPerWindow)
	}
	if recorder.config.MaxWindows != 100 {
		t.Fatalf("expected default max windows, got %d", recorder.config.MaxWindows)
	}
	if recorder.config.RetentionDuration != 24*time.Hour {
		t.Fatalf("expected default retention, got %s", recorder.config.RetentionDuration)
	}
}

func TestNewIncidentRecorderTrimsDataDir(t *testing.T) {
	dir := t.TempDir()

	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		DataDir: "  " + dir + "  ",
	})

	if recorder.config.DataDir != dir {
		t.Fatalf("expected trimmed data dir %q, got %q", dir, recorder.config.DataDir)
	}
	if recorder.dataDir != dir {
		t.Fatalf("expected recorder data dir %q, got %q", dir, recorder.dataDir)
	}
	wantPath := filepath.Join(dir, "incident_windows.json")
	if recorder.filePath != wantPath {
		t.Fatalf("expected file path %q, got %q", wantPath, recorder.filePath)
	}
}

func TestNewIncidentRecorderWhitespaceOnlyDataDirDisablesPersistence(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		DataDir: "   ",
	})

	if recorder.config.DataDir != "" {
		t.Fatalf("expected empty config data dir, got %q", recorder.config.DataDir)
	}
	if recorder.dataDir != "" {
		t.Fatalf("expected empty recorder data dir, got %q", recorder.dataDir)
	}
	if recorder.filePath != "" {
		t.Fatalf("expected empty file path, got %q", recorder.filePath)
	}
}

func TestStartRecordingExtendsWindow(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		PreIncidentWindow:  time.Minute,
		PostIncidentWindow: time.Minute,
	})

	firstID := recorder.StartRecording("res-1", "db", "host", "alert", "alert-1")
	firstWindow := recorder.activeWindows[firstID]
	if firstWindow == nil {
		t.Fatalf("expected window for %s", firstID)
	}
	firstEnd := *firstWindow.EndTime

	secondID := recorder.StartRecording("res-1", "db", "host", "alert", "alert-2")
	if secondID != firstID {
		t.Fatalf("expected same window ID, got %s and %s", firstID, secondID)
	}
	secondWindow := recorder.activeWindows[secondID]
	if secondWindow.EndTime.Before(firstEnd) {
		t.Fatalf("expected end time to extend or remain, got %s before %s", secondWindow.EndTime, firstEnd)
	}
}

func TestRecordSampleBuffersAndCleansUp(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		PreIncidentWindow:      time.Minute,
		PostIncidentWindow:     time.Minute,
		MaxDataPointsPerWindow: 10,
	})

	provider := &stubMetricsProvider{
		metricsByID: map[string]map[string]float64{
			"res-1": {"cpu": 1},
			"res-2": {"cpu": 2},
		},
		ids: []string{"res-1", "res-2"},
	}
	recorder.SetMetricsProvider(provider)

	recorder.preIncidentBuffer["gone"] = []IncidentDataPoint{
		{Timestamp: time.Now().Add(-time.Minute), Metrics: map[string]float64{"cpu": 0.5}},
	}

	windowID := recorder.StartRecording("res-1", "db", "host", "alert", "alert-1")
	recorder.recordSample()

	window := recorder.activeWindows[windowID]
	if window == nil {
		t.Fatalf("expected active window %s", windowID)
	}
	if len(window.DataPoints) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(window.DataPoints))
	}

	if len(recorder.preIncidentBuffer["res-1"]) == 0 {
		t.Fatalf("expected pre-incident buffer for res-1")
	}
	if len(recorder.preIncidentBuffer["res-2"]) == 0 {
		t.Fatalf("expected pre-incident buffer for res-2")
	}
	if _, ok := recorder.preIncidentBuffer["gone"]; ok {
		t.Fatalf("expected cleanup of unmonitored resource buffer")
	}
}

func TestStopRecordingCompletesWindow(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{
		PreIncidentWindow:  time.Minute,
		PostIncidentWindow: time.Minute,
	})
	provider := &stubMetricsProvider{
		metricsByID: map[string]map[string]float64{
			"res-1": {"cpu": 1},
		},
		ids: []string{"res-1"},
	}
	recorder.SetMetricsProvider(provider)

	windowID := recorder.StartRecording("res-1", "db", "host", "alert", "alert-1")
	recorder.recordSample()
	recorder.StopRecording(windowID)

	if _, ok := recorder.activeWindows[windowID]; ok {
		t.Fatalf("expected window %s to be removed from active windows", windowID)
	}
	if len(recorder.completedWindows) != 1 {
		t.Fatalf("expected 1 completed window, got %d", len(recorder.completedWindows))
	}
	if recorder.completedWindows[0].Status != IncidentWindowStatusComplete {
		t.Fatalf("expected completed status, got %s", recorder.completedWindows[0].Status)
	}
	if recorder.completedWindows[0].Summary == nil {
		t.Fatalf("expected summary to be computed")
	}
}

func TestComputeSummary(t *testing.T) {
	recorder := NewIncidentRecorder(IncidentRecorderConfig{})
	start := time.Now().Add(-time.Second)
	end := start.Add(time.Second)
	window := &IncidentWindow{
		DataPoints: []IncidentDataPoint{
			{Timestamp: start, Metrics: map[string]float64{"cpu": 1, "mem": 4}},
			{Timestamp: end, Metrics: map[string]float64{"cpu": 3, "mem": 2}},
		},
	}

	summary := recorder.computeSummary(window)
	if summary == nil {
		t.Fatalf("expected summary")
	}
	if summary.DataPoints != 2 {
		t.Fatalf("expected 2 data points, got %d", summary.DataPoints)
	}
	if summary.Peaks["cpu"] != 3 || summary.Lows["cpu"] != 1 {
		t.Fatalf("unexpected cpu stats: peaks=%v lows=%v", summary.Peaks["cpu"], summary.Lows["cpu"])
	}
	if summary.Peaks["mem"] != 4 || summary.Lows["mem"] != 2 {
		t.Fatalf("unexpected mem stats: peaks=%v lows=%v", summary.Peaks["mem"], summary.Lows["mem"])
	}
	if summary.Averages["cpu"] != 2 {
		t.Fatalf("unexpected cpu average: %v", summary.Averages["cpu"])
	}
	if summary.Averages["mem"] != 3 {
		t.Fatalf("unexpected mem average: %v", summary.Averages["mem"])
	}
	if summary.Changes["cpu"] != 2 || summary.Changes["mem"] != -2 {
		t.Fatalf("unexpected changes: cpu=%v mem=%v", summary.Changes["cpu"], summary.Changes["mem"])
	}
	if summary.Duration != time.Second {
		t.Fatalf("unexpected duration: %s", summary.Duration)
	}
}

func TestCopyWindowDeepCopy(t *testing.T) {
	now := time.Now()
	end := now.Add(time.Second)
	window := &IncidentWindow{
		ID:      "window-1",
		EndTime: &end,
		DataPoints: []IncidentDataPoint{
			{
				Timestamp: now,
				Metrics:   map[string]float64{"cpu": 1},
				Metadata:  map[string]interface{}{"host": "node-1"},
			},
		},
		Summary: &IncidentSummary{
			Peaks:     map[string]float64{"cpu": 1},
			Lows:      map[string]float64{"cpu": 1},
			Averages:  map[string]float64{"cpu": 1},
			Changes:   map[string]float64{"cpu": 0},
			Anomalies: []string{"initial"},
		},
	}

	clone := copyWindow(window)
	if clone == nil || clone == window {
		t.Fatalf("expected deep copy")
	}
	if clone.Summary == window.Summary {
		t.Fatalf("expected summary to be copied")
	}

	window.DataPoints[0].Metrics["cpu"] = 9
	window.DataPoints[0].Metadata["host"] = "mutated"
	window.Summary.Peaks["cpu"] = 9
	window.Summary.Anomalies[0] = "mutated"
	*window.EndTime = end.Add(5 * time.Second)

	if clone.DataPoints[0].Metrics["cpu"] != 1 {
		t.Fatalf("expected data points to be copied")
	}
	if clone.DataPoints[0].Metadata["host"] != "node-1" {
		t.Fatalf("expected metadata to be copied")
	}
	if clone.EndTime.Equal(*window.EndTime) {
		t.Fatalf("expected end time to be copied")
	}
	if clone.Summary.Peaks["cpu"] != 1 {
		t.Fatalf("expected summary maps to be copied")
	}
	if clone.Summary.Anomalies[0] != "initial" {
		t.Fatalf("expected summary anomalies to be copied")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	recorder := NewIncidentRecorder(IncidentRecorderConfig{DataDir: dir})

	end := time.Now()
	recorder.completedWindows = []*IncidentWindow{
		{
			ID:         "window-1",
			EndTime:    &end,
			Status:     IncidentWindowStatusComplete,
			DataPoints: []IncidentDataPoint{{Timestamp: end, Metrics: map[string]float64{"cpu": 1}}},
		},
	}

	if err := recorder.saveToDisk(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded := NewIncidentRecorder(IncidentRecorderConfig{DataDir: dir})
	window := loaded.GetWindow("window-1")
	if window == nil {
		t.Fatalf("expected window to load from disk")
	}
	if window.Status != IncidentWindowStatusComplete {
		t.Fatalf("expected status to persist, got %s", window.Status)
	}
}

func TestSaveToDiskSecuresPermissions(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod dir failed: %v", err)
	}

	recorder := NewIncidentRecorder(IncidentRecorderConfig{DataDir: dir})
	recorder.completedWindows = []*IncidentWindow{{ID: "window-1", EndTime: ptrTime(time.Now())}}

	if err := recorder.saveToDisk(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir failed: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("expected dir permissions 0700, got %o", got)
	}

	fileInfo, err := os.Stat(filepath.Join(dir, "incident_windows.json"))
	if err != nil {
		t.Fatalf("stat file failed: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected file permissions 0600, got %o", got)
	}
}

func TestLoadFromDiskRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write target failed: %v", err)
	}
	link := filepath.Join(dir, "incident_windows.json")
	requireSymlinkOrSkip(t, target, link)

	recorder := &IncidentRecorder{
		config:   DefaultIncidentRecorderConfig(),
		dataDir:  dir,
		filePath: link,
	}
	err := recorder.loadFromDisk()
	if err == nil {
		t.Fatal("expected symlink path to be rejected")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got: %v", err)
	}
}

func TestLoadFromDiskRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "incident_windows.json")
	tooLarge := make([]byte, maxIncidentWindowsFileSize+1)
	if err := os.WriteFile(path, tooLarge, 0o600); err != nil {
		t.Fatalf("write oversized file failed: %v", err)
	}

	recorder := &IncidentRecorder{
		config:   DefaultIncidentRecorderConfig(),
		dataDir:  dir,
		filePath: path,
	}
	err := recorder.loadFromDisk()
	if err == nil {
		t.Fatal("expected oversized file to be rejected")
	}
	if !strings.Contains(err.Error(), "exceeds size limit") {
		t.Fatalf("expected size-limit error, got: %v", err)
	}
}

func TestSaveToDiskRejectsSymlinkDestination(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write target failed: %v", err)
	}
	link := filepath.Join(dir, "incident_windows.json")
	requireSymlinkOrSkip(t, target, link)

	recorder := &IncidentRecorder{
		config:   DefaultIncidentRecorderConfig(),
		dataDir:  dir,
		filePath: link,
		completedWindows: []*IncidentWindow{
			{ID: "window-1", EndTime: ptrTime(time.Now())},
		},
	}
	err := recorder.saveToDisk()
	if err == nil {
		t.Fatal("expected symlink destination to be rejected")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got: %v", err)
	}
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func requireSymlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}
}
