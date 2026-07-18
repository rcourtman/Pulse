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
