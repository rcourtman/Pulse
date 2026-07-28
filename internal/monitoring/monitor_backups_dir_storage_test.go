package monitoring

import (
	"context"
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
