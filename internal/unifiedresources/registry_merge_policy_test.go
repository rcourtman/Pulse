package unifiedresources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
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

func onlyResourceOfType(t *testing.T, rr *ResourceRegistry, resourceType ResourceType) Resource {
	t.Helper()
	resources := rr.ListByType(resourceType)
	if len(resources) != 1 {
		t.Fatalf("expected one %s resource, got %d: %+v", resourceType, len(resources), resources)
	}
	return resources[0]
}

// A host agent past its reporting lease is offline. Its source sighting is
// stale too, and the stale pass used to rank that above offline, so the
// machine reached the frontend as a warning with its last report rendered
// as current readings.
func TestLeaseExpiredHostAgentStaysOffline(t *testing.T) {
	lastReport := time.Now().UTC().Add(-14 * time.Minute)
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{
			ID:              "host-silent",
			MachineID:       "machine-silent",
			Hostname:        "silent",
			Status:          "offline",
			LastSeen:        lastReport,
			IntervalSeconds: 30,
			CPUUsage:        32,
		}},
	})

	resource := onlyResourceOfType(t, rr, ResourceTypeAgent)
	if resource.Status != StatusOffline {
		t.Fatalf("status = %q, want offline for an agent past its lease", resource.Status)
	}
	if resource.Agent == nil || !resource.Agent.Stale {
		t.Fatalf("expected the agent facet to stay flagged stale, got %+v", resource.Agent)
	}
	// Status describes the resource; the sighting keeps describing delivery
	// freshness, which health and monitored-system reasons read.
	if got := resource.SourceStatus[SourceAgent].Status; got != "stale" {
		t.Fatalf("agent sighting = %q, want stale", got)
	}

	// The monitor adapter runs the stale pass again after record sources.
	rr.MarkStale(time.Now().UTC(), nil)
	if got, _ := rr.Get(resource.ID); got.Status != StatusOffline {
		t.Fatalf("second stale pass changed status to %q, want offline", got.Status)
	}

	health := EvaluateResourceHealth(resource, nil, time.Now().UTC())
	if health.Verdict != HealthCritical || len(health.Reasons) < 2 ||
		health.Reasons[0].Code != "offline" || health.Reasons[1].Code != "telemetry_stale" {
		t.Fatalf("health = %+v, want critical offline with the stale reason after it", health)
	}
}

// An agent that is late but still inside its lease keeps the warning: the
// monitor has not decided it is gone, so its last readings are only stale.
func TestLateHostAgentInsideLeaseStaysWarning(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{
			ID:              "host-late",
			MachineID:       "machine-late",
			Hostname:        "late",
			Status:          "online",
			LastSeen:        time.Now().UTC().Add(-90 * time.Second),
			IntervalSeconds: 30,
		}},
	})

	resource := onlyResourceOfType(t, rr, ResourceTypeAgent)
	if resource.Status != StatusWarning {
		t.Fatalf("status = %q, want warning for a late agent inside its lease", resource.Status)
	}
}

// A Proxmox node whose linked agent stopped reporting stays online through
// the PVE poll (issue #1515). Only the agent sighting carries the lease.
func TestLeaseExpiredAgentOnLiveProxmoxNodeStaysOnline(t *testing.T) {
	now := time.Now().UTC()
	const machineID = "machine-pve1"
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{{
			ID:          "homelab-pve1",
			Name:        "pve1",
			Instance:    "homelab",
			ClusterName: "homelab",
			Status:      "online",
			LastSeen:    now,
		}},
		Hosts: []models.Host{{
			ID:              machineID,
			MachineID:       machineID,
			Hostname:        "pve1",
			LinkedNodeID:    "homelab-pve1",
			Status:          "offline",
			LastSeen:        now.Add(-14 * time.Minute),
			IntervalSeconds: 30,
		}},
	})

	resource := onlyResourceOfType(t, rr, ResourceTypeAgent)
	if _, ok := resource.SourceStatus[SourceProxmox]; !ok {
		t.Fatalf("expected the node and its agent to share one resource, got sources %+v", resource.SourceStatus)
	}
	if resource.Status != StatusOnline {
		t.Fatalf("status = %q, want online through the live PVE poll", resource.Status)
	}
	if resource.Agent == nil || !resource.Agent.Stale {
		t.Fatalf("expected the dead agent to stay flagged stale, got %+v", resource.Agent)
	}
}

// Docker hosts hold a shorter lease than the registry's Docker stale
// threshold, so a silent Docker host was offline for a while and then
// flipped to warning once its sighting went stale.
func TestLeaseExpiredDockerHostStaysOfflineAfterSightingGoesStale(t *testing.T) {
	lastReport := time.Now().UTC().Add(-5 * time.Minute)
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID:              "docker-silent",
			AgentID:         "docker-agent-silent",
			Hostname:        "docker-silent",
			MachineID:       "machine-docker-silent",
			Status:          "offline",
			LastSeen:        lastReport,
			IntervalSeconds: 30,
			Containers: []models.DockerContainer{{
				ID:    "container-web",
				Name:  "web",
				State: "running",
			}},
		}},
	})

	host := onlyResourceOfType(t, rr, ResourceTypeAgent)
	if host.Status != StatusOffline {
		t.Fatalf("docker host status = %q, want offline after its sighting went stale", host.Status)
	}
	if got := host.SourceStatus[SourceDocker].Status; got != "stale" {
		t.Fatalf("docker sighting = %q, want stale", got)
	}

	// Containers are what the host reports about, not lease holders: a
	// running container on a silent host is stale, not known to be down.
	container := onlyResourceOfType(t, rr, ResourceTypeAppContainer)
	if container.Status != StatusWarning {
		t.Fatalf("container status = %q, want warning on a silent host", container.Status)
	}
}

// A machine reporting through both the host agent and Docker is offline once
// both leases expire. The Docker lease ends first, so its sighting can still
// be fresh when the agent's goes stale.
func TestMachineWithExpiredAgentAndDockerLeasesIsOffline(t *testing.T) {
	now := time.Now().UTC()
	const machineID = "machine-both"
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{
			ID:              "host-both",
			MachineID:       machineID,
			Hostname:        "both",
			Status:          "offline",
			LastSeen:        now.Add(-3 * time.Minute),
			IntervalSeconds: 30,
		}},
		DockerHosts: []models.DockerHost{{
			ID:              "docker-both",
			AgentID:         "host-both",
			Hostname:        "both",
			MachineID:       machineID,
			Status:          "offline",
			LastSeen:        now.Add(-90 * time.Second),
			IntervalSeconds: 10,
		}},
	})

	resource := onlyResourceOfType(t, rr, ResourceTypeAgent)
	if _, ok := resource.SourceStatus[SourceDocker]; !ok {
		t.Fatalf("expected agent and Docker to share one resource, got sources %+v", resource.SourceStatus)
	}
	if got := resource.SourceStatus[SourceDocker].Status; got != "online" {
		t.Fatalf("docker sighting = %q, want it still inside the stale threshold", got)
	}
	if resource.Status != StatusOffline {
		t.Fatalf("status = %q, want offline once both leases expired", resource.Status)
	}
}

// A live reporter still carries the machine when the other lease expired.
func TestMachineWithLiveDockerReporterStaysOnline(t *testing.T) {
	now := time.Now().UTC()
	const machineID = "machine-split"
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{
			ID:              "host-split",
			MachineID:       machineID,
			Hostname:        "split",
			Status:          "offline",
			LastSeen:        now.Add(-14 * time.Minute),
			IntervalSeconds: 30,
		}},
		DockerHosts: []models.DockerHost{{
			ID:              "docker-split",
			AgentID:         "docker-agent-split",
			Hostname:        "split",
			MachineID:       machineID,
			Status:          "online",
			LastSeen:        now,
			IntervalSeconds: 10,
		}},
	})

	resource := onlyResourceOfType(t, rr, ResourceTypeAgent)
	if resource.Status != StatusOnline {
		t.Fatalf("status = %q, want online through the live Docker reporter", resource.Status)
	}
}

// The lease marker is unexported, so a resource that went through JSON (a
// persisted or remote copy) must recover it from its stored status.
func TestLeaseExpiredHostAgentStaysOfflineAfterJSONRoundTrip(t *testing.T) {
	source := NewRegistry(nil)
	source.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{
			ID:              "host-silent",
			MachineID:       "machine-silent",
			Hostname:        "silent",
			Status:          "offline",
			LastSeen:        time.Now().UTC().Add(-14 * time.Minute),
			IntervalSeconds: 30,
		}},
	})
	payload, err := json.Marshal(source.List())
	if err != nil {
		t.Fatalf("marshal resources: %v", err)
	}
	var copied []Resource
	if err := json.Unmarshal(payload, &copied); err != nil {
		t.Fatalf("unmarshal resources: %v", err)
	}

	rr := NewRegistry(nil)
	rr.IngestResources(copied)

	resource := onlyResourceOfType(t, rr, ResourceTypeAgent)
	if resource.Status != StatusOffline {
		t.Fatalf("status = %q, want offline after a JSON round trip", resource.Status)
	}
}

// A manual link can join two resources reported by the same source, such as
// an agent reinstalled under a new id. The expired sighting of the old one
// must not replace the live sighting of the new one.
func TestManualLinkKeepsTheLiveSightingOfASharedSource(t *testing.T) {
	now := time.Now().UTC()
	store := NewMemoryStore()
	if err := store.AddLink(ResourceLink{
		ResourceA: "agent-reinstalled",
		ResourceB: "agent-retired",
		PrimaryID: "agent-reinstalled",
	}); err != nil {
		t.Fatalf("add link: %v", err)
	}
	rr := NewRegistry(store)
	rr.IngestResources([]Resource{
		{
			ID:       "agent-reinstalled",
			Type:     ResourceTypeAgent,
			Name:     "tower",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceAgent},
			SourceStatus: map[DataSource]SourceStatus{
				SourceAgent: {Status: "online", LastSeen: now},
			},
			Agent: &AgentData{AgentID: "host-new", Hostname: "tower"},
		},
		{
			ID:       "agent-retired",
			Type:     ResourceTypeAgent,
			Name:     "tower",
			Status:   StatusOffline,
			LastSeen: now.Add(-14 * time.Minute),
			Sources:  []DataSource{SourceAgent},
			SourceStatus: map[DataSource]SourceStatus{
				SourceAgent: {Status: "stale", LastSeen: now.Add(-14 * time.Minute)},
			},
			Agent: &AgentData{AgentID: "host-old", Hostname: "tower", Stale: true},
		},
	})

	resource, ok := rr.Get("agent-reinstalled")
	if !ok {
		t.Fatal("linked resource missing")
	}
	if resource.Status != StatusOnline {
		t.Fatalf("status = %q, want online through the live reinstalled agent", resource.Status)
	}
	if got := resource.SourceStatus[SourceAgent]; !got.LastSeen.Equal(now) {
		t.Fatalf("agent sighting = %+v, want the live sighting", got)
	}
}

func TestLeaseExpiredKubernetesClusterStaysOffline(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:              "cluster-silent",
			AgentID:         "k8s-agent-silent",
			Name:            "silent",
			Status:          "offline",
			LastSeen:        time.Now().UTC().Add(-5 * time.Minute),
			IntervalSeconds: 30,
		}},
	})

	cluster := onlyResourceOfType(t, rr, ResourceTypeK8sCluster)
	if cluster.Status != StatusOffline {
		t.Fatalf("cluster status = %q, want offline after its sighting went stale", cluster.Status)
	}
}

// Pull sources keep the stale-to-warning rule: a stale PVE sighting means
// Pulse lost its poller, not that the node reported itself gone.
func TestStaleProxmoxNodeStaysWarning(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{{
			ID:          "homelab-pve9",
			Name:        "pve9",
			Instance:    "homelab",
			ClusterName: "homelab",
			Status:      "online",
			LastSeen:    time.Now().UTC().Add(-5 * time.Minute),
		}},
	})

	node := onlyResourceOfType(t, rr, ResourceTypeAgent)
	if node.Status != StatusWarning {
		t.Fatalf("node status = %q, want warning for a stale PVE sighting", node.Status)
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

func TestProxmoxInferenceRejectsSharedHostLocalNetworks(t *testing.T) {
	for _, tc := range []struct{ name, nic, address string }{
		{"loopback", "lo", "127.0.0.1/8"},
		{"link-local", "eth0", "fe80::1/64"},
		{"docker", "docker0", "172.17.0.1/16"},
		{"generated-bridge", "br-0123456789ab", "192.0.2.1/24"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			networks := []models.HostNetworkInterface{{Name: tc.nic, Addresses: []string{tc.address}}}
			host := &models.Host{ID: "agent", Hostname: "pve", LinkedNodeID: "site-a", NetworkInterfaces: networks}
			left := models.Node{ID: "site-a", Name: "pve", Instance: "a", Host: "https://a.example:8006", LinkedAgentID: host.ID, NetworkInterfaces: networks}
			right := models.Node{ID: "site-b", Name: "pve", Instance: "b", Host: "https://b.example:8006", NetworkInterfaces: networks}
			got := inferLinkedHostsForProxmoxNodes([]models.Node{left, right}, map[string]*models.Host{host.ID: host})
			if got[left.ID] != host {
				t.Fatal("lost explicit reciprocal link")
			}
			registry := NewRegistry(NewMemoryStore())
			registry.IngestSnapshot(models.StateSnapshot{Nodes: []models.Node{left, right}, Hosts: []models.Host{*host}})
			seenUnrelatedProvider := false
			for _, resource := range registry.ListForPresentation() {
				if resource.Proxmox != nil && resource.Proxmox.SourceID == right.ID {
					seenUnrelatedProvider = true
				}
				if resource.Proxmox != nil && resource.Proxmox.SourceID == right.ID && resource.Agent != nil && resource.Agent.AgentID == host.ID {
					t.Fatal("presentation attached linked agent to unrelated provider")
				}
			}
			if !seenUnrelatedProvider {
				t.Fatal("unrelated provider disappeared from presentation")
			}
			if got[right.ID] != nil {
				t.Fatal("shared host-local network lent agent identity to another provider")
			}
		})
	}
}

func TestProxmoxInferencePreservesManagementNetworkCorroboration(t *testing.T) {
	for _, nic := range []string{"eth0", "vmbr0", "br-mgmt", ""} {
		t.Run(nic, func(t *testing.T) {
			networks := []models.HostNetworkInterface{{Name: nic, Addresses: []string{"172.17.1.2/24"}}}
			host := &models.Host{ID: "agent", Hostname: "pve", LinkedNodeID: "a", NetworkInterfaces: networks}
			left := models.Node{ID: "a", Name: "pve", Instance: "a", Host: "https://a.example:8006", LinkedAgentID: host.ID, NetworkInterfaces: networks}
			right := models.Node{ID: "b", Name: "pve", Instance: "b", Host: "https://b.example:8006", NetworkInterfaces: networks}
			got := inferLinkedHostsForProxmoxNodes([]models.Node{left, right}, map[string]*models.Host{host.ID: host})
			if got[left.ID] != host || got[right.ID] != host {
				t.Fatal("lost corroborated duplicate provider")
			}
		})
	}
}

func TestProxmoxOneWayLinkRejectsHostLocalEndpointCorroboration(t *testing.T) {
	host := models.Host{ID: "agent", Hostname: "nas", NetworkInterfaces: []models.HostNetworkInterface{{Name: "docker0", Addresses: []string{"172.17.0.1/16"}}}}
	node := models.Node{ID: "node", Name: "pve", Host: "https://172.17.0.1:8006", LinkedAgentID: host.ID}
	if trustedProxmoxNodeHostLink(node, host) {
		t.Fatal("uncorroborated one-way link trusted via Docker bridge")
	}
	host.LinkedNodeID = node.ID
	if !trustedProxmoxNodeHostLink(node, host) {
		t.Fatal("reciprocal explicit link lost")
	}
	host.LinkedNodeID = ""
	host.ReportIP = "172.17.0.1"
	if !trustedProxmoxNodeHostLink(node, host) {
		t.Fatal("explicit private report-IP corroboration lost")
	}
}

// The Proxmox disk row carries a copy of the agent's collection state from the
// last disk poll. When the agent's own row later withdraws a reading (its
// reporting lease expired), the canonical disk must follow the agent in either
// ingest order, even if no disk poll has refreshed the Proxmox copy (a host that
// is down entirely is never disk-polled).
func TestPhysicalDiskMergeFollowsAgentWithdrawalOfItsOwnReadings(t *testing.T) {
	proxmoxDisk := func(agent diskinventory.FieldStatus) Resource {
		return Resource{
			Type: ResourceTypePhysicalDisk, Name: "WDC", Status: StatusOnline,
			PhysicalDisk: &PhysicalDiskMeta{
				DevPath: "/dev/sdb", Serial: "WD-SILENT1", Temperature: 41,
				Collection: &diskinventory.CollectionStatus{
					Serial:      diskinventory.Available("proxmox_disks"),
					Temperature: agent,
					Pool:        diskinventory.Available("proxmox_zfs"),
				},
			},
		}
	}
	agentDisk := func(temperature diskinventory.FieldStatus) Resource {
		return Resource{
			Type: ResourceTypePhysicalDisk, Name: "WDC", Status: StatusOnline,
			PhysicalDisk: &PhysicalDiskMeta{
				DevPath: "/dev/sdb", Serial: "WD-SILENT1", Temperature: 41,
				Collection: &diskinventory.CollectionStatus{
					Serial:      diskinventory.Available("smartctl"),
					Temperature: temperature,
				},
			},
		}
	}
	identity := ResourceIdentity{MachineID: "WD-SILENT1", Hostnames: []string{"node1"}}
	collected := diskinventory.Available("smartctl")
	expired := diskinventory.Unavailable("smartctl", "host agent stopped reporting")

	for _, tc := range []struct {
		name         string
		proxmoxFirst bool
		agent        diskinventory.FieldStatus
		want         diskinventory.FieldStatus
	}{
		{"proxmox then expired agent", true, expired, expired},
		{"expired agent then proxmox", false, expired, expired},
		{"proxmox then reporting agent", true, collected, collected},
		{"reporting agent then proxmox", false, collected, collected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewRegistry(nil)
			// The Proxmox row still holds the earlier "collected" copy.
			ingestProxmox := func() { registry.ingest(SourceProxmox, "pve1-node1-sdb", proxmoxDisk(collected), identity) }
			ingestAgent := func() { registry.ingest(SourceAgent, "agent-1-sdb", agentDisk(tc.agent), identity) }
			if tc.proxmoxFirst {
				ingestProxmox()
				ingestAgent()
			} else {
				ingestAgent()
				ingestProxmox()
			}
			disks := registry.ListByType(ResourceTypePhysicalDisk)
			if len(disks) != 1 || disks[0].PhysicalDisk == nil || disks[0].PhysicalDisk.Collection == nil {
				t.Fatalf("expected one merged disk, got %+v", disks)
			}
			collection := disks[0].PhysicalDisk.Collection
			if collection.Temperature != tc.want {
				t.Fatalf("temperature collection = %+v, want %+v", collection.Temperature, tc.want)
			}
			if disks[0].PhysicalDisk.Temperature != 41 {
				t.Fatalf("last-known temperature was not retained: %d", disks[0].PhysicalDisk.Temperature)
			}
			if collection.Pool != diskinventory.Available("proxmox_zfs") {
				t.Fatalf("Proxmox-owned evidence changed: %+v", collection.Pool)
			}
		})
	}
}
