package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type mockPVEClientSnapshots struct {
	mockPVEClientExtra
	snapshots []proxmox.Snapshot
}

func (m *mockPVEClientSnapshots) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	if vmid == 999 {
		// simulate timeout/error
		return nil, fmt.Errorf("timeout")
	}
	return m.snapshots, nil
}

func (m *mockPVEClientSnapshots) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return m.snapshots, nil
}

func TestMonitor_PollGuestSnapshots_Coverage(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}

	// 1. Setup State directly
	vms := []models.VM{
		{ID: "qemu/100", VMID: 100, Node: "node1", Instance: "pve1", Name: "vm100", Template: false},
		{ID: "qemu/101", VMID: 101, Node: "node1", Instance: "pve1", Name: "vm101-tmpl", Template: true}, // Should start skip
		{ID: "qemu/999", VMID: 999, Node: "node1", Instance: "pve1", Name: "vm999-fail", Template: false},
	}
	ct := []models.Container{
		{ID: "lxc/200", VMID: 200, Node: "node1", Instance: "pve1", Name: "ct200", Template: false},
	}
	m.state.UpdateVMsForInstance("pve1", vms)
	m.state.UpdateContainersForInstance("pve1", ct)

	// 2. Setup Client
	snaps := []proxmox.Snapshot{
		{Name: "snap1", SnapTime: 1234567890, Description: "test snap"},
	}
	client := &mockPVEClientSnapshots{
		snapshots: snaps,
	}

	// 3. Run
	ctx := context.Background()
	m.pollGuestSnapshots(ctx, "pve1", client)

	// 4. Verify
	// Check if snapshots are stored in State
	snapshot := m.state.GetSnapshot()
	found := false
	t.Logf("Found %d guest snapshots in state", len(snapshot.PVEBackups.GuestSnapshots))
	for _, gst := range snapshot.PVEBackups.GuestSnapshots {
		t.Logf("Snapshot: VMID=%d, Name=%s", gst.VMID, gst.Name)
		if gst.VMID == 100 && gst.Name == "snap1" {
			found = true
			if gst.Description != "test snap" {
				t.Errorf("Expected description 'test snap', got %s", gst.Description)
			}
		}
		if gst.VMID == 101 {
			t.Error("Should not have snapshots for template VM 101")
		}
	}
	if !found {
		t.Error("Expected snapshot 'snap1' for VM 100")
	}

	// 5. Test Context Deadline Exceeded Early Return
	shortCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond) // Ensure it expired
	m.pollGuestSnapshots(shortCtx, "pve1", client)

	// Should log warn and return (no change to state, but coverage of check)
}
