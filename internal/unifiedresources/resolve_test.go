package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestResolveResource_NilReadState(t *testing.T) {
	loc := ResolveResource(nil, "anything")
	if loc.Found {
		t.Fatal("expected not found for nil ReadState")
	}
}

func TestResolveResource_TrimmedLookupName(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{ID: "host1", Hostname: "myserver", Platform: "linux"}},
	})

	loc := ResolveResource(rr, "  myserver  ")
	if !loc.Found || loc.ResourceType != "agent" {
		t.Fatalf("expected trimmed lookup to find agent, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.Name != "myserver" {
		t.Fatalf("expected canonical name myserver, got %q", loc.Name)
	}
	if loc.TargetID != "host1" {
		t.Fatalf("expected TargetID=host1, got %q", loc.TargetID)
	}
}

func TestResolveResource_WhitespaceOnlyInput(t *testing.T) {
	loc := ResolveResource(nil, "   ")
	if loc.Found {
		t.Fatal("expected whitespace-only lookup to be not found")
	}
	if loc.Name != "" {
		t.Fatalf("expected empty normalized name, got %q", loc.Name)
	}
}

func TestResolveResource_Node(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "pve-node"}},
	})
	loc := ResolveResource(rr, "pve-node")
	if !loc.Found {
		t.Fatal("expected node to be found")
	}
	if loc.ResourceType != "node" {
		t.Fatalf("expected node type, got %q", loc.ResourceType)
	}
	if loc.TargetHost != "pve-node" {
		t.Fatalf("expected target_host pve-node, got %q", loc.TargetHost)
	}
}

func TestResolveResource_VM(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		VMs: []models.VM{{ID: "vm-1", Name: "alpha", VMID: 101, Node: "node1"}},
	})
	loc := ResolveResource(rr, "alpha")
	if !loc.Found || loc.ResourceType != "vm" {
		t.Fatalf("expected vm, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.VMID != 101 || loc.Node != "node1" {
		t.Fatalf("expected VMID=101 Node=node1, got VMID=%d Node=%q", loc.VMID, loc.Node)
	}
}

func TestResolveResource_Container(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Containers: []models.Container{{ID: "lxc-1", Name: "beta", VMID: 201, Node: "node1", Type: "lxc"}},
	})
	loc := ResolveResource(rr, "beta")
	if !loc.Found || loc.ResourceType != "system-container" {
		t.Fatalf("expected system-container, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.VMID != 201 {
		t.Fatalf("expected VMID=201, got %d", loc.VMID)
	}
}

func TestResolveResource_DockerContainer(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Containers: []models.Container{{ID: "lxc-1", Name: "dock1", VMID: 100, Node: "node1", Type: "lxc"}},
		DockerHosts: []models.DockerHost{{
			ID:       "dock1",
			Hostname: "dock1",
			Containers: []models.DockerContainer{{
				ID:    "cid1",
				Name:  "homepage",
				State: "running",
			}},
		}},
	})
	loc := ResolveResource(rr, "homepage")
	if !loc.Found || loc.ResourceType != "app-container" {
		t.Fatalf("expected app-container, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.DockerHostName != "dock1" {
		t.Fatalf("expected docker host dock1, got %q", loc.DockerHostName)
	}
	if loc.DockerHostType != "system-container" {
		t.Fatalf("expected docker host type system-container, got %q", loc.DockerHostType)
	}
	if loc.DockerHostVMID != 100 {
		t.Fatalf("expected docker host VMID 100, got %d", loc.DockerHostVMID)
	}
	// TargetHost must be rewritten to the LXC name for command routing.
	if loc.TargetHost != "dock1" {
		t.Fatalf("expected target_host rewritten to LXC name dock1, got %q", loc.TargetHost)
	}
}

func TestResolveResource_DockerContainerTargetHostRewrite(t *testing.T) {
	// Docker container lookup must rewrite TargetHost to the backing LXC name.
	// Use Docker host ID matching (not hostname) to verify the rewrite path.
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Containers: []models.Container{{ID: "lxc-1", Name: "docker-host-lxc", VMID: 100, Node: "node1", Type: "lxc"}},
		DockerHosts: []models.DockerHost{{
			ID:       "docker-host-lxc",
			Hostname: "docker-host-lxc",
			Containers: []models.DockerContainer{{
				ID:    "cid1",
				Name:  "myapp",
				State: "running",
			}},
		}},
	})
	loc := ResolveResource(rr, "myapp")
	if !loc.Found || loc.ResourceType != "app-container" {
		t.Fatalf("expected app-container, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.TargetHost != "docker-host-lxc" {
		t.Fatalf("expected target_host rewritten to LXC name, got %q", loc.TargetHost)
	}
	if loc.DockerHostType != "system-container" {
		t.Fatalf("expected system-container, got %q", loc.DockerHostType)
	}

	// Docker HOST lookup must NOT rewrite TargetHost.
	locHost := ResolveResource(rr, "docker-host-lxc")
	if !locHost.Found || locHost.ResourceType != "system-container" {
		// Should match as system-container first (earlier in resolution order)
		t.Fatalf("expected system-container, got found=%v type=%q", locHost.Found, locHost.ResourceType)
	}
}

func TestResolveResource_DockerHost(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID:       "standalone1",
			Hostname: "standalone1",
		}},
	})
	loc := ResolveResource(rr, "standalone1")
	if !loc.Found || loc.ResourceType != "docker-host" {
		t.Fatalf("expected docker-host, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.DockerHostType != "standalone" {
		t.Fatalf("expected standalone, got %q", loc.DockerHostType)
	}
}

func TestResolveResource_TrueNASAppDoesNotUseDockerRouting(t *testing.T) {
	rr := newTrueNASAppRegistryFixture()

	loc := ResolveResource(rr, "Nextcloud")
	if !loc.Found || loc.ResourceType != "app-container" {
		t.Fatalf("expected TrueNAS app-container, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.TargetHost != "truenas-main" {
		t.Fatalf("expected target host truenas-main, got %q", loc.TargetHost)
	}
	if loc.DockerHostName != "" || loc.DockerHostType != "" {
		t.Fatalf("expected TrueNAS app to avoid docker routing, got docker_host=%q type=%q", loc.DockerHostName, loc.DockerHostType)
	}
}

func TestResolveResource_VMwareStorage(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Now().UTC()
	rr.IngestRecords(SourceVMware, []IngestRecord{{
		SourceID: "vc-1:datastore:datastore-11",
		Resource: Resource{
			Type:       ResourceTypeStorage,
			Name:       "nvme-primary",
			Status:     StatusOnline,
			LastSeen:   now,
			UpdatedAt:  now,
			ParentName: "Lab VC",
			Storage:    &StorageMeta{Type: "vmfs"},
			VMware: &VMwareData{
				ConnectionID:    "vc-1",
				ConnectionName:  "Lab VC",
				ManagedObjectID: "datastore-11",
				EntityType:      "datastore",
			},
		},
	}})

	loc := ResolveResource(rr, "nvme-primary")
	if !loc.Found || loc.ResourceType != "storage" {
		t.Fatalf("expected storage resource, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.TargetID != "Lab VC" || loc.TargetHost != "Lab VC" {
		t.Fatalf("expected VMware storage target to route through Lab VC, got %+v", loc)
	}

	resolved := ResolveResourceContext(rr, "nvme-primary")
	if resolved.Resource == nil {
		t.Fatal("expected resolved VMware storage resource")
	}
	if resolved.Resource.Type != ResourceTypeStorage {
		t.Fatalf("expected storage resource payload, got %q", resolved.Resource.Type)
	}
	if resolved.Resource.VMware == nil || resolved.Resource.VMware.ManagedObjectID != "datastore-11" {
		t.Fatalf("expected VMware datastore metadata, got %+v", resolved.Resource.VMware)
	}
}

func TestLookupResolvedResource_TrueNASAppUsesCanonicalAppContainerSet(t *testing.T) {
	rr := newTrueNASAppRegistryFixture()

	loc := ResolveResource(rr, "Nextcloud")
	resource := lookupResolvedResource(rr, loc)
	if resource == nil {
		t.Fatal("expected resolved resource for TrueNAS app")
	}
	if resource.Type != ResourceTypeAppContainer {
		t.Fatalf("expected app-container resource, got %q", resource.Type)
	}
	if resource.TrueNAS == nil {
		t.Fatalf("expected TrueNAS metadata on resolved resource, got %+v", resource)
	}
	if resource.Name != "Nextcloud" {
		t.Fatalf("expected canonical app name Nextcloud, got %q", resource.Name)
	}
}

func newTrueNASAppRegistryFixture() *ResourceRegistry {
	rr := NewRegistry(nil)
	now := time.Now().UTC()
	hostID := "agent:truenas-main"
	hostSourceID := "truenas-system:truenas-main"
	rr.IngestRecords(SourceTrueNAS, []IngestRecord{
		{
			SourceID: hostSourceID,
			Resource: Resource{
				ID:        hostID,
				Type:      ResourceTypeAgent,
				Name:      "truenas-main",
				Status:    StatusOnline,
				LastSeen:  now,
				UpdatedAt: now,
				Agent: &AgentData{
					Hostname: "truenas-main",
					Platform: "truenas",
				},
				TrueNAS: &TrueNASData{Hostname: "truenas-main"},
			},
			Identity: ResourceIdentity{Hostnames: []string{"truenas-main"}},
		},
		{
			SourceID:       "app:nextcloud",
			ParentSourceID: hostSourceID,
			Resource: Resource{
				ID:         "app-container:truenas-main:nextcloud",
				Type:       ResourceTypeAppContainer,
				Name:       "Nextcloud",
				Status:     StatusOnline,
				LastSeen:   now,
				UpdatedAt:  now,
				ParentID:   stringPtr(hostID),
				ParentName: "truenas-main",
				Docker: &DockerData{
					ContainerID: "nextcloud",
					Hostname:    "truenas-main",
				},
				TrueNAS: &TrueNASData{Hostname: "truenas-main"},
				Canonical: &CanonicalIdentity{
					DisplayName: "Nextcloud",
					Hostname:    "truenas-main",
					PrimaryID:   "nextcloud",
					Aliases:     []string{"Nextcloud", "nextcloud"},
				},
				Tags: []string{"truenas", "app"},
			},
			Identity: ResourceIdentity{Hostnames: []string{"truenas-main"}},
		},
	})
	return rr
}

func stringPtr(value string) *string {
	return &value
}

func TestResolveResource_Host(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{ID: "host1", Hostname: "myserver", Platform: "linux"}},
	})
	loc := ResolveResource(rr, "myserver")
	if !loc.Found || loc.ResourceType != "agent" {
		t.Fatalf("expected agent, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.Platform != "linux" {
		t.Fatalf("expected linux platform, got %q", loc.Platform)
	}
	if loc.TargetID != "host1" {
		t.Fatalf("expected TargetID=host1 (canonical target ID), got %q", loc.TargetID)
	}

	// Lookup by agent/source ID should also work.
	loc2 := ResolveResource(rr, "host1")
	if !loc2.Found || loc2.ResourceType != "agent" {
		t.Fatalf("expected agent lookup by agent ID, got found=%v type=%q", loc2.Found, loc2.ResourceType)
	}
}

func TestResolveResource_K8sCluster(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:      "k8s1",
			Name:    "prod",
			AgentID: "agent-1",
		}},
	})
	loc := ResolveResource(rr, "prod")
	if !loc.Found || loc.ResourceType != "k8s-cluster" {
		t.Fatalf("expected k8s-cluster, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.K8sAgentID != "agent-1" {
		t.Fatalf("expected agent-1, got %q", loc.K8sAgentID)
	}
}

func TestResolveResource_K8sPod(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:      "k8s1",
			Name:    "prod",
			AgentID: "agent-1",
			Pods: []models.KubernetesPod{{
				Name:      "nginx-abc",
				Namespace: "default",
			}},
		}},
	})
	loc := ResolveResource(rr, "nginx-abc")
	if !loc.Found || loc.ResourceType != "k8s-pod" {
		t.Fatalf("expected k8s-pod, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.K8sNamespace != "default" {
		t.Fatalf("expected namespace default, got %q", loc.K8sNamespace)
	}
	if loc.K8sClusterName != "prod" {
		t.Fatalf("expected cluster prod, got %q", loc.K8sClusterName)
	}
}

func TestResolveResource_K8sDeployment(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:      "k8s1",
			Name:    "prod",
			AgentID: "agent-1",
			Deployments: []models.KubernetesDeployment{{
				Name:      "web-deploy",
				Namespace: "production",
			}},
		}},
	})
	loc := ResolveResource(rr, "web-deploy")
	if !loc.Found || loc.ResourceType != "k8s-deployment" {
		t.Fatalf("expected k8s-deployment, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.K8sNamespace != "production" {
		t.Fatalf("expected namespace production, got %q", loc.K8sNamespace)
	}
}

func TestResolveResource_NotFound(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{})
	loc := ResolveResource(rr, "nonexistent")
	if loc.Found {
		t.Fatal("expected not found")
	}
	if loc.Name != "nonexistent" {
		t.Fatalf("expected name preserved, got %q", loc.Name)
	}
}

func TestResolveResourceContext_ReturnsGovernedResource(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Containers: []models.Container{{
			ID:   "lxc-1",
			Name: "customer-pg",
			VMID: 201,
			Node: "node1",
			Type: "lxc",
			Tags: []string{"customer-data"},
		}},
	})

	resolved := ResolveResourceContext(rr, "customer-pg")
	if !resolved.Location.Found || resolved.Location.ResourceType != "system-container" {
		t.Fatalf("expected resolved system container, got %#v", resolved.Location)
	}
	if resolved.Resource == nil {
		t.Fatal("expected unified resource metadata")
	}
	if resolved.Resource.Policy == nil {
		t.Fatal("expected policy metadata on resolved resource")
	}
	if resolved.Resource.Policy.Sensitivity != ResourceSensitivityRestricted {
		t.Fatalf("expected restricted sensitivity, got %q", resolved.Resource.Policy.Sensitivity)
	}
	if resolved.Resource.Policy.Routing.Scope != ResourceRoutingScopeLocalOnly {
		t.Fatalf("expected local-only routing, got %q", resolved.Resource.Policy.Routing.Scope)
	}
	if resolved.Resource.AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary to be populated")
	}
	if resolved.Resource.AISafeSummary == resolved.Resource.Name {
		t.Fatalf("expected aiSafeSummary to avoid raw name, got %q", resolved.Resource.AISafeSummary)
	}
}

func TestResolveResourceContext_NotFound(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{})

	resolved := ResolveResourceContext(rr, "missing")
	if resolved.Location.Found {
		t.Fatalf("expected missing lookup to remain not found, got %#v", resolved.Location)
	}
	if resolved.Resource != nil {
		t.Fatalf("expected no resolved resource for missing lookup, got %#v", resolved.Resource)
	}
}

// linkedGuestEstate is an agent inside a vSphere VM, joined by an operator
// link, as Pulse sees it after a restart before the agent reports again: the
// published registry has only the VM, and saved-host continuity fills in the
// agent.
type linkedGuestEstate struct {
	saved     models.Host
	vmRecords []IngestRecord
	agentID   string
	vmID      string
}

func newLinkedGuestEstate(t *testing.T) linkedGuestEstate {
	t.Helper()
	now := time.Now().UTC()
	estate := linkedGuestEstate{
		saved: models.Host{
			ID: "host-app-guest", MachineID: "machine-app-guest", Hostname: "app-guest",
			Platform: "linux", Status: "offline", LastSeen: now.Add(-10 * time.Minute),
			Memory: models.Memory{Total: 8 << 30, Used: 7 << 30, Free: 1 << 30, Usage: 88},
		},
		vmRecords: []IngestRecord{{
			SourceID: "vc-1:vm:vm-42",
			Resource: Resource{
				Type:       ResourceTypeVM,
				Technology: "vmware",
				Name:       "app-guest",
				Status:     StatusOnline,
				LastSeen:   now,
				Metrics:    &ResourceMetrics{Memory: &MetricValue{Percent: 40, Source: SourceVMware}},
				VMware:     &VMwareData{ConnectionID: "vc-1", ManagedObjectID: "vm-42", EntityType: "vm"},
			},
			Identity: ResourceIdentity{Hostnames: []string{"app-guest"}},
		}},
	}
	unlinked := NewMonitorAdapter(NewRegistry(nil))
	unlinked.PopulateSnapshotAndSupplemental(
		models.StateSnapshot{Hosts: []models.Host{estate.saved}, LastUpdate: now},
		map[DataSource][]IngestRecord{SourceVMware: estate.vmRecords},
	)
	if len(unlinked.VMs()) != 1 || len(unlinked.Hosts()) != 1 {
		t.Fatalf("unlinked estate = %d VMs, %d agents, want one of each", len(unlinked.VMs()), len(unlinked.Hosts()))
	}
	estate.vmID, estate.agentID = unlinked.VMs()[0].ID(), unlinked.Hosts()[0].ID()
	return estate
}

// readStateAfterRestart publishes the VM alone and overlays the saved agent,
// as Monitor.GetUnifiedReadStateOrSnapshot does.
func (e linkedGuestEstate) readStateAfterRestart(store ResourceStore) *MonitorAdapter {
	published := NewMonitorAdapter(NewRegistry(store))
	published.PopulateSnapshotAndSupplemental(
		models.StateSnapshot{LastUpdate: time.Now().UTC()},
		map[DataSource][]IngestRecord{SourceVMware: e.vmRecords},
	)
	return ReadStateWithHostContinuity(published, []IngestRecord{HostIngestRecord(e.saved)}).(*MonitorAdapter)
}

// An operator link names one identity, so every reference to the saved
// agent resolves to the VM, as it does once the agent reports and folds in.
// The agent's saved telemetry stays on its own row in the read state and in
// the resources API, which seeds a registry from that listing and re-applies
// the links.
func TestContinuityAgentLinkedIntoLiveGuestAnswersToTheGuest(t *testing.T) {
	estate := newLinkedGuestEstate(t)
	store := NewMemoryStore()
	if err := store.AddLink(ResourceLink{ResourceA: estate.vmID, ResourceB: estate.agentID, PrimaryID: estate.agentID}); err != nil {
		t.Fatalf("add link: %v", err)
	}
	readState := estate.readStateAfterRestart(store)
	resources := NewRegistry(store)
	resources.IngestResources(readState.GetAll())

	for _, registry := range []struct {
		name     string
		registry *ResourceRegistry
	}{
		{"read state", readState.registry},
		{"resources API", resources},
	} {
		t.Run(registry.name, func(t *testing.T) {
			rr := registry.registry
			agent, ok := rr.Get(estate.agentID)
			if !ok || agent.Status != StatusOffline {
				t.Fatalf("saved agent row = %+v (listed=%v), want its own offline row", agent, ok)
			}
			vm, ok := rr.Get(estate.vmID)
			if !ok {
				t.Fatalf("linked VM %s missing", estate.vmID)
			}
			if vm.Agent != nil || vm.Status != StatusOnline || vm.Metrics == nil || vm.Metrics.Memory == nil || vm.Metrics.Memory.Source != SourceVMware {
				t.Fatalf("VM agent=%v status=%s metrics=%+v, want vSphere's online VM without the saved agent's payload", vm.Agent != nil, vm.Status, vm.Metrics)
			}
			// The hostname both rows carry stays unambiguous: the saved row shares
			// the VM's identity.
			for _, ref := range []string{estate.agentID, "agent:" + estate.saved.ID, estate.saved.MachineID, "app-guest"} {
				if resolved, ok := rr.ResolveReferenceID(ref); !ok || resolved != estate.vmID {
					t.Fatalf("ResolveReferenceID(%q) = %q (%v), want the link primary %s", ref, resolved, ok, estate.vmID)
				}
				if _, resolved, ok := rr.GetByReference(ref); !ok || resolved != estate.vmID {
					t.Fatalf("GetByReference(%q) = %q (%v), want the link primary %s", ref, resolved, ok, estate.vmID)
				}
			}
			if resolved, ok := rr.ResolveReferenceID(estate.vmID); !ok || resolved != estate.vmID {
				t.Fatalf("VM reference resolved to %q (%v)", resolved, ok)
			}
		})
	}
	if resolved, ok := readState.ResolveCanonicalResourceID("agent:" + estate.saved.ID); !ok || resolved != estate.vmID {
		t.Fatalf("read-state canonical resolution = %q (%v), want %s", resolved, ok, estate.vmID)
	}

	// Once the agent reports, the link folds it in as before.
	live := estate.saved
	live.Status, live.LastSeen = "online", time.Now().UTC()
	published := NewMonitorAdapter(NewRegistry(store))
	published.PopulateSnapshotAndSupplemental(
		models.StateSnapshot{Hosts: []models.Host{live}, LastUpdate: time.Now().UTC()},
		map[DataSource][]IngestRecord{SourceVMware: estate.vmRecords},
	)
	if len(published.Hosts()) != 0 {
		t.Fatal("reporting agent still listed beside its linked VM")
	}
	if resolved, ok := published.ResolveCanonicalResourceID(estate.agentID); !ok || resolved != estate.vmID {
		t.Fatalf("folded agent resolved to %q (%v), want %s", resolved, ok, estate.vmID)
	}
}

// A saved primary never captures a live member: the live row keeps its own
// references and telemetry, and the saved row its own, until the primary
// reports and the link folds them.
func TestContinuityPrimaryLeavesLiveLinkMemberItsOwnIdentity(t *testing.T) {
	now := time.Now().UTC()
	live := models.Host{ID: "host-live", MachineID: "machine-live", Hostname: "live", Platform: "linux", Status: "online", LastSeen: now, CPUUsage: 23}
	saved := models.Host{ID: "host-saved", MachineID: "machine-saved", Hostname: "saved", Platform: "linux", Status: "offline", LastSeen: now.Add(-time.Hour)}

	ids := NewMonitorAdapter(NewRegistry(nil))
	ids.PopulateFromSnapshot(models.StateSnapshot{Hosts: []models.Host{live, saved}, LastUpdate: now})
	if len(ids.Hosts()) != 2 {
		t.Fatalf("unlinked estate has %d agents, want 2", len(ids.Hosts()))
	}
	agentIDs := map[string]string{}
	for _, host := range ids.Hosts() {
		agentIDs[host.AgentID()] = host.ID()
	}
	liveID, savedID := agentIDs[live.ID], agentIDs[saved.ID]

	store := NewMemoryStore()
	if err := store.AddLink(ResourceLink{ResourceA: savedID, ResourceB: liveID, PrimaryID: savedID}); err != nil {
		t.Fatalf("add link: %v", err)
	}
	published := NewMonitorAdapter(NewRegistry(store))
	published.PopulateFromSnapshot(models.StateSnapshot{Hosts: []models.Host{live}, LastUpdate: now})
	readState := ReadStateWithHostContinuity(published, []IngestRecord{HostIngestRecord(saved)}).(*MonitorAdapter)
	resources := NewRegistry(store)
	resources.IngestResources(readState.GetAll())

	for name, rr := range map[string]*ResourceRegistry{"read state": readState.registry, "resources API": resources} {
		liveRow, ok := rr.Get(liveID)
		if !ok || liveRow.Status != StatusOnline || len(liveRow.Sources) != 1 {
			t.Fatalf("%s: live member = %+v (listed=%v), want its own online row", name, liveRow, ok)
		}
		if _, ok := rr.Get(savedID); !ok {
			t.Fatalf("%s: saved primary row missing", name)
		}
		for ref, want := range map[string]string{liveID: liveID, "agent:" + live.ID: liveID, savedID: savedID, "agent:" + saved.ID: savedID} {
			if resolved, ok := rr.ResolveReferenceID(ref); !ok || resolved != want {
				t.Fatalf("%s: %q resolved to %q (%v), want %s", name, ref, resolved, ok, want)
			}
		}
	}
}

// An observation that merges into a saved row makes it a live row again, so
// the link folds it: the resources API's record replay is one such path.
func TestObservationMergedIntoASavedRowLetsItsLinkFold(t *testing.T) {
	estate := newLinkedGuestEstate(t)
	store := NewMemoryStore()
	if err := store.AddLink(ResourceLink{ResourceA: estate.vmID, ResourceB: estate.agentID, PrimaryID: estate.vmID}); err != nil {
		t.Fatalf("add link: %v", err)
	}
	rr := estate.readStateAfterRestart(store).registry
	if _, held := rr.Get(estate.agentID); !held {
		t.Fatal("saved agent not listed before it reports")
	}
	live := estate.saved
	live.Status, live.LastSeen = "online", time.Now().UTC()
	rr.IngestRecords(SourceAgent, []IngestRecord{HostIngestRecord(live)})
	if _, listed := rr.Get(estate.agentID); listed {
		t.Fatal("reported agent still listed beside its linked VM")
	}
	if vm, ok := rr.Get(estate.vmID); !ok || vm.Agent == nil {
		t.Fatalf("linked VM = %+v (listed=%v), want it carrying the reported agent", vm, ok)
	}
}

// Holds follow the links one pass applies, whatever their order: a primary a
// later link folds away hands its held members to the row holding it, a saved
// member held under a saved primary answers to that primary's own target, and
// a cycle of saved members resolves each to itself.
func TestLinkHoldsFollowTheirPrimaryThroughThePass(t *testing.T) {
	saved := func(id string) Resource {
		return Resource{ID: id, Type: ResourceTypeAgent, Name: id, Status: StatusOffline, Sources: []DataSource{SourceAgent}, continuityOnly: true}
	}
	vm := func(id string) Resource {
		return Resource{ID: id, Type: ResourceTypeVM, Name: id, Status: StatusOnline, Sources: []DataSource{SourceVMware}}
	}
	for _, tc := range []struct {
		name      string
		resources []Resource
		links     []ResourceLink
		want      map[string]string
	}{
		{
			name:      "primary folded later",
			resources: []Resource{saved("agent-a"), vm("vm-v"), vm("vm-w")},
			links: []ResourceLink{
				{ResourceA: "agent-a", ResourceB: "vm-v", PrimaryID: "vm-v"},
				{ResourceA: "vm-v", ResourceB: "vm-w", PrimaryID: "vm-w"},
			},
			want: map[string]string{"agent-a": "vm-w", "vm-v": "vm-w", "vm-w": "vm-w"},
		},
		{
			name:      "saved primary held in turn",
			resources: []Resource{saved("agent-x"), saved("agent-a"), vm("vm-v")},
			links: []ResourceLink{
				{ResourceA: "agent-x", ResourceB: "agent-a", PrimaryID: "agent-a"},
				{ResourceA: "agent-a", ResourceB: "vm-v", PrimaryID: "vm-v"},
			},
			want: map[string]string{"agent-x": "vm-v", "agent-a": "vm-v", "vm-v": "vm-v"},
		},
		{
			name:      "cycle",
			resources: []Resource{saved("agent-a"), saved("agent-b"), saved("agent-c")},
			links: []ResourceLink{
				{ResourceA: "agent-a", ResourceB: "agent-b", PrimaryID: "agent-a"},
				{ResourceA: "agent-b", ResourceB: "agent-c", PrimaryID: "agent-b"},
				{ResourceA: "agent-c", ResourceB: "agent-a", PrimaryID: "agent-c"},
			},
			want: map[string]string{"agent-a": "agent-a", "agent-b": "agent-b", "agent-c": "agent-c"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := NewMemoryStore()
			for _, link := range tc.links {
				if err := store.AddLink(link); err != nil {
					t.Fatalf("add link: %v", err)
				}
			}
			rr := NewRegistry(store)
			rr.IngestResources(tc.resources)
			for ref, want := range tc.want {
				if resolved, ok := rr.ResolveReferenceID(ref); !ok || resolved != want {
					t.Fatalf("%q resolved to %q (%v), want %s", ref, resolved, ok, want)
				}
			}
			for _, resource := range tc.resources {
				if !resource.continuityOnly {
					continue
				}
				if row, listed := rr.Get(resource.ID); !listed || row.Status != StatusOffline {
					t.Fatalf("saved row %s = %+v (listed=%v), want it kept", resource.ID, row, listed)
				}
			}
		})
	}
}
