package proxmox

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewEventCorrelator_Defaults(t *testing.T) {
	c := NewEventCorrelator(EventCorrelatorConfig{})
	if c.config.CorrelationWindow != 15*time.Minute {
		t.Fatalf("expected default correlation window")
	}
	if c.config.MaxEvents != 5000 {
		t.Fatalf("expected default max events")
	}
	if c.config.MaxCorrelations != 1000 {
		t.Fatalf("expected default max correlations")
	}
	if c.config.RetentionDays != 30 {
		t.Fatalf("expected default retention days")
	}
}

func TestRecordEvent_EndOperationClosesWindow(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	start := ProxmoxEvent{
		ID:         "start-1",
		Type:       EventMigrationStart,
		ResourceID: "vm-1",
		Node:       "pve1",
		TargetNode: "pve2",
		Timestamp:  time.Now().Add(-5 * time.Minute),
	}
	c.RecordEvent(start)

	if len(c.GetActiveOperations()) != 1 {
		t.Fatalf("expected active operation")
	}

	end := ProxmoxEvent{
		Type:       EventMigrationEnd,
		ResourceID: "vm-1",
		Timestamp:  time.Now(),
	}
	c.RecordEvent(end)

	if len(c.GetActiveOperations()) != 0 {
		t.Fatalf("expected active operation to close on end event")
	}
}

func TestRecordAnomaly_OutsideWindow(t *testing.T) {
	cfg := DefaultEventCorrelatorConfig()
	cfg.CorrelationWindow = time.Minute
	c := NewEventCorrelator(cfg)

	c.RecordEvent(ProxmoxEvent{
		Type:       EventBackupStart,
		ResourceID: "vm-1",
		Timestamp:  time.Now().Add(-10 * time.Minute),
	})

	anomaly := MetricAnomaly{
		ResourceID: "vm-1",
		Metric:     "cpu",
		Timestamp:  time.Now(),
	}
	if correlation := c.RecordAnomaly(anomaly); correlation != nil {
		t.Fatalf("expected no correlation outside window")
	}
}

func TestGetRecentEventsWithLimit(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	now := time.Now()
	c.RecordEvent(ProxmoxEvent{Type: EventVMStart, ResourceID: "vm-1", Timestamp: now.Add(-2 * time.Minute)})
	c.RecordEvent(ProxmoxEvent{Type: EventVMStop, ResourceID: "vm-1", Timestamp: now.Add(-1 * time.Minute)})

	events := c.GetRecentEventsWithLimit(10*time.Minute, 1)
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	if events[0].Type != EventVMStop {
		t.Fatalf("expected most recent event")
	}
}

func TestGetRecentEventsWithLimit_NoLimitAndCutoff(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	now := time.Now()
	c.RecordEvent(ProxmoxEvent{Type: EventVMStop, ResourceID: "vm-1", Timestamp: now.Add(-2 * time.Hour)})
	c.RecordEvent(ProxmoxEvent{Type: EventVMStart, ResourceID: "vm-1", Timestamp: now.Add(-2 * time.Minute)})
	c.RecordEvent(ProxmoxEvent{Type: EventVMStop, ResourceID: "vm-1", Timestamp: now.Add(-1 * time.Minute)})

	events := c.GetRecentEventsWithLimit(30*time.Minute, 0)
	if len(events) != 2 {
		t.Fatalf("expected cutoff to exclude old events, got %d", len(events))
	}
}

func TestGetEventsForResource_IncludesNodeAndStorage(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	c.RecordEvent(ProxmoxEvent{
		Type:    EventBackupStart,
		Node:    "pve1",
		Storage: "local",
	})

	if len(c.GetEventsForResource("pve1", 10)) != 1 {
		t.Fatalf("expected node match to return event")
	}
	if len(c.GetEventsForResource("local", 10)) != 1 {
		t.Fatalf("expected storage match to return event")
	}
}

func TestGetCorrelationsForResource(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	c.correlations = append(c.correlations, EventCorrelation{
		ID:                "c1",
		ImpactedResources: []string{"vm-1"},
	})
	if len(c.GetCorrelationsForResource("vm-1")) != 1 {
		t.Fatalf("expected correlation for resource")
	}
}

func TestGetActiveOperations_Expires(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	c.activeWindows["w1"] = &OperationWindow{
		EventID:     "w1",
		EventType:   EventBackupStart,
		StartTime:   time.Now().Add(-2 * time.Hour),
		ExpectedEnd: time.Now().Add(-1 * time.Hour),
	}
	if len(c.GetActiveOperations()) != 0 {
		t.Fatalf("expected expired active window to be filtered")
	}
}

func TestFormatForPatrol_IncludesActiveAndCorrelations(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	c.RecordEvent(ProxmoxEvent{
		Type:       EventMigrationStart,
		ResourceID: "vm-1",
		Node:       "pve1",
		TargetNode: "pve2",
	})
	c.RecordAnomaly(MetricAnomaly{
		ResourceID: "vm-1",
		Metric:     "cpu",
		Timestamp:  time.Now(),
	})

	context := c.FormatForPatrol(1 * time.Hour)
	if !containsStr(context, "Currently Active Operations") {
		t.Fatalf("expected active operations section")
	}
	if !containsStr(context, "Detected Correlations") {
		t.Fatalf("expected correlations section")
	}
}

func TestFormatForResource_NoData(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	if c.FormatForResource("vm-1") != "" {
		t.Fatalf("expected empty context when no data")
	}
}

func TestFormatForResource_WithCorrelation(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	c.RecordEvent(ProxmoxEvent{
		Type:       EventVMStart,
		ResourceID: "vm-1",
		Status:     "running",
	})
	c.correlations = append(c.correlations, EventCorrelation{
		ID:                "corr-1",
		ImpactedResources: []string{"vm-1"},
		Explanation:       "VM start spike explained",
		CreatedAt:         time.Now(),
	})

	context := c.FormatForResource("vm-1")
	if !containsStr(context, "Correlated events") {
		t.Fatalf("expected correlated events section")
	}
}

func TestTrimEventsAndCorrelations(t *testing.T) {
	cfg := DefaultEventCorrelatorConfig()
	cfg.MaxEvents = 2
	cfg.MaxCorrelations = 1
	cfg.RetentionDays = 1
	c := NewEventCorrelator(cfg)

	oldTime := time.Now().Add(-48 * time.Hour)
	c.events = []ProxmoxEvent{
		{ID: "e1", Timestamp: oldTime},
		{ID: "e2", Timestamp: time.Now()},
		{ID: "e3", Timestamp: time.Now()},
	}
	c.correlations = []EventCorrelation{
		{ID: "c1", CreatedAt: oldTime},
		{ID: "c2", CreatedAt: time.Now()},
	}

	c.trimEvents()
	c.trimCorrelations()

	if len(c.events) != 2 {
		t.Fatalf("expected events trimmed to max")
	}
	if len(c.correlations) != 1 {
		t.Fatalf("expected correlations trimmed to max")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	c := NewEventCorrelator(EventCorrelatorConfig{})
	c.events = append(c.events, ProxmoxEvent{
		ID:         "e1",
		Type:       EventBackupStart,
		ResourceID: "vm-1",
		Timestamp:  time.Now(),
	})
	c.correlations = append(c.correlations, EventCorrelation{
		ID:                "c1",
		Event:             ProxmoxEvent{ID: "e1", Type: EventBackupStart, ResourceID: "vm-1"},
		ImpactedResources: []string{"vm-1"},
		CreatedAt:         time.Now(),
	})
	c.dataDir = dir
	if err := c.saveToDisk(); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "proxmox_events.json")); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	loaded := NewEventCorrelator(EventCorrelatorConfig{DataDir: dir})
	if len(loaded.events) == 0 || len(loaded.correlations) == 0 {
		t.Fatalf("expected data to load")
	}
}

func TestSaveLoad_NoDir(t *testing.T) {
	c := NewEventCorrelator(EventCorrelatorConfig{})
	if err := c.saveToDisk(); err != nil {
		t.Fatalf("expected saveToDisk to no-op without DataDir, got %v", err)
	}
	if err := c.loadFromDisk(); err != nil {
		t.Fatalf("expected loadFromDisk to no-op without DataDir, got %v", err)
	}
}

func TestSaveIfDirty(t *testing.T) {
	dir := t.TempDir()
	c := NewEventCorrelator(EventCorrelatorConfig{DataDir: dir})
	c.events = append(c.events, ProxmoxEvent{ID: "e1", Timestamp: time.Now()})
	c.saveIfDirty()

	if _, err := os.Stat(filepath.Join(dir, "proxmox_events.json")); err != nil {
		t.Fatalf("expected saveIfDirty to persist data: %v", err)
	}
}

func TestSaveIfDirty_Error(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	c := NewEventCorrelator(EventCorrelatorConfig{DataDir: filePath})
	c.events = append(c.events, ProxmoxEvent{ID: "e1", Timestamp: time.Now()})
	c.saveIfDirty()
}

func TestSaveToDisk_Error(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	c := NewEventCorrelator(EventCorrelatorConfig{DataDir: filePath})
	c.events = append(c.events, ProxmoxEvent{ID: "e1", Timestamp: time.Now()})
	if err := c.saveToDisk(); err == nil {
		t.Fatalf("expected saveToDisk to fail with invalid data dir")
	}
}

func TestLoadFromDisk_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proxmox_events.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0600); err != nil {
		t.Fatalf("failed to write invalid json: %v", err)
	}

	c := NewEventCorrelator(EventCorrelatorConfig{DataDir: dir})
	if err := c.loadFromDisk(); err == nil {
		t.Fatalf("expected loadFromDisk to fail on invalid json")
	}
}

func TestCreateOperationWindow_Types(t *testing.T) {
	c := NewEventCorrelator(DefaultEventCorrelatorConfig())
	backup := c.createOperationWindow(ProxmoxEvent{Type: EventBackupStart, ResourceID: "vm-1", Storage: "local"})
	if len(backup.ExpectedMetrics) == 0 || len(backup.AffectedResources) == 0 {
		t.Fatalf("expected backup operation window to be populated")
	}

	snapshot := c.createOperationWindow(ProxmoxEvent{Type: EventSnapshotDelete, ResourceID: "vm-1", Storage: "local"})
	if len(snapshot.ExpectedMetrics) == 0 || len(snapshot.AffectedResources) == 0 {
		t.Fatalf("expected snapshot operation window to be populated")
	}

	ha := c.createOperationWindow(ProxmoxEvent{Type: EventHAFailover, ResourceID: "vm-1", Node: "pve1", TargetNode: "pve2"})
	if len(ha.ExpectedMetrics) == 0 || len(ha.AffectedResources) == 0 {
		t.Fatalf("expected HA operation window to be populated")
	}
}

func TestHelperFunctions(t *testing.T) {
	if !isOngoingOperation(EventBackupStart) || isOngoingOperation(EventBackupEnd) {
		t.Fatalf("expected ongoing operation detection")
	}
	if !isEndOperation(EventMigrationEnd) || isEndOperation(EventMigrationStart) {
		t.Fatalf("expected end operation detection")
	}

	start := ProxmoxEvent{Type: EventMigrationStart, ResourceID: "vm-1"}
	end := ProxmoxEvent{Type: EventMigrationEnd, ResourceID: "vm-1"}
	if !matchesEndEvent(start, end) {
		t.Fatalf("expected start/end match")
	}
	backupStart := ProxmoxEvent{Type: EventBackupStart, ResourceID: "vm-1"}
	backupEnd := ProxmoxEvent{Type: EventBackupEnd, ResourceID: "vm-1"}
	if !matchesEndEvent(backupStart, backupEnd) {
		t.Fatalf("expected backup start/end match")
	}
	if matchesEndEvent(ProxmoxEvent{Type: EventSnapshotCreate, ResourceID: "vm-1"}, ProxmoxEvent{Type: EventSnapshotDelete, ResourceID: "vm-1"}) {
		t.Fatalf("expected snapshot start/end to not match")
	}
	if matchesEndEvent(start, ProxmoxEvent{Type: EventMigrationEnd, ResourceID: "vm-2"}) {
		t.Fatalf("expected mismatch for different resource")
	}

	if estimateOperationDuration(EventBackupStart) != 2*time.Hour {
		t.Fatalf("expected backup duration")
	}
	if estimateOperationDuration(ProxmoxEventType("custom")) != 15*time.Minute {
		t.Fatalf("expected default duration")
	}
	if len(getExpectedMetrics(EventSnapshotCreate)) == 0 {
		t.Fatalf("expected metrics for snapshot")
	}
	if len(getExpectedMetrics(EventHAFailover)) == 0 {
		t.Fatalf("expected metrics for HA")
	}
	if len(getExpectedMetrics(ProxmoxEventType("custom"))) != 0 {
		t.Fatalf("expected no metrics for unknown event type")
	}

	if formatEventType(ProxmoxEventType("custom")) != "custom" {
		t.Fatalf("expected fallback format")
	}

	if !containsString([]string{"a", "b"}, "b") {
		t.Fatalf("expected containsString to match")
	}
	if containsString([]string{"a", "b"}, "c") {
		t.Fatalf("expected containsString to return false")
	}
	if minFloat(1.0, 2.0) != 1.0 {
		t.Fatalf("expected minFloat to return min")
	}
}

func TestFormatEventType_All(t *testing.T) {
	types := []ProxmoxEventType{
		EventMigrationStart,
		EventMigrationEnd,
		EventBackupStart,
		EventBackupEnd,
		EventSnapshotCreate,
		EventSnapshotDelete,
		EventHAFailover,
		EventHAMigration,
		EventClusterJoin,
		EventClusterLeave,
		EventStorageOnline,
		EventStorageOffline,
		EventNodeReboot,
		EventVMCreate,
		EventVMDestroy,
		EventVMStart,
		EventVMStop,
	}

	for _, eventType := range types {
		if formatEventType(eventType) == "" {
			t.Fatalf("expected format for %s", eventType)
		}
	}
}

func TestGenerateExplanation_Cases(t *testing.T) {
	event := ProxmoxEvent{Type: EventBackupStart, ResourceID: "vm-1"}
	anomaly := MetricAnomaly{ResourceID: "vm-1", Metric: "io"}
	if generateExplanation(event, anomaly) == "" {
		t.Fatalf("expected explanation for backup")
	}

	event = ProxmoxEvent{Type: EventSnapshotCreate, ResourceID: "vm-1"}
	if generateExplanation(event, anomaly) == "" {
		t.Fatalf("expected explanation for snapshot")
	}

	event = ProxmoxEvent{Type: EventHAFailover, ResourceID: "vm-1"}
	if generateExplanation(event, anomaly) == "" {
		t.Fatalf("expected explanation for HA failover")
	}

	event = ProxmoxEvent{Type: ProxmoxEventType("custom"), ResourceID: "vm-1"}
	if generateExplanation(event, anomaly) == "" {
		t.Fatalf("expected default explanation")
	}
}

func TestSortEventsByTimestamp_Additional(t *testing.T) {
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	events := []ProxmoxEvent{
		{ID: "old", Timestamp: older},
		{ID: "new", Timestamp: newer},
	}

	SortEventsByTimestamp(events)
	if events[0].ID != "new" {
		t.Fatalf("expected newest event first")
	}
}
