package models

import (
	"testing"
	"time"
)

func TestStateUpdateRecentlyResolvedSnapshotIsolation(t *testing.T) {
	state := NewState()
	resolvedAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	input := []ResolvedAlert{
		{
			Alert: Alert{
				ID:           "alert-1",
				ResourceName: "node-a",
			},
			ResolvedTime: resolvedAt,
		},
	}

	state.UpdateRecentlyResolved(input)
	snapshot := state.GetSnapshot()
	if len(snapshot.RecentlyResolved) != 1 {
		t.Fatalf("RecentlyResolved length = %d, want 1", len(snapshot.RecentlyResolved))
	}
	if snapshot.RecentlyResolved[0].ID != "alert-1" {
		t.Fatalf("snapshot alert id = %q, want alert-1", snapshot.RecentlyResolved[0].ID)
	}

	snapshot.RecentlyResolved[0].ID = "mutated-snapshot"
	latest := state.GetSnapshot()
	if latest.RecentlyResolved[0].ID != "alert-1" {
		t.Fatalf("state changed with snapshot mutation: %q", latest.RecentlyResolved[0].ID)
	}
}

func TestStateSnapshotResolveResourceDockerAndKubernetesRouting(t *testing.T) {
	snapshot := StateSnapshot{
		Nodes: []Node{
			{Name: "pve-node"},
		},
		VMs: []VM{
			{Name: "vm-app", VMID: 101, Node: "pve-node"},
			{Name: "docker-vm", VMID: 102, Node: "pve-node"},
		},
		Containers: []Container{
			{Name: "lxc-app", VMID: 201, Node: "pve-node"},
			{Name: "docker-lxc", VMID: 202, Node: "pve-node"},
		},
		DockerHosts: []DockerHost{
			{
				ID:       "dh-lxc-id",
				Hostname: "docker-lxc",
				Containers: []DockerContainer{
					{Name: "ctr-on-lxc"},
				},
			},
			{
				ID:       "dh-vm-id",
				Hostname: "docker-vm",
				Containers: []DockerContainer{
					{Name: "ctr-on-vm"},
				},
			},
			{
				ID:       "dh-standalone-id",
				Hostname: "docker-standalone",
				Containers: []DockerContainer{
					{Name: "ctr-on-standalone"},
				},
			},
		},
		Hosts: []Host{
			{ID: "host-1", Hostname: "linux-1", Platform: "linux"},
		},
		KubernetesClusters: []KubernetesCluster{
			{
				ID:          "k8s-id",
				AgentID:     "agent-1",
				Name:        "k8s-main",
				DisplayName: "K8S Main",
				Pods: []KubernetesPod{
					{Name: "pod-a", Namespace: "ns-a"},
				},
				Deployments: []KubernetesDeployment{
					{Name: "deploy-a", Namespace: "ns-b"},
				},
			},
		},
	}

	testCases := []struct {
		name        string
		query       string
		wantType    string
		wantTarget  string
		wantName    string
		wantAgentID string
	}{
		{
			name:       "node",
			query:      "pve-node",
			wantType:   "node",
			wantTarget: "pve-node",
			wantName:   "pve-node",
		},
		{
			name:       "vm",
			query:      "vm-app",
			wantType:   "vm",
			wantTarget: "vm-app",
			wantName:   "vm-app",
		},
		{
			name:       "lxc",
			query:      "lxc-app",
			wantType:   "system-container",
			wantTarget: "lxc-app",
			wantName:   "lxc-app",
		},
		{
			name:       "docker host by id",
			query:      "dh-vm-id",
			wantType:   "docker-host",
			wantTarget: "docker-vm",
			wantName:   "docker-vm",
		},
		{
			name:       "docker container on lxc routes to lxc host",
			query:      "ctr-on-lxc",
			wantType:   "app-container",
			wantTarget: "docker-lxc",
			wantName:   "ctr-on-lxc",
		},
		{
			name:       "docker container on vm routes to vm host",
			query:      "ctr-on-vm",
			wantType:   "app-container",
			wantTarget: "docker-vm",
			wantName:   "ctr-on-vm",
		},
		{
			name:       "docker container on standalone routes to docker host",
			query:      "ctr-on-standalone",
			wantType:   "app-container",
			wantTarget: "docker-standalone",
			wantName:   "ctr-on-standalone",
		},
		{
			name:       "host by id",
			query:      "host-1",
			wantType:   "agent",
			wantTarget: "linux-1",
			wantName:   "linux-1",
		},
		{
			name:        "kubernetes cluster by display name",
			query:       "K8S Main",
			wantType:    "k8s-cluster",
			wantTarget:  "k8s-main",
			wantName:    "k8s-main",
			wantAgentID: "agent-1",
		},
		{
			name:        "kubernetes pod",
			query:       "pod-a",
			wantType:    "k8s-pod",
			wantTarget:  "k8s-main",
			wantName:    "pod-a",
			wantAgentID: "agent-1",
		},
		{
			name:        "kubernetes deployment",
			query:       "deploy-a",
			wantType:    "k8s-deployment",
			wantTarget:  "k8s-main",
			wantName:    "deploy-a",
			wantAgentID: "agent-1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			loc := snapshot.ResolveResource(tc.query)
			if !loc.Found {
				t.Fatalf("ResolveResource(%q) not found", tc.query)
			}
			if loc.ResourceType != tc.wantType {
				t.Fatalf("resource type = %q, want %q", loc.ResourceType, tc.wantType)
			}
			if loc.TargetHost != tc.wantTarget {
				t.Fatalf("target host = %q, want %q", loc.TargetHost, tc.wantTarget)
			}
			if loc.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", loc.Name, tc.wantName)
			}
			if tc.wantAgentID != "" && loc.AgentID != tc.wantAgentID {
				t.Fatalf("agent id = %q, want %q", loc.AgentID, tc.wantAgentID)
			}
		})
	}

	notFound := snapshot.ResolveResource("does-not-exist")
	if notFound.Found {
		t.Fatalf("ResolveResource should report not found for missing resource")
	}
	if notFound.Name != "does-not-exist" {
		t.Fatalf("missing resource name = %q, want does-not-exist", notFound.Name)
	}
}

func TestNewStateAndSnapshotInitializeCollectionSlices(t *testing.T) {
	state := NewState()
	snapshot := state.GetSnapshot()

	if state.Nodes == nil || snapshot.Nodes == nil {
		t.Fatal("expected nodes to be initialized")
	}
	if state.DockerHosts == nil || snapshot.DockerHosts == nil {
		t.Fatal("expected docker hosts to be initialized")
	}
	if state.RemovedDockerHosts == nil || snapshot.RemovedDockerHosts == nil {
		t.Fatal("expected removed docker hosts to be initialized")
	}
	if state.KubernetesClusters == nil || snapshot.KubernetesClusters == nil {
		t.Fatal("expected kubernetes clusters to be initialized")
	}
	if state.RemovedKubernetesClusters == nil || snapshot.RemovedKubernetesClusters == nil {
		t.Fatal("expected removed kubernetes clusters to be initialized")
	}
	if state.Hosts == nil || snapshot.Hosts == nil {
		t.Fatal("expected hosts to be initialized")
	}
	if state.RemovedHostAgents == nil || snapshot.RemovedHostAgents == nil {
		t.Fatal("expected removed host agents to be initialized")
	}
	if state.Storage == nil || snapshot.Storage == nil {
		t.Fatal("expected storage to be initialized")
	}
	if state.CephClusters == nil || snapshot.CephClusters == nil {
		t.Fatal("expected ceph clusters to be initialized")
	}
	if state.PhysicalDisks == nil || snapshot.PhysicalDisks == nil {
		t.Fatal("expected physical disks to be initialized")
	}
	if state.PBSInstances == nil || snapshot.PBSInstances == nil {
		t.Fatal("expected pbs instances to be initialized")
	}
	if state.PMGInstances == nil || snapshot.PMGInstances == nil {
		t.Fatal("expected pmg instances to be initialized")
	}
	if state.PBSBackups == nil || snapshot.PBSBackups == nil {
		t.Fatal("expected pbs backups to be initialized")
	}
	if state.PMGBackups == nil || snapshot.PMGBackups == nil {
		t.Fatal("expected pmg backups to be initialized")
	}
	if state.ReplicationJobs == nil || snapshot.ReplicationJobs == nil {
		t.Fatal("expected replication jobs to be initialized")
	}
	if state.Metrics == nil || snapshot.Metrics == nil {
		t.Fatal("expected metrics to be initialized")
	}
	if state.ActiveAlerts == nil || snapshot.ActiveAlerts == nil {
		t.Fatal("expected active alerts to be initialized")
	}
	if state.RecentlyResolved == nil || snapshot.RecentlyResolved == nil {
		t.Fatal("expected recently resolved alerts to be initialized")
	}
	if snapshot.PVEBackups.BackupTasks == nil || snapshot.PVEBackups.StorageBackups == nil || snapshot.PVEBackups.GuestSnapshots == nil {
		t.Fatal("expected pve backup collections to be initialized")
	}
	if snapshot.Backups.PVE.BackupTasks == nil || snapshot.Backups.PVE.StorageBackups == nil || snapshot.Backups.PVE.GuestSnapshots == nil {
		t.Fatal("expected aggregated pve backup collections to be initialized")
	}
	if snapshot.Backups.PBS == nil || snapshot.Backups.PMG == nil {
		t.Fatal("expected aggregated pbs/pmg backup collections to be initialized")
	}
}

func TestStateSnapshotNormalizeCollectionsInitializesNestedBackupCollections(t *testing.T) {
	snapshot := StateSnapshot{}
	snapshot.NormalizeCollections()

	if snapshot.PVEBackups.BackupTasks == nil || snapshot.PVEBackups.StorageBackups == nil || snapshot.PVEBackups.GuestSnapshots == nil {
		t.Fatal("expected PVEBackups collections to be initialized")
	}
	if snapshot.Backups.PVE.BackupTasks == nil || snapshot.Backups.PVE.StorageBackups == nil || snapshot.Backups.PVE.GuestSnapshots == nil {
		t.Fatal("expected Backups.PVE collections to be initialized")
	}
	if snapshot.Backups.PBS == nil || snapshot.Backups.PMG == nil {
		t.Fatal("expected Backups.PBS and Backups.PMG to be initialized")
	}
	if snapshot.ConnectionHealth == nil {
		t.Fatal("expected ConnectionHealth to be initialized")
	}
}

func TestEmptyStateSnapshotReturnsNormalizedZeroSnapshot(t *testing.T) {
	snapshot := EmptyStateSnapshot()

	if snapshot.Nodes == nil || snapshot.Hosts == nil || snapshot.DockerHosts == nil {
		t.Fatal("expected top-level collections to be initialized")
	}
	if snapshot.Backups.PBS == nil || snapshot.Backups.PMG == nil {
		t.Fatal("expected nested backup collections to be initialized")
	}
	if snapshot.ConnectionHealth == nil {
		t.Fatal("expected connection health map to be initialized")
	}
}

func TestStateSnapshotNormalizeCollectionsNormalizesNestedResourceCollections(t *testing.T) {
	snapshot := StateSnapshot{
		Nodes: []Node{{
			ID: "node-1",
		}},
		VMs: []VM{{
			ID: "vm-1",
		}},
		Containers: []Container{{
			ID: "ct-1",
		}},
		DockerHosts: []DockerHost{{
			ID:                "docker-1",
			NetworkInterfaces: []HostNetworkInterface{{Name: "eth0"}},
			Containers:        []DockerContainer{{ID: "container-1"}},
			Services:          []DockerService{{ID: "service-1"}},
		}},
		KubernetesClusters: []KubernetesCluster{{
			ID:          "k8s-1",
			Nodes:       []KubernetesNode{{UID: "node-1"}},
			Pods:        []KubernetesPod{{UID: "pod-1"}},
			Deployments: []KubernetesDeployment{{UID: "deploy-1"}},
		}},
		Hosts: []Host{{
			ID:                "host-1",
			NetworkInterfaces: []HostNetworkInterface{{Name: "eth0"}},
			Sensors:           HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu": 42}},
			RAID:              []HostRAIDArray{{Device: "md0"}},
			Unraid:            &HostUnraidStorage{},
			Ceph:              &HostCephCluster{},
		}},
		Storage: []Storage{{
			ID: "storage-1",
		}},
		CephClusters: []CephCluster{{
			ID: "ceph-1",
		}},
	}

	snapshot.NormalizeCollections()

	if snapshot.Nodes[0].LoadAverage == nil {
		t.Fatal("expected node loadAverage to be initialized")
	}
	if snapshot.VMs[0].Disks == nil || snapshot.VMs[0].IPAddresses == nil || snapshot.VMs[0].NetworkInterfaces == nil || snapshot.VMs[0].Tags == nil {
		t.Fatal("expected VM nested collections to be initialized")
	}
	if snapshot.Containers[0].Disks == nil || snapshot.Containers[0].IPAddresses == nil || snapshot.Containers[0].NetworkInterfaces == nil || snapshot.Containers[0].Tags == nil {
		t.Fatal("expected container nested collections to be initialized")
	}
	if snapshot.DockerHosts[0].LoadAverage == nil || snapshot.DockerHosts[0].Disks == nil || snapshot.DockerHosts[0].NetworkInterfaces == nil || snapshot.DockerHosts[0].Tasks == nil {
		t.Fatal("expected docker host top-level collections to be initialized")
	}
	if snapshot.DockerHosts[0].NetworkInterfaces[0].Addresses == nil {
		t.Fatal("expected docker host network interface addresses to be initialized")
	}
	if snapshot.DockerHosts[0].Containers[0].Ports == nil || snapshot.DockerHosts[0].Containers[0].Labels == nil || snapshot.DockerHosts[0].Containers[0].Networks == nil || snapshot.DockerHosts[0].Containers[0].Mounts == nil {
		t.Fatal("expected docker container nested collections to be initialized")
	}
	if snapshot.DockerHosts[0].Services[0].Labels == nil || snapshot.DockerHosts[0].Services[0].EndpointPorts == nil {
		t.Fatal("expected docker service nested collections to be initialized")
	}
	if snapshot.KubernetesClusters[0].Nodes[0].Roles == nil || snapshot.KubernetesClusters[0].Pods[0].Labels == nil || snapshot.KubernetesClusters[0].Pods[0].Containers == nil || snapshot.KubernetesClusters[0].Deployments[0].Labels == nil {
		t.Fatal("expected kubernetes nested collections to be initialized")
	}
	if snapshot.Hosts[0].LoadAverage == nil || snapshot.Hosts[0].Disks == nil || snapshot.Hosts[0].DiskIO == nil || snapshot.Hosts[0].NetworkInterfaces == nil || snapshot.Hosts[0].Tags == nil || snapshot.Hosts[0].DiskExclude == nil {
		t.Fatal("expected host top-level collections to be initialized")
	}
	if snapshot.Hosts[0].NetworkInterfaces[0].Addresses == nil {
		t.Fatal("expected host network interface addresses to be initialized")
	}
	if snapshot.Hosts[0].Sensors.FanRPM == nil || snapshot.Hosts[0].Sensors.Additional == nil || snapshot.Hosts[0].Sensors.SMART == nil {
		t.Fatal("expected host sensor collections to be initialized")
	}
	if snapshot.Hosts[0].RAID[0].Devices == nil {
		t.Fatal("expected host RAID device collection to be initialized")
	}
	if snapshot.Hosts[0].Unraid == nil || snapshot.Hosts[0].Unraid.Disks == nil {
		t.Fatal("expected host unraid collections to be initialized")
	}
	if snapshot.Hosts[0].Ceph == nil || snapshot.Hosts[0].Ceph.Pools == nil || snapshot.Hosts[0].Ceph.Services == nil || snapshot.Hosts[0].Ceph.Health.Checks == nil || snapshot.Hosts[0].Ceph.Health.Summary == nil || snapshot.Hosts[0].Ceph.MonMap.Monitors == nil {
		t.Fatal("expected host ceph collections to be initialized")
	}
	if snapshot.Storage[0].Nodes == nil || snapshot.Storage[0].NodeIDs == nil {
		t.Fatal("expected storage collections to be initialized")
	}
	if snapshot.CephClusters[0].Pools == nil || snapshot.CephClusters[0].Services == nil {
		t.Fatal("expected ceph cluster collections to be initialized")
	}
}

func TestStateIngressClonesNormalizeSourceEntityCollections(t *testing.T) {
	state := NewState()

	state.UpdateNodes([]Node{{ID: "node-1", Temperature: &Temperature{}}})
	state.UpdateVMs([]VM{{ID: "vm-1", NetworkInterfaces: []GuestNetworkInterface{{Name: "eth0"}}}})
	state.UpdateContainers([]Container{{ID: "ct-1", NetworkInterfaces: []GuestNetworkInterface{{Name: "eth0"}}}})
	state.UpsertDockerHost(DockerHost{
		ID:                "docker-1",
		NetworkInterfaces: []HostNetworkInterface{{Name: "eth0"}},
		Containers:        []DockerContainer{{ID: "container-1"}},
		Services:          []DockerService{{ID: "service-1"}},
	})
	state.UpsertKubernetesCluster(KubernetesCluster{
		ID:          "k8s-1",
		Nodes:       []KubernetesNode{{UID: "node-1"}},
		Pods:        []KubernetesPod{{UID: "pod-1"}},
		Deployments: []KubernetesDeployment{{UID: "deploy-1"}},
	})
	state.UpsertHost(Host{
		ID:                "host-1",
		NetworkInterfaces: []HostNetworkInterface{{Name: "eth0"}},
		Sensors:           HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu": 42}},
		RAID:              []HostRAIDArray{{Device: "md0"}},
		Unraid:            &HostUnraidStorage{},
		Ceph:              &HostCephCluster{},
	})
	state.UpdateStorage([]Storage{{ID: "storage-1"}})
	state.UpsertCephCluster(CephCluster{ID: "ceph-1"})
	state.UpdatePBSInstances([]PBSInstance{{ID: "pbs-1", Datastores: []PBSDatastore{{Name: "ds-1"}}}})
	state.UpdatePMGInstances([]PMGInstance{{ID: "pmg-1", Nodes: []PMGNodeStatus{{Name: "node-1"}}}})
	state.UpdatePBSBackups("pbs-1", []PBSBackup{{ID: "backup-1"}})

	if state.Nodes[0].LoadAverage == nil {
		t.Fatal("expected stored node loadAverage to be initialized")
	}
	if state.Nodes[0].Temperature != nil && (state.Nodes[0].Temperature.Cores == nil || state.Nodes[0].Temperature.GPU == nil || state.Nodes[0].Temperature.NVMe == nil || state.Nodes[0].Temperature.SMART == nil) {
		t.Fatal("expected stored node temperature collections to be initialized")
	}
	if state.VMs[0].Disks == nil || state.VMs[0].IPAddresses == nil || state.VMs[0].NetworkInterfaces == nil || state.VMs[0].Tags == nil {
		t.Fatal("expected stored VM collections to be initialized")
	}
	if state.VMs[0].NetworkInterfaces[0].Addresses == nil {
		t.Fatal("expected stored VM guest network interface addresses to be initialized")
	}
	if state.Containers[0].Disks == nil || state.Containers[0].IPAddresses == nil || state.Containers[0].NetworkInterfaces == nil || state.Containers[0].Tags == nil {
		t.Fatal("expected stored container collections to be initialized")
	}
	if state.Containers[0].NetworkInterfaces[0].Addresses == nil {
		t.Fatal("expected stored container guest network interface addresses to be initialized")
	}
	if state.DockerHosts[0].LoadAverage == nil || state.DockerHosts[0].Disks == nil || state.DockerHosts[0].NetworkInterfaces == nil || state.DockerHosts[0].Tasks == nil {
		t.Fatal("expected stored docker host collections to be initialized")
	}
	if state.DockerHosts[0].NetworkInterfaces[0].Addresses == nil {
		t.Fatal("expected stored docker host network interface addresses to be initialized")
	}
	if state.DockerHosts[0].Containers[0].Ports == nil || state.DockerHosts[0].Containers[0].Labels == nil || state.DockerHosts[0].Containers[0].Networks == nil || state.DockerHosts[0].Containers[0].Mounts == nil {
		t.Fatal("expected stored docker container collections to be initialized")
	}
	if state.DockerHosts[0].Services[0].Labels == nil || state.DockerHosts[0].Services[0].EndpointPorts == nil {
		t.Fatal("expected stored docker service collections to be initialized")
	}
	if state.KubernetesClusters[0].Nodes[0].Roles == nil || state.KubernetesClusters[0].Pods[0].Labels == nil || state.KubernetesClusters[0].Pods[0].Containers == nil || state.KubernetesClusters[0].Deployments[0].Labels == nil {
		t.Fatal("expected stored kubernetes collections to be initialized")
	}
	if state.Hosts[0].LoadAverage == nil || state.Hosts[0].Disks == nil || state.Hosts[0].DiskIO == nil || state.Hosts[0].NetworkInterfaces == nil || state.Hosts[0].Tags == nil || state.Hosts[0].DiskExclude == nil {
		t.Fatal("expected stored host top-level collections to be initialized")
	}
	if state.Hosts[0].NetworkInterfaces[0].Addresses == nil {
		t.Fatal("expected stored host network interface addresses to be initialized")
	}
	if state.Hosts[0].Sensors.FanRPM == nil || state.Hosts[0].Sensors.Additional == nil || state.Hosts[0].Sensors.SMART == nil {
		t.Fatal("expected stored host sensor collections to be initialized")
	}
	if state.Hosts[0].RAID[0].Devices == nil {
		t.Fatal("expected stored host RAID device collection to be initialized")
	}
	if state.Hosts[0].Unraid == nil || state.Hosts[0].Unraid.Disks == nil {
		t.Fatal("expected stored host unraid collections to be initialized")
	}
	if state.Hosts[0].Ceph == nil || state.Hosts[0].Ceph.Pools == nil || state.Hosts[0].Ceph.Services == nil || state.Hosts[0].Ceph.Health.Checks == nil || state.Hosts[0].Ceph.Health.Summary == nil || state.Hosts[0].Ceph.MonMap.Monitors == nil {
		t.Fatal("expected stored host ceph collections to be initialized")
	}
	if state.Storage[0].Nodes == nil || state.Storage[0].NodeIDs == nil {
		t.Fatal("expected stored storage collections to be initialized")
	}
	if state.CephClusters[0].Pools == nil || state.CephClusters[0].Services == nil {
		t.Fatal("expected stored ceph cluster collections to be initialized")
	}
	if state.PBSInstances[0].Datastores == nil || state.PBSInstances[0].BackupJobs == nil || state.PBSInstances[0].SyncJobs == nil || state.PBSInstances[0].VerifyJobs == nil || state.PBSInstances[0].PruneJobs == nil || state.PBSInstances[0].GarbageJobs == nil {
		t.Fatal("expected stored PBS instance collections to be initialized")
	}
	if state.PBSInstances[0].Datastores[0].Namespaces == nil {
		t.Fatal("expected stored PBS datastore namespaces to be initialized")
	}
	if state.PMGInstances[0].Nodes == nil || state.PMGInstances[0].MailCount == nil || state.PMGInstances[0].SpamDistribution == nil || state.PMGInstances[0].RelayDomains == nil || state.PMGInstances[0].DomainStats == nil {
		t.Fatal("expected stored PMG instance collections to be initialized")
	}
	if state.PBSBackups[0].Files == nil {
		t.Fatal("expected stored PBS backup files to be initialized")
	}
}

func TestStateSnapshotNormalizeCollectionsNormalizesBackupAndAuxiliaryOwners(t *testing.T) {
	snapshot := StateSnapshot{
		PBSInstances: []PBSInstance{{ID: "pbs-1", Datastores: []PBSDatastore{{Name: "ds-1"}}}},
		PMGInstances: []PMGInstance{{ID: "pmg-1", Nodes: []PMGNodeStatus{{Name: "node-1"}}}},
		PBSBackups:   []PBSBackup{{ID: "backup-1"}},
		Metrics:      []Metric{{ID: "metric-1"}},
		Backups: Backups{
			PVE: PVEBackups{},
			PBS: []PBSBackup{{ID: "backup-2"}},
		},
	}

	snapshot.NormalizeCollections()

	if snapshot.PBSInstances[0].Datastores == nil || snapshot.PBSInstances[0].BackupJobs == nil || snapshot.PBSInstances[0].SyncJobs == nil || snapshot.PBSInstances[0].VerifyJobs == nil || snapshot.PBSInstances[0].PruneJobs == nil || snapshot.PBSInstances[0].GarbageJobs == nil {
		t.Fatal("expected snapshot PBS instance collections to be initialized")
	}
	if snapshot.PBSInstances[0].Datastores[0].Namespaces == nil {
		t.Fatal("expected snapshot PBS datastore namespaces to be initialized")
	}
	if snapshot.PMGInstances[0].Nodes == nil || snapshot.PMGInstances[0].MailCount == nil || snapshot.PMGInstances[0].SpamDistribution == nil || snapshot.PMGInstances[0].RelayDomains == nil || snapshot.PMGInstances[0].DomainStats == nil {
		t.Fatal("expected snapshot PMG instance collections to be initialized")
	}
	if snapshot.PBSBackups[0].Files == nil {
		t.Fatal("expected snapshot PBS backup files to be initialized")
	}
	if snapshot.Metrics[0].Values == nil {
		t.Fatal("expected snapshot metric values to be initialized")
	}
	if snapshot.Backups.PVE.BackupTasks == nil || snapshot.Backups.PVE.StorageBackups == nil || snapshot.Backups.PVE.GuestSnapshots == nil {
		t.Fatal("expected snapshot aggregate PVE backup collections to be initialized")
	}
	if snapshot.Backups.PBS[0].Files == nil || snapshot.Backups.PMG == nil {
		t.Fatal("expected snapshot aggregate backup collections to be initialized")
	}
}

func TestStatePerformanceUsesCanonicalEmptyMap(t *testing.T) {
	state := NewState()
	if state.Performance.APICallDuration == nil {
		t.Fatal("expected live state performance apiCallDuration to be initialized")
	}

	snapshot := StateSnapshot{}
	snapshot.NormalizeCollections()
	if snapshot.Performance.APICallDuration == nil {
		t.Fatal("expected snapshot performance apiCallDuration to be initialized")
	}
}
