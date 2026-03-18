package unifiedresources

import "testing"

func TestLinkedMergeAllowsOneSidedNodeHostLinkWhenHostnameCorroborates(t *testing.T) {
	registry := NewRegistry(NewMemoryStore())

	agentResource := Resource{
		Type:   ResourceTypeAgent,
		Name:   "pve1",
		Status: StatusOnline,
		Agent:  &AgentData{},
	}
	registry.ingest(SourceAgent, "host-1", agentResource, ResourceIdentity{Hostnames: []string{"pve1"}})

	nodeResource := Resource{
		Type:   ResourceTypeAgent,
		Name:   "pve1",
		Status: StatusOnline,
		Proxmox: &ProxmoxData{
			LinkedAgentID: "host-1",
		},
	}
	registry.ingest(SourceProxmox, "node-1", nodeResource, ResourceIdentity{Hostnames: []string{"pve1"}})

	resources := registry.List()
	if len(resources) != 1 {
		t.Fatalf("expected 1 merged resource when one-sided link is corroborated, got %d", len(resources))
	}
	resource := resources[0]
	if !containsDataSource(resource.Sources, SourceAgent) || !containsDataSource(resource.Sources, SourceProxmox) {
		t.Fatalf("expected merged agent+proxmox sources, got %+v", resource.Sources)
	}
}

func TestLinkedMergeSucceedsWithBidirectionalNodeHostLink(t *testing.T) {
	registry := NewRegistry(NewMemoryStore())

	agentResource := Resource{
		Type:   ResourceTypeAgent,
		Name:   "pve1",
		Status: StatusOnline,
		Agent: &AgentData{
			LinkedNodeID: "node-1",
		},
	}
	registry.ingest(SourceAgent, "host-1", agentResource, ResourceIdentity{Hostnames: []string{"pve1"}})

	nodeResource := Resource{
		Type:   ResourceTypeAgent,
		Name:   "pve1",
		Status: StatusOnline,
		Proxmox: &ProxmoxData{
			LinkedAgentID: "host-1",
		},
	}
	registry.ingest(SourceProxmox, "node-1", nodeResource, ResourceIdentity{Hostnames: []string{"pve1"}})

	resources := registry.List()
	if len(resources) != 1 {
		t.Fatalf("expected 1 merged resource, got %d", len(resources))
	}
	resource := resources[0]
	if !containsDataSource(resource.Sources, SourceAgent) || !containsDataSource(resource.Sources, SourceProxmox) {
		t.Fatalf("expected merged agent+proxmox sources, got %+v", resource.Sources)
	}
}

func TestLinkedMergeDoesNotTrustOneSidedNodeHostLinkWithoutHostnameCorroboration(t *testing.T) {
	registry := NewRegistry(NewMemoryStore())

	agentResource := Resource{
		Type:   ResourceTypeAgent,
		Name:   "minipc",
		Status: StatusOnline,
		Agent:  &AgentData{},
	}
	registry.ingest(SourceAgent, "host-1", agentResource, ResourceIdentity{Hostnames: []string{"minipc"}})

	nodeResource := Resource{
		Type:   ResourceTypeAgent,
		Name:   "pve1",
		Status: StatusOnline,
		Proxmox: &ProxmoxData{
			LinkedAgentID: "host-1",
		},
	}
	registry.ingest(SourceProxmox, "node-1", nodeResource, ResourceIdentity{Hostnames: []string{"pve1"}})

	resources := registry.List()
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources when one-sided link lacks corroborating hostname, got %d", len(resources))
	}
}

func TestHostnameIPDoesNotAutoMerge(t *testing.T) {
	registry := NewRegistry(NewMemoryStore())

	agentResource := Resource{
		Type:   ResourceTypeAgent,
		Name:   "alpha",
		Status: StatusOnline,
		Agent:  &AgentData{},
	}
	registry.ingest(SourceAgent, "host-1", agentResource, ResourceIdentity{
		Hostnames:   []string{"alpha"},
		IPAddresses: []string{"10.0.0.9"},
	})

	dockerResource := Resource{
		Type:   ResourceTypeAgent,
		Name:   "alpha",
		Status: StatusOnline,
		Docker: &DockerData{},
	}
	registry.ingest(SourceDocker, "docker-1", dockerResource, ResourceIdentity{
		Hostnames:   []string{"alpha"},
		IPAddresses: []string{"10.0.0.9"},
	})

	resources := registry.List()
	if len(resources) != 2 {
		t.Fatalf("expected hostname+ip to stay separate, got %d resources", len(resources))
	}
}

func containsDataSource(sources []DataSource, want DataSource) bool {
	for _, source := range sources {
		if source == want {
			return true
		}
	}
	return false
}
