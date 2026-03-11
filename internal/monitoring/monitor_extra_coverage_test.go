package monitoring

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestMonitor_GetConnectionStatuses_MockMode_Extra(t *testing.T) {
	m := &Monitor{
		state:          models.NewState(),
		alertManager:   alerts.NewManager(),
		metricsHistory: NewMetricsHistory(10, time.Hour),
	}
	defer m.alertManager.Stop()

	m.SetMockMode(true)
	defer m.SetMockMode(false)

	statuses := m.GetConnectionStatuses()
	if statuses == nil {
		t.Error("Statuses should not be nil")
	}
}

func TestMonitor_Cleanup_Extra(t *testing.T) {
	m := &Monitor{
		nodeSnapshots:   make(map[string]NodeMemorySnapshot),
		guestSnapshots:  make(map[string]GuestMemorySnapshot),
		nodeRRDMemCache: make(map[string]rrdMemCacheEntry),
	}

	now := time.Now()
	stale := now.Add(-2 * time.Hour)
	fresh := now.Add(-10 * time.Second)

	m.nodeSnapshots["stale"] = NodeMemorySnapshot{RetrievedAt: stale}
	m.nodeSnapshots["fresh"] = NodeMemorySnapshot{RetrievedAt: fresh}
	m.guestSnapshots["stale"] = GuestMemorySnapshot{RetrievedAt: stale}
	m.guestSnapshots["fresh"] = GuestMemorySnapshot{RetrievedAt: fresh}

	m.cleanupDiagnosticSnapshots(now)

	if _, ok := m.nodeSnapshots["stale"]; ok {
		t.Error("Stale node snapshot not removed")
	}
	if _, ok := m.nodeSnapshots["fresh"]; !ok {
		t.Error("Fresh node snapshot removed")
	}
	if _, ok := m.guestSnapshots["stale"]; ok {
		t.Error("Stale guest snapshot not removed")
	}
	if _, ok := m.guestSnapshots["fresh"]; !ok {
		t.Error("Fresh guest snapshot removed")
	}

	// RRD Cache
	m.rrdCacheMu.Lock()
	m.nodeRRDMemCache["stale"] = rrdMemCacheEntry{fetchedAt: stale}
	m.nodeRRDMemCache["fresh"] = rrdMemCacheEntry{fetchedAt: fresh}
	m.rrdCacheMu.Unlock()

	m.cleanupRRDCache(now)

	if _, ok := m.nodeRRDMemCache["stale"]; ok {
		t.Error("Stale RRD cache entry not removed")
	}
	if _, ok := m.nodeRRDMemCache["fresh"]; !ok {
		t.Error("Fresh RRD cache entry removed")
	}
}

func TestMonitor_SetMockMode_Advanced_Extra(t *testing.T) {
	m := &Monitor{
		config: &config.Config{
			DiscoveryEnabled: true,
			DiscoverySubnet:  "192.168.1.0/24",
		},
		state:          models.NewState(),
		alertManager:   alerts.NewManager(),
		metricsHistory: NewMetricsHistory(10, time.Hour),
		runtimeCtx:     context.Background(),
		wsHub:          websocket.NewHub(nil),
	}
	defer m.alertManager.Stop()

	// Switch to mock mode
	m.SetMockMode(true)
	if !mock.IsMockEnabled() {
		t.Error("Mock mode should be enabled")
	}

	// Switch back
	m.SetMockMode(false)
	if mock.IsMockEnabled() {
		t.Error("Mock mode should be disabled")
	}
}

func TestMonitor_GetConfiguredHostIPs_Extra(t *testing.T) {
	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{Host: "https://192.168.1.10:8006"},
				{Host: "192.168.1.11"},
			},
			PBSInstances: []config.PBSInstance{
				{Host: "http://192.168.1.20:8007"},
			},
		},
	}

	ips := m.getConfiguredHostIPs()
	ipMap := make(map[string]bool)
	for _, ip := range ips {
		ipMap[ip] = true
	}

	if !ipMap["192.168.1.10"] {
		t.Error("Missing 192.168.1.10")
	}
	if !ipMap["192.168.1.11"] {
		t.Error("Missing 192.168.1.11")
	}
	if !ipMap["192.168.1.20"] {
		t.Error("Missing 192.168.1.20")
	}
}

func TestMonitor_ConsolidateDuplicateClusters_Extra(t *testing.T) {
	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{Name: "c1", ClusterName: "cluster-A", IsCluster: true, ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "n1"}}},
				{Name: "c2", ClusterName: "cluster-A", IsCluster: true, ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "n2"}}},
				{Name: "c3", ClusterName: "cluster-B", IsCluster: true},
			},
		},
	}

	m.consolidateDuplicateClusters()

	if len(m.config.PVEInstances) != 2 {
		t.Errorf("Expected 2 instances after consolidation, got %d", len(m.config.PVEInstances))
	}

	// c1 should now have n1 and n2 endpoints
	foundC1 := false
	for _, inst := range m.config.PVEInstances {
		if inst.Name == "c1" {
			foundC1 = true
			if len(inst.ClusterEndpoints) != 2 {
				t.Errorf("Expected 2 endpoints in c1, got %d", len(inst.ClusterEndpoints))
			}
		}
	}
	if !foundC1 {
		t.Error("c1 not found in consolidated instances")
	}
}

func TestMonitor_ConsolidateDuplicateClusters_RemovesStandaloneCoveredByClusterEndpoint(t *testing.T) {
	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "homelab",
					ClusterName: "cluster-A",
					IsCluster:   true,
					ClusterEndpoints: []config.ClusterEndpoint{
						{NodeName: "minipc", Host: "https://10.0.0.5:8006"},
					},
				},
				{
					Name:        "minipc-standalone",
					Host:        "10.0.0.5",
					GuestURL:    "https://minipc.example",
					Fingerprint: "fp-standalone",
				},
			},
		},
	}

	m.consolidateDuplicateClusters()

	if len(m.config.PVEInstances) != 1 {
		t.Fatalf("expected 1 instance after consolidation, got %d", len(m.config.PVEInstances))
	}
	cluster := m.config.PVEInstances[0]
	if !cluster.IsCluster {
		t.Fatalf("expected remaining instance to be cluster")
	}
	if len(cluster.ClusterEndpoints) != 1 {
		t.Fatalf("expected 1 cluster endpoint, got %d", len(cluster.ClusterEndpoints))
	}
	if cluster.ClusterEndpoints[0].GuestURL != "https://minipc.example" {
		t.Fatalf("GuestURL = %q, want https://minipc.example", cluster.ClusterEndpoints[0].GuestURL)
	}
	if cluster.ClusterEndpoints[0].Fingerprint != "fp-standalone" {
		t.Fatalf("Fingerprint = %q, want fp-standalone", cluster.ClusterEndpoints[0].Fingerprint)
	}
}

func TestMonitor_ConsolidateDuplicateClusters_RetiresRemovedStandaloneRuntimeState(t *testing.T) {
	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "homelab",
					ClusterName: "cluster-A",
					IsCluster:   true,
					ClusterEndpoints: []config.ClusterEndpoint{
						{NodeName: "minipc", Host: "https://10.0.0.5:8006"},
					},
				},
				{
					Name: "minipc-standalone",
					Host: "10.0.0.5",
				},
			},
		},
		state:           models.NewState(),
		pveClients:      map[string]PVEClientInterface{"homelab": nil, "minipc-standalone": nil},
		taskQueue:       NewTaskQueue(),
		deadLetterQueue: NewTaskQueue(),
		circuitBreakers: map[string]*circuitBreaker{schedulerKey(InstanceTypePVE, "minipc-standalone"): newCircuitBreaker(3, time.Second, time.Minute, 30*time.Second)},
		failureCounts:   map[string]int{schedulerKey(InstanceTypePVE, "minipc-standalone"): 2},
		lastOutcome:     map[string]taskOutcome{schedulerKey(InstanceTypePVE, "minipc-standalone"): {recordedAt: time.Now()}},
		instanceInfoCache: map[string]*instanceInfo{
			schedulerKey(InstanceTypePVE, "minipc-standalone"): {
				Key:         schedulerKey(InstanceTypePVE, "minipc-standalone"),
				Type:        InstanceTypePVE,
				DisplayName: "minipc-standalone",
			},
		},
		pollStatusMap:            map[string]*pollStatus{schedulerKey(InstanceTypePVE, "minipc-standalone"): {}},
		dlqInsightMap:            map[string]*dlqInsight{schedulerKey(InstanceTypePVE, "minipc-standalone"): {}},
		lastClusterCheck:         map[string]time.Time{"minipc-standalone": time.Now()},
		lastPhysicalDiskPoll:     map[string]time.Time{"minipc-standalone": time.Now()},
		lastPVEBackupPoll:        map[string]time.Time{"minipc-standalone": time.Now()},
		backupPermissionWarnings: map[string]string{"minipc-standalone": "warn"},
		nodeLastOnline:           map[string]time.Time{"minipc-standalone-minipc": time.Now()},
		nodePendingUpdatesCache:  map[string]pendingUpdatesCache{"minipc-standalone-minipc": {count: 1, checkedAt: time.Now()}},
		nodeSnapshots:            map[string]NodeMemorySnapshot{"minipc-standalone|minipc": {RetrievedAt: time.Now()}},
		guestSnapshots:           map[string]GuestMemorySnapshot{"minipc-standalone|qemu|minipc|100": {RetrievedAt: time.Now()}},
		guestMetadataCache:       map[string]guestMetadataCacheEntry{"minipc-standalone|minipc|100": {fetchedAt: time.Now()}},
		guestMetadataLimiter:     map[string]time.Time{"minipc-standalone|minipc|100": time.Now().Add(time.Minute)},
	}
	connectionHealthKey := m.connectionHealthStateKey(InstanceTypePVE, "minipc-standalone")
	m.state.SetConnectionHealth(connectionHealthKey, true)
	m.state.UpdateNodesForInstance("minipc-standalone", []models.Node{{ID: "minipc-standalone-minipc", Name: "minipc", Instance: "minipc-standalone"}})
	m.state.UpdateVMsForInstance("minipc-standalone", []models.VM{{ID: "vm-1", VMID: 100, Instance: "minipc-standalone"}})
	m.state.UpdateContainersForInstance("minipc-standalone", []models.Container{{ID: "ct-1", VMID: 101, Instance: "minipc-standalone"}})
	m.state.UpdateStorageForInstance("minipc-standalone", []models.Storage{{ID: "st-1", Instance: "minipc-standalone"}})
	m.state.UpdatePhysicalDisks("minipc-standalone", []models.PhysicalDisk{{ID: "pd-1", Instance: "minipc-standalone"}})
	m.state.UpdateReplicationJobsForInstance("minipc-standalone", []models.ReplicationJob{{ID: "rep-1", Instance: "minipc-standalone"}})
	m.taskQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "minipc-standalone", NextRun: time.Now()})
	m.deadLetterQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "minipc-standalone", NextRun: time.Now()})

	m.consolidateDuplicateClusters()

	if len(m.config.PVEInstances) != 1 {
		t.Fatalf("expected 1 config instance after consolidation, got %d", len(m.config.PVEInstances))
	}
	if _, ok := m.pveClients["minipc-standalone"]; ok {
		t.Fatalf("expected retired standalone client to be removed")
	}
	if _, ok := m.state.ConnectionHealth[connectionHealthKey]; ok {
		t.Fatalf("expected retired standalone connection health to be removed")
	}
	snapshot := m.state.GetSnapshot()
	if len(snapshot.Nodes) != 0 || len(snapshot.VMs) != 0 || len(snapshot.Containers) != 0 || len(snapshot.Storage) != 0 || len(snapshot.PhysicalDisks) != 0 || len(snapshot.ReplicationJobs) != 0 {
		t.Fatalf("expected retired standalone runtime state to be cleared, got %+v", snapshot)
	}
	if m.taskQueue.Size() != 0 {
		t.Fatalf("expected standalone task to be removed from scheduler queue")
	}
	if m.deadLetterQueue.Size() != 0 {
		t.Fatalf("expected standalone task to be removed from dead letter queue")
	}
	if _, ok := m.pollStatusMap[schedulerKey(InstanceTypePVE, "minipc-standalone")]; ok {
		t.Fatalf("expected poll status to be removed for retired instance")
	}
	if _, ok := m.nodeSnapshots["minipc-standalone|minipc"]; ok {
		t.Fatalf("expected node diagnostic snapshots to be cleared")
	}
}

func TestDetectClusterMembership_CanonicalizesAndRetiresStandaloneOverlap(t *testing.T) {
	originalDetect := detectMonitorPVECluster
	t.Cleanup(func() { detectMonitorPVECluster = originalDetect })

	detectMonitorPVECluster = func(clientConfig proxmox.ClientConfig, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return true, "cluster-A", []config.ClusterEndpoint{
			{NodeName: "minipc", Host: "https://10.0.0.5:8006"},
		}
	}

	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "homelab",
					ClusterName: "cluster-A",
					IsCluster:   true,
					ClusterEndpoints: []config.ClusterEndpoint{
						{NodeName: "minipc", Host: "https://10.0.0.5:8006"},
					},
				},
				{
					Name: "minipc-standalone",
					Host: "https://10.0.0.5:8006",
				},
			},
		},
		state:            models.NewState(),
		pveClients:       map[string]PVEClientInterface{"homelab": nil, "minipc-standalone": nil},
		taskQueue:        NewTaskQueue(),
		deadLetterQueue:  NewTaskQueue(),
		lastClusterCheck: make(map[string]time.Time),
	}
	m.state.UpdateNodesForInstance("minipc-standalone", []models.Node{{ID: "minipc-standalone-minipc", Name: "minipc", Instance: "minipc-standalone"}})
	instanceCfg := &m.config.PVEInstances[1]

	m.detectClusterMembership(context.Background(), "minipc-standalone", instanceCfg, &stubPVEClient{})

	if len(m.config.PVEInstances) != 1 {
		t.Fatalf("expected runtime cluster canonicalization to leave 1 instance, got %d", len(m.config.PVEInstances))
	}
	if m.config.PVEInstances[0].Name != "homelab" {
		t.Fatalf("expected surviving instance homelab, got %q", m.config.PVEInstances[0].Name)
	}
	if _, ok := m.pveClients["minipc-standalone"]; ok {
		t.Fatalf("expected retired standalone client to be removed")
	}
	if len(m.state.GetSnapshot().Nodes) != 0 {
		t.Fatalf("expected retired standalone node state to be removed")
	}
}

func TestMonitor_ConsolidateDuplicateClusters_KeepsStandaloneWithoutExplicitEndpointOverlap(t *testing.T) {
	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{
					Name:        "homelab",
					ClusterName: "cluster-A",
					IsCluster:   true,
					ClusterEndpoints: []config.ClusterEndpoint{
						{NodeName: "minipc", Host: "https://minipc.local:8006"},
					},
				},
				{
					Name: "minipc-standalone",
					Host: "https://10.0.0.5:8006",
				},
			},
		},
	}

	m.consolidateDuplicateClusters()

	if len(m.config.PVEInstances) != 2 {
		t.Fatalf("expected standalone to remain without exact endpoint overlap, got %d instances", len(m.config.PVEInstances))
	}
}

func TestMonitor_CleanupGuestMetadataCache_Extra(t *testing.T) {
	m := &Monitor{
		guestMetadataCache:   make(map[string]guestMetadataCacheEntry),
		guestMetadataLimiter: make(map[string]time.Time),
	}

	now := time.Now()
	stale := now.Add(-2 * time.Hour)
	m.guestMetadataCache["stale"] = guestMetadataCacheEntry{fetchedAt: stale}
	m.guestMetadataCache["fresh"] = guestMetadataCacheEntry{fetchedAt: now}
	m.guestMetadataLimiter["stale"] = stale
	m.guestMetadataLimiter["fresh"] = now.Add(-2 * time.Minute)
	m.guestMetadataLimiter["future"] = now.Add(1 * time.Minute)

	m.cleanupGuestMetadataCache(now)

	if _, ok := m.guestMetadataCache["stale"]; ok {
		t.Error("Stale metadata cache entry not removed")
	}
	if _, ok := m.guestMetadataCache["fresh"]; !ok {
		t.Error("Fresh metadata cache entry removed")
	}
	if _, ok := m.guestMetadataLimiter["stale"]; ok {
		t.Error("Stale metadata limiter entry not removed")
	}
	if _, ok := m.guestMetadataLimiter["fresh"]; !ok {
		t.Error("Fresh metadata limiter entry removed")
	}
	if _, ok := m.guestMetadataLimiter["future"]; !ok {
		t.Error("Future metadata limiter entry removed")
	}
}

func TestMonitor_LinkNodeToHostAgent_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}
	m.state.Nodes = []models.Node{{ID: "node1:node1", Name: "node1"}}

	m.linkNodeToHostAgent("node1:node1", "host1")

	if m.state.Nodes[0].LinkedAgentID != "host1" {
		t.Errorf("Expected link to host1, got %s", m.state.Nodes[0].LinkedAgentID)
	}
}

type mockPVEClientExtra struct {
	mockPVEClient
	resources []proxmox.ClusterResource
	vmStatus  *proxmox.VMStatus
	fsInfo    []proxmox.VMFileSystem
	netIfaces []proxmox.VMNetworkInterface
}

func (m *mockPVEClientExtra) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return m.resources, nil
}

func (m *mockPVEClientExtra) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return m.vmStatus, nil
}

func (m *mockPVEClientExtra) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	return m.fsInfo, nil
}

func (m *mockPVEClientExtra) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	return m.netIfaces, nil
}

func (m *mockPVEClientExtra) GetContainers(ctx context.Context, node string) ([]proxmox.Container, error) {
	return []proxmox.Container{}, nil
}

func (m *mockPVEClientExtra) GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error) {
	return &proxmox.Container{
		Status: "running",
		IP:     "192.168.1.101",
		Network: map[string]proxmox.ContainerNetworkConfig{
			"eth0": {Name: "eth0", HWAddr: "00:11:22:33:44:55"},
		},
	}, nil
}

func (m *mockPVEClientExtra) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return map[string]interface{}{"hostname": "ct101"}, nil
}

func (m *mockPVEClientExtra) GetContainerInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.ContainerInterface, error) {
	return []proxmox.ContainerInterface{
		{Name: "eth0", Inet: "192.168.1.101/24"},
	}, nil
}

func (m *mockPVEClientExtra) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return map[string]interface{}{"os": "linux"}, nil
}

func (m *mockPVEClientExtra) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	return "1.0", nil
}

func (m *mockPVEClientExtra) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}

func (m *mockPVEClientExtra) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}

func (m *mockPVEClientExtra) GetNodeStatus(ctx context.Context, node string) (*proxmox.NodeStatus, error) {
	return &proxmox.NodeStatus{CPU: 0.1}, nil
}

func (m *mockPVEClientExtra) GetReplicationStatus(ctx context.Context) ([]proxmox.ReplicationJob, error) {
	return nil, nil
}

func (m *mockPVEClientExtra) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return []proxmox.Snapshot{{Name: "snap1"}}, nil
}

func (m *mockPVEClientExtra) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return []proxmox.Snapshot{{Name: "snap1"}}, nil
}

func (m *mockPVEClientExtra) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	return []proxmox.Storage{{Storage: "local", Content: "images", Active: 1}}, nil
}

func (m *mockPVEClientExtra) GetAllStorage(ctx context.Context) ([]proxmox.Storage, error) {
	return []proxmox.Storage{{Storage: "local", Content: "images", Active: 1}}, nil
}

func (m *mockPVEClientExtra) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	return []proxmox.StorageContent{{Volid: "local:100/snap1", VMID: 100, Size: 1024}}, nil
}

func TestMonitor_PollVMsAndContainersEfficient_Extra(t *testing.T) {
	m := &Monitor{
		state:                    models.NewState(),
		guestAgentFSInfoTimeout:  time.Second,
		guestAgentRetries:        1,
		guestAgentNetworkTimeout: time.Second,
		guestAgentOSInfoTimeout:  time.Second,
		guestAgentVersionTimeout: time.Second,
		guestMetadataCache:       make(map[string]guestMetadataCacheEntry),
		guestMetadataLimiter:     make(map[string]time.Time),
		rateTracker:              NewRateTracker(),
		metricsHistory:           NewMetricsHistory(100, time.Hour),
		alertManager:             alerts.NewManager(),
		stalenessTracker:         NewStalenessTracker(nil),
	}
	defer m.alertManager.Stop()

	client := &mockPVEClientExtra{
		resources: []proxmox.ClusterResource{
			{Type: "qemu", VMID: 100, Name: "vm100", Node: "node1", Status: "running", MaxMem: 2048, Mem: 1024, MaxDisk: 50 * 1024 * 1024 * 1024, Disk: 25 * 1024 * 1024 * 1024},
			{Type: "lxc", VMID: 101, Name: "ct101", Node: "node1", Status: "running", MaxMem: 1024, Mem: 512, MaxDisk: 20 * 1024 * 1024 * 1024, Disk: 5 * 1024 * 1024 * 1024},
		},
		vmStatus: &proxmox.VMStatus{
			Status: "running",
			Agent:  proxmox.VMAgentField{Value: 1},
			MaxMem: 2048,
			Mem:    1024,
		},
		fsInfo: []proxmox.VMFileSystem{
			{Mountpoint: "/", TotalBytes: 100 * 1024 * 1024 * 1024, UsedBytes: 50 * 1024 * 1024 * 1024, Type: "ext4"},
		},
		netIfaces: []proxmox.VMNetworkInterface{
			{Name: "eth0", IPAddresses: []proxmox.VMIPAddress{{Address: "192.168.1.100", Prefix: 24}}},
		},
	}

	nodeStatus := map[string]string{"node1": "online"}
	success := m.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, nodeStatus)

	if !success {
		t.Error("pollVMsAndContainersEfficient failed")
	}

	state := m.GetState()
	if len(state.VMs) != 1 {
		t.Errorf("Expected 1 VM, got %d", len(state.VMs))
	}
	if len(state.Containers) != 1 {
		t.Errorf("Expected 1 Container, got %d", len(state.Containers))
	}
}

func TestMonitor_EnrichContainerMetadata_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}
	container := &models.Container{
		ID:       "pve1:node1:101",
		Instance: "pve1",
		Node:     "node1",
		VMID:     101,
		Status:   "running",
	}
	client := &mockPVEClientExtra{}
	m.enrichContainerMetadata(context.Background(), client, "pve1", "node1", container)

	if len(container.NetworkInterfaces) == 0 {
		t.Error("Expected network interfaces to be enriched")
	}
}

func TestMonitor_TokenBindings_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			APITokens: []config.APITokenRecord{{ID: "token1"}},
		},
		dockerTokenBindings: map[string]string{"token1": "agent1", "orphaned": "agent2"},
		hostTokenBindings:   map[string]string{"token1:host1": "host1", "orphaned:host2": "host2"},
	}

	m.RebuildTokenBindings()

	if _, ok := m.dockerTokenBindings["orphaned"]; ok {
		t.Error("Orphaned docker token binding not removed")
	}
	if _, ok := m.hostTokenBindings["orphaned:host2"]; ok {
		t.Error("Orphaned host token binding not removed")
	}
}

func TestMonitor_TokenBindings_UseUnifiedReadState(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:        "docker-source-1",
				AgentID:   "docker-agent-1",
				Hostname:  "docker-host-1",
				Status:    "online",
				TokenID:   "docker-token",
				TokenName: "Docker Token",
				TokenHint: "dock_1234",
			},
		},
		Hosts: []models.Host{
			{
				ID:        "host-agent-1",
				Hostname:  "host-1",
				Status:    "online",
				TokenID:   "host-token",
				TokenName: "Host Token",
				TokenHint: "host_1234",
			},
		},
	})

	m := &Monitor{
		state:               models.NewState(),
		resourceStore:       unifiedresources.NewMonitorAdapter(registry),
		config:              &config.Config{APITokens: []config.APITokenRecord{{ID: "docker-token"}, {ID: "host-token"}}},
		dockerTokenBindings: map[string]string{"stale": "old-agent"},
		hostTokenBindings:   map[string]string{"stale:host": "old-host"},
	}

	m.RebuildTokenBindings()

	if got := m.dockerTokenBindings["docker-token"]; got != "docker-agent-1" {
		t.Fatalf("expected docker binding from unified read-state, got %q", got)
	}
	if got := m.hostTokenBindings["host-token:host-1"]; got != "host-agent-1" {
		t.Fatalf("expected host binding from unified read-state, got %q", got)
	}
	if _, ok := m.dockerTokenBindings["stale"]; ok {
		t.Fatal("expected stale docker binding to be removed")
	}
	if _, ok := m.hostTokenBindings["stale:host"]; ok {
		t.Fatal("expected stale host binding to be removed")
	}
}

func TestMonitor_StorageBackups_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}
	m.state.UpdateVMsForInstance("pve1", []models.VM{
		{ID: "pve1:node1:100", Instance: "pve1", Node: "node1", VMID: 100},
	})
	m.state.UpdateContainersForInstance("pve1", []models.Container{
		{ID: "pve1:node1:100", Instance: "pve1", Node: "node1", VMID: 100},
	})

	// Create a custom mock client that returns storage and content
	// We need to override the GetStorage and GetStorageContent methods dynamically or via struct fields
	// Since mockPVEClientExtra methods are hardcoded to return simple/nil values, let's define a new struct for this test

	mockClient := &mockPVEClientStorage{
		storage: []proxmox.Storage{{Storage: "local", Content: "backup", Active: 1, Type: "dir", Enabled: 1}},
		content: []proxmox.StorageContent{{Volid: "local:backup/vzdump-qemu-100-2023-01-01.tar.gz", Size: 100, VMID: 100, Content: "backup", Format: "tar.gz"}},
	}

	nodes := []proxmox.Node{{Node: "node1", Status: "online"}, {Node: "node2", Status: "offline"}}
	nodeStatus := map[string]string{"node1": "online", "node2": "offline"}

	m.pollStorageBackupsWithNodes(context.Background(), "pve1", mockClient, nodes, nodeStatus)

	if len(m.state.PVEBackups.StorageBackups) != 1 {
		t.Errorf("Expected 1 backup, got %d", len(m.state.PVEBackups.StorageBackups))
	}
}

type mockPVEClientStorage struct {
	mockPVEClientExtra
	storage     []proxmox.Storage
	content     []proxmox.StorageContent
	failStorage bool
}

func (m *mockPVEClientStorage) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	if m.failStorage {
		return nil, fmt.Errorf("timeout")
	}
	return m.storage, nil
}

func (m *mockPVEClientStorage) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	return m.content, nil
}

func TestMonitor_RetryPVEPortFallback_Extra(t *testing.T) {
	m := &Monitor{
		config: &config.Config{},
	}
	inst := &config.PVEInstance{Host: "https://localhost:8006"}
	client := &mockPVEClientExtra{}

	// Should return early if error is not a port-related connection error
	_, _, err := m.retryPVEPortFallback(context.Background(), "pve1", inst, client, fmt.Errorf("some other error"))
	if err == nil || err.Error() != "some other error" {
		t.Errorf("Expected original error, got %v", err)
	}
}

func TestMonitor_GuestMetadata_Extra(t *testing.T) {
	tempDir := t.TempDir()
	store := config.NewGuestMetadataStore(tempDir, nil)

	// Use store.Set directly to avoid race of async persistGuestIdentity
	_ = store.Set("pve1:node1:100", &config.GuestMetadata{LastKnownName: "vm100", LastKnownType: "qemu"})
	_ = store.Set("pve1:node1:101", &config.GuestMetadata{LastKnownName: "ct101", LastKnownType: "oci"})

	// Test persistGuestIdentity separately for coverage
	persistGuestIdentity(store, "pve1:node1:101", "ct101", "lxc") // Should not downgrade oci

	time.Sleep(100 * time.Millisecond) // Wait for async save

	meta := store.Get("pve1:node1:101")
	if meta == nil || meta.LastKnownType != "oci" {
		t.Errorf("Expected type oci, got %v", meta)
	}

	// Test enrichWithPersistedMetadata
	byVMID := make(map[string][]alerts.GuestLookup)
	enrichWithPersistedMetadata(store, byVMID)

	if len(byVMID["100"]) == 0 {
		t.Error("Expected enriched metadata for VMID 100")
	}
}

func TestMonitor_BackupTimeout_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}
	m.state.UpdateVMsForInstance("pve1", []models.VM{{Instance: "pve1", VMID: 100}})

	timeout := m.calculateBackupOperationTimeout("pve1")
	if timeout < 2*time.Minute {
		t.Errorf("Expected timeout at least 2m, got %v", timeout)
	}
}

type mockResourceStoreExtra struct {
	ResourceStoreInterface
	resources []unifiedresources.Resource
	snapshots []models.StateSnapshot
	freshness time.Time
}

func (m *mockResourceStoreExtra) GetAll() []unifiedresources.Resource {
	return m.resources
}

func (m *mockResourceStoreExtra) PopulateFromSnapshot(snapshot models.StateSnapshot) {
	m.snapshots = append(m.snapshots, snapshot)
}

func (m *mockResourceStoreExtra) UnifiedResourceFreshness() time.Time {
	return m.freshness
}

func TestMonitor_ResourcesForBroadcast_Extra(t *testing.T) {
	m := &Monitor{}
	if m.getResourcesForBroadcast() != nil {
		t.Error("Expected nil when store is nil")
	}

	now := time.Now().UTC()
	total := int64(100)
	used := int64(40)
	uptime := int64(3600)

	m.resourceStore = &mockResourceStoreExtra{
		resources: []unifiedresources.Resource{
			{
				ID:       "node-1",
				Type:     unifiedresources.ResourceTypeAgent,
				Name:     "Node One",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Proxmox: &unifiedresources.ProxmoxData{
					NodeName:    "node1",
					Instance:    "p1",
					Uptime:      uptime,
					ClusterName: "cluster-a",
				},
				Metrics: &unifiedresources.ResourceMetrics{
					CPU:    &unifiedresources.MetricValue{Percent: 12.5},
					Memory: &unifiedresources.MetricValue{Percent: 40, Total: &total, Used: &used},
					NetIn:  &unifiedresources.MetricValue{Value: 111},
					NetOut: &unifiedresources.MetricValue{Value: 222},
				},
				Identity: unifiedresources.ResourceIdentity{
					Hostnames:   []string{"node1"},
					MachineID:   "mid-1",
					IPAddresses: []string{"10.0.0.10"},
				},
			},
		},
	}

	res := m.getResourcesForBroadcast()
	if len(res) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(res))
	}
	if res[0].ID != "node-1" || res[0].Type != "node" || res[0].DisplayName != "Node One" {
		t.Fatalf("unexpected resource identity payload: %#v", res[0])
	}
	if res[0].PlatformType != "proxmox-pve" || res[0].SourceType != "api" || res[0].Status != "online" {
		t.Fatalf("unexpected platform/status payload: %#v", res[0])
	}
	if res[0].CPU == nil || res[0].Memory == nil || res[0].Network == nil {
		t.Fatalf("expected cpu/memory/network payloads, got %#v", res[0])
	}
	if len(res[0].Alerts) != 0 {
		t.Fatalf("expected no direct alert payload, got %#v", res[0].Alerts)
	}
	if res[0].Identity == nil || res[0].Identity.Hostname != "node1" {
		t.Fatalf("expected identity payload to be preserved, got %#v", res[0].Identity)
	}
}

func TestMonitor_BuildBroadcastFrontendState_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}
	stateUpdate := time.Now().Add(-time.Minute).UTC()
	m.state.UpdateNodes([]models.Node{{ID: "node-1", Name: "Node One", Status: "online", LastSeen: stateUpdate}})

	storeFreshness := time.Now().UTC()
	store := &mockResourceStoreExtra{
		resources: []unifiedresources.Resource{
			{
				ID:       "node-1",
				Type:     unifiedresources.ResourceTypeAgent,
				Name:     "Node One",
				Status:   unifiedresources.StatusOnline,
				LastSeen: time.Now().UTC(),
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Proxmox: &unifiedresources.ProxmoxData{
					NodeName: "node1",
					Instance: "p1",
				},
			},
		},
		freshness: storeFreshness,
	}
	m.resourceStore = store

	frontendState := m.BuildBroadcastFrontendState()

	if len(store.snapshots) != 1 {
		t.Fatalf("expected resource store population once, got %d", len(store.snapshots))
	}
	if len(store.snapshots[0].Nodes) != 1 {
		t.Fatalf("expected snapshot nodes to be populated, got %#v", store.snapshots[0].Nodes)
	}
	if len(frontendState.Nodes) != 1 {
		t.Fatalf("expected frontend nodes to be populated, got %#v", frontendState.Nodes)
	}
	if len(frontendState.Resources) != 1 {
		t.Fatalf("expected frontend resources to be populated, got %#v", frontendState.Resources)
	}
	if frontendState.Resources[0].ID != "node-1" {
		t.Fatalf("expected broadcast resource node-1, got %#v", frontendState.Resources[0])
	}
	if frontendState.LastUpdate != storeFreshness.UnixMilli() {
		t.Fatalf("expected broadcast lastUpdate %d from unified store freshness, got %d", storeFreshness.UnixMilli(), frontendState.LastUpdate)
	}
}

func TestMonitor_BuildFrontendState_UsesCanonicalResources(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}
	stateUpdate := time.Now().Add(-time.Minute).UTC()
	m.state.UpdateNodes([]models.Node{{ID: "node-1", Name: "Node One", Status: "online", LastSeen: stateUpdate}})

	storeFreshness := time.Now().UTC()
	store := &mockResourceStoreExtra{
		resources: []unifiedresources.Resource{
			{
				ID:       "node-1",
				Type:     unifiedresources.ResourceTypeAgent,
				Name:     "Node One",
				Status:   unifiedresources.StatusOnline,
				LastSeen: time.Now().UTC(),
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Proxmox: &unifiedresources.ProxmoxData{
					NodeName: "node1",
					Instance: "p1",
				},
			},
		},
		freshness: storeFreshness,
	}
	m.resourceStore = store

	frontendState := m.BuildFrontendState()

	if len(store.snapshots) != 1 {
		t.Fatalf("expected resource store population once, got %d", len(store.snapshots))
	}
	if len(frontendState.Resources) != 1 {
		t.Fatalf("expected frontend resources to be populated, got %#v", frontendState.Resources)
	}
	if frontendState.LastUpdate != storeFreshness.UnixMilli() {
		t.Fatalf("expected frontend lastUpdate %d from unified store freshness, got %d", storeFreshness.UnixMilli(), frontendState.LastUpdate)
	}
}

func TestMonitor_PreviousGuestContextForInstance_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-1", Instance: "pve1", VMID: 101, Name: "vm1"},
			{ID: "vm-2", Instance: "pve2", VMID: 202, Name: "vm2"},
		},
		Containers: []models.Container{
			{ID: "ct-1", Instance: "pve1", VMID: 111, Name: "ct1", Type: "oci", IsOCI: true},
			{ID: "ct-2", Instance: "pve1", VMID: 112, Name: "ct2", Type: "lxc"},
			{ID: "ct-3", Instance: "pve2", VMID: 211, Name: "ct3", Type: "oci", IsOCI: true},
		},
		Hosts: []models.Host{
			{ID: "host-1", LinkedVMID: "vm-1", Status: "online", Memory: models.Memory{Total: 1024}},
			{ID: "host-2", LinkedVMID: "vm-2", Status: "offline", Memory: models.Memory{Total: 1024}},
			{ID: "host-3", LinkedVMID: "vm-3", Status: "online", Memory: models.Memory{Total: 0}},
		},
	})
	m.resourceStore = unifiedresources.NewMonitorAdapter(registry)

	prev := m.previousGuestContextForInstance("pve1")

	if len(prev.vms) != 1 || prev.vms[0].VMID != 101 || prev.vms[0].Instance != "pve1" || prev.vms[0].Name != "vm1" {
		t.Fatalf("expected only pve1 VMs, got %#v", prev.vms)
	}
	if len(prev.containers) != 2 {
		t.Fatalf("expected only pve1 containers, got %#v", prev.containers)
	}
	if !prev.containerOCIByVMID[111] {
		t.Fatalf("expected OCI container VMID 111 to be tracked, got %#v", prev.containerOCIByVMID)
	}
	if prev.containerOCIByVMID[112] || prev.containerOCIByVMID[211] {
		t.Fatalf("unexpected OCI classification leakage: %#v", prev.containerOCIByVMID)
	}
	if len(prev.hostAgentsByVMID) != 1 || prev.hostAgentsByVMID["vm-1"].LinkedVMID != "vm-1" {
		t.Fatalf("expected only online linked host with memory to be tracked, got %#v", prev.hostAgentsByVMID)
	}
}

func TestMonitor_PreviousNodesForInstance_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Instance: "pve1", Name: "a", DisplayName: "a", Memory: models.Memory{Total: 100}},
			{ID: "node-2", Instance: "pve2", Name: "b", DisplayName: "b", Memory: models.Memory{Total: 200}},
		},
	})
	m.resourceStore = unifiedresources.NewMonitorAdapter(registry)

	prevNodeMemory, prevNodes := m.previousNodesForInstance("pve1")

	if len(prevNodes) != 1 || prevNodes[0].Instance != "pve1" || prevNodes[0].DisplayName != "a" {
		t.Fatalf("expected only pve1 nodes, got %#v", prevNodes)
	}
	if mem, ok := prevNodeMemory[prevNodes[0].ID]; !ok || mem.Total != 100 {
		t.Fatalf("expected node memory for node-1, got %#v", prevNodeMemory)
	}
	if len(prevNodeMemory) != 1 {
		t.Fatalf("expected node-2 to be filtered out, got %#v", prevNodeMemory)
	}
}

func TestMonitor_MoreUtilities_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
	}

	// convertAgentSMARTToModels
	smart := []agentshost.DiskSMART{{Device: "/dev/sda", Model: "Samsung"}}
	res := convertAgentSMARTToModels(smart)
	if len(res) != 1 || res[0].Device != "/dev/sda" {
		t.Error("convertAgentSMARTToModels failed")
	}
	convertAgentSMARTToModels(nil)

	// buildPBSBackupCache
	m.state.PBSBackups = []models.PBSBackup{
		{Instance: "pbs1", Datastore: "ds1", BackupTime: time.Now()},
	}
	cache := m.buildPBSBackupCache("pbs1")
	if len(cache) != 1 {
		t.Error("buildPBSBackupCache failed")
	}

	// normalizePBSNamespacePath
	if normalizePBSNamespacePath("/") != "" {
		t.Error("normalizePBSNamespacePath / failed")
	}
	if normalizePBSNamespacePath("ns1") != "ns1" {
		t.Error("normalizePBSNamespacePath ns1 failed")
	}
}

func TestMonitor_AI_Extra(t *testing.T) {
	m := &Monitor{
		alertManager:    alerts.NewManager(),
		notificationMgr: notifications.NewNotificationManager("http://localhost:8080"),
	}
	defer m.alertManager.Stop()

	// Enable alerts
	cfg := m.alertManager.GetConfig()
	cfg.ActivationState = alerts.ActivationActive
	// Set very short grouping window to ensure callback fires immediately for test
	cfg.Schedule.Grouping.Window = 1
	m.alertManager.UpdateConfig(cfg)

	called := make(chan bool)
	m.SetAlertTriggeredAICallback(func(a *alerts.Alert) {
		called <- true
	})

	// Manually wire AlertManager to Monitor (mimicking Start)
	m.alertManager.SetAlertForAICallback(func(alert *alerts.Alert) {
		if m.alertTriggeredAICallback != nil {
			m.alertTriggeredAICallback(alert)
		}
	})

	// Trigger an alert
	host := models.DockerHost{ID: "h1", DisplayName: "h1"}
	// Need 3 confirmations
	m.alertManager.HandleDockerHostOffline(host)
	m.alertManager.HandleDockerHostOffline(host)
	m.alertManager.HandleDockerHostOffline(host)

	select {
	case <-called:
		// Success
	case <-time.After(time.Second):
		t.Error("AI callback not called")
	}
}

func TestMonitor_PruneDockerAlerts_Extra(t *testing.T) {
	m := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManagerWithDataDir(t.TempDir()),
	}
	defer m.alertManager.Stop()

	// Add an active alert for a non-existent docker host
	host := models.DockerHost{ID: "stale-host", DisplayName: "Stale Host"}
	m.alertManager.HandleDockerHostOffline(host)
	m.alertManager.HandleDockerHostOffline(host)
	m.alertManager.HandleDockerHostOffline(host)

	if !m.pruneStaleDockerAlerts() {
		t.Error("Expected stale alert to be pruned")
	}
}

func TestMonitor_SyncAlertsToState_UsesCanonicalAlertID(t *testing.T) {
	m := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManagerWithDataDir(t.TempDir()),
	}
	defer m.alertManager.Stop()

	host := models.DockerHost{ID: "sync-host", DisplayName: "Sync Host", Hostname: "sync-host"}
	m.state.UpsertDockerHost(host)
	m.alertManager.HandleDockerHostOffline(host)
	m.alertManager.HandleDockerHostOffline(host)
	m.alertManager.HandleDockerHostOffline(host)

	m.syncAlertsToState()

	snapshot := m.state.GetSnapshot()
	if len(snapshot.ActiveAlerts) == 0 {
		t.Fatal("expected active alert snapshot")
	}
	alert := snapshot.ActiveAlerts[0]
	if !strings.HasPrefix(alert.ID, "docker:sync-host::") {
		t.Fatalf("snapshot alert ID = %q, want canonical docker host alert ID", alert.ID)
	}
}

func TestMonitor_PruneDockerAlerts_UsesUnifiedReadState(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{ID: "live-host", DisplayName: "Live Host", Hostname: "live-host", Status: "online"},
		},
	})

	m := &Monitor{
		state:         models.NewState(),
		alertManager:  alerts.NewManagerWithDataDir(t.TempDir()),
		resourceStore: unifiedresources.NewMonitorAdapter(registry),
	}
	defer m.alertManager.Stop()

	host := models.DockerHost{ID: "live-host", DisplayName: "Live Host"}
	m.alertManager.HandleDockerHostOffline(host)
	m.alertManager.HandleDockerHostOffline(host)
	m.alertManager.HandleDockerHostOffline(host)

	if m.pruneStaleDockerAlerts() {
		t.Fatal("expected alert pruning to keep hosts present in unified read-state")
	}

	if len(m.alertManager.GetActiveAlerts()) == 0 {
		t.Fatal("expected docker offline alert to remain active")
	}
}

func TestMonitor_AllowExecution_Extra(t *testing.T) {
	m := &Monitor{}
	if !m.allowExecution(ScheduledTask{InstanceType: "pve", InstanceName: "pve1"}) {
		t.Error("Should allow execution when breakers are nil")
	}

	m.circuitBreakers = make(map[string]*circuitBreaker)
	m.allowExecution(ScheduledTask{InstanceType: "pve", InstanceName: "pve1"})
}

func TestMonitor_CephConversion_Detailed_Extra(t *testing.T) {
	// Full population
	ceph := &agentshost.CephCluster{
		FSID: "fsid",
		Health: agentshost.CephHealth{
			Status: "HEALTH_OK",
			Checks: map[string]agentshost.CephCheck{
				"check1": {Severity: "HEALTH_WARN", Message: "msg1", Detail: []string{"d1"}},
			},
			Summary: []agentshost.CephHealthSummary{{Severity: "HEALTH_OK", Message: "ok"}},
		},
		MonMap: agentshost.CephMonitorMap{
			Monitors: []agentshost.CephMonitor{{Name: "mon1", Rank: 0, Addr: "addr1", Status: "up"}},
		},
		MgrMap: agentshost.CephManagerMap{
			ActiveMgr: "mgr1",
		},
		Pools: []agentshost.CephPool{
			{ID: 1, Name: "pool1", BytesUsed: 100, PercentUsed: 0.1},
		},
		Services: []agentshost.CephService{
			{Type: "osd", Running: 1, Total: 1},
		},
		CollectedAt: time.Now().Format(time.RFC3339),
	}

	model := convertAgentCephToModels(ceph)
	if model == nil {
		t.Fatal("Expected non-nil model")
	}
	if len(model.Health.Checks) != 1 {
		t.Error("Expected 1 health check")
	}
	if len(model.MonMap.Monitors) != 1 {
		t.Error("Expected 1 monitor")
	}
	if len(model.Pools) != 1 {
		t.Error("Expected 1 pool")
	}
	if len(model.Services) != 1 {
		t.Error("Expected 1 service")
	}

	// Test convertAgentCephToGlobalCluster with populated data
	global := convertAgentCephToGlobalCluster(ceph, "host1", "h1", time.Now())
	if global.ID != "fsid" {
		t.Errorf("Expected global ID fsid, got %s", global.ID)
	}
	if len(global.Pools) != 1 {
		t.Error("Expected 1 global pool")
	}
	if global.HealthMessage == "" {
		t.Error("Expected health message from checks")
	}

	// Test with missing FSID
	cephEmpty := &agentshost.CephCluster{}
	globalEmpty := convertAgentCephToGlobalCluster(cephEmpty, "host1", "h1", time.Now())
	if globalEmpty.ID != "agent-ceph-h1" {
		t.Errorf("Expected generated ID agent-ceph-h1, got %s", globalEmpty.ID)
	}
}

func TestMonitor_PollPBSBackups_Extra(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
		// pbsClients map not needed for this direct call
	}
	cfg := pbs.ClientConfig{
		Host:       "http://localhost:12345",
		User:       "root@pam",
		TokenName:  "root@pam!test",
		TokenValue: "test",
	}
	client, err := pbs.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ds := []models.PBSDatastore{{Name: "ds1"}}
	m.pollPBSBackups(context.Background(), "pbs1", client, ds)
}

func TestMonitor_RetryPVEPortFallback_Detailed_Extra(t *testing.T) {
	orig := newProxmoxClientFunc
	defer func() { newProxmoxClientFunc = orig }()

	m := &Monitor{
		config: &config.Config{
			ConnectionTimeout: time.Second,
			PVEInstances: []config.PVEInstance{
				{Name: "pve1", Host: "https://localhost:8006"},
			},
		},
		pveClients: make(map[string]PVEClientInterface),
	}

	instanceCfg := &config.PVEInstance{Name: "pve1", Host: "https://localhost:8006"}
	currentClient := &mockPVEClientExtra{}
	cause := fmt.Errorf("dial tcp 127.0.0.1:8006: connect: connection refused")

	// 1. Success case
	newProxmoxClientFunc = func(cfg proxmox.ClientConfig) (PVEClientInterface, error) {
		if strings.Contains(cfg.Host, "8006") {
			return nil, fmt.Errorf("should not be called with 8006 in fallback")
		}
		return &mockPVEClientExtra{}, nil
	}

	nodes, client, err := m.retryPVEPortFallback(context.Background(), "pve1", instanceCfg, currentClient, cause)
	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}
	if client == nil {
		t.Error("Expected fallback client")
	}
	if _, ok := m.pveClients["pve1"]; !ok {
		t.Error("Expected fallback client to be stored on monitor")
	}
	if got := m.config.PVEInstances[0].Host; got != "https://localhost:443" {
		t.Errorf("Expected config host updated to fallback endpoint, got %q", got)
	}
	_ = nodes // ignore

	// 2. Failure to create client
	newProxmoxClientFunc = func(cfg proxmox.ClientConfig) (PVEClientInterface, error) {
		return nil, fmt.Errorf("create failed")
	}
	_, _, err = m.retryPVEPortFallback(context.Background(), "pve1", instanceCfg, currentClient, cause)
	if err != cause {
		t.Error("Expected original cause on client creation failure")
	}

	// 3. Failure to get nodes
	newProxmoxClientFunc = func(cfg proxmox.ClientConfig) (PVEClientInterface, error) {
		// Return a client that fails GetNodes
		return &mockPVEClientFailNodes{}, nil
	}
	_, _, err = m.retryPVEPortFallback(context.Background(), "pve1", instanceCfg, currentClient, cause)
	if err != cause {
		t.Error("Expected original cause on GetNodes failure")
	}
}

type mockPVEClientFailNodes struct {
	mockPVEClientExtra
}

func (m *mockPVEClientFailNodes) GetNodes(ctx context.Context) ([]proxmox.Node, error) {
	return nil, fmt.Errorf("nodes failed")
}

type mockExecutor struct {
	executed []PollTask
}

func (m *mockExecutor) Execute(ctx context.Context, task PollTask) {
	m.executed = append(m.executed, task)
}

func TestMonitor_ExecuteScheduledTask_Extra(t *testing.T) {
	m := &Monitor{
		pveClients: map[string]PVEClientInterface{"pve1": &mockPVEClientExtra{}},
		pbsClients: map[string]*pbs.Client{"pbs1": {}}, // Use real structs or nil
		pmgClients: map[string]*pmg.Client{"pmg1": {}},
	}

	exec := &mockExecutor{}
	m.SetExecutor(exec)

	// PVE Task
	taskPVE := ScheduledTask{InstanceName: "pve1", InstanceType: InstanceTypePVE}
	m.executeScheduledTask(context.Background(), taskPVE)
	if len(exec.executed) != 1 || exec.executed[0].InstanceName != "pve1" {
		t.Error("PVE task not executed")
	}

	// Check failure types (missing client)
	taskPBS := ScheduledTask{InstanceName: "missing", InstanceType: InstanceTypePBS}
	m.executeScheduledTask(context.Background(), taskPBS)
	if len(exec.executed) != 1 {
		t.Error("PBS task should not be executed (missing client)")
	}
}

func TestMonitor_Start_Extra(t *testing.T) {
	t.Setenv("PULSE_MOCK_TRENDS_SEED_DURATION", "5m")
	t.Setenv("PULSE_MOCK_TRENDS_SAMPLE_INTERVAL", "5m")

	m := &Monitor{
		config: &config.Config{
			DiscoveryEnabled: false,
		},
		state:            models.NewState(),
		alertManager:     alerts.NewManager(),
		metricsHistory:   NewMetricsHistory(10, time.Hour),
		rateTracker:      NewRateTracker(),
		stalenessTracker: NewStalenessTracker(nil),
	}
	defer m.alertManager.Stop()

	// Use MockMode to skip discovery
	m.SetMockMode(true)
	defer m.SetMockMode(false)
	m.mockMetricsCancel = func() {} // Skip mock metrics seeding to keep Start responsive in tests.

	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine
	done := make(chan struct{})
	go func() {
		m.Start(ctx, nil)
		close(done)
	}()

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Start did not return after context cancel")
	}
}
