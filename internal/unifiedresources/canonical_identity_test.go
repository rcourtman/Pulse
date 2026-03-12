package unifiedresources

import "testing"

func TestRefreshCanonicalIdentityPrefersTargetsAndCanonicalHostData(t *testing.T) {
	resource := Resource{
		ID:   "agent-1",
		Type: ResourceTypeAgent,
		Name: "Tower",
		Identity: ResourceIdentity{
			Hostnames: []string{"tower.local"},
			MachineID: "machine-1",
		},
		DiscoveryTarget: &DiscoveryTarget{
			ResourceType: "agent",
			AgentID:      "host-1",
			ResourceID:   "host-1",
		},
		MetricsTarget: &MetricsTarget{
			ResourceType: "docker-host",
			ResourceID:   "docker-runtime-1",
		},
		Proxmox: &ProxmoxData{
			NodeName: "pve1",
		},
		Agent: &AgentData{
			AgentID:  "host-1",
			Hostname: "tower-agent.local",
		},
		Docker: &DockerData{
			HostSourceID: "docker-runtime-1",
			Hostname:     "tower-docker.local",
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.DisplayName; got != "Tower" {
		t.Fatalf("displayName = %q, want Tower", got)
	}
	if got := resource.Canonical.Hostname; got != "tower.local" {
		t.Fatalf("hostname = %q, want tower.local", got)
	}
	if got := resource.Canonical.PlatformID; got != "pve1" {
		t.Fatalf("platformId = %q, want pve1", got)
	}
	if got := resource.Canonical.PrimaryID; got != "docker-host:docker-runtime-1" {
		t.Fatalf("primaryId = %q, want docker-host:docker-runtime-1", got)
	}

	wantAliases := []string{
		"docker-host:docker-runtime-1",
		"docker-runtime-1",
		"host-1",
		"pve1",
		"tower.local",
		"machine-1",
		"agent-1",
	}
	if len(resource.Canonical.Aliases) != len(wantAliases) {
		t.Fatalf("aliases len = %d, want %d (%v)", len(resource.Canonical.Aliases), len(wantAliases), resource.Canonical.Aliases)
	}
	for i, want := range wantAliases {
		if got := resource.Canonical.Aliases[i]; got != want {
			t.Fatalf("alias[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestRefreshCanonicalIdentityFallsBackWithoutTargets(t *testing.T) {
	resource := Resource{
		ID:   "pbs-1",
		Type: ResourceTypePBS,
		Name: "",
		PBS: &PBSData{
			InstanceID: "pbs-main",
			Hostname:   "pbs.example",
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.DisplayName; got != "pbs.example" {
		t.Fatalf("displayName = %q, want pbs.example", got)
	}
	if got := resource.Canonical.Hostname; got != "pbs.example" {
		t.Fatalf("hostname = %q, want pbs.example", got)
	}
	if got := resource.Canonical.PlatformID; got != "pbs.example" {
		t.Fatalf("platformId = %q, want pbs.example", got)
	}
	if got := resource.Canonical.PrimaryID; got != "pbs:pbs-main" {
		t.Fatalf("primaryId = %q, want pbs:pbs-main", got)
	}
}

func TestRefreshCanonicalIdentityPrefersProxmoxNodePrimaryIDForAgentResources(t *testing.T) {
	resource := Resource{
		ID:   "agent-1",
		Type: ResourceTypeAgent,
		Name: "pve1",
		Proxmox: &ProxmoxData{
			SourceID: "instance-pve1",
			NodeName: "pve1",
		},
		DiscoveryTarget: &DiscoveryTarget{
			ResourceType: "agent",
			ResourceID:   "host-1",
			AgentID:      "host-1",
		},
		Agent: &AgentData{
			AgentID:  "host-1",
			Hostname: "pve1",
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.PrimaryID; got != "node:instance-pve1" {
		t.Fatalf("primaryId = %q, want node:instance-pve1", got)
	}
}
