package unifiedresources

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func TestResourceFromProxmoxNodeIncludesTemperature(t *testing.T) {
	node := models.Node{
		ID:     "mock-cluster-pve1",
		Name:   "pve1",
		Status: "online",
		Temperature: &models.Temperature{
			Available:  true,
			HasCPU:     true,
			CPUPackage: 55.2,
			CPUMax:     61.9,
		},
	}

	resource, _ := resourceFromProxmoxNode(node, nil)
	if resource.Proxmox == nil {
		t.Fatal("expected proxmox payload")
	}
	if resource.Proxmox.SourceID != node.ID {
		t.Fatalf("SourceID = %q, want %q (must preserve legacy node ID for MetricsHistory)", resource.Proxmox.SourceID, node.ID)
	}
	if resource.Proxmox.Temperature == nil {
		t.Fatal("expected proxmox temperature to be populated")
	}
	if got, want := *resource.Proxmox.Temperature, 61.9; got != want {
		t.Fatalf("temperature = %v, want %v", got, want)
	}
}

func TestResourceFromProxmoxNodeStoresEndpointIPAsIPAddress(t *testing.T) {
	node := models.Node{
		ID:   "mock-cluster-minipc",
		Name: "minipc",
		Host: "https://10.0.0.5:8006",
	}

	_, identity := resourceFromProxmoxNode(node, nil)
	if len(identity.Hostnames) != 1 || identity.Hostnames[0] != "minipc" {
		t.Fatalf("Hostnames = %v, want [minipc]", identity.Hostnames)
	}
	if len(identity.IPAddresses) != 1 || identity.IPAddresses[0] != "10.0.0.5" {
		t.Fatalf("IPAddresses = %v, want [10.0.0.5]", identity.IPAddresses)
	}
}

func TestResourceFromProxmoxNodeInheritsLinkedHostIdentity(t *testing.T) {
	node := models.Node{
		ID:            "mock-cluster-minipc",
		Name:          "minipc",
		Host:          "https://10.0.0.5:8006",
		LinkedAgentID: "host-1",
	}
	host := &models.Host{
		ID:        "host-1",
		Hostname:  "minipc.local",
		MachineID: "machine-1",
		ReportIP:  "10.0.0.5",
		NetworkInterfaces: []models.HostNetworkInterface{
			{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5/24"}},
		},
	}

	_, identity := resourceFromProxmoxNode(node, host)
	if identity.MachineID != "machine-1" {
		t.Fatalf("MachineID = %q, want machine-1", identity.MachineID)
	}
	if len(identity.MACAddresses) != 1 || identity.MACAddresses[0] != "00:11:22:33:44:55" {
		t.Fatalf("MACAddresses = %v, want [00:11:22:33:44:55]", identity.MACAddresses)
	}
}

func TestMaxNodeTempHandlesUnavailableData(t *testing.T) {
	if temp := maxNodeTemp(nil); temp != nil {
		t.Fatalf("expected nil temperature for nil input, got %v", *temp)
	}

	input := &models.Temperature{
		Available: false,
		HasCPU:    false,
	}
	if temp := maxNodeTemp(input); temp != nil {
		t.Fatalf("expected nil temperature for unavailable snapshot, got %v", *temp)
	}
}

func TestResourceFromDockerHostIncludesTemperature(t *testing.T) {
	value := 52.7
	host := models.DockerHost{
		ID:          "docker-1",
		Hostname:    "docker-1",
		DisplayName: "Docker 1",
		Status:      "online",
		Temperature: &value,
	}

	resource, _ := resourceFromDockerHost(host)
	if resource.Docker == nil {
		t.Fatal("expected docker payload")
	}
	if resource.Docker.Temperature == nil {
		t.Fatal("expected docker temperature to be populated")
	}
	if got, want := *resource.Docker.Temperature, value; got != want {
		t.Fatalf("temperature = %v, want %v", got, want)
	}
}

func TestResourceFromDockerHostOmitsStandaloneInactiveSwarm(t *testing.T) {
	host := models.DockerHost{
		ID:       "docker-1",
		Hostname: "docker-1",
		Status:   "online",
		Swarm: &models.DockerSwarmInfo{
			NodeRole:   "worker",
			LocalState: "inactive",
			Scope:      "node",
		},
	}

	resource, _ := resourceFromDockerHost(host)
	if resource.Docker == nil {
		t.Fatal("expected docker payload")
	}
	if resource.Docker.Swarm != nil {
		t.Fatalf("standalone inactive Docker Swarm metadata should be omitted, got %+v", resource.Docker.Swarm)
	}
}

func TestResourceFromDockerContainerIncludesContainerID(t *testing.T) {
	container := models.DockerContainer{
		ID:            "aurora-3-abcdef123456",
		Name:          "web",
		State:         "running",
		Image:         "ghcr.io/example/web:1.0.0",
		ImageDigest:   "sha256:current",
		UptimeSeconds: 1234,
		CPUPercent:    12.5,
	}

	host := models.DockerHost{
		ID:             "docker-1",
		Hostname:       "docker-1",
		Runtime:        "podman",
		RuntimeVersion: "5.2.0",
	}
	resource, _ := resourceFromDockerContainer(container, host)
	if resource.Docker == nil {
		t.Fatal("expected docker payload")
	}
	if got, want := resource.Docker.ContainerID, container.ID; got != want {
		t.Fatalf("containerId = %q, want %q", got, want)
	}
	if got, want := resource.Docker.Runtime, host.Runtime; got != want {
		t.Fatalf("runtime = %q, want %q", got, want)
	}
	if got, want := resource.Docker.Hostname, host.Hostname; got != want {
		t.Fatalf("hostname = %q, want %q", got, want)
	}
	if got, want := resource.Docker.RuntimeVersion, host.RuntimeVersion; got != want {
		t.Fatalf("runtimeVersion = %q, want %q", got, want)
	}
	if got, want := resource.Docker.ImageID, container.ImageDigest; got != want {
		t.Fatalf("imageId = %q, want %q", got, want)
	}
}

func TestResourceFromHostProjectsAgentHostProfile(t *testing.T) {
	host := models.Host{
		ID:        "tower-host",
		Hostname:  "tower",
		Platform:  "slackware",
		OSName:    "Unraid",
		OSVersion: "7.1.0",
		Status:    "online",
	}

	resource, _ := resourceFromHost(host)
	if resource.Technology != "linux" {
		t.Fatalf("Technology = %q, want linux runtime platform", resource.Technology)
	}
	if resource.Agent == nil {
		t.Fatal("expected agent payload")
	}
	if resource.Agent.Platform != "linux" {
		t.Fatalf("agent platform = %q, want linux runtime platform", resource.Agent.Platform)
	}
	if resource.Agent.HostProfile != "unraid" {
		t.Fatalf("agent host profile = %q, want unraid", resource.Agent.HostProfile)
	}
}

func TestResourceFromVMPreservesProxmoxPool(t *testing.T) {
	vm := models.VM{
		ID:       "cluster-a:pve-a:101",
		Name:     "app-vm",
		Node:     "pve-a",
		Pool:     "prod-vms",
		Instance: "cluster-a",
		VMID:     101,
		Status:   "running",
	}

	resource, _ := resourceFromVM(vm)
	if resource.Proxmox == nil {
		t.Fatal("expected proxmox payload")
	}
	if got, want := resource.Proxmox.Pool, "prod-vms"; got != want {
		t.Fatalf("pool = %q, want %q", got, want)
	}
}

func TestResourceFromContainerPreservesProxmoxPool(t *testing.T) {
	container := models.Container{
		ID:       "cluster-a:pve-a:202",
		Name:     "cache-ct",
		Node:     "pve-a",
		Pool:     "ops-lxc",
		Instance: "cluster-a",
		VMID:     202,
		Status:   "running",
		Type:     "lxc",
	}

	resource, _ := resourceFromContainer(container)
	if resource.Proxmox == nil {
		t.Fatal("expected proxmox payload")
	}
	if got, want := resource.Proxmox.Pool, "ops-lxc"; got != want {
		t.Fatalf("pool = %q, want %q", got, want)
	}
}

func TestResourceFromStorageIncludesStorageMetadata(t *testing.T) {
	storage := models.Storage{
		ID:       "storage-1",
		Name:     "ceph-rbd-pool",
		Node:     "pve-1",
		Instance: "cluster-a",
		Type:     "RBD",
		Pool:     "ceph/rbd-a",
		Content:  "images, rootdir,images",
		Shared:   true,
		Status:   "available",
		Enabled:  true,
		Active:   true,
		Total:    100,
		Used:     20,
		Free:     80,
		Usage:    20,
	}

	resource, _ := resourceFromStorage(storage)
	if resource.Storage == nil {
		t.Fatal("expected storage metadata payload")
	}
	if got, want := resource.Storage.Type, "rbd"; got != want {
		t.Fatalf("storage type = %q, want %q", got, want)
	}
	if got, want := resource.Storage.Content, "images, rootdir,images"; got != want {
		t.Fatalf("storage content = %q, want %q", got, want)
	}
	if got, want := resource.Storage.Pool, "ceph/rbd-a"; got != want {
		t.Fatalf("storage pool = %q, want %q", got, want)
	}
	wantContentTypes := []string{"images", "rootdir"}
	if len(resource.Storage.ContentTypes) != len(wantContentTypes) {
		t.Fatalf("contentTypes length = %d, want %d (%v)", len(resource.Storage.ContentTypes), len(wantContentTypes), resource.Storage.ContentTypes)
	}
	for i, want := range wantContentTypes {
		if got := resource.Storage.ContentTypes[i]; got != want {
			t.Fatalf("contentTypes[%d] = %q, want %q", i, got, want)
		}
	}
	if !resource.Storage.Shared {
		t.Fatalf("expected shared=true")
	}
	if !resource.Storage.IsCeph {
		t.Fatalf("expected isCeph=true")
	}
	if resource.Storage.IsZFS {
		t.Fatalf("expected isZfs=false")
	}
	if resource.Proxmox == nil || resource.Proxmox.NodeName != "pve-1" || resource.Proxmox.Instance != "cluster-a" {
		t.Fatalf("expected existing proxmox mapping to remain populated, got %+v", resource.Proxmox)
	}
}

func TestResourceFromStorageDetectsZFSMetadata(t *testing.T) {
	storage := models.Storage{
		ID:       "storage-zfs",
		Name:     "local-zfs",
		Node:     "pve-2",
		Instance: "cluster-a",
		Type:     "zfspool",
		Content:  "images",
		Shared:   false,
		Status:   "available",
		Enabled:  true,
		Active:   true,
		ZFSPool: &models.ZFSPool{
			Name:  "rpool",
			State: "ONLINE",
		},
	}

	resource, _ := resourceFromStorage(storage)
	if resource.Storage == nil {
		t.Fatal("expected storage metadata payload")
	}
	if !resource.Storage.IsZFS {
		t.Fatalf("expected isZfs=true")
	}
	if resource.Storage.IsCeph {
		t.Fatalf("expected isCeph=false")
	}
	if resource.Storage.Shared {
		t.Fatalf("expected shared=false")
	}
}

func TestResourceFromStorageDerivesZFSTopologyRisk(t *testing.T) {
	storage := models.Storage{
		ID:       "storage-zfs-risk",
		Name:     "tank",
		Node:     "pve-2",
		Instance: "cluster-a",
		Type:     "zfspool",
		Content:  "images",
		Shared:   false,
		Status:   "available",
		Enabled:  true,
		Active:   true,
		ZFSPool: &models.ZFSPool{
			Name:           "tank",
			State:          "ONLINE",
			ChecksumErrors: 2,
		},
	}

	resource, _ := resourceFromStorage(storage)
	if resource.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", resource.Status, StatusWarning)
	}
	if resource.Storage == nil || resource.Storage.Risk == nil {
		t.Fatalf("expected zfs topology risk payload, got %+v", resource.Storage)
	}
	if resource.Storage.Risk.Level != storagehealth.RiskWarning {
		t.Fatalf("risk level = %q, want %q", resource.Storage.Risk.Level, storagehealth.RiskWarning)
	}
}

func TestResourceFromPBSDatastoreDerivesRiskAndIncidents(t *testing.T) {
	now := time.Now().UTC()
	instance := models.PBSInstance{
		ID:       "pbs-1",
		Name:     "pbs-main",
		Host:     "https://pbs-main.local:8007",
		LastSeen: now,
	}
	datastore := models.PBSDatastore{
		Name:   "backup-store",
		Status: "online",
		Total:  100,
		Used:   96,
	}

	resource, _ := resourceFromPBSDatastore(instance, datastore)
	if resource.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", resource.Status, StatusWarning)
	}
	if resource.Storage == nil || resource.Storage.Risk == nil {
		t.Fatalf("expected PBS datastore risk payload, got %+v", resource.Storage)
	}
	if resource.Storage.Risk.Level != storagehealth.RiskCritical {
		t.Fatalf("risk level = %q, want %q", resource.Storage.Risk.Level, storagehealth.RiskCritical)
	}
	if len(resource.Incidents) != 1 {
		t.Fatalf("expected 1 datastore incident, got %+v", resource.Incidents)
	}
	if resource.Incidents[0].Code != "capacity_runway_low" {
		t.Fatalf("incident code = %q, want capacity_runway_low", resource.Incidents[0].Code)
	}
	if resource.Incidents[0].Source != "pbs" {
		t.Fatalf("incident source = %q, want pbs", resource.Incidents[0].Source)
	}
}

func TestResourceFromPBSInstanceRollsUpDatastoreRisk(t *testing.T) {
	now := time.Now().UTC()
	instance := models.PBSInstance{
		ID:       "pbs-1",
		Name:     "pbs-main",
		Host:     "https://pbs-main.local:8007",
		Status:   "online",
		LastSeen: now,
		Datastores: []models.PBSDatastore{
			{
				Name:   "backup-store",
				Status: "online",
				Total:  100,
				Used:   96,
			},
		},
	}

	resource, _ := resourceFromPBSInstance(instance)
	if resource.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", resource.Status, StatusWarning)
	}
	if resource.PBS == nil || resource.PBS.StorageRisk == nil {
		t.Fatalf("expected PBS storage risk payload, got %+v", resource.PBS)
	}
	if resource.PBS.StorageRisk.Level != storagehealth.RiskCritical {
		t.Fatalf("storage risk level = %q, want %q", resource.PBS.StorageRisk.Level, storagehealth.RiskCritical)
	}
	if len(resource.Incidents) != 1 {
		t.Fatalf("expected 1 rolled-up PBS incident, got %+v", resource.Incidents)
	}
	if resource.Incidents[0].Code != "capacity_runway_low" {
		t.Fatalf("incident code = %q, want capacity_runway_low", resource.Incidents[0].Code)
	}
}

func TestResourceFromHostUnraidStorageIncludesTopologyMetadata(t *testing.T) {
	host := models.Host{
		ID:          "tower-host",
		Hostname:    "tower",
		DisplayName: "Tower",
		Status:      "online",
		LastSeen:    time.Now().UTC(),
		Disks: []models.Disk{
			{Mountpoint: "/mnt/user", Total: 1000, Used: 400, Free: 600, Usage: 40},
		},
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			SyncAction:   "check",
			SyncProgress: 22,
			NumProtected: 1,
			Disks: []models.HostUnraidDisk{
				{Name: "parity", Role: "parity", Status: "online"},
				{Name: "disk1", Role: "data", Status: "online"},
			},
		},
	}

	resource, identity := resourceFromHostUnraidStorage(host)
	if resource.Type != ResourceTypeStorage {
		t.Fatalf("Type = %q, want %q", resource.Type, ResourceTypeStorage)
	}
	if resource.Storage == nil {
		t.Fatal("expected storage metadata payload")
	}
	if got := resource.Storage.Type; got != "unraid-array" {
		t.Fatalf("storage type = %q, want unraid-array", got)
	}
	if got := resource.Storage.Platform; got != "unraid" {
		t.Fatalf("platform = %q, want unraid", got)
	}
	if got := resource.Storage.Protection; got != "single-parity" {
		t.Fatalf("protection = %q, want single-parity", got)
	}
	if resource.Metrics == nil || resource.Metrics.Disk == nil || resource.Metrics.Disk.Percent != 40 {
		t.Fatalf("expected disk metrics from /mnt/user, got %+v", resource.Metrics)
	}
	if got := identity.MachineID; got != "" && !strings.Contains(got, "/storage/unraid-array") {
		t.Fatalf("expected unraid storage identity suffix, got %q", got)
	}
}

func TestResourceFromHostDerivesStorageTopologyRisk(t *testing.T) {
	host := models.Host{
		ID:       "tower-host",
		Hostname: "tower",
		Status:   "online",
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			Disks: []models.HostUnraidDisk{
				{Name: "parity", Role: "parity", Status: "disabled"},
				{Name: "disk1", Role: "data", Status: "online"},
			},
		},
	}

	resource, _ := resourceFromHost(host)
	if resource.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", resource.Status, StatusWarning)
	}
	if resource.Agent == nil || resource.Agent.StorageRisk == nil {
		t.Fatalf("expected host storage risk payload, got %+v", resource.Agent)
	}
	if resource.Agent.StorageRisk.Level != storagehealth.RiskCritical {
		t.Fatalf("risk level = %q, want %q", resource.Agent.StorageRisk.Level, storagehealth.RiskCritical)
	}
	if resource.Agent.StorageRiskSummary != "Unraid parity disk parity is DISABLED" {
		t.Fatalf("storage risk summary = %q", resource.Agent.StorageRiskSummary)
	}
	if resource.Agent.StoragePostureSummary != "Unraid parity disk parity is DISABLED" {
		t.Fatalf("storage posture summary = %q", resource.Agent.StoragePostureSummary)
	}
	if !resource.Agent.ProtectionReduced || resource.Agent.ProtectionSummary != "Unraid parity disk parity is DISABLED" {
		t.Fatalf("protection semantics = reduced:%v summary:%q", resource.Agent.ProtectionReduced, resource.Agent.ProtectionSummary)
	}
	if resource.Agent.Unraid == nil || resource.Agent.Unraid.Risk == nil {
		t.Fatalf("expected unraid risk payload, got %+v", resource.Agent.Unraid)
	}
	if resource.Agent.Unraid.RiskSummary != "Unraid parity disk parity is DISABLED" || resource.Agent.Unraid.PostureSummary != "Unraid parity disk parity is DISABLED" {
		t.Fatalf("unexpected unraid summaries %+v", resource.Agent.Unraid)
	}
}

func TestResourceFromHostSMARTDiskCarriesUnraidRole(t *testing.T) {
	host := models.Host{
		ID:       "tower-host",
		Hostname: "tower",
		LastSeen: time.Now().UTC(),
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			Disks: []models.HostUnraidDisk{
				{
					Name:       "parity",
					Device:     "/dev/sdb",
					Role:       "parity",
					Status:     "online",
					Model:      "Seagate",
					Serial:     "PARITY-1",
					SizeBytes:  6_000_000_000_000,
					SpunDown:   true,
					ReadCount:  11,
					WriteCount: 12,
					ErrorCount: 16,
				},
			},
		},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "sdb [sat]", Serial: "PARITY-1"},
			},
		},
	}

	resource, _ := resourceFromHostSMARTDisk(host, host.Sensors.SMART[0])
	if resource.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if resource.PhysicalDisk.StorageRole != "parity" {
		t.Fatalf("storageRole = %q, want parity", resource.PhysicalDisk.StorageRole)
	}
	if resource.PhysicalDisk.StorageGroup != "unraid-array" {
		t.Fatalf("storageGroup = %q, want unraid-array", resource.PhysicalDisk.StorageGroup)
	}
	if resource.PhysicalDisk.SizeBytes != 6_000_000_000_000 {
		t.Fatalf("sizeBytes = %d, want unraid disk size", resource.PhysicalDisk.SizeBytes)
	}
	if resource.PhysicalDisk.Health != "PASSED" {
		t.Fatalf("health = %q, want PASSED from Unraid online state", resource.PhysicalDisk.Health)
	}
	if !resource.PhysicalDisk.SpunDown || resource.PhysicalDisk.ReadCount != 11 || resource.PhysicalDisk.WriteCount != 12 || resource.PhysicalDisk.ErrorCount != 16 {
		t.Fatalf("expected native Unraid disk counters, got %+v", resource.PhysicalDisk)
	}
}

func TestResourceFromHostSMARTDiskInfersUnraidArrayRole(t *testing.T) {
	host := models.Host{
		ID:       "tower-host",
		Hostname: "tower",
		LastSeen: time.Now().UTC(),
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			Disks: []models.HostUnraidDisk{
				{
					Name:      "md1p1",
					Device:    "/dev/sde",
					Status:    "online",
					SizeBytes: 6_000_000_000_000,
					Slot:      1,
				},
			},
		},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "sde [sat]", Health: "UNKNOWN"},
			},
		},
	}

	resource, _ := resourceFromHostSMARTDisk(host, host.Sensors.SMART[0])
	if resource.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if resource.PhysicalDisk.StorageRole != "data" {
		t.Fatalf("storageRole = %q, want data", resource.PhysicalDisk.StorageRole)
	}
	if resource.PhysicalDisk.StorageGroup != "unraid-array" {
		t.Fatalf("storageGroup = %q, want unraid-array", resource.PhysicalDisk.StorageGroup)
	}
	if resource.PhysicalDisk.SizeBytes != 6_000_000_000_000 {
		t.Fatalf("sizeBytes = %d, want unraid disk size", resource.PhysicalDisk.SizeBytes)
	}
	if resource.PhysicalDisk.Health != "PASSED" {
		t.Fatalf("health = %q, want PASSED from Unraid online state", resource.PhysicalDisk.Health)
	}
}

func TestResourceFromHostSMARTDiskNormalizesLegacyUnraidKiBSize(t *testing.T) {
	const legacyDiskKiB = int64(5_860_522_532)
	const normalizedDiskBytes = legacyDiskKiB * 1024
	host := models.Host{
		ID:       "tower-host",
		Hostname: "tower",
		LastSeen: time.Now().UTC(),
		Disks: []models.Disk{
			{
				Mountpoint: "/mnt/user",
				Total:      normalizedDiskBytes * 4,
				Used:       normalizedDiskBytes * 3,
				Free:       normalizedDiskBytes,
				Usage:      75,
			},
		},
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			Disks: []models.HostUnraidDisk{
				{Name: "md1p1", Device: "/dev/sde", Status: "online", SizeBytes: legacyDiskKiB, Slot: 1},
				{Name: "md2p1", Device: "/dev/sdd", Status: "online", SizeBytes: legacyDiskKiB, Slot: 2},
				{Name: "md3p1", Device: "/dev/sdb", Status: "online", SizeBytes: legacyDiskKiB, Slot: 3},
				{Name: "md4p1", Device: "/dev/sdc", Status: "online", SizeBytes: legacyDiskKiB, Slot: 4},
			},
		},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "sde [sat]", Health: "UNKNOWN"},
			},
		},
	}

	resource, _ := resourceFromHostSMARTDisk(host, host.Sensors.SMART[0])
	if resource.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if resource.PhysicalDisk.SizeBytes != normalizedDiskBytes {
		t.Fatalf("sizeBytes = %d, want normalized Unraid bytes %d", resource.PhysicalDisk.SizeBytes, normalizedDiskBytes)
	}

	agentResource, _ := resourceFromHost(host)
	if agentResource.Agent == nil || agentResource.Agent.Unraid == nil || len(agentResource.Agent.Unraid.Disks) == 0 {
		t.Fatalf("expected agent Unraid metadata, got %+v", agentResource.Agent)
	}
	if got := agentResource.Agent.Unraid.Disks[0].SizeBytes; got != normalizedDiskBytes {
		t.Fatalf("agent Unraid disk sizeBytes = %d, want %d", got, normalizedDiskBytes)
	}
}

func TestResourceFromKubernetesDeployment_PopulatesMetricsUnderMockMode(t *testing.T) {
	mockruntime.SetEnabled(true)
	t.Cleanup(func() { mockruntime.SetEnabled(false) })

	cluster := models.KubernetesCluster{
		ID:   "cluster-1",
		Name: "production",
	}
	deployment := models.KubernetesDeployment{
		UID:                "dep-uid-1",
		Name:               "checkout-api",
		Namespace:          "services",
		CreatedAt:          time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC),
		DesiredReplicas:    3,
		UpdatedReplicas:    3,
		ReadyReplicas:      2,
		AvailableReplicas:  2,
		ObservedGeneration: 12,
	}

	resource, _ := resourceFromKubernetesDeployment(cluster, deployment, nil)
	if resource.Kubernetes == nil {
		t.Fatal("expected kubernetes payload")
	}
	if resource.Kubernetes.ResourceKind != "Deployment" || resource.Kubernetes.ResourceUID != "dep-uid-1" {
		t.Fatalf("deployment resource identity fields = kind %q uid %q", resource.Kubernetes.ResourceKind, resource.Kubernetes.ResourceUID)
	}
	if resource.Kubernetes.ObservedGeneration != 12 {
		t.Fatalf("observedGeneration = %d, want 12", resource.Kubernetes.ObservedGeneration)
	}
	if resource.Kubernetes.CreatedAt == nil || !resource.Kubernetes.CreatedAt.Equal(deployment.CreatedAt) {
		t.Fatalf("createdAt = %+v, want %s", resource.Kubernetes.CreatedAt, deployment.CreatedAt)
	}
	if resource.Metrics == nil {
		t.Fatal("expected mock-mode deployment to carry synthetic Metrics, got nil")
	}
	if resource.Metrics.CPU == nil || resource.Metrics.CPU.Percent <= 0 {
		t.Fatalf("expected non-zero CPU percent, got %+v", resource.Metrics.CPU)
	}
	if resource.Metrics.Memory == nil || resource.Metrics.Memory.Percent <= 0 {
		t.Fatalf("expected non-zero Memory percent, got %+v", resource.Metrics.Memory)
	}
	if resource.Metrics.Disk == nil || resource.Metrics.Disk.Percent < 0 {
		t.Fatalf("expected Disk metric, got %+v", resource.Metrics.Disk)
	}
}

func TestResourceFromKubernetesDeployment_NilMetricsOutsideMockMode(t *testing.T) {
	mockruntime.SetEnabled(false)
	cluster := models.KubernetesCluster{ID: "cluster-2", Name: "live"}
	deployment := models.KubernetesDeployment{
		Name: "frontend", Namespace: "web", DesiredReplicas: 2, ReadyReplicas: 2, AvailableReplicas: 2,
	}
	resource, _ := resourceFromKubernetesDeployment(cluster, deployment, nil)
	if resource.Metrics != nil {
		t.Fatalf("expected nil Metrics outside mock mode (no canonical aggregation yet), got %+v", resource.Metrics)
	}
}
