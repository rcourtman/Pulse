package resources

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestFromNodeTemperatureAndCluster(t *testing.T) {
	now := time.Now()
	node := models.Node{
		ID:              "node-1",
		Name:            "node-1",
		Instance:        "pve1",
		Status:          "online",
		CPU:             0.5,
		Memory:          models.Memory{Total: 100, Used: 50, Free: 50, Usage: 50},
		Disk:            models.Disk{Total: 200, Used: 100, Free: 100, Usage: 50},
		Uptime:          10,
		LastSeen:        now,
		IsClusterMember: true,
		ClusterName:     "cluster-a",
		Temperature: &models.Temperature{
			Available:  true,
			HasCPU:     true,
			CPUPackage: 72.5,
		},
	}
	r := FromNode(node)
	if r.Temperature == nil || *r.Temperature != 72.5 {
		t.Fatalf("expected CPU package temperature")
	}
	if r.ClusterID != "pve-cluster/cluster-a" {
		t.Fatalf("expected cluster ID")
	}

	node2 := node
	node2.ID = "node-2"
	node2.ClusterName = ""
	node2.IsClusterMember = false
	node2.Temperature = &models.Temperature{
		Available: true,
		HasCPU:    true,
		Cores: []models.CoreTemp{
			{Core: 0, Temp: 60},
			{Core: 1, Temp: 80},
		},
	}
	r2 := FromNode(node2)
	if r2.Temperature == nil || *r2.Temperature != 70 {
		t.Fatalf("expected averaged core temperature")
	}
	if r2.ClusterID != "" {
		t.Fatalf("expected empty cluster ID when not a member")
	}
}

func TestFromHostAndDockerHost(t *testing.T) {
	now := time.Now()
	host := models.Host{
		ID:       "host-1",
		Hostname: "host-1",
		Status:   "degraded",
		Memory:   models.Memory{Total: 100, Used: 50, Free: 50, Usage: 50},
		Disks: []models.Disk{
			{Total: 100, Used: 60, Free: 40, Usage: 60},
		},
		NetworkInterfaces: []models.HostNetworkInterface{
			{Addresses: []string{"10.0.0.1"}, RXBytes: 10, TXBytes: 20},
		},
		Sensors: models.HostSensorSummary{
			TemperatureCelsius: map[string]float64{"cpu": 55},
		},
		LastSeen: now,
	}
	hr := FromHost(host)
	if hr.Temperature == nil || *hr.Temperature != 55 {
		t.Fatalf("expected host temperature")
	}
	if hr.Network == nil || hr.Network.RXBytes != 10 || hr.Network.TXBytes != 20 {
		t.Fatalf("expected host network totals")
	}
	if hr.Disk == nil || hr.Disk.Current == 0 {
		t.Fatalf("expected host disk metrics")
	}
	if hr.Status != StatusDegraded {
		t.Fatalf("expected degraded host status")
	}

	dockerHost := models.DockerHost{
		ID:                "docker-1",
		AgentID:           "agent-1",
		Hostname:          "docker-1",
		DisplayName:       "docker-1",
		CustomDisplayName: "custom",
		Status:            "offline",
		Memory:            models.Memory{Total: 100, Used: 50, Free: 50, Usage: 50},
		Disks: []models.Disk{
			{Total: 100, Used: 50, Free: 50, Usage: 50},
		},
		NetworkInterfaces: []models.HostNetworkInterface{
			{Addresses: []string{"10.0.0.2"}, RXBytes: 1, TXBytes: 2},
		},
		Swarm: &models.DockerSwarmInfo{
			ClusterID: "swarm-1",
		},
		LastSeen: now,
	}
	dr := FromDockerHost(dockerHost)
	if dr.DisplayName != "custom" {
		t.Fatalf("expected custom display name")
	}
	if dr.ClusterID != "docker-swarm/swarm-1" {
		t.Fatalf("expected docker swarm cluster ID")
	}
	if dr.Status != StatusOffline {
		t.Fatalf("expected docker host status offline")
	}
	if dr.Identity == nil || len(dr.Identity.IPs) != 1 || dr.Identity.IPs[0] != "10.0.0.2" {
		t.Fatalf("expected docker host identity IPs")
	}
}

func TestFromHostPlatformData(t *testing.T) {
	now := time.Now()
	host := models.Host{
		ID:       "host-2",
		Hostname: "host-2",
		Status:   "online",
		DiskIO: []models.DiskIO{
			{Device: "sda", ReadBytes: 100, WriteBytes: 200, ReadOps: 1, WriteOps: 2, ReadTime: 3, WriteTime: 4, IOTime: 5},
		},
		RAID: []models.HostRAIDArray{
			{
				Device:         "md0",
				Level:          "raid1",
				State:          "clean",
				TotalDevices:   2,
				ActiveDevices:  2,
				WorkingDevices: 2,
				FailedDevices:  0,
				SpareDevices:   0,
				Devices: []models.HostRAIDDevice{
					{Device: "sda1", State: "active", Slot: 0},
				},
				RebuildPercent: 0,
			},
		},
		LastSeen: now,
	}

	r := FromHost(host)
	var pd HostPlatformData
	if err := r.GetPlatformData(&pd); err != nil {
		t.Fatalf("failed to get platform data: %v", err)
	}
	if len(pd.DiskIO) != 1 || len(pd.RAID) != 1 {
		t.Fatalf("expected disk IO and RAID entries")
	}
}

func TestFromDockerContainerPodman(t *testing.T) {
	now := time.Now()
	container := models.DockerContainer{
		ID:            "container-1",
		Name:          "app",
		Image:         "app:latest",
		State:         "paused",
		Status:        "Paused",
		CPUPercent:    5,
		MemoryUsage:   50,
		MemoryLimit:   100,
		MemoryPercent: 50,
		UptimeSeconds: 10,
		Ports: []models.DockerContainerPort{
			{PrivatePort: 80, PublicPort: 8080, Protocol: "tcp", IP: "0.0.0.0"},
		},
		Networks: []models.DockerContainerNetworkLink{
			{Name: "bridge", IPv4: "172.17.0.2"},
		},
		Podman: &models.DockerPodmanContainer{
			PodName: "pod",
		},
		CreatedAt: now,
	}

	r := FromDockerContainer(container, "host-1", "host-1")
	if r.Status != StatusPaused {
		t.Fatalf("expected paused status")
	}
	if r.Memory == nil || r.Memory.Current != 50 {
		t.Fatalf("expected container memory metrics")
	}
	if r.ID != "host-1/container-1" {
		t.Fatalf("expected container resource ID to be host-1/container-1")
	}
}

func TestFromVMWithBackup(t *testing.T) {
	now := time.Now()
	vm := models.VM{
		ID:         "vm-1",
		VMID:       100,
		Name:       "vm-1",
		Node:       "node-1",
		Instance:   "pve1",
		Status:     "running",
		CPU:        0.25,
		LastBackup: now,
		LastSeen:   now,
	}

	r := FromVM(vm)
	var pd VMPlatformData
	if err := r.GetPlatformData(&pd); err != nil {
		t.Fatalf("failed to get platform data: %v", err)
	}
	if pd.LastBackup == nil || !pd.LastBackup.Equal(now) {
		t.Fatalf("expected last backup to be set")
	}
}

func TestFromContainerOCI(t *testing.T) {
	now := time.Now()
	ct := models.Container{
		ID:         "ct-1",
		VMID:       200,
		Name:       "ct-1",
		Node:       "node-1",
		Instance:   "pve1",
		Status:     "paused",
		Type:       "lxc",
		IsOCI:      true,
		CPU:        0.1,
		Memory:     models.Memory{Total: 100, Used: 50, Free: 50, Usage: 50},
		Disk:       models.Disk{Total: 200, Used: 100, Free: 100, Usage: 50},
		LastBackup: now,
		LastSeen:   now,
	}

	r := FromContainer(ct)
	if r.Type != ResourceTypeOCIContainer {
		t.Fatalf("expected OCI container type")
	}
	var pd ContainerPlatformData
	if err := r.GetPlatformData(&pd); err != nil {
		t.Fatalf("failed to get platform data: %v", err)
	}
	if !pd.IsOCI || pd.LastBackup == nil {
		t.Fatalf("expected OCI platform data with backup")
	}
}

func TestKubernetesConversions(t *testing.T) {
	now := time.Now()
	cluster := models.KubernetesCluster{
		ID:                "cluster-1",
		AgentID:           "agent-1",
		CustomDisplayName: "custom",
		Status:            "online",
		LastSeen:          now,
		Nodes: []models.KubernetesNode{
			{Name: "node-1", Ready: true, Unschedulable: true},
		},
		Pods: []models.KubernetesPod{
			{Name: "pod-1", Namespace: "default", Phase: "Succeeded", NodeName: "node-1"},
		},
		Deployments: []models.KubernetesDeployment{
			{Name: "dep-1", Namespace: "default", DesiredReplicas: 1, AvailableReplicas: 1},
		},
	}

	cr := FromKubernetesCluster(cluster)
	if cr.Name != "custom" || cr.DisplayName != "custom" {
		t.Fatalf("expected custom display name to be used")
	}

	node := cluster.Nodes[0]
	nr := FromKubernetesNode(node, cluster)
	if nr.Status != StatusDegraded {
		t.Fatalf("expected unschedulable node to be degraded")
	}
	if !strings.Contains(nr.ID, "cluster-1/node/") {
		t.Fatalf("expected node ID to include cluster and node")
	}

	pod := cluster.Pods[0]
	pr := FromKubernetesPod(pod, cluster)
	if pr.Status != StatusStopped {
		t.Fatalf("expected succeeded pod to be stopped")
	}
	if !strings.Contains(pr.ParentID, "cluster-1/node/") {
		t.Fatalf("expected pod parent to be node")
	}

	dep := cluster.Deployments[0]
	dr := FromKubernetesDeployment(dep, cluster)
	if dr.Status != StatusRunning {
		t.Fatalf("expected deployment running")
	}

	emptyCluster := models.KubernetesCluster{
		ID:       "cluster-2",
		AgentID:  "agent-2",
		Status:   "offline",
		LastSeen: now,
	}
	er := FromKubernetesCluster(emptyCluster)
	if er.Name != "cluster-2" || er.DisplayName != "cluster-2" {
		t.Fatalf("expected cluster name fallback to ID")
	}
	if er.Status != StatusOffline {
		t.Fatalf("expected offline cluster status")
	}
}

func TestKubernetesNodeNotReady(t *testing.T) {
	now := time.Now()
	cluster := models.KubernetesCluster{ID: "cluster-1", AgentID: "agent-1", LastSeen: now}
	node := models.KubernetesNode{
		UID:   "node-uid",
		Name:  "node-offline",
		Ready: false,
	}
	r := FromKubernetesNode(node, cluster)
	if r.Status != StatusOffline {
		t.Fatalf("expected offline node status")
	}
	if !strings.Contains(r.ID, "node-uid") {
		t.Fatalf("expected node UID in resource ID")
	}
}

func TestKubernetesPodParentCluster(t *testing.T) {
	now := time.Now()
	cluster := models.KubernetesCluster{ID: "cluster-1", AgentID: "agent-1", LastSeen: now}
	pod := models.KubernetesPod{
		UID:       "pod-uid",
		Name:      "pod-1",
		Namespace: "default",
		Phase:     "Pending",
	}

	r := FromKubernetesPod(pod, cluster)
	if r.ParentID != cluster.ID {
		t.Fatalf("expected pod parent to be cluster ID")
	}
	if !strings.Contains(r.ID, "pod-uid") {
		t.Fatalf("expected pod UID in resource ID")
	}
}

func TestKubernetesPodContainers(t *testing.T) {
	now := time.Now()
	cluster := models.KubernetesCluster{ID: "cluster-3", AgentID: "agent-3", LastSeen: now}
	pod := models.KubernetesPod{
		Name:      "pod-2",
		Namespace: "default",
		NodeName:  "node-2",
		Phase:     "Running",
		Containers: []models.KubernetesPodContainer{
			{Name: "c1", Image: "busybox", Ready: true, RestartCount: 1, State: "running"},
		},
	}

	r := FromKubernetesPod(pod, cluster)
	if r.Status != StatusRunning {
		t.Fatalf("expected running pod status")
	}
	if !strings.Contains(r.ID, "cluster-3/pod/") {
		t.Fatalf("expected pod ID to include cluster")
	}
}

func TestKubernetesDeploymentStatuses(t *testing.T) {
	now := time.Now()
	cluster := models.KubernetesCluster{ID: "cluster-1", AgentID: "agent-1", LastSeen: now}

	depStopped := FromKubernetesDeployment(models.KubernetesDeployment{
		Name:            "dep-stop",
		Namespace:       "default",
		DesiredReplicas: 0,
	}, cluster)
	if depStopped.Status != StatusStopped {
		t.Fatalf("expected stopped deployment")
	}

	depDegraded := FromKubernetesDeployment(models.KubernetesDeployment{
		Name:              "dep-degraded",
		Namespace:         "default",
		DesiredReplicas:   3,
		AvailableReplicas: 1,
	}, cluster)
	if depDegraded.Status != StatusDegraded {
		t.Fatalf("expected degraded deployment")
	}

	depUnknown := FromKubernetesDeployment(models.KubernetesDeployment{
		Name:              "dep-unknown",
		Namespace:         "default",
		DesiredReplicas:   2,
		AvailableReplicas: 0,
	}, cluster)
	if depUnknown.Status != StatusUnknown {
		t.Fatalf("expected unknown deployment")
	}
}

func TestFromPBSInstanceAndStorage(t *testing.T) {
	now := time.Now()
	pbs := models.PBSInstance{
		ID:               "pbs-1",
		Name:             "pbs",
		Host:             "pbs.local",
		Status:           "online",
		ConnectionHealth: "unhealthy",
		CPU:              20,
		Memory:           50,
		MemoryTotal:      100,
		MemoryUsed:       50,
		Uptime:           10,
		LastSeen:         now,
	}
	pr := FromPBSInstance(pbs)
	if pr.Status != StatusDegraded {
		t.Fatalf("expected degraded status for unhealthy pbs")
	}

	pbs2 := pbs
	pbs2.ID = "pbs-2"
	pbs2.ConnectionHealth = "healthy"
	pbs2.Status = "offline"
	pr2 := FromPBSInstance(pbs2)
	if pr2.Status != StatusOffline {
		t.Fatalf("expected offline status for pbs")
	}

	ds := models.PBSDatastore{
		Name:                "main",
		Total:               200,
		Used:                100,
		Free:                100,
		Usage:               50,
		Status:              "available",
		DeduplicationFactor: 2.4,
	}
	dsr := FromPBSDatastore(pbs, ds)
	if dsr.Type != ResourceTypeDatastore {
		t.Fatalf("expected datastore resource type")
	}
	if dsr.ParentID != pbs.ID {
		t.Fatalf("expected datastore parent ID to be PBS instance ID")
	}
	if dsr.Status != StatusOnline {
		t.Fatalf("expected online datastore status")
	}

	dsErr := ds
	dsErr.Error = "connect timeout"
	dsrErr := FromPBSDatastore(pbs, dsErr)
	if dsrErr.Status != StatusDegraded {
		t.Fatalf("expected degraded status for datastore with error")
	}

	pmg := models.PMGInstance{
		ID:               "pmg-1",
		Name:             "pmg",
		Host:             "pmg.local",
		Status:           "online",
		ConnectionHealth: "healthy",
		LastSeen:         now,
		LastUpdated:      now,
		Nodes: []models.PMGNodeStatus{
			{
				Name:   "pmg-node-1",
				Uptime: 120,
				QueueStatus: &models.PMGQueueStatus{
					Active: 1,
					Total:  1,
				},
			},
		},
	}
	pmgr := FromPMGInstance(pmg)
	if pmgr.Type != ResourceTypePMG {
		t.Fatalf("expected PMG resource type")
	}
	if pmgr.Status != StatusOnline {
		t.Fatalf("expected online PMG status")
	}

	pmg.ConnectionHealth = "unhealthy"
	pmgr2 := FromPMGInstance(pmg)
	if pmgr2.Status != StatusDegraded {
		t.Fatalf("expected degraded PMG status when connection is unhealthy")
	}

	storageOnline := FromStorage(models.Storage{
		ID:       "storage-1",
		Name:     "local",
		Instance: "pve1",
		Node:     "node1",
		Total:    100,
		Used:     50,
		Free:     50,
		Usage:    50,
		Active:   true,
		Enabled:  true,
	})
	if storageOnline.Status != StatusOnline {
		t.Fatalf("expected online storage")
	}

	storageStopped := FromStorage(models.Storage{
		ID:       "storage-2",
		Name:     "local",
		Instance: "pve1",
		Node:     "node1",
		Active:   true,
		Enabled:  false,
	})
	if storageStopped.Status != StatusStopped {
		t.Fatalf("expected stopped storage")
	}

	storageOffline := FromStorage(models.Storage{
		ID:       "storage-3",
		Name:     "local",
		Instance: "pve1",
		Node:     "node1",
		Active:   false,
		Enabled:  true,
	})
	if storageOffline.Status != StatusOffline {
		t.Fatalf("expected offline storage")
	}
}

func TestStatusMappings(t *testing.T) {
	if mapGuestStatus("running") != StatusRunning {
		t.Fatalf("expected running guest status")
	}
	if mapGuestStatus("stopped") != StatusStopped {
		t.Fatalf("expected stopped guest status")
	}
	if mapGuestStatus("paused") != StatusPaused {
		t.Fatalf("expected paused guest status")
	}
	if mapGuestStatus("unknown") != StatusUnknown {
		t.Fatalf("expected unknown guest status")
	}

	if mapHostStatus("online") != StatusOnline {
		t.Fatalf("expected online host status")
	}
	if mapHostStatus("offline") != StatusOffline {
		t.Fatalf("expected offline host status")
	}
	if mapHostStatus("degraded") != StatusDegraded {
		t.Fatalf("expected degraded host status")
	}
	if mapHostStatus("unknown") != StatusUnknown {
		t.Fatalf("expected unknown host status")
	}

	if mapDockerHostStatus("online") != StatusOnline {
		t.Fatalf("expected online docker host status")
	}
	if mapDockerHostStatus("offline") != StatusOffline {
		t.Fatalf("expected offline docker host status")
	}
	if mapDockerHostStatus("unknown") != StatusUnknown {
		t.Fatalf("expected unknown docker host status")
	}

	if mapDockerContainerStatus("running") != StatusRunning {
		t.Fatalf("expected running container status")
	}
	if mapDockerContainerStatus("exited") != StatusStopped {
		t.Fatalf("expected exited container status")
	}
	if mapDockerContainerStatus("dead") != StatusStopped {
		t.Fatalf("expected dead container status")
	}
	if mapDockerContainerStatus("paused") != StatusPaused {
		t.Fatalf("expected paused container status")
	}
	if mapDockerContainerStatus("restarting") != StatusUnknown {
		t.Fatalf("expected restarting container status unknown")
	}
	if mapDockerContainerStatus("created") != StatusUnknown {
		t.Fatalf("expected created container status unknown")
	}
	if mapDockerContainerStatus("other") != StatusUnknown {
		t.Fatalf("expected unknown container status")
	}

	if mapKubernetesClusterStatus(" online ") != StatusOnline {
		t.Fatalf("expected online k8s cluster status")
	}
	if mapKubernetesClusterStatus("offline") != StatusOffline {
		t.Fatalf("expected offline k8s cluster status")
	}
	if mapKubernetesClusterStatus("unknown") != StatusUnknown {
		t.Fatalf("expected unknown k8s cluster status")
	}

	if mapKubernetesPodStatus("running") != StatusRunning {
		t.Fatalf("expected running pod status")
	}
	if mapKubernetesPodStatus("succeeded") != StatusStopped {
		t.Fatalf("expected succeeded pod status")
	}
	if mapKubernetesPodStatus("failed") != StatusStopped {
		t.Fatalf("expected failed pod status")
	}
	if mapKubernetesPodStatus("pending") != StatusUnknown {
		t.Fatalf("expected pending pod status")
	}
	if mapKubernetesPodStatus("unknown") != StatusUnknown {
		t.Fatalf("expected unknown pod status")
	}

	if mapPBSStatus("online", "healthy") != StatusOnline {
		t.Fatalf("expected online PBS status")
	}
	if mapPBSStatus("offline", "healthy") != StatusOffline {
		t.Fatalf("expected offline PBS status")
	}
	if mapPBSStatus("other", "healthy") != StatusUnknown {
		t.Fatalf("expected unknown PBS status")
	}
	if mapPBSStatus("online", "unhealthy") != StatusDegraded {
		t.Fatalf("expected degraded PBS status")
	}
}
