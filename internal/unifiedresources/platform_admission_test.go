package unifiedresources

import "testing"

func agentResource(id string, sources ...DataSource) Resource {
	return Resource{ID: id, Type: ResourceTypeAgent, Sources: sources, Agent: &AgentData{}}
}

func TestBuildPlatformAdmission_ProviderOwnedAgentDoesNotAdmitStandalone(t *testing.T) {
	// A TrueNAS host reports through the agent source and carries the "agent"
	// platform scope, so any count-based derivation would admit the standalone
	// page here. It is owned by TrueNAS and must not.
	truenasHost := agentResource("truenas-host", SourceAgent, SourceTrueNAS)
	truenasHost.TrueNAS = &TrueNASData{}
	RefreshPlatformScopes(&truenasHost)

	admission := BuildPlatformAdmission([]Resource{truenasHost})
	if admission.Standalone {
		t.Fatalf("provider-owned agent must not admit standalone, got %+v", admission)
	}
	if !admission.TrueNAS {
		t.Fatalf("expected truenas admission, got %+v", admission)
	}
}

func TestBuildPlatformAdmission_GenuinePulseAgentAdmitsStandalone(t *testing.T) {
	host := agentResource("pulse-host", SourceAgent)
	RefreshPlatformScopes(&host)

	admission := BuildPlatformAdmission([]Resource{host})
	if !admission.Standalone {
		t.Fatalf("genuine pulse agent must admit standalone, got %+v", admission)
	}
}

func TestBuildPlatformAdmission_ProxmoxOwnedAgentDoesNotAdmitStandalone(t *testing.T) {
	node := agentResource("pve-node", SourceAgent, SourceProxmox)
	node.Proxmox = &ProxmoxData{}
	RefreshPlatformScopes(&node)

	admission := BuildPlatformAdmission([]Resource{node})
	if admission.Standalone {
		t.Fatalf("proxmox-owned agent must not admit standalone, got %+v", admission)
	}
	if !admission.Proxmox {
		t.Fatalf("expected proxmox admission, got %+v", admission)
	}
}

func TestBuildPlatformAdmission_SourceStatusEvidenceCountsAsProviderOwnership(t *testing.T) {
	host := agentResource("vsphere-host", SourceAgent)
	host.SourceStatus = map[DataSource]SourceStatus{SourceVMware: {}}
	RefreshPlatformScopes(&host)

	if IsPulseAgentPlatformResource(host) {
		t.Fatalf("source-status provider evidence must disqualify the standalone claim")
	}
}

func TestBuildPlatformAdmission_AvailabilityEndpointAdmitsStandalone(t *testing.T) {
	endpoint := Resource{ID: "check-1", Type: ResourceTypeNetworkEndpoint, Sources: []DataSource{SourceAvailability}}
	RefreshPlatformScopes(&endpoint)

	admission := BuildPlatformAdmission([]Resource{endpoint})
	if !admission.Standalone {
		t.Fatalf("availability endpoint must admit standalone, got %+v", admission)
	}
}

func TestBuildPlatformAdmission_EmptyEstateAdmitsNothing(t *testing.T) {
	if admission := BuildPlatformAdmission(nil); admission != (PlatformAdmission{}) {
		t.Fatalf("empty estate must admit nothing, got %+v", admission)
	}
}

func TestBuildPlatformAdmission_DerivesScopesWhenNotRefreshed(t *testing.T) {
	// Admission must not depend on whether the caller refreshed scopes first.
	unrefreshed := Resource{ID: "k8s-node", Type: ResourceTypeAgent, Sources: []DataSource{SourceK8s}}
	admission := BuildPlatformAdmission([]Resource{unrefreshed})
	if !admission.Kubernetes {
		t.Fatalf("expected kubernetes admission from unrefreshed resource, got %+v", admission)
	}
	if admission.Standalone {
		t.Fatalf("kubernetes-owned agent must not admit standalone, got %+v", admission)
	}
}

func TestBuildPlatformAdmission_MixedEstate(t *testing.T) {
	pulseHost := agentResource("pulse-host", SourceAgent)
	truenasHost := agentResource("truenas-host", SourceAgent, SourceTrueNAS)
	truenasHost.TrueNAS = &TrueNASData{}
	dockerHost := Resource{ID: "docker-1", Type: ResourceTypeDockerService, Sources: []DataSource{SourceDocker}, Docker: &DockerData{}}
	for _, r := range []*Resource{&pulseHost, &truenasHost, &dockerHost} {
		RefreshPlatformScopes(r)
	}

	admission := BuildPlatformAdmission([]Resource{pulseHost, truenasHost, dockerHost})
	want := PlatformAdmission{Docker: true, TrueNAS: true, Standalone: true}
	if admission != want {
		t.Fatalf("admission = %+v, want %+v", admission, want)
	}
}
