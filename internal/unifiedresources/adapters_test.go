package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

	resource, _ := resourceFromProxmoxNode(node)
	if resource.Proxmox == nil {
		t.Fatal("expected proxmox payload")
	}
	if resource.Proxmox.Temperature == nil {
		t.Fatal("expected proxmox temperature to be populated")
	}
	if got, want := *resource.Proxmox.Temperature, 61.9; got != want {
		t.Fatalf("temperature = %v, want %v", got, want)
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
