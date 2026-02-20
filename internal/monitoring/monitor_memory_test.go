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
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type stubPVEClient struct {
	nodes      []proxmox.Node
	nodeStatus *proxmox.NodeStatus
	rrdPoints  []proxmox.NodeRRDPoint
}

var _ PVEClientInterface = (*stubPVEClient)(nil)

func (s *stubPVEClient) GetNodes(ctx context.Context) ([]proxmox.Node, error) {
	return s.nodes, nil
}

func (s *stubPVEClient) GetNodeStatus(ctx context.Context, node string) (*proxmox.NodeStatus, error) {
	return s.nodeStatus, nil
}

func (s *stubPVEClient) GetNodeRRDData(ctx context.Context, node, timeframe, cf string, ds []string) ([]proxmox.NodeRRDPoint, error) {
	return s.rrdPoints, nil
}

func (s *stubPVEClient) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}

func (s *stubPVEClient) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}

func (s *stubPVEClient) GetVMs(ctx context.Context, node string) ([]proxmox.VM, error) {
	return nil, nil
}

func (s *stubPVEClient) GetContainers(ctx context.Context, node string) ([]proxmox.Container, error) {
	return nil, nil
}

func (s *stubPVEClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	return nil, nil
}

func (s *stubPVEClient) GetAllStorage(ctx context.Context) ([]proxmox.Storage, error) {
	return nil, nil
}

func (s *stubPVEClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	return nil, nil
}

func (s *stubPVEClient) GetReplicationStatus(ctx context.Context) ([]proxmox.ReplicationJob, error) {
	return nil, nil
}

func (s *stubPVEClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	return nil, nil
}

func (s *stubPVEClient) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return nil, nil
}

func (s *stubPVEClient) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return nil, nil
}

func (s *stubPVEClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return nil, nil
}

func (s *stubPVEClient) GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error) {
	return nil, nil
}

func (s *stubPVEClient) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubPVEClient) GetContainerInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.ContainerInterface, error) {
	return nil, nil
}

func (s *stubPVEClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return nil, nil
}

func (s *stubPVEClient) IsClusterMember(ctx context.Context) (bool, error) {
	return false, nil
}

func (s *stubPVEClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	return nil, nil
}

func (s *stubPVEClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	return nil, nil
}

func (s *stubPVEClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (s *stubPVEClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	return "", nil
}

func (s *stubPVEClient) GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (s *stubPVEClient) GetZFSPoolStatus(ctx context.Context, node string) ([]proxmox.ZFSPoolStatus, error) {
	return nil, nil
}

func (s *stubPVEClient) GetZFSPoolsWithDetails(ctx context.Context, node string) ([]proxmox.ZFSPoolInfo, error) {
	return nil, nil
}

func (s *stubPVEClient) GetDisks(ctx context.Context, node string) ([]proxmox.Disk, error) {
	return nil, nil
}

func (s *stubPVEClient) GetCephStatus(ctx context.Context) (*proxmox.CephStatus, error) {
	return nil, nil
}

func (s *stubPVEClient) GetCephDF(ctx context.Context) (*proxmox.CephDF, error) {
	return nil, nil
}

func (s *stubPVEClient) GetNodePendingUpdates(ctx context.Context, node string) ([]proxmox.AptPackage, error) {
	return nil, nil
}

func floatPtr(v float64) *float64 { return &v }

func newTestPVEMonitor(instanceName string) *Monitor {
	return &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{{
				Name: instanceName,
				Host: "https://pve",
			}},
		},
		state:                models.NewState(),
		alertManager:         alerts.NewManager(),
		notificationMgr:      notifications.NewNotificationManager(""),
		metricsHistory:       NewMetricsHistory(32, time.Hour),
		nodeSnapshots:        make(map[string]NodeMemorySnapshot),
		guestSnapshots:       make(map[string]GuestMemorySnapshot),
		nodeRRDMemCache:      make(map[string]rrdMemCacheEntry),
		lastClusterCheck:     make(map[string]time.Time),
		lastPhysicalDiskPoll: make(map[string]time.Time),
		failureCounts:        make(map[string]int),
		lastOutcome:          make(map[string]taskOutcome),
		pollStatusMap:        make(map[string]*pollStatus),
		dlqInsightMap:        make(map[string]*dlqInsight),
		authFailures:         make(map[string]int),
		lastAuthAttempt:      make(map[string]time.Time),
		nodeLastOnline:       make(map[string]time.Time),
	}
}

func TestPollPVEInstanceUsesRRDMemUsedFallback(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	total := uint64(16 * 1024 * 1024 * 1024)
	actualUsed := total / 3

	client := &stubPVEClient{
		nodes: []proxmox.Node{
			{
				Node:    "node1",
				Status:  "online",
				CPU:     0.15,
				MaxCPU:  8,
				Mem:     total,
				MaxMem:  total,
				Disk:    0,
				MaxDisk: 0,
				Uptime:  3600,
			},
		},
		nodeStatus: &proxmox.NodeStatus{
			Memory: &proxmox.MemoryStatus{
				Total: total,
				Used:  total,
				Free:  0,
			},
		},
		rrdPoints: []proxmox.NodeRRDPoint{
			{
				MemTotal: floatPtr(float64(total)),
				MemUsed:  floatPtr(float64(actualUsed)),
			},
		},
	}

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.pollPVEInstance(context.Background(), "test", client)

	snapshot := mon.state.GetSnapshot()
	if len(snapshot.Nodes) != 1 {
		t.Fatalf("expected one node in state, got %d", len(snapshot.Nodes))
	}

	node := snapshot.Nodes[0]
	expectedUsage := (float64(actualUsed) / float64(total)) * 100
	if diff := math.Abs(node.Memory.Usage - expectedUsage); diff > 0.5 {
		t.Fatalf("memory usage mismatch: got %.2f want %.2f (diff %.2f)", node.Memory.Usage, expectedUsage, diff)
	}
	if node.Memory.Used != int64(actualUsed) {
		t.Fatalf("memory used mismatch: got %d want %d", node.Memory.Used, actualUsed)
	}

	snapKey := makeNodeSnapshotKey("test", "node1")
	mon.diagMu.RLock()
	snap, ok := mon.nodeSnapshots[snapKey]
	mon.diagMu.RUnlock()
	if !ok {
		t.Fatal("expected node snapshot entry to be recorded")
	}
	if snap.MemorySource != "rrd-memused" {
		t.Fatalf("expected memory source rrd-memused, got %q", snap.MemorySource)
	}
	if snap.Raw.ProxmoxMemorySource != "rrd-memused" {
		t.Fatalf("expected proxmox memory source rrd-memused, got %q", snap.Raw.ProxmoxMemorySource)
	}
	if snap.Raw.RRDUsed != actualUsed {
		t.Fatalf("expected snapshot RRD used %d, got %d", actualUsed, snap.Raw.RRDUsed)
	}
}

func TestPollPVEInstancePreservesRecentNodesWhenGetNodesReturnsEmpty(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &stubPVEClient{
		nodes: []proxmox.Node{
			{
				Node:    "node1",
				Status:  "online",
				CPU:     0.10,
				MaxCPU:  8,
				Mem:     4 * 1024 * 1024 * 1024,
				MaxMem:  8 * 1024 * 1024 * 1024,
				Uptime:  7200,
				MaxDisk: 100 * 1024 * 1024 * 1024,
				Disk:    40 * 1024 * 1024 * 1024,
			},
		},
		nodeStatus: &proxmox.NodeStatus{
			Memory: &proxmox.MemoryStatus{
				Total: 8 * 1024 * 1024 * 1024,
				Used:  4 * 1024 * 1024 * 1024,
				Free:  4 * 1024 * 1024 * 1024,
			},
		},
	}

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.pollPVEInstance(context.Background(), "test", client)

	// Simulate transient API gap: node list temporarily empty.
	client.nodes = nil
	mon.pollPVEInstance(context.Background(), "test", client)

	snapshot := mon.state.GetSnapshot()
	if len(snapshot.Nodes) != 1 {
		t.Fatalf("expected one node in state, got %d", len(snapshot.Nodes))
	}

	node := snapshot.Nodes[0]
	if node.Status == "offline" {
		t.Fatalf("expected recent node to remain non-offline during grace window, got %q", node.Status)
	}
}

func TestPollPVEInstanceMarksStaleNodesOfflineWhenGetNodesReturnsEmpty(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &stubPVEClient{
		nodes: []proxmox.Node{
			{
				Node:    "node1",
				Status:  "online",
				CPU:     0.10,
				MaxCPU:  8,
				Mem:     4 * 1024 * 1024 * 1024,
				MaxMem:  8 * 1024 * 1024 * 1024,
				Uptime:  7200,
				MaxDisk: 100 * 1024 * 1024 * 1024,
				Disk:    40 * 1024 * 1024 * 1024,
			},
		},
		nodeStatus: &proxmox.NodeStatus{
			Memory: &proxmox.MemoryStatus{
				Total: 8 * 1024 * 1024 * 1024,
				Used:  4 * 1024 * 1024 * 1024,
				Free:  4 * 1024 * 1024 * 1024,
			},
		},
	}

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.pollPVEInstance(context.Background(), "test", client)

	first := mon.state.GetSnapshot()
	if len(first.Nodes) != 1 {
		t.Fatalf("expected one node after first poll, got %d", len(first.Nodes))
	}

	staleNode := first.Nodes[0]
	staleNode.LastSeen = time.Now().Add(-nodeOfflineGracePeriod - 2*time.Second)
	mon.state.UpdateNodesForInstance("test", []models.Node{staleNode})

	client.nodes = nil
	mon.pollPVEInstance(context.Background(), "test", client)

	second := mon.state.GetSnapshot()
	if len(second.Nodes) != 1 {
		t.Fatalf("expected one node after fallback poll, got %d", len(second.Nodes))
	}

	node := second.Nodes[0]
	if node.Status != "offline" {
		t.Fatalf("expected stale node to be marked offline, got %q", node.Status)
	}
	if node.ConnectionHealth != "error" {
		t.Fatalf("expected stale node connection health error, got %q", node.ConnectionHealth)
	}
}
