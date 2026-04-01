package monitoring

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// fakeStorageClient provides minimal PVE responses needed by the optimized storage poller.
type fakeStorageClient struct {
	allStorage     []proxmox.Storage
	storageByNode  map[string][]proxmox.Storage
	zfsPoolsByNode map[string][]proxmox.ZFSPoolInfo
}

func (f *fakeStorageClient) GetNodes(ctx context.Context) ([]proxmox.Node, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetNodeStatus(ctx context.Context, node string) (*proxmox.NodeStatus, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetNodeRRDData(ctx context.Context, node string, timeframe string, cf string, ds []string) ([]proxmox.NodeRRDPoint, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetVMs(ctx context.Context, node string) ([]proxmox.VM, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetContainers(ctx context.Context, node string) ([]proxmox.Container, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	if storages, ok := f.storageByNode[node]; ok {
		return storages, nil
	}
	return nil, fmt.Errorf("unexpected node: %s", node)
}

func (f *fakeStorageClient) GetAllStorage(ctx context.Context) ([]proxmox.Storage, error) {
	return f.allStorage, nil
}

func (f *fakeStorageClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetReplicationStatus(ctx context.Context) ([]proxmox.ReplicationJob, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeStorageClient) GetContainerInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.ContainerInterface, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return nil, nil
}

func (f *fakeStorageClient) IsClusterMember(ctx context.Context) (bool, error) {
	return false, nil
}

func (f *fakeStorageClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	return "", nil
}

func (f *fakeStorageClient) GetZFSPoolStatus(ctx context.Context, node string) ([]proxmox.ZFSPoolStatus, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetZFSPoolsWithDetails(ctx context.Context, node string) ([]proxmox.ZFSPoolInfo, error) {
	if pools, ok := f.zfsPoolsByNode[node]; ok {
		return pools, nil
	}
	return nil, nil
}

func (f *fakeStorageClient) GetDisks(ctx context.Context, node string) ([]proxmox.Disk, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetCephStatus(ctx context.Context) (*proxmox.CephStatus, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetCephDF(ctx context.Context) (*proxmox.CephDF, error) {
	return nil, nil
}

func (f *fakeStorageClient) GetNodePendingUpdates(ctx context.Context, node string) ([]proxmox.AptPackage, error) {
	return nil, nil
}

func TestPollStorageWithNodesOptimizedRecordsMetricsAndAlerts(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	monitor := &Monitor{
		state:          &models.State{},
		metricsHistory: NewMetricsHistory(16, time.Hour),
		alertManager:   alerts.NewManager(),
	}
	t.Cleanup(func() {
		monitor.alertManager.Stop()
	})

	// Ensure storage alerts trigger immediately for the test.
	cfg := monitor.alertManager.GetConfig()
	cfg.MinimumDelta = 0
	if cfg.TimeThresholds == nil {
		cfg.TimeThresholds = make(map[string]int)
	}
	cfg.TimeThresholds["storage"] = 0
	monitor.alertManager.UpdateConfig(cfg)

	storage := proxmox.Storage{
		Storage:   "local",
		Type:      "dir",
		Content:   "images",
		Active:    1,
		Enabled:   1,
		Shared:    0,
		Total:     1000,
		Used:      900,
		Available: 100,
	}

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{storage},
		storageByNode: map[string][]proxmox.Storage{
			"node1": {storage},
		},
	}

	nodes := []proxmox.Node{
		{Node: "node1", Status: "online"},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	metrics := monitor.metricsHistory.GetAllStorageMetrics("inst1-node1-local", time.Minute)
	if len(metrics["usage"]) != 1 {
		t.Fatalf("expected one usage metric entry, got %d", len(metrics["usage"]))
	}
	if len(metrics["used"]) != 1 {
		t.Fatalf("expected one used metric entry, got %d", len(metrics["used"]))
	}
	if len(metrics["total"]) != 1 {
		t.Fatalf("expected one total metric entry, got %d", len(metrics["total"]))
	}
	if len(metrics["avail"]) != 1 {
		t.Fatalf("expected one avail metric entry, got %d", len(metrics["avail"]))
	}

	if diff := math.Abs(metrics["usage"][0].Value - 90); diff > 0.001 {
		t.Fatalf("expected usage metric 90, diff %.4f", diff)
	}
	if diff := math.Abs(metrics["used"][0].Value - 900); diff > 0.001 {
		t.Fatalf("expected used metric 900, diff %.4f", diff)
	}
	if diff := math.Abs(metrics["total"][0].Value - 1000); diff > 0.001 {
		t.Fatalf("expected total metric 1000, diff %.4f", diff)
	}
	if diff := math.Abs(metrics["avail"][0].Value - 100); diff > 0.001 {
		t.Fatalf("expected avail metric 100, diff %.4f", diff)
	}

	alerts := monitor.alertManager.GetActiveAlerts()
	found := false
	for _, alert := range alerts {
		if alert.Instance == "inst1" &&
			alert.Node == "node1" &&
			(alert.ResourceID == "inst1-node1-local" ||
				alert.ResourceName == "local" ||
				alert.ID == "inst1-node1-local::inst1-node1-local-usage") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected storage usage alert to be active, alerts=%+v", alerts)
	}
}

func TestPollStorageWithNodesSynthesizesSharedClusterOnlyStorage(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "inst1",
					IsCluster:   true,
					ClusterName: "cluster-a",
				},
			},
		},
	}

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{
			{
				Storage:   "cephfs",
				Type:      "cephfs",
				Content:   "images,backup",
				Shared:    1,
				Total:     1000,
				Used:      100,
				Available: 900,
				Nodes:     "node1,node2",
			},
		},
		storageByNode: map[string][]proxmox.Storage{
			"node1": {
				{Storage: "local", Type: "dir", Content: "images", Active: 1, Enabled: 1, Total: 100, Used: 10, Available: 90},
			},
			"node2": {
				{Storage: "local", Type: "dir", Content: "images", Active: 1, Enabled: 1, Total: 100, Used: 20, Available: 80},
			},
		},
	}

	nodes := []proxmox.Node{
		{Node: "node1", Status: "online"},
		{Node: "node2", Status: "online"},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	var shared *models.Storage
	for _, storage := range monitor.state.GetSnapshot().Storage {
		if storage.Name == "cephfs" {
			storageCopy := storage
			shared = &storageCopy
			break
		}
	}
	if shared == nil {
		t.Fatalf("expected synthesized shared storage in state, got %+v", monitor.state.GetSnapshot().Storage)
	}
	if shared.ID != "cluster-a-cluster-cephfs" || shared.Node != "cluster" || !shared.Shared {
		t.Fatalf("unexpected synthesized shared storage identity: %+v", *shared)
	}
	if shared.NodeCount != 2 || len(shared.Nodes) != 2 {
		t.Fatalf("expected cluster storage node affinity, got %+v", *shared)
	}
	if shared.Total != 1000 || shared.Used != 100 || shared.Free != 900 {
		t.Fatalf("expected cluster storage capacity from config, got %+v", *shared)
	}
}

func TestPollStorageWithNodesAttachesZFSPoolFromExplicitPoolField(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	monitor := &Monitor{
		state:          &models.State{},
		metricsHistory: NewMetricsHistory(16, time.Hour),
		alertManager:   alerts.NewManager(),
	}
	t.Cleanup(func() {
		monitor.alertManager.Stop()
	})

	storage := proxmox.Storage{
		Storage:   "local-zfs",
		Type:      "zfspool",
		Pool:      "rpool/data",
		Content:   "images,rootdir",
		Active:    1,
		Enabled:   1,
		Shared:    0,
		Total:     1000,
		Used:      250,
		Available: 750,
	}

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{storage},
		storageByNode: map[string][]proxmox.Storage{
			"node1": {storage},
		},
		zfsPoolsByNode: map[string][]proxmox.ZFSPoolInfo{
			"node1": {
				{Name: "rpool", Size: 1000, Alloc: 250, Free: 750, Frag: 1, Dedup: 1.0, Health: "ONLINE"},
			},
		},
	}

	nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	if len(monitor.state.Storage) != 1 {
		t.Fatalf("expected 1 storage entry, got %d", len(monitor.state.Storage))
	}
	if got := monitor.state.Storage[0].Pool; got != "rpool/data" {
		t.Fatalf("storage pool = %q, want %q", got, "rpool/data")
	}
	if monitor.state.Storage[0].ZFSPool == nil {
		t.Fatal("expected explicit pool field to attach ZFS pool details")
	}
	if monitor.state.Storage[0].ZFSPool.Name != "rpool" {
		t.Fatalf("ZFS pool name = %q, want rpool", monitor.state.Storage[0].ZFSPool.Name)
	}
}

func TestPollStorageWithNodesUsesClusterStoragePoolFallback(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	monitor := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "inst1",
					IsCluster:   true,
					ClusterName: "cluster-a",
				},
			},
		},
		metricsHistory: NewMetricsHistory(16, time.Hour),
		alertManager:   alerts.NewManager(),
	}
	t.Cleanup(func() {
		monitor.alertManager.Stop()
	})

	clusterStorage := proxmox.Storage{
		Storage:   "local-zfs",
		Type:      "zfspool",
		Pool:      "rpool/data",
		Content:   "images,rootdir",
		Shared:    0,
		Total:     1000,
		Used:      250,
		Available: 750,
	}
	nodeStorage := clusterStorage
	nodeStorage.Pool = ""
	nodeStorage.Active = 1
	nodeStorage.Enabled = 1

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{clusterStorage},
		storageByNode: map[string][]proxmox.Storage{
			"node1": {nodeStorage},
		},
		zfsPoolsByNode: map[string][]proxmox.ZFSPoolInfo{
			"node1": {
				{Name: "rpool", Size: 1000, Alloc: 250, Free: 750, Frag: 1, Dedup: 1.0, Health: "ONLINE"},
			},
		},
	}

	nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	if len(monitor.state.Storage) != 1 {
		t.Fatalf("expected 1 storage entry, got %d", len(monitor.state.Storage))
	}
	if got := monitor.state.Storage[0].Pool; got != "rpool/data" {
		t.Fatalf("storage pool = %q, want %q", got, "rpool/data")
	}
	if monitor.state.Storage[0].ZFSPool == nil || monitor.state.Storage[0].ZFSPool.Name != "rpool" {
		t.Fatalf("expected cluster pool fallback to attach rpool, got %#v", monitor.state.Storage[0].ZFSPool)
	}
}
