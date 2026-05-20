package unifiedresources

import (
	"strings"
	"testing"
	"time"
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

func containsDataSource(sources []DataSource, want DataSource) bool {
	for _, source := range sources {
		if source == want {
			return true
		}
	}
	return false
}
