package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestChangeDetector_DefaultsAndHelpers(t *testing.T) {
	detector := NewChangeDetector(ChangeDetectorConfig{})
	if detector.maxChanges != 1000 {
		t.Fatalf("expected default maxChanges=1000, got %d", detector.maxChanges)
	}

	if got := intToString(0); got != "0" {
		t.Errorf("intToString(0) = %q", got)
	}
	if got := intToString(42); got != "42" {
		t.Errorf("intToString(42) = %q", got)
	}
	if got := formatFloat(2.0); got != "2" {
		t.Errorf("formatFloat(2.0) = %q", got)
	}
	if got := formatFloat(2.5); got != "2.5" {
		t.Errorf("formatFloat(2.5) = %q", got)
	}

	cpu := formatCPUChangeDescription("vm-1", 4, 2)
	if !strings.Contains(cpu, "decreased") {
		t.Errorf("expected cpu decrease description, got %q", cpu)
	}
	mem := formatMemoryChangeDescription("vm-1", 8<<30, 4<<30)
	if !strings.Contains(mem, "decreased") {
		t.Errorf("expected memory decrease description, got %q", mem)
	}
}

func TestNewChangeDetector_LoadsFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_changes.json")
	changes := []Change{
		{ID: "c1", DetectedAt: time.Now().Add(-2 * time.Hour)},
	}
	data, err := json.Marshal(changes)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	detector := NewChangeDetector(ChangeDetectorConfig{
		MaxChanges: 10,
		DataDir:    tmpDir,
	})
	if len(detector.changes) != 1 {
		t.Fatalf("expected 1 change loaded, got %d", len(detector.changes))
	}
}

func TestNewChangeDetector_LoadError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_changes.json")
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	detector := NewChangeDetector(ChangeDetectorConfig{
		MaxChanges: 10,
		DataDir:    tmpDir,
	})
	if len(detector.changes) != 0 {
		t.Fatalf("expected no changes after load error, got %d", len(detector.changes))
	}
}

func TestChangeDetector_SaveToDisk_Scenarios(t *testing.T) {
	t.Run("NoDataDir", func(t *testing.T) {
		d := &ChangeDetector{}
		if err := d.saveToDisk(); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("MissingDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		missing := filepath.Join(tmpDir, "missing")
		d := &ChangeDetector{
			dataDir: missing,
			changes: []Change{{ID: "c1"}},
		}
		if err := d.saveToDisk(); err == nil {
			t.Fatal("expected error for missing directory")
		}
	})

	t.Run("MarshalError", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := &ChangeDetector{
			dataDir: tmpDir,
			changes: []Change{{ID: "c1", Before: func() {}}},
		}
		if err := d.saveToDisk(); err == nil {
			t.Fatal("expected marshal error")
		}
	})

	t.Run("RenameError", func(t *testing.T) {
		tmpDir := t.TempDir()
		destDir := filepath.Join(tmpDir, "ai_changes.json")
		if err := os.MkdirAll(destDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		d := &ChangeDetector{
			dataDir: tmpDir,
			changes: []Change{{ID: "c1"}},
		}
		if err := d.saveToDisk(); err == nil {
			t.Fatal("expected rename error")
		}
	})

	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := &ChangeDetector{
			dataDir: tmpDir,
			changes: []Change{{ID: "c1"}},
		}
		if err := d.saveToDisk(); err != nil {
			t.Fatalf("saveToDisk error: %v", err)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "ai_changes.json")); err != nil {
			t.Fatalf("expected file to exist: %v", err)
		}
	})
}

func TestChangeDetector_LoadFromDisk_Scenarios(t *testing.T) {
	t.Run("NoFile", func(t *testing.T) {
		d := &ChangeDetector{dataDir: t.TempDir()}
		if err := d.loadFromDisk(); err != nil {
			t.Fatalf("expected nil error for missing file, got %v", err)
		}
	})

	t.Run("EmptyDataDir", func(t *testing.T) {
		d := &ChangeDetector{dataDir: ""}
		if err := d.loadFromDisk(); err != nil {
			t.Fatalf("expected nil error for empty dataDir, got %v", err)
		}
	})

	t.Run("ReadError", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "not-dir")
		if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		d := &ChangeDetector{dataDir: filePath}
		if err := d.loadFromDisk(); err == nil {
			t.Fatalf("expected read error")
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "ai_changes.json")
		if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		d := &ChangeDetector{dataDir: tmpDir}
		if err := d.loadFromDisk(); err == nil {
			t.Fatal("expected JSON error")
		}
	})

	t.Run("TooLarge", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "ai_changes.json")
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := file.Truncate(10<<20 + 1); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}

		d := &ChangeDetector{dataDir: tmpDir}
		if err := d.loadFromDisk(); err == nil {
			t.Fatal("expected size error")
		}
	})

	t.Run("SortedAndTrimmed", func(t *testing.T) {
		tmpDir := t.TempDir()
		now := time.Now()
		changes := []Change{
			{ID: "c2", DetectedAt: now.Add(-1 * time.Hour)},
			{ID: "c1", DetectedAt: now.Add(-2 * time.Hour)},
			{ID: "c3", DetectedAt: now.Add(-10 * time.Minute)},
		}
		data, err := json.Marshal(changes)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		path := filepath.Join(tmpDir, "ai_changes.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		d := &ChangeDetector{
			dataDir:    tmpDir,
			maxChanges: 2,
		}
		if err := d.loadFromDisk(); err != nil {
			t.Fatalf("loadFromDisk error: %v", err)
		}
		if len(d.changes) != 2 {
			t.Fatalf("expected 2 changes after trim, got %d", len(d.changes))
		}
		if !d.changes[0].DetectedAt.Equal(changes[0].DetectedAt) {
			t.Fatalf("expected oldest remaining change to be c2")
		}
	})
}

func TestChangeDetector_DetectChanges_SaveError(t *testing.T) {
	tmpDir := t.TempDir()
	badDir := filepath.Join(tmpDir, "not-dir")
	if err := os.WriteFile(badDir, []byte("x"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	detector := NewChangeDetector(ChangeDetectorConfig{DataDir: badDir})
	detector.DetectChanges([]ResourceSnapshot{
		{ID: "vm-1", Name: "vm-1", Type: "vm", Status: "running"},
	})
	time.Sleep(20 * time.Millisecond)
}

func TestIncidentStore_DefaultsAndSummary(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{})
	if store.maxIncidents != defaultIncidentMaxIncidents {
		t.Fatalf("expected default max incidents, got %d", store.maxIncidents)
	}
	if store.maxEvents != defaultIncidentMaxEvents {
		t.Fatalf("expected default max events, got %d", store.maxEvents)
	}
	if store.maxAge != time.Duration(defaultIncidentMaxAgeDays)*24*time.Hour {
		t.Fatalf("expected default max age, got %v", store.maxAge)
	}

	if got := formatAlertSummary(nil); got != "Alert triggered" {
		t.Fatalf("unexpected nil summary %q", got)
	}
	noValue := formatAlertSummary(&alerts.Alert{Type: "cpu", Level: alerts.AlertLevelWarning})
	if !strings.Contains(noValue, "Alert triggered: cpu (warning)") {
		t.Fatalf("unexpected summary %q", noValue)
	}
	withValue := formatAlertSummary(&alerts.Alert{
		Type:      "cpu",
		Level:     alerts.AlertLevelCritical,
		Value:     90,
		Threshold: 80,
	})
	if !strings.Contains(withValue, ">= 80.0") {
		t.Fatalf("expected threshold summary, got %q", withValue)
	}
}

func TestNewIncidentStore_LoadsFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, incidentFileName)
	incidents := []*Incident{
		{ID: "inc-1", AlertID: "alert-1", Status: IncidentStatusOpen, OpenedAt: time.Now()},
	}
	data, err := json.Marshal(incidents)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store := NewIncidentStore(IncidentStoreConfig{DataDir: tmpDir})
	if len(store.incidents) != 1 {
		t.Fatalf("expected incidents loaded, got %d", len(store.incidents))
	}
}

func TestNewIncidentStore_LoadError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, incidentFileName)
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store := NewIncidentStore(IncidentStoreConfig{DataDir: tmpDir})
	if len(store.incidents) != 0 {
		t.Fatalf("expected no incidents after load error")
	}
}

func TestIncidentStore_RecordAlertFired_Existing(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{})
	store.RecordAlertFired(nil)

	alert := &alerts.Alert{
		ID:           "alert-fired",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "res-1",
		ResourceName: "vm-1",
		Message:      "original",
	}
	store.RecordAlertFired(alert)
	alert.Message = "updated"
	store.RecordAlertFired(alert)

	timeline := store.GetTimelineByAlertID(alert.ID)
	if timeline == nil {
		t.Fatalf("expected timeline")
	}
	if timeline.Message != "updated" {
		t.Fatalf("expected updated message, got %q", timeline.Message)
	}
	if len(timeline.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(timeline.Events))
	}
}

func TestIncidentStore_RecordAlertAcknowledged_WithAckTime(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{})
	ackTime := time.Now().Add(-5 * time.Minute)
	alert := &alerts.Alert{
		ID:      "alert-ack",
		Type:    "memory",
		Level:   alerts.AlertLevelWarning,
		AckTime: &ackTime,
	}
	store.RecordAlertAcknowledged(nil, "user")
	store.RecordAlertAcknowledged(alert, "user")

	timeline := store.GetTimelineByAlertID(alert.ID)
	if timeline == nil {
		t.Fatalf("expected timeline")
	}
	if timeline.AckTime == nil || !timeline.AckTime.Equal(ackTime) {
		t.Fatalf("expected ack time to match")
	}
	if timeline.AckUser != "user" {
		t.Fatalf("expected ack user")
	}
}

func TestIncidentStore_RecordAlertResolved_ZeroTime(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{})
	alert := &alerts.Alert{
		ID:    "alert-resolved",
		Type:  "disk",
		Level: alerts.AlertLevelCritical,
	}
	store.RecordAlertResolved(nil, time.Time{})
	store.RecordAlertResolved(alert, time.Time{})

	timeline := store.GetTimelineByAlertID(alert.ID)
	if timeline == nil || timeline.ClosedAt == nil {
		t.Fatalf("expected closed incident")
	}
}

func TestIncidentStore_RecordAnalysis_CommandDetails(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{})
	store.RecordAnalysis("", "", nil)
	store.RecordAnalysis("alert-analysis", "", nil)

	timeline := store.GetTimelineByAlertID("alert-analysis")
	if timeline == nil {
		t.Fatalf("expected timeline")
	}
	if len(timeline.Events) == 0 || timeline.Events[0].Summary != "AI analysis completed" {
		t.Fatalf("expected default analysis summary")
	}

	store.RecordCommand("", "", false, "", nil)
	store.RecordCommand("alert-cmd", "echo test", false, "", nil)

	cmdTimeline := store.GetTimelineByAlertID("alert-cmd")
	if cmdTimeline == nil || len(cmdTimeline.Events) == 0 {
		t.Fatalf("expected command event")
	}
	if _, ok := cmdTimeline.Events[0].Details["output_excerpt"]; ok {
		t.Fatalf("did not expect output_excerpt for empty output")
	}
}

func TestIncidentStore_Timelines_EmptyAndZeroTime(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{})
	if got := store.GetTimelineByAlertID(""); got != nil {
		t.Fatalf("expected nil for empty alert ID")
	}
	if got := store.GetTimelineByAlertAt("", time.Now()); got != nil {
		t.Fatalf("expected nil for empty alert ID in GetTimelineByAlertAt")
	}
	if got := store.GetTimelineByAlertAt("missing-alert", time.Now()); got != nil {
		t.Fatalf("expected nil for missing alert")
	}

	alert := &alerts.Alert{
		ID:           "alert-zero",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceName: "vm-1",
		StartTime:    time.Now().Add(-10 * time.Minute),
	}
	store.RecordAlertFired(alert)

	timeline := store.GetTimelineByAlertAt(alert.ID, time.Time{})
	if timeline == nil || timeline.AlertID != alert.ID {
		t.Fatalf("expected timeline for zero start time")
	}
	timeline = store.GetTimelineByAlertAt(alert.ID, alert.StartTime.Add(5*time.Minute))
	if timeline == nil || timeline.AlertID != alert.ID {
		t.Fatalf("expected timeline for later start time")
	}
}

func TestIncidentStore_GetTimelineByAlertAt_SkipsMismatched(t *testing.T) {
	store := &IncidentStore{
		incidents: []*Incident{
			nil,
			{ID: "inc-a", AlertID: "alert-a", OpenedAt: time.Now().Add(-10 * time.Minute)},
			{ID: "inc-b", AlertID: "alert-b", OpenedAt: time.Now().Add(-5 * time.Minute)},
		},
	}

	timeline := store.GetTimelineByAlertAt("alert-b", time.Now())
	if timeline == nil || timeline.AlertID != "alert-b" {
		t.Fatalf("expected timeline for alert-b")
	}
}

func TestIncidentStore_FormatForPatrol_MessageFallback(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{})
	store.incidents = append(store.incidents, &Incident{
		ID:       "inc-message",
		AlertID:  "alert-message",
		Status:   IncidentStatusOpen,
		OpenedAt: time.Now(),
		Message:  "fallback message",
	})
	store.incidents = append(store.incidents, &Incident{
		ID:           "inc-ack",
		AlertID:      "alert-ack",
		Status:       IncidentStatusOpen,
		OpenedAt:     time.Now().Add(1 * time.Minute),
		Acknowledged: true,
		ResourceName: "vm-ack",
		AlertType:    "cpu",
	})
	store.incidents = append(store.incidents, nil)

	result := store.FormatForPatrol(2)
	if !strings.Contains(result, "fallback message") {
		t.Fatalf("expected message fallback in patrol output")
	}
	if !strings.Contains(result, "acknowledged") {
		t.Fatalf("expected acknowledged status in patrol output")
	}
}

func TestIncidentStore_HelperPaths(t *testing.T) {
	store := &IncidentStore{
		incidents:    make([]*Incident, 0),
		maxEvents:    1,
		maxIncidents: 1,
		maxAge:       30 * time.Minute,
	}

	alert := &alerts.Alert{
		ID:    "alert-helper",
		Type:  "cpu",
		Level: alerts.AlertLevelWarning,
	}

	incident := store.ensureIncidentForAlertLocked(alert)
	if incident == nil || len(store.incidents) != 1 {
		t.Fatalf("expected incident created")
	}
	store.ensureIncidentForAlertLocked(alert)
	if len(store.incidents) != 1 {
		t.Fatalf("expected same incident to be reused")
	}

	updateIncidentFromAlert(nil, alert)
	updateIncidentFromAlert(incident, nil)

	store.addEventLocked(nil, IncidentEventAnalysis, "", nil)
	store.addEventLocked(incident, IncidentEventAnalysis, "", nil)
	store.addEventLocked(incident, IncidentEventNote, "note", nil)
	if len(incident.Events) != 1 || incident.Events[0].Type != IncidentEventNote {
		t.Fatalf("expected events trimmed to last entry")
	}
	if incident.Events[0].Summary == "" {
		t.Fatalf("expected summary to be set")
	}

	store.incidents = append([]*Incident{nil}, store.incidents...)
	if store.findOpenIncidentByAlertIDLocked("") != nil {
		t.Fatalf("expected nil for empty alert ID")
	}
	if store.findLatestIncidentByAlertIDLocked("") != nil {
		t.Fatalf("expected nil for empty alert ID")
	}
	if store.findIncidentByIDLocked("") != nil {
		t.Fatalf("expected nil for empty incident ID")
	}

	oldClosed := time.Now().Add(-2 * time.Hour)
	store.incidents = []*Incident{
		nil,
		{ID: "old-open", AlertID: "old", Status: IncidentStatusOpen, OpenedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "old-closed", AlertID: "oldc", Status: IncidentStatusResolved, OpenedAt: time.Now().Add(-3 * time.Hour), ClosedAt: &oldClosed},
		{ID: "recent", AlertID: "recent", Status: IncidentStatusOpen, OpenedAt: time.Now().Add(-5 * time.Minute)},
		{ID: "recent2", AlertID: "recent2", Status: IncidentStatusOpen, OpenedAt: time.Now().Add(-4 * time.Minute)},
	}
	store.trimLocked()
	if len(store.incidents) != 1 || store.incidents[0].ID != "recent2" {
		t.Fatalf("expected trim to keep most recent incident")
	}
}

func TestIncidentStore_SaveAsyncAndPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	store := &IncidentStore{
		incidents: []*Incident{
			{ID: "inc-1", AlertID: "alert-1", Status: IncidentStatusOpen, OpenedAt: time.Now()},
		},
		dataDir:  tmpDir,
		filePath: filepath.Join(tmpDir, incidentFileName),
	}

	store.saveAsync()

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		if _, err := os.Stat(store.filePath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected saveAsync to create file")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestIncidentStore_SaveAsync_Error(t *testing.T) {
	tmpDir := t.TempDir()
	badDir := filepath.Join(tmpDir, "not-dir")
	if err := os.WriteFile(badDir, []byte("x"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store := &IncidentStore{
		incidents: []*Incident{
			{ID: "inc-err", AlertID: "alert-err", Status: IncidentStatusOpen, OpenedAt: time.Now()},
		},
		dataDir:  badDir,
		filePath: filepath.Join(badDir, incidentFileName),
	}
	store.saveAsync()
	time.Sleep(20 * time.Millisecond)
}

func TestIncidentStore_SaveToDisk_Scenarios(t *testing.T) {
	t.Run("NoDataDir", func(t *testing.T) {
		store := &IncidentStore{}
		if err := store.saveToDisk(); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("MkdirError", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "not-a-dir")
		if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		store := &IncidentStore{
			dataDir:  filePath,
			filePath: filepath.Join(filePath, incidentFileName),
		}
		if err := store.saveToDisk(); err == nil {
			t.Fatalf("expected mkdir error")
		}
	})

	t.Run("WriteError", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, incidentFileName)
		tmpFile := filePath + ".tmp"
		if err := os.MkdirAll(tmpFile, 0755); err != nil {
			t.Fatalf("mkdir tmp: %v", err)
		}
		store := &IncidentStore{
			dataDir:  tmpDir,
			filePath: filePath,
		}
		if err := store.saveToDisk(); err == nil {
			t.Fatalf("expected write error")
		}
	})

	t.Run("MarshalError", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := &IncidentStore{
			incidents: []*Incident{
				{
					ID:       "inc-1",
					AlertID:  "alert-1",
					Status:   IncidentStatusOpen,
					OpenedAt: time.Now(),
					Events: []IncidentEvent{
						{
							ID:        "evt-1",
							Type:      IncidentEventNote,
							Timestamp: time.Now(),
							Summary:   "note",
							Details:   map[string]interface{}{"bad": make(chan int)},
						},
					},
				},
			},
			dataDir:  tmpDir,
			filePath: filepath.Join(tmpDir, incidentFileName),
		}
		if err := store.saveToDisk(); err == nil {
			t.Fatalf("expected marshal error")
		}
	})

	t.Run("RenameError", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, incidentFileName)
		if err := os.MkdirAll(filePath, 0755); err != nil {
			t.Fatalf("mkdir file path: %v", err)
		}
		store := &IncidentStore{
			incidents: []*Incident{
				{ID: "inc-1", AlertID: "alert-1", Status: IncidentStatusOpen, OpenedAt: time.Now()},
			},
			dataDir:  tmpDir,
			filePath: filePath,
		}
		if err := store.saveToDisk(); err == nil {
			t.Fatalf("expected rename error")
		}
	})

	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := &IncidentStore{
			incidents: []*Incident{
				{
					ID:       "inc-1",
					AlertID:  "alert-1",
					Status:   IncidentStatusOpen,
					OpenedAt: time.Now(),
					Events: []IncidentEvent{
						{ID: "evt-1", Type: IncidentEventNote, Timestamp: time.Now(), Summary: "note", Details: map[string]interface{}{"k": "v"}},
					},
				},
			},
			dataDir:  tmpDir,
			filePath: filepath.Join(tmpDir, incidentFileName),
		}
		if err := store.saveToDisk(); err != nil {
			t.Fatalf("saveToDisk error: %v", err)
		}
		if _, err := os.Stat(store.filePath); err != nil {
			t.Fatalf("expected file to exist: %v", err)
		}
	})
}

func TestIncidentStore_LoadFromDisk_Scenarios(t *testing.T) {
	t.Run("EmptyFilePath", func(t *testing.T) {
		store := &IncidentStore{filePath: ""}
		if err := store.loadFromDisk(); err != nil {
			t.Fatalf("expected nil error for empty file path, got %v", err)
		}
	})

	t.Run("NoFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := &IncidentStore{
			filePath: filepath.Join(tmpDir, incidentFileName),
		}
		if err := store.loadFromDisk(); err != nil {
			t.Fatalf("expected nil error for missing file, got %v", err)
		}
	})

	t.Run("ReadError", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, incidentFileName)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		store := &IncidentStore{filePath: path}
		if err := store.loadFromDisk(); err == nil {
			t.Fatalf("expected read error")
		}
	})

	t.Run("StatError", func(t *testing.T) {
		tmpDir := t.TempDir()
		notDir := filepath.Join(tmpDir, "not-dir")
		if err := os.WriteFile(notDir, []byte("x"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		store := &IncidentStore{
			filePath: filepath.Join(notDir, incidentFileName),
		}
		if err := store.loadFromDisk(); err == nil {
			t.Fatalf("expected stat error")
		}
	})

	t.Run("FileTooLarge", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, incidentFileName)
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := file.Truncate(maxIncidentFileSize + 1); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		store := &IncidentStore{filePath: path}
		if err := store.loadFromDisk(); err == nil {
			t.Fatalf("expected size error")
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, incidentFileName)
		if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		store := &IncidentStore{filePath: path}
		if err := store.loadFromDisk(); err == nil {
			t.Fatalf("expected JSON error")
		}
	})

	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, incidentFileName)
		closed := time.Now().Add(-90 * time.Minute)
		incidents := []*Incident{
			{
				ID:       "inc-a",
				AlertID:  "alert-a",
				Status:   IncidentStatusResolved,
				OpenedAt: time.Now().Add(-2 * time.Hour),
				ClosedAt: &closed,
			},
			{
				ID:       "inc-b",
				AlertID:  "alert-b",
				Status:   IncidentStatusOpen,
				OpenedAt: time.Now().Add(-10 * time.Minute),
			},
		}
		data, err := json.Marshal(incidents)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		store := &IncidentStore{
			filePath:    path,
			maxIncidents: 1,
			maxAge:       24 * time.Hour,
		}
		if err := store.loadFromDisk(); err != nil {
			t.Fatalf("loadFromDisk error: %v", err)
		}
		if len(store.incidents) != 1 {
			t.Fatalf("expected trimmed incidents, got %d", len(store.incidents))
		}
	})
}

func TestCloneIncident(t *testing.T) {
	if cloneIncident(nil) != nil {
		t.Fatalf("expected nil clone")
	}
	now := time.Now()
	ack := now.Add(-5 * time.Minute)
	closed := now.Add(-2 * time.Minute)
	incident := &Incident{
		ID:       "inc-1",
		AlertID:  "alert-1",
		Status:   IncidentStatusResolved,
		OpenedAt: now.Add(-10 * time.Minute),
		AckTime:  &ack,
		ClosedAt: &closed,
		Events: []IncidentEvent{
			{
				ID:        "evt-1",
				Type:      IncidentEventNote,
				Timestamp: now,
				Summary:   "note",
				Details:   map[string]interface{}{"key": "value"},
			},
			{
				ID:        "evt-2",
				Type:      IncidentEventAnalysis,
				Timestamp: now,
				Summary:   "analysis",
			},
		},
	}

	clone := cloneIncident(incident)
	if clone == nil || clone.AckTime == nil || clone.ClosedAt == nil {
		t.Fatalf("expected clone with ack and close time")
	}
	clone.Events[0].Details["key"] = "changed"
	if incident.Events[0].Details["key"] == "changed" {
		t.Fatalf("expected deep copy of details")
	}
}

func TestRemediationLog_DefaultsAndLog(t *testing.T) {
	log := NewRemediationLog(RemediationLogConfig{})
	if log.maxRecords != 500 {
		t.Fatalf("expected default max records, got %d", log.maxRecords)
	}

	if err := log.Log(RemediationRecord{Problem: "p", Action: "a"}); err != nil {
		t.Fatalf("log error: %v", err)
	}
	if len(log.records) != 1 {
		t.Fatalf("expected record logged")
	}
	if log.records[0].ID == "" || log.records[0].Timestamp.IsZero() {
		t.Fatalf("expected ID and Timestamp to be set")
	}
}

func TestRemediationLog_Log_SaveError(t *testing.T) {
	tmpDir := t.TempDir()
	badDir := filepath.Join(tmpDir, "not-dir")
	if err := os.WriteFile(badDir, []byte("x"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	log := &RemediationLog{
		dataDir:    badDir,
		maxRecords: 1,
	}
	if err := log.Log(RemediationRecord{Problem: "p", Action: "a"}); err != nil {
		t.Fatalf("log error: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
}

func TestNewRemediationLog_LoadsFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_remediations.json")
	records := []RemediationRecord{{ID: "r1", Problem: "p", Action: "a"}}
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	log := NewRemediationLog(RemediationLogConfig{DataDir: tmpDir})
	if len(log.records) != 1 {
		t.Fatalf("expected records loaded")
	}
}

func TestNewRemediationLog_LoadError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_remediations.json")
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	log := NewRemediationLog(RemediationLogConfig{DataDir: tmpDir})
	if len(log.records) != 0 {
		t.Fatalf("expected no records after load error")
	}
}

func TestRemediationLog_SimilarAndStatsBranches(t *testing.T) {
	log := NewRemediationLog(RemediationLogConfig{})
	if matches := log.GetSimilar("a b c", 5); matches != nil {
		t.Fatalf("expected nil for no keywords")
	}

	log.Log(RemediationRecord{Problem: "memory issue", Action: "a1", Outcome: OutcomePartial})
	log.Log(RemediationRecord{Problem: "memory issue", Action: "a2", Outcome: OutcomeFailed})

	success := log.GetSuccessfulRemediations("memory issue", 5)
	if len(success) != 1 || success[0].Outcome != OutcomePartial {
		t.Fatalf("expected partial to be included")
	}

	log.Log(RemediationRecord{Problem: "unknown", Action: "a3", Outcome: OutcomeUnknown})
	stats := log.GetRecentRemediationStats(time.Now().Add(-1 * time.Hour))
	if stats["unknown"] == 0 {
		t.Fatalf("expected unknown outcome to be counted")
	}
}

func TestRemediationLog_GetSuccessfulRemediations_Limit(t *testing.T) {
	log := NewRemediationLog(RemediationLogConfig{})
	log.Log(RemediationRecord{Problem: "disk full", Action: "a1", Outcome: OutcomeResolved})
	log.Log(RemediationRecord{Problem: "disk full", Action: "a2", Outcome: OutcomePartial})

	results := log.GetSuccessfulRemediations("disk full", 1)
	if len(results) != 1 {
		t.Fatalf("expected limited results, got %d", len(results))
	}
}

func TestRemediationLog_FormatAndStats(t *testing.T) {
	log := NewRemediationLog(RemediationLogConfig{})
	log.Log(RemediationRecord{
		ResourceID: "res-1",
		Problem:    "issue",
		Action:     "action",
		Outcome:    OutcomeUnknown,
		Note:       "note",
	})
	log.Log(RemediationRecord{
		ResourceID: "res-1",
		Problem:    "issue",
		Action:     "action",
		Outcome:    OutcomePartial,
	})

	formatted := log.FormatForContext("res-1", 5)
	if !strings.Contains(formatted, "Note: note") {
		t.Fatalf("expected note in formatted context")
	}

	stats := log.GetRemediationStats()
	if stats["unknown"] != 1 {
		t.Fatalf("expected unknown count")
	}
	if stats["partial"] != 1 {
		t.Fatalf("expected partial count")
	}
}

func TestRemediationLog_SaveLoad_Scenarios(t *testing.T) {
	t.Run("SaveNoDataDir", func(t *testing.T) {
		log := &RemediationLog{}
		if err := log.saveToDisk(); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("SaveMissingDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		missing := filepath.Join(tmpDir, "missing")
		log := &RemediationLog{
			dataDir: missing,
			records: []RemediationRecord{{ID: "r1", Problem: "p", Action: "a"}},
		}
		if err := log.saveToDisk(); err == nil {
			t.Fatalf("expected error for missing directory")
		}
	})

	t.Run("SaveRenameError", func(t *testing.T) {
		tmpDir := t.TempDir()
		destDir := filepath.Join(tmpDir, "ai_remediations.json")
		if err := os.MkdirAll(destDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		log := &RemediationLog{
			dataDir: tmpDir,
			records: []RemediationRecord{{ID: "r1", Problem: "p", Action: "a"}},
		}
		if err := log.saveToDisk(); err == nil {
			t.Fatalf("expected rename error")
		}
	})

	t.Run("SaveSuccess", func(t *testing.T) {
		tmpDir := t.TempDir()
		log := &RemediationLog{
			dataDir: tmpDir,
			records: []RemediationRecord{{ID: "r1", Problem: "p", Action: "a"}},
		}
		if err := log.saveToDisk(); err != nil {
			t.Fatalf("saveToDisk error: %v", err)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "ai_remediations.json")); err != nil {
			t.Fatalf("expected file to exist: %v", err)
		}
	})

	t.Run("SaveMarshalError", func(t *testing.T) {
		tmpDir := t.TempDir()
		log := &RemediationLog{
			dataDir: tmpDir,
			records: []RemediationRecord{
				{
					ID:        "r1",
					Problem:   "p",
					Action:    "a",
					Timestamp: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		}
		if err := log.saveToDisk(); err == nil {
			t.Fatalf("expected marshal error")
		}
	})

	t.Run("LoadEmptyDataDir", func(t *testing.T) {
		log := &RemediationLog{dataDir: ""}
		if err := log.loadFromDisk(); err != nil {
			t.Fatalf("expected nil error for empty dataDir, got %v", err)
		}
	})

	t.Run("LoadNoFile", func(t *testing.T) {
		log := &RemediationLog{dataDir: t.TempDir()}
		if err := log.loadFromDisk(); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("LoadReadError", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "not-dir")
		if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		log := &RemediationLog{dataDir: filePath}
		if err := log.loadFromDisk(); err == nil {
			t.Fatalf("expected read error")
		}
	})

	t.Run("LoadTooLarge", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "ai_remediations.json")
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := file.Truncate(10<<20 + 1); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}

		log := &RemediationLog{dataDir: tmpDir}
		if err := log.loadFromDisk(); err == nil {
			t.Fatalf("expected size error")
		}
	})

	t.Run("LoadInvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "ai_remediations.json")
		if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		log := &RemediationLog{dataDir: tmpDir}
		if err := log.loadFromDisk(); err == nil {
			t.Fatalf("expected JSON error")
		}
	})

	t.Run("LoadSuccess", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "ai_remediations.json")
		records := []RemediationRecord{
			{ID: "r2", Problem: "p2", Action: "a2", Timestamp: time.Now().Add(-1 * time.Hour)},
			{ID: "r1", Problem: "p1", Action: "a1", Timestamp: time.Now().Add(-2 * time.Hour)},
		}
		data, err := json.Marshal(records)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		log := &RemediationLog{
			dataDir:    tmpDir,
			maxRecords: 1,
		}
		if err := log.loadFromDisk(); err != nil {
			t.Fatalf("loadFromDisk error: %v", err)
		}
		if len(log.records) != 1 {
			t.Fatalf("expected trimmed records, got %d", len(log.records))
		}
	})
}
