package ai

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// --- filterStateByScope tests ---

func TestFilterStateByScope_NoScope(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
		VMs:   []models.VM{{ID: "vm1", Name: "vm-1"}},
	}
	scope := PatrolScope{} // no resource IDs or types

	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)

	if len(filtered.Nodes) != 1 {
		t.Errorf("expected 1 node with no scope filter, got %d", len(filtered.Nodes))
	}
	if len(filtered.VMs) != 1 {
		t.Errorf("expected 1 VM with no scope filter, got %d", len(filtered.VMs))
	}
}

func TestFilterStateByScope_ByResourceID(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1"},
			{ID: "n2", Name: "node-2"},
		},
	}
	scope := PatrolScope{
		ResourceIDs:   []string{"n1"},
		ResourceTypes: []string{"node"},
	}

	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)

	if len(filtered.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(filtered.Nodes))
	}
	if filtered.Nodes[0].ID != "n1" {
		t.Errorf("expected node n1, got %s", filtered.Nodes[0].ID)
	}
}

func TestFilterStateByScope_ByResourceName(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1"},
			{ID: "n2", Name: "node-2"},
		},
	}
	scope := PatrolScope{
		ResourceIDs:   []string{"node-1"},
		ResourceTypes: []string{"node"},
	}

	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)

	if len(filtered.Nodes) != 1 {
		t.Fatalf("expected 1 node matched by name, got %d", len(filtered.Nodes))
	}
	if filtered.Nodes[0].Name != "node-1" {
		t.Errorf("expected node-1, got %s", filtered.Nodes[0].Name)
	}
}

func TestFilterStateByScope_ByType_VM(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
		VMs:   []models.VM{{ID: "vm1", Name: "vm-1"}},
	}
	scope := PatrolScope{
		ResourceTypes: []string{"vm"},
	}

	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)

	if len(filtered.Nodes) != 0 {
		t.Errorf("expected 0 nodes when scoped to VM type, got %d", len(filtered.Nodes))
	}
	if len(filtered.VMs) != 1 {
		t.Errorf("expected 1 VM, got %d", len(filtered.VMs))
	}
}

func TestFilterStateByScope_TypeAliasesRejected(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		VMs:        []models.VM{{ID: "vm1", Name: "vm-1"}},
		Containers: []models.Container{{ID: "ct1", Name: "ct-1"}},
		Hosts:      []models.Host{{ID: "h1", Hostname: "host-1"}},
		PhysicalDisks: []models.PhysicalDisk{
			{ID: "disk-1", DevPath: "/dev/sda", Model: "sda"},
		},
		PBSInstances: []models.PBSInstance{
			{ID: "pbs1", Name: "pbs-main", Datastores: []models.PBSDatastore{{Name: "ds1"}}},
		},
		PMGInstances: []models.PMGInstance{
			{ID: "pmg1", Name: "pmg-main", Host: "pmg.local"},
		},
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k1", Name: "cluster-1"},
		},
		DockerHosts: []models.DockerHost{
			{ID: "dh1", Hostname: "docker-host-1"},
		},
	}

	// Legacy alias should not match canonical VM resources.
	scope := PatrolScope{ResourceTypes: []string{"qemu"}}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.VMs) != 0 {
		t.Errorf("expected legacy 'qemu' alias to be rejected, got %d VMs", len(filtered.VMs))
	}

	// Legacy alias should not match canonical system-container resources.
	scope = PatrolScope{ResourceTypes: []string{"container"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Containers) != 0 {
		t.Errorf("expected legacy 'container' alias to be rejected, got %d containers", len(filtered.Containers))
	}

	// Removed host alias should not match canonical agent resources.
	scope = PatrolScope{ResourceTypes: []string{"host"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Hosts) != 0 {
		t.Errorf("expected legacy 'host' alias to be rejected, got %d hosts", len(filtered.Hosts))
	}

	// Canonical v6 type should match containers.
	scope = PatrolScope{ResourceTypes: []string{"system-container"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Containers) != 1 {
		t.Errorf("expected 'system-container' to match LXC containers, got %d", len(filtered.Containers))
	}

	// Underscore alias should be rejected; hyphenated canonical type is required.
	scope = PatrolScope{ResourceTypes: []string{"system_container"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Containers) != 0 {
		t.Errorf("expected legacy 'system_container' alias to be rejected, got %d containers", len(filtered.Containers))
	}

	// Additional removed aliases should also be rejected.
	scope = PatrolScope{ResourceTypes: []string{"docker_container"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.DockerHosts) != 0 {
		t.Errorf("expected legacy 'docker_container' alias to be rejected, got %d docker hosts", len(filtered.DockerHosts))
	}

	scope = PatrolScope{ResourceTypes: []string{"kubernetes_cluster"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.KubernetesClusters) != 0 {
		t.Errorf("expected legacy 'kubernetes_cluster' alias to be rejected, got %d kubernetes clusters", len(filtered.KubernetesClusters))
	}

	scope = PatrolScope{ResourceTypes: []string{"docker"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.DockerHosts) != 0 {
		t.Errorf("expected non-canonical 'docker' alias to be rejected, got %d docker hosts", len(filtered.DockerHosts))
	}

	scope = PatrolScope{ResourceTypes: []string{"kubernetes"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.KubernetesClusters) != 0 {
		t.Errorf("expected non-canonical 'kubernetes' alias to be rejected, got %d kubernetes clusters", len(filtered.KubernetesClusters))
	}

	scope = PatrolScope{ResourceTypes: []string{"pbs_datastore"}, ResourceIDs: []string{"pbs1:ds1"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.PBSInstances) != 0 {
		t.Errorf("expected non-canonical 'pbs_datastore' alias to be rejected, got %d pbs instances", len(filtered.PBSInstances))
	}

	scope = PatrolScope{ResourceTypes: []string{"agent_raid"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Hosts) != 0 {
		t.Errorf("expected non-canonical 'agent_raid' alias to be rejected, got %d hosts", len(filtered.Hosts))
	}

	scope = PatrolScope{ResourceTypes: []string{"pmg"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.PMGInstances) != 1 {
		t.Errorf("expected canonical 'pmg' to match PMG instances, got %d", len(filtered.PMGInstances))
	}

	scope = PatrolScope{ResourceTypes: []string{"physical_disk"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.PhysicalDisks) != 1 {
		t.Errorf("expected canonical 'physical_disk' to match physical disks, got %d", len(filtered.PhysicalDisks))
	}
}

func TestFilterStateByScope_AppContainerAlias(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "dh1",
				Hostname: "docker-host-1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "web"},
				},
			},
		},
	}

	// Canonical semantic name should match Docker resources.
	scope := PatrolScope{ResourceTypes: []string{"app-container"}}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.DockerHosts) != 1 {
		t.Errorf("expected 'app-container' to match Docker hosts, got %d", len(filtered.DockerHosts))
	}

	// Underscore alias should be rejected.
	scope = PatrolScope{ResourceTypes: []string{"app_container"}}
	filtered = ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.DockerHosts) != 0 {
		t.Errorf("expected legacy 'app_container' alias to be rejected, got %d docker hosts", len(filtered.DockerHosts))
	}
}

func TestFilterStateByScope_DockerHost(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "dh1",
				Hostname: "docker-host-1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "web"},
					{ID: "c2", Name: "db"},
				},
			},
		},
	}

	// Match by host ID
	scope := PatrolScope{
		ResourceIDs:   []string{"dh1"},
		ResourceTypes: []string{"docker-host"},
	}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.DockerHosts) != 1 {
		t.Fatalf("expected 1 docker host, got %d", len(filtered.DockerHosts))
	}
	// When the host itself matches, all containers should be included
	if len(filtered.DockerHosts[0].Containers) != 2 {
		t.Errorf("expected all 2 containers when host matches, got %d", len(filtered.DockerHosts[0].Containers))
	}
}

func TestFilterStateByScope_DockerContainerOnly(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "dh1",
				Hostname: "docker-host-1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "web"},
					{ID: "c2", Name: "db"},
				},
			},
		},
	}

	// Match by container ID only
	scope := PatrolScope{
		ResourceIDs:   []string{"c1"},
		ResourceTypes: []string{"app-container"},
	}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.DockerHosts) != 1 {
		t.Fatalf("expected 1 docker host, got %d", len(filtered.DockerHosts))
	}
	if len(filtered.DockerHosts[0].Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(filtered.DockerHosts[0].Containers))
	}
	if filtered.DockerHosts[0].Containers[0].ID != "c1" {
		t.Errorf("expected container c1, got %s", filtered.DockerHosts[0].Containers[0].ID)
	}
}

func TestFilterStateByScope_Storage(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{
			{ID: "s1", Name: "local"},
			{ID: "s2", Name: "ceph"},
		},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"s1"},
		ResourceTypes: []string{"storage"},
	}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Storage) != 1 {
		t.Fatalf("expected 1 storage, got %d", len(filtered.Storage))
	}
	if filtered.Storage[0].ID != "s1" {
		t.Errorf("expected storage s1, got %s", filtered.Storage[0].ID)
	}
}

func TestFilterStateByScope_Hosts(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "h1", Hostname: "host-1"},
			{ID: "h2", Hostname: "host-2"},
		},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"h1"},
		ResourceTypes: []string{"agent"},
	}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(filtered.Hosts))
	}
	if filtered.Hosts[0].ID != "h1" {
		t.Errorf("expected host h1, got %s", filtered.Hosts[0].ID)
	}
}

func TestFilterStateByScope_PBS(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{
			{
				ID:   "pbs1",
				Name: "pbs-main",
				Datastores: []models.PBSDatastore{
					{Name: "ds1"},
				},
			},
			{
				ID:   "pbs2",
				Name: "pbs-secondary",
			},
		},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"pbs1"},
		ResourceTypes: []string{"pbs"},
	}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance, got %d", len(filtered.PBSInstances))
	}
	if filtered.PBSInstances[0].ID != "pbs1" {
		t.Errorf("expected PBS pbs1, got %s", filtered.PBSInstances[0].ID)
	}
}

func TestFilterStateByScope_Kubernetes(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k1", Name: "cluster-1"},
			{ID: "k2", Name: "cluster-2"},
		},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"k1"},
		ResourceTypes: []string{"k8s-cluster"},
	}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.KubernetesClusters) != 1 {
		t.Fatalf("expected 1 k8s cluster, got %d", len(filtered.KubernetesClusters))
	}
	if filtered.KubernetesClusters[0].ID != "k1" {
		t.Errorf("expected cluster k1, got %s", filtered.KubernetesClusters[0].ID)
	}
}

func TestFilterStateByScope_PreservesScopedMetadataOnly(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	now := time.Now()
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1"},
		},
		VMs: []models.VM{
			{ID: "vm-101", VMID: 101, Name: "vm-101"},
		},
		LastUpdate: now,
		ConnectionHealth: map[string]bool{
			"node-1": true,
			"node-2": false,
		},
		ActiveAlerts: []models.Alert{
			{ID: "a1", ResourceID: "node-1", Message: "scoped"},
			{ID: "a2", ResourceID: "node-2", Message: "global"},
		},
		RecentlyResolved: []models.ResolvedAlert{
			{Alert: models.Alert{ID: "r1", ResourceID: "node-1", Message: "resolved scoped"}},
			{Alert: models.Alert{ID: "r2", ResourceID: "node-2", Message: "resolved global"}},
		},
		PVEBackups: models.PVEBackups{
			BackupTasks: []models.BackupTask{
				{ID: "bt-101", VMID: 101},
				{ID: "bt-202", VMID: 202},
			},
			StorageBackups: []models.StorageBackup{
				{ID: "sb-101", VMID: 101},
				{ID: "sb-202", VMID: 202},
			},
			GuestSnapshots: []models.GuestSnapshot{
				{ID: "gs-101", VMID: 101},
				{ID: "gs-202", VMID: 202},
			},
		},
		PBSBackups: []models.PBSBackup{
			{ID: "pb-101", VMID: "101"},
			{ID: "pb-202", VMID: "202"},
		},
	}
	scope := PatrolScope{ResourceTypes: []string{"node", "vm"}}

	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)

	if len(filtered.ConnectionHealth) != 1 {
		t.Fatalf("expected only scoped ConnectionHealth entries, got %d", len(filtered.ConnectionHealth))
	}
	if !filtered.ConnectionHealth["node-1"] {
		t.Fatal("expected node-1 connection health to be preserved")
	}
	if len(filtered.ActiveAlerts) != 1 || filtered.ActiveAlerts[0].ResourceID != "node-1" {
		t.Fatalf("expected only scoped active alerts, got %+v", filtered.ActiveAlerts)
	}
	if len(filtered.RecentlyResolved) != 1 || filtered.RecentlyResolved[0].ResourceID != "node-1" {
		t.Fatalf("expected only scoped resolved alerts, got %+v", filtered.RecentlyResolved)
	}
	if len(filtered.PVEBackups.BackupTasks) != 1 || filtered.PVEBackups.BackupTasks[0].VMID != 101 {
		t.Fatalf("expected only scoped PVE backup tasks, got %+v", filtered.PVEBackups.BackupTasks)
	}
	if len(filtered.PVEBackups.StorageBackups) != 1 || filtered.PVEBackups.StorageBackups[0].VMID != 101 {
		t.Fatalf("expected only scoped storage backups, got %+v", filtered.PVEBackups.StorageBackups)
	}
	if len(filtered.PVEBackups.GuestSnapshots) != 1 || filtered.PVEBackups.GuestSnapshots[0].VMID != 101 {
		t.Fatalf("expected only scoped guest snapshots, got %+v", filtered.PVEBackups.GuestSnapshots)
	}
	if len(filtered.PBSBackups) != 1 || filtered.PBSBackups[0].VMID != "101" {
		t.Fatalf("expected only scoped PBS backups, got %+v", filtered.PBSBackups)
	}
}

func TestFilterStateByScope_RebuildsScopedProviders(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	state := patrolRuntimeState{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1"},
			{ID: "n2", Name: "node-2"},
		},
		VMs: []models.VM{
			{ID: "vm1", Name: "vm-1"},
			{ID: "vm2", Name: "vm-2"},
		},
		Storage: []models.Storage{
			{ID: "s1", Name: "local", Usage: 42},
			{ID: "s2", Name: "backup", Usage: 10},
		},
		PhysicalDisks: []models.PhysicalDisk{
			{ID: "disk-1", DevPath: "/dev/sda", Model: "sda"},
			{ID: "disk-2", DevPath: "/dev/sdb", Model: "sdb"},
		},
		PMGInstances: []models.PMGInstance{
			{ID: "pmg1", Name: "pmg-main", Host: "pmg.local"},
			{ID: "pmg2", Name: "pmg-edge", Host: "pmg2.local"},
		},
	}

	filtered := ps.filterStateByScopeState(state, PatrolScope{
		ResourceIDs:   []string{"n1", "s1"},
		ResourceTypes: []string{"node", "storage"},
	})

	if filtered.readState == nil {
		t.Fatal("expected scoped readState to be rebuilt")
	}
	if filtered.unifiedResourceProvider == nil {
		t.Fatal("expected scoped unified resource provider to be rebuilt")
	}
	if got := len(filtered.readState.Nodes()); got != 1 {
		t.Fatalf("expected 1 scoped node in readState, got %d", got)
	}
	if got := len(filtered.readState.VMs()); got != 0 {
		t.Fatalf("expected 0 scoped VMs in readState, got %d", got)
	}
	if got := len(filtered.readState.StoragePools()); got != 1 {
		t.Fatalf("expected 1 scoped storage pool in readState, got %d", got)
	}
	if got := len(filtered.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypeStorage)); got != 1 {
		t.Fatalf("expected 1 scoped storage resource in provider, got %d", got)
	}
	if got := len(filtered.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypePhysicalDisk)); got != 0 {
		t.Fatalf("expected no scoped physical disk resources in provider, got %d", got)
	}

	filteredPMG := ps.filterStateByScopeState(state, PatrolScope{
		ResourceIDs:   []string{"pmg.local"},
		ResourceTypes: []string{"pmg"},
	})
	if got := len(filteredPMG.PMGInstances); got != 1 {
		t.Fatalf("expected 1 scoped PMG instance, got %d", got)
	}
	if filteredPMG.PMGInstances[0].ID != "pmg1" {
		t.Fatalf("expected scoped PMG instance pmg1, got %s", filteredPMG.PMGInstances[0].ID)
	}
	if filteredPMG.readState == nil {
		t.Fatal("expected scoped PMG readState to be rebuilt")
	}
	if got := len(filteredPMG.readState.PMGInstances()); got != 1 {
		t.Fatalf("expected 1 scoped PMG instance in readState, got %d", got)
	}
	if got := len(filteredPMG.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypePMG)); got != 1 {
		t.Fatalf("expected 1 scoped PMG resource in provider, got %d", got)
	}

	filteredDisks := ps.filterStateByScopeState(state, PatrolScope{
		ResourceIDs:   []string{"/dev/sda"},
		ResourceTypes: []string{"physical_disk"},
	})
	if got := len(filteredDisks.PhysicalDisks); got != 1 {
		t.Fatalf("expected 1 scoped physical disk, got %d", got)
	}
	if filteredDisks.PhysicalDisks[0].ID != "disk-1" {
		t.Fatalf("expected scoped physical disk disk-1, got %s", filteredDisks.PhysicalDisks[0].ID)
	}
	if got := len(filteredDisks.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypePhysicalDisk)); got != 1 {
		t.Fatalf("expected 1 scoped physical disk resource in provider, got %d", got)
	}

	counts := patrolRuntimeCountResources(filteredDisks)
	if counts.storage != 1 {
		t.Fatalf("expected scoped runtime counts to include 1 storage resource, got %#v", counts)
	}
	if counts.total() != 1 {
		t.Fatalf("expected scoped runtime total to be 1, got %#v", counts)
	}
}

func TestPatrolRuntimeResourceHelpers_PrepareCanonicalInventory(t *testing.T) {
	nodeView := unifiedresources.NewNodeView(&unifiedresources.Resource{
		ID:   "node-1",
		Name: "node-a",
		Type: unifiedresources.ResourceTypeAgent,
	})
	vmView := newTestVMView("qemu/100", "vm-a", 100, "pve1", "", unifiedresources.StatusOnline, false, nil)
	ctView := newTestContainerView("lxc/200", "ct-a", 200, "pve1", "", unifiedresources.StatusOnline, false, nil)
	storageView := unifiedresources.NewStoragePoolView(&unifiedresources.Resource{
		ID:   "storage-1",
		Name: "local-zfs",
		Type: unifiedresources.ResourceTypeStorage,
	})
	dockerHostView := unifiedresources.NewDockerHostView(&unifiedresources.Resource{
		ID:   "docker-1",
		Name: "docker-a",
		Type: unifiedresources.ResourceTypeAgent,
		Docker: &unifiedresources.DockerData{
			Hostname: "docker.local",
		},
	})
	dockerCtrView := unifiedresources.NewDockerContainerView(&unifiedresources.Resource{
		ID:   "ctr-1",
		Name: "web",
		Type: unifiedresources.ResourceTypeAppContainer,
	})
	hostView := unifiedresources.NewHostView(&unifiedresources.Resource{
		ID:   "agent-1",
		Name: "agent-a",
		Type: unifiedresources.ResourceTypeAgent,
		Agent: &unifiedresources.AgentData{
			Hostname: "agent.local",
		},
	})
	pbsView := unifiedresources.NewPBSInstanceView(&unifiedresources.Resource{
		ID:   "pbs-1",
		Name: "pbs-a",
		Type: unifiedresources.ResourceTypePBS,
	})
	pmgView := unifiedresources.NewPMGInstanceView(&unifiedresources.Resource{
		ID:   "pmg-1",
		Name: "pmg-a",
		Type: unifiedresources.ResourceTypePMG,
	})
	k8sView := unifiedresources.NewK8sClusterView(&unifiedresources.Resource{
		ID:   "k8s-1",
		Name: "cluster-a",
		Type: unifiedresources.ResourceTypeK8sCluster,
	})

	state := patrolRuntimeState{
		readState: &mockReadState{
			nodes:       []*unifiedresources.NodeView{&nodeView},
			vms:         []*unifiedresources.VMView{vmView},
			containers:  []*unifiedresources.ContainerView{ctView},
			hosts:       []*unifiedresources.HostView{&hostView},
			dockerHosts: []*unifiedresources.DockerHostView{&dockerHostView},
			dockerCtrs:  []*unifiedresources.DockerContainerView{&dockerCtrView},
			storage:     []*unifiedresources.StoragePoolView{&storageView},
			pbs:         []*unifiedresources.PBSInstanceView{&pbsView},
			pmg:         []*unifiedresources.PMGInstanceView{&pmgView},
			k8sClusters: []*unifiedresources.K8sClusterView{&k8sView},
		},
		unifiedResourceProvider: &mockUnifiedResourceProvider{
			getByTypeFunc: func(t unifiedresources.ResourceType) []unifiedresources.Resource {
				if t != unifiedresources.ResourceTypePhysicalDisk {
					return nil
				}
				return []unifiedresources.Resource{
					{
						ID:   "disk-1",
						Name: "disk-a",
						Type: unifiedresources.ResourceTypePhysicalDisk,
						PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
							DevPath: "/dev/sda",
							Model:   "Samsung SSD",
						},
					},
				}
			},
		},
	}

	counts := patrolRuntimeCountResources(state)
	if counts.nodes != 1 || counts.guests != 2 || counts.storage != 2 || counts.docker != 1 || counts.hosts != 1 || counts.pbs != 1 || counts.pmg != 1 || counts.kubernetes != 1 {
		t.Fatalf("unexpected canonical runtime counts: %#v", counts)
	}
	if counts.total() != 10 {
		t.Fatalf("expected canonical runtime total 10, got %#v", counts)
	}

	resourceIDs := patrolRuntimeSortedResourceIDs(state)
	for _, want := range []string{"node-1", "qemu/100", "lxc/200", "storage-1", "docker-1", "ctr-1", "agent-1", "pbs-1", "pmg-1", "k8s-1", "disk-1", "/dev/sda"} {
		found := false
		for _, got := range resourceIDs {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected runtime resource IDs to include %q, got %v", want, resourceIDs)
		}
	}

	known := patrolRuntimeKnownResources(state)
	for _, want := range []string{"node-a", "vm-a", "ct-a", "local-zfs", "docker.local", "web", "agent.local", "cluster-a", "Samsung SSD"} {
		if !known[want] {
			t.Fatalf("expected known runtime resources to include %q", want)
		}
	}
}

func TestPatrolRuntimeState_WithDerivedProviders_UsesResourceInventoryOnly(t *testing.T) {
	state := patrolRuntimeState{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-a"},
		},
		ActiveAlerts: []models.Alert{
			{ResourceID: "node-1"},
		},
		ConnectionHealth: map[string]bool{
			"node-1": true,
		},
		PVEBackups: models.PVEBackups{
			BackupTasks: []models.BackupTask{{ID: "bt-1", VMID: 101}},
		},
		PBSBackups: []models.PBSBackup{{ID: "pb-1", VMID: "101"}},
	}

	derived := state.withDerivedProviders()
	if derived.readState == nil || derived.unifiedResourceProvider == nil {
		t.Fatal("expected derived providers to be rebuilt")
	}
	if got := len(derived.readState.Nodes()); got != 1 {
		t.Fatalf("expected 1 node in derived readState, got %d", got)
	}
	if got := len(derived.readState.VMs()); got != 0 {
		t.Fatalf("expected no VM inventory from metadata-only fields, got %d", got)
	}
	if len(derived.ActiveAlerts) != 1 || derived.ActiveAlerts[0].ResourceID != "node-1" {
		t.Fatalf("expected active alerts to remain on patrol runtime state, got %+v", derived.ActiveAlerts)
	}
	if !derived.ConnectionHealth["node-1"] {
		t.Fatalf("expected connection health to remain on patrol runtime state, got %+v", derived.ConnectionHealth)
	}
	if len(derived.PVEBackups.BackupTasks) != 1 || len(derived.PBSBackups) != 1 {
		t.Fatalf("expected backup metadata to remain on patrol runtime state, got %+v / %+v", derived.PVEBackups.BackupTasks, derived.PBSBackups)
	}
}

func TestFilterStateByScope_WhitespaceInIDs(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
	}

	// IDs with whitespace should be trimmed
	scope := PatrolScope{
		ResourceIDs:   []string{"  n1  "},
		ResourceTypes: []string{"node"},
	}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Nodes) != 1 {
		t.Errorf("expected whitespace-trimmed ID to match, got %d nodes", len(filtered.Nodes))
	}
}

func TestFilterStateByScope_EmptyIDsIgnored(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"", "  ", "n1"},
		ResourceTypes: []string{"node"},
	}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Nodes) != 1 {
		t.Errorf("expected empty IDs to be ignored, got %d nodes", len(filtered.Nodes))
	}
}

func TestFilterStateByScope_CaseInsensitiveTypes(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
	}

	scope := PatrolScope{
		ResourceTypes: []string{"NODE"},
	}
	filtered := ps.filterStateByScopeState(ps.patrolRuntimeStateForSnapshot(state), scope)
	if len(filtered.Nodes) != 1 {
		t.Errorf("expected case-insensitive type matching, got %d nodes", len(filtered.Nodes))
	}
}

// --- tryStartRun / endRun tests ---

func TestTryStartRun_Basic(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	if !ps.tryStartRun("full") {
		t.Error("expected first tryStartRun to succeed")
	}
	if ps.tryStartRun("full") {
		t.Error("expected second tryStartRun to fail (run in progress)")
	}

	ps.endRun()

	if !ps.tryStartRun("full") {
		t.Error("expected tryStartRun to succeed after endRun")
	}
	ps.endRun()
}

func TestTryStartRun_Concurrent(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	var wg sync.WaitGroup
	var successes int32
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ps.tryStartRun("scoped") {
				mu.Lock()
				successes++
				mu.Unlock()
				// Simulate some work
				time.Sleep(1 * time.Millisecond)
				ps.endRun()
			}
		}()
	}
	wg.Wait()

	if successes == 0 {
		t.Error("expected at least one goroutine to acquire the run")
	}
	// After all goroutines complete, run should not be in progress
	ps.mu.RLock()
	inProgress := ps.runInProgress
	ps.mu.RUnlock()
	if inProgress {
		t.Error("expected runInProgress to be false after all goroutines complete")
	}
}

// --- Subscribe / Unsubscribe ---

func TestSubscribeUnsubscribe(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ch1 := ps.SubscribeToStream()
	ch2 := ps.SubscribeToStream()

	ps.streamMu.RLock()
	count := len(ps.streamSubscribers)
	ps.streamMu.RUnlock()

	if count != 2 {
		t.Errorf("expected 2 subscribers, got %d", count)
	}

	ps.UnsubscribeFromStream(ch1)

	ps.streamMu.RLock()
	count = len(ps.streamSubscribers)
	ps.streamMu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", count)
	}

	ps.UnsubscribeFromStream(ch2)

	ps.streamMu.RLock()
	count = len(ps.streamSubscribers)
	ps.streamMu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 subscribers after all unsubscribe, got %d", count)
	}
}

func TestSubscribeToStream_ReceivesCurrentState(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Set active streaming state
	ps.streamMu.Lock()
	ps.streamPhase = "analyzing"
	ps.currentOutput.WriteString("some output")
	ps.streamMu.Unlock()

	ch := ps.SubscribeToStream()
	defer ps.UnsubscribeFromStream(ch)

	// New subscriber should receive a snapshot of the current state.
	select {
	case event := <-ch:
		if event.Type != "snapshot" {
			t.Errorf("expected snapshot event first, got %s", event.Type)
		}
		if event.Phase != "analyzing" {
			t.Errorf("expected phase 'analyzing', got %q", event.Phase)
		}
		if event.Content != "some output" {
			t.Errorf("expected 'some output', got %q", event.Content)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive snapshot event on subscribe")
	}
}

// --- broadcast ---

func TestBroadcast_MultipleSubscribers(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ch1 := ps.SubscribeToStream()
	ch2 := ps.SubscribeToStream()
	ch3 := ps.SubscribeToStream()

	event := PatrolStreamEvent{Type: "test", Content: "hello"}
	ps.broadcast(event)

	for i, ch := range []chan PatrolStreamEvent{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			if received.Content != "hello" {
				t.Errorf("subscriber %d: expected 'hello', got %q", i, received.Content)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d: timed out waiting for event", i)
		}
	}

	ps.UnsubscribeFromStream(ch1)
	ps.UnsubscribeFromStream(ch2)
	ps.UnsubscribeFromStream(ch3)
}

func TestBroadcast_StaleSubscriberRemoved(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Create a subscriber with a full buffer
	ch := ps.SubscribeToStream()

	// Fill the buffer (100 capacity)
	for i := 0; i < 100; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "fill", Content: "x"})
	}

	// Keep broadcasting without reading; subscriber should eventually be dropped
	// after repeated full-buffer backpressure.
	for i := 0; i < 30; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "overflow"})
	}

	// Give goroutine time to close the channel
	time.Sleep(50 * time.Millisecond)

	ps.streamMu.RLock()
	_, exists := ps.streamSubscribers[ch]
	ps.streamMu.RUnlock()

	if exists {
		t.Error("expected stale subscriber to be removed")
	}
}

func TestSubscribeToStreamFrom_ReplaysBufferedEvents(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-1")

	// No subscribers: broadcast should still buffer events for replay.
	ps.broadcast(PatrolStreamEvent{Type: "phase", Phase: "analyzing"})
	ps.broadcast(PatrolStreamEvent{Type: "content", Content: "hello"})
	ps.broadcast(PatrolStreamEvent{Type: "tool_start", ToolName: "uptime"})

	ps.streamMu.RLock()
	lastSeq := ps.streamSeq
	ps.streamMu.RUnlock()
	if lastSeq < 3 {
		t.Fatalf("expected seq to be >= 3, got %d", lastSeq)
	}

	// Subscribe from seq 1; should replay seq 2 and 3.
	ch := ps.SubscribeToStreamFrom(1)
	defer ps.UnsubscribeFromStream(ch)

	// First replayed event: content
	select {
	case ev := <-ch:
		if ev.Type != "content" {
			t.Fatalf("expected content replay, got %s", ev.Type)
		}
		if ev.Content != "hello" {
			t.Fatalf("expected hello content, got %q", ev.Content)
		}
		if ev.Seq <= 1 {
			t.Fatalf("expected seq > 1, got %d", ev.Seq)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for replayed content event")
	}

	// Second replayed event: tool_start
	select {
	case ev := <-ch:
		if ev.Type != "tool_start" {
			t.Fatalf("expected tool_start replay, got %s", ev.Type)
		}
		if ev.ToolName != "uptime" {
			t.Fatalf("expected tool uptime, got %q", ev.ToolName)
		}
		if ev.Seq <= 1 {
			t.Fatalf("expected seq > 1, got %d", ev.Seq)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for replayed tool_start event")
	}
}

func TestSubscribeToStreamFrom_BufferRotatedSnapshot(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-1")
	ps.setStreamPhase("analyzing")

	// Fill beyond replay buffer to force rotation.
	for i := 0; i < 260; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "content", Content: "x"})
	}

	ps.streamMu.RLock()
	start, end := ps.streamBufferWindowLocked()
	ps.streamMu.RUnlock()
	if start == 0 || end == 0 || end <= start {
		t.Fatalf("expected non-empty buffer window, got start=%d end=%d", start, end)
	}

	// Ask to resume from a seq that is behind the buffered window.
	ch := ps.SubscribeToStreamFrom(start - 1)
	defer ps.UnsubscribeFromStream(ch)

	select {
	case ev := <-ch:
		if ev.Type != "snapshot" {
			t.Fatalf("expected snapshot, got %s", ev.Type)
		}
		if ev.ResyncReason != "buffer_rotated" {
			t.Fatalf("expected resync_reason buffer_rotated, got %q", ev.ResyncReason)
		}
		if ev.BufferStart == 0 || ev.BufferEnd == 0 || ev.BufferEnd < ev.BufferStart {
			t.Fatalf("expected buffer window in snapshot, got start=%d end=%d", ev.BufferStart, ev.BufferEnd)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for buffer_rotated snapshot")
	}
}

func TestSubscribeToStreamFrom_BufferRotatedSnapshotNotDuplicated(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-1")
	ps.setStreamPhase("analyzing")

	for i := 0; i < 260; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "content", Content: "x"})
	}

	ps.streamMu.RLock()
	start, _ := ps.streamBufferWindowLocked()
	ps.streamMu.RUnlock()

	ch := ps.SubscribeToStreamFrom(start - 1)
	defer ps.UnsubscribeFromStream(ch)

	snapshotCount := 0
	read := 0
	for read < 25 {
		select {
		case ev := <-ch:
			read++
			if ev.Type == "snapshot" {
				snapshotCount++
			}
		case <-time.After(100 * time.Millisecond):
			read = 25
		}
	}

	if snapshotCount != 1 {
		t.Fatalf("expected exactly 1 snapshot for buffer_rotated resume, got %d", snapshotCount)
	}
}

func TestSubscribeToStreamFrom_ChurnDoesNotLeakSubscribers(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	const workers = 12
	const loopsPerWorker = 60

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < loopsPerWorker; i++ {
				ch := ps.SubscribeToStreamFrom(0)
				ps.broadcast(PatrolStreamEvent{Type: "content", Content: "x"})
				ps.UnsubscribeFromStream(ch)
			}
		}()
	}
	wg.Wait()

	// Allow concurrent close/remove paths to settle.
	time.Sleep(25 * time.Millisecond)

	ps.streamMu.RLock()
	subs := len(ps.streamSubscribers)
	ps.streamMu.RUnlock()
	if subs != 0 {
		t.Fatalf("expected no leaked subscribers after churn, got %d", subs)
	}

	// Sanity: broadcasting after heavy churn should still work.
	ps.broadcast(PatrolStreamEvent{Type: "content", Content: "ok"})
}

func TestSubscribeToStreamFrom_RecordsReplayMetrics(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-metrics-replay")
	ps.setStreamPhase("analyzing")

	ps.broadcast(PatrolStreamEvent{Type: "phase", Phase: "analyzing"})
	ps.broadcast(PatrolStreamEvent{Type: "content", Content: "hello"})
	ps.broadcast(PatrolStreamEvent{Type: "tool_start", ToolName: "uptime"})

	m := GetPatrolMetrics()
	beforeOutcome := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("replay"))
	beforeEvents := testutil.ToFloat64(m.streamReplayEvents)

	ch := ps.SubscribeToStreamFrom(1)
	defer ps.UnsubscribeFromStream(ch)

	// Drain replayed events to avoid backpressure affecting subsequent tests.
	<-ch
	<-ch

	afterOutcome := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("replay"))
	afterEvents := testutil.ToFloat64(m.streamReplayEvents)
	if afterOutcome <= beforeOutcome {
		t.Fatalf("expected replay outcome counter to increase (before=%f after=%f)", beforeOutcome, afterOutcome)
	}
	if afterEvents < beforeEvents+2 {
		t.Fatalf("expected replay event counter to increase by at least 2 (before=%f after=%f)", beforeEvents, afterEvents)
	}
}

func TestSubscribeToStreamFrom_RecordsSnapshotAndMissMetrics(t *testing.T) {
	// Snapshot path (buffer rotated)
	psSnap := NewPatrolService(nil, nil)
	psSnap.resetStreamForRun("run-metrics-snapshot")
	psSnap.setStreamPhase("analyzing")
	for i := 0; i < 260; i++ {
		psSnap.broadcast(PatrolStreamEvent{Type: "content", Content: "x"})
	}
	psSnap.streamMu.RLock()
	start, _ := psSnap.streamBufferWindowLocked()
	psSnap.streamMu.RUnlock()

	m := GetPatrolMetrics()
	beforeSnapshotOutcome := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("snapshot"))
	beforeReason := testutil.ToFloat64(m.streamResyncReason.WithLabelValues("buffer_rotated"))

	chSnap := psSnap.SubscribeToStreamFrom(start - 1)
	defer psSnap.UnsubscribeFromStream(chSnap)
	<-chSnap // consume snapshot

	afterSnapshotOutcome := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("snapshot"))
	afterReason := testutil.ToFloat64(m.streamResyncReason.WithLabelValues("buffer_rotated"))
	if afterSnapshotOutcome <= beforeSnapshotOutcome {
		t.Fatalf("expected snapshot outcome counter to increase (before=%f after=%f)", beforeSnapshotOutcome, afterSnapshotOutcome)
	}
	if afterReason <= beforeReason {
		t.Fatalf("expected buffer_rotated reason counter to increase (before=%f after=%f)", beforeReason, afterReason)
	}

	// Miss path (resume requested while stream idle and no buffered events)
	psMiss := NewPatrolService(nil, nil)
	psMiss.resetStreamForRun("run-metrics-miss")

	beforeMiss := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("miss"))
	chMiss := psMiss.SubscribeToStreamFrom(42)
	defer psMiss.UnsubscribeFromStream(chMiss)
	afterMiss := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("miss"))
	if afterMiss <= beforeMiss {
		t.Fatalf("expected miss outcome counter to increase (before=%f after=%f)", beforeMiss, afterMiss)
	}
}

func TestBroadcast_RecordsBackpressureDropMetric(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ch := ps.SubscribeToStream()
	defer ps.UnsubscribeFromStream(ch)

	m := GetPatrolMetrics()
	beforeDrop := testutil.ToFloat64(m.streamSubscriberDrop.WithLabelValues("backpressure"))

	for i := 0; i < 100; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "fill", Content: "x"})
	}
	for i := 0; i < 30; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "overflow"})
	}

	time.Sleep(25 * time.Millisecond)
	afterDrop := testutil.ToFloat64(m.streamSubscriberDrop.WithLabelValues("backpressure"))
	if afterDrop <= beforeDrop {
		t.Fatalf("expected backpressure drop metric to increase (before=%f after=%f)", beforeDrop, afterDrop)
	}
}

func TestStreamOutput_TruncatesTailAndSnapshotMarksIt(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-1")
	ps.setStreamPhase("analyzing")

	// Write more than the tail buffer can hold.
	payload := strings.Repeat("a", patrolStreamMaxOutputBytes+123)
	ps.appendStreamContent(payload)

	out, phase := ps.GetCurrentStreamOutput()
	if phase != "analyzing" {
		t.Fatalf("expected phase analyzing, got %q", phase)
	}
	if len(out) != patrolStreamMaxOutputBytes {
		t.Fatalf("expected output len %d, got %d", patrolStreamMaxOutputBytes, len(out))
	}
	expectedTail := payload[len(payload)-patrolStreamMaxOutputBytes:]
	if out != expectedTail {
		t.Fatalf("expected tail output to match last %d bytes", patrolStreamMaxOutputBytes)
	}

	ch := ps.SubscribeToStreamFrom(0)
	defer ps.UnsubscribeFromStream(ch)
	select {
	case ev := <-ch:
		if ev.Type != "snapshot" {
			t.Fatalf("expected snapshot, got %s", ev.Type)
		}
		if ev.ContentTruncated == nil || !*ev.ContentTruncated {
			t.Fatalf("expected snapshot content_truncated to be true")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for snapshot")
	}
}

// --- generateFindingID ---

func TestGenerateFindingID_Deterministic(t *testing.T) {
	id1 := generateFindingID("res-1", "performance", "high-cpu")
	id2 := generateFindingID("res-1", "performance", "high-cpu")

	if id1 != id2 {
		t.Errorf("expected deterministic ID, got %s and %s", id1, id2)
	}
}

func TestGenerateFindingID_Different(t *testing.T) {
	id1 := generateFindingID("res-1", "performance", "high-cpu")
	id2 := generateFindingID("res-2", "performance", "high-cpu")
	id3 := generateFindingID("res-1", "reliability", "high-cpu")
	id4 := generateFindingID("res-1", "performance", "high-mem")

	if id1 == id2 {
		t.Error("different resource should produce different ID")
	}
	if id1 == id3 {
		t.Error("different category should produce different ID")
	}
	if id1 == id4 {
		t.Error("different issue should produce different ID")
	}
}

func TestGenerateFindingID_Length(t *testing.T) {
	id := generateFindingID("res-1", "performance", "high-cpu")
	// sha256[:8] = 8 bytes = 16 hex chars
	if len(id) != 16 {
		t.Errorf("expected 16-char hex ID, got %d chars: %s", len(id), id)
	}
}

// --- joinParts (already tested in patrol_test.go, but verify here for completeness) ---

func TestJoinParts_Zero(t *testing.T) {
	if joinParts(nil) != "" {
		t.Error("expected empty string for nil")
	}
}

func TestJoinParts_Three(t *testing.T) {
	result := joinParts([]string{"a", "b", "c"})
	if result != "a, b, and c" {
		t.Errorf("expected 'a, b, and c', got %q", result)
	}
}

// --- GetStatus ---

func TestGetStatus_Running(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Simulate a run in progress
	ps.mu.Lock()
	ps.runInProgress = true
	ps.mu.Unlock()

	status := ps.GetStatus()
	if !status.Running {
		t.Error("expected Running to be true when runInProgress is true")
	}

	ps.mu.Lock()
	ps.runInProgress = false
	ps.mu.Unlock()

	status = ps.GetStatus()
	if status.Running {
		t.Error("expected Running to be false when runInProgress is false")
	}
}

func TestGetStatus_NextPatrolAt(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Set nextScheduledAt so nextPatrolAt is returned
	nextTime := time.Now().Add(30 * time.Minute)
	ps.mu.Lock()
	ps.nextScheduledAt = nextTime
	ps.mu.Unlock()

	status := ps.GetStatus()
	if status.NextPatrolAt == nil {
		t.Fatal("expected NextPatrolAt to be calculated")
	}

	if !status.NextPatrolAt.Equal(nextTime) {
		t.Errorf("expected NextPatrolAt %v, got %v", nextTime, *status.NextPatrolAt)
	}
}

func TestGetStatus_NextPatrolAt_IndependentOfLastPatrol(t *testing.T) {
	// nextScheduledAt should drive NextPatrolAt, not lastPatrol + interval.
	// This is the scenario that was previously broken: user changes interval
	// mid-cycle, lastPatrol is old, so lastPatrol+newInterval could be in the past.
	ps := NewPatrolService(nil, nil)

	// Simulate: patrol ran 45 min ago
	ps.mu.Lock()
	ps.lastPatrol = time.Now().Add(-45 * time.Minute)
	// But the ticker was just reset with a 15-min interval, so next fire is ~15 min from now
	expectedNext := time.Now().Add(15 * time.Minute)
	ps.nextScheduledAt = expectedNext
	ps.mu.Unlock()

	status := ps.GetStatus()
	if status.NextPatrolAt == nil {
		t.Fatal("expected NextPatrolAt to be set")
	}
	// NextPatrolAt must be the tracked nextScheduledAt (in the future),
	// NOT lastPatrol + interval (which would be 30 min in the past).
	if !status.NextPatrolAt.Equal(expectedNext) {
		t.Errorf("expected NextPatrolAt = %v, got %v", expectedNext, *status.NextPatrolAt)
	}
	if status.NextPatrolAt.Before(time.Now()) {
		t.Errorf("NextPatrolAt should be in the future, got %v", *status.NextPatrolAt)
	}
}

func TestGetStatus_NextPatrolAt_NilWhenNotScheduled(t *testing.T) {
	// Before the patrol loop starts, nextScheduledAt is zero — no NextPatrolAt should be returned.
	ps := NewPatrolService(nil, nil)

	status := ps.GetStatus()
	if status.NextPatrolAt != nil {
		t.Errorf("expected NextPatrolAt to be nil before patrol loop starts, got %v", *status.NextPatrolAt)
	}
}

func TestGetStatus_Disabled(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ps.SetConfig(PatrolConfig{Enabled: false})

	// Even if nextScheduledAt is set, disabled patrol should not report NextPatrolAt
	ps.mu.Lock()
	ps.nextScheduledAt = time.Now().Add(10 * time.Minute)
	ps.mu.Unlock()

	status := ps.GetStatus()
	if status.Enabled {
		t.Error("expected Enabled to be false")
	}
	if status.NextPatrolAt != nil {
		t.Error("expected NextPatrolAt to be nil when disabled")
	}
}

func TestGetStatus_BlockedReason(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ps.setBlockedReason("circuit breaker is open")

	status := ps.GetStatus()
	if status.BlockedReason != "circuit breaker is open" {
		t.Errorf("expected blocked reason, got %q", status.BlockedReason)
	}
	if status.BlockedAt == nil {
		t.Error("expected BlockedAt to be set")
	}

	ps.clearBlockedReason()
	status = ps.GetStatus()
	if status.BlockedReason != "" {
		t.Errorf("expected empty blocked reason after clear, got %q", status.BlockedReason)
	}
}

// --- appendStreamContent ---

func TestAppendStreamContent(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ch := ps.SubscribeToStream()
	defer ps.UnsubscribeFromStream(ch)

	ps.appendStreamContent("hello ")
	ps.appendStreamContent("world")

	output, _ := ps.GetCurrentStreamOutput()
	if output != "hello world" {
		t.Errorf("expected 'hello world', got %q", output)
	}

	// Should have broadcast 2 events
	for i := 0; i < 2; i++ {
		select {
		case event := <-ch:
			if event.Type != "content" {
				t.Errorf("expected content event, got %s", event.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("expected broadcast event %d", i)
		}
	}
}

// --- setStreamPhase ---

func TestSetStreamPhase_ResetClearsOutput(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ps.setStreamPhase("analyzing")
	ps.appendStreamContent("some data")

	output, phase := ps.GetCurrentStreamOutput()
	if phase != "analyzing" {
		t.Errorf("expected phase 'analyzing', got %q", phase)
	}
	if output != "some data" {
		t.Errorf("expected 'some data', got %q", output)
	}

	ps.setStreamPhase("idle")
	output, phase = ps.GetCurrentStreamOutput()
	if phase != "idle" {
		t.Errorf("expected phase 'idle', got %q", phase)
	}
	// Output is no longer cleared on idle; it is cleared when a new run starts.
}

// --- isResourceOnline (heuristic alert resolution) ---

func TestIsResourceOnline_Node(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", Status: "online"},
			{ID: "n2", Name: "node-2", Status: "offline"},
		},
	}

	alert := AlertInfo{ResourceID: "n1", ResourceType: "node"}
	if !ps.isResourceOnline(alert, state) {
		t.Error("expected node n1 to be online")
	}

	alert = AlertInfo{ResourceID: "n2", ResourceType: "node"}
	if ps.isResourceOnline(alert, state) {
		t.Error("expected node n2 to be offline")
	}
}

func TestIsResourceOnline_VM(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm1", Name: "vm-1", Status: "running"},
			{ID: "vm2", Name: "vm-2", Status: "stopped"},
		},
	}

	alert := AlertInfo{ResourceID: "vm1", ResourceType: "vm"}
	if !ps.isResourceOnline(alert, state) {
		t.Error("expected VM vm1 to be online")
	}

	alert = AlertInfo{ResourceID: "vm2", ResourceType: "vm"}
	if ps.isResourceOnline(alert, state) {
		t.Error("expected VM vm2 to be offline")
	}
}

func TestIsResourceOnline_SystemContainer(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Containers: []models.Container{
			{ID: "ct1", Name: "ct-1", Status: "running"},
		},
	}

	alert := AlertInfo{ResourceID: "ct1", ResourceType: "system-container"}
	if !ps.isResourceOnline(alert, state) {
		t.Error("expected container ct1 to be online")
	}
}

func TestIsResourceOnline_AppContainer(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				Containers: []models.DockerContainer{
					{ID: "dc1", Name: "web", State: "running"},
					{ID: "dc2", Name: "db", State: "exited"},
				},
			},
		},
	}

	alert := AlertInfo{ResourceID: "dc1", ResourceType: "app-container"}
	if !ps.isResourceOnline(alert, state) {
		t.Error("expected docker container dc1 to be online")
	}

	alert = AlertInfo{ResourceID: "dc2", ResourceType: "app-container"}
	if ps.isResourceOnline(alert, state) {
		t.Error("expected docker container dc2 to be offline")
	}
}

func TestIsResourceOnline_Agent(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "h1", Hostname: "host-1", DisplayName: "Host One", Status: "online"},
			{ID: "h2", Hostname: "host-2", Status: "offline"},
		},
	}

	alert := AlertInfo{ResourceID: "h1", ResourceType: "agent"}
	if !ps.isResourceOnline(alert, state) {
		t.Error("expected host h1 to be online")
	}

	alert = AlertInfo{ResourceID: "h2", ResourceType: "agent"}
	if ps.isResourceOnline(alert, state) {
		t.Error("expected host h2 to be offline")
	}
}

func TestIsResourceOnline_UnknownType(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{}

	alert := AlertInfo{ResourceID: "x", ResourceType: "unknown"}
	if ps.isResourceOnline(alert, state) {
		t.Error("expected unknown type to return false")
	}
}

// --- getCurrentMetricValue ---

func TestGetCurrentMetricValue_Node(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", CPU: 0.75, Memory: models.Memory{Usage: 60.0}},
		},
	}

	alert := AlertInfo{ResourceID: "n1", ResourceType: "node", Type: "cpu"}
	val := ps.getCurrentMetricValue(alert, state)
	if val != 75.0 {
		t.Errorf("expected CPU 75.0 (0.75 * 100), got %f", val)
	}

	alert.Type = "memory"
	val = ps.getCurrentMetricValue(alert, state)
	if val != 60.0 {
		t.Errorf("expected memory 60.0, got %f", val)
	}
}

func TestGetCurrentMetricValue_NotFound(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{}

	alert := AlertInfo{ResourceID: "missing", ResourceType: "node", Type: "cpu"}
	val := ps.getCurrentMetricValue(alert, state)
	if val != -1 {
		t.Errorf("expected -1 for not found, got %f", val)
	}
}

func TestGetCurrentMetricValue_AppContainer(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				Containers: []models.DockerContainer{
					{ID: "dc1", Name: "web", CPUPercent: 45.0, MemoryPercent: 30.0},
				},
			},
		},
	}

	alert := AlertInfo{ResourceID: "dc1", ResourceType: "app-container", Type: "cpu"}
	val := ps.getCurrentMetricValue(alert, state)
	if val != 45.0 {
		t.Errorf("expected docker CPU 45.0, got %f", val)
	}

	alert.Type = "memory"
	val = ps.getCurrentMetricValue(alert, state)
	if val != 30.0 {
		t.Errorf("expected docker memory 30.0, got %f", val)
	}
}

func TestGetCurrentMetricValue_Agent(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "h1", Hostname: "host-1", CPUUsage: 67.0, Memory: models.Memory{Usage: 54.0}},
		},
	}

	alert := AlertInfo{ResourceID: "h1", ResourceType: "agent", Type: "cpu"}
	val := ps.getCurrentMetricValue(alert, state)
	if val != 67.0 {
		t.Errorf("expected host CPU 67.0, got %f", val)
	}

	alert.Type = "memory"
	val = ps.getCurrentMetricValue(alert, state)
	if val != 54.0 {
		t.Errorf("expected host memory 54.0, got %f", val)
	}
}

func TestGetCurrentMetricValue_Storage(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{
			{ID: "s1", Name: "local", Usage: 72.5},
		},
	}

	alert := AlertInfo{ResourceID: "s1", ResourceType: "storage", Type: "usage"}
	val := ps.getCurrentMetricValue(alert, state)
	if val != 72.5 {
		t.Errorf("expected storage usage 72.5, got %f", val)
	}
}

func TestGetCurrentMetricValue_UsesReadStateWhenLegacySlicesEmpty(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := newPatrolRuntimeState(models.StateSnapshot{})
	nodeView := unifiedresources.NewNodeView(&unifiedresources.Resource{
		ID:     "n1",
		Name:   "node-1",
		Type:   unifiedresources.ResourceTypeAgent,
		Status: unifiedresources.StatusOnline,
		Proxmox: &unifiedresources.ProxmoxData{
			NodeName: "node-1",
		},
		Metrics: &unifiedresources.ResourceMetrics{
			CPU:    &unifiedresources.MetricValue{Percent: 42},
			Memory: &unifiedresources.MetricValue{Percent: 33},
		},
	})
	state.readState = &mockReadState{nodes: []*unifiedresources.NodeView{&nodeView}}

	val := ps.getCurrentMetricValueState(AlertInfo{ResourceID: "n1", ResourceType: "node", Type: "cpu"}, state)
	if val != 42.0 {
		t.Fatalf("expected readState-backed CPU 42.0, got %f", val)
	}

	val = ps.getCurrentMetricValueState(AlertInfo{ResourceID: "n1", ResourceType: "node", Type: "memory"}, state)
	if val != 33.0 {
		t.Fatalf("expected readState-backed memory 33.0, got %f", val)
	}
}

func TestIsResourceOnline_UsesReadStateWhenLegacySlicesEmpty(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := newPatrolRuntimeState(models.StateSnapshot{})
	hostView := unifiedresources.NewHostView(&unifiedresources.Resource{
		ID:     "h1",
		Name:   "host-1",
		Type:   unifiedresources.ResourceTypeAgent,
		Status: unifiedresources.StatusOnline,
		Agent: &unifiedresources.AgentData{
			Hostname: "host-1",
		},
	})
	state.readState = &mockReadState{hosts: []*unifiedresources.HostView{&hostView}}

	if !ps.isResourceOnlineState(AlertInfo{ResourceID: "h1", ResourceType: "agent", Type: "offline"}, state) {
		t.Fatal("expected host to be online via readState")
	}
}

func TestGetResourceCurrentState_UsesReadStateWhenLegacySlicesEmpty(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := newPatrolRuntimeState(models.StateSnapshot{})
	storageView := unifiedresources.NewStoragePoolView(&unifiedresources.Resource{
		ID:     "s1",
		Name:   "local",
		Type:   unifiedresources.ResourceTypeStorage,
		Status: unifiedresources.StatusOnline,
		Metrics: &unifiedresources.ResourceMetrics{
			Disk: &unifiedresources.MetricValue{Percent: 72.5},
		},
	})
	state.readState = &mockReadState{storage: []*unifiedresources.StoragePoolView{&storageView}}

	desc := ps.getResourceCurrentStateState(AlertInfo{ResourceID: "s1", ResourceType: "storage"}, state)
	if !strings.Contains(desc, "Storage 'local': 72.5% used") {
		t.Fatalf("expected readState-backed storage description, got %q", desc)
	}
}

func TestAlertResolutionHelpers_UseReadStateForAppContainer(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := newPatrolRuntimeState(models.StateSnapshot{})
	parentID := "docker-1"
	containerView := unifiedresources.NewDockerContainerView(&unifiedresources.Resource{
		ID:       "dc-1",
		Name:     "web",
		Type:     unifiedresources.ResourceTypeAppContainer,
		Status:   unifiedresources.StatusOnline,
		ParentID: &parentID,
		Docker: &unifiedresources.DockerData{
			ContainerState: "running",
		},
		Metrics: &unifiedresources.ResourceMetrics{
			CPU:    &unifiedresources.MetricValue{Percent: 12.3},
			Memory: &unifiedresources.MetricValue{Percent: 45.6},
		},
	})
	state.readState = &mockReadState{dockerCtrs: []*unifiedresources.DockerContainerView{&containerView}}

	alert := AlertInfo{ResourceID: "dc-1", ResourceName: "web", ResourceType: "app-container", Type: "cpu"}
	if got := ps.getCurrentMetricValueState(alert, state); got != 12.3 {
		t.Fatalf("expected readState-backed docker CPU 12.3, got %f", got)
	}
	if !ps.isResourceOnlineState(AlertInfo{ResourceID: "dc-1", ResourceName: "web", ResourceType: "app-container"}, state) {
		t.Fatal("expected docker container to be online via readState")
	}
	desc := ps.getResourceCurrentStateState(AlertInfo{ResourceID: "dc-1", ResourceName: "web", ResourceType: "app-container"}, state)
	if !strings.Contains(desc, "Docker container 'web': CPU 12.3%, Memory 45.6%, State: running") {
		t.Fatalf("expected readState-backed docker container description, got %q", desc)
	}
}

// --- shouldResolveAlert (heuristic-only, no AI) ---

func TestShouldResolveAlert_StorageUsageDropped(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{
			{ID: "s1", Name: "local", Usage: 70.0, Status: "active"},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "s1",
		ResourceType: "usage",
		Type:         "usage",
		Threshold:    85.0,
		Value:        88.0,
		StartTime:    time.Now().Add(-1 * time.Hour),
	}

	shouldResolve, reason := ps.shouldResolveAlert(nil, alert, state, nil)
	if !shouldResolve {
		t.Error("expected alert to be resolved (usage dropped below threshold)")
	}
	if reason == "" {
		t.Error("expected a reason string")
	}
}

func TestShouldResolveAlert_CPUDropped(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", CPU: 0.5, Memory: models.Memory{Usage: 40.0}},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "n1",
		ResourceType: "node",
		Type:         "cpu",
		Threshold:    90.0,
		Value:        95.0,
		StartTime:    time.Now().Add(-30 * time.Minute),
	}

	shouldResolve, _ := ps.shouldResolveAlert(nil, alert, state, nil)
	if !shouldResolve {
		t.Error("expected alert to be resolved (CPU dropped)")
	}
}

func TestShouldResolveAlert_OfflineNowOnline(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", Status: "online"},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "n1",
		ResourceType: "node",
		Type:         "offline",
		StartTime:    time.Now().Add(-1 * time.Hour),
	}

	shouldResolve, reason := ps.shouldResolveAlert(nil, alert, state, nil)
	if !shouldResolve {
		t.Error("expected offline alert to be resolved (resource now online)")
	}
	if reason != "resource is now online/running" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestShouldResolveAlert_NoMatch(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{} // resource not in state

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "n1",
		ResourceType: "node",
		Type:         "cpu",
		Threshold:    90.0,
		Value:        95.0,
		StartTime:    time.Now().Add(-30 * time.Minute),
	}

	shouldResolve, _ := ps.shouldResolveAlert(nil, alert, state, nil)
	if shouldResolve {
		t.Error("expected alert NOT to be resolved when resource not found")
	}
}

func TestShouldResolveAlert_StorageStillHigh(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{
			{ID: "s1", Name: "local", Usage: 90.0},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "s1",
		ResourceType: "usage",
		Type:         "usage",
		Threshold:    85.0,
		Value:        88.0,
		StartTime:    time.Now().Add(-1 * time.Hour),
	}

	shouldResolve, _ := ps.shouldResolveAlert(nil, alert, state, nil)
	if shouldResolve {
		t.Error("expected alert NOT to be resolved (storage usage still above threshold)")
	}
}

// Regression test: CPU at 95% (0.95 raw) must NOT auto-resolve a 90% threshold alert.
// Before the fix, getCurrentMetricValue returned 0.95 (raw fraction) which was always
// below the 90.0 percentage threshold, causing every CPU alert to incorrectly auto-resolve.
func TestShouldResolveAlert_CPUScaleRegression(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", CPU: 0.95, Memory: models.Memory{Usage: 40.0}},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "n1",
		ResourceType: "node",
		Type:         "cpu",
		Threshold:    90.0,
		Value:        95.0,
		StartTime:    time.Now().Add(-30 * time.Minute),
	}

	shouldResolve, _ := ps.shouldResolveAlert(nil, alert, state, nil)
	if shouldResolve {
		t.Error("expected CPU alert NOT to be resolved (95% is still above 90% threshold)")
	}
}

func TestGetCurrentMetricValue_CPUScalePercent(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Node CPU
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", CPU: 0.42}},
	}
	val := ps.getCurrentMetricValue(AlertInfo{ResourceID: "n1", ResourceType: "node", Type: "cpu"}, state)
	if val != 42.0 {
		t.Errorf("node CPU: expected 42.0, got %f", val)
	}

	// VM CPU
	state = models.StateSnapshot{
		VMs: []models.VM{{ID: "vm1", CPU: 0.88}},
	}
	val = ps.getCurrentMetricValue(AlertInfo{ResourceID: "vm1", ResourceType: "vm", Type: "cpu"}, state)
	if val != 88.0 {
		t.Errorf("VM CPU: expected 88.0, got %f", val)
	}

	// Container CPU
	state = models.StateSnapshot{
		Containers: []models.Container{{ID: "ct1", CPU: 0.15}},
	}
	val = ps.getCurrentMetricValue(AlertInfo{ResourceID: "ct1", ResourceType: "system-container", Type: "cpu"}, state)
	if val != 15.0 {
		t.Errorf("Container CPU: expected 15.0, got %f", val)
	}
}

// --- reviewAndResolveAlerts ---

func TestReviewAndResolveAlerts_NilResolver(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{}

	result := ps.reviewAndResolveAlerts(nil, state, true)
	if result != 0 {
		t.Errorf("expected 0 resolved with nil resolver, got %d", result)
	}
}

func TestReviewAndResolveAlerts_NoActiveAlerts(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &mockAlertResolver{alerts: nil}
	ps.mu.Lock()
	ps.alertResolver = resolver
	ps.mu.Unlock()

	state := models.StateSnapshot{}
	result := ps.reviewAndResolveAlerts(nil, state, true)
	if result != 0 {
		t.Errorf("expected 0 resolved with no alerts, got %d", result)
	}
}

func TestReviewAndResolveAlerts_SkipsRecentAlerts(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &mockAlertResolver{
		alerts: []AlertInfo{
			{
				ID:           "a1",
				ResourceID:   "n1",
				ResourceType: "node",
				Type:         "offline",
				StartTime:    time.Now().Add(-5 * time.Minute), // only 5 minutes old
			},
		},
	}
	ps.mu.Lock()
	ps.alertResolver = resolver
	ps.mu.Unlock()

	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1", Status: "online"}},
	}

	result := ps.reviewAndResolveAlerts(nil, state, true)
	if result != 0 {
		t.Errorf("expected 0 resolved (alert too recent), got %d", result)
	}
}

// --- mockAlertResolver ---

type mockAlertResolver struct {
	alerts   []AlertInfo
	resolved []string
}

func (m *mockAlertResolver) GetActiveAlerts() []AlertInfo {
	return m.alerts
}

func (m *mockAlertResolver) ResolveAlert(alertID string) bool {
	m.resolved = append(m.resolved, alertID)
	return true
}
