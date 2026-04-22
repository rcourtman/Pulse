package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestBuildConnectionSystems_AttachesMergedAgentToOwningProxmoxSource(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	adapter := unified.NewMonitorAdapter(nil)
	adapter.PopulateSupplementalRecords(unified.SourceProxmox, []unified.IngestRecord{
		{
			SourceID: "node-pi",
			Resource: unified.Resource{
				ID:       "resource-node-pi",
				Type:     unified.ResourceTypeAgent,
				Name:     "pi",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourceProxmox, unified.SourceAgent},
				Identity: unified.ResourceIdentity{
					MachineID: "machine-pi",
					Hostnames: []string{"pi"},
				},
				Proxmox: &unified.ProxmoxData{
					Instance:      "pi",
					NodeName:      "pi",
					LinkedAgentID: "agent-pi",
				},
				Agent: &unified.AgentData{
					AgentID:      "agent-pi",
					Hostname:     "pi",
					MachineID:    "machine-pi",
					LinkedNodeID: "node-pi",
				},
			},
		},
	})
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	connections := []Connection{
		{
			ID:           "pve:pi",
			Type:         ConnectionTypePVE,
			Name:         "pi",
			Address:      "https://192.168.0.2:8006",
			State:        ConnectionStateActive,
			Enabled:      true,
			Surfaces:     []string{"vms", "containers", "storage", "backups"},
			Scope:        map[string]bool{"vms": true, "containers": true, "storage": true, "backups": true},
			Source:       ConnectionSourceAgent,
			Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
		},
		{
			ID:           "agent:agent-pi",
			Type:         ConnectionTypeAgent,
			Name:         "pi",
			Address:      "pi",
			State:        ConnectionStateActive,
			Enabled:      true,
			Surfaces:     []string{"host"},
			Scope:        map[string]bool{"host": true},
			LastSeen:     &now,
			Source:       ConnectionSourceAgent,
			Capabilities: ConnectionCapabilities{SupportsPause: false, SupportsScope: false, SupportsTest: false},
		},
	}

	systems := buildConnectionSystems(connections, monitor)
	if len(systems) != 1 {
		t.Fatalf("expected 1 grouped system, got %d (%+v)", len(systems), systems)
	}

	system := systems[0]
	if system.ID != "pve:pi" {
		t.Fatalf("system id = %q, want %q", system.ID, "pve:pi")
	}
	if system.Type != ConnectionTypePVE {
		t.Fatalf("system type = %q, want %q", system.Type, ConnectionTypePVE)
	}
	if len(system.Components) != 2 {
		t.Fatalf("expected 2 system components, got %+v", system.Components)
	}
	if system.Components[0].ConnectionID != "pve:pi" || system.Components[0].Role != ConnectionSystemComponentRolePrimary {
		t.Fatalf("unexpected primary component: %+v", system.Components[0])
	}
	if system.Components[1].ConnectionID != "agent:agent-pi" || system.Components[1].Role != ConnectionSystemComponentRoleAttachment {
		t.Fatalf("unexpected attachment component: %+v", system.Components[1])
	}
}
