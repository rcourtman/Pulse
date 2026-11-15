package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// On Docker fault set D6 a small model investigating a full mount on an agent
// host asked action=config for the host six times, with the types agent, none,
// node, system-container, vm and system. It was told "invalid resource_type:
// agent. Use vm or system-container" for types it had not sent, "resource_type
// is required" though the handle names its type, and "not found" for the guest
// types, and spent six of its ten evidence calls without reading the disk.
// action=config now reads the type from a canonical handle, and for a resource
// with no config says what config covers and where disk usage is read.
func TestGetResourceConfigPointsAHostAtTheReadTool(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: models.StateSnapshot{
			Containers: []models.Container{{ID: "ct1", VMID: 200, Name: "ct1", Status: "running", Node: "node1", Instance: "pve1"}},
		}},
		GuestConfigProvider: &mockGuestConfigProvider{config: map[string]interface{}{}},
	})
	const host = "agent-b2d7e93ee6fa2348"

	for _, resourceType := range []string{"agent", "", "node", "system", "vm", "system-container"} {
		t.Run("type "+resourceType, func(t *testing.T) {
			args := map[string]interface{}{"resource_id": host}
			if resourceType != "" {
				args["resource_type"] = resourceType
			}
			result, _ := executor.executeGetResourceConfig(context.Background(), args)
			if !result.IsError {
				t.Fatalf("config of an agent host must be refused, got %s", result.Content[0].Text)
			}
			text := result.Content[0].Text
			for _, want := range []string{
				`"agent-b2d7e93ee6fa2348" is an agent host`,
				"Proxmox VMs (vm), LXC containers (system-container) and Docker containers (app-container)",
				"use pulse_read (action disk_usage, file or logs) if that tool is offered to you",
				"use action=resource",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("refusal lacks %q:\n%s", want, text)
				}
			}
			for _, wrong := range []string{"invalid resource_type", "resource_type is required", "not found"} {
				if strings.Contains(text, wrong) {
					t.Errorf("refusal still says %q:\n%s", wrong, text)
				}
			}
		})
	}

	for resourceType, what := range map[string]string{"docker-host": "a Docker host", "storage": "a storage pool", "storage-pool": "a storage pool", "physical-disk": "a physical disk"} {
		t.Run(resourceType, func(t *testing.T) {
			result, _ := executor.executeGetResourceConfig(context.Background(), map[string]interface{}{"resource_type": resourceType, "resource_id": "local-zfs"})
			if !result.IsError || !strings.Contains(result.Content[0].Text, `"local-zfs" is `+what) {
				t.Fatalf("refusal for %s: %s", resourceType, result.Content[0].Text)
			}
		})
	}

	// A guest's own config is unchanged, and a name that only looks like a
	// handle ("<word>-<hex>") keeps the type the model passed.
	result, _ := executor.executeGetResourceConfig(context.Background(), map[string]interface{}{"resource_type": "system-container", "resource_id": "ct1"})
	if result.IsError {
		t.Fatalf("guest config: %s", result.Content[0].Text)
	}
	result, _ = executor.executeGetResourceConfig(context.Background(), map[string]interface{}{"resource_type": "system-container", "resource_id": "backup-20261115"})
	if !result.IsError || !strings.Contains(result.Content[0].Text, "system-container not found: backup-20261115") {
		t.Fatalf("a guest-like name must keep the passed type: %s", result.Content[0].Text)
	}
}
