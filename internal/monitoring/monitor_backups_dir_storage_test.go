package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	pveapi "github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// dirStorageNamedPBSClient serves a dir-type storage whose NAME starts with
// "pbs-" alongside a real pbs-type storage. Only the latter may be treated as
// PBS-backed when a direct PBS connection exists (#1592).
type dirStorageNamedPBSClient struct {
	mockPVEClientExtra
}

func (c *dirStorageNamedPBSClient) GetStorage(ctx context.Context, node string) ([]pveapi.Storage, error) {
	return []pveapi.Storage{
		{Storage: "pbs-backup", Content: "backup", Type: "dir", Enabled: 1, Active: 1},
		{Storage: "pbs-real", Content: "backup", Type: "pbs", Enabled: 1, Active: 1},
	}, nil
}

func (c *dirStorageNamedPBSClient) GetStorageContent(ctx context.Context, node, storage string) ([]pveapi.StorageContent, error) {
	switch storage {
	case "pbs-backup":
		return []pveapi.StorageContent{{
			Volid:   "pbs-backup:backup/vzdump-qemu-106-2026_07_17-01_00_00.vma.zst",
			VMID:    106,
			Size:    2048,
			CTime:   time.Date(2026, 7, 17, 1, 0, 0, 0, time.UTC).Unix(),
			Content: "backup",
		}}, nil
	case "pbs-real":
		return []pveapi.StorageContent{{
			Volid:   "pbs-real:backup/vm/107/2026-07-17T01:00:00Z",
			VMID:    107,
			Size:    2048,
			CTime:   time.Date(2026, 7, 17, 1, 0, 0, 0, time.UTC).Unix(),
			Content: "backup",
			Format:  "pbs-vm",
		}}, nil
	}
	return nil, nil
}

func TestPollStorageBackups_KeepsDirStorageNamedLikePBS(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			PBSInstances: []config.PBSInstance{{Name: "pbs-1", Host: "https://pbs.example:8007"}},
		},
	}

	client := &dirStorageNamedPBSClient{}
	nodes := []pveapi.Node{{Node: "node1", Status: "online"}}

	m.pollStorageBackupsWithNodes(context.Background(), "pve1", client, nodes, map[string]string{"node1": "online"})

	backups := m.state.GetSnapshot().PVEBackups.StorageBackups
	if len(backups) != 1 {
		t.Fatalf("expected exactly the dir-storage backup to survive, got %+v", backups)
	}
	if backups[0].Storage != "pbs-backup" || backups[0].VMID != 106 {
		t.Fatalf("expected vzdump from dir storage named pbs-backup, got %+v", backups[0])
	}
}

// pbsPartialFailureClient serves two pbs-type storages and can be told to
// fail one storage's content query, standing in for a partial poll failure.
type pbsPartialFailureClient struct {
	mockPVEClientExtra
	snapshotTime time.Time
	failStorage  string
}

func (c *pbsPartialFailureClient) GetStorage(ctx context.Context, node string) ([]pveapi.Storage, error) {
	return []pveapi.Storage{
		{Storage: "pbs-one", Content: "backup", Type: "pbs", Enabled: 1, Active: 1},
		{Storage: "pbs-two", Content: "backup", Type: "pbs", Enabled: 1, Active: 1},
	}, nil
}

func (c *pbsPartialFailureClient) GetStorageContent(ctx context.Context, node, storage string) ([]pveapi.StorageContent, error) {
	if storage == c.failStorage {
		return nil, fmt.Errorf("500 internal error")
	}
	vmid := 173
	if storage == "pbs-two" {
		vmid = 174
	}
	return []pveapi.StorageContent{{
		Volid:   fmt.Sprintf("%s:backup/vm/%d/2026-07-27T01:00:00Z", storage, vmid),
		VMID:    vmid,
		Size:    2048,
		CTime:   c.snapshotTime.Unix(),
		Content: "backup",
		Format:  "pbs-vm",
	}}, nil
}

// TestPollStorageBackups_Issue1639PreservesConfirmationsOnPartialFailure
// verifies that a storage whose content query fails keeps its previous PBS
// guest confirmations instead of having them evicted by the partial set.
// Losing them flips collision-VMID attribution from cycle to cycle.
func TestPollStorageBackups_Issue1639PreservesConfirmationsOnPartialFailure(t *testing.T) {
	snapshotTime := time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC)

	m := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			PBSInstances: []config.PBSInstance{{Name: "pbs-1", Host: "https://pbs.example:8007"}},
		},
	}
	nodes := []pveapi.Node{{Node: "node-a", Status: "online"}}
	online := map[string]string{"node-a": "online"}

	// Healthy cycle: both storages list one snapshot each.
	m.pollStorageBackupsWithNodes(context.Background(), "cluster-a",
		&pbsPartialFailureClient{snapshotTime: snapshotTime}, nodes, online)

	healthy := m.state.PBSGuestConfirmationsForInstance("cluster-a")
	if len(healthy) != 2 {
		t.Fatalf("expected both storages to contribute confirmations, got %+v", healthy)
	}

	// Degraded cycle: pbs-one's content query fails. Its evidence must
	// survive, and pbs-two's must be refreshed as usual.
	m.pollStorageBackupsWithNodes(context.Background(), "cluster-a",
		&pbsPartialFailureClient{snapshotTime: snapshotTime, failStorage: "pbs-one"}, nodes, online)

	degraded := m.state.PBSGuestConfirmationsForInstance("cluster-a")
	if len(degraded) != 2 {
		t.Fatalf("partial failure evicted confirmation evidence, got %+v", degraded)
	}
	byStorage := map[string]models.PBSGuestConfirmation{}
	for _, confirmation := range degraded {
		byStorage[confirmation.Storage] = confirmation
	}
	if got, ok := byStorage["pbs-one"]; !ok || got.VMID != 173 {
		t.Fatalf("pbs-one confirmation not preserved across the failed query, got %+v", degraded)
	}
	if got, ok := byStorage["pbs-two"]; !ok || got.VMID != 174 {
		t.Fatalf("pbs-two confirmation missing after successful query, got %+v", degraded)
	}
}

// pbsCollisionStorageClient serves one pbs-type storage listing a single
// snapshot for VMID 173, standing in for cluster-a's own PBS storage view.
type pbsCollisionStorageClient struct {
	mockPVEClientExtra
	snapshotTime time.Time
}

func (c *pbsCollisionStorageClient) GetStorage(ctx context.Context, node string) ([]pveapi.Storage, error) {
	return []pveapi.Storage{
		{Storage: "pbs-store", Content: "backup", Type: "pbs", Enabled: 1, Active: 1},
	}, nil
}

func (c *pbsCollisionStorageClient) GetStorageContent(ctx context.Context, node, storage string) ([]pveapi.StorageContent, error) {
	return []pveapi.StorageContent{{
		Volid:   "pbs-store:backup/vm/173/2026-07-27T01:00:00Z",
		VMID:    173,
		Size:    2048,
		CTime:   c.snapshotTime.Unix(),
		Content: "backup",
		Format:  "pbs-vm",
	}}, nil
}

// TestPollStorageBackups_Issue1639HarvestsPBSGuestConfirmations verifies
// that snapshots skipped in favor of the direct PBS connection still leave
// PVE-side attribution evidence: the cluster whose storage listed a
// snapshot gets it as LastBackup even though the VMID exists on another
// cluster too, and the other cluster's unconfirmed snapshot is not guessed.
func TestPollStorageBackups_Issue1639HarvestsPBSGuestConfirmations(t *testing.T) {
	timeA := time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC)
	timeB := time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC)

	state := models.NewState()
	state.UpdateVMs([]models.VM{
		{VMID: 173, Name: "web", Instance: "cluster-a", Node: "node-a", Status: "running"},
		{VMID: 173, Name: "web", Instance: "cluster-b", Node: "node-b", Status: "running"},
	})
	state.UpdatePBSBackups("pbs-1", []models.PBSBackup{
		{ID: "a-173", VMID: "173", BackupType: "vm", BackupTime: timeA, Instance: "pbs-1", Datastore: "backups", Owner: "shared@pbs!token"},
		{ID: "b-173", VMID: "173", BackupType: "vm", BackupTime: timeB, Instance: "pbs-1", Datastore: "backups", Owner: "shared@pbs!token"},
	})

	m := &Monitor{
		state: state,
		config: &config.Config{
			PBSInstances: []config.PBSInstance{{Name: "pbs-1", Host: "https://pbs.example:8007"}},
		},
	}

	client := &pbsCollisionStorageClient{snapshotTime: timeA}
	nodes := []pveapi.Node{{Node: "node-a", Status: "online"}}

	m.pollStorageBackupsWithNodes(context.Background(), "cluster-a", client, nodes, map[string]string{"node-a": "online"})

	snapshot := m.state.GetSnapshot()
	if len(snapshot.PVEBackups.StorageBackups) != 0 {
		t.Fatalf("pbs-type storage contents must stay out of StorageBackups, got %+v", snapshot.PVEBackups.StorageBackups)
	}
	for _, vm := range snapshot.VMs {
		switch vm.Instance {
		case "cluster-a":
			if !vm.LastBackup.Equal(timeA) {
				t.Errorf("cluster-a VM 173 LastBackup = %v, want confirmed snapshot %v", vm.LastBackup, timeA)
			}
		case "cluster-b":
			if !vm.LastBackup.IsZero() {
				t.Errorf("cluster-b VM 173 LastBackup = %v, want zero (its snapshot is unconfirmed)", vm.LastBackup)
			}
		}
	}
}

func TestPollPVEBackupsAsyncUsesRuntimeContextIndependentOfCycle(t *testing.T) {
	runtimeCtx, cancelRuntime := context.WithCancel(context.Background())
	cancelRuntime()
	m := &Monitor{
		state:             models.NewState(),
		runtimeCtx:        runtimeCtx,
		lastPVEBackupPoll: make(map[string]time.Time),
		config: &config.Config{
			EnableBackupPolling: true,
			BackupPollingCycles: 1,
		},
	}
	m.pollPVEBackupsAsync(
		"cluster-a",
		&config.PVEInstance{MonitorBackups: true},
		&mockPVEClientExtra{},
		nil,
		map[string]string{},
	)
	m.mu.RLock()
	scheduledAt := m.lastPVEBackupPoll["cluster-a"]
	m.mu.RUnlock()
	if scheduledAt.IsZero() {
		t.Fatal("backup polling was not scheduled on the monitor runtime context")
	}
}
