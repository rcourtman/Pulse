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
	allStorage     []proxmox.Storage
	storageByNode  map[string][]proxmox.Storage
	zfsPoolsByNode map[string][]proxmox.ZFSPoolInfo
	cephStatus     *proxmox.CephStatus
	cephDF         *proxmox.CephDF
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

func (f *fakeStorageClient) GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error) {
	return 0, fmt.Errorf("not implemented")
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
	return f.cephStatus, nil
}

func (f *fakeStorageClient) GetCephDF(ctx context.Context) (*proxmox.CephDF, error) {
	return f.cephDF, nil
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

func TestMatchZFSPoolForStorage(t *testing.T) {
	t.Parallel()

	rpool := &models.ZFSPool{Name: "rpool"}
	tank := &models.ZFSPool{Name: "tank"}

	cases := []struct {
		name    string
		storage models.Storage
		pools   map[string]*models.ZFSPool
		want    string
	}{
		{
			name:    "exact storage name match",
			storage: models.Storage{Name: "tank"},
			pools:   map[string]*models.ZFSPool{"tank": tank},
			want:    "tank",
		},
		{
			name:    "matches explicit pool field before alias name",
			storage: models.Storage{Name: "local-zfs", Pool: "rpool/data"},
			pools:   map[string]*models.ZFSPool{"rpool": rpool},
			want:    "rpool",
		},
		{
			name:    "matches pool from dataset path",
			storage: models.Storage{Name: "local-zfs", Path: "/rpool/data"},
			pools:   map[string]*models.ZFSPool{"rpool": rpool},
			want:    "rpool",
		},
		{
			name:    "matches dir storage from dataset path",
			storage: models.Storage{Name: "local", Type: "dir", Path: "/rpool/data"},
			pools:   map[string]*models.ZFSPool{"rpool": rpool},
			want:    "rpool",
		},
		{
			name:    "single pool fallback for local zfs",
			storage: models.Storage{Name: "local-zfs", Type: "local-zfs"},
			pools:   map[string]*models.ZFSPool{"rpool": rpool},
			want:    "rpool",
		},
		{
			name:    "no ambiguous fallback across multiple pools",
			storage: models.Storage{Name: "local-zfs"},
			pools:   map[string]*models.ZFSPool{"rpool": rpool, "tank": tank},
			want:    "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			matched := matchZFSPoolForStorage(tc.storage, tc.pools)
			if tc.want == "" {
				if matched != nil {
					t.Fatalf("matched pool = %q, want nil", matched.Name)
				}
				return
			}
			if matched == nil {
				t.Fatalf("expected pool %q, got nil", tc.want)
			}
			if matched.Name != tc.want {
				t.Fatalf("matched pool = %q, want %q", matched.Name, tc.want)
			}
		})
	}
}

func TestPollStorageWithNodesOptimizedAttachesZFSPoolFromDatasetPath(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	monitor := &Monitor{
		state: &models.State{},
	}

	storage := proxmox.Storage{
		Storage:   "local-zfs",
		Type:      "local-zfs",
		Content:   "images,rootdir",
		Active:    1,
		Enabled:   1,
		Shared:    0,
		Path:      "/rpool/data",
		Total:     1000,
		Used:      400,
		Available: 600,
	}

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{storage},
		storageByNode: map[string][]proxmox.Storage{
			"pve1": {storage},
		},
		zfsPoolsByNode: map[string][]proxmox.ZFSPoolInfo{
			"pve1": {
				{Name: "rpool", State: "ONLINE", Health: "ONLINE"},
			},
		},
	}

	nodes := []proxmox.Node{
		{Node: "pve1", Status: "online"},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	if len(monitor.state.Storage) != 1 {
		t.Fatalf("expected 1 storage entry, got %d", len(monitor.state.Storage))
	}
	if monitor.state.Storage[0].ZFSPool == nil {
		t.Fatal("expected ZFS pool details to be attached")
	}
	if monitor.state.Storage[0].ZFSPool.Name != "rpool" {
		t.Fatalf("ZFS pool name = %q, want rpool", monitor.state.Storage[0].ZFSPool.Name)
	}
}

func TestPollStorageWithNodesOptimizedSynthesizesSharedClusterOnlyStorage(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	monitor := &Monitor{
		state: &models.State{},
	}

	localStorage := proxmox.Storage{
		Storage:   "local",
		Type:      "dir",
		Content:   "images",
		Active:    1,
		Enabled:   1,
		Total:     1000,
		Used:      250,
		Available: 750,
	}

	cephStorage := proxmox.Storage{
		Storage: "ceph-shared",
		Type:    "rbd",
		Content: "images,rootdir",
		Nodes:   "pve1,pve2",
		Pool:    "ceph-pool",
	}

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{localStorage, cephStorage},
		storageByNode: map[string][]proxmox.Storage{
			"pve1": {localStorage},
			"pve2": {},
		},
	}

	nodes := []proxmox.Node{
		{Node: "pve1", Status: "online"},
		{Node: "pve2", Status: "online"},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	if len(monitor.state.Storage) != 2 {
		t.Fatalf("expected 2 storage entries, got %d", len(monitor.state.Storage))
	}

	var shared *models.Storage
	for i := range monitor.state.Storage {
		if monitor.state.Storage[i].ID == "inst1-cluster-ceph-shared" {
			shared = &monitor.state.Storage[i]
			break
		}
	}
	if shared == nil {
		t.Fatal("expected synthesized shared Ceph storage entry")
	}
	if !shared.Shared {
		t.Fatal("expected synthesized storage to be marked shared")
	}
	if shared.Node != "cluster" {
		t.Fatalf("shared storage node = %q, want cluster", shared.Node)
	}
	if shared.Instance != "inst1" {
		t.Fatalf("shared storage instance = %q, want inst1", shared.Instance)
	}
	if shared.Type != "rbd" {
		t.Fatalf("shared storage type = %q, want rbd", shared.Type)
	}
	if shared.Pool != "ceph-pool" {
		t.Fatalf("shared storage pool = %q, want ceph-pool", shared.Pool)
	}
	if shared.NodeCount != 2 {
		t.Fatalf("shared storage node count = %d, want 2", shared.NodeCount)
	}
	if len(shared.Nodes) != 2 || shared.Nodes[0] != "pve1" || shared.Nodes[1] != "pve2" {
		t.Fatalf("shared storage nodes = %#v, want [pve1 pve2]", shared.Nodes)
	}
	if len(shared.NodeIDs) != 2 || shared.NodeIDs[0] != "inst1-pve1" || shared.NodeIDs[1] != "inst1-pve2" {
		t.Fatalf("shared storage node IDs = %#v, want [inst1-pve1 inst1-pve2]", shared.NodeIDs)
	}
}

func TestPollStorageWithNodesOptimizedHydratesSharedCephStorageFromDF(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	monitor := &Monitor{
		state:          &models.State{},
		metricsHistory: NewMetricsHistory(16, time.Hour),
		alertManager:   alerts.NewManager(),
	}
	t.Cleanup(func() {
		monitor.alertManager.Stop()
	})

	cfg := monitor.alertManager.GetConfig()
	cfg.MinimumDelta = 0
	if cfg.TimeThresholds == nil {
		cfg.TimeThresholds = make(map[string]int)
	}
	cfg.TimeThresholds["storage"] = 0
	cfg.StorageDefault = alerts.HysteresisThreshold{Trigger: 10, Clear: 5}
	monitor.alertManager.UpdateConfig(cfg)

	cephStorage := proxmox.Storage{
		Storage: "ceph-shared",
		Type:    "rbd",
		Content: "images,rootdir",
		Nodes:   "pve1,pve2",
		Pool:    "ceph-pool",
	}

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{cephStorage},
		storageByNode: map[string][]proxmox.Storage{
			"pve1": {},
			"pve2": {},
		},
		cephStatus: &proxmox.CephStatus{
			FSID: "ceph-fsid",
			PGMap: proxmox.CephPGMap{
				BytesTotal: 1000,
				BytesUsed:  200,
				BytesAvail: 800,
			},
		},
		cephDF: &proxmox.CephDF{
			Data: proxmox.CephDFData{
				Pools: []proxmox.CephDFPool{
					{
						ID:   1,
						Name: "ceph-pool",
						Stats: proxmox.CephDFPoolStat{
							BytesUsed:   200,
							MaxAvail:    800,
							PercentUsed: 20,
						},
					},
				},
			},
		},
	}

	nodes := []proxmox.Node{
		{Node: "pve1", Status: "online"},
		{Node: "pve2", Status: "online"},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	var shared *models.Storage
	for i := range monitor.state.Storage {
		if monitor.state.Storage[i].ID == "inst1-cluster-ceph-shared" {
			shared = &monitor.state.Storage[i]
			break
		}
	}
	if shared == nil {
		t.Fatal("expected synthesized shared Ceph storage entry")
	}
	if shared.Total != 1000 {
		t.Fatalf("shared storage total = %d, want 1000", shared.Total)
	}
	if shared.Used != 200 {
		t.Fatalf("shared storage used = %d, want 200", shared.Used)
	}
	if shared.Free != 800 {
		t.Fatalf("shared storage free = %d, want 800", shared.Free)
	}
	if diff := math.Abs(shared.Usage - 20); diff > 0.001 {
		t.Fatalf("shared storage usage = %.4f, want 20", shared.Usage)
	}

	alerts := monitor.alertManager.GetActiveAlerts()
	found := false
	for _, alert := range alerts {
		if alert.ID == "inst1-cluster-ceph-shared-usage" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected shared Ceph storage usage alert to be active")
	}
}

func TestPollStorageWithNodesOptimizedChecksCephPoolAlerts(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	monitor := &Monitor{
		state:          &models.State{},
		metricsHistory: NewMetricsHistory(16, time.Hour),
		alertManager:   alerts.NewManager(),
	}
	t.Cleanup(func() {
		monitor.alertManager.Stop()
	})

	cfg := monitor.alertManager.GetConfig()
	cfg.MinimumDelta = 0
	if cfg.TimeThresholds == nil {
		cfg.TimeThresholds = make(map[string]int)
	}
	cfg.TimeThresholds["storage"] = 0
	cfg.StorageDefault = alerts.HysteresisThreshold{Trigger: 80, Clear: 70}
	monitor.alertManager.UpdateConfig(cfg)

	cephStorage := proxmox.Storage{
		Storage: "ceph-shared",
		Type:    "rbd",
		Content: "images,rootdir",
		Nodes:   "pve1,pve2",
		Pool:    "ceph-pool",
	}

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{cephStorage},
		storageByNode: map[string][]proxmox.Storage{
			"pve1": {},
			"pve2": {},
		},
		cephStatus: &proxmox.CephStatus{
			FSID: "ceph-fsid",
			PGMap: proxmox.CephPGMap{
				BytesTotal: 5000,
				BytesUsed:  4500,
				BytesAvail: 500,
			},
		},
		cephDF: &proxmox.CephDF{
			Data: proxmox.CephDFData{
				Pools: []proxmox.CephDFPool{
					{
						ID:   2,
						Name: "data_replication",
						Stats: proxmox.CephDFPoolStat{
							BytesUsed:   910,
							MaxAvail:    90,
							PercentUsed: 91,
						},
					},
				},
			},
		},
	}

	nodes := []proxmox.Node{
		{Node: "pve1", Status: "online"},
		{Node: "pve2", Status: "online"},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	for _, storage := range monitor.state.Storage {
		if storage.ID == "inst1-ceph-pool-data_replication" {
			t.Fatal("Ceph pool alert target should not be inserted into the main storage inventory")
		}
	}

	metrics := monitor.metricsHistory.GetAllStorageMetrics("inst1-ceph-pool-data_replication", time.Minute)
	if len(metrics["usage"]) != 1 {
		t.Fatalf("expected one Ceph pool usage metric entry, got %d", len(metrics["usage"]))
	}
	if diff := math.Abs(metrics["usage"][0].Value - 91); diff > 0.001 {
		t.Fatalf("expected Ceph pool usage metric 91, diff %.4f", diff)
	}

	alerts := monitor.alertManager.GetActiveAlerts()
	found := false
	for _, alert := range alerts {
		if alert.ID == "inst1-ceph-pool-data_replication-usage" {
			found = true
			if alert.ResourceName != "data_replication" {
				t.Fatalf("Ceph pool alert resource name = %q, want data_replication", alert.ResourceName)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected Ceph pool usage alert to be active")
	}
}

// Regression for #1341: a per-pool Ceph threshold override (lower than the
// storage default) must fire when the pool's usage exceeds the override
// trigger. The reporter set a 50% threshold on a Ceph pool sitting at ~61%
// and never received an alert.
func TestPollStorageWithNodesOptimizedAppliesCephPoolOverride(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	monitor := &Monitor{
		state:          &models.State{},
		metricsHistory: NewMetricsHistory(16, time.Hour),
		alertManager:   alerts.NewManager(),
	}
	t.Cleanup(func() {
		monitor.alertManager.Stop()
	})

	overrideTrigger := alerts.HysteresisThreshold{Trigger: 50, Clear: 45}
	cfg := monitor.alertManager.GetConfig()
	cfg.MinimumDelta = 0
	if cfg.TimeThresholds == nil {
		cfg.TimeThresholds = make(map[string]int)
	}
	cfg.TimeThresholds["storage"] = 0
	// Default high enough that only the override could fire.
	cfg.StorageDefault = alerts.HysteresisThreshold{Trigger: 95, Clear: 90}
	cfg.Overrides = map[string]alerts.ThresholdConfig{
		"inst1-ceph-pool-data_replication": {Usage: &overrideTrigger},
	}
	monitor.alertManager.UpdateConfig(cfg)

	cephStorage := proxmox.Storage{
		Storage: "ceph-shared",
		Type:    "rbd",
		Content: "images,rootdir",
		Nodes:   "pve1,pve2",
		Pool:    "ceph-pool",
	}

	client := &fakeStorageClient{
		allStorage: []proxmox.Storage{cephStorage},
		storageByNode: map[string][]proxmox.Storage{
			"pve1": {},
			"pve2": {},
		},
		cephStatus: &proxmox.CephStatus{
			FSID: "ceph-fsid",
			PGMap: proxmox.CephPGMap{
				BytesTotal: 5000,
				BytesUsed:  3050,
				BytesAvail: 1950,
			},
		},
		cephDF: &proxmox.CephDF{
			Data: proxmox.CephDFData{
				Pools: []proxmox.CephDFPool{
					{
						ID:   2,
						Name: "data_replication",
						Stats: proxmox.CephDFPoolStat{
							BytesUsed:   611,
							MaxAvail:    389,
							PercentUsed: 61.1,
						},
					},
				},
			},
		},
	}

	nodes := []proxmox.Node{
		{Node: "pve1", Status: "online"},
		{Node: "pve2", Status: "online"},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", client, nodes)

	active := monitor.alertManager.GetActiveAlerts()
	found := false
	for _, alert := range active {
		if alert.ID == "inst1-ceph-pool-data_replication-usage" {
			found = true
			if alert.Threshold != 50 {
				t.Fatalf("Ceph pool alert fired but threshold = %.1f, want 50 (override)", alert.Threshold)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected Ceph pool usage alert to fire from per-pool 50%% override at 61.1%% usage; got %d active alerts", len(active))
	}
}

func TestPollStorageWithNodesOptimizedClearsStaleStorageAlertsWhenIdentityChanges(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	monitor := &Monitor{
		state:          &models.State{},
		metricsHistory: NewMetricsHistory(16, time.Hour),
		alertManager:   alerts.NewManager(),
	}
	t.Cleanup(func() {
		monitor.alertManager.Stop()
	})

	cfg := monitor.alertManager.GetConfig()
	cfg.MinimumDelta = 0
	if cfg.TimeThresholds == nil {
		cfg.TimeThresholds = make(map[string]int)
	}
	cfg.TimeThresholds["storage"] = 0
	cfg.StorageDefault = alerts.HysteresisThreshold{Trigger: 80, Clear: 70}
	monitor.alertManager.UpdateConfig(cfg)

	nodes := []proxmox.Node{{Node: "node1", Status: "online"}}

	oldStorage := proxmox.Storage{
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
	oldClient := &fakeStorageClient{
		allStorage: []proxmox.Storage{oldStorage},
		storageByNode: map[string][]proxmox.Storage{
			"node1": {oldStorage},
		},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", oldClient, nodes)

	foundOldAlert := false
	for _, alert := range monitor.alertManager.GetActiveAlerts() {
		if alert.ID == "inst1-node1-local-usage" {
			foundOldAlert = true
			break
		}
	}
	if !foundOldAlert {
		t.Fatal("expected initial storage usage alert to be active")
	}

	newStorage := proxmox.Storage{
		Storage:   "rootfs",
		Type:      "dir",
		Content:   "images",
		Active:    1,
		Enabled:   1,
		Shared:    0,
		Total:     1000,
		Used:      200,
		Available: 800,
	}
	newClient := &fakeStorageClient{
		allStorage: []proxmox.Storage{newStorage},
		storageByNode: map[string][]proxmox.Storage{
			"node1": {newStorage},
		},
	}

	monitor.pollStorageWithNodes(context.Background(), "inst1", newClient, nodes)

	foundOldAlert = false
	foundNewAlert := false
	for _, alert := range monitor.alertManager.GetActiveAlerts() {
		switch alert.ID {
		case "inst1-node1-local-usage":
			foundOldAlert = true
		case "inst1-node1-rootfs-usage":
			foundNewAlert = true
		}
	}
	if foundOldAlert {
		t.Fatal("expected stale storage usage alert to be cleared after storage identity changed")
	}
	if foundNewAlert {
		t.Fatal("expected no usage alert for replacement storage below threshold")
	}
}

func TestPollStorageWithNodesOptimizedAttachesZFSPoolForDirStorageOnDatasetPath(t *testing.T) {
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
		Storage:   "local",
		Type:      "dir",
		Path:      "/rpool/data",
		Content:   "images",
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
	if monitor.state.Storage[0].ZFSPool == nil {
		t.Fatal("expected dir storage on ZFS dataset path to have ZFS pool attached")
	}
	if monitor.state.Storage[0].ZFSPool.Name != "rpool" {
		t.Fatalf("ZFS pool name = %q, want rpool", monitor.state.Storage[0].ZFSPool.Name)
	}
}

func TestPollStorageWithNodesOptimizedAttachesZFSPoolFromExplicitPoolField(t *testing.T) {
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
	if monitor.state.Storage[0].ZFSPool == nil {
		t.Fatal("expected zfspool storage with explicit pool field to have ZFS pool attached")
	}
	if monitor.state.Storage[0].ZFSPool.Name != "rpool" {
		t.Fatalf("ZFS pool name = %q, want rpool", monitor.state.Storage[0].ZFSPool.Name)
	}
}
