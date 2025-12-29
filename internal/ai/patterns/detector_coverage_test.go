package patterns

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDetector_LoadFromDiskSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now()
	data := struct {
		Events   []HistoricalEvent   `json:"events"`
		Patterns map[string]*Pattern `json:"patterns"`
	}{
		Events: []HistoricalEvent{
			{
				ID:         "event-1",
				ResourceID: "vm-1",
				EventType:  EventHighCPU,
				Timestamp:  now,
			},
		},
		Patterns: map[string]*Pattern{
			"vm-1:high_cpu": {
				ResourceID:     "vm-1",
				EventType:      EventHighCPU,
				Occurrences:    3,
				LastOccurrence: now,
				NextPredicted:  now.Add(24 * time.Hour),
				Confidence:     0.5,
			},
		},
	}
	blob, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	path := filepath.Join(tmpDir, "ai_patterns.json")
	if err := os.WriteFile(path, blob, 0600); err != nil {
		t.Fatalf("write data: %v", err)
	}

	d := NewDetector(DetectorConfig{DataDir: tmpDir})
	if len(d.events) == 0 {
		t.Fatal("expected events to be loaded")
	}
	if len(d.patterns) == 0 {
		t.Fatal("expected patterns to be loaded")
	}
}

func TestNewDetector_LoadFromDiskError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_patterns.json")
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}

	d := NewDetector(DetectorConfig{DataDir: tmpDir})
	if len(d.events) != 0 {
		t.Error("expected no events on load error")
	}
}

func TestRecordEvent_SaveError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "patterns-dir")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	defer os.Remove(path)

	d := NewDetector(DetectorConfig{DataDir: path, MinOccurrences: 1})
	d.RecordEvent(HistoricalEvent{
		ResourceID: "vm-1",
		EventType:  EventHighCPU,
		Timestamp:  time.Now(),
	})

	time.Sleep(20 * time.Millisecond)
}

func TestGetPredictions_FiltersAndSorts(t *testing.T) {
	d := NewDetector(DetectorConfig{MinOccurrences: 2, PredictionLimit: 48 * time.Hour})
	now := time.Now()
	d.patterns["nil"] = nil
	d.patterns["low-confidence"] = &Pattern{
		ResourceID:    "vm-1",
		EventType:     EventHighCPU,
		Occurrences:   2,
		Confidence:    0.2,
		NextPredicted: now.Add(2 * time.Hour),
	}
	d.patterns["low-occurrences"] = &Pattern{
		ResourceID:    "vm-2",
		EventType:     EventHighMemory,
		Occurrences:   1,
		Confidence:    0.9,
		NextPredicted: now.Add(2 * time.Hour),
	}
	d.patterns["past"] = &Pattern{
		ResourceID:    "vm-3",
		EventType:     EventDiskFull,
		Occurrences:   2,
		Confidence:    0.9,
		NextPredicted: now.Add(-2 * time.Hour),
	}
	d.patterns["future"] = &Pattern{
		ResourceID:    "vm-4",
		EventType:     EventOOM,
		Occurrences:   2,
		Confidence:    0.9,
		NextPredicted: now.Add(72 * time.Hour),
	}
	d.patterns["soon"] = &Pattern{
		ResourceID:    "vm-5",
		EventType:     EventRestart,
		Occurrences:   2,
		Confidence:    0.9,
		NextPredicted: now.Add(4 * time.Hour),
	}
	d.patterns["later"] = &Pattern{
		ResourceID:    "vm-6",
		EventType:     EventUnresponsive,
		Occurrences:   2,
		Confidence:    0.9,
		NextPredicted: now.Add(12 * time.Hour),
	}

	predictions := d.GetPredictions()
	if len(predictions) != 2 {
		t.Fatalf("expected 2 predictions, got %d", len(predictions))
	}
	if predictions[0].DaysUntil > predictions[1].DaysUntil {
		t.Fatalf("expected predictions sorted by days until")
	}
}

func TestGetPatterns_SkipsNil(t *testing.T) {
	d := NewDetector(DefaultConfig())
	d.patterns["nil"] = nil
	d.patterns["ok"] = &Pattern{ResourceID: "vm-1", EventType: EventHighCPU}

	result := d.GetPatterns()
	if len(result) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(result))
	}
	if result["ok"] == nil {
		t.Fatal("expected non-nil pattern")
	}
}

func TestComputePattern_AverageDuration(t *testing.T) {
	d := NewDetector(DetectorConfig{MinOccurrences: 2, PatternWindow: 24 * time.Hour})
	now := time.Now()
	d.events = []HistoricalEvent{
		{
			ResourceID: "vm-1",
			EventType:  EventHighCPU,
			Timestamp:  now.Add(-4 * time.Hour),
			Duration:   10 * time.Minute,
		},
		{
			ResourceID: "vm-1",
			EventType:  EventHighCPU,
			Timestamp:  now.Add(-2 * time.Hour),
			Duration:   20 * time.Minute,
		},
		{
			ResourceID: "vm-1",
			EventType:  EventHighCPU,
			Timestamp:  now,
		},
	}

	pattern := d.computePattern("vm-1", EventHighCPU)
	if pattern == nil {
		t.Fatal("expected pattern")
	}
	if pattern.AverageDuration == 0 {
		t.Fatal("expected average duration to be set")
	}
}

func TestSaveToDisk_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	d := NewDetector(DetectorConfig{DataDir: tmpDir})
	d.patterns["bad"] = &Pattern{Confidence: math.NaN()}
	if err := d.saveToDisk(); err == nil {
		t.Fatal("expected json marshal error")
	}

	tmpFile, err := os.CreateTemp("", "patterns-dir")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	defer os.Remove(path)

	d = NewDetector(DetectorConfig{DataDir: path})
	if err := d.saveToDisk(); err == nil {
		t.Fatal("expected write error")
	}

	tmpDir = t.TempDir()
	path = filepath.Join(tmpDir, "ai_patterns.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	d = NewDetector(DetectorConfig{DataDir: tmpDir})
	if err := d.saveToDisk(); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestLoadFromDisk_ErrorsAndPrune(t *testing.T) {
	d := &Detector{}
	if err := d.loadFromDisk(); err != nil {
		t.Fatalf("expected empty dataDir to return nil, got %v", err)
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_patterns.json")
	blob := make([]byte, (10<<20)+1)
	if err := os.WriteFile(path, blob, 0600); err != nil {
		t.Fatalf("write large file: %v", err)
	}
	d = &Detector{dataDir: tmpDir}
	if err := d.loadFromDisk(); err == nil {
		t.Fatal("expected size error")
	}

	tmpDir = t.TempDir()
	path = filepath.Join(tmpDir, "ai_patterns.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	d = &Detector{dataDir: tmpDir}
	if err := d.loadFromDisk(); err == nil {
		t.Fatal("expected read error")
	}

	tmpDir = t.TempDir()
	path = filepath.Join(tmpDir, "ai_patterns.json")
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}
	d = &Detector{dataDir: tmpDir}
	if err := d.loadFromDisk(); err == nil {
		t.Fatal("expected json error")
	}

	tmpDir = t.TempDir()
	now := time.Now()
	data := struct {
		Events   []HistoricalEvent   `json:"events"`
		Patterns map[string]*Pattern `json:"patterns"`
	}{
		Events: []HistoricalEvent{
			{
				ID:         "old",
				ResourceID: "vm-1",
				EventType:  EventHighCPU,
				Timestamp:  now.Add(-2 * time.Hour),
			},
			{
				ID:         "new",
				ResourceID: "vm-1",
				EventType:  EventHighCPU,
				Timestamp:  now,
			},
		},
		Patterns: map[string]*Pattern{
			"old": {
				ResourceID:     "vm-1",
				EventType:      EventHighCPU,
				Occurrences:    1,
				LastOccurrence: now.Add(-2 * time.Hour),
				Confidence:     0.5,
			},
			"nil": nil,
			"new": {
				ResourceID:     "vm-1",
				EventType:      EventHighCPU,
				Occurrences:    3,
				LastOccurrence: now,
				Confidence:     0.5,
			},
		},
	}
	blob, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	path = filepath.Join(tmpDir, "ai_patterns.json")
	if err := os.WriteFile(path, blob, 0600); err != nil {
		t.Fatalf("write data: %v", err)
	}
	d = &Detector{
		dataDir:        tmpDir,
		maxEvents:      10,
		minOccurrences: 2,
		patternWindow:  time.Hour,
	}
	if err := d.loadFromDisk(); err != nil {
		t.Fatalf("loadFromDisk failed: %v", err)
	}
	if _, ok := d.patterns["old"]; ok {
		t.Fatal("expected old pattern to be pruned")
	}
	if _, ok := d.patterns["nil"]; ok {
		t.Fatal("expected nil pattern to be pruned")
	}
	if _, ok := d.patterns["new"]; !ok {
		t.Fatal("expected new pattern to remain")
	}
	if len(d.events) != 1 {
		t.Fatalf("expected trimmed events, got %d", len(d.events))
	}
}

func TestFormatForContext_Limits(t *testing.T) {
	d := NewDetector(DetectorConfig{MinOccurrences: 1, PredictionLimit: 365 * 24 * time.Hour})
	now := time.Now()
	for i := 0; i < 50; i++ {
		key := "res-" + intToStr(i)
		d.patterns[key] = &Pattern{
			ResourceID:     key,
			EventType:      EventHighCPU,
			Occurrences:    3,
			Confidence:     0.9,
			LastOccurrence: now.Add(-24 * time.Hour),
			NextPredicted:  now.Add(2 * time.Hour),
		}
	}

	result := d.FormatForContext("")
	if result == "" {
		t.Fatal("expected non-empty context")
	}
	if !contains(result, "... and more") {
		t.Fatalf("expected truncation marker, got %q", result)
	}
}

func TestFormatPatternBasis_EventNames(t *testing.T) {
	now := time.Now()
	tests := []struct {
		eventType EventType
		expected  string
		overdue   bool
	}{
		{EventDiskFull, "disk space critical", true},
		{EventUnresponsive, "unresponsive periods", false},
		{EventBackupFailed, "backup failures", false},
	}

	for _, tt := range tests {
		next := now.Add(2 * time.Hour)
		if tt.overdue {
			next = now.Add(-2 * time.Hour)
		}
		result := formatPatternBasis(&Pattern{
			EventType:       tt.eventType,
			AverageInterval: 24 * time.Hour,
			LastOccurrence:  now.Add(-48 * time.Hour),
			NextPredicted:   next,
		})
		if !contains(result, tt.expected) {
			t.Fatalf("expected %q in result, got %q", tt.expected, result)
		}
		if tt.overdue && !contains(result, "overdue") {
			t.Fatalf("expected overdue wording, got %q", result)
		}
	}
}
