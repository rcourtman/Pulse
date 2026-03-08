package unifiedresources

import (
	"strings"
	"testing"
	"time"

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

func TestResourceFromDockerContainerIncludesContainerID(t *testing.T) {
	container := models.DockerContainer{
		ID:            "aurora-3-abcdef123456",
		Name:          "web",
		State:         "running",
		Image:         "ghcr.io/example/web:1.0.0",
		UptimeSeconds: 1234,
		CPUPercent:    12.5,
	}

	host := models.DockerHost{
		ID:       "docker-1",
		Hostname: "docker-1",
		Runtime:  "podman",
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
}

func TestResourceFromStorageIncludesStorageMetadata(t *testing.T) {
	storage := models.Storage{
		ID:       "storage-1",
		Name:     "ceph-rbd-pool",
		Node:     "pve-1",
		Instance: "cluster-a",
		Type:     "RBD",
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
	if resource.Agent.Unraid == nil || resource.Agent.Unraid.Risk == nil {
		t.Fatalf("expected unraid risk payload, got %+v", resource.Agent.Unraid)
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
				{Name: "parity", Device: "/dev/sdb", Role: "parity", Status: "online", Serial: "PARITY-1"},
			},
		},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "/dev/sdb", Serial: "PARITY-1", Model: "Seagate"},
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
}
