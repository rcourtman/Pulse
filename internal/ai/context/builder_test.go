package context

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder()
	if builder == nil {
		t.Fatal("Expected non-nil builder")
	}
}

func TestBuilder_WithMethods(t *testing.T) {
	builder := NewBuilder()

	// Test chaining methods return the builder
	result := builder.
		WithMetricsHistory(nil).
		WithKnowledge(nil).
		WithFindings(nil).
		WithBaseline(nil)

	if result != builder {
		t.Error("Expected method chaining to return same builder instance")
	}
}

func TestBuilder_BuildForInfrastructure_Empty(t *testing.T) {
	builder := NewBuilder()

	// Empty state
	state := models.StateSnapshot{}

	ctx := builder.BuildForInfrastructure(state)

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	// Empty state should have zero totals
	if ctx.TotalResources != 0 {
		t.Errorf("Expected 0 total resources for empty state, got %d", ctx.TotalResources)
	}

	// GeneratedAt should be set
	if ctx.GeneratedAt.IsZero() {
		t.Error("Expected GeneratedAt to be set")
	}
}

func TestBuilder_BuildForInfrastructure_WithNodes(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Name:   "pve-primary",
				Status: "online",
				CPU:    0.45,
			},
			{
				ID:     "node-2",
				Name:   "pve-secondary",
				Status: "online",
				CPU:    0.25,
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(ctx.Nodes))
	}
}

func TestBuilder_BuildForInfrastructure_WithGuests(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "pve-primary", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "vm-100", Name: "web-server", Node: "pve-primary", Status: "running", CPU: 0.30},
			{ID: "vm-101", Name: "database", Node: "pve-primary", Status: "running", CPU: 0.50},
		},
		Containers: []models.Container{
			{ID: "ct-200", Name: "nginx-proxy", Node: "pve-primary", Status: "running", CPU: 0.10},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.VMs) != 2 {
		t.Errorf("Expected 2 VMs, got %d", len(ctx.VMs))
	}

	if len(ctx.Containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(ctx.Containers))
	}
}

func TestBuilder_BuildForInfrastructure_SkipsTemplates(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-100", Name: "web-server", Status: "running", Template: false},
			{ID: "vm-101", Name: "template", Status: "stopped", Template: true},
		},
		Containers: []models.Container{
			{ID: "ct-200", Name: "nginx", Status: "running", Template: false},
			{ID: "ct-201", Name: "template", Status: "stopped", Template: true},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	// Should skip templates
	if len(ctx.VMs) != 1 {
		t.Errorf("Expected 1 VM (template skipped), got %d", len(ctx.VMs))
	}

	if len(ctx.Containers) != 1 {
		t.Errorf("Expected 1 container (template skipped), got %d", len(ctx.Containers))
	}
}

func TestBuilder_BuildForInfrastructure_WithStorage(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Storage: []models.Storage{
			{
				ID:     "storage-1",
				Name:   "local-zfs",
				Type:   "zfspool",
				Status: "available",
			},
			{
				ID:     "storage-2",
				Name:   "nfs-share",
				Type:   "nfs",
				Status: "available",
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.Storage) != 2 {
		t.Errorf("Expected 2 storage, got %d", len(ctx.Storage))
	}
}

func TestBuilder_BuildForInfrastructure_WithDocker(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "docker-1",
				Hostname: "docker-host-1",
				Containers: []models.DockerContainer{
					{ID: "container-1", Name: "nginx", State: "running"},
					{ID: "container-2", Name: "redis", State: "running"},
				},
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.DockerHosts) != 1 {
		t.Errorf("Expected 1 docker host, got %d", len(ctx.DockerHosts))
	}
}

func TestBuilder_BuildForInfrastructure_WithHosts(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-1",
				Hostname: "server-1",
				Status:   "online",
				CPUCount: 8,
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.Hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(ctx.Hosts))
	}
}

func TestBuilder_BuildForInfrastructure_TotalResources(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "pve-1", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "vm-100", Name: "web", Status: "running"},
			{ID: "vm-101", Name: "db", Status: "running"},
		},
		Containers: []models.Container{
			{ID: "ct-200", Name: "nginx", Status: "running"},
		},
		Storage: []models.Storage{
			{ID: "local", Name: "local", Status: "available"},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	// 1 node + 2 VMs + 1 container + 1 storage = 5
	expected := 5
	if ctx.TotalResources != expected {
		t.Errorf("Expected %d total resources, got %d", expected, ctx.TotalResources)
	}
}

func TestBuilder_BuildForInfrastructure_OCI(t *testing.T) {
	builder := NewBuilder()

	state := models.StateSnapshot{
		Containers: []models.Container{
			{
				ID:         "ct-200",
				Name:       "nginx-oci",
				Status:     "running",
				IsOCI:      true,
				OSTemplate: "docker.io/library/nginx:latest",
			},
		},
	}

	ctx := builder.BuildForInfrastructure(state)

	if len(ctx.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(ctx.Containers))
	}

	// OCI container should have type oci_container
	if ctx.Containers[0].ResourceType != "oci_container" {
		t.Errorf("Expected resource type 'oci_container', got '%s'", ctx.Containers[0].ResourceType)
	}

	// Should have metadata with image
	if ctx.Containers[0].Metadata == nil {
		t.Error("Expected metadata to be set for OCI container")
	} else if ctx.Containers[0].Metadata["oci_image"] != "docker.io/library/nginx:latest" {
		t.Errorf("Expected oci_image metadata, got %v", ctx.Containers[0].Metadata)
	}
}

func TestBuilder_MergeContexts(t *testing.T) {
	builder := NewBuilder()

	target := &ResourceContext{
		ResourceID:   "vm-100",
		ResourceType: "vm",
		ResourceName: "web-server",
		Status:       "running",
		Node:         "pve-1",
	}

	infra := &InfrastructureContext{
		TotalResources: 10,
		GeneratedAt:    time.Now(),
		VMs: []ResourceContext{
			{ResourceID: "vm-100", ResourceName: "web-server", Node: "pve-1"},
			{ResourceID: "vm-101", ResourceName: "database", Node: "pve-1"},
		},
	}

	result := builder.MergeContexts(target, infra)

	if result == "" {
		t.Error("Expected non-empty merged context")
	}

	// Should contain target resource section
	if !containsSubstring(result, "Target Resource") {
		t.Error("Expected merged context to contain 'Target Resource' section")
	}
}

func TestBuilder_MergeContexts_IncludesRelated(t *testing.T) {
	builder := NewBuilder()

	target := &ResourceContext{
		ResourceID:   "vm-100",
		ResourceType: "vm",
		ResourceName: "web-server",
		Node:         "pve-1",
	}

	infra := &InfrastructureContext{
		VMs: []ResourceContext{
			{ResourceID: "vm-100", ResourceName: "web-server", Node: "pve-1"},
			{ResourceID: "vm-101", ResourceName: "database", Node: "pve-1"},
			{ResourceID: "vm-102", ResourceName: "other", Node: "pve-2"}, // Different node
		},
	}

	result := builder.MergeContexts(target, infra)

	// Should include related resources section
	if !containsSubstring(result, "Related Resources") {
		t.Error("Expected merged context to contain 'Related Resources' section when target has a node")
	}
}

func TestResourceContext_Fields(t *testing.T) {
	ctx := ResourceContext{
		ResourceID:    "vm-100",
		ResourceType:  "vm",
		ResourceName:  "test-vm",
		Node:          "node-1",
		CurrentCPU:    0.45,
		CurrentMemory: 0.65,
		CurrentDisk:   0.30,
		Status:        "running",
	}

	if ctx.ResourceID != "vm-100" {
		t.Errorf("Expected ResourceID 'vm-100', got '%s'", ctx.ResourceID)
	}

	if ctx.CurrentCPU != 0.45 {
		t.Errorf("Expected CurrentCPU 0.45, got %f", ctx.CurrentCPU)
	}

	if ctx.Node != "node-1" {
		t.Errorf("Expected Node 'node-1', got '%s'", ctx.Node)
	}
}

func TestInfrastructureContext_Fields(t *testing.T) {
	now := time.Now()
	ctx := InfrastructureContext{
		GeneratedAt:       now,
		TotalResources:    25,
		ResourcesWithData: 20,
	}

	if ctx.TotalResources != 25 {
		t.Errorf("Expected 25 total resources, got %d", ctx.TotalResources)
	}

	if ctx.GeneratedAt != now {
		t.Error("Expected GeneratedAt to match")
	}

	if ctx.ResourcesWithData != 20 {
		t.Errorf("Expected 20 resources with data, got %d", ctx.ResourcesWithData)
	}
}

// Helper function
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
