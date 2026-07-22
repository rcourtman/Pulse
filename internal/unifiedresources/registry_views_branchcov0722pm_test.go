package unifiedresources

import (
	"testing"
	"time"
)

// TestBranchcov0722PM mechanically raises branch/function coverage on four
// previously-uncovered *ResourceRegistry accessors declared in registry.go:
//   - ListForPresentation() []Resource
//   - MetricsTarget(resourceID string) *MetricsTarget
//   - DockerContainers() []*DockerContainerView
//   - K8sNodes() []*K8sNodeView
//
// Each subtest seeds a ResourceRegistry through the public IngestResources /
// IngestRecords paths (or, where the function semantics require an unmatched
// resource with no source mapping, via the in-package rr.resources map — the
// same pattern used by the sibling TestBuildMetricsTargetForRegistryFallsBackToStoredMetricsTarget)
// and asserts the concrete returned value, driving both arms of every
// conditional (empty vs populated, hit vs miss, nil vs non-nil, kind match vs
// kind mismatch). No source file or pre-existing test is modified.
func TestBranchcov0722PM(t *testing.T) {
	now := time.Date(2026, 7, 22, 14, 0, 0, 0, time.UTC)

	// ---------------------------------------------------------------------------
	// ListForPresentation
	// ---------------------------------------------------------------------------

	t.Run("ListForPresentation/EmptyRegistryReturnsNonNilEmptySlice", func(t *testing.T) {
		rr := NewRegistry(nil)
		got := rr.ListForPresentation()
		// ListForPresentation delegates to List() (which always allocates a
		// non-nil slice) and then to CoalescePresentationHostResourcesWithExclusions,
		// whose len==0 early-return hands that empty slice straight back.
		if got == nil {
			t.Fatal("expected non-nil empty slice for empty registry, got nil")
		}
		if len(got) != 0 {
			t.Fatalf("expected 0 resources for empty registry, got %d: %+v", len(got), got)
		}
	})

	t.Run("ListForPresentation/PreservesAllNonHostKindsSortedByName", func(t *testing.T) {
		rr := NewRegistry(nil)
		rr.IngestResources([]Resource{
			{ID: "vm-zeta", Type: ResourceTypeVM, Name: "zeta-vm", Status: StatusOnline, LastSeen: now},
			{ID: "k8snode-alpha", Type: ResourceTypeK8sNode, Name: "alpha-node", Status: StatusOnline, LastSeen: now},
			{ID: "container-beta", Type: ResourceTypeAppContainer, Name: "beta-app", Status: StatusOnline, LastSeen: now},
			{ID: "storage-gamma", Type: ResourceTypeStorage, Name: "gamma-pool", Status: StatusOnline, LastSeen: now},
		})

		got := rr.ListForPresentation()
		// None of the seeded resources are ResourceTypeAgent, so the host
		// coalesce step is a no-op and every input kind survives.
		if len(got) != 4 {
			t.Fatalf("expected all 4 non-host resources preserved, got %d: %+v", len(got), got)
		}
		// ListForPresentation sorts by canonical name (lowercased name, then
		// type, then id) via the List() it composes on top of.
		wantNames := []string{"alpha-node", "beta-app", "gamma-pool", "zeta-vm"}
		gotNames := make([]string, 0, len(got))
		for _, r := range got {
			gotNames = append(gotNames, r.Name)
		}
		assertStringSlice(t, gotNames, wantNames)

		// Every requested kind must be reachable in the output — the
		// function must not silently drop non-agent kinds.
		wantTypes := map[ResourceType]bool{
			ResourceTypeVM:           true,
			ResourceTypeK8sNode:      true,
			ResourceTypeAppContainer: true,
			ResourceTypeStorage:      true,
		}
		seenTypes := make(map[ResourceType]int, len(got))
		for _, r := range got {
			seenTypes[CanonicalResourceType(r.Type)]++
		}
		for wantType := range wantTypes {
			if seenTypes[wantType] != 1 {
				t.Fatalf("expected exactly one %q in presentation output, got %d (seen=%v)", wantType, seenTypes[wantType], seenTypes)
			}
		}
	})

	t.Run("ListForPresentation/CoalescesSplitAgentHostViews", func(t *testing.T) {
		// Two ResourceTypeAgent records that share a canonical short hostname
		// — one sourced from Proxmox (platform/runtime view), one from the
		// Pulse agent — must collapse into a single presentation host while
		// List() still returns both raw records. This is the only filtering
		// the function performs: it never drops a kind outright, it only
		// merges qualifying agent views.
		rr := NewRegistry(nil)
		rr.IngestResources([]Resource{
			{
				ID:       "agent-proxmox-tower",
				Type:     ResourceTypeAgent,
				Name:     "tower",
				Status:   StatusWarning,
				LastSeen: now.Add(-1 * time.Minute),
				Sources:  []DataSource{SourceProxmox},
				Identity: ResourceIdentity{Hostnames: []string{"tower"}},
				Proxmox: &ProxmoxData{
					NodeName:    "tower",
					ClusterName: "homelab",
				},
			},
			{
				ID:       "agent-runtime-tower",
				Type:     ResourceTypeAgent,
				Name:     "tower",
				Status:   StatusOnline,
				LastSeen: now,
				Sources:  []DataSource{SourceAgent},
				Identity: ResourceIdentity{
					MachineID: "agent-machine-tower",
					Hostnames: []string{"tower"},
				},
				Agent: &AgentData{
					AgentID:  "agent-machine-tower",
					Hostname: "tower",
					OSName:   "Proxmox VE",
				},
			},
			// A non-agent resource rides along to prove the coalesce only
			// touches host views — the VM count is unchanged.
			{ID: "vm-bystander", Type: ResourceTypeVM, Name: "bystander-vm", Status: StatusOnline, LastSeen: now},
		})

		raw := rr.List()
		if len(raw) != 3 {
			t.Fatalf("List() should still expose all 3 raw records, got %d", len(raw))
		}

		got := rr.ListForPresentation()
		if len(got) != 2 {
			t.Fatalf("expected split agent host views to coalesce into 1 (plus the VM = 2 total), got %d: %+v", len(got), got)
		}

		// The merged host must carry both agent and proxmox facets, and the
		// agent-source record wins as primary (its ID is the canonical one
		// the metrics API writes under).
		var mergedHost *Resource
		var vmCount int
		for i := range got {
			if CanonicalResourceType(got[i].Type) == ResourceTypeAgent {
				mergedHost = &got[i]
			}
			if CanonicalResourceType(got[i].Type) == ResourceTypeVM {
				vmCount++
			}
		}
		if mergedHost == nil {
			t.Fatalf("expected a coalesced agent host in output, got %+v", got)
		}
		if vmCount != 1 {
			t.Fatalf("expected the bystander VM to survive coalesce untouched, got %d", vmCount)
		}
		if mergedHost.ID != "agent-runtime-tower" {
			t.Fatalf("expected agent-source record to win as primary, got id=%q", mergedHost.ID)
		}
		if mergedHost.Agent == nil || mergedHost.Proxmox == nil {
			t.Fatalf("expected merged host to carry both agent and proxmox facets, got agent=%+v proxmox=%+v", mergedHost.Agent, mergedHost.Proxmox)
		}
		if !containsDataSource(mergedHost.Sources, SourceAgent) || !containsDataSource(mergedHost.Sources, SourceProxmox) {
			t.Fatalf("expected merged host sources to include agent+proxmox, got %+v", mergedHost.Sources)
		}
	})

	t.Run("ListForPresentation/ExclusionBlocksHostCoalesce", func(t *testing.T) {
		// The closure passed to CoalescePresentationHostResourcesWithExclusions
		// consults rr.exclusions: if the two candidate IDs are listed there,
		// the merge is skipped and both host views survive in the output.
		// We seed the exclusion in-package (the same way loadOverrides
		// populates it from the store) and assert the otherwise-coalescing
		// pair stays split.
		rr := NewRegistry(nil)
		rr.IngestResources([]Resource{
			{
				ID:       "agent-proxmox-pve1",
				Type:     ResourceTypeAgent,
				Name:     "pve1",
				Status:   StatusOnline,
				LastSeen: now,
				Sources:  []DataSource{SourceProxmox},
				Identity: ResourceIdentity{Hostnames: []string{"pve1"}},
				Proxmox:  &ProxmoxData{NodeName: "pve1", ClusterName: "homelab"},
			},
			{
				ID:       "agent-runtime-pve1",
				Type:     ResourceTypeAgent,
				Name:     "pve1",
				Status:   StatusOnline,
				LastSeen: now,
				Sources:  []DataSource{SourceAgent},
				Identity: ResourceIdentity{Hostnames: []string{"pve1"}},
				Agent:    &AgentData{AgentID: "agent-machine-pve1", Hostname: "pve1"},
			},
		})
		// Sanity: without an exclusion they coalesce.
		if got := rr.ListForPresentation(); len(got) != 1 {
			t.Fatalf("precondition: expected pair to coalesce into 1, got %d", len(got))
		}
		// Register the exclusion between the two IDs and re-run; the
		// closure's lookup must now return true and block the merge.
		rr.exclusions[exclusionKey("agent-proxmox-pve1", "agent-runtime-pve1")] = struct{}{}
		got := rr.ListForPresentation()
		if len(got) != 2 {
			t.Fatalf("expected exclusion to keep both host views split, got %d: %+v", len(got), got)
		}
		// The defensive "blank ID" arm of the exclusion closure
		// (leftID=="" || rightID == "" -> return false) is only reachable
		// when the coalesce step actually compares two host views and one
		// has an empty canonical ID. Such a resource cannot enter via the
		// public Ingest* APIs (they reject empty IDs), so we insert it
		// in-package sharing a hostname with the real agent above; the
		// coalesce step then invokes the closure with the blank ID, the
		// blank-ID arm fires, and the lookup short-circuits without panic.
		rr.resources[""] = &Resource{
			ID:       "",
			Type:     ResourceTypeAgent,
			Name:     "pve1",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox},
			Identity: ResourceIdentity{Hostnames: []string{"pve1"}},
			Proxmox:  &ProxmoxData{NodeName: "pve1", ClusterName: "homelab"},
		}
		// Must not panic; the blank-ID arm short-circuits the exclusion lookup.
		_ = rr.ListForPresentation()
	})

	t.Run("ListForPresentation/ResultIsIndependentOfRegistryStorage", func(t *testing.T) {
		rr := NewRegistry(nil)
		rr.IngestResources([]Resource{
			{ID: "vm-independent", Type: ResourceTypeVM, Name: "original-name", Status: StatusOnline, LastSeen: now},
		})
		first := rr.ListForPresentation()
		if len(first) != 1 {
			t.Fatalf("expected 1 resource, got %d", len(first))
		}
		origName := first[0].Name
		// Mutate the returned slice in place — List() deep-clones each
		// resource and the coalesce step operates on those clones, so this
		// must not leak back into the registry's internal storage.
		first[0].Name = "MUTATED-NAME"
		first[0].Status = StatusOffline

		second := rr.ListForPresentation()
		if len(second) != 1 {
			t.Fatalf("expected 1 resource on re-call, got %d", len(second))
		}
		if second[0].Name != origName {
			t.Fatalf("mutation of returned slice leaked into registry: name=%q want %q", second[0].Name, origName)
		}
		if second[0].Status != StatusOnline {
			t.Fatalf("mutation of returned slice leaked into registry: status=%q want %q", second[0].Status, StatusOnline)
		}
	})

	// ---------------------------------------------------------------------------
	// MetricsTarget — covers (rr *ResourceRegistry) MetricsTarget and the
	// private metricsTargetForResourceLocked helper it delegates to.
	// ---------------------------------------------------------------------------

	t.Run("MetricsTarget/UnknownIDReturnsNil", func(t *testing.T) {
		rr := NewRegistry(nil)
		// An empty registry has no resources at all, so any ID misses.
		if got := rr.MetricsTarget("does-not-exist"); got != nil {
			t.Fatalf("expected nil MetricsTarget for unknown id on empty registry, got %+v", got)
		}
		// A populated registry must still return nil for an id that is
		// simply not present — the resource lookup is the second arm of
		// metricsTargetForResourceLocked.
		rr.IngestResources([]Resource{
			{ID: "vm-real", Type: ResourceTypeVM, Name: "real-vm", Status: StatusOnline, LastSeen: now},
		})
		if got := rr.MetricsTarget("also-does-not-exist"); got != nil {
			t.Fatalf("expected nil MetricsTarget for unknown id, got %+v", got)
		}
	})

	t.Run("MetricsTarget/ResolvedFromSourceMapping", func(t *testing.T) {
		// An AppContainer ingested via SourceDocker gets a host-scoped
		// docker source mapping, so BuildMetricsTarget succeeds and
		// MetricsTarget returns the resolved target rather than falling
		// back to the stored MetricsTarget field.
		rr := NewRegistry(nil)
		rr.IngestRecords(SourceDocker, []IngestRecord{{
			SourceID: "host-1/container/web-1",
			Resource: Resource{
				Type:     ResourceTypeAppContainer,
				Name:     "web",
				Status:   StatusOnline,
				LastSeen: now,
				Docker: &DockerData{
					HostSourceID:   "host-1",
					ContainerID:    "web-1",
					ContainerState: "running",
				},
			},
		}})

		resources := rr.ListByType(ResourceTypeAppContainer)
		if len(resources) != 1 {
			t.Fatalf("expected 1 seeded app-container, got %d", len(resources))
		}
		got := rr.MetricsTarget(resources[0].ID)
		if got == nil {
			t.Fatal("expected non-nil MetricsTarget resolved from source mapping")
		}
		// BuildMetricsTarget prefers the canonical ContainerID for
		// app-containers because that is the key the metrics writer uses.
		if got.ResourceType != "app-container" {
			t.Fatalf("ResourceType = %q, want %q", got.ResourceType, "app-container")
		}
		if got.ResourceID != "web-1" {
			t.Fatalf("ResourceID = %q, want %q", got.ResourceID, "web-1")
		}

		// Mutating the returned target must not mutate the registry's
		// in-flight computation (clone isolation on the success path).
		got.ResourceID = "MUTATED"
		again := rr.MetricsTarget(resources[0].ID)
		if again == nil || again.ResourceID != "web-1" {
			t.Fatalf("expected clone isolation on MetricsTarget success path, got %+v", again)
		}
	})

	t.Run("MetricsTarget/FallsBackToStoredMetricsTargetField", func(t *testing.T) {
		// When BuildMetricsTarget returns nil (no source mapping), the
		// function falls back to cloneMetricsTarget(resource.MetricsTarget).
		// We seed directly into rr.resources — the same in-package pattern
		// the sibling TestBuildMetricsTargetForRegistryFallsBackToStoredMetricsTarget
		// uses — so bySource stays empty for this id.
		rr := NewRegistry(nil)
		const resourceID = "app-stored-target"
		rr.resources[resourceID] = &Resource{
			ID:            resourceID,
			Type:          ResourceTypeAppContainer,
			Name:          "stored",
			Status:        StatusOnline,
			LastSeen:      now,
			MetricsTarget: &MetricsTarget{ResourceType: "dockerContainer", ResourceID: "nextcloud-web-1"},
		}

		got := rr.MetricsTarget(resourceID)
		if got == nil {
			t.Fatal("expected fallback MetricsTarget from stored field")
		}
		if got.ResourceType != "dockerContainer" || got.ResourceID != "nextcloud-web-1" {
			t.Fatalf("fallback MetricsTarget = %+v, want dockerContainer/nextcloud-web-1", got)
		}
	})

	t.Run("MetricsTarget/ExistsButHasNoTargetReturnsNil", func(t *testing.T) {
		// A resource that exists, has no source mapping (so BuildMetricsTarget
		// returns nil) AND has no stored MetricsTarget field must yield nil.
		// This covers the final `return cloneMetricsTarget(resource.MetricsTarget)`
		// arm with a nil stored field.
		rr := NewRegistry(nil)
		const resourceID = "orphan-no-target"
		rr.resources[resourceID] = &Resource{
			ID:       resourceID,
			Type:     ResourceTypeAppContainer,
			Name:     "orphan",
			Status:   StatusOnline,
			LastSeen: now,
			// MetricsTarget intentionally left nil.
		}
		if got := rr.MetricsTarget(resourceID); got != nil {
			t.Fatalf("expected nil MetricsTarget for resource with no source mapping and no stored target, got %+v", got)
		}
	})

	// ---------------------------------------------------------------------------
	// DockerContainers
	// ---------------------------------------------------------------------------

	t.Run("DockerContainers/EmptyRegistryReturnsEmpty", func(t *testing.T) {
		rr := NewRegistry(nil)
		got := rr.DockerContainers()
		if len(got) != 0 {
			t.Fatalf("expected 0 docker container views for empty registry, got %d", len(got))
		}
	})

	t.Run("DockerContainers/PopulatedReturnsViewsSortedByName", func(t *testing.T) {
		rr := NewRegistry(nil)
		rr.IngestResources([]Resource{
			{ID: "dc-zeta", Type: ResourceTypeAppContainer, Name: "zeta-svc", Status: StatusOnline, LastSeen: now, Docker: &DockerData{ContainerID: "cid-zeta"}},
			{ID: "dc-alpha", Type: ResourceTypeAppContainer, Name: "alpha-svc", Status: StatusOnline, LastSeen: now, Docker: &DockerData{ContainerID: "cid-alpha"}},
			{ID: "dc-mid", Type: ResourceTypeAppContainer, Name: "mid-svc", Status: StatusOnline, LastSeen: now, Docker: &DockerData{ContainerID: "cid-mid"}},
		})

		got := rr.DockerContainers()
		if len(got) != 3 {
			t.Fatalf("expected 3 docker container views, got %d", len(got))
		}
		// rebuildViews sorts cachedDockerContainers by canonical name.
		wantNames := []string{"alpha-svc", "mid-svc", "zeta-svc"}
		gotNames := make([]string, 0, len(got))
		for _, v := range got {
			gotNames = append(gotNames, v.Name())
		}
		assertStringSlice(t, gotNames, wantNames)

		// The view wraps the cloned resource, so its container id comes
		// straight from the Docker facet.
		if got[0].ContainerID() != "cid-alpha" {
			t.Fatalf("first view ContainerID = %q, want cid-alpha", got[0].ContainerID())
		}
		if got[0].ID() != "dc-alpha" {
			t.Fatalf("first view ID = %q, want dc-alpha", got[0].ID())
		}
	})

	t.Run("DockerContainers/ReturnsEmptyWhenOnlyOtherKindSeeded", func(t *testing.T) {
		// A registry seeded exclusively with K8sNode resources must return
		// zero docker container views — never the wrong kind.
		rr := NewRegistry(nil)
		rr.IngestResources([]Resource{
			{ID: "k8s-node-only-1", Type: ResourceTypeK8sNode, Name: "node-a", Status: StatusOnline, LastSeen: now, Kubernetes: &K8sData{NodeName: "node-a"}},
			{ID: "k8s-node-only-2", Type: ResourceTypeK8sNode, Name: "node-b", Status: StatusOnline, LastSeen: now, Kubernetes: &K8sData{NodeName: "node-b"}},
		})
		if got := rr.DockerContainers(); len(got) != 0 {
			t.Fatalf("expected 0 docker container views when only K8sNodes seeded, got %d: %+v", len(got), got)
		}
		// Sanity: the k8s node cache DID build, proving the rebuild ran
		// and the docker cache stayed empty by kind, not by accident.
		if got := rr.K8sNodes(); len(got) != 2 {
			t.Fatalf("expected 2 k8s node views as a control, got %d", len(got))
		}
	})

	// ---------------------------------------------------------------------------
	// K8sNodes
	// ---------------------------------------------------------------------------

	t.Run("K8sNodes/EmptyRegistryReturnsEmpty", func(t *testing.T) {
		rr := NewRegistry(nil)
		got := rr.K8sNodes()
		if len(got) != 0 {
			t.Fatalf("expected 0 k8s node views for empty registry, got %d", len(got))
		}
	})

	t.Run("K8sNodes/PopulatedReturnsViewsSortedByName", func(t *testing.T) {
		rr := NewRegistry(nil)
		rr.IngestResources([]Resource{
			{ID: "k8s-n-zeta", Type: ResourceTypeK8sNode, Name: "zeta-node", Status: StatusOnline, LastSeen: now, Kubernetes: &K8sData{NodeUID: "uid-zeta", NodeName: "zeta-node", ClusterName: "cluster-a"}},
			{ID: "k8s-n-alpha", Type: ResourceTypeK8sNode, Name: "alpha-node", Status: StatusOnline, LastSeen: now, Kubernetes: &K8sData{NodeUID: "uid-alpha", NodeName: "alpha-node", ClusterName: "cluster-a"}},
			{ID: "k8s-n-mid", Type: ResourceTypeK8sNode, Name: "mid-node", Status: StatusOnline, LastSeen: now, Kubernetes: &K8sData{NodeUID: "uid-mid", NodeName: "mid-node", ClusterName: "cluster-b"}},
		})

		got := rr.K8sNodes()
		if len(got) != 3 {
			t.Fatalf("expected 3 k8s node views, got %d", len(got))
		}
		// rebuildViews sorts cachedK8sNodes by canonical name.
		wantNames := []string{"alpha-node", "mid-node", "zeta-node"}
		gotNames := make([]string, 0, len(got))
		for _, v := range got {
			gotNames = append(gotNames, v.Name())
		}
		assertStringSlice(t, gotNames, wantNames)

		// View accessors surface the underlying K8sData fields.
		if got[0].NodeUID() != "uid-alpha" {
			t.Fatalf("first view NodeUID = %q, want uid-alpha", got[0].NodeUID())
		}
		if got[0].NodeName() != "alpha-node" {
			t.Fatalf("first view NodeName = %q, want alpha-node", got[0].NodeName())
		}
		if got[0].ClusterName() != "cluster-a" {
			t.Fatalf("first view ClusterName = %q, want cluster-a", got[0].ClusterName())
		}
		if got[0].ID() != "k8s-n-alpha" {
			t.Fatalf("first view ID = %q, want k8s-n-alpha", got[0].ID())
		}
	})

	t.Run("K8sNodes/ReturnsEmptyWhenOnlyOtherKindSeeded", func(t *testing.T) {
		// A registry seeded exclusively with AppContainer resources must
		// return zero k8s node views — never the wrong kind.
		rr := NewRegistry(nil)
		rr.IngestResources([]Resource{
			{ID: "app-only-1", Type: ResourceTypeAppContainer, Name: "app-a", Status: StatusOnline, LastSeen: now, Docker: &DockerData{ContainerID: "cid-a"}},
			{ID: "app-only-2", Type: ResourceTypeAppContainer, Name: "app-b", Status: StatusOnline, LastSeen: now, Docker: &DockerData{ContainerID: "cid-b"}},
		})
		if got := rr.K8sNodes(); len(got) != 0 {
			t.Fatalf("expected 0 k8s node views when only AppContainers seeded, got %d: %+v", len(got), got)
		}
		// Control: the docker cache DID build.
		if got := rr.DockerContainers(); len(got) != 2 {
			t.Fatalf("expected 2 docker container views as a control, got %d", len(got))
		}
	})
}
