package monitoring

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type fakeSnapshotClient struct {
	storages map[string][]proxmox.Storage
	contents map[string]map[string][]proxmox.StorageContent
}

func (f fakeSnapshotClient) GetNodes(ctx context.Context) ([]proxmox.Node, error) { return nil, nil }
func (f fakeSnapshotClient) GetNodeStatus(ctx context.Context, node string) (*proxmox.NodeStatus, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetNodeRRDData(ctx context.Context, node string, timeframe string, cf string, ds []string) ([]proxmox.NodeRRDPoint, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetVMs(ctx context.Context, node string) ([]proxmox.VM, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetContainers(ctx context.Context, node string) ([]proxmox.Container, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	return f.storages[node], nil
}
func (f fakeSnapshotClient) GetAllStorage(ctx context.Context) ([]proxmox.Storage, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetTaskLog(ctx context.Context, node, upid string) ([]proxmox.TaskLogLine, error) {
	return nil, nil
}

func (f fakeSnapshotClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetReplicationStatus(ctx context.Context) ([]proxmox.ReplicationJob, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	if storageContents, ok := f.contents[node]; ok {
		return storageContents[storage], nil
	}
	return nil, nil
}
func (f fakeSnapshotClient) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error) {
	return nil, nil
}

func (f fakeSnapshotClient) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetContainerInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.ContainerInterface, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return nil, nil
}
func (f fakeSnapshotClient) IsClusterMember(ctx context.Context) (bool, error) { return false, nil }
func (f fakeSnapshotClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (f fakeSnapshotClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	return "", nil
}
func (f fakeSnapshotClient) GetZFSPoolStatus(ctx context.Context, node string) ([]proxmox.ZFSPoolStatus, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetZFSPoolsWithDetails(ctx context.Context, node string) ([]proxmox.ZFSPoolInfo, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetDisks(ctx context.Context, node string) ([]proxmox.Disk, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetCephStatus(ctx context.Context) (*proxmox.CephStatus, error) {
	return nil, nil
}
func (f fakeSnapshotClient) GetCephDF(ctx context.Context) (*proxmox.CephDF, error) { return nil, nil }
func (f fakeSnapshotClient) GetNodePendingUpdates(ctx context.Context, node string) ([]proxmox.AptPackage, error) {
	return nil, nil
}

func TestCollectSnapshotSizes(t *testing.T) {
	m := &Monitor{}
	snapshots := []models.GuestSnapshot{
		{
			ID:       "inst-node1-100-pre",
			Name:     "pre",
			Node:     "node1",
			Instance: "inst",
			Type:     "qemu",
			VMID:     100,
		},
	}

	client := fakeSnapshotClient{
		storages: map[string][]proxmox.Storage{
			"node1": {
				{Storage: "local-zfs", Content: "images", Active: 1, Enabled: 1},
			},
		},
		contents: map[string]map[string][]proxmox.StorageContent{
			"node1": {
				"local-zfs": {
					{Volid: "local-zfs:vm-100-disk-0@pre", VMID: 100, Size: 20 << 30},
					// Duplicate entry should be deduped via volid tracking
					{Volid: "local-zfs:vm-100-disk-0@pre", VMID: 100, Size: 20 << 30},
				},
			},
		},
	}

	sizes := m.collectSnapshotSizes(context.Background(), "inst", client, snapshots)
	got, ok := sizes[snapshots[0].ID]
	if !ok {
		t.Fatalf("expected size entry for snapshot")
	}
	want := int64(20 << 30)
	if got != want {
		t.Fatalf("unexpected size: got %d want %d", got, want)
	}
}

func TestHostsSnapshotPreservesCanonicalUpdaterEvidence(t *testing.T) {
	now := time.Now().UTC()
	checked, attempted := now.Add(-time.Minute), now.Add(-time.Second)
	failed := &models.AgentUpdateStatus{State: "error", AutoUpdate: true, AvailableVersion: "6.7.0", LastCheckedAt: &checked, LastAttemptAt: &attempted, LastError: "restart failed; helper rollback restored running executable"}
	host := models.Host{ID: "updater-host", Hostname: "updater.example.test", AgentVersion: "6.6.0", Status: "online", LastSeen: now, AgentUpdate: failed}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{host}, LastUpdate: now})
	m := &Monitor{state: models.NewState(), resourceStore: unifiedresources.NewMonitorAdapter(registry)}
	hosts := m.HostsSnapshot()
	if len(hosts) != 1 || !reflect.DeepEqual(hosts[0].AgentUpdate, failed) {
		t.Fatalf("UPDATER_EVIDENCE_DROPPED: host snapshot=%+v", hosts)
	}
	hosts[0].AgentUpdate.LastError = "snapshot reader mutation"
	*hosts[0].AgentUpdate.LastAttemptAt = now.Add(time.Hour)
	again := m.HostsSnapshot()
	if len(again) != 1 || !reflect.DeepEqual(again[0].AgentUpdate, failed) {
		t.Fatalf("snapshot mutation changed canonical updater evidence: %+v", again)
	}
	succeeded := now.Add(time.Second)
	host.AgentVersion = "6.7.0"
	host.LastSeen = succeeded
	host.AgentUpdate = &models.AgentUpdateStatus{State: "idle", AutoUpdate: true, UpdatedFrom: "6.6.0", LastSuccessAt: &succeeded}
	registry.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{host}, LastUpdate: succeeded})
	current := m.HostsSnapshot()
	if len(current) != 1 || !reflect.DeepEqual(current[0].AgentUpdate, host.AgentUpdate) {
		t.Fatalf("accepted update outcome did not replace error evidence: %+v", current)
	}
}
