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

func (m *mockPVEClientSnapshots) GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error) {
	return 0, fmt.Errorf("not implemented")
}

type backupStorageTimeoutSnapshotClient struct {
	mockPVEClientExtra
	snapshots     []proxmox.Snapshot
	snapshotCalls int
	storageCalls  int
}

func (m *backupStorageTimeoutSnapshotClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	return nil, nil
}

func (m *backupStorageTimeoutSnapshotClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	m.storageCalls++
	if m.storageCalls > 1 {
		return nil, nil
	}

	<-ctx.Done()
	return nil, fmt.Errorf("storage scan exceeded backup inventory budget")
}

func (m *backupStorageTimeoutSnapshotClient) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	m.snapshotCalls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.snapshots, nil
}

func (m *backupStorageTimeoutSnapshotClient) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
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

// TestMonitor_PollGuestSnapshots_PreservesPreviousOnPerVMError guards against
// #1437: when a per-VM snapshot fetch fails (transient timeout/error), the
// previously-known snapshots for that VM must be carried forward so they
// do not silently disappear. Successfully-polled VMs in the same cycle
// still get their fresh snapshots, so newly-created snapshots show up.
func TestMonitor_PollGuestSnapshots_PreservesPreviousOnPerVMError(t *testing.T) {
	m := &Monitor{state: models.NewState()}

	vms := []models.VM{
		{ID: "qemu/100", VMID: 100, Node: "node1", Instance: "pve1", Name: "vm100", Template: false},
		{ID: "qemu/999", VMID: 999, Node: "node1", Instance: "pve1", Name: "vm999-fail", Template: false},
	}
	m.state.UpdateVMsForInstance("pve1", vms)

	// Seed previous state: vm100 has snap_old; vm999 has snap_persisted that
	// must survive a transient fetch failure.
	previous := []models.GuestSnapshot{
		{ID: "pve1-node1-100-snap_old", Name: "snap_old", Node: "node1", Instance: "pve1", Type: "qemu", VMID: 100, Time: time.Unix(1000, 0)},
		{ID: "pve1-node1-999-snap_persisted", Name: "snap_persisted", Node: "node1", Instance: "pve1", Type: "qemu", VMID: 999, Time: time.Unix(2000, 0)},
	}
	m.state.UpdateGuestSnapshotsForInstance("pve1", previous)

	// Fresh poll: vm100 succeeds with a NEW snapshot, vm999 fails.
	client := &mockPVEClientSnapshots{
		snapshots: []proxmox.Snapshot{
			{Name: "snap_new", SnapTime: 3000, Description: "fresh"},
		},
	}

	m.pollGuestSnapshots(context.Background(), "pve1", client)

	got := m.state.GetSnapshot().PVEBackups.GuestSnapshots
	byName := make(map[string]models.GuestSnapshot, len(got))
	for _, snap := range got {
		byName[snap.Name] = snap
	}

	// vm100's old snapshot must be replaced by the fresh one.
	if _, oldStillThere := byName["snap_old"]; oldStillThere {
		t.Errorf("expected snap_old (vm100) to be replaced by fresh poll, but it persisted")
	}
	if _, freshHere := byName["snap_new"]; !freshHere {
		t.Errorf("expected snap_new (vm100) from fresh poll, got names=%v", keys(byName))
	}

	// vm999's snapshot must be carried forward from previous state because
	// the fetch failed this cycle. Without the carry-forward fix, this
	// snapshot would disappear and the user would think their snapshot
	// was deleted (#1437).
	if _, persisted := byName["snap_persisted"]; !persisted {
		t.Errorf("expected snap_persisted (vm999) to be carried forward after fetch failure, got names=%v", keys(byName))
	}
}

func TestMonitor_PollPVEBackupsAndSnapshots_DoesNotStarveSnapshotsAfterStorageTimeout(t *testing.T) {
	m := &Monitor{state: models.NewState()}

	m.state.UpdateVMsForInstance("pve1", []models.VM{{
		ID:       "qemu/100",
		VMID:     100,
		Node:     "node1",
		Instance: "pve1",
		Name:     "vm100",
		Template: false,
	}})

	client := &backupStorageTimeoutSnapshotClient{
		snapshots: []proxmox.Snapshot{{
			Name:        "snap_after_storage_timeout",
			SnapTime:    4000,
			Description: "created while storage scan was slow",
		}},
	}

	m.pollPVEBackupsAndSnapshots(
		context.Background(),
		"pve1",
		client,
		[]proxmox.Node{{Node: "node1", Status: "online"}},
		map[string]string{"node1": "online"},
		time.Millisecond,
	)

	if client.snapshotCalls == 0 {
		t.Fatal("expected guest snapshot polling to run even after storage backup polling exhausted its budget")
	}

	got := m.state.GetSnapshot().PVEBackups.GuestSnapshots
	if len(got) != 1 {
		t.Fatalf("expected one guest snapshot after storage timeout, got %#v", got)
	}
	if got[0].Name != "snap_after_storage_timeout" {
		t.Fatalf("expected fresh snapshot after storage timeout, got %#v", got[0])
	}
}

func keys[K comparable, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
