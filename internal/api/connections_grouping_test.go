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
			Identity: unified.ResourceIdentity{
				MachineID:   "machine-delly",
				Hostnames:   []string{"delly"},
				IPAddresses: []string{"192.168.0.10"},
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
			Identity: unified.ResourceIdentity{
				MachineID:   "machine-minipc",
				Hostnames:   []string{"minipc"},
				IPAddresses: []string{"192.168.0.11"},
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

func TestMergeConnectionSystemMembersKeepsMostSevereState(t *testing.T) {
	activeMember := ConnectionSystemMember{
		ID:      "node-minipc",
		Name:    "minipc",
		State:   ConnectionStateActive,
		Primary: true,
	}
	offlineMember := ConnectionSystemMember{
		ID:    "resource-node-minipc",
		Name:  "minipc",
		State: ConnectionStateUnreachable,
	}

	merged := mergeConnectionSystemMembers(activeMember, offlineMember)
	if merged.State != ConnectionStateUnreachable {
		t.Fatalf("merged state = %q, want %q", merged.State, ConnectionStateUnreachable)
	}
	if !merged.Primary {
		t.Fatalf("expected primary marker to survive merge: %+v", merged)
	}

	staleMember := ConnectionSystemMember{Name: "minipc", State: ConnectionStateStale}
	freshMember := ConnectionSystemMember{Name: "minipc", State: ConnectionStateActive}
	merged = mergeConnectionSystemMembers(staleMember, freshMember)
	if merged.State != ConnectionStateStale {
		t.Fatalf("merged state = %q, want %q", merged.State, ConnectionStateStale)
	}
}

// A lapsed projection (orphaned unified-resource entry after a
// remove/re-enroll cycle) must not pair its severity with the live plane's
// fresh heartbeat: that rendered members as "Stale · 5s ago". The plane with
// the newer LastSeen decides the state; severity still wins when neither side
// carries evidence or the timestamps tie. Refs #1728.
func TestMergeConnectionSystemMembersFresherEvidenceDecidesState(t *testing.T) {
	now := time.Now().UTC()
	fresh := now.Add(-5 * time.Second)
	lapsed := now.Add(-3 * 24 * time.Hour)

	liveNode := ConnectionSystemMember{
		ID:       "node-minipc",
		Name:     "minipc",
		State:    ConnectionStateActive,
		LastSeen: &fresh,
		Primary:  true,
	}
	orphanResource := ConnectionSystemMember{
		ID:       "resource-node-minipc",
		Name:     "minipc",
		State:    ConnectionStateStale,
		LastSeen: &lapsed,
	}

	for name, merged := range map[string]ConnectionSystemMember{
		"live-first":   mergeConnectionSystemMembers(liveNode, orphanResource),
		"orphan-first": mergeConnectionSystemMembers(orphanResource, liveNode),
	} {
		if merged.State != ConnectionStateActive {
			t.Fatalf("%s: merged state = %q, want %q", name, merged.State, ConnectionStateActive)
		}
		if merged.LastSeen == nil || !merged.LastSeen.Equal(fresh) {
			t.Fatalf("%s: merged lastSeen = %v, want %v", name, merged.LastSeen, fresh)
		}
	}

	// An evidence-bearing severe plane still wins over a plane that has no
	// evidence at all, and over an equally recent healthy plane.
	unreachable := ConnectionSystemMember{
		Name:     "minipc",
		State:    ConnectionStateUnreachable,
		LastSeen: &lapsed,
	}
	noEvidence := ConnectionSystemMember{Name: "minipc", State: ConnectionStateActive}
	if merged := mergeConnectionSystemMembers(noEvidence, unreachable); merged.State != ConnectionStateUnreachable {
		t.Fatalf("no-evidence merge state = %q, want %q", merged.State, ConnectionStateUnreachable)
	}
	tied := ConnectionSystemMember{Name: "minipc", State: ConnectionStateActive, LastSeen: &lapsed}
	if merged := mergeConnectionSystemMembers(tied, unreachable); merged.State != ConnectionStateUnreachable {
		t.Fatalf("tied-recency merge state = %q, want %q", merged.State, ConnectionStateUnreachable)
	}
}

func TestConnectionSystemMemberUsesDisplayNameWithoutChangingMemberKey(t *testing.T) {
	resource := unified.Resource{
		ID:     "resource-node",
		Type:   unified.ResourceTypeAgent,
		Name:   "Render East",
		Status: unified.StatusOnline,
		Proxmox: &unified.ProxmoxData{
			SourceID: "production-pve1", NodeIdentity: "production-pve1",
			NodeName: "pve1", NodeDisplayName: "Render East",
			Instance: "cluster", IsClusterMember: true,
		},
	}
	connections := map[string]Connection{
		"pve:cluster": {ID: "pve:cluster", Type: ConnectionTypePVE},
	}
	member, primaryID, ok := connectionSystemMemberFromResource(resource, connections, nil)
	if !ok || primaryID != "pve:cluster" {
		t.Fatalf("member projection failed: primary=%q member=%+v", primaryID, member)
	}
	if member.Name != "Render East" || member.NativeName != "pve1" ||
		member.NodeIdentity != "production-pve1" {
		t.Fatalf("member presentation/native fields were not separated: %+v", member)
	}
	beforeKey := connectionSystemMemberKey(member)
	member.Name = "Another label"
	if afterKey := connectionSystemMemberKey(member); afterKey != beforeKey {
		t.Fatalf("cosmetic rename changed member key: before=%q after=%q", beforeKey, afterKey)
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

func TestBuildConnectionSystems_AttachesHostAgentToMatchingProxmoxSourceWithoutNodeInventory(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	now := time.Date(2026, 5, 13, 23, 45, 0, 0, time.UTC)
	connections := []Connection{
		{
			ID:           "pve:delly",
			Type:         ConnectionTypePVE,
			Name:         "delly",
			Address:      "https://delly:8006",
			HostAliases:  []string{"delly"},
			State:        ConnectionStateUnauthorized,
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
			HostAliases:  []string{"delly", "192.168.0.5"},
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
			HostAliases:  []string{"minipc", "192.168.0.134"},
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
	systemsByID := make(map[string]ConnectionSystem, len(systems))
	for _, system := range systems {
		systemsByID[system.ID] = system
	}

	dellySystem := systemsByID["pve:delly"]
	if len(dellySystem.Components) != 2 {
		t.Fatalf("expected delly API and agent components, got %+v", dellySystem.Components)
	}

	componentRoles := make(map[string]ConnectionSystemComponentRole, len(dellySystem.Components))
	for _, component := range dellySystem.Components {
		componentRoles[component.ConnectionID] = component.Role
	}
	if componentRoles["pve:delly"] != ConnectionSystemComponentRolePrimary {
		t.Fatalf("pve:delly role = %q, want %q", componentRoles["pve:delly"], ConnectionSystemComponentRolePrimary)
	}
	if componentRoles["agent:agent-delly"] != ConnectionSystemComponentRoleAttachment {
		t.Fatalf("agent:agent-delly role = %q, want %q", componentRoles["agent:agent-delly"], ConnectionSystemComponentRoleAttachment)
	}

	minipcSystem := systemsByID["agent:agent-minipc"]
	if len(minipcSystem.Components) != 1 || minipcSystem.Components[0].Role != ConnectionSystemComponentRolePrimary {
		t.Fatalf("minipc should remain standalone without a direct source host match, got %+v", minipcSystem.Components)
	}
}

// A PBS host running a Pulse agent must collapse into one "API + Agent" row
// the way PVE hosts do, even when the configured PBS address (an FQDN or
// DNS alias) shares no literal host string with anything the agent reports.
// The pairing comes from the shared top-level-system resolver, which holds
// short-form hostname equivalence — the same contract the Infrastructure
// surfaces already use to count these as one system.
func TestBuildConnectionSystems_AttachesHostAgentToPBSSourceByIdentityGrouping(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	adapter := unified.NewMonitorAdapter(nil)
	adapter.PopulateSupplementalRecords(unified.SourcePBS, []unified.IngestRecord{
		{
			SourceID: "pbs-backup-01",
			Resource: unified.Resource{
				ID:       "resource-pbs-backup-01",
				Type:     unified.ResourceTypePBS,
				Name:     "backup-01",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourcePBS},
				Identity: unified.ResourceIdentity{
					Hostnames: []string{"pbs01.internal.example"},
				},
				PBS: &unified.PBSData{
					InstanceID: "pbs-backup-01",
					Hostname:   "pbs01.internal.example",
					HostURL:    "https://pbs01.internal.example:8007",
				},
			},
		},
		{
			SourceID: "pbs-backup-02",
			Resource: unified.Resource{
				ID:       "resource-pbs-backup-02",
				Type:     unified.ResourceTypePBS,
				Name:     "backup-02",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourcePBS},
				Identity: unified.ResourceIdentity{
					Hostnames: []string{"pbs02.internal.example"},
				},
				PBS: &unified.PBSData{
					InstanceID: "pbs-backup-02",
					Hostname:   "pbs02.internal.example",
					HostURL:    "https://pbs02.internal.example:8007",
				},
			},
		},
	})
	adapter.PopulateSupplementalRecords(unified.SourceAgent, []unified.IngestRecord{
		{
			SourceID: "agent-pbs01",
			Resource: unified.Resource{
				ID:       "resource-agent-pbs01",
				Type:     unified.ResourceTypeAgent,
				Name:     "pbs01",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourceAgent},
				Identity: unified.ResourceIdentity{
					MachineID: "machine-pbs01",
					Hostnames: []string{"pbs01"},
				},
				Agent: &unified.AgentData{
					AgentID:   "agent-pbs01",
					Hostname:  "pbs01",
					MachineID: "machine-pbs01",
				},
			},
		},
		{
			SourceID: "agent-pbs02",
			Resource: unified.Resource{
				ID:       "resource-agent-pbs02",
				Type:     unified.ResourceTypeAgent,
				Name:     "pbs02",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourceAgent},
				Identity: unified.ResourceIdentity{
					MachineID: "machine-pbs02",
					Hostnames: []string{"pbs02"},
				},
				Agent: &unified.AgentData{
					AgentID:   "agent-pbs02",
					Hostname:  "pbs02",
					MachineID: "machine-pbs02",
				},
			},
		},
	})
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	connections := []Connection{
		{
			ID:           "pbs:backup-01",
			Type:         ConnectionTypePBS,
			Name:         "backup-01",
			Address:      "https://pbs01.internal.example:8007",
			HostAliases:  []string{"backup-01", "pbs01.internal.example"},
			State:        ConnectionStateActive,
			Enabled:      true,
			Surfaces:     []string{"backups", "datastores"},
			Scope:        map[string]bool{"backups": true, "datastores": true},
			Source:       ConnectionSourceManual,
			Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
		},
		{
			ID:           "pbs:backup-02",
			Type:         ConnectionTypePBS,
			Name:         "backup-02",
			Address:      "https://pbs02.internal.example:8007",
			HostAliases:  []string{"backup-02", "pbs02.internal.example"},
			State:        ConnectionStateActive,
			Enabled:      true,
			Surfaces:     []string{"backups", "datastores"},
			Scope:        map[string]bool{"backups": true, "datastores": true},
			Source:       ConnectionSourceManual,
			Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
		},
		{
			ID:           "agent:agent-pbs01",
			Type:         ConnectionTypeAgent,
			Name:         "pbs01",
			Address:      "pbs01",
			HostAliases:  []string{"pbs01", "192.168.40.11"},
			State:        ConnectionStateActive,
			Enabled:      true,
			Surfaces:     []string{"host"},
			Scope:        map[string]bool{"host": true},
			LastSeen:     &now,
			Source:       ConnectionSourceAgent,
			Capabilities: ConnectionCapabilities{SupportsPause: false, SupportsScope: false, SupportsTest: false},
		},
		{
			ID:           "agent:agent-pbs02",
			Type:         ConnectionTypeAgent,
			Name:         "pbs02",
			Address:      "pbs02",
			HostAliases:  []string{"pbs02", "192.168.40.12"},
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

	for pbsID, agentID := range map[string]string{
		"pbs:backup-01": "agent:agent-pbs01",
		"pbs:backup-02": "agent:agent-pbs02",
	} {
		system, ok := systemsByID[pbsID]
		if !ok {
			t.Fatalf("expected system %q, got %+v", pbsID, systems)
		}
		if len(system.Components) != 2 {
			t.Fatalf("%s: expected API and agent components, got %+v", pbsID, system.Components)
		}
		componentRoles := make(map[string]ConnectionSystemComponentRole, len(system.Components))
		for _, component := range system.Components {
			componentRoles[component.ConnectionID] = component.Role
		}
		if componentRoles[pbsID] != ConnectionSystemComponentRolePrimary {
			t.Fatalf("%s role = %q, want %q", pbsID, componentRoles[pbsID], ConnectionSystemComponentRolePrimary)
		}
		if componentRoles[agentID] != ConnectionSystemComponentRoleAttachment {
			t.Fatalf("%s role = %q, want %q", agentID, componentRoles[agentID], ConnectionSystemComponentRoleAttachment)
		}
	}
}

// The PMG poller writes "pmg-<name>" instance IDs the same way PBS does, so
// a PMG host running a Pulse agent gets the same identity-grouped pairing.
func TestBuildConnectionSystems_AttachesHostAgentToPMGSourceByIdentityGrouping(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	now := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)
	adapter := unified.NewMonitorAdapter(nil)
	adapter.PopulateSupplementalRecords(unified.SourcePMG, []unified.IngestRecord{
		{
			SourceID: "pmg-mail-01",
			Resource: unified.Resource{
				ID:       "resource-pmg-mail-01",
				Type:     unified.ResourceTypePMG,
				Name:     "mail-01",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourcePMG},
				Identity: unified.ResourceIdentity{
					Hostnames: []string{"pmg01.internal.example"},
				},
				PMG: &unified.PMGData{
					InstanceID: "pmg-mail-01",
					Hostname:   "pmg01.internal.example",
					HostURL:    "https://pmg01.internal.example:8006",
				},
			},
		},
	})
	adapter.PopulateSupplementalRecords(unified.SourceAgent, []unified.IngestRecord{
		{
			SourceID: "agent-pmg01",
			Resource: unified.Resource{
				ID:       "resource-agent-pmg01",
				Type:     unified.ResourceTypeAgent,
				Name:     "pmg01",
				Status:   unified.StatusOnline,
				LastSeen: now,
				Sources:  []unified.DataSource{unified.SourceAgent},
				Identity: unified.ResourceIdentity{
					MachineID: "machine-pmg01",
					Hostnames: []string{"pmg01"},
				},
				Agent: &unified.AgentData{
					AgentID:   "agent-pmg01",
					Hostname:  "pmg01",
					MachineID: "machine-pmg01",
				},
			},
		},
	})
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	connections := []Connection{
		{
			ID:           "pmg:mail-01",
			Type:         ConnectionTypePMG,
			Name:         "mail-01",
			Address:      "https://pmg01.internal.example:8006",
			HostAliases:  []string{"mail-01", "pmg01.internal.example"},
			State:        ConnectionStateActive,
			Enabled:      true,
			Surfaces:     []string{"mailStats", "queues"},
			Scope:        map[string]bool{"mailStats": true, "queues": true},
			Source:       ConnectionSourceManual,
			Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
		},
		{
			ID:           "agent:agent-pmg01",
			Type:         ConnectionTypeAgent,
			Name:         "pmg01",
			Address:      "pmg01",
			HostAliases:  []string{"pmg01", "192.168.40.21"},
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
	if system.ID != "pmg:mail-01" {
		t.Fatalf("system id = %q, want %q", system.ID, "pmg:mail-01")
	}
	if len(system.Components) != 2 {
		t.Fatalf("expected API and agent components, got %+v", system.Components)
	}
	componentRoles := make(map[string]ConnectionSystemComponentRole, len(system.Components))
	for _, component := range system.Components {
		componentRoles[component.ConnectionID] = component.Role
	}
	if componentRoles["pmg:mail-01"] != ConnectionSystemComponentRolePrimary {
		t.Fatalf("pmg:mail-01 role = %q, want %q", componentRoles["pmg:mail-01"], ConnectionSystemComponentRolePrimary)
	}
	if componentRoles["agent:agent-pmg01"] != ConnectionSystemComponentRoleAttachment {
		t.Fatalf("agent:agent-pmg01 role = %q, want %q", componentRoles["agent:agent-pmg01"], ConnectionSystemComponentRoleAttachment)
	}
}

func TestDirectPlatformHostAttachmentSupportsSingleHostAPISources(t *testing.T) {
	tests := []struct {
		name string
		typ  ConnectionType
	}{
		{name: "Proxmox VE", typ: ConnectionTypePVE},
		{name: "Proxmox Backup Server", typ: ConnectionTypePBS},
		{name: "Proxmox Mail Gateway", typ: ConnectionTypePMG},
		{name: "TrueNAS", typ: ConnectionTypeTrueNAS},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			connections := map[string]Connection{
				"platform:source": {
					ID:          "platform:source",
					Type:        test.typ,
					Name:        "source-01",
					Address:     "https://192.0.2.30:8443",
					HostAliases: []string{"source-01", "192.0.2.30"},
					Enabled:     true,
				},
				"agent:source": {
					ID:          "agent:source",
					Type:        ConnectionTypeAgent,
					Name:        "source-01",
					Address:     "192.0.2.30",
					HostAliases: []string{"source-01", "192.0.2.30"},
					Enabled:     true,
				},
			}
			if got := directPlatformHostAttachment(connections["agent:source"], connections); got != "platform:source" {
				t.Fatalf("directPlatformHostAttachment() = %q, want %q", got, "platform:source")
			}
		})
	}

	t.Run("ambiguous platform sources fail closed", func(t *testing.T) {
		agent := Connection{ID: "agent:source", Type: ConnectionTypeAgent, Name: "source-01"}
		connections := map[string]Connection{
			agent.ID:          agent,
			"pve:source":      {ID: "pve:source", Type: ConnectionTypePVE, Name: "source-01", Enabled: true},
			"pbs:source":      {ID: "pbs:source", Type: ConnectionTypePBS, Name: "source-01", Enabled: true},
			"vmware:source":   {ID: "vmware:source", Type: ConnectionTypeVMware, Name: "source-01", Enabled: true},
			"disabled:source": {ID: "disabled:source", Type: ConnectionTypePMG, Name: "source-01"},
		}
		if got := directPlatformHostAttachment(agent, connections); got != "" {
			t.Fatalf("directPlatformHostAttachment() = %q, want an ambiguous fail-closed result", got)
		}
	})

	t.Run("vCenter is not a single-host attachment", func(t *testing.T) {
		agent := Connection{ID: "agent:source", Type: ConnectionTypeAgent, Name: "source-01"}
		connections := map[string]Connection{
			agent.ID:        agent,
			"vmware:source": {ID: "vmware:source", Type: ConnectionTypeVMware, Name: "source-01", Enabled: true},
		}
		if got := directPlatformHostAttachment(agent, connections); got != "" {
			t.Fatalf("directPlatformHostAttachment() = %q, want no vCenter attachment", got)
		}
	})
}

// One vCenter connection spans many ESXi hosts, so vSphere host resources
// compose as members of their owning vmware connection the way Proxmox
// cluster nodes compose under their cluster source. Resources pointing at an
// unknown or differently-typed connection produce no member.
func TestConnectionSystemMemberFromVMwareHost(t *testing.T) {
	lastSeen := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	hostResource := unified.Resource{
		ID:       "vc-1:host:host-101",
		Name:     "esxi-01.lab.local",
		Status:   unified.StatusOnline,
		LastSeen: lastSeen,
		VMware: &unified.VMwareData{
			ConnectionID:    "vc-1",
			EntityType:      "host",
			ManagedObjectID: "host-101",
		},
	}
	connections := map[string]Connection{
		"vmware:vc-1": {ID: "vmware:vc-1", Type: ConnectionTypeVMware, Name: "Lab vCenter"},
	}

	member, primaryID, ok := connectionSystemMemberFromVMwareHost(hostResource, connections)
	if !ok {
		t.Fatal("expected ESXi host resource to produce a vmware system member")
	}
	if primaryID != "vmware:vc-1" {
		t.Fatalf("primary = %q, want %q", primaryID, "vmware:vc-1")
	}
	if member.Name != "esxi-01.lab.local" || member.State != ConnectionStateActive {
		t.Fatalf("unexpected member projection: %+v", member)
	}
	if member.LastSeen == nil || !member.LastSeen.Equal(lastSeen) {
		t.Fatalf("expected member lastSeen %v, got %+v", lastSeen, member.LastSeen)
	}
	if member.Primary {
		t.Fatalf("ESXi hosts must not claim the primary marker: %+v", member)
	}

	vmResource := hostResource
	vmResource.VMware = &unified.VMwareData{ConnectionID: "vc-1", EntityType: "vm"}
	if _, _, ok := connectionSystemMemberFromVMwareHost(vmResource, connections); ok {
		t.Fatal("VM resources must not become system members")
	}

	orphanResource := hostResource
	orphanResource.VMware = &unified.VMwareData{ConnectionID: "vc-unknown", EntityType: "host"}
	if _, _, ok := connectionSystemMemberFromVMwareHost(orphanResource, connections); ok {
		t.Fatal("hosts pointing at an unconfigured vCenter must not become members")
	}
}
