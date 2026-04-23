package api

import (
	"reflect"
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

func TestBuildConnectionSystems_ClusterMemberAgentsAttachToOwningProxmoxSystem(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	adapter := unified.NewMonitorAdapter(nil)
	adapter.PopulateSupplementalRecords(unified.SourceProxmox, []unified.IngestRecord{
		{
			SourceID: "node-delly",
			Resource: unified.Resource{
				ID:       "resource-node-delly",
				Type:     unified.ResourceTypeAgent,
				Name:     "delly",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourceProxmox, unified.SourceAgent},
				Identity: unified.ResourceIdentity{
					MachineID:   "machine-delly",
					Hostnames:   []string{"delly"},
					IPAddresses: []string{"192.168.0.10"},
				},
				Proxmox: &unified.ProxmoxData{
					Instance:        "delly",
					NodeName:        "delly",
					ClusterName:     "homelab",
					IsClusterMember: true,
					HostURL:         "https://delly:8006",
					LinkedAgentID:   "agent-delly",
				},
				Agent: &unified.AgentData{
					AgentID:      "agent-delly",
					Hostname:     "delly",
					MachineID:    "machine-delly",
					LinkedNodeID: "node-delly",
				},
			},
		},
		{
			SourceID: "node-minipc",
			Resource: unified.Resource{
				ID:       "resource-node-minipc",
				Type:     unified.ResourceTypeAgent,
				Name:     "minipc",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourceProxmox, unified.SourceAgent},
				Identity: unified.ResourceIdentity{
					MachineID:   "machine-minipc",
					Hostnames:   []string{"minipc"},
					IPAddresses: []string{"192.168.0.11"},
				},
				Proxmox: &unified.ProxmoxData{
					Instance:        "delly",
					NodeName:        "minipc",
					ClusterName:     "homelab",
					IsClusterMember: true,
					HostURL:         "https://minipc:8006",
					LinkedAgentID:   "agent-minipc",
				},
				Agent: &unified.AgentData{
					AgentID:      "agent-minipc",
					Hostname:     "minipc",
					MachineID:    "machine-minipc",
					LinkedNodeID: "node-minipc",
				},
			},
		},
	})
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	connections := []Connection{
		{
			ID:           "pve:delly",
			Type:         ConnectionTypePVE,
			Name:         "delly",
			Address:      "https://delly:8006",
			State:        ConnectionStateActive,
			Enabled:      true,
			Surfaces:     []string{"vms", "containers", "storage", "backups"},
			Scope:        map[string]bool{"vms": true, "containers": true, "storage": true, "backups": true},
			Source:       ConnectionSourceAgent,
			Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
		},
		{
			ID:           "agent:agent-delly",
			Type:         ConnectionTypeAgent,
			Name:         "delly",
			Address:      "delly",
			State:        ConnectionStateActive,
			Enabled:      true,
			Surfaces:     []string{"host"},
			Scope:        map[string]bool{"host": true},
			LastSeen:     &now,
			Source:       ConnectionSourceAgent,
			Capabilities: ConnectionCapabilities{SupportsPause: false, SupportsScope: false, SupportsTest: false},
		},
		{
			ID:           "agent:agent-minipc",
			Type:         ConnectionTypeAgent,
			Name:         "minipc",
			Address:      "minipc",
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
		t.Fatalf("expected 1 grouped cluster system, got %d (%+v)", len(systems), systems)
	}

	system := systems[0]
	if system.ID != "pve:delly" {
		t.Fatalf("system id = %q, want %q", system.ID, "pve:delly")
	}
	if system.Type != ConnectionTypePVE {
		t.Fatalf("system type = %q, want %q", system.Type, ConnectionTypePVE)
	}
	if system.ClusterName != "homelab" {
		t.Fatalf("system clusterName = %q, want %q", system.ClusterName, "homelab")
	}
	if len(system.Components) != 3 {
		t.Fatalf("expected 3 system components, got %+v", system.Components)
	}
	if len(system.Members) != 2 {
		t.Fatalf("expected 2 system members, got %+v", system.Members)
	}

	componentRoles := make(map[string]ConnectionSystemComponentRole, len(system.Components))
	for _, component := range system.Components {
		componentRoles[component.ConnectionID] = component.Role
	}
	if componentRoles["pve:delly"] != ConnectionSystemComponentRolePrimary {
		t.Fatalf("pve:delly role = %q, want %q", componentRoles["pve:delly"], ConnectionSystemComponentRolePrimary)
	}
	if componentRoles["agent:agent-delly"] != ConnectionSystemComponentRoleAttachment {
		t.Fatalf("agent:agent-delly role = %q, want %q", componentRoles["agent:agent-delly"], ConnectionSystemComponentRoleAttachment)
	}
	if componentRoles["agent:agent-minipc"] != ConnectionSystemComponentRoleAttachment {
		t.Fatalf("agent:agent-minipc role = %q, want %q", componentRoles["agent:agent-minipc"], ConnectionSystemComponentRoleAttachment)
	}

	membersByName := make(map[string]ConnectionSystemMember, len(system.Members))
	for _, member := range system.Members {
		membersByName[member.Name] = member
	}

	dellyMember, ok := membersByName["delly"]
	if !ok {
		t.Fatalf("expected delly member, got %+v", system.Members)
	}
	if !dellyMember.Primary {
		t.Fatalf("expected delly to be the primary cluster member, got %+v", dellyMember)
	}
	if dellyMember.Endpoint != "https://delly:8006" {
		t.Fatalf("delly endpoint = %q, want %q", dellyMember.Endpoint, "https://delly:8006")
	}
	if !reflect.DeepEqual(dellyMember.HostAliases, []string{"delly", "192.168.0.10"}) {
		t.Fatalf("delly host aliases = %+v, want %+v", dellyMember.HostAliases, []string{"delly", "192.168.0.10"})
	}
	if dellyMember.AgentConnectionID != "agent:agent-delly" {
		t.Fatalf("delly agent connection = %q, want %q", dellyMember.AgentConnectionID, "agent:agent-delly")
	}
	if dellyMember.State != ConnectionStateActive {
		t.Fatalf("delly state = %q, want %q", dellyMember.State, ConnectionStateActive)
	}

	minipcMember, ok := membersByName["minipc"]
	if !ok {
		t.Fatalf("expected minipc member, got %+v", system.Members)
	}
	if minipcMember.Primary {
		t.Fatalf("minipc should not be marked primary: %+v", minipcMember)
	}
	if minipcMember.Endpoint != "https://minipc:8006" {
		t.Fatalf("minipc endpoint = %q, want %q", minipcMember.Endpoint, "https://minipc:8006")
	}
	if !reflect.DeepEqual(minipcMember.HostAliases, []string{"minipc", "192.168.0.11"}) {
		t.Fatalf("minipc host aliases = %+v, want %+v", minipcMember.HostAliases, []string{"minipc", "192.168.0.11"})
	}
	if minipcMember.AgentConnectionID != "agent:agent-minipc" {
		t.Fatalf("minipc agent connection = %q, want %q", minipcMember.AgentConnectionID, "agent:agent-minipc")
	}
	if minipcMember.State != ConnectionStateActive {
		t.Fatalf("minipc state = %q, want %q", minipcMember.State, ConnectionStateActive)
	}
}

func TestBuildConnectionSystems_GuestAgentStaysStandaloneWhenOnlyClusterInstanceMatches(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	adapter := unified.NewMonitorAdapter(nil)
	adapter.PopulateSupplementalRecords(unified.SourceProxmox, []unified.IngestRecord{
		{
			SourceID: "guest-100",
			Resource: unified.Resource{
				ID:       "resource-guest-100",
				Type:     unified.ResourceTypeAgent,
				Name:     "docker-vm",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourceProxmox, unified.SourceAgent},
				Identity: unified.ResourceIdentity{
					MachineID: "machine-guest-100",
					Hostnames: []string{"docker-vm"},
				},
				Proxmox: &unified.ProxmoxData{
					Instance:        "delly",
					NodeName:        "minipc",
					ClusterName:     "homelab",
					IsClusterMember: true,
					VMID:            100,
				},
				Agent: &unified.AgentData{
					AgentID:      "agent-guest-100",
					Hostname:     "docker-vm",
					MachineID:    "machine-guest-100",
					LinkedVMID:   "guest-100",
					LinkedNodeID: "",
				},
			},
		},
	})
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	connections := []Connection{
		{
			ID:           "pve:delly",
			Type:         ConnectionTypePVE,
			Name:         "delly",
			Address:      "https://delly:8006",
			State:        ConnectionStateActive,
			Enabled:      true,
			Surfaces:     []string{"vms", "containers", "storage", "backups"},
			Scope:        map[string]bool{"vms": true, "containers": true, "storage": true, "backups": true},
			Source:       ConnectionSourceAgent,
			Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
		},
		{
			ID:           "agent:agent-guest-100",
			Type:         ConnectionTypeAgent,
			Name:         "docker-vm",
			Address:      "docker-vm",
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
	if len(systems) != 2 {
		t.Fatalf("expected 2 grouped systems, got %d (%+v)", len(systems), systems)
	}

	systemsByID := make(map[string]ConnectionSystem, len(systems))
	for _, system := range systems {
		systemsByID[system.ID] = system
	}

	if len(systemsByID["pve:delly"].Components) != 1 {
		t.Fatalf("pve:delly should keep only its primary component, got %+v", systemsByID["pve:delly"].Components)
	}
	guestSystem := systemsByID["agent:agent-guest-100"]
	if len(guestSystem.Components) != 1 || guestSystem.Components[0].Role != ConnectionSystemComponentRolePrimary {
		t.Fatalf("guest agent should remain standalone, got %+v", guestSystem.Components)
	}
}
