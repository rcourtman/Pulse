package correlation

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDetector_LoadsFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now()
	data := struct {
		Events       []Event                 `json:"events"`
		Correlations map[string]*Correlation `json:"correlations"`
	}{
		Events: []Event{
			{
				ID:         "event-1",
				ResourceID: "node-1",
				EventType:  EventHighCPU,
				Timestamp:  now,
			},
		},
		Correlations: map[string]*Correlation{
			"node-1:vm-1:high_cpu -> restart": {
				SourceID:     "node-1",
				SourceType:   "node",
				TargetID:     "vm-1",
				TargetType:   "vm",
				EventPattern: "high_cpu -> restart",
				Occurrences:  2,
				AvgDelay:     2 * time.Minute,
				Confidence:   0.4,
				LastSeen:     now,
			},
		},
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	path := filepath.Join(tmpDir, "ai_correlations.json")
	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		t.Fatalf("write data: %v", err)
	}

	d := NewDetector(Config{
		MaxEvents:         100,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    1,
		RetentionWindow:   24 * time.Hour,
		DataDir:           tmpDir,
	})
	if len(d.events) == 0 {
		t.Error("expected events loaded from disk")
	}
}

func TestNewDetector_LoadFromDiskError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_correlations.json")
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
		t.Fatalf("write invalid data: %v", err)
	}

	d := NewDetector(Config{DataDir: tmpDir})
	if len(d.events) != 0 {
		t.Error("expected empty events on load error")
	}
}

func TestRecordEvent_AssignsDefaults(t *testing.T) {
	d := NewDetector(DefaultConfig())
	d.RecordEvent(Event{ResourceID: "vm-1", EventType: EventHighCPU})

	if len(d.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(d.events))
	}
	if d.events[0].ID == "" {
		t.Error("expected event ID to be generated")
	}
	if d.events[0].Timestamp.IsZero() {
		t.Error("expected event timestamp to be set")
	}
}

func TestRecordEvent_SaveError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "correlation-dir")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	defer os.Remove(path)

	d := NewDetector(Config{DataDir: path})
	d.RecordEvent(Event{ResourceID: "vm-1", EventType: EventHighCPU})

	time.Sleep(20 * time.Millisecond)
}

func TestGetCorrelations_SortOrder(t *testing.T) {
	d := NewDetector(Config{MinOccurrences: 1})
	d.correlations["a"] = &Correlation{Occurrences: 2, Confidence: 0.4, EventPattern: "high_cpu -> restart"}
	d.correlations["b"] = &Correlation{Occurrences: 2, Confidence: 0.8, EventPattern: "high_mem -> restart"}

	result := d.GetCorrelations()
	if len(result) != 2 {
		t.Fatalf("expected 2 correlations, got %d", len(result))
	}
	if result[0].Confidence < result[1].Confidence {
		t.Error("expected correlations sorted by confidence descending")
	}
}

func TestPredictCascade_FilterAndSort(t *testing.T) {
	d := NewDetector(Config{MinOccurrences: 1})
	d.correlations["a"] = &Correlation{
		SourceID:     "node-1",
		TargetID:     "vm-1",
		EventPattern: "high_cpu -> restart",
		Occurrences:  2,
		AvgDelay:     time.Minute,
		Confidence:   0.6,
	}
	d.correlations["b"] = &Correlation{
		SourceID:     "node-1",
		TargetID:     "vm-2",
		EventPattern: "high_cpu -> restart",
		Occurrences:  2,
		AvgDelay:     2 * time.Minute,
		Confidence:   0.9,
	}
	d.correlations["c"] = &Correlation{
		SourceID:     "node-1",
		TargetID:     "vm-3",
		EventPattern: "high_mem -> restart",
		Occurrences:  2,
		AvgDelay:     time.Minute,
		Confidence:   0.95,
	}

	predictions := d.PredictCascade("node-1", EventHighCPU)
	if len(predictions) != 2 {
		t.Fatalf("expected 2 predictions, got %d", len(predictions))
	}
	if predictions[0].Confidence < predictions[1].Confidence {
		t.Error("expected predictions sorted by confidence descending")
	}
}

func TestFormatForContext_LimitsAndFallback(t *testing.T) {
	d := NewDetector(Config{MinOccurrences: 1})
	for i := 0; i < 12; i++ {
		desc := ""
		if i%2 == 0 {
			desc = "Observed pattern"
		}
		key := "k-" + intToStr(i)
		d.correlations[key] = &Correlation{
			SourceID:     "node-1",
			TargetID:     "vm-" + intToStr(i),
			EventPattern: "high_cpu -> restart",
			Occurrences:  2,
			Confidence:   0.3 + float64(i)*0.01,
			Description:  desc,
		}
	}

	result := d.FormatForContext("")
	if !strings.Contains(result, "... and more") {
		t.Error("expected context truncation indicator")
	}
	if !strings.Contains(result, "confidence") {
		t.Error("expected confidence fallback text")
	}
}

func TestSaveToDisk_EmptyDataDir(t *testing.T) {
	d := NewDetector(DefaultConfig())
	if err := d.saveToDisk(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSaveToDisk_MarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	d := NewDetector(Config{DataDir: tmpDir})
	d.events = append(d.events, Event{
		ID:         "event-1",
		ResourceID: "node-1",
		EventType:  EventHighCPU,
		Timestamp:  time.Now(),
		Value:      math.NaN(),
	})
	d.correlations["nil"] = nil

	if err := d.saveToDisk(); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestSaveToDisk_Success(t *testing.T) {
	tmpDir := t.TempDir()
	d := NewDetector(Config{DataDir: tmpDir})
	d.events = append(d.events, Event{
		ID:         "event-1",
		ResourceID: "node-1",
		EventType:  EventHighCPU,
		Timestamp:  time.Now(),
		Value:      1.0,
	})
	d.correlations["ok"] = &Correlation{
		SourceID:     "node-1",
		TargetID:     "vm-1",
		EventPattern: "high_cpu -> restart",
		Occurrences:  1,
		AvgDelay:     time.Minute,
		Confidence:   0.4,
		LastSeen:     time.Now(),
	}

	if err := d.saveToDisk(); err != nil {
		t.Fatalf("expected saveToDisk to succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "ai_correlations.json")); err != nil {
		t.Fatalf("expected correlations file, got error: %v", err)
	}
}

func TestSaveToDisk_WriteError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "correlation-data")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	defer os.Remove(path)

	d := NewDetector(Config{DataDir: path})
	if err := d.saveToDisk(); err == nil {
		t.Fatal("expected write error")
	}
}

func TestSaveToDisk_RenameError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_correlations.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatalf("create dir: %v", err)
	}

	d := NewDetector(Config{DataDir: tmpDir})
	if err := d.saveToDisk(); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestLoadFromDisk_EmptyDataDir(t *testing.T) {
	d := &Detector{}
	if err := d.loadFromDisk(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestLoadFromDisk_TooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_correlations.json")
	blob := make([]byte, (10<<20)+1)
	if err := os.WriteFile(path, blob, 0600); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	d := &Detector{dataDir: tmpDir}
	if err := d.loadFromDisk(); err == nil {
		t.Fatal("expected size error")
	}
}

func TestLoadFromDisk_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_correlations.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatalf("create dir: %v", err)
	}

	d := &Detector{dataDir: tmpDir}
	if err := d.loadFromDisk(); err == nil {
		t.Fatal("expected read error")
	}
}

func TestLoadFromDisk_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_correlations.json")
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}

	d := &Detector{dataDir: tmpDir}
	if err := d.loadFromDisk(); err == nil {
		t.Fatal("expected json parse error")
	}
}

func TestLoadFromDisk_NullCorrelations(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_correlations.json")
	payload := `{"events":[{"id":"event-1","resource_id":"node-1","event_type":"high_cpu","timestamp":"2024-01-01T00:00:00Z"}],"correlations":null}`
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatalf("write json: %v", err)
	}

	d := &Detector{
		dataDir:         tmpDir,
		maxEvents:       100,
		retentionWindow: 24 * time.Hour,
	}
	if err := d.loadFromDisk(); err != nil {
		t.Fatalf("loadFromDisk failed: %v", err)
	}
	if d.correlations == nil {
		t.Fatal("expected correlations map to be initialized")
	}
}

func TestLoadFromDisk_NotExist(t *testing.T) {
	tmpDir := t.TempDir()
	d := &Detector{dataDir: tmpDir}
	if err := d.loadFromDisk(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestLoadFromDisk_PruneAndNormalize(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now()
	data := struct {
		Events       []Event                 `json:"events"`
		Correlations map[string]*Correlation `json:"correlations"`
	}{
		Events: []Event{
			{
				ID:         "old",
				ResourceID: "node-1",
				EventType:  EventHighCPU,
				Timestamp:  now.Add(-2 * time.Hour),
			},
			{
				ID:         "new",
				ResourceID: "node-1",
				EventType:  EventHighCPU,
				Timestamp:  now,
			},
		},
		Correlations: map[string]*Correlation{
			"old": {
				SourceID:     "node-1",
				TargetID:     "vm-1",
				EventPattern: "high_cpu -> restart",
				Occurrences:  1,
				Confidence:   0.4,
				LastSeen:     now.Add(-2 * time.Hour),
			},
			"nil": nil,
			"new": {
				SourceID:     "node-1",
				TargetID:     "vm-2",
				EventPattern: "high_cpu -> restart",
				Occurrences:  1,
				Confidence:   0.4,
				LastSeen:     now,
			},
		},
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	path := filepath.Join(tmpDir, "ai_correlations.json")
	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		t.Fatalf("write data: %v", err)
	}

	d := &Detector{
		dataDir:         tmpDir,
		maxEvents:       100,
		retentionWindow: time.Hour,
	}
	if err := d.loadFromDisk(); err != nil {
		t.Fatalf("loadFromDisk failed: %v", err)
	}
	if d.correlations == nil {
		t.Fatal("expected correlations map to be initialized")
	}
	if _, ok := d.correlations["old"]; ok {
		t.Error("expected old correlation to be pruned")
	}
	if _, ok := d.correlations["nil"]; ok {
		t.Error("expected nil correlation to be pruned")
	}
	if _, ok := d.correlations["new"]; !ok {
		t.Error("expected recent correlation to remain")
	}
	if len(d.events) != 1 {
		t.Errorf("expected 1 recent event, got %d", len(d.events))
	}
}
