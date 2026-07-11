package unifiedresources

import (
	"slices"
	"testing"
	"time"
)

func TestCoalescePresentationHostResourcesMergesSplitRuntimeAndPlatformHost(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 30, 0, 0, time.UTC)
	resources := []Resource{
		{
			ID:       "agent-proxmox-delly",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   StatusWarning,
			LastSeen: now.Add(-1 * time.Minute),
			Sources:  []DataSource{SourceProxmox},
			Identity: ResourceIdentity{Hostnames: []string{"delly"}},
			Proxmox: &ProxmoxData{
				NodeName:    "delly",
				ClusterName: "homelab",
			},
		},
		{
			ID:       "agent-runtime-delly",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceAgent},
			Identity: ResourceIdentity{
				MachineID: "agent-machine-delly",
				Hostnames: []string{"delly"},
			},
			Agent: &AgentData{
				AgentID:  "agent-machine-delly",
				Hostname: "delly",
				OSName:   "Proxmox VE",
			},
		},
	}

	coalesced := CoalescePresentationHostResources(resources)
	if len(coalesced) != 1 {
		t.Fatalf("expected split host resources to coalesce into 1 resource, got %d: %#v", len(coalesced), coalesced)
	}

	resource := coalesced[0]
	if resource.ID != "agent-runtime-delly" {
		t.Fatalf("expected agent-backed resource ID, got %q", resource.ID)
	}
	if resource.Agent == nil || resource.Proxmox == nil {
		t.Fatalf("expected merged agent and Proxmox facets, got agent=%+v proxmox=%+v", resource.Agent, resource.Proxmox)
	}
	if !slices.Contains(resource.Sources, SourceAgent) || !slices.Contains(resource.Sources, SourceProxmox) {
		t.Fatalf("expected merged agent and Proxmox sources, got %+v", resource.Sources)
	}
}

func TestCoalescePresentationHostResourcesSurfacesStaleAgentOnLiveNode(t *testing.T) {
	// A Proxmox node whose Pulse Agent has stopped reporting (401) stays online
	// via the PVE API poll, but the dead agent must remain flagged stale so the
	// UI does not present its last version as if it were current (issue #1515).
	now := time.Date(2026, 7, 8, 10, 30, 0, 0, time.UTC)
	resources := []Resource{
		{
			ID:       "agent-runtime-pve1",
			Type:     ResourceTypeAgent,
			Name:     "pve1",
			Status:   StatusOffline,
			LastSeen: now.Add(-30 * time.Minute),
			Sources:  []DataSource{SourceAgent},
			Identity: ResourceIdentity{Hostnames: []string{"pve1"}},
			Agent: &AgentData{
				AgentID:      "agent-machine-pve1",
				Hostname:     "pve1",
				AgentVersion: "6.0.2",
				Stale:        true,
			},
		},
		{
			ID:       "proxmox-node-pve1",
			Type:     ResourceTypeAgent,
			Name:     "pve1",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox},
			Identity: ResourceIdentity{Hostnames: []string{"pve1"}},
			Proxmox:  &ProxmoxData{NodeName: "pve1", ClusterName: "homelab"},
		},
	}

	coalesced := CoalescePresentationHostResources(resources)
	if len(coalesced) != 1 {
		t.Fatalf("expected split host resources to coalesce into 1 resource, got %d", len(coalesced))
	}
	resource := coalesced[0]
	if resource.Status != StatusOnline {
		t.Fatalf("node should stay online via the PVE source, got %q", resource.Status)
	}
	if resource.Agent == nil || !resource.Agent.Stale {
		t.Fatalf("expected the dead agent to remain flagged stale after merge, got %+v", resource.Agent)
	}
	if resource.Agent.AgentVersion != "6.0.2" {
		t.Fatalf("expected stale agent version retained, got %q", resource.Agent.AgentVersion)
	}
}

func TestCoalescePresentationHostResourcesRedirectsProxmoxChildrenToAgentBackedParent(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 30, 0, 0, time.UTC)
	proxmoxParentID := "agent-proxmox-delly"
	resources := []Resource{
		{
			ID:       proxmoxParentID,
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox},
			Identity: ResourceIdentity{Hostnames: []string{"delly"}},
			Proxmox: &ProxmoxData{
				SourceID:    "homelab-delly",
				NodeName:    "delly",
				ClusterName: "homelab",
				Instance:    "delly",
			},
		},
		{
			ID:       "system-container-grafana",
			Type:     ResourceTypeSystemContainer,
			Name:     "grafana",
			Status:   StatusOnline,
			LastSeen: now,
			ParentID: &proxmoxParentID,
			Sources:  []DataSource{SourceProxmox},
			Identity: ResourceIdentity{Hostnames: []string{"grafana"}},
			Proxmox: &ProxmoxData{
				SourceID:    "delly:delly:124",
				NodeName:    "delly",
				ClusterName: "homelab",
				Instance:    "delly",
				VMID:        124,
			},
		},
		{
			ID:       "agent-runtime-delly",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   StatusOnline,
			LastSeen: now.Add(time.Second),
			Sources:  []DataSource{SourceAgent},
			Identity: ResourceIdentity{Hostnames: []string{"delly"}},
			Agent: &AgentData{
				AgentID:  "agent-machine-delly",
				Hostname: "delly",
			},
		},
	}

	coalesced := CoalescePresentationHostResources(resources)
	if len(coalesced) != 2 {
		t.Fatalf("expected merged parent plus child, got %d: %#v", len(coalesced), coalesced)
	}

	var child *Resource
	for i := range coalesced {
		if coalesced[i].ID == "system-container-grafana" {
			child = &coalesced[i]
			break
		}
	}
	if child == nil {
		t.Fatal("expected grafana child in coalesced resources")
	}
	if child.ParentID == nil || *child.ParentID != "agent-runtime-delly" {
		t.Fatalf("expected child parent agent-runtime-delly, got %+v", child.ParentID)
	}
	if child.Proxmox == nil || child.Proxmox.LinkedAgentID != "agent-machine-delly" {
		t.Fatalf("expected child linked agent agent-machine-delly, got %+v", child.Proxmox)
	}
}

// The #1559 shape: distinct standalone machines whose dotted hostnames share
// a short name (cloud.rnd-lax1 vs cloud.gce-or1) must not coalesce, while a
// short name still pairs with its own FQDN.
func TestCoalescePresentationHostResourcesKeepsDottedHostnameSiblingsApart(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 30, 0, 0, time.UTC)
	agentHost := func(id, hostname string, lastSeen time.Time) Resource {
		return Resource{
			ID:       id,
			Type:     ResourceTypeAgent,
			Name:     hostname,
			Status:   StatusOnline,
			LastSeen: lastSeen,
			Sources:  []DataSource{SourceAgent},
			Identity: ResourceIdentity{MachineID: "machine-" + id, Hostnames: []string{hostname}},
			Agent:    &AgentData{AgentID: "machine-" + id, Hostname: hostname},
		}
	}
	dockerHost := func(id, hostname string, lastSeen time.Time) Resource {
		return Resource{
			ID:       id,
			Type:     ResourceTypeAgent,
			Name:     hostname,
			Status:   StatusOnline,
			LastSeen: lastSeen,
			Sources:  []DataSource{SourceDocker},
			Identity: ResourceIdentity{Hostnames: []string{hostname}},
			Docker:   &DockerData{HostSourceID: id, Hostname: hostname},
		}
	}

	resources := []Resource{
		agentHost("agent-rnd", "cloud.rnd-lax1", now),
		dockerHost("docker-gce", "cloud.gce-or1", now.Add(time.Second)),
		dockerHost("docker-rnd", "cloud.rnd-lax1", now.Add(2*time.Second)),
	}

	coalesced := CoalescePresentationHostResources(resources)
	if len(coalesced) != 2 {
		t.Fatalf("expected the two cloud.rnd-lax1 views to merge and cloud.gce-or1 to stay apart, got %d: %#v", len(coalesced), coalesced)
	}
	for _, resource := range coalesced {
		if resource.ID == "agent-rnd" {
			if resource.Docker == nil || resource.Docker.HostSourceID != "docker-rnd" {
				t.Fatalf("expected agent-rnd to absorb its own docker view, got %+v", resource.Docker)
			}
		}
		if resource.ID == "docker-gce" && resource.Agent != nil {
			t.Fatalf("cloud.gce-or1 must not absorb another machine's agent view, got %+v", resource.Agent)
		}
	}

	// Short name and its own FQDN still coalesce.
	fqdnPair := []Resource{
		agentHost("agent-web", "web01.lan", now),
		dockerHost("docker-web", "web01", now.Add(time.Second)),
	}
	if merged := CoalescePresentationHostResources(fqdnPair); len(merged) != 1 {
		t.Fatalf("expected short/FQDN views of the same host to coalesce, got %d: %#v", len(merged), merged)
	}
}

func TestCoalescePresentationHostResourcesDoesNotMergeRuntimeOnlyNameCollision(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 30, 0, 0, time.UTC)
	resources := []Resource{
		{
			ID:       "agent-left",
			Type:     ResourceTypeAgent,
			Name:     "shared-host",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceAgent},
			Identity: ResourceIdentity{Hostnames: []string{"shared-host"}},
			Agent:    &AgentData{AgentID: "agent-left", Hostname: "shared-host"},
		},
		{
			ID:       "agent-right",
			Type:     ResourceTypeAgent,
			Name:     "shared-host",
			Status:   StatusOnline,
			LastSeen: now.Add(time.Second),
			Sources:  []DataSource{SourceAgent},
			Identity: ResourceIdentity{Hostnames: []string{"shared-host"}},
			Agent:    &AgentData{AgentID: "agent-right", Hostname: "shared-host"},
		},
	}

	if coalesced := CoalescePresentationHostResources(resources); len(coalesced) != 2 {
		t.Fatalf("expected runtime-only host collision to stay split, got %#v", coalesced)
	}
}

func TestCoalescePresentationHostResourcesConvergesOrderSensitiveFragments(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 30, 0, 0, time.UTC)
	resources := []Resource{
		{
			ID:       "agent-k8s-cluster-a-worker-1",
			Type:     ResourceTypeAgent,
			Name:     "worker-1",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceK8s},
			Identity: ResourceIdentity{Hostnames: []string{"worker-1"}},
			Kubernetes: &K8sData{
				ClusterID:   "cluster-a",
				ClusterName: "production",
				NodeName:    "worker-1",
			},
		},
		{
			ID:       "agent-k8s-cluster-b-worker-1",
			Type:     ResourceTypeAgent,
			Name:     "worker-1",
			Status:   StatusOnline,
			LastSeen: now.Add(time.Second),
			Sources:  []DataSource{SourceK8s},
			Identity: ResourceIdentity{Hostnames: []string{"worker-1"}},
			Kubernetes: &K8sData{
				ClusterID:   "cluster-b",
				ClusterName: "production",
				NodeName:    "worker-1",
			},
		},
		{
			ID:       "agent-runtime-worker-1",
			Type:     ResourceTypeAgent,
			Name:     "worker-1",
			Status:   StatusOnline,
			LastSeen: now.Add(2 * time.Second),
			Sources:  []DataSource{SourceAgent},
			Identity: ResourceIdentity{Hostnames: []string{"worker-1"}},
			Agent:    &AgentData{AgentID: "agent-worker-1", Hostname: "worker-1"},
		},
	}

	coalesced := CoalescePresentationHostResources(resources)
	if len(coalesced) != 1 {
		t.Fatalf("expected order-sensitive host fragments to converge into 1 resource, got %d: %#v", len(coalesced), coalesced)
	}
	if !slices.Contains(coalesced[0].Sources, SourceAgent) || !slices.Contains(coalesced[0].Sources, SourceK8s) {
		t.Fatalf("expected merged agent and kubernetes sources, got %+v", coalesced[0].Sources)
	}
}

func TestCoalescePresentationHostResourcesWithExclusionsHonorsManualSplit(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 30, 0, 0, time.UTC)
	resources := []Resource{
		{
			ID:       "agent-runtime-alpha",
			Type:     ResourceTypeAgent,
			Name:     "alpha",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceAgent},
			Identity: ResourceIdentity{Hostnames: []string{"alpha"}},
			Agent:    &AgentData{AgentID: "agent-alpha", Hostname: "alpha"},
		},
		{
			ID:       "agent-docker-alpha",
			Type:     ResourceTypeAgent,
			Name:     "alpha",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceDocker},
			Identity: ResourceIdentity{Hostnames: []string{"alpha"}},
			Docker:   &DockerData{Hostname: "alpha"},
		},
	}

	coalesced := CoalescePresentationHostResourcesWithExclusions(resources, func(left, right Resource) bool {
		return (left.ID == "agent-runtime-alpha" && right.ID == "agent-docker-alpha") ||
			(left.ID == "agent-docker-alpha" && right.ID == "agent-runtime-alpha")
	})
	if len(coalesced) != 2 {
		t.Fatalf("expected manual split exclusion to keep resources separate, got %#v", coalesced)
	}
}

// A Proxmox node whose Pulse Agent has gone offline must show the live PVE CPU,
// not the agent's last (0) reading. The agent is the presentation primary, so
// without a freshness gate its stale 0 CPU was kept and the live value dropped.
func TestCoalescePresentationHostResourcesPrefersLiveProxmoxCPUOverOfflineAgent(t *testing.T) {
	now := time.Now().UTC()
	resources := []Resource{
		{
			ID:       "proxmox-delly",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox},
			SourceStatus: map[DataSource]SourceStatus{
				SourceProxmox: {Status: "online", LastSeen: now},
			},
			Identity: ResourceIdentity{Hostnames: []string{"delly"}},
			Proxmox:  &ProxmoxData{NodeName: "delly", ClusterName: "homelab"},
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{Value: 2.88, Percent: 2.88, Unit: "percent", Source: SourceProxmox},
			},
		},
		{
			ID:       "agent-delly",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   StatusOnline,
			LastSeen: now.Add(-2 * time.Hour),
			Sources:  []DataSource{SourceAgent},
			SourceStatus: map[DataSource]SourceStatus{
				SourceAgent: {Status: "stale", LastSeen: now.Add(-2 * time.Hour)},
			},
			Identity: ResourceIdentity{MachineID: "agent-machine-delly", Hostnames: []string{"delly"}},
			Agent:    &AgentData{AgentID: "agent-machine-delly", Hostname: "delly"},
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{Value: 0, Percent: 0, Unit: "percent", Source: SourceAgent},
			},
		},
	}

	coalesced := CoalescePresentationHostResources(resources)
	if len(coalesced) != 1 {
		t.Fatalf("expected split host resources to coalesce into 1, got %d: %#v", len(coalesced), coalesced)
	}
	cpu := coalesced[0].Metrics.CPU
	if cpu == nil || cpu.Percent != 2.88 {
		t.Fatalf("expected live Proxmox CPU 2.88 to win over offline agent CPU 0, got %+v", cpu)
	}
}

// When the agent is live it stays the preferred CPU source (the presentation
// primary), so a fresh agent reading is not displaced by the API value.
func TestCoalescePresentationHostResourcesKeepsLiveAgentCPU(t *testing.T) {
	now := time.Now().UTC()
	resources := []Resource{
		{
			ID:       "proxmox-pi",
			Type:     ResourceTypeAgent,
			Name:     "pi",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox},
			SourceStatus: map[DataSource]SourceStatus{
				SourceProxmox: {Status: "online", LastSeen: now},
			},
			Identity: ResourceIdentity{Hostnames: []string{"pi"}},
			Proxmox:  &ProxmoxData{NodeName: "pi"},
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{Value: 5, Percent: 5, Unit: "percent", Source: SourceProxmox},
			},
		},
		{
			ID:       "agent-pi",
			Type:     ResourceTypeAgent,
			Name:     "pi",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceAgent},
			SourceStatus: map[DataSource]SourceStatus{
				SourceAgent: {Status: "online", LastSeen: now},
			},
			Identity: ResourceIdentity{MachineID: "agent-machine-pi", Hostnames: []string{"pi"}},
			Agent:    &AgentData{AgentID: "agent-machine-pi", Hostname: "pi"},
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{Value: 12, Percent: 12, Unit: "percent", Source: SourceAgent},
			},
		},
	}

	coalesced := CoalescePresentationHostResources(resources)
	if len(coalesced) != 1 {
		t.Fatalf("expected coalesce into 1, got %d", len(coalesced))
	}
	cpu := coalesced[0].Metrics.CPU
	if cpu == nil || cpu.Percent != 12 {
		t.Fatalf("expected live agent CPU 12 to be kept, got %+v", cpu)
	}
}
