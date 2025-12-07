package resources

import (
	"testing"
	"time"
)

func TestStoreUpsertAndGet(t *testing.T) {
	store := NewStore()

	r := Resource{
		ID:           "test-1",
		Type:         ResourceTypeNode,
		Name:         "node1",
		PlatformType: PlatformProxmoxPVE,
		SourceType:   SourceAPI,
		Status:       StatusOnline,
		LastSeen:     time.Now(),
	}

	id := store.Upsert(r)
	if id != "test-1" {
		t.Errorf("Expected ID test-1, got %s", id)
	}

	retrieved, ok := store.Get("test-1")
	if !ok {
		t.Fatal("Failed to retrieve resource")
	}
	if retrieved.Name != "node1" {
		t.Errorf("Expected name node1, got %s", retrieved.Name)
	}
}

func TestStoreGetAll(t *testing.T) {
	store := NewStore()

	store.Upsert(Resource{ID: "1", Type: ResourceTypeNode, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "2", Type: ResourceTypeVM, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "3", Type: ResourceTypeContainer, LastSeen: time.Now()})

	all := store.GetAll()
	if len(all) != 3 {
		t.Errorf("Expected 3 resources, got %d", len(all))
	}
}

func TestStoreGetByType(t *testing.T) {
	store := NewStore()

	store.Upsert(Resource{ID: "node1", Type: ResourceTypeNode, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "node2", Type: ResourceTypeNode, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "vm1", Type: ResourceTypeVM, LastSeen: time.Now()})

	nodes := store.GetByType(ResourceTypeNode)
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}

	vms := store.GetByType(ResourceTypeVM)
	if len(vms) != 1 {
		t.Errorf("Expected 1 VM, got %d", len(vms))
	}
}

func TestStoreGetByPlatform(t *testing.T) {
	store := NewStore()

	store.Upsert(Resource{ID: "1", PlatformType: PlatformProxmoxPVE, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "2", PlatformType: PlatformProxmoxPVE, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "3", PlatformType: PlatformDocker, LastSeen: time.Now()})

	pve := store.GetByPlatform(PlatformProxmoxPVE)
	if len(pve) != 2 {
		t.Errorf("Expected 2 PVE resources, got %d", len(pve))
	}

	docker := store.GetByPlatform(PlatformDocker)
	if len(docker) != 1 {
		t.Errorf("Expected 1 Docker resource, got %d", len(docker))
	}
}

func TestStoreGetInfrastructureAndWorkloads(t *testing.T) {
	store := NewStore()

	store.Upsert(Resource{ID: "node1", Type: ResourceTypeNode, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "host1", Type: ResourceTypeHost, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "vm1", Type: ResourceTypeVM, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "ct1", Type: ResourceTypeContainer, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "dc1", Type: ResourceTypeDockerContainer, LastSeen: time.Now()})

	infra := store.GetInfrastructure()
	if len(infra) != 2 {
		t.Errorf("Expected 2 infrastructure resources, got %d", len(infra))
	}

	workloads := store.GetWorkloads()
	if len(workloads) != 3 {
		t.Errorf("Expected 3 workload resources, got %d", len(workloads))
	}
}

func TestStoreGetChildren(t *testing.T) {
	store := NewStore()

	store.Upsert(Resource{ID: "node1", Type: ResourceTypeNode, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "vm1", Type: ResourceTypeVM, ParentID: "node1", LastSeen: time.Now()})
	store.Upsert(Resource{ID: "vm2", Type: ResourceTypeVM, ParentID: "node1", LastSeen: time.Now()})
	store.Upsert(Resource{ID: "vm3", Type: ResourceTypeVM, ParentID: "node2", LastSeen: time.Now()})

	children := store.GetChildren("node1")
	if len(children) != 2 {
		t.Errorf("Expected 2 children of node1, got %d", len(children))
	}
}

func TestStoreRemove(t *testing.T) {
	store := NewStore()

	store.Upsert(Resource{ID: "test-1", LastSeen: time.Now()})
	store.Upsert(Resource{ID: "test-2", LastSeen: time.Now()})

	if len(store.GetAll()) != 2 {
		t.Fatal("Expected 2 resources before remove")
	}

	store.Remove("test-1")

	if len(store.GetAll()) != 1 {
		t.Error("Expected 1 resource after remove")
	}

	_, ok := store.Get("test-1")
	if ok {
		t.Error("Removed resource should not be retrievable")
	}
}

func TestDeduplicationByHostname(t *testing.T) {
	store := NewStore()

	now := time.Now()

	// Add a Proxmox node (API source)
	nodeResource := Resource{
		ID:           "pve1/node/server1",
		Type:         ResourceTypeNode,
		Name:         "server1",
		PlatformType: PlatformProxmoxPVE,
		SourceType:   SourceAPI,
		Status:       StatusOnline,
		CPU:          &MetricValue{Current: 50.0},
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "server1",
		},
	}
	store.Upsert(nodeResource)

	// Add a host agent for the same server (agent source - should be preferred)
	hostResource := Resource{
		ID:           "host-agent/server1",
		Type:         ResourceTypeHost,
		Name:         "server1",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		Status:       StatusOnline,
		CPU:          &MetricValue{Current: 55.0}, // Slightly different value
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "server1",
		},
	}
	store.Upsert(hostResource)

	// We should only have 1 resource (the agent one, as it's preferred)
	all := store.GetAll()
	if len(all) != 1 {
		t.Errorf("Expected 1 resource after dedup, got %d", len(all))
	}

	// The agent resource should be the one stored
	r, ok := store.Get("host-agent/server1")
	if !ok {
		t.Fatal("Failed to get agent resource")
	}
	if r.SourceType != SourceAgent {
		t.Errorf("Expected agent source, got %s", r.SourceType)
	}
	if r.CPU.Current != 55.0 {
		t.Errorf("Expected CPU 55.0 from agent, got %f", r.CPU.Current)
	}

	// The node resource should redirect to the agent resource
	if !store.IsSuppressed("pve1/node/server1") {
		t.Error("Node resource should be suppressed")
	}
	preferred := store.GetPreferredID("pve1/node/server1")
	if preferred != "host-agent/server1" {
		t.Errorf("Expected preferred ID to be host-agent/server1, got %s", preferred)
	}

	// Accessing by suppressed ID should return the preferred resource
	r, ok = store.Get("pve1/node/server1")
	if !ok {
		t.Fatal("Should be able to get by suppressed ID")
	}
	if r.ID != "host-agent/server1" {
		t.Errorf("Expected to get agent resource when accessing by node ID, got %s", r.ID)
	}
}

func TestDeduplicationByMachineID(t *testing.T) {
	store := NewStore()

	now := time.Now()
	machineID := "abc-123-def-456"

	// Add a Docker host
	dockerHost := Resource{
		ID:           "docker-host-1",
		Type:         ResourceTypeDockerHost,
		Name:         "server-different-name",
		PlatformType: PlatformDocker,
		SourceType:   SourceAgent,
		Status:       StatusOnline,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname:  "server-different-name",
			MachineID: machineID,
		},
	}
	store.Upsert(dockerHost)

	// Add a host agent with the same machine ID but different hostname
	hostAgent := Resource{
		ID:           "host-agent-1",
		Type:         ResourceTypeHost,
		Name:         "server-production",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		Status:       StatusOnline,
		LastSeen:     now.Add(time.Second), // Slightly newer
		Identity: &ResourceIdentity{
			Hostname:  "server-production",
			MachineID: machineID,
		},
	}
	store.Upsert(hostAgent)

	// Should only have 1 resource (the newer one, since both are agent sources)
	all := store.GetAll()
	if len(all) != 1 {
		t.Errorf("Expected 1 resource after dedup by machineID, got %d", len(all))
	}
}

func TestDeduplicationByIP(t *testing.T) {
	store := NewStore()

	now := time.Now()
	sharedIP := "192.168.1.100"

	// Add a Proxmox node
	node := Resource{
		ID:           "node-1",
		Type:         ResourceTypeNode,
		Name:         "pve-node",
		PlatformType: PlatformProxmoxPVE,
		SourceType:   SourceAPI,
		Status:       StatusOnline,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "pve-node",
			IPs:      []string{sharedIP},
		},
	}
	store.Upsert(node)

	// Add a host agent with the same IP
	host := Resource{
		ID:           "host-1",
		Type:         ResourceTypeHost,
		Name:         "different-hostname",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent, // Agent preferred
		Status:       StatusOnline,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "different-hostname",
			IPs:      []string{sharedIP},
		},
	}
	store.Upsert(host)

	// Should only have 1 resource
	all := store.GetAll()
	if len(all) != 1 {
		t.Errorf("Expected 1 resource after dedup by IP, got %d", len(all))
	}

	// Agent should be preferred
	r, _ := store.Get("host-1")
	if r == nil || r.SourceType != SourceAgent {
		t.Error("Agent resource should be preferred")
	}
}

func TestNoDeduplicationForWorkloads(t *testing.T) {
	store := NewStore()

	now := time.Now()

	// VMs with the same hostname should NOT be deduplicated
	// (they're workloads, not infrastructure)
	vm1 := Resource{
		ID:           "pve1/vm/100",
		Type:         ResourceTypeVM,
		Name:         "webserver",
		PlatformType: PlatformProxmoxPVE,
		SourceType:   SourceAPI,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "webserver", // Same hostname
		},
	}
	store.Upsert(vm1)

	vm2 := Resource{
		ID:           "pve2/vm/100",
		Type:         ResourceTypeVM,
		Name:         "webserver",
		PlatformType: PlatformProxmoxPVE,
		SourceType:   SourceAPI,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "webserver", // Same hostname
		},
	}
	store.Upsert(vm2)

	// Both VMs should exist (workloads are not deduplicated)
	all := store.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 VMs (no dedup for workloads), got %d", len(all))
	}
}

func TestNoDeduplicationForLocalhost(t *testing.T) {
	store := NewStore()

	now := time.Now()

	host1 := Resource{
		ID:           "host-1",
		Type:         ResourceTypeHost,
		Name:         "server1",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "server1",
			IPs:      []string{"127.0.0.1", "192.168.1.1"},
		},
	}
	store.Upsert(host1)

	host2 := Resource{
		ID:           "host-2",
		Type:         ResourceTypeHost,
		Name:         "server2",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "server2",
			IPs:      []string{"127.0.0.1", "192.168.1.2"}, // Both have localhost
		},
	}
	store.Upsert(host2)

	// Both should exist (127.0.0.1 shouldn't trigger dedup)
	all := store.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 hosts (localhost shouldn't dedup), got %d", len(all))
	}
}

func TestStoreStats(t *testing.T) {
	store := NewStore()

	store.Upsert(Resource{ID: "1", Type: ResourceTypeNode, PlatformType: PlatformProxmoxPVE, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "2", Type: ResourceTypeNode, PlatformType: PlatformProxmoxPVE, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "3", Type: ResourceTypeVM, PlatformType: PlatformProxmoxPVE, LastSeen: time.Now()})
	store.Upsert(Resource{ID: "4", Type: ResourceTypeDockerHost, PlatformType: PlatformDocker, LastSeen: time.Now()})

	stats := store.GetStats()

	if stats.TotalResources != 4 {
		t.Errorf("Expected 4 total resources, got %d", stats.TotalResources)
	}
	if stats.ByType[ResourceTypeNode] != 2 {
		t.Errorf("Expected 2 nodes, got %d", stats.ByType[ResourceTypeNode])
	}
	if stats.ByPlatform[PlatformProxmoxPVE] != 3 {
		t.Errorf("Expected 3 PVE resources, got %d", stats.ByPlatform[PlatformProxmoxPVE])
	}
}

func TestStoreQuery(t *testing.T) {
	store := NewStore()

	store.Upsert(Resource{
		ID:           "1",
		Type:         ResourceTypeNode,
		PlatformType: PlatformProxmoxPVE,
		Status:       StatusOnline,
		LastSeen:     time.Now(),
	})
	store.Upsert(Resource{
		ID:           "2",
		Type:         ResourceTypeVM,
		PlatformType: PlatformProxmoxPVE,
		Status:       StatusRunning,
		ParentID:     "1",
		LastSeen:     time.Now(),
	})
	store.Upsert(Resource{
		ID:           "3",
		Type:         ResourceTypeVM,
		PlatformType: PlatformProxmoxPVE,
		Status:       StatusStopped,
		ParentID:     "1",
		LastSeen:     time.Now(),
	})
	store.Upsert(Resource{
		ID:           "4",
		Type:         ResourceTypeDockerContainer,
		PlatformType: PlatformDocker,
		Status:       StatusRunning,
		LastSeen:     time.Now(),
	})

	// Query by type
	vms := store.Query().OfType(ResourceTypeVM).Execute()
	if len(vms) != 2 {
		t.Errorf("Expected 2 VMs, got %d", len(vms))
	}

	// Query by status
	running := store.Query().WithStatus(StatusRunning).Execute()
	if len(running) != 2 {
		t.Errorf("Expected 2 running resources, got %d", len(running))
	}

	// Query by platform
	pve := store.Query().FromPlatform(PlatformProxmoxPVE).Execute()
	if len(pve) != 3 {
		t.Errorf("Expected 3 PVE resources, got %d", len(pve))
	}

	// Query by parent
	node1Children := store.Query().WithParent("1").Execute()
	if len(node1Children) != 2 {
		t.Errorf("Expected 2 children of node 1, got %d", len(node1Children))
	}

	// Combined query
	runningVMs := store.Query().
		OfType(ResourceTypeVM).
		WithStatus(StatusRunning).
		Execute()
	if len(runningVMs) != 1 {
		t.Errorf("Expected 1 running VM, got %d", len(runningVMs))
	}

	// Count
	count := store.Query().OfType(ResourceTypeVM).Count()
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}

	// Limit
	limited := store.Query().Limit(2).Execute()
	if len(limited) > 2 {
		t.Errorf("Expected at most 2 results, got %d", len(limited))
	}
}

func TestMarkStale(t *testing.T) {
	store := NewStore()

	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()

	store.Upsert(Resource{
		ID:       "old-1",
		Status:   StatusOnline,
		LastSeen: old,
	})
	store.Upsert(Resource{
		ID:       "recent-1",
		Status:   StatusOnline,
		LastSeen: recent,
	})

	stale := store.MarkStale(time.Hour)

	if len(stale) != 1 {
		t.Errorf("Expected 1 stale resource, got %d", len(stale))
	}

	r, _ := store.Get("old-1")
	if r.Status != StatusDegraded {
		t.Errorf("Expected stale resource to be degraded, got %s", r.Status)
	}

	r, _ = store.Get("recent-1")
	if r.Status != StatusOnline {
		t.Errorf("Recent resource should still be online, got %s", r.Status)
	}
}

func TestPruneStale(t *testing.T) {
	store := NewStore()

	veryOld := time.Now().Add(-48 * time.Hour)
	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()

	store.Upsert(Resource{ID: "very-old", LastSeen: veryOld})
	store.Upsert(Resource{ID: "old", LastSeen: old})
	store.Upsert(Resource{ID: "recent", LastSeen: recent})

	removed := store.PruneStale(time.Hour, 24*time.Hour)

	if len(removed) != 1 {
		t.Errorf("Expected 1 removed resource, got %d", len(removed))
	}

	if len(store.GetAll()) != 2 {
		t.Errorf("Expected 2 remaining resources, got %d", len(store.GetAll()))
	}
}

func TestAPIToAgentPreference(t *testing.T) {
	store := NewStore()

	now := time.Now()

	// First, add an API resource
	apiResource := Resource{
		ID:           "api-node",
		Type:         ResourceTypeNode,
		Name:         "server",
		PlatformType: PlatformProxmoxPVE,
		SourceType:   SourceAPI,
		CPU:          &MetricValue{Current: 50.0},
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "server",
		},
	}
	store.Upsert(apiResource)

	// Then, add an agent resource for the same machine
	agentResource := Resource{
		ID:           "agent-host",
		Type:         ResourceTypeHost,
		Name:         "server",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		CPU:          &MetricValue{Current: 55.0},
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "server",
		},
	}
	store.Upsert(agentResource)

	// Only agent resource should exist
	all := store.GetAll()
	if len(all) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(all))
	}

	if all[0].SourceType != SourceAgent {
		t.Errorf("Expected agent source type, got %s", all[0].SourceType)
	}
}

func TestGetTopByCPU(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:       "vm1",
		Type:     ResourceTypeVM,
		Name:     "low-cpu-vm",
		CPU:      &MetricValue{Current: 20.0},
		LastSeen: now,
	})
	store.Upsert(Resource{
		ID:       "vm2",
		Type:     ResourceTypeVM,
		Name:     "high-cpu-vm",
		CPU:      &MetricValue{Current: 85.0},
		LastSeen: now,
	})
	store.Upsert(Resource{
		ID:       "node1",
		Type:     ResourceTypeNode,
		Name:     "busy-node",
		CPU:      &MetricValue{Current: 75.0},
		LastSeen: now,
	})

	// Get top 2 by CPU
	top := store.GetTopByCPU(2, nil)
	if len(top) != 2 {
		t.Fatalf("Expected 2 resources, got %d", len(top))
	}
	if top[0].Name != "high-cpu-vm" {
		t.Errorf("Expected high-cpu-vm first, got %s", top[0].Name)
	}
	if top[1].Name != "busy-node" {
		t.Errorf("Expected busy-node second, got %s", top[1].Name)
	}

	// Filter by type
	topVMs := store.GetTopByCPU(10, []ResourceType{ResourceTypeVM})
	if len(topVMs) != 2 {
		t.Errorf("Expected 2 VMs, got %d", len(topVMs))
	}
}

func TestGetRelated(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:        "node1",
		Type:      ResourceTypeNode,
		Name:      "parent-node",
		ClusterID: "cluster1",
		LastSeen:  now,
	})
	store.Upsert(Resource{
		ID:        "vm1",
		Type:      ResourceTypeVM,
		Name:      "child-vm-1",
		ParentID:  "node1",
		ClusterID: "cluster1",
		LastSeen:  now,
	})
	store.Upsert(Resource{
		ID:        "vm2",
		Type:      ResourceTypeVM,
		Name:      "child-vm-2",
		ParentID:  "node1",
		ClusterID: "cluster1",
		LastSeen:  now,
	})
	store.Upsert(Resource{
		ID:        "node2",
		Type:      ResourceTypeNode,
		Name:      "cluster-peer",
		ClusterID: "cluster1",
		LastSeen:  now,
	})

	// Get related resources for vm1
	related := store.GetRelated("vm1")

	// Should have parent
	if parent, ok := related["parent"]; !ok || len(parent) != 1 {
		t.Error("Expected 1 parent")
	}

	// Should have sibling (vm2)
	if siblings, ok := related["siblings"]; !ok || len(siblings) != 1 {
		t.Errorf("Expected 1 sibling, got %d", len(related["siblings"]))
	}

	// Should have cluster members
	if cluster, ok := related["cluster_members"]; !ok || len(cluster) != 3 {
		t.Errorf("Expected 3 cluster members, got %d", len(related["cluster_members"]))
	}
}

func TestGetResourceSummary(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Upsert(Resource{
		ID:           "node1",
		Type:         ResourceTypeNode,
		PlatformType: PlatformProxmoxPVE,
		Status:       StatusOnline,
		CPU:          &MetricValue{Current: 50},
		Memory:       &MetricValue{Current: 50}, // 50% usage
		LastSeen:     now,
	})
	store.Upsert(Resource{
		ID:           "vm1",
		Type:         ResourceTypeVM,
		PlatformType: PlatformProxmoxPVE,
		Status:       StatusRunning,
		CPU:          &MetricValue{Current: 70},
		Memory:       &MetricValue{Current: 50}, // 50% usage
		LastSeen:     now,
	})
	store.Upsert(Resource{
		ID:           "vm2",
		Type:         ResourceTypeVM,
		PlatformType: PlatformProxmoxPVE,
		Status:       StatusStopped,
		LastSeen:     now,
	})

	summary := store.GetResourceSummary()

	if summary.TotalResources != 3 {
		t.Errorf("Expected 3 total resources, got %d", summary.TotalResources)
	}
	if summary.Healthy != 2 {
		t.Errorf("Expected 2 healthy, got %d", summary.Healthy)
	}
	if summary.Offline != 1 {
		t.Errorf("Expected 1 offline, got %d", summary.Offline)
	}

	// Check per-type stats
	vmStats := summary.ByType[ResourceTypeVM]
	if vmStats.Count != 2 {
		t.Errorf("Expected 2 VMs, got %d", vmStats.Count)
	}
}

