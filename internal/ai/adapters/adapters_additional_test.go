package adapters

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

type stubIncidentRecorder struct {
	windows []*IncidentWindowData
	window  *IncidentWindowData
}

func (s *stubIncidentRecorder) GetWindowsForResource(resourceID string, limit int) []*IncidentWindowData {
	return s.windows
}

func (s *stubIncidentRecorder) GetWindow(windowID string) *IncidentWindowData {
	return s.window
}

type stubEventCorrelator struct {
	correlations []EventCorrelationData
	events       []ProxmoxEventData
}

func (s *stubEventCorrelator) GetCorrelationsForResource(resourceID string) []EventCorrelationData {
	return s.correlations
}

func (s *stubEventCorrelator) GetEventsForResource(resourceID string, limit int) []ProxmoxEventData {
	return s.events
}

func TestForecastDataAdapter_GetMetricHistory(t *testing.T) {
	history := monitoring.NewMetricsHistory(10, time.Hour)
	now := time.Now()

	history.AddGuestMetric("vm-1", "cpu", 1, now.Add(-10*time.Minute))
	history.AddGuestMetric("vm-1", "cpu", 2, now.Add(-time.Minute))

	adapter := NewForecastDataAdapter(history)
	points, err := adapter.GetMetricHistory("vm-1", "cpu", now.Add(-5*time.Minute), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 1 || points[0].Value != 2 {
		t.Fatalf("expected filtered guest points")
	}

	nodeHistory := monitoring.NewMetricsHistory(10, time.Hour)
	nodeHistory.AddNodeMetric("node-1", "cpu", 3, now.Add(-time.Minute))
	nodeAdapter := NewForecastDataAdapter(nodeHistory)
	points, err = nodeAdapter.GetMetricHistory("node-1", "cpu", now.Add(-5*time.Minute), now)
	if err != nil || len(points) != 1 || points[0].Value != 3 {
		t.Fatalf("expected node points")
	}

	storageHistory := monitoring.NewMetricsHistory(10, time.Hour)
	storageHistory.AddStorageMetric("store-1", "usage", 55, now.Add(-time.Minute))
	storageAdapter := NewForecastDataAdapter(storageHistory)
	points, err = storageAdapter.GetMetricHistory("store-1", "usage", now.Add(-5*time.Minute), now)
	if err != nil || len(points) != 1 || points[0].Value != 55 {
		t.Fatalf("expected storage points")
	}
}

func TestMetricsAdapter_GetMonitoredResourceIDs(t *testing.T) {
	state := models.StateSnapshot{
		Nodes:      []models.Node{{ID: "node/pve1", Name: "pve1", Instance: "inst1"}},
		VMs:        []models.VM{{ID: "qemu/100", VMID: 100, Name: "vm-1", Node: "pve1", Instance: "inst1"}},
		Containers: []models.Container{{ID: "lxc/200", VMID: 200, Name: "ct-1", Node: "pve1", Instance: "inst1"}},
	}
	adapter := NewMetricsAdapter(readStateFromSnapshot(state))
	ids := adapter.GetMonitoredResourceIDs()

	// Should include both unified IDs and source IDs (3 resources × 2 IDs each = 6)
	if len(ids) < 3 {
		t.Fatalf("expected at least 3 IDs, got %d: %v", len(ids), ids)
	}

	// Verify no empty IDs
	for _, id := range ids {
		if id == "" {
			t.Fatalf("unexpected empty ID in %v", ids)
		}
	}

	// Verify source IDs are present (for pre-incident buffer compatibility)
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet["qemu/100"] {
		t.Errorf("expected source ID 'qemu/100' in monitored IDs, got %v", ids)
	}
	if !idSet["lxc/200"] {
		t.Errorf("expected source ID 'lxc/200' in monitored IDs, got %v", ids)
	}
	if !idSet["node/pve1"] {
		t.Errorf("expected source ID 'node/pve1' in monitored IDs, got %v", ids)
	}
}

func TestIncidentRecorderMCPAdapter(t *testing.T) {
	adapter := NewIncidentRecorderMCPAdapter(nil)
	if adapter.GetWindowsForResource("res", 1) != nil {
		t.Fatalf("expected nil windows for nil recorder")
	}
	if adapter.GetWindow("id") != nil {
		t.Fatalf("expected nil window for nil recorder")
	}

	recorder := &stubIncidentRecorder{
		windows: []*IncidentWindowData{{ID: "w1"}},
		window:  &IncidentWindowData{ID: "w1"},
	}
	adapter = NewIncidentRecorderMCPAdapter(recorder)
	if len(adapter.GetWindowsForResource("res", 1)) != 1 {
		t.Fatalf("expected windows from recorder")
	}
	if adapter.GetWindow("w1") == nil {
		t.Fatalf("expected window from recorder")
	}
}

func TestEventCorrelatorMCPAdapter(t *testing.T) {
	adapter := NewEventCorrelatorMCPAdapter(nil)
	if adapter.GetCorrelationsForResource("res", time.Minute) != nil {
		t.Fatalf("expected nil correlations for nil correlator")
	}

	correlator := &stubEventCorrelator{
		correlations: []EventCorrelationData{{ID: "c1"}},
	}
	adapter = NewEventCorrelatorMCPAdapter(correlator)
	if len(adapter.GetCorrelationsForResource("res", time.Minute)) != 1 {
		t.Fatalf("expected correlations from correlator")
	}
}

func TestKnowledgeStore_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewKnowledgeStore(dir)

	if err := store.SaveNote("res-1", "note", "general"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	store.saveToDisk()
	info, err := os.Stat(filepath.Join(dir, "knowledge_store.json"))
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("expected mode 0600, got %o", got)
	}

	loaded := NewKnowledgeStore(dir)
	if err := loaded.loadFromDisk(); err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	entries := loaded.GetKnowledge("res-1", "general")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestKnowledgeStore_LoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	store := NewKnowledgeStore(dir)
	path := filepath.Join(dir, "knowledge_store.json")
	if err := store.loadFromDisk(); err == nil {
		t.Fatalf("expected error for missing file %s", path)
	}
}
