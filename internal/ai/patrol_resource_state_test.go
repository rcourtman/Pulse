package ai

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestGetResourceCurrentState(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	state := models.StateSnapshot{
		Storage:    []models.Storage{{ID: "storage-1", Name: "local", Usage: 42, Status: "active"}},
		Nodes:      []models.Node{{ID: "node-1", Name: "node-1", CPU: 0.2, Memory: models.Memory{Usage: 33}, Status: "online"}},
		VMs:        []models.VM{{ID: "vm-1", Name: "vm-1", CPU: 0.1, Memory: models.Memory{Usage: 44}, Status: "running"}},
		Containers: []models.Container{{ID: "ct-1", Name: "ct-1", CPU: 0.3, Memory: models.Memory{Usage: 55}, Status: "running"}},
		DockerHosts: []models.DockerHost{{
			ID:         "docker-1",
			Hostname:   "dockhost",
			Status:     "online",
			Containers: []models.DockerContainer{{ID: "dc-1", Name: "web", CPUPercent: 12.3, MemoryPercent: 45.6, State: "running"}},
		}},
	}

	cases := []struct {
		alert  AlertInfo
		assert string
	}{
		{alert: AlertInfo{ResourceType: "Storage", ResourceID: "storage-1", ResourceName: "local"}, assert: "Storage 'local'"},
		{alert: AlertInfo{ResourceType: "node", ResourceID: "node-1", ResourceName: "node-1"}, assert: "Node 'node-1'"},
		{alert: AlertInfo{ResourceType: "vm", ResourceID: "vm-1", ResourceName: "vm-1"}, assert: "VM 'vm-1'"},
		{alert: AlertInfo{ResourceType: "container", ResourceID: "ct-1", ResourceName: "ct-1"}, assert: "Container 'ct-1'"},
		{alert: AlertInfo{ResourceType: "docker", ResourceID: "dc-1", ResourceName: "web"}, assert: "Docker container 'web'"},
		{alert: AlertInfo{ResourceType: "unknown", ResourceID: "x"}, assert: "Resource state unknown"},
	}

	for _, tc := range cases {
		stateText := ps.getResourceCurrentState(tc.alert, state)
		if !strings.Contains(stateText, tc.assert) {
			t.Fatalf("expected %q in state text, got %q", tc.assert, stateText)
		}
	}
}

func TestGetCurrentMetricValue_DockerAndStorage(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "storage-1", Name: "local", Usage: 77}},
		DockerHosts: []models.DockerHost{{
			Containers: []models.DockerContainer{{ID: "dc-1", Name: "web", CPUPercent: 12.3, MemoryPercent: 45.6}},
		}},
	}

	storageAlert := AlertInfo{ResourceType: "Storage", ResourceID: "storage-1"}
	if got := ps.getCurrentMetricValue(storageAlert, state); got != 77 {
		t.Fatalf("expected storage usage 77, got %.1f", got)
	}

	dockerCPU := AlertInfo{ResourceType: "docker", ResourceID: "dc-1", Type: "cpu"}
	if got := ps.getCurrentMetricValue(dockerCPU, state); got != 12.3 {
		t.Fatalf("expected docker cpu 12.3, got %.1f", got)
	}

	dockerMem := AlertInfo{ResourceType: "docker", ResourceID: "dc-1", Type: "memory"}
	if got := ps.getCurrentMetricValue(dockerMem, state); got != 45.6 {
		t.Fatalf("expected docker memory 45.6, got %.1f", got)
	}
}
