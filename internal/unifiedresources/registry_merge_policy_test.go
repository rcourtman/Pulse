package unifiedresources

import (
	"encoding/json"
	"strconv"
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

// A Proxmox node the cluster reports offline on a live poll (pollPVENode
// stamps LastSeen now) stays offline when its linked agent falls silent, as a
// powered-off machine's agent does. The stale pass used to read the live PVE
// sighting as online, because it only says the poll delivered, and lifted the
// node back to online.
func TestProxmoxNodeReportedOfflineStaysOfflineWhenItsAgentFallsSilent(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name        string
		agentStatus string
		agentSeen   time.Time
	}{
		{name: "agent late inside its lease", agentStatus: "online", agentSeen: now.Add(-90 * time.Second)},
		{name: "agent past its lease", agentStatus: "offline", agentSeen: now.Add(-5 * time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const machineID = "machine-pve1"
			rr := NewRegistry(nil)
			rr.IngestSnapshot(models.StateSnapshot{
				Nodes: []models.Node{{
					ID:               "homelab-pve1",
					Name:             "pve1",
					Instance:         "homelab",
					ClusterName:      "homelab",
					Status:           "offline",
					ConnectionHealth: "error",
					LastSeen:         now,
				}},
				Hosts: []models.Host{{
					ID:              machineID,
					MachineID:       machineID,
					Hostname:        "pve1",
					LinkedNodeID:    "homelab-pve1",
					Status:          tc.agentStatus,
					LastSeen:        tc.agentSeen,
					IntervalSeconds: 30,
				}},
			})

			resource := onlyResourceOfType(t, rr, ResourceTypeAgent)
			if got := resource.SourceStatus[SourceProxmox].Status; got != "online" {
				t.Fatalf("proxmox sighting = %q, want the live poll's delivery", got)
			}
			if got := resource.SourceStatus[SourceAgent].Status; got != "stale" {
				t.Fatalf("agent sighting = %q, want stale", got)
			}
			if resource.Status != StatusOffline {
				t.Fatalf("status = %q, want offline as the live poll reported", resource.Status)
			}
			nodes := rr.Nodes()
			if len(nodes) != 1 || nodes[0].Status() != StatusOffline {
				t.Fatalf("node view status = %v, want offline", nodes)
			}

			rr.MarkStale(time.Now().UTC(), nil)
			if got, _ := rr.Get(resource.ID); got.Status != StatusOffline {
				t.Fatalf("second stale pass changed status to %q, want offline", got.Status)
			}
		})
	}
}

// preserveOrExpireNodes marks a node offline once its poll has failed past
// the grace period and keeps its old LastSeen. That verdict is the poller's
// own and survives the sighting going stale; it used to read as warning.
func TestPollerExpiredProxmoxNodeStaysOffline(t *testing.T) {
	expiredAt := time.Now().UTC().Add(-10 * time.Minute)
	node := models.Node{
		ID:               "homelab-pve9",
		Name:             "pve9",
		Instance:         "homelab",
		ClusterName:      "homelab",
		Status:           "offline",
		ConnectionHealth: "error",
		LastSeen:         expiredAt,
	}

	t.Run("node alone", func(t *testing.T) {
		rr := NewRegistry(nil)
		rr.IngestSnapshot(models.StateSnapshot{Nodes: []models.Node{node}})

		resource := onlyResourceOfType(t, rr, ResourceTypeAgent)
		if got := resource.SourceStatus[SourceProxmox].Status; got != "stale" {
			t.Fatalf("proxmox sighting = %q, want stale", got)
		}
		if resource.Status != StatusOffline {
			t.Fatalf("status = %q, want offline for a node the poller expired", resource.Status)
		}
		// The sighting still reports the lost poller to health.
		health := EvaluateResourceHealth(resource, nil, time.Now().UTC())
		if health.Verdict != HealthCritical || len(health.Reasons) < 2 ||
			health.Reasons[0].Code != "offline" || health.Reasons[1].Code != "telemetry_stale" {
			t.Fatalf("health = %+v, want critical offline with the stale reason after it", health)
		}
	})

	t.Run("node and its silent agent", func(t *testing.T) {
		const machineID = "machine-pve9"
		rr := NewRegistry(nil)
		rr.IngestSnapshot(models.StateSnapshot{
			Nodes: []models.Node{node},
			Hosts: []models.Host{{
				ID:              machineID,
				MachineID:       machineID,
				Hostname:        "pve9",
				LinkedNodeID:    node.ID,
				Status:          "offline",
				LastSeen:        expiredAt,
				IntervalSeconds: 30,
			}},
		})

		resource := onlyResourceOfType(t, rr, ResourceTypeAgent)
		if _, ok := resource.SourceStatus[SourceAgent]; !ok {
			t.Fatalf("expected the node and its agent to share one resource, got sources %+v", resource.SourceStatus)
		}
		if resource.Status != StatusOffline {
			t.Fatalf("status = %q, want offline when both sources last reported it offline", resource.Status)
		}
	})
}

// Guests take the same rule: a guest Proxmox last saw stopped stays offline
// when the poll goes quiet, while a running one is only known to be stale.
func TestQuietProxmoxPollKeepsEachGuestsLastVerdict(t *testing.T) {
	lastPoll := time.Now().UTC().Add(-10 * time.Minute)
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		VMs: []models.VM{
			{ID: "homelab-pve9-101", VMID: 101, Name: "stopped-vm", Node: "pve9", Instance: "homelab", Type: "qemu", Status: "stopped", LastSeen: lastPoll},
			{ID: "homelab-pve9-102", VMID: 102, Name: "running-vm", Node: "pve9", Instance: "homelab", Type: "qemu", Status: "running", LastSeen: lastPoll},
		},
	})

	want := map[string]ResourceStatus{"stopped-vm": StatusOffline, "running-vm": StatusWarning}
	vms := rr.ListByType(ResourceTypeVM)
	if len(vms) != len(want) {
		t.Fatalf("expected %d VMs, got %d", len(want), len(vms))
	}
	for _, vm := range vms {
		if got := vm.SourceStatus[SourceProxmox].Status; got != "stale" {
			t.Fatalf("%s sighting = %q, want stale", vm.Name, got)
		}
		if vm.Status != want[vm.Name] {
			t.Fatalf("%s status = %q, want %q", vm.Name, vm.Status, want[vm.Name])
		}
	}
}

// A copy that went through JSON loses the unexported verdicts and recovers
// them from its stored status, so a node reported offline with a silent
// agent does not come back online.
func TestProxmoxNodeReportedOfflineStaysOfflineAfterJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC()
	const machineID = "machine-pve1"
	source := NewRegistry(nil)
	source.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{{
			ID:               "homelab-pve1",
			Name:             "pve1",
			Instance:         "homelab",
			ClusterName:      "homelab",
			Status:           "offline",
			ConnectionHealth: "error",
			LastSeen:         now,
		}},
		Hosts: []models.Host{{
			ID:              machineID,
			MachineID:       machineID,
			Hostname:        "pve1",
			LinkedNodeID:    "homelab-pve1",
			Status:          "offline",
			LastSeen:        now.Add(-5 * time.Minute),
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

// An in-memory copy keeps its verdicts, and a facet sighting that has none
// on purpose (the PBS host-agent association) must not take the stored
// status as one: that would let the facet decide once the agent goes quiet.
func TestIngestResourcesKeepsAFacetSightingWithoutAVerdict(t *testing.T) {
	now := time.Now().UTC()
	rr := NewRegistry(nil)
	rr.IngestResources([]Resource{{
		ID:       "agent-pbs-host",
		Type:     ResourceTypeAgent,
		Name:     "pbs-host",
		Status:   StatusWarning,
		LastSeen: now,
		Sources:  []DataSource{SourceAgent, SourcePBS},
		SourceStatus: map[DataSource]SourceStatus{
			SourceAgent: {Status: "online", LastSeen: now, reported: StatusWarning},
			SourcePBS:   {Status: "online", LastSeen: now},
		},
		Agent: &AgentData{AgentID: "host-pbs", Hostname: "pbs-host"},
	}})

	resource, ok := rr.Get("agent-pbs-host")
	if !ok {
		t.Fatal("resource missing")
	}
	if got := resource.SourceStatus[SourceAgent].reported; got != StatusWarning {
		t.Fatalf("agent verdict = %q, want the copied warning", got)
	}
	if got := resource.SourceStatus[SourcePBS].reported; got != "" {
		t.Fatalf("PBS facet verdict = %q, want none", got)
	}
	if resource.Status != StatusWarning {
		t.Fatalf("status = %q, want the agent's warning", resource.Status)
	}
}

// aggregateStatus is the one rule the stale pass and manual links apply.
// Current sources decide by their own verdicts in chooseStatus's priority
// order; quiet sources only decide when no source is current.
func TestAggregateStatusReadsSourceVerdictsNotDelivery(t *testing.T) {
	current := func(reported ResourceStatus) SourceStatus {
		return SourceStatus{Status: "online", reported: reported}
	}
	quiet := func(reported ResourceStatus) SourceStatus {
		return SourceStatus{Status: "stale", reported: reported}
	}
	for _, tc := range []struct {
		name      string
		sightings map[DataSource]SourceStatus
		want      ResourceStatus
	}{
		{
			name:      "live poll reports offline, agent quiet",
			sightings: map[DataSource]SourceStatus{SourceProxmox: current(StatusOffline), SourceAgent: quiet(StatusOnline)},
			want:      StatusOffline,
		},
		{
			name:      "live poll reports online, agent past its lease",
			sightings: map[DataSource]SourceStatus{SourceProxmox: current(StatusOnline), SourceAgent: quiet(StatusOffline)},
			want:      StatusOnline,
		},
		{
			name:      "live agent outranks a poll that reports offline",
			sightings: map[DataSource]SourceStatus{SourceProxmox: current(StatusOffline), SourceAgent: current(StatusOnline)},
			want:      StatusOnline,
		},
		{
			name:      "live agent warning holds as at merge time",
			sightings: map[DataSource]SourceStatus{SourceProxmox: current(StatusOnline), SourceAgent: current(StatusWarning), SourceDocker: quiet(StatusOnline)},
			want:      StatusWarning,
		},
		{
			name:      "quiet source that reported offline keeps it",
			sightings: map[DataSource]SourceStatus{SourceProxmox: quiet(StatusOffline)},
			want:      StatusOffline,
		},
		{
			name:      "quiet source that reported online is a warning",
			sightings: map[DataSource]SourceStatus{SourceProxmox: quiet(StatusOnline)},
			want:      StatusWarning,
		},
		{
			name:      "best of quiet sources",
			sightings: map[DataSource]SourceStatus{SourceProxmox: quiet(StatusOnline), SourceAgent: quiet(StatusOffline)},
			want:      StatusWarning,
		},
		{
			name:      "current facet without a verdict counts as online",
			sightings: map[DataSource]SourceStatus{SourceProxmox: quiet(StatusOffline), SourcePBS: current("")},
			want:      StatusOnline,
		},
		{
			name:      "a facet without a verdict never outranks a current verdict",
			sightings: map[DataSource]SourceStatus{SourceProxmox: current(StatusOffline), SourceAvailability: current("")},
			want:      StatusOffline,
		},
		{
			name:      "current unknown verdict leaves the quiet sources to decide",
			sightings: map[DataSource]SourceStatus{SourceProxmox: current(StatusUnknown), SourceAgent: quiet(StatusOnline)},
			want:      StatusWarning,
		},
		{
			name:      "no delivered sightings",
			sightings: map[DataSource]SourceStatus{SourceProxmox: {Status: "unknown"}},
			want:      StatusUnknown,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := aggregateStatus(&Resource{SourceStatus: tc.sightings}); got != tc.want {
				t.Fatalf("aggregateStatus = %q, want %q", got, tc.want)
			}
		})
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

// A merged disk must never present its temperature under a state the
// reading's own row does not claim. A silent agent's retained reading stays as
// last-known context under the agent's own state, whatever the Proxmox row
// collected or copied; a resumed agent's fresh reading stays collected over a
// Proxmox copy of its earlier withdrawal; a standby withdrawal keeps the
// pre-sleep reading as last-known.
func TestPhysicalDiskMergePairsTemperatureWithItsCollectionState(t *testing.T) {
	disk := func(temperature int, status diskinventory.FieldStatus) Resource {
		return Resource{
			Type: ResourceTypePhysicalDisk, Name: "WDC", Status: StatusOnline,
			PhysicalDisk: &PhysicalDiskMeta{
				DevPath: "/dev/sdb", Serial: "WD-PAIR1", Temperature: temperature,
				Collection: &diskinventory.CollectionStatus{Temperature: status},
			},
		}
	}
	// The Proxmox row can also carry the SMART attributes copied from the
	// agent's earlier report, which changes whose value the merge shows.
	withSMART := func(r Resource) Resource {
		powerOnHours := int64(1000)
		r.PhysicalDisk.SMART = &SMARTMeta{PowerOnHours: &powerOnHours}
		return r
	}
	identity := ResourceIdentity{MachineID: "WD-PAIR1", Hostnames: []string{"node1"}}
	retained := diskinventory.Unavailable("host_agent", "host agent stopped reporting")
	nodeSMART := diskinventory.Available("proxmox_node_smart")
	smartctl := diskinventory.Available("smartctl")
	withdrawn := diskinventory.Unavailable("smartctl", "host agent stopped reporting")
	standby := diskinventory.Unavailable("smartctl", "disk is in standby")
	noProxmoxTemp := diskinventory.Unsupported("proxmox_disks", "Proxmox disk inventory does not expose temperature")
	// An agent from before collection provenance withdraws without a source;
	// monitoring copies its readings onto the Proxmox row as host_agent.
	legacyWithdrawn := diskinventory.Unavailable("", "host agent stopped reporting")
	legacyCopy := diskinventory.Available(diskinventory.LegacyHostAgentSource)

	for _, tc := range []struct {
		name           string
		agent, proxmox Resource
		// wantTemperature 0 accepts either row's value: source preference,
		// not this rule, decides which one is shown.
		wantTemperature int
		wantState       diskinventory.FieldStatus
	}{
		{"silent agent, Proxmox collected", disk(72, retained), disk(40, nodeSMART), 72, retained},
		{"silent agent, no Proxmox reading", disk(72, retained), disk(0, noProxmoxTemp), 72, retained},
		{"reporting agent, Proxmox collected", disk(41, smartctl), disk(40, nodeSMART), 41, smartctl},
		{"silent agent, Proxmox copy of its earlier reading", disk(72, withdrawn), disk(41, smartctl), 72, withdrawn},
		{"silent agent, Proxmox copy with SMART attributes", disk(72, withdrawn), withSMART(disk(41, smartctl)), 0, withdrawn},
		{"resumed agent, Proxmox copy of its withdrawal", disk(50, smartctl), disk(41, withdrawn), 50, smartctl},
		{"resumed agent, same value as a Proxmox copy of its withdrawal with SMART attributes", disk(41, smartctl), withSMART(disk(41, withdrawn)), 41, smartctl},
		{"legacy agent past its lease, Proxmox copy with SMART attributes", disk(72, legacyWithdrawn), withSMART(disk(41, legacyCopy)), 0, legacyWithdrawn},
		{"standby agent, Proxmox copy of its pre-sleep reading", disk(0, standby), disk(41, smartctl), 41, standby},
	} {
		for _, proxmoxFirst := range []bool{true, false} {
			registry := NewRegistry(nil)
			ingestProxmox := func() { registry.ingest(SourceProxmox, "pve1-node1-sdb", tc.proxmox, identity) }
			ingestAgent := func() { registry.ingest(SourceAgent, "agent-1-sdb", tc.agent, identity) }
			if proxmoxFirst {
				ingestProxmox()
				ingestAgent()
			} else {
				ingestAgent()
				ingestProxmox()
			}
			disks := registry.ListByType(ResourceTypePhysicalDisk)
			if len(disks) != 1 || disks[0].PhysicalDisk == nil {
				t.Fatalf("%s (proxmox first=%v): expected one merged disk, got %+v", tc.name, proxmoxFirst, disks)
			}
			got := disks[0].PhysicalDisk
			var state diskinventory.FieldStatus
			if got.Collection != nil {
				state = got.Collection.Temperature
			}
			valueOK := got.Temperature == tc.wantTemperature ||
				(tc.wantTemperature == 0 && (got.Temperature == tc.agent.PhysicalDisk.Temperature || got.Temperature == tc.proxmox.PhysicalDisk.Temperature))
			if !valueOK || state != tc.wantState {
				t.Errorf("%s (proxmox first=%v): temperature=%d state=%+v, want %d with %+v",
					tc.name, proxmoxFirst, got.Temperature, state, tc.wantTemperature, tc.wantState)
			}
		}
	}
}

// Three rows for one disk, none collected now: the agent's SMART row withdrew
// its reading, the Unraid inventory row is past the same lease, and the
// Proxmox row still carries a copy of the agent's earlier "available" state.
// Whatever the ingest order, the merged disk must not present a reading as
// collected, including when the Unraid inventory and the Proxmox copy happen
// to hold the same value, so that an earlier merge step's single state cannot
// tell whose reading is shown.
func TestPhysicalDiskMergeNeverPresentsAWithdrawnTemperatureAsCollected(t *testing.T) {
	disk := func(temperature int, status diskinventory.FieldStatus) Resource {
		return Resource{
			Type: ResourceTypePhysicalDisk, Name: "WDC", Status: StatusOnline,
			PhysicalDisk: &PhysicalDiskMeta{
				DevPath: "/dev/sdb", Serial: "WD-THREE1", Temperature: temperature,
				Collection: &diskinventory.CollectionStatus{Temperature: status},
			},
		}
	}
	identity := ResourceIdentity{MachineID: "WD-THREE1", Hostnames: []string{"tower"}}
	for _, unraidTemperature := range []int{37, 41} {
		rows := []physicalDiskMergeRow{
			{SourceAgent, "agent-1-sdb", disk(72, diskinventory.Unavailable("smartctl", "host agent stopped reporting"))},
			{SourceAgent, "agent-1-unraid-sdb", disk(unraidTemperature, diskinventory.Unavailable("unraid", "host agent stopped reporting"))},
			{SourceProxmox, "pve1-node1-sdb", disk(41, diskinventory.Available("smartctl"))},
		}
		for _, order := range physicalDiskMergeOrders(len(rows)) {
			got := mergePhysicalDiskRows(t, rows, order, identity)
			if diskinventory.TemperatureCollected(got.Temperature, got.Collection) {
				t.Errorf("unraid %d, order %v: withdrawn temperature presented as collected: temperature=%d collection=%+v",
					unraidTemperature, order, got.Temperature, got.Collection)
			}
		}
	}
}

// A legacy agent (before collection provenance) sends only readings it
// collected, with no state. Its reading must stay collected whatever other
// rows the disk has: a Proxmox inventory row that cannot read temperatures
// must not lend it "unsupported", and a second Proxmox row collecting a
// different value must not change which state belongs to which value.
func TestPhysicalDiskMergeKeepsALegacyAgentReadingCollected(t *testing.T) {
	disk := func(temperature int, status *diskinventory.FieldStatus) Resource {
		meta := &PhysicalDiskMeta{DevPath: "/dev/sdb", Serial: "WD-LEGACY1", Temperature: temperature}
		if status != nil {
			meta.Collection = &diskinventory.CollectionStatus{Temperature: *status}
		}
		return Resource{Type: ResourceTypePhysicalDisk, Name: "WDC", Status: StatusOnline, PhysicalDisk: meta}
	}
	unsupported := diskinventory.Unsupported("proxmox_disks", "Proxmox disk inventory does not expose temperature")
	nodeSMART := diskinventory.Available("proxmox_node_smart")
	identity := ResourceIdentity{MachineID: "WD-LEGACY1", Hostnames: []string{"node1"}}
	legacy := physicalDiskMergeRow{SourceAgent, "agent-1-sdb", disk(50, nil)}
	inventory := physicalDiskMergeRow{SourceProxmox, "pve1-node1-sdb", disk(0, &unsupported)}
	otherNode := physicalDiskMergeRow{SourceProxmox, "pve2-node2-sdb", disk(40, &nodeSMART)}

	for _, rows := range [][]physicalDiskMergeRow{{legacy, inventory}, {legacy, inventory, otherNode}} {
		for _, order := range physicalDiskMergeOrders(len(rows)) {
			got := mergePhysicalDiskRows(t, rows, order, identity)
			var state diskinventory.FieldStatus
			if got.Collection != nil {
				state = got.Collection.Temperature
			}
			want := map[int]diskinventory.FieldStatus{50: {}, 40: nodeSMART}
			if wantState, ok := want[got.Temperature]; !ok || state != wantState {
				t.Errorf("%d rows, order %v: temperature=%d state=%+v, want 50 without state or 40 with %+v",
					len(rows), order, got.Temperature, state, nodeSMART)
			}
		}
	}
}

// Whatever rows a disk has and whatever order they arrive in, the merged
// temperature must carry a state that belongs to a row holding that value,
// after the agent's own withdrawal of a source supersedes another row's copy
// of that source's earlier "available" state. The merge may still choose
// which row's value to show by source preference; it may not pair the value
// with another row's state. The catalog covers a reporting, silent, standby
// and legacy agent, a SMART row with and without attributes, the Unraid
// inventory (collected, past the lease, spun down), and Proxmox rows that
// read nothing, collect their own reading, or carry a fresh or stale copy of
// the agent's.
func TestPhysicalDiskMergePairsEveryShownTemperatureWithItsRowState(t *testing.T) {
	disk := func(temperature int, status *diskinventory.FieldStatus, smart bool) Resource {
		meta := &PhysicalDiskMeta{DevPath: "/dev/sdb", Serial: "WD-CATALOG1", Temperature: temperature}
		if status != nil {
			meta.Collection = &diskinventory.CollectionStatus{Temperature: *status}
		}
		if smart {
			powerOnHours := int64(1000)
			meta.SMART = &SMARTMeta{PowerOnHours: &powerOnHours}
		}
		return Resource{Type: ResourceTypePhysicalDisk, Name: "WDC", Status: StatusOnline, PhysicalDisk: meta}
	}
	state := func(status diskinventory.FieldStatus) *diskinventory.FieldStatus { return &status }
	present := func(resource Resource) *Resource { return &resource }
	stopped := "host agent stopped reporting"
	agentRows := []Resource{
		disk(41, state(diskinventory.Available("smartctl")), false),
		disk(50, state(diskinventory.Available("smartctl")), true),
		disk(72, state(diskinventory.Unavailable("smartctl", stopped)), false),
		disk(41, state(diskinventory.Unavailable("smartctl", stopped)), true),
		disk(0, state(diskinventory.Unavailable("smartctl", "disk is in standby")), false),
		disk(50, nil, false),
		disk(72, state(diskinventory.Unavailable("host_agent", stopped)), false),
		disk(72, state(diskinventory.Unavailable("", stopped)), false),
	}
	unraidRows := []*Resource{
		nil,
		present(disk(41, state(diskinventory.Unavailable("unraid", stopped)), false)),
		present(disk(37, state(diskinventory.Available("unraid")), false)),
		present(disk(41, state(diskinventory.Unavailable("unraid", "disk is reported spun down")), false)),
	}
	proxmoxRows := []Resource{
		disk(0, state(diskinventory.Unsupported("proxmox_disks", "Proxmox disk inventory does not expose temperature")), false),
		disk(40, state(diskinventory.Available("proxmox_node_smart")), false),
		disk(41, state(diskinventory.Available("smartctl")), false),
		disk(41, state(diskinventory.Available("smartctl")), true),
		disk(41, state(diskinventory.Unavailable("smartctl", stopped)), true),
		disk(72, state(diskinventory.Available("host_agent")), false),
		disk(41, state(diskinventory.Available("host_agent")), true),
	}
	// TrueNAS rows carry no collection state.
	trueNASRows := []*Resource{nil, present(disk(40, nil, true))}
	identity := ResourceIdentity{MachineID: "WD-CATALOG1", Hostnames: []string{"node1"}}

	for _, agent := range agentRows {
		for _, unraid := range unraidRows {
			for _, proxmox := range proxmoxRows {
				for _, trueNAS := range trueNASRows {
					rows := []physicalDiskMergeRow{
						{SourceAgent, "agent-1-sdb", agent},
						{SourceProxmox, "pve1-node1-sdb", proxmox},
					}
					if unraid != nil {
						rows = append(rows, physicalDiskMergeRow{SourceAgent, "agent-1-unraid-sdb", *unraid})
					}
					if trueNAS != nil {
						rows = append(rows, physicalDiskMergeRow{SourceTrueNAS, "truenas-1-sdb", *trueNAS})
					}
					for _, order := range physicalDiskMergeOrders(len(rows)) {
						assertPhysicalDiskTemperaturePairedWithItsRows(t, rows, order, mergePhysicalDiskRows(t, rows, order, identity))
					}
				}
			}
		}
	}
}

// assertPhysicalDiskTemperaturePairedWithItsRows checks a merged disk against
// its rows: a shown temperature carries the state of a row reporting that
// value, preferring a collected reading, then the agent's own report over a
// copy.
func assertPhysicalDiskTemperaturePairedWithItsRows(t *testing.T, rows []physicalDiskMergeRow, order []int, got *PhysicalDiskMeta) {
	t.Helper()
	if got.Temperature <= 0 {
		return
	}
	var shown diskinventory.FieldStatus
	if got.Collection != nil {
		shown = got.Collection.Temperature
	}
	type candidate struct {
		state diskinventory.FieldStatus
		rank  int
	}
	var candidates []candidate
	bestRank := -1
	for _, row := range rows {
		if row.resource.PhysicalDisk.Temperature != got.Temperature {
			continue
		}
		state := physicalDiskRowTemperatureStateAfterWithdrawals(row, rows)
		rank := 0
		if !diskinventory.TemperatureCollected(got.Temperature, &diskinventory.CollectionStatus{Temperature: state}) {
			rank += 2
		}
		if row.source != SourceAgent {
			rank++
		}
		candidates = append(candidates, candidate{state, rank})
		if bestRank < 0 || rank < bestRank {
			bestRank = rank
		}
	}
	for _, candidate := range candidates {
		if candidate.rank == bestRank && candidate.state == shown {
			return
		}
	}
	t.Errorf("rows %s, order %v: temperature %d shown under %+v, but its rows say %+v (best rank %d)",
		describePhysicalDiskMergeRows(rows), order, got.Temperature, shown, candidates, bestRank)
}

type physicalDiskMergeRow struct {
	source   DataSource
	sourceID string
	resource Resource
}

// physicalDiskRowTemperatureStateAfterWithdrawals is the test oracle: a row's
// own temperature state, unless the row is not the agent's own report and the
// agent withdrew the source of the state it copied, which then gives way to
// that withdrawal. An agent withdrawal without a source is a legacy agent's,
// whose copies carry the legacy agent source.
func physicalDiskRowTemperatureStateAfterWithdrawals(row physicalDiskMergeRow, rows []physicalDiskMergeRow) diskinventory.FieldStatus {
	own := func(r physicalDiskMergeRow) diskinventory.FieldStatus {
		if r.resource.PhysicalDisk.Collection == nil {
			return diskinventory.FieldStatus{}
		}
		return r.resource.PhysicalDisk.Collection.Temperature
	}
	state := own(row)
	if row.source == SourceAgent || state.State == "" {
		return state
	}
	for _, other := range rows {
		withdrawal := own(other)
		source := withdrawal.Source
		if source == "" {
			source = diskinventory.LegacyHostAgentSource
		}
		if other.source == SourceAgent && withdrawal.State != "" && withdrawal.State != diskinventory.FieldAvailable &&
			strings.EqualFold(source, state.Source) {
			return withdrawal
		}
	}
	return state
}

func mergePhysicalDiskRows(t *testing.T, rows []physicalDiskMergeRow, order []int, identity ResourceIdentity) *PhysicalDiskMeta {
	t.Helper()
	registry := NewRegistry(nil)
	for _, i := range order {
		row := rows[i]
		resource := row.resource
		resource.PhysicalDisk = clonePhysicalDiskMeta(row.resource.PhysicalDisk)
		registry.ingest(row.source, row.sourceID, resource, identity)
	}
	disks := registry.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 || disks[0].PhysicalDisk == nil {
		t.Fatalf("rows %s, order %v: expected one merged disk, got %d", describePhysicalDiskMergeRows(rows), order, len(disks))
	}
	return disks[0].PhysicalDisk
}

// physicalDiskMergeOrders returns every ingest order of n rows.
func physicalDiskMergeOrders(n int) [][]int {
	if n == 0 {
		return [][]int{{}}
	}
	var orders [][]int
	for _, rest := range physicalDiskMergeOrders(n - 1) {
		for position := 0; position <= len(rest); position++ {
			order := append(append(append([]int{}, rest[:position]...), n-1), rest[position:]...)
			orders = append(orders, order)
		}
	}
	return orders
}

func describePhysicalDiskMergeRows(rows []physicalDiskMergeRow) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		meta := row.resource.PhysicalDisk
		state := "none"
		if meta.Collection != nil {
			state = string(meta.Collection.Temperature.State) + "(" + meta.Collection.Temperature.Source + ")"
		}
		smart := ""
		if meta.SMART != nil {
			smart = "+smart"
		}
		parts = append(parts, string(row.source)+":"+strconv.Itoa(meta.Temperature)+"/"+state+smart)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// The readings behind a merged disk's temperature describe the rows that
// registry merged. A disk cloned out and seeded back continues from them; a
// disk known only by its presented value and state (read back from JSON, or
// edited) stands as one reading of unknown rows, which an agent's later word
// supersedes and which still withdraws for the agent; a disk that enters as a
// new row is that row alone; and a row that reports again replaces its own
// earlier reading. A second agent reporting the same disk is not a copy of the
// first, so its withdrawal leaves the other agent's own reading collected.
func TestPhysicalDiskMergeTemperatureReadingsAcrossIngestBoundaries(t *testing.T) {
	disk := func(temperature int, status diskinventory.FieldStatus) Resource {
		return Resource{
			Type: ResourceTypePhysicalDisk, Name: "WDC", Status: StatusOnline,
			PhysicalDisk: &PhysicalDiskMeta{
				DevPath: "/dev/sdb", Serial: "WD-BOUNDARY1", Temperature: temperature,
				Collection: &diskinventory.CollectionStatus{Temperature: status},
			},
		}
	}
	identity := ResourceIdentity{MachineID: "WD-BOUNDARY1", Hostnames: []string{"node1"}}
	smartctl := diskinventory.Available("smartctl")
	standby := diskinventory.Unavailable("smartctl", "disk is in standby")
	stopped := diskinventory.Unavailable("smartctl", "host agent stopped reporting")
	spunDown := diskinventory.Unavailable("unraid", "disk is reported spun down")
	only := func(t *testing.T, registry *ResourceRegistry) *PhysicalDiskMeta {
		t.Helper()
		disks := registry.ListByType(ResourceTypePhysicalDisk)
		if len(disks) != 1 || disks[0].PhysicalDisk == nil {
			t.Fatalf("expected one merged disk, got %d", len(disks))
		}
		return disks[0].PhysicalDisk
	}
	stateOf := func(meta *PhysicalDiskMeta) diskinventory.FieldStatus {
		if meta.Collection == nil {
			return diskinventory.FieldStatus{}
		}
		return meta.Collection.Temperature
	}
	// presented is a merged disk known only by what it presents.
	presented := func(temperature int, status diskinventory.FieldStatus) Resource {
		resource := disk(temperature, status)
		resource.ID = "physical-disk:WD-BOUNDARY1"
		resource.Identity = identity
		resource.Sources = []DataSource{SourceAgent, SourceProxmox}
		return resource
	}
	expect := func(t *testing.T, registry *ResourceRegistry, temperature int, state diskinventory.FieldStatus) {
		t.Helper()
		if got := only(t, registry); got.Temperature != temperature || stateOf(got) != state {
			t.Fatalf("temperature=%d state=%+v, want %d under %+v", got.Temperature, stateOf(got), temperature, state)
		}
	}

	t.Run("a cloned disk seeded back continues from its rows", func(t *testing.T) {
		first := NewRegistry(nil)
		first.ingest(SourceAgent, "agent-1-sdb", disk(0, standby), identity)
		first.ingest(SourceAgent, "agent-1-unraid-sdb", disk(0, spunDown), identity)
		second := NewRegistry(nil)
		second.IngestResources(first.ListByType(ResourceTypePhysicalDisk))
		second.ingest(SourceProxmox, "pve1-node1-sdb", disk(41, smartctl), identity)
		expect(t, second, 41, standby)
	})

	t.Run("an edited clone starts again from what it presents", func(t *testing.T) {
		first := NewRegistry(nil)
		first.ingest(SourceAgent, "agent-1-sdb", disk(50, smartctl), identity)
		first.ingest(SourceProxmox, "pve1-node1-sdb", disk(40, diskinventory.Available("proxmox_node_smart")), identity)
		resources := first.ListByType(ResourceTypePhysicalDisk)
		if len(resources) != 1 || resources[0].PhysicalDisk.Temperature != 50 {
			t.Fatalf("expected one merged disk showing the agent's 50, got %+v", resources)
		}
		resources[0].PhysicalDisk.Collection.Temperature = stopped
		second := NewRegistry(nil)
		second.IngestResources(resources)
		second.ingest(SourceProxmox, "pve1-node1-sdb", disk(50, smartctl), identity)
		if got := only(t, second); diskinventory.TemperatureCollected(got.Temperature, got.Collection) {
			t.Fatalf("temperature=%d state=%+v, want the edited withdrawal to stand over the Proxmox copy", got.Temperature, stateOf(got))
		}
	})

	t.Run("a presented disk's withdrawal still supersedes a copy", func(t *testing.T) {
		registry := NewRegistry(nil)
		registry.IngestResources([]Resource{presented(50, stopped)})
		registry.ingest(SourceProxmox, "pve1-node1-sdb", disk(50, smartctl), identity)
		if got := only(t, registry); diskinventory.TemperatureCollected(got.Temperature, got.Collection) {
			t.Fatalf("temperature=%d state=%+v, want the presented withdrawal to stand over the Proxmox copy", got.Temperature, stateOf(got))
		}
	})

	t.Run("an agent's later word supersedes a presented disk's availability", func(t *testing.T) {
		registry := NewRegistry(nil)
		registry.IngestResources([]Resource{presented(50, smartctl)})
		registry.ingest(SourceAgent, "agent-1-sdb", disk(50, stopped), identity)
		expect(t, registry, 50, stopped)
	})

	t.Run("an agent's later word is presented over a presented disk's state", func(t *testing.T) {
		registry := NewRegistry(nil)
		registry.IngestResources([]Resource{presented(41, stopped)})
		registry.ingest(SourceAgent, "agent-1-sdb", disk(41, standby), identity)
		expect(t, registry, 41, standby)
	})

	t.Run("a merged disk ingested again as a Proxmox row is a copy", func(t *testing.T) {
		first := NewRegistry(nil)
		first.ingest(SourceAgent, "agent-1-sdb", disk(50, smartctl), identity)
		first.ingest(SourceProxmox, "pve1-node1-sdb", disk(40, diskinventory.Available("proxmox_node_smart")), identity)
		resources := first.ListByType(ResourceTypePhysicalDisk)
		if len(resources) != 1 {
			t.Fatalf("expected one merged disk, got %d", len(resources))
		}
		second := NewRegistry(nil)
		second.IngestRecords(SourceProxmox, []IngestRecord{{SourceID: "pve1-node1-sdb", Resource: resources[0], Identity: identity}})
		second.ingest(SourceAgent, "agent-1-sdb", disk(0, standby), identity)
		expect(t, second, 50, standby)
	})

	t.Run("an agent row reporting again replaces its own reading", func(t *testing.T) {
		registry := NewRegistry(nil)
		registry.ingest(SourceAgent, "agent-1-sdb", disk(41, smartctl), identity)
		registry.ingest(SourceAgent, "agent-1-sdb", disk(0, standby), identity)
		registry.ingest(SourceProxmox, "pve1-node1-sdb", disk(41, smartctl), identity)
		expect(t, registry, 41, standby)
	})

	t.Run("a value kept after its row reports again keeps the row's later word", func(t *testing.T) {
		rows := map[string]physicalDiskMergeRow{
			"silent":  {SourceAgent, "agent-1-sdb", disk(72, stopped)},
			"standby": {SourceAgent, "agent-1-sdb", disk(0, standby)},
			"proxmox": {SourceProxmox, "pve1-node1-sdb", disk(40, diskinventory.Available("proxmox_node_smart"))},
		}
		for _, sequence := range [][]string{{"silent", "standby", "proxmox"}, {"silent", "proxmox", "standby"}} {
			registry := NewRegistry(nil)
			for _, name := range sequence {
				registry.ingest(rows[name].source, rows[name].sourceID, rows[name].resource, identity)
			}
			if got := only(t, registry); got.Temperature != 72 || stateOf(got) != standby {
				t.Errorf("%v: temperature=%d state=%+v, want the retained 72 under the agent's standby", sequence, got.Temperature, stateOf(got))
			}
		}
	})

	t.Run("a value kept from another source takes its row's later withdrawal", func(t *testing.T) {
		// The SMART row first borrowed the Unraid inventory's reading, then
		// reported its own standby without a temperature.
		proxmox := func(registry *ResourceRegistry) {
			registry.ingest(SourceProxmox, "pve1-node1-sdb", disk(40, diskinventory.Available("proxmox_node_smart")), identity)
		}
		for _, proxmoxFirst := range []bool{false, true} {
			registry := NewRegistry(nil)
			if proxmoxFirst {
				proxmox(registry)
			}
			registry.ingest(SourceAgent, "agent-1-sdb", disk(72, diskinventory.Available("unraid")), identity)
			registry.ingest(SourceAgent, "agent-1-sdb", disk(0, standby), identity)
			if !proxmoxFirst {
				proxmox(registry)
			}
			expect(t, registry, 72, standby)
		}
	})

	t.Run("a kept value stays paired however often its row reports again", func(t *testing.T) {
		withSMART := func(r Resource) Resource {
			powerOnHours := int64(1000)
			r.PhysicalDisk.SMART = &SMARTMeta{PowerOnHours: &powerOnHours}
			return r
		}
		registry := NewRegistry(nil)
		registry.ingest(SourceAgent, "agent-1-sdb", withSMART(disk(72, stopped)), identity)
		registry.ingest(SourceAgent, "agent-1-sdb", disk(41, smartctl), identity)
		registry.ingest(SourceAgent, "agent-1-sdb", disk(42, smartctl), identity)
		expect(t, registry, 72, stopped)
	})

	t.Run("a kept legacy reading takes its row's later withdrawal", func(t *testing.T) {
		legacy := func(temperature int, smart bool) Resource {
			r := Resource{
				Type: ResourceTypePhysicalDisk, Name: "WDC", Status: StatusOnline,
				PhysicalDisk: &PhysicalDiskMeta{DevPath: "/dev/sdb", Serial: "WD-BOUNDARY1", Temperature: temperature},
			}
			if smart {
				powerOnHours := int64(1000)
				r.PhysicalDisk.SMART = &SMARTMeta{PowerOnHours: &powerOnHours}
			}
			return r
		}
		legacyStopped := diskinventory.Unavailable("", "host agent stopped reporting")
		registry := NewRegistry(nil)
		registry.ingest(SourceAgent, "agent-1-sdb", legacy(72, true), identity)
		registry.ingest(SourceAgent, "agent-1-sdb", legacy(41, false), identity)
		registry.ingest(SourceAgent, "agent-1-sdb", disk(0, legacyStopped), identity)
		expect(t, registry, 72, legacyStopped)
	})

	t.Run("a kept copy gives way to the agent's later withdrawal", func(t *testing.T) {
		withSMART := func(r Resource) Resource {
			powerOnHours := int64(1000)
			r.PhysicalDisk.SMART = &SMARTMeta{PowerOnHours: &powerOnHours}
			return r
		}
		registry := NewRegistry(nil)
		registry.ingest(SourceProxmox, "pve1-node1-sdb", withSMART(disk(72, smartctl)), identity)
		registry.ingest(SourceProxmox, "pve1-node1-sdb", disk(40, smartctl), identity)
		registry.ingest(SourceAgent, "agent-1-sdb", disk(0, standby), identity)
		expect(t, registry, 72, standby)
	})

	t.Run("a row's earlier value is not current evidence", func(t *testing.T) {
		withSMART := func(r Resource) Resource {
			powerOnHours := int64(1000)
			r.PhysicalDisk.SMART = &SMARTMeta{PowerOnHours: &powerOnHours}
			return r
		}
		nodeSMART := diskinventory.Available("proxmox_node_smart")
		registry := NewRegistry(nil)
		registry.ingest(SourceProxmox, "pve1-node1-sdb", disk(72, nodeSMART), identity)
		registry.ingest(SourceProxmox, "pve1-node1-sdb", disk(40, nodeSMART), identity)
		registry.ingest(SourceAgent, "agent-1-sdb", withSMART(disk(72, stopped)), identity)
		expect(t, registry, 72, stopped)
	})

	t.Run("a reported row is presented over a presented disk's stand-in", func(t *testing.T) {
		registry := NewRegistry(nil)
		registry.IngestResources([]Resource{presented(41, diskinventory.Unavailable("unraid", "host agent stopped reporting"))})
		registry.ingest(SourceAgent, "agent-1-sdb", disk(0, standby), identity)
		registry.ingest(SourceProxmox, "pve1-node1-sdb", disk(41, smartctl), identity)
		expect(t, registry, 41, standby)
	})

	t.Run("an agent's later withdrawal supersedes a presented disk's older one", func(t *testing.T) {
		for _, withCopy := range []bool{false, true} {
			registry := NewRegistry(nil)
			registry.IngestResources([]Resource{presented(41, stopped)})
			registry.ingest(SourceAgent, "agent-1-sdb", disk(0, standby), identity)
			if withCopy {
				registry.ingest(SourceProxmox, "pve1-node1-sdb", disk(41, smartctl), identity)
			}
			if got := only(t, registry); got.Temperature != 41 || stateOf(got) != standby {
				t.Errorf("with Proxmox copy=%v: temperature=%d state=%+v, want 41 under the agent's standby", withCopy, got.Temperature, stateOf(got))
			}
		}
	})

	t.Run("two agents' withdrawals of one source settle the same in any order", func(t *testing.T) {
		rows := []physicalDiskMergeRow{
			{SourceAgent, "agent-1-sdb", disk(0, standby)},
			{SourceAgent, "agent-2-sdb", disk(0, stopped)},
			{SourceProxmox, "pve1-node1-sdb", disk(41, smartctl)},
		}
		var first diskinventory.FieldStatus
		for i, order := range physicalDiskMergeOrders(len(rows)) {
			got := mergePhysicalDiskRows(t, rows, order, identity)
			if got.Temperature != 41 || diskinventory.TemperatureCollected(got.Temperature, got.Collection) {
				t.Fatalf("order %v: temperature=%d state=%+v, want the copied 41 withdrawn", order, got.Temperature, stateOf(got))
			}
			if i == 0 {
				first = stateOf(got)
			} else if stateOf(got) != first {
				t.Errorf("order %v: state=%+v, but another order gave %+v", order, stateOf(got), first)
			}
		}
	})

	t.Run("a second agent's withdrawal is not the first agent's", func(t *testing.T) {
		rows := []physicalDiskMergeRow{
			{SourceAgent, "agent-1-sdb", disk(41, smartctl)},
			{SourceAgent, "agent-2-sdb", disk(41, stopped)},
		}
		for _, order := range physicalDiskMergeOrders(len(rows)) {
			if got := mergePhysicalDiskRows(t, rows, order, identity); got.Temperature != 41 || stateOf(got) != smartctl {
				t.Errorf("order %v: temperature=%d state=%+v, want the first agent's collected 41", order, got.Temperature, stateOf(got))
			}
		}
	})
}
