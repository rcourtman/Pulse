package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	pveapi "github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type canonicalBackupStorageClient struct {
	mockPVEClientExtra
}

func (c *canonicalBackupStorageClient) GetStorage(ctx context.Context, node string) ([]pveapi.Storage, error) {
	return []pveapi.Storage{
		{Storage: "local", Content: "backup", Type: "dir", Enabled: 1, Active: 1},
	}, nil
}

func (c *canonicalBackupStorageClient) GetStorageContent(ctx context.Context, node, storage string) ([]pveapi.StorageContent, error) {
	return []pveapi.StorageContent{
		{
			Volid:   "backup/vzdump-qemu-100-2026_03_11-10_00_00.vma.zst",
			VMID:    100,
			Size:    1024,
			CTime:   time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC).Unix(),
			Content: "backup",
		},
	}, nil
}

func backupReadStateResourceStore(resources []unifiedresources.Resource) *resourceOnlyStore {
	return &resourceOnlyStore{resources: resources}
}

func backupReadState(resources []unifiedresources.Resource) unifiedresources.ReadState {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources(resources)
	return registry
}

func TestPopulateGuestNodeMapFromReadState_UsesCanonicalWorkloads(t *testing.T) {
	readState := backupReadState([]unifiedresources.Resource{
		{
			ID:     "vm-1",
			Type:   unifiedresources.ResourceTypeVM,
			Name:   "vm-100",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "node-from-store",
				VMID:     100,
			},
		},
		{
			ID:     "ct-1",
			Type:   unifiedresources.ResourceTypeSystemContainer,
			Name:   "ct-200",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "ct-node-from-store",
				VMID:     200,
			},
		},
	})

	guestNodeMap := map[int]string{}
	populateGuestNodeMapFromReadState(readState, "pve1", guestNodeMap)

	if guestNodeMap[100] != "node-from-store" {
		t.Fatalf("expected VM node from canonical read-state, got %q", guestNodeMap[100])
	}
	if guestNodeMap[200] != "ct-node-from-store" {
		t.Fatalf("expected container node from canonical read-state, got %q", guestNodeMap[200])
	}
}

func TestStorageNamesForNode_UsesCanonicalStoragePools(t *testing.T) {
	readState := backupReadState([]unifiedresources.Resource{
		{
			ID:     "storage-local",
			Type:   unifiedresources.ResourceTypeStorage,
			Name:   "local",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "node1",
			},
			Storage: &unifiedresources.StorageMeta{
				Content: "images,backup",
			},
		},
		{
			ID:     "storage-shared",
			Type:   unifiedresources.ResourceTypeStorage,
			Name:   "shared",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
			},
			Storage: &unifiedresources.StorageMeta{
				Content: "backup",
				Nodes:   []string{"node2", "node3"},
			},
		},
		{
			ID:     "storage-no-backup",
			Type:   unifiedresources.ResourceTypeStorage,
			Name:   "fast",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "node2",
			},
			Storage: &unifiedresources.StorageMeta{
				Content: "images",
			},
		},
	})

	got := storageNamesForNode(readState, "pve1", "node2")
	if len(got) != 1 || got[0] != "shared" {
		t.Fatalf("expected canonical backup storage names [shared], got %+v", got)
	}
}

func TestMonitorCalculateBackupOperationTimeout_UsesCanonicalReadState(t *testing.T) {
	resources := make([]unifiedresources.Resource, 0, 61)
	for i := 0; i < 61; i++ {
		resources = append(resources, unifiedresources.Resource{
			ID:     fmt.Sprintf("vm-%d", i),
			Type:   unifiedresources.ResourceTypeVM,
			Name:   fmt.Sprintf("vm-%d", i),
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "node1",
				VMID:     100 + i,
			},
		})
	}

	m := &Monitor{
		state:         models.NewState(),
		resourceStore: backupReadStateResourceStore(resources),
	}

	timeout := m.calculateBackupOperationTimeout("pve1")
	if want := 122 * time.Second; timeout != want {
		t.Fatalf("expected timeout %v from canonical workload count, got %v", want, timeout)
	}
}

func TestMonitorPollGuestSnapshots_UsesCanonicalReadState(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
		resourceStore: backupReadStateResourceStore([]unifiedresources.Resource{
			{
				ID:     "vm-store-100",
				Type:   unifiedresources.ResourceTypeVM,
				Name:   "vm100",
				Status: unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{
					Instance: "pve1",
					NodeName: "node1",
					VMID:     100,
				},
			},
			{
				ID:     "ct-store-200",
				Type:   unifiedresources.ResourceTypeSystemContainer,
				Name:   "ct200",
				Status: unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{
					Instance: "pve1",
					NodeName: "node1",
					VMID:     200,
				},
			},
		}),
	}

	client := &mockPVEClientSnapshots{
		snapshots: []pveapi.Snapshot{{Name: "snap1", SnapTime: 1234567890, Description: "from store"}},
	}

	m.pollGuestSnapshots(context.Background(), "pve1", client)

	snapshot := m.state.GetSnapshot()
	if len(snapshot.PVEBackups.GuestSnapshots) != 2 {
		t.Fatalf("expected guest snapshots from canonical workloads, got %+v", snapshot.PVEBackups.GuestSnapshots)
	}
}

func TestMonitorPollStorageBackupsWithNodes_UsesCanonicalReadStateForGuestNodeLookup(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
		resourceStore: backupReadStateResourceStore([]unifiedresources.Resource{
			{
				ID:     "vm-store-100",
				Type:   unifiedresources.ResourceTypeVM,
				Name:   "vm100",
				Status: unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{
					Instance: "pve1",
					NodeName: "node2",
					VMID:     100,
				},
			},
		}),
	}

	client := &canonicalBackupStorageClient{}
	nodes := []pveapi.Node{{Node: "node1", Status: "online"}}

	m.pollStorageBackupsWithNodes(context.Background(), "pve1", client, nodes, map[string]string{"node1": "online"})

	backups := m.state.GetSnapshot().PVEBackups.StorageBackups
	if len(backups) != 1 {
		t.Fatalf("expected one storage backup, got %+v", backups)
	}
	if backups[0].Node != "node2" {
		t.Fatalf("expected guest node from canonical read-state, got %q", backups[0].Node)
	}
}

func TestBuildPBSGuestCandidates_UsesCanonicalReadState(t *testing.T) {
	readState := backupReadState([]unifiedresources.Resource{
		{
			ID:     "vm-store-100",
			Type:   unifiedresources.ResourceTypeVM,
			Name:   "vm100",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "nodeA",
				VMID:     100,
			},
		},
		{
			ID:     "ct-store-200",
			Type:   unifiedresources.ResourceTypeSystemContainer,
			Name:   "ct200",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "nodeB",
				VMID:     200,
			},
		},
	})

	candidates := buildPBSGuestCandidates(readState)

	assertCandidate := func(key string, resourceType unifiedresources.ResourceType, node string, vmid int) {
		t.Helper()
		entries := candidates[key]
		if len(entries) != 1 {
			t.Fatalf("expected one candidate for %s, got %+v", key, entries)
		}
		if entries[0] != (proxmoxmapper.GuestCandidate{
			SourceID:     fmt.Sprintf("%s-store-%d", map[unifiedresources.ResourceType]string{unifiedresources.ResourceTypeVM: "vm", unifiedresources.ResourceTypeSystemContainer: "ct"}[resourceType], vmid),
			ResourceType: resourceType,
			DisplayName:  fmt.Sprintf("%s%d", map[unifiedresources.ResourceType]string{unifiedresources.ResourceTypeVM: "vm", unifiedresources.ResourceTypeSystemContainer: "ct"}[resourceType], vmid),
			InstanceName: "pve1",
			NodeName:     node,
			VMID:         vmid,
		}) {
			t.Fatalf("unexpected candidate for %s: %+v", key, entries[0])
		}
	}

	assertCandidate("vm:100", unifiedresources.ResourceTypeVM, "nodeA", 100)
	assertCandidate("ct:200", unifiedresources.ResourceTypeSystemContainer, "nodeB", 200)
}
