package extensions

import (
	"testing"
)

func TestEmptyReportingStateSnapshot_AllFieldsNonNil(t *testing.T) {
	snapshot := EmptyReportingStateSnapshot()

	if snapshot.Nodes == nil {
		t.Error("Nodes should be non-nil")
	}
	if snapshot.VMs == nil {
		t.Error("VMs should be non-nil")
	}
	if snapshot.Containers == nil {
		t.Error("Containers should be non-nil")
	}
	if snapshot.ActiveAlerts == nil {
		t.Error("ActiveAlerts should be non-nil")
	}
	if snapshot.ResolvedAlerts == nil {
		t.Error("ResolvedAlerts should be non-nil")
	}
	if snapshot.Storage == nil {
		t.Error("Storage should be non-nil")
	}
	if snapshot.Disks == nil {
		t.Error("Disks should be non-nil")
	}
	if snapshot.LegacyBackups == nil {
		t.Error("LegacyBackups should be non-nil")
	}
}

func TestEmptyReportingStateSnapshot_AllFieldsEmptyLength(t *testing.T) {
	snapshot := EmptyReportingStateSnapshot()

	if len(snapshot.Nodes) != 0 {
		t.Errorf("Nodes should be empty, got %d", len(snapshot.Nodes))
	}
	if len(snapshot.VMs) != 0 {
		t.Errorf("VMs should be empty, got %d", len(snapshot.VMs))
	}
	if len(snapshot.Containers) != 0 {
		t.Errorf("Containers should be empty, got %d", len(snapshot.Containers))
	}
	if len(snapshot.ActiveAlerts) != 0 {
		t.Errorf("ActiveAlerts should be empty, got %d", len(snapshot.ActiveAlerts))
	}
	if len(snapshot.ResolvedAlerts) != 0 {
		t.Errorf("ResolvedAlerts should be empty, got %d", len(snapshot.ResolvedAlerts))
	}
	if len(snapshot.Storage) != 0 {
		t.Errorf("Storage should be empty, got %d", len(snapshot.Storage))
	}
	if len(snapshot.Disks) != 0 {
		t.Errorf("Disks should be empty, got %d", len(snapshot.Disks))
	}
	if len(snapshot.LegacyBackups) != 0 {
		t.Errorf("LegacyBackups should be empty, got %d", len(snapshot.LegacyBackups))
	}
}

func TestNormalizeCollections_PreservesExistingData(t *testing.T) {
	snapshot := ReportingStateSnapshot{
		Nodes: []ReportingNodeSnapshot{
			{ID: "node-1", Name: "pve1"},
		},
		VMs: []ReportingVMSnapshot{
			{ID: "vm-1", Name: "web-server"},
		},
		// Leave other fields nil.
	}
	snapshot.NormalizeCollections()

	// Pre-populated fields should be preserved.
	if len(snapshot.Nodes) != 1 || snapshot.Nodes[0].ID != "node-1" {
		t.Error("NormalizeCollections should preserve existing Nodes")
	}
	if len(snapshot.VMs) != 1 || snapshot.VMs[0].ID != "vm-1" {
		t.Error("NormalizeCollections should preserve existing VMs")
	}

	// Nil fields should become non-nil empty slices.
	if snapshot.Containers == nil {
		t.Error("Containers should be normalized to non-nil")
	}
	if snapshot.ActiveAlerts == nil {
		t.Error("ActiveAlerts should be normalized to non-nil")
	}
	if snapshot.ResolvedAlerts == nil {
		t.Error("ResolvedAlerts should be normalized to non-nil")
	}
	if snapshot.Storage == nil {
		t.Error("Storage should be normalized to non-nil")
	}
	if snapshot.Disks == nil {
		t.Error("Disks should be normalized to non-nil")
	}
	if snapshot.LegacyBackups == nil {
		t.Error("LegacyBackups should be normalized to non-nil")
	}
}

func TestNormalizeCollections_IdempotentOnNonNilSlices(t *testing.T) {
	snapshot := EmptyReportingStateSnapshot()
	// Running normalize again should not change anything.
	snapshot.NormalizeCollections()
	snapshot.NormalizeCollections()

	if snapshot.Nodes == nil || len(snapshot.Nodes) != 0 {
		t.Error("multiple normalization calls should be idempotent")
	}
}
