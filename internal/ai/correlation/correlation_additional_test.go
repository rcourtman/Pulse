package correlation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetector_RecordEventAndCorrelation(t *testing.T) {
	cfg := Config{
		MaxEvents:         10,
		CorrelationWindow: time.Minute,
		MinOccurrences:    1,
		RetentionWindow:   time.Hour,
	}
	d := NewDetector(cfg)

	start := time.Now()
	d.RecordEvent(Event{ResourceID: "a", EventType: EventAlert, Timestamp: start})
	d.RecordEvent(Event{ResourceID: "b", EventType: EventRestart, Timestamp: start.Add(10 * time.Second)})
	d.RecordEvent(Event{ResourceID: "a", EventType: EventAlert, Timestamp: start.Add(20 * time.Second)})
	d.RecordEvent(Event{ResourceID: "b", EventType: EventRestart, Timestamp: start.Add(30 * time.Second)})

	corrs := d.GetCorrelations()
	if len(corrs) == 0 {
		t.Fatalf("expected correlations")
	}
	if corrs[0].Occurrences < 2 {
		t.Fatalf("expected occurrences to increase")
	}
}

func TestDetector_ConfidenceAndFormatting(t *testing.T) {
	d := NewDetector(Config{MinOccurrences: 3})
	if d.calculateConfidence(1) != 0.1 {
		t.Fatalf("expected low confidence")
	}
	if d.calculateConfidence(3) < 0.3 {
		t.Fatalf("expected baseline confidence for threshold")
	}

	c := &Correlation{
		SourceID:     "src",
		TargetID:     "dst",
		EventPattern: "alert -> restart",
		AvgDelay:     2 * time.Minute,
	}
	desc := d.formatCorrelationDescription(c)
	if !strings.Contains(desc, "src") || !strings.Contains(desc, "dst") {
		t.Fatalf("expected description to use IDs")
	}

	if formatDuration(30*time.Second) != "seconds" {
		t.Fatalf("expected seconds duration")
	}
	if formatDuration(1*time.Minute) != "1 minute" {
		t.Fatalf("expected singular minute")
	}
	if formatDuration(2*time.Hour) != "2 hours" {
		t.Fatalf("expected hour format")
	}
	if formatConfidence(0.5) != "50%" {
		t.Fatalf("expected confidence percent")
	}
}

func TestDetector_DependencyQueries(t *testing.T) {
	d := NewDetector(Config{MinOccurrences: 1})
	d.correlations[correlationKey("a", "b", EventAlert, EventRestart)] = &Correlation{
		SourceID:     "a",
		TargetID:     "b",
		Occurrences:  1,
		EventPattern: "alert -> restart",
		Confidence:   0.5,
	}

	if len(d.GetDependencies("a")) != 1 {
		t.Fatalf("expected dependency")
	}
	if len(d.GetDependsOn("b")) != 1 {
		t.Fatalf("expected depends-on")
	}

	predictions := d.PredictCascade("a", EventAlert)
	if len(predictions) != 1 {
		t.Fatalf("expected cascade prediction")
	}
}

func TestDetector_FormatForContextAndTrim(t *testing.T) {
	d := NewDetector(Config{MinOccurrences: 1})
	if d.FormatForContext("") != "" {
		t.Fatalf("expected empty context without correlations")
	}

	d.correlations["k1"] = &Correlation{
		SourceID:     "a",
		TargetID:     "b",
		Occurrences:  2,
		Confidence:   0.5,
		EventPattern: "alert -> restart",
		Description:  "desc",
	}
	out := d.FormatForContext("")
	if !strings.Contains(out, "desc") {
		t.Fatalf("expected description in context")
	}

	d.maxEvents = 2
	d.retentionWindow = time.Minute
	d.events = []Event{
		{Timestamp: time.Now().Add(-2 * time.Minute)},
		{Timestamp: time.Now()},
		{Timestamp: time.Now()},
	}
	d.trimEvents()
	if len(d.events) != 2 {
		t.Fatalf("expected trimmed events")
	}
}

func TestDetector_SaveLoadAndLargeFile(t *testing.T) {
	dir := t.TempDir()
	d := NewDetector(Config{
		DataDir:         dir,
		MinOccurrences:  1,
		RetentionWindow: time.Hour,
	})
	d.events = []Event{{ID: "e1", ResourceID: "r1", EventType: EventAlert, Timestamp: time.Now()}}
	d.correlations["k1"] = &Correlation{
		SourceID:    "r1",
		TargetID:    "r2",
		Occurrences: 1,
		LastSeen:    time.Now(),
	}

	if err := d.saveToDisk(); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	loaded := NewDetector(Config{DataDir: dir, MinOccurrences: 1})
	if len(loaded.events) == 0 {
		t.Fatalf("expected events to load")
	}

	path := filepath.Join(dir, "ai_correlations.json")
	large := make([]byte, (10<<20)+1)
	if err := os.WriteFile(path, large, 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := loaded.loadFromDisk(); err == nil {
		t.Fatalf("expected error for large file")
	}
}

func TestDetector_LoadMissingAndInvalid(t *testing.T) {
	dir := t.TempDir()
	d := NewDetector(Config{DataDir: dir})
	if err := d.loadFromDisk(); err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}

	path := filepath.Join(dir, "ai_correlations.json")
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := d.loadFromDisk(); err == nil {
		t.Fatalf("expected error for invalid json")
	}
}

func TestDetector_HelperFunctions(t *testing.T) {
	if generateEventID() == "" {
		t.Fatalf("expected event id")
	}
	if intToStr(12) != "12" || intToStr(0) != "0" {
		t.Fatalf("expected intToStr")
	}
	if correlationKey("a", "b", EventAlert, EventRestart) == "" {
		t.Fatalf("expected correlation key")
	}
}
