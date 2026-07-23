package unifiedresources

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

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

func TestCloneDockerDataPreservesContainerRuntimeMetadata(t *testing.T) {
	startedAt := time.Date(2026, 6, 11, 13, 15, 30, 0, time.UTC)
	finishedAt := startedAt.Add(45 * time.Minute)
	original := &DockerData{
		ContainerID: "container-1",
		StartedAt:   &startedAt,
		FinishedAt:  &finishedAt,
		BlockIO: &DockerContainerBlockIOMeta{
			ReadBytes:  9_876_543,
			WriteBytes: 1_234_567,
		},
		Podman: &DockerPodmanContainerMeta{
			PodName:          "edge-pod",
			PodID:            "pod-123",
			Infra:            true,
			ComposeProject:   "orion",
			ComposeService:   "web",
			AutoUpdatePolicy: "registry",
			UserNamespace:    "keep-id",
		},
	}

	cloned := cloneDockerData(original)
	if cloned == nil {
		t.Fatal("expected docker clone")
	}
	if cloned.StartedAt == nil || !cloned.StartedAt.Equal(startedAt) {
		t.Fatalf("startedAt = %v, want %v", cloned.StartedAt, startedAt)
	}
	if cloned.FinishedAt == nil || !cloned.FinishedAt.Equal(finishedAt) {
		t.Fatalf("finishedAt = %v, want %v", cloned.FinishedAt, finishedAt)
	}
	if cloned.BlockIO == nil {
		t.Fatal("expected block IO clone")
	}
	if got, want := cloned.BlockIO.ReadBytes, original.BlockIO.ReadBytes; got != want {
		t.Fatalf("blockIo.readBytes = %d, want %d", got, want)
	}
	if cloned.Podman == nil {
		t.Fatal("expected podman clone")
	}
	if got, want := cloned.Podman.ComposeProject, original.Podman.ComposeProject; got != want {
		t.Fatalf("podman.composeProject = %q, want %q", got, want)
	}

	*cloned.StartedAt = cloned.StartedAt.Add(time.Hour)
	cloned.BlockIO.ReadBytes = 1
	cloned.Podman.ComposeProject = "mutated"
	if !original.StartedAt.Equal(startedAt) {
		t.Fatalf("original startedAt mutated to %v", original.StartedAt)
	}
	if original.BlockIO.ReadBytes != 9_876_543 {
		t.Fatalf("original blockIo.readBytes mutated to %d", original.BlockIO.ReadBytes)
	}
	if original.Podman.ComposeProject != "orion" {
		t.Fatalf("original podman.composeProject mutated to %q", original.Podman.ComposeProject)
	}
}

func TestMergeTrueNASDataPreservesNativeAppFacetAsClone(t *testing.T) {
	existing := &TrueNASData{
		Hostname: "truenas-a.local",
		Version:  "25.04.1",
		App: &TrueNASApp{
			ID:   "old-app",
			Name: "Old App",
		},
	}
	incoming := &TrueNASData{
		Hostname: "truenas-b.local",
		App: &TrueNASApp{
			ID:       "nextcloud",
			Name:     "Nextcloud",
			State:    "RUNNING",
			Images:   []string{"nextcloud:stable"},
			Volumes:  []TrueNASAppVolume{{Source: "ix-apps/nextcloud", Destination: "/data"}},
			Networks: []TrueNASAppNetwork{{Name: "ix-nextcloud", Labels: map[string]string{"app": "nextcloud"}}},
			UsedPorts: []TrueNASAppPort{
				{
					ContainerPort: 8080,
					Protocol:      "tcp",
					HostPorts:     []TrueNASAppHostPort{{HostIP: "0.0.0.0", HostPort: 30080}},
				},
			},
			Containers: []TrueNASAppContainer{
				{
					ID:          "container-1",
					ServiceName: "web",
					Image:       "nextcloud:stable",
					PortConfig: []TrueNASAppPort{
						{
							ContainerPort: 8080,
							Protocol:      "tcp",
							HostPorts:     []TrueNASAppHostPort{{HostIP: "0.0.0.0", HostPort: 30080}},
						},
					},
					VolumeMounts: []TrueNASAppVolume{{Source: "ix-apps/nextcloud", Destination: "/data"}},
				},
			},
		},
		VM: &TrueNASVM{
			ID:          "42",
			Name:        "windows-lab",
			State:       "RUNNING",
			VCPUs:       4,
			MemoryBytes: 8 * 1024 * 1024 * 1024,
		},
		Share: &TrueNASShare{
			ID:       "smb-media",
			Name:     "Media",
			Protocol: "SMB",
			Path:     "/mnt/tank/media",
			Dataset:  "tank/media",
			Enabled:  true,
			Aliases:  []string{"media"},
			Hosts:    []string{"media.lab"},
			Networks: []string{"10.10.20.0/24"},
			Security: []string{"SYS"},
		},
		Services: []TrueNASService{
			{ID: "1", Service: "smb", Enabled: true, State: "RUNNING", PIDs: []int{2418, 2420}},
		},
	}

	merged := mergeTrueNASData(existing, incoming)
	if merged == nil || merged.App == nil {
		t.Fatalf("expected merged TrueNAS app facet")
	}
	if got := merged.Hostname; got != "truenas-b.local" {
		t.Fatalf("hostname = %q, want incoming hostname", got)
	}
	if got := merged.Version; got != "25.04.1" {
		t.Fatalf("version = %q, want existing version preserved", got)
	}
	if got := merged.App.ID; got != "nextcloud" {
		t.Fatalf("app id = %q, want nextcloud", got)
	}
	if merged.VM == nil || merged.VM.ID != "42" || merged.VM.Name != "windows-lab" {
		t.Fatalf("unexpected merged TrueNAS VM facet: %+v", merged.VM)
	}
	if merged.VM == incoming.VM {
		t.Fatal("expected merged TrueNAS VM facet to be cloned")
	}
	if merged.Share == nil || merged.Share.ID != "smb-media" || merged.Share.Dataset != "tank/media" {
		t.Fatalf("unexpected merged TrueNAS share facet: %+v", merged.Share)
	}
	if merged.Share == incoming.Share {
		t.Fatal("expected merged TrueNAS share facet to be cloned")
	}
	if len(merged.Services) != 1 || merged.Services[0].Service != "smb" || len(merged.Services[0].PIDs) != 2 {
		t.Fatalf("unexpected merged TrueNAS services: %+v", merged.Services)
	}

	incoming.App.Images[0] = "mutated:latest"
	incoming.App.Volumes[0].Source = "mutated"
	incoming.App.Networks[0].Labels["app"] = "mutated"
	incoming.App.UsedPorts[0].HostPorts[0].HostPort = 39999
	incoming.App.Containers[0].PortConfig[0].HostPorts[0].HostPort = 39999
	incoming.App.Containers[0].VolumeMounts[0].Source = "mutated"
	incoming.Share.Aliases[0] = "mutated"
	incoming.Share.Hosts[0] = "mutated"
	incoming.Share.Networks[0] = "mutated"
	incoming.Share.Security[0] = "mutated"
	incoming.Services[0].PIDs[0] = 9999

	if got := merged.App.Images[0]; got != "nextcloud:stable" {
		t.Fatalf("merged app image mutated through incoming slice: %q", got)
	}
	if got := merged.App.Volumes[0].Source; got != "ix-apps/nextcloud" {
		t.Fatalf("merged app volume mutated through incoming slice: %q", got)
	}
	if got := merged.App.Networks[0].Labels["app"]; got != "nextcloud" {
		t.Fatalf("merged app network labels mutated through incoming map: %q", got)
	}
	if got := merged.App.UsedPorts[0].HostPorts[0].HostPort; got != 30080 {
		t.Fatalf("merged app host port mutated through incoming slice: %d", got)
	}
	if got := merged.App.Containers[0].PortConfig[0].HostPorts[0].HostPort; got != 30080 {
		t.Fatalf("merged app container port config mutated through incoming slice: %d", got)
	}
	if got := merged.App.Containers[0].VolumeMounts[0].Source; got != "ix-apps/nextcloud" {
		t.Fatalf("merged app container volume mount mutated through incoming slice: %q", got)
	}
	if got := merged.Share.Aliases[0]; got != "media" {
		t.Fatalf("merged share alias mutated through incoming slice: %q", got)
	}
	if got := merged.Share.Hosts[0]; got != "media.lab" {
		t.Fatalf("merged share host mutated through incoming slice: %q", got)
	}
	if got := merged.Share.Networks[0]; got != "10.10.20.0/24" {
		t.Fatalf("merged share network mutated through incoming slice: %q", got)
	}
	if got := merged.Share.Security[0]; got != "SYS" {
		t.Fatalf("merged share security mutated through incoming slice: %q", got)
	}
	if got := merged.Services[0].PIDs[0]; got != 2418 {
		t.Fatalf("merged service PIDs mutated through incoming slice: %d", got)
	}
}

func TestTrueNASNetworkSharePolicyAndParentRelationship(t *testing.T) {
	parentID := "storage:truenas:tank/media"
	resource := Resource{
		ID:       "network-share:truenas-main:smb:media",
		Type:     ResourceTypeNetworkShare,
		Name:     "SMB Media",
		ParentID: &parentID,
		Status:   StatusOnline,
		LastSeen: time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
		Sources:  []DataSource{SourceTrueNAS},
		TrueNAS: &TrueNASData{
			Share: &TrueNASShare{
				ID:       "smb-media",
				Name:     "Media",
				Protocol: "SMB",
				Path:     "/mnt/tank/media",
				Dataset:  "tank/media",
				Enabled:  true,
			},
		},
	}

	RefreshPolicyMetadata(&resource)
	if resource.Policy == nil || resource.Policy.Sensitivity != ResourceSensitivitySensitive {
		t.Fatalf("network-share policy = %+v, want sensitive", resource.Policy)
	}
	if !containsRedactionHint(resource.Policy.Routing.Redact, ResourceRedactionPath) {
		t.Fatalf("network-share policy redactions = %+v, want path", resource.Policy.Routing.Redact)
	}
	if !strings.Contains(resource.AISafeSummary, "network share resource") {
		t.Fatalf("network-share AI safe summary = %q", resource.AISafeSummary)
	}

	relationships := ResourceRelationshipsWithCanonicalParent(resource)
	if len(relationships) != 1 {
		t.Fatalf("expected one parent relationship, got %#v", relationships)
	}
	if got := relationships[0].Type; got != RelMountedTo {
		t.Fatalf("network-share parent relationship = %q, want %q", got, RelMountedTo)
	}
	if relationships[0].TargetID != parentID {
		t.Fatalf("network-share parent target = %q, want %q", relationships[0].TargetID, parentID)
	}
}

func TestComputeWorkloadPolicyIsInternalUnlessEscalated(t *testing.T) {
	// Recalibrated default: ordinary compute workloads are Internal (cloud-summary,
	// no redaction) so the cloud Assistant can see their names/IPs. Only a tag (or
	// a genuinely sensitive type) escalates them to Sensitive/redacted.
	plain := Resource{
		ID:     "vm-100",
		Name:   "grafana",
		Type:   ResourceTypeVM,
		Status: StatusOnline,
		Identity: ResourceIdentity{
			Hostnames:   []string{"grafana.lan"},
			IPAddresses: []string{"192.168.1.20"},
		},
	}
	RefreshPolicyMetadata(&plain)
	if plain.Policy == nil || plain.Policy.Sensitivity != ResourceSensitivityInternal {
		t.Fatalf("plain VM policy = %+v, want Internal", plain.Policy)
	}
	if plain.Policy.Routing.Scope != ResourceRoutingScopeCloudSummary {
		t.Fatalf("plain VM routing = %q, want cloud-summary", plain.Policy.Routing.Scope)
	}
	if len(plain.Policy.Routing.Redact) != 0 {
		t.Fatalf("plain VM should not redact, got %+v", plain.Policy.Routing.Redact)
	}

	escalated := plain
	escalated.Tags = []string{"database"}
	RefreshPolicyMetadata(&escalated)
	if escalated.Policy.Sensitivity != ResourceSensitivitySensitive {
		t.Fatalf("database-tagged VM = %q, want Sensitive", escalated.Policy.Sensitivity)
	}
	if !containsRedactionHint(escalated.Policy.Routing.Redact, ResourceRedactionHostname) {
		t.Fatalf("escalated VM should redact hostname, got %+v", escalated.Policy.Routing.Redact)
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

func TestRegistryMemoryUnavailableClearsOnlySameSourceMetric(t *testing.T) {
	total := int64(8 << 30)
	used := total / 2
	unavailable := &AgentMemoryMeta{
		Total:            total,
		UsageUnavailable: true,
	}
	lastSeen := time.Now().UTC()

	registry := NewRegistry(nil)
	registry.IngestRecords(SourceAgent, []IngestRecord{{
		SourceID: "agent-1501",
		Resource: Resource{
			Type:     ResourceTypeAgent,
			Name:     "linux-host",
			Status:   StatusOnline,
			LastSeen: lastSeen,
			Metrics: &ResourceMetrics{Memory: &MetricValue{
				Used:    &used,
				Total:   &total,
				Percent: 50,
				Source:  SourceAgent,
			}},
			Agent: &AgentData{Memory: &AgentMemoryMeta{Total: total, Used: used}},
		},
	}})
	registry.IngestRecords(SourceAgent, []IngestRecord{{
		SourceID: "agent-1501",
		Resource: Resource{
			Type:     ResourceTypeAgent,
			Name:     "linux-host",
			Status:   StatusOnline,
			LastSeen: lastSeen.Add(time.Second),
			Metrics:  &ResourceMetrics{},
			Agent:    &AgentData{Memory: unavailable},
		},
	}})

	resources := registry.ListByType(ResourceTypeAgent)
	if len(resources) != 1 || resources[0].Metrics == nil || resources[0].Metrics.Memory != nil {
		t.Fatalf("same-source resources = %+v, want stale memory metric cleared", resources)
	}

	proxmoxUnavailable := &Resource{
		Type:     ResourceTypeVM,
		Proxmox:  &ProxmoxData{Memory: &models.Memory{Total: total, UsageUnavailable: true}},
		Metrics:  &ResourceMetrics{},
		LastSeen: lastSeen.Add(2 * time.Second),
	}
	trusted := &ResourceMetrics{Memory: &MetricValue{
		Used:    &used,
		Total:   &total,
		Percent: 50,
		Source:  SourceAgent,
	}}
	merged := mergeMetrics(
		proxmoxUnavailable,
		trusted,
		proxmoxUnavailable.Metrics,
		SourceProxmox,
		proxmoxUnavailable.LastSeen,
		nil,
		nil,
	)
	if merged.Memory == nil || merged.Memory.Percent != 50 || merged.Memory.Source != SourceAgent {
		t.Fatalf("cross-source memory = %+v, want trusted agent metric preserved", merged.Memory)
	}
}
