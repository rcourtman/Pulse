package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIssue1753StandaloneProviderScopesSurvivePinnedBootWindow(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC)
	adapter := NewMonitorAdapter(NewRegistry(store))
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes: []models.Node{{
			ID: "staging-pve", NodeIdentity: "staging-pve", Name: "pve",
			DisplayName: "Staging", Instance: "staging", Host: "https://pve.staging.example:8006",
			Status: "online", LinkedAgentID: "agent-staging", LastSeen: now,
		}},
		Hosts: []models.Host{{
			ID: "agent-staging", Hostname: "pve", MachineID: "machine-staging",
			LinkedNodeID: "staging-pve", Status: "online", LastSeen: now,
		}},
		LastUpdate: now,
	})

	// Provider polling can lead agent check-in after restart. The durable pin
	// from the established first site must not lend its machine identity to a
	// new standalone provider solely because both native nodes are named pve.
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "staging-pve", NodeIdentity: "staging-pve", Name: "pve", DisplayName: "Staging", Instance: "staging", Host: "https://pve.staging.example:8006", Status: "online", LastSeen: now.Add(time.Minute)},
			{ID: "production-pve", NodeIdentity: "production-pve", Name: "pve", DisplayName: "Production", Instance: "production", Host: "https://pve.production.example:8006", Status: "online", LastSeen: now.Add(time.Minute)},
		},
		LastUpdate: now.Add(time.Minute),
	})

	resources := adapter.GetAll()
	var nodes []Resource
	for _, resource := range resources {
		if resource.Type == ResourceTypeAgent && resource.Proxmox != nil {
			nodes = append(nodes, resource)
		}
	}
	if len(nodes) != 2 {
		t.Fatalf("presentation nodes = %d, want 2: %+v", len(nodes), nodes)
	}
	byInstance := make(map[string]Resource, len(nodes))
	for _, node := range nodes {
		byInstance[node.Proxmox.Instance] = node
	}
	if got := byInstance["staging"].ID; got != MachineIdentityCanonicalID(ResourceTypeAgent, "machine-staging") {
		t.Fatalf("staging canonical ID = %q, want pinned machine identity", got)
	}
	if byInstance["production"].ID == "" || byInstance["production"].ID == byInstance["staging"].ID {
		t.Fatalf("provider identities were not separated: %+v", byInstance)
	}
	if byInstance["staging"].Name != "Staging" || byInstance["production"].Name != "Production" {
		t.Fatalf("provider display names were lost: %+v", byInstance)
	}
}

func TestIssue1753LegacyShortHostnamePinCannotClaimMultipleProviderScopes(t *testing.T) {
	store := NewMemoryStore()
	if err := store.UpsertResourceIdentityPins([]ResourceIdentityPin{{
		CanonicalID:  MachineIdentityCanonicalID(ResourceTypeAgent, "machine-staging"),
		ResourceType: ResourceTypeAgent,
		MachineID:    "machine-staging",
		Hostname:     "pve",
	}}); err != nil {
		t.Fatalf("seed legacy identity pin: %v", err)
	}

	registry := NewRegistry(store)
	registry.IngestSnapshot(models.StateSnapshot{Nodes: []models.Node{
		{ID: "staging-pve", NodeIdentity: "staging-pve", Name: "pve", DisplayName: "Staging", Instance: "staging", Host: "https://pve.staging.example:8006", Status: "online"},
		{ID: "production-pve", NodeIdentity: "production-pve", Name: "pve", DisplayName: "Production", Instance: "production", Host: "https://pve.production.example:8006", Status: "online"},
	}})

	resources := registry.ListForPresentation()
	var nodes []Resource
	for _, resource := range resources {
		if resource.Type == ResourceTypeAgent && resource.Proxmox != nil {
			nodes = append(nodes, resource)
		}
	}
	if len(nodes) != 2 {
		t.Fatalf("legacy short pin collapsed provider scopes: %+v", nodes)
	}
	for _, node := range nodes {
		if node.Identity.MachineID != "" {
			t.Fatalf("legacy short pin leaked machine identity into %q: %+v", node.Proxmox.Instance, node.Identity)
		}
	}
}

func TestIssue1753RepeatedShortProviderEndpointCannotClaimLegacyPin(t *testing.T) {
	store := NewMemoryStore()
	if err := store.UpsertResourceIdentityPins([]ResourceIdentityPin{{
		CanonicalID:  MachineIdentityCanonicalID(ResourceTypeAgent, "machine-staging"),
		ResourceType: ResourceTypeAgent,
		MachineID:    "machine-staging",
		Hostname:     "pve",
	}}); err != nil {
		t.Fatalf("seed legacy identity pin: %v", err)
	}

	registry := NewRegistry(store)
	registry.IngestSnapshot(models.StateSnapshot{Nodes: []models.Node{
		{ID: "staging-pve", NodeIdentity: "staging-pve", Name: "pve", DisplayName: "Staging", Instance: "staging", Host: "https://pve:8006", Status: "online"},
		{ID: "production-pve", NodeIdentity: "production-pve", Name: "pve", DisplayName: "Production", Instance: "production", Host: "https://pve:8006", Status: "online"},
	}})

	var nodes []Resource
	for _, resource := range registry.ListForPresentation() {
		if resource.Type == ResourceTypeAgent && resource.Proxmox != nil {
			nodes = append(nodes, resource)
		}
	}
	if len(nodes) != 2 {
		t.Fatalf("repeated short endpoint collapsed provider scopes: %+v", nodes)
	}
	for _, node := range nodes {
		if node.Identity.MachineID != "" {
			t.Fatalf("short endpoint leaked machine identity into %q: %+v", node.Proxmox.Instance, node.Identity)
		}
	}
}
