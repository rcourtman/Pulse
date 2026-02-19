package monitoring

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// fakeStorageClient provides minimal PVE responses needed by the optimized storage poller.
type fakeStorageClient struct {
	allStorage    []proxmox.Storage
	storageByNode map[string][]proxmox.Storage
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
		if alert.ID == "inst1-node1-local-usage" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected storage usage alert to be active")
	}
}
