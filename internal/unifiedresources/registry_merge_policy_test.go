package unifiedresources

import (
	"encoding/json"
	"slices"
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

// Either side of an operator link can arrive as a supplemental record: a
// vSphere VM with a Pulse agent inside, or an agent on a TrueNAS host. The
// monitor's rebuild ingests records after the snapshot, so it must join them
// there too, keep the record side's provider payload, and list what the
// resources API lists after seeding from it.
func TestManualLinksJoinRecordIngestedResourcesInMonitorRebuild(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "host-app-guest", Hostname: "app-guest", MachineID: "machine-app-guest", Status: "online", LastSeen: now},
			{ID: "host-nas", Hostname: "nas-a-mgmt", MachineID: "machine-nas-a", Status: "online", LastSeen: now},
		},
		LastUpdate: now,
	}
	vmRecord := IngestRecord{
		SourceID: "vc-1:vm:vm-42",
		Resource: Resource{
			Type:       ResourceTypeVM,
			Technology: "vmware",
			Name:       "app-guest",
			Status:     StatusOnline,
			LastSeen:   now,
			VMware:     &VMwareData{ConnectionID: "vc-1", ManagedObjectID: "vm-42", EntityType: "vm"},
		},
		Identity: ResourceIdentity{Hostnames: []string{"app-guest"}},
	}
	records := map[DataSource][]IngestRecord{
		SourceVMware: {vmRecord},
		SourceTrueNAS: {
			{
				SourceID: "system:tn-1",
				Resource: Resource{
					Type:     ResourceTypeAgent,
					Name:     "nas-a",
					Status:   StatusOnline,
					LastSeen: now,
					TrueNAS:  &TrueNASData{Hostname: "nas-a"},
				},
				Identity: ResourceIdentity{Hostnames: []string{"nas-a"}},
			},
			{
				SourceID:       "system:tn-1:pool:tank",
				ParentSourceID: "system:tn-1",
				Resource: Resource{
					Type:     ResourceTypeStorage,
					Name:     "tank",
					Status:   StatusOnline,
					LastSeen: now,
					Storage:  &StorageMeta{Type: "zfs-pool", Platform: "truenas", Topology: "pool"},
				},
			},
		},
	}

	for _, agentPrimary := range []bool{false, true} {
		name := "record-primary"
		if agentPrimary {
			name = "agent-primary"
		}
		t.Run(name, func(t *testing.T) {
			store := NewMemoryStore()
			adapter := NewMonitorAdapter(NewRegistry(store))
			adapter.PopulateSnapshotAndSupplemental(snapshot, records)

			var vmID, guestAgentID, systemID, nasAgentID string
			for _, resource := range adapter.GetAll() {
				switch {
				case resource.VMware != nil:
					vmID = resource.ID
				case resource.TrueNAS != nil:
					systemID = resource.ID
				case resource.Name == "app-guest":
					guestAgentID = resource.ID
				case resource.Name == "nas-a-mgmt":
					nasAgentID = resource.ID
				}
			}
			if vmID == "" || guestAgentID == "" || systemID == "" || nasAgentID == "" {
				t.Fatalf("unlinked estate = %v, want the VM, both agents and the TrueNAS system", resourceIDs(adapter.GetAll()))
			}
			guestPrimary, nasPrimary := vmID, systemID
			if agentPrimary {
				guestPrimary, nasPrimary = guestAgentID, nasAgentID
			}
			for _, link := range []ResourceLink{
				{ResourceA: vmID, ResourceB: guestAgentID, PrimaryID: guestPrimary},
				{ResourceA: systemID, ResourceB: nasAgentID, PrimaryID: nasPrimary},
			} {
				if err := store.AddLink(link); err != nil {
					t.Fatalf("add link: %v", err)
				}
			}

			assertJoined := func(stage string) {
				t.Helper()
				resources := adapter.GetAll()
				if len(resources) != 3 {
					t.Fatalf("%s: resources = %v, want the linked VM, the linked TrueNAS system and its pool", stage, resourceIDs(resources))
				}
				// An agent inside a guest supplements the guest, whichever
				// side the operator chose as primary.
				guest, ok := adapter.currentRegistry().Get(vmID)
				if !ok {
					t.Fatalf("%s: linked vSphere VM %s missing from %v", stage, vmID, resourceIDs(resources))
				}
				if guest.Type != ResourceTypeVM || guest.VMware == nil || guest.Agent == nil {
					t.Fatalf("%s: linked VM type=%s vmware=%t agent=%t, want the VM carrying its agent", stage, guest.Type, guest.VMware != nil, guest.Agent != nil)
				}
				if !slices.Contains(guest.Sources, SourceVMware) || !slices.Contains(guest.Sources, SourceAgent) {
					t.Fatalf("%s: linked VM sources = %v, want vmware and agent", stage, guest.Sources)
				}
				system, ok := adapter.currentRegistry().Get(nasPrimary)
				if !ok {
					t.Fatalf("%s: linked TrueNAS system %s missing from %v", stage, nasPrimary, resourceIDs(resources))
				}
				if system.TrueNAS == nil || system.Agent == nil {
					t.Fatalf("%s: linked system truenas=%t agent=%t, want the primary carrying both payloads", stage, system.TrueNAS != nil, system.Agent != nil)
				}
				if !slices.Contains(system.Sources, SourceTrueNAS) || !slices.Contains(system.Sources, SourceAgent) {
					t.Fatalf("%s: linked system sources = %v, want truenas and agent", stage, system.Sources)
				}
				pool := onlyResourceOfType(t, adapter.currentRegistry(), ResourceTypeStorage)
				if pool.ParentID == nil || *pool.ParentID != nasPrimary {
					t.Fatalf("%s: pool parent = %v, want the linked system %s", stage, pool.ParentID, nasPrimary)
				}

				// Identity pins persisted by the rebuild must not re-key a link
				// onto its own primary.
				links, err := store.GetLinks()
				if err != nil {
					t.Fatalf("%s: get links: %v", stage, err)
				}
				for _, link := range links {
					if link.ResourceA == link.ResourceB {
						t.Fatalf("%s: link collapsed onto %s", stage, link.ResourceA)
					}
				}

				// The resources API seeds its registry from this listing
				// through resource ingest and must agree with it.
				rest := NewRegistry(store)
				rest.IngestResources(resources)
				if got, want := resourceIDs(rest.List()), resourceIDs(resources); !slices.Equal(got, want) {
					t.Fatalf("%s: a registry seeded from the monitor lists %v, monitor lists %v", stage, got, want)
				}
			}

			next := snapshot
			next.LastUpdate = now.Add(time.Second)
			adapter.PopulateSnapshotAndSupplemental(next, records)
			assertJoined("rebuild")

			adapter.PopulateSupplementalRecords(SourceVMware, records[SourceVMware])
			adapter.PopulateSupplementalRecords(SourceTrueNAS, records[SourceTrueNAS])
			assertJoined("supplemental refresh")

			next.LastUpdate = now.Add(2 * time.Second)
			adapter.PopulateSnapshotAndSupplemental(next, records)
			assertJoined("second rebuild")
		})
	}
}

// A Proxmox node that absorbs a linked agent carries the agent's machine key,
// so the rebuild pins the node with it. The agent is still observed: that pin
// must not succeed the agent's ID onto the node, which re-keyed the link onto
// the node itself and split the pair on the next rebuild.
func TestManualLinkKeepsItsEndpointsThroughIdentityPinPersistence(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{{
			ID: "lab-pve1", Name: "pve1", Instance: "lab",
			Host: "https://192.0.2.10:8006", Status: "online", LastSeen: now,
		}},
		Hosts: []models.Host{{
			ID: "host-box", Hostname: "box-x", MachineID: "machine-box", Status: "online", LastSeen: now,
		}},
		LastUpdate: now,
	}
	store := NewMemoryStore()
	adapter := NewMonitorAdapter(NewRegistry(store))
	adapter.PopulateSnapshotAndSupplemental(snapshot, nil)
	var nodeID, agentID string
	for _, resource := range adapter.GetAll() {
		if resource.Proxmox != nil {
			nodeID = resource.ID
		} else if resource.Agent != nil {
			agentID = resource.ID
		}
	}
	if nodeID == "" || agentID == "" || nodeID == agentID {
		t.Fatalf("unlinked estate = %v, want a node and a separate agent", resourceIDs(adapter.GetAll()))
	}
	if err := store.AddLink(ResourceLink{ResourceA: nodeID, ResourceB: agentID, PrimaryID: nodeID}); err != nil {
		t.Fatalf("add link: %v", err)
	}

	for rebuild := 1; rebuild <= 3; rebuild++ {
		next := snapshot
		next.LastUpdate = now.Add(time.Duration(rebuild) * time.Second)
		adapter.PopulateSnapshotAndSupplemental(next, nil)
		if got := resourceIDs(adapter.GetAll()); !slices.Equal(got, []string{nodeID}) {
			t.Fatalf("rebuild %d: resources = %v, want only the linked node %s", rebuild, got, nodeID)
		}
		links, err := store.GetLinks()
		if err != nil {
			t.Fatalf("get links: %v", err)
		}
		if len(links) != 1 || links[0].ResourceA != nodeID || links[0].ResourceB != agentID {
			t.Fatalf("rebuild %d: links = %+v, want %s -> %s kept", rebuild, links, nodeID, agentID)
		}
	}
}

// A record may name an older canonical ID as superseded on every ingest. When
// that ID belongs to a resource a manual link folded into the record's own
// resource, it is still observed: succeeding it on a later refresh would
// re-key the link onto its primary.
func TestRecordDeclaredSuccessionSparesALinkFoldedResource(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Hosts:      []models.Host{{ID: "host-nas", Hostname: "nas-a-mgmt", MachineID: "machine-nas-a", Status: "online", LastSeen: now}},
		LastUpdate: now,
	}
	agentID := MachineIdentityCanonicalID(ResourceTypeAgent, "machine-nas-a")
	records := []IngestRecord{{
		SourceID:               "system:tn-1",
		SupersededCanonicalIDs: []string{agentID},
		Resource: Resource{
			Type:     ResourceTypeAgent,
			Name:     "nas-a",
			Status:   StatusOnline,
			LastSeen: now,
			TrueNAS:  &TrueNASData{Hostname: "nas-a"},
		},
		Identity: ResourceIdentity{Hostnames: []string{"nas-a"}},
	}}

	unlinked := NewMonitorAdapter(NewRegistry(nil))
	unlinked.PopulateSnapshotAndSupplemental(snapshot, map[DataSource][]IngestRecord{SourceTrueNAS: records})
	var systemID string
	for _, resource := range unlinked.GetAll() {
		if resource.TrueNAS != nil {
			systemID = resource.ID
		}
	}
	if systemID == "" || !slices.Contains(resourceIDs(unlinked.GetAll()), agentID) {
		t.Fatalf("unlinked estate = %v, want agent %s and a TrueNAS system", resourceIDs(unlinked.GetAll()), agentID)
	}

	store := NewMemoryStore()
	if err := store.AddLink(ResourceLink{ResourceA: systemID, ResourceB: agentID, PrimaryID: systemID}); err != nil {
		t.Fatalf("add link: %v", err)
	}
	adapter := NewMonitorAdapter(NewRegistry(store))
	adapter.PopulateSnapshotAndSupplemental(snapshot, map[DataSource][]IngestRecord{SourceTrueNAS: records})
	adapter.PopulateSupplementalRecords(SourceTrueNAS, records)

	if got := resourceIDs(adapter.GetAll()); !slices.Equal(got, []string{systemID}) {
		t.Fatalf("resources = %v, want only the linked system %s", got, systemID)
	}
	links, err := store.GetLinks()
	if err != nil {
		t.Fatalf("get links: %v", err)
	}
	if len(links) != 1 || links[0].ResourceA != systemID || links[0].ResourceB != agentID {
		t.Fatalf("links = %+v, want %s -> %s kept", links, systemID, agentID)
	}
}

// A link merge folds the other resource's provider payloads into the primary
// where the primary has none, whichever side the operator chose.
func TestManualLinkKeepsProviderPayloadsThePrimaryLacks(t *testing.T) {
	now := time.Now().UTC()
	store := NewMemoryStore()
	for _, link := range []ResourceLink{
		{ResourceA: "agent-nas", ResourceB: "truenas-system", PrimaryID: "agent-nas"},
		{ResourceA: "agent-esxi", ResourceB: "vmware-host", PrimaryID: "agent-esxi"},
	} {
		if err := store.AddLink(link); err != nil {
			t.Fatalf("add link: %v", err)
		}
	}
	host := func(id, name string, sources ...DataSource) Resource {
		status := make(map[DataSource]SourceStatus, len(sources))
		for _, source := range sources {
			status[source] = SourceStatus{Status: "online", LastSeen: now}
		}
		return Resource{ID: id, Type: ResourceTypeAgent, Name: name, Status: StatusOnline, LastSeen: now, Sources: sources, SourceStatus: status}
	}
	agentNAS := host("agent-nas", "nas-a-mgmt", SourceAgent)
	agentNAS.Agent = &AgentData{AgentID: "agent-nas", Hostname: "nas-a-mgmt"}
	system := host("truenas-system", "nas-a", SourceTrueNAS)
	system.TrueNAS = &TrueNASData{Hostname: "nas-a"}
	agentESXi := host("agent-esxi", "esxi-01", SourceAgent)
	agentESXi.Agent = &AgentData{AgentID: "agent-esxi", Hostname: "esxi-01"}
	esxi := host("vmware-host", "esxi-01.lab", SourceVMware)
	esxi.VMware = &VMwareData{ConnectionID: "vc-1", ManagedObjectID: "host-101", EntityType: "host"}

	rr := NewRegistry(store)
	rr.IngestResources([]Resource{agentNAS, system, agentESXi, esxi})

	if got := resourceIDs(rr.List()); !slices.Equal(got, []string{"agent-esxi", "agent-nas"}) {
		t.Fatalf("resources = %v, want the two linked primaries", got)
	}
	if nas, _ := rr.Get("agent-nas"); nas.TrueNAS == nil || nas.Agent == nil {
		t.Fatalf("linked TrueNAS system truenas=%t agent=%t, want both payloads", nas.TrueNAS != nil, nas.Agent != nil)
	}
	if host, _ := rr.Get("agent-esxi"); host.VMware == nil || host.Agent == nil {
		t.Fatalf("linked vSphere host vmware=%t agent=%t, want both payloads", host.VMware != nil, host.Agent != nil)
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
// order; quiet sources only decide when no source is current. Availability
// checks on a monitored resource are judged by their own evidence, not their
// shared sighting.
func TestAggregateStatusReadsSourceVerdictsNotDelivery(t *testing.T) {
	now := time.Now().UTC()
	lapsed := now.Add(-10 * time.Minute)
	current := func(reported ResourceStatus) SourceStatus {
		return SourceStatus{Status: "online", reported: reported}
	}
	quiet := func(reported ResourceStatus) SourceStatus {
		return SourceStatus{Status: "stale", reported: reported}
	}
	check := func(passing bool, checkedAt time.Time) []AvailabilityData {
		return []AvailabilityData{{
			TargetID: "probe-1", Enabled: true, Available: passing, LastChecked: &checkedAt,
			Evidence: availabilityProbeEvidence(t, "probe-1", checkedAt),
		}}
	}
	for _, tc := range []struct {
		name         string
		resourceType ResourceType
		sightings    map[DataSource]SourceStatus
		checks       []AvailabilityData
		want         ResourceStatus
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
			sightings: map[DataSource]SourceStatus{SourceProxmox: current(StatusOffline), SourcePBS: current("")},
			want:      StatusOffline,
		},
		{
			name:      "a current failing check abstains",
			sightings: map[DataSource]SourceStatus{SourceProxmox: quiet(StatusOffline), SourceAvailability: current("")},
			checks:    check(false, now),
			want:      StatusOffline,
		},
		{
			name:      "a quiet failing check abstains rather than lifting an offline verdict to warning",
			sightings: map[DataSource]SourceStatus{SourceProxmox: quiet(StatusOffline), SourceAvailability: quiet("")},
			checks:    check(false, lapsed),
			want:      StatusOffline,
		},
		{
			name:      "a passing check whose evidence lapsed abstains under a current sighting",
			sightings: map[DataSource]SourceStatus{SourceProxmox: quiet(StatusOnline), SourceAvailability: current("")},
			checks:    check(true, lapsed),
			want:      StatusWarning,
		},
		{
			name:      "a passing check with current evidence proves the target answers under a quiet sighting",
			sightings: map[DataSource]SourceStatus{SourceProxmox: quiet(StatusOffline), SourceAvailability: quiet("")},
			checks:    check(true, now),
			want:      StatusOnline,
		},
		{
			name:      "a passing check never lifts a resource whose own sources have not gone quiet",
			sightings: map[DataSource]SourceStatus{SourceProxmox: {Status: "unknown"}, SourceAvailability: current("")},
			checks:    check(true, now),
			want:      StatusUnknown,
		},
		{
			name:      "a passing check never outranks a current verdict",
			sightings: map[DataSource]SourceStatus{SourceProxmox: current(StatusOffline), SourceAvailability: current("")},
			checks:    check(true, now),
			want:      StatusOffline,
		},
		{
			name:         "a check's own row keeps its quiet verdict",
			resourceType: ResourceTypeNetworkEndpoint,
			sightings:    map[DataSource]SourceStatus{SourceAvailability: quiet(StatusOffline)},
			want:         StatusOffline,
		},
		{
			name:         "a check's own row keeps its current verdict",
			resourceType: ResourceTypeNetworkEndpoint,
			sightings:    map[DataSource]SourceStatus{SourceAvailability: current(StatusWarning)},
			want:         StatusWarning,
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
			resource := &Resource{Type: tc.resourceType, SourceStatus: tc.sightings, AvailabilityChecks: tc.checks}
			if got := aggregateStatus(resource, now); got != tc.want {
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

const sharedDiskSerial = "SHARED-SERIAL-1"

// sharedSerialDiskSnapshot puts one /dev/sda carrying sharedDiskSerial under
// each named PVE node of one instance, optionally with a linked host agent
// reporting that disk over SMART on every node. pve1 also carries a disk whose
// serial no other machine reports.
func sharedSerialDiskSnapshot(now time.Time, withAgents bool, nodeNames ...string) models.StateSnapshot {
	var snapshot models.StateSnapshot
	for _, name := range nodeNames {
		node := models.Node{
			ID:       proxmoxNodeSourceID("pve", name),
			Name:     name,
			Instance: "pve",
			Host:     "https://" + name + ":8006",
			Status:   "online",
			LastSeen: now,
		}
		if withAgents {
			node.LinkedAgentID = "host-" + name
			snapshot.Hosts = append(snapshot.Hosts, models.Host{
				ID:           "host-" + name,
				Hostname:     name,
				MachineID:    "machine-" + name,
				LinkedNodeID: node.ID,
				Status:       "online",
				LastSeen:     now,
				Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
					Device:      "/dev/sda",
					Model:       "SEAGATE ST4000NM0023",
					Serial:      sharedDiskSerial,
					Type:        "sas",
					Temperature: 31,
					Health:      "PASSED",
				}}},
			})
		}
		snapshot.Nodes = append(snapshot.Nodes, node)
		snapshot.PhysicalDisks = append(snapshot.PhysicalDisks, models.PhysicalDisk{
			ID:          ProxmoxPhysicalDiskSourceID("pve", name, "/dev/sda", "", ""),
			Instance:    "pve",
			Node:        name,
			DevPath:     "/dev/sda",
			Model:       "SEAGATE ST4000NM0023",
			Serial:      sharedDiskSerial,
			Type:        "sas",
			Health:      "PASSED",
			LastChecked: now,
		})
		if name == "pve1" {
			snapshot.PhysicalDisks = append(snapshot.PhysicalDisks, models.PhysicalDisk{
				ID:          ProxmoxPhysicalDiskSourceID("pve", name, "/dev/sdb", "", ""),
				Instance:    "pve",
				Node:        name,
				DevPath:     "/dev/sdb",
				Model:       "Samsung SSD 870",
				Serial:      "UNIQUE-SERIAL-1",
				Type:        "ssd",
				Health:      "PASSED",
				LastChecked: now,
			})
		}
	}
	return snapshot
}

// reversedSnapshot reverses every slice sharedSerialDiskSnapshot fills, so a
// test can prove IDs do not depend on ingest order.
func reversedSnapshot(snapshot models.StateSnapshot) models.StateSnapshot {
	slices.Reverse(snapshot.Nodes)
	slices.Reverse(snapshot.Hosts)
	slices.Reverse(snapshot.PhysicalDisks)
	return snapshot
}

// sharedSerialDisksByNode indexes the registry's sharedDiskSerial disks by
// Proxmox node, failing when two claim one node or one lost its node.
func sharedSerialDisksByNode(t *testing.T, rr *ResourceRegistry) map[string]Resource {
	t.Helper()
	byNode := make(map[string]Resource)
	for _, disk := range rr.ListByType(ResourceTypePhysicalDisk) {
		if disk.PhysicalDisk == nil || disk.PhysicalDisk.Serial != sharedDiskSerial {
			continue
		}
		if disk.Proxmox == nil || disk.Proxmox.NodeName == "" {
			t.Fatalf("disk %s has no Proxmox node: %+v", disk.ID, disk.Proxmox)
		}
		if _, dup := byNode[disk.Proxmox.NodeName]; dup {
			t.Fatalf("two disks claim node %s", disk.Proxmox.NodeName)
		}
		byNode[disk.Proxmox.NodeName] = disk
	}
	return byNode
}

func TestPhysicalDisksSharingASerialStayOnEachMachine(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		withAgents bool
		sources    []DataSource
	}{
		{name: "proxmox only", sources: []DataSource{SourceProxmox}},
		{name: "linked host agents", withAgents: true, sources: []DataSource{SourceProxmox, SourceAgent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := NewRegistry(nil)
			rr.IngestSnapshot(sharedSerialDiskSnapshot(now, tc.withAgents, "pve1", "pve2", "pve3"))

			byNode := sharedSerialDisksByNode(t, rr)
			if len(byNode) != 3 {
				t.Fatalf("shared-serial disks by node = %d, want one per node", len(byNode))
			}
			bareID := rr.canonicalIDFromIdentity(ResourceTypePhysicalDisk, ResourceIdentity{MachineID: sharedDiskSerial})
			for _, name := range []string{"pve1", "pve2", "pve3"} {
				disk := byNode[name]
				nodeID := rr.bySource[SourceProxmox][proxmoxNodeSourceID("pve", name)]
				if disk.ParentID == nil || *disk.ParentID != nodeID {
					t.Fatalf("disk on %s parent = %v, want its node %s", name, disk.ParentID, nodeID)
				}
				for _, source := range tc.sources {
					if !containsDataSource(disk.Sources, source) {
						t.Fatalf("disk on %s sources = %v, want %s merged in", name, disk.Sources, source)
					}
				}
				if !slices.ContainsFunc(rr.GetChildren(nodeID), func(child Resource) bool { return child.ID == disk.ID }) {
					t.Fatalf("node %s does not list its disk %s as a child", name, disk.ID)
				}
				// No machine keeps the unscoped ID, so it can never pass
				// from one machine's disk to another's.
				if disk.ID == bareID {
					t.Fatalf("disk on %s holds the unscoped ID %s shared by every machine", name, bareID)
				}
			}

			uniqueID := rr.canonicalIDFromIdentity(ResourceTypePhysicalDisk, ResourceIdentity{MachineID: "UNIQUE-SERIAL-1"})
			if unique, ok := rr.Get(uniqueID); !ok || unique.PhysicalDisk == nil || unique.PhysicalDisk.Serial != "UNIQUE-SERIAL-1" {
				t.Fatalf("a disk only one machine reports must keep its unscoped ID %s", uniqueID)
			}

			reordered := NewRegistry(nil)
			reordered.IngestSnapshot(reversedSnapshot(sharedSerialDiskSnapshot(now, tc.withAgents, "pve1", "pve2", "pve3")))
			for name, disk := range sharedSerialDisksByNode(t, reordered) {
				if disk.ID != byNode[name].ID {
					t.Fatalf("reversed ingest moved the disk on %s from %s to %s", name, byNode[name].ID, disk.ID)
				}
			}
		})
	}
}

func TestPhysicalDiskIDsHoldAcrossRehydrationWhenAMachineJoins(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	first := NewRegistry(nil)
	first.IngestSnapshot(sharedSerialDiskSnapshot(now, false, "pve2", "pve3"))
	before := sharedSerialDisksByNode(t, first)

	rehydrated := NewRegistry(nil)
	rehydrated.IngestResources(first.List())
	rehydrated.IngestSnapshot(sharedSerialDiskSnapshot(now.Add(time.Minute), false, "pve1", "pve2", "pve3"))
	after := sharedSerialDisksByNode(t, rehydrated)
	if len(after) != 3 {
		t.Fatalf("shared-serial disks by node = %d, want one per node", len(after))
	}
	for name, disk := range before {
		if after[name].ID != disk.ID {
			t.Fatalf("disk on %s moved from %s to %s when pve1 joined", name, disk.ID, after[name].ID)
		}
	}

	fresh := NewRegistry(nil)
	fresh.IngestSnapshot(sharedSerialDiskSnapshot(now.Add(time.Minute), false, "pve1", "pve2", "pve3"))
	for name, disk := range sharedSerialDisksByNode(t, fresh) {
		if after[name].ID != disk.ID {
			t.Fatalf("disk on %s is %s after rehydration but %s after a fresh rebuild", name, after[name].ID, disk.ID)
		}
	}
}

// trueNASDiskRecords describes one TrueNAS system holding one disk, under a
// pool when pool is set.
func trueNASDiskRecords(now time.Time, system, hostname, pool, serial string) []IngestRecord {
	records := []IngestRecord{{
		SourceID: "system:" + system,
		Resource: Resource{Type: ResourceTypeAgent, Name: hostname, Status: StatusOnline, LastSeen: now, TrueNAS: &TrueNASData{Hostname: hostname}},
		Identity: ResourceIdentity{Hostnames: []string{hostname}},
	}}
	diskParent := "system:" + system
	if pool != "" {
		diskParent = "pool:" + system + ":" + pool
		records = append(records, IngestRecord{
			SourceID:       diskParent,
			ParentSourceID: "system:" + system,
			Resource:       Resource{Type: ResourceTypeStorage, Name: pool, Status: StatusOnline, LastSeen: now},
			Identity:       ResourceIdentity{Hostnames: []string{hostname}},
		})
	}
	return append(records, IngestRecord{
		SourceID:       "disk:" + system + ":sda",
		ParentSourceID: diskParent,
		Resource: Resource{
			Type: ResourceTypePhysicalDisk, Name: "sda", Status: StatusOnline, LastSeen: now,
			PhysicalDisk: &PhysicalDiskMeta{DevPath: "/dev/sda", Serial: serial, Wearout: WearoutUnreported},
		},
		Identity: ResourceIdentity{MachineID: serial, Hostnames: []string{hostname}},
	})
}

func TestPhysicalDiskReportersOnOneMachineJoinAcrossGroupingParents(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	agentOnArchive := models.StateSnapshot{Hosts: []models.Host{{
		ID: "host-archive", Hostname: "archive", MachineID: "machine-archive", Status: "online", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
			Device: "/dev/sda", Model: "WDC WD80EFAX", Serial: sharedDiskSerial, Type: "sata", Temperature: 30, Health: "PASSED",
		}}},
	}}}

	// The TrueNAS API and the agent on the same box report one disk under
	// different parents (pool and agent host) that the registry never merged.
	rr := NewRegistry(nil)
	rr.IngestSnapshot(agentOnArchive)
	rr.IngestRecords(SourceTrueNAS, trueNASDiskRecords(now, "a", "archive", "tank", sharedDiskSerial))
	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 || !containsDataSource(disks[0].Sources, SourceAgent) || !containsDataSource(disks[0].Sources, SourceTrueNAS) {
		t.Fatalf("disks = %+v, want the agent and TrueNAS rows of one box merged", disks)
	}

	// A cloned TrueNAS keeps both the serial and the hostname; one reporter
	// placing the serial on two systems still proves two disks.
	rr.IngestRecords(SourceTrueNAS, trueNASDiskRecords(now, "b", "archive", "tank", sharedDiskSerial))
	disks = rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 2 {
		t.Fatalf("disks = %d, want the cloned system's disk kept apart", len(disks))
	}
	bySystem := make(map[string]Resource)
	for _, disk := range disks {
		for _, source := range []string{"disk:a:sda", "disk:b:sda"} {
			if rr.bySource[SourceTrueNAS][source] == disk.ID {
				bySystem[source] = disk
			}
		}
	}
	if len(bySystem) != 2 {
		t.Fatalf("TrueNAS disk mappings = %+v, want one disk per system", rr.bySource[SourceTrueNAS])
	}
	if !containsDataSource(bySystem["disk:a:sda"].Sources, SourceAgent) {
		t.Fatalf("system a disk sources = %v, want the agent row still merged", bySystem["disk:a:sda"].Sources)
	}

	// A disk moving into a pool on the same machine is the same disk.
	moved := NewRegistry(nil)
	moved.IngestRecords(SourceTrueNAS, trueNASDiskRecords(now, "a", "archive", "", sharedDiskSerial))
	moved.IngestRecords(SourceTrueNAS, trueNASDiskRecords(now.Add(time.Minute), "a", "archive", "tank", sharedDiskSerial))
	if disks := moved.ListByType(ResourceTypePhysicalDisk); len(disks) != 1 {
		t.Fatalf("disks after moving into a pool = %d, want 1", len(disks))
	}
}

func TestPhysicalDiskWithoutItsNodeRowJoinsItsOwnMachinesDisk(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	snapshot := sharedSerialDiskSnapshot(now, false, "pve1", "pve2")
	// pve3 has an agent but no node row this poll, so its PVE disk arrives
	// without a parent while the serial already spans other machines.
	snapshot.Hosts = append(snapshot.Hosts, models.Host{
		ID: "host-pve3", Hostname: "pve3", MachineID: "machine-pve3", Status: "online", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
			Device: "/dev/sda", Model: "SEAGATE ST4000NM0023", Serial: sharedDiskSerial, Type: "sas", Temperature: 31, Health: "PASSED",
		}}},
	})
	snapshot.PhysicalDisks = append(snapshot.PhysicalDisks, models.PhysicalDisk{
		ID: ProxmoxPhysicalDiskSourceID("pve", "pve3", "/dev/sda", "", ""), Instance: "pve", Node: "pve3",
		DevPath: "/dev/sda", Model: "SEAGATE ST4000NM0023", Serial: sharedDiskSerial, Type: "sas", Health: "PASSED", LastChecked: now,
	})

	rr := NewRegistry(nil)
	rr.IngestSnapshot(snapshot)
	byNode := sharedSerialDisksByNode(t, rr)
	if len(byNode) != 3 {
		t.Fatalf("shared-serial disks by node = %d, want one per node", len(byNode))
	}
	if sources := byNode["pve3"].Sources; !containsDataSource(sources, SourceAgent) || !containsDataSource(sources, SourceProxmox) {
		t.Fatalf("pve3 disk sources = %v, want its agent and parentless PVE rows merged", sources)
	}
}

func TestPhysicalDiskSiblingJoinHonoursManualExclusions(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	// An agent on pve1 that the registry never linked to the node reports the
	// disk after both nodes' rows; hostname evidence alone joins it to pve1's.
	agentOnPVE1 := models.StateSnapshot{Hosts: []models.Host{{
		ID: "host-pve1", Hostname: "pve1", MachineID: "machine-pve1", Status: "online", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
			Device: "/dev/sda", Model: "SEAGATE ST4000NM0023", Serial: sharedDiskSerial, Type: "sas", Temperature: 31, Health: "PASSED",
		}}},
	}}}
	ingest := func(store ResourceStore) *ResourceRegistry {
		rr := NewRegistry(store)
		rr.IngestSnapshot(sharedSerialDiskSnapshot(now, false, "pve1", "pve2"))
		rr.IngestSnapshot(agentOnPVE1)
		return rr
	}

	joined := ingest(nil)
	pve1Disk := sharedSerialDisksByNode(t, joined)["pve1"]
	if !containsDataSource(pve1Disk.Sources, SourceAgent) {
		t.Fatalf("pve1 disk sources = %v, want the agent row joined", pve1Disk.Sources)
	}

	store := NewMemoryStore()
	agentCandidate := buildHashID(ResourceTypePhysicalDisk, string(SourceAgent)+":"+normalizeSourceID(sharedDiskSerial))
	if err := store.AddExclusion(ResourceExclusion{ResourceA: pve1Disk.ID, ResourceB: agentCandidate}); err != nil {
		t.Fatal(err)
	}
	split := ingest(store)
	if disk, ok := split.Get(pve1Disk.ID); !ok || containsDataSource(disk.Sources, SourceAgent) {
		t.Fatalf("excluded pve1 disk = %+v, want it kept apart from the agent row", disk)
	}
	if agentDisk, ok := split.Get(agentCandidate); !ok || !containsDataSource(agentDisk.Sources, SourceAgent) {
		t.Fatalf("agent row should keep its source-specific ID %s once excluded", agentCandidate)
	}
}

func TestPhysicalDiskClonesStayApartAfterSerializedRehydration(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	first := NewRegistry(nil)
	first.IngestRecords(SourceTrueNAS, trueNASDiskRecords(now, "a", "archive", "tank", sharedDiskSerial))
	payload, err := json.Marshal(first.List())
	if err != nil {
		t.Fatal(err)
	}
	var persisted []Resource
	if err := json.Unmarshal(payload, &persisted); err != nil {
		t.Fatal(err)
	}

	rehydrated := NewRegistry(nil)
	rehydrated.IngestResources(persisted)
	// A clone keeps the hostname; the persisted disk lost its per-source
	// parents, so only its own parent says which system TrueNAS saw it on.
	rehydrated.IngestRecords(SourceTrueNAS, trueNASDiskRecords(now, "b", "archive", "tank", sharedDiskSerial))
	if disks := rehydrated.ListByType(ResourceTypePhysicalDisk); len(disks) != 2 {
		t.Fatalf("disks = %d, want the clone's disk kept apart after rehydration", len(disks))
	}
}

func TestPhysicalDiskRekeyCarriesRetiredIDClaims(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	records := trueNASDiskRecords(now, "a", "archive-a", "tank", sharedDiskSerial)
	records[len(records)-1].SupersededCanonicalIDs = []string{"physical_disk-retired"}
	rr := NewRegistry(nil)
	rr.IngestRecords(SourceTrueNAS, records)
	bareID := rr.canonicalIDFromIdentity(ResourceTypePhysicalDisk, ResourceIdentity{MachineID: sharedDiskSerial})
	if got := rr.supersededResourceIDLocked("physical_disk-retired"); got != bareID {
		t.Fatalf("retired ID resolves to %q, want the disk %s", got, bareID)
	}

	rr.IngestRecords(SourceTrueNAS, trueNASDiskRecords(now, "b", "archive-b", "tank", sharedDiskSerial))
	systemADisk := rr.bySource[SourceTrueNAS]["disk:a:sda"]
	if systemADisk == bareID || systemADisk == "" {
		t.Fatalf("system a disk = %q, want it re-keyed off the unscoped ID", systemADisk)
	}
	if got := rr.supersededResourceIDLocked("physical_disk-retired"); got != systemADisk {
		t.Fatalf("retired ID resolves to %q after the re-key, want %s", got, systemADisk)
	}
}

func TestPhysicalDiskSplitSurvivesARekey(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	agentDisk := func(id, hostname string) models.Host {
		return models.Host{
			ID: id, Hostname: hostname, MachineID: "machine-" + hostname, Status: "online", LastSeen: now,
			Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
				Device: "/dev/sda", Model: "WDC WD80EFAX", Serial: sharedDiskSerial, Type: "sata", Temperature: 30, Health: "PASSED",
			}}},
		}
	}
	trueNASCandidate := buildHashID(ResourceTypePhysicalDisk, string(SourceTrueNAS)+":"+normalizeSourceID("disk:a:sda"))
	bareID := MachineIdentityCanonicalID(ResourceTypePhysicalDisk, sharedDiskSerial)
	ingest := func(t *testing.T, excludedID string, hosts ...models.Host) *ResourceRegistry {
		t.Helper()
		store := NewMemoryStore()
		if err := store.AddExclusion(ResourceExclusion{ResourceA: excludedID, ResourceB: trueNASCandidate}); err != nil {
			t.Fatal(err)
		}
		rr := NewRegistry(store)
		rr.IngestSnapshot(models.StateSnapshot{Hosts: hosts})
		rr.IngestRecords(SourceTrueNAS, trueNASDiskRecords(now, "a", "archive", "tank", sharedDiskSerial))
		return rr
	}
	split := func(t *testing.T, rr *ResourceRegistry) {
		t.Helper()
		trueNASDisk, ok := rr.Get(trueNASCandidate)
		if !ok || containsDataSource(trueNASDisk.Sources, SourceAgent) {
			t.Fatalf("TrueNAS row = %+v, want it kept on %s apart from the agent row", trueNASDisk, trueNASCandidate)
		}
	}

	t.Run("split recorded against the unscoped ID after another machine re-keyed the disk", func(t *testing.T) {
		rr := ingest(t, bareID, agentDisk("host-archive", "archive"), agentDisk("host-other", "other"))
		if _, ok := rr.Get(bareID); ok {
			t.Fatalf("the unscoped ID %s should be re-keyed once two machines report the serial", bareID)
		}
		split(t, rr)
	})
	t.Run("split recorded against the scoped ID before the serial spans machines", func(t *testing.T) {
		scoped := ingest(t, bareID, agentDisk("host-archive", "archive"), agentDisk("host-other", "other"))
		archiveAgentDisk := ""
		for _, disk := range scoped.ListByType(ResourceTypePhysicalDisk) {
			if disk.ParentID != nil && *disk.ParentID == scoped.bySource[SourceAgent]["host-archive"] {
				archiveAgentDisk = disk.ID
			}
		}
		if archiveAgentDisk == "" || archiveAgentDisk == bareID {
			t.Fatalf("archive agent disk = %q, want a machine-scoped ID", archiveAgentDisk)
		}
		split(t, ingest(t, archiveAgentDisk, agentDisk("host-archive", "archive")))
	})
}

func TestPhysicalDiskExclusionFallbackStaysPerMachine(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	agents := models.StateSnapshot{}
	for _, name := range []string{"pve1", "pve2"} {
		agents.Hosts = append(agents.Hosts, models.Host{
			ID: "host-" + name, Hostname: name, MachineID: "machine-" + name, Status: "online", LastSeen: now,
			Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
				Device: "/dev/sda", Model: "SEAGATE ST4000NM0023", Serial: sharedDiskSerial, Type: "sas", Temperature: 31, Health: "PASSED",
			}}},
		})
	}
	probe := NewRegistry(nil)
	probe.IngestSnapshot(sharedSerialDiskSnapshot(now, false, "pve1", "pve2"))
	byNode := sharedSerialDisksByNode(t, probe)

	// Both nodes' disks are split from the agent observation, whose
	// source-specific ID is the same on every host.
	store := NewMemoryStore()
	agentCandidate := buildHashID(ResourceTypePhysicalDisk, string(SourceAgent)+":"+normalizeSourceID(sharedDiskSerial))
	for _, name := range []string{"pve1", "pve2"} {
		if err := store.AddExclusion(ResourceExclusion{ResourceA: byNode[name].ID, ResourceB: agentCandidate}); err != nil {
			t.Fatal(err)
		}
	}
	rr := NewRegistry(store)
	rr.IngestSnapshot(sharedSerialDiskSnapshot(now, false, "pve1", "pve2"))
	rr.IngestSnapshot(agents)

	agentDisks := 0
	for _, disk := range rr.ListByType(ResourceTypePhysicalDisk) {
		if !containsDataSource(disk.Sources, SourceAgent) {
			continue
		}
		agentDisks++
		if containsDataSource(disk.Sources, SourceProxmox) {
			t.Fatalf("disk %s merged an excluded agent row into a node's disk", disk.ID)
		}
	}
	if agentDisks != 2 {
		t.Fatalf("agent disks = %d, want one per host rather than both hosts on the shared fallback ID", agentDisks)
	}
}
