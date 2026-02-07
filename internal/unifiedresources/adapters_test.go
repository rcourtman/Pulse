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

	resource, _ := resourceFromDockerContainer(container)
	if resource.Docker == nil {
		t.Fatal("expected docker payload")
	}
	if got, want := resource.Docker.ContainerID, container.ID; got != want {
		t.Fatalf("containerId = %q, want %q", got, want)
	}
}
