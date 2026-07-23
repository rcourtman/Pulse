package monitoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type resourceOnlyStore struct {
	resources []unifiedresources.Resource
}

func (s *resourceOnlyStore) ShouldSkipAPIPolling(string) bool { return false }

func (s *resourceOnlyStore) GetPollingRecommendations() map[string]float64 { return nil }

func (s *resourceOnlyStore) GetAll() []unifiedresources.Resource {
	out := make([]unifiedresources.Resource, len(s.resources))
	copy(out, s.resources)
	return out
}

func (s *resourceOnlyStore) PopulateFromSnapshot(models.StateSnapshot) {}

type freshnessResourceStore struct {
	resourceOnlyStore
	freshness time.Time
}

func (s *freshnessResourceStore) UnifiedResourceFreshness() time.Time {
	return s.freshness
}

type testSupplementalInventoryReadinessProvider struct {
	source  unifiedresources.DataSource
	records []unifiedresources.IngestRecord
	settled bool
	readyAt time.Time
}

func (p *testSupplementalInventoryReadinessProvider) SupplementalRecords(_ *Monitor, _ string) []unifiedresources.IngestRecord {
	out := make([]unifiedresources.IngestRecord, len(p.records))
	copy(out, p.records)
	return out
}

func (p *testSupplementalInventoryReadinessProvider) SnapshotOwnedSources() []unifiedresources.DataSource {
	if p.source == "" {
		return nil
	}
	return []unifiedresources.DataSource{p.source}
}

func (p *testSupplementalInventoryReadinessProvider) SupplementalInventoryReadyAt(*Monitor, string) (time.Time, bool) {
	return p.readyAt, p.settled
}

type unifiedResourceGetter interface {
	GetAll() []unifiedresources.Resource
}

func hasUnifiedResource(resources []unifiedresources.Resource, id string) bool {
	for _, resource := range resources {
		if resource.ID == id {
			return true
		}
	}
	return false
}

func hasUnifiedResourceName(resources []unifiedresources.Resource, name string) bool {
	for _, resource := range resources {
		if resource.Name == name {
			return true
		}
	}
	return false
}

func TestConvertResourcesForBroadcastCoalescesSplitHostResources(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 30, 0, 0, time.UTC)
	resources := []unifiedresources.Resource{
		{
			ID:       "agent-proxmox-delly",
			Type:     unifiedresources.ResourceTypeAgent,
			Name:     "delly",
			Status:   unifiedresources.StatusOnline,
			LastSeen: now.Add(-1 * time.Second),
			Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"delly"}},
			Proxmox: &unifiedresources.ProxmoxData{
				NodeName:    "delly",
				ClusterName: "homelab",
				PVEVersion:  "9.1.9",
			},
		},
		{
			ID:       "agent-runtime-delly",
			Type:     unifiedresources.ResourceTypeAgent,
			Name:     "delly",
			Status:   unifiedresources.StatusOnline,
			LastSeen: now,
			Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
			Identity: unifiedresources.ResourceIdentity{
				MachineID: "agent-machine-delly",
				Hostnames: []string{
					"delly",
				},
			},
			Agent: &unifiedresources.AgentData{
				AgentID:  "agent-machine-delly",
				Hostname: "delly",
				OSName:   "Proxmox VE",
			},
			Metrics: &unifiedresources.ResourceMetrics{
				DiskRead: &unifiedresources.MetricValue{
					Value:  4096,
					Unit:   "bytes/s",
					Source: unifiedresources.SourceAgent,
				},
				DiskWrite: &unifiedresources.MetricValue{
					Value:  8192,
					Unit:   "bytes/s",
					Source: unifiedresources.SourceAgent,
				},
			},
		},
	}

	frontend := convertResourcesForBroadcast(resources)
	if len(frontend) != 1 {
		t.Fatalf("expected split host resources to coalesce into 1 frontend resource, got %d: %#v", len(frontend), frontend)
	}

	resource := frontend[0]
	if resource.ID != "agent-runtime-delly" {
		t.Fatalf("expected agent-backed resource ID, got %q", resource.ID)
	}
	if resource.SourceType != "hybrid" {
		t.Fatalf("expected hybrid source type, got %q", resource.SourceType)
	}
	if resource.PlatformType != "proxmox-pve" {
		t.Fatalf("expected proxmox-pve platform type, got %q", resource.PlatformType)
	}
	sourceSet := make(map[string]bool, len(resource.Sources))
	for _, source := range resource.Sources {
		sourceSet[source] = true
	}
	if !sourceSet["agent"] || !sourceSet["proxmox"] {
		t.Fatalf("expected agent and proxmox sources, got %#v", resource.Sources)
	}
	if len(resource.Proxmox) == 0 {
		t.Fatal("expected merged Proxmox facet")
	}
	if len(resource.Agent) == 0 {
		t.Fatal("expected merged agent facet")
	}
	if resource.DiskIO == nil {
		t.Fatal("expected aggregate disk I/O rates in broadcast resource")
	}
	if resource.DiskIO.ReadRate != 4096 || resource.DiskIO.WriteRate != 8192 {
		t.Fatalf("unexpected aggregate disk I/O rates: %+v", resource.DiskIO)
	}
}

func TestConvertResourcesForBroadcastAttachesResolvedStorageMetricsTarget(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 45, 0, 0, time.UTC)
	agentID := "de6d3fee-2595-6c2b-6b08-43db6b0ab427"
	storageID := "storage-2642e9fca16b2f87"
	parentID := agentID
	total := int64(24_000_000_000_000)
	used := int64(16_900_000_000_000)

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources([]unifiedresources.Resource{
		{
			ID:       agentID,
			Type:     unifiedresources.ResourceTypeAgent,
			Name:     "Tower",
			Status:   unifiedresources.StatusOnline,
			LastSeen: now,
			Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
			Agent: &unifiedresources.AgentData{
				AgentID:  agentID,
				Hostname: "Tower",
			},
		},
		{
			ID:       storageID,
			Type:     unifiedresources.ResourceTypeStorage,
			Name:     "Tower Array",
			Status:   unifiedresources.StatusWarning,
			LastSeen: now,
			Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
			ParentID: &parentID,
			Metrics: &unifiedresources.ResourceMetrics{
				Disk: &unifiedresources.MetricValue{
					Used:    &used,
					Total:   &total,
					Percent: 70.4,
					Unit:    "bytes",
					Source:  unifiedresources.SourceAgent,
				},
			},
			Storage: &unifiedresources.StorageMeta{
				Type:     "unraid-array",
				Platform: "unraid",
				Topology: "array",
			},
		},
	})
	adapter := unifiedresources.NewMonitorAdapter(registry)

	frontend := convertResourcesForBroadcast(adapter.GetAll(), adapter)

	var storagePayload struct {
		ID            string `json:"id"`
		MetricsTarget struct {
			ResourceType string `json:"resourceType"`
			ResourceID   string `json:"resourceId"`
		} `json:"metricsTarget"`
	}
	for _, resource := range frontend {
		if resource.ID != storageID {
			continue
		}
		encoded, err := json.Marshal(resource)
		if err != nil {
			t.Fatalf("marshal storage frontend resource: %v", err)
		}
		if err := json.Unmarshal(encoded, &storagePayload); err != nil {
			t.Fatalf("unmarshal storage frontend resource: %v", err)
		}
		break
	}

	if storagePayload.ID != storageID {
		t.Fatalf("expected storage resource %q in frontend payload, got %q", storageID, storagePayload.ID)
	}
	if storagePayload.MetricsTarget.ResourceType != "storage" ||
		storagePayload.MetricsTarget.ResourceID != agentID+"/storage:unraid-array" {
		t.Fatalf("metrics target = %+v", storagePayload.MetricsTarget)
	}
}

func TestConvertResourcesForBroadcastKeepsLinkedGuestCPUAndHistoryTargetAligned(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name          string
		resourceType  unifiedresources.ResourceType
		technology    string
		targetType    string
		containerType string
	}{
		{
			name:          "lxc",
			resourceType:  unifiedresources.ResourceTypeSystemContainer,
			technology:    "lxc",
			targetType:    "system-container",
			containerType: "lxc",
		},
		{
			name:         "qemu",
			resourceType: unifiedresources.ResourceTypeVM,
			technology:   "qemu",
			targetType:   "vm",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			guestID := string(tc.resourceType) + "-guest-301"
			guestSourceID := "cluster-a-node-1-" + tc.name + "-301"
			const agentID = "agent-inside-guest-301"
			store := unifiedresources.NewMemoryStore()
			if err := store.AddLink(unifiedresources.ResourceLink{
				ResourceA: guestID,
				ResourceB: agentID,
				PrimaryID: agentID,
			}); err != nil {
				t.Fatalf("add link: %v", err)
			}
			registry := unifiedresources.NewRegistry(store)
			registry.IngestResources([]unifiedresources.Resource{
				{
					ID:         guestID,
					Type:       tc.resourceType,
					Technology: tc.technology,
					Name:       tc.name + "-301",
					Status:     unifiedresources.StatusOnline,
					LastSeen:   now,
					Sources:    []unifiedresources.DataSource{unifiedresources.SourceProxmox},
					SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
						unifiedresources.SourceProxmox: {Status: "online", LastSeen: now},
					},
					Proxmox: &unifiedresources.ProxmoxData{
						SourceID:      guestSourceID,
						VMID:          301,
						ContainerType: tc.containerType,
						CPUs:          8,
					},
					Metrics: &unifiedresources.ResourceMetrics{
						CPU: &unifiedresources.MetricValue{
							Value:   0.58,
							Percent: 0.58,
							Unit:    "percent",
							Source:  unifiedresources.SourceProxmox,
						},
					},
				},
				{
					ID:       agentID,
					Type:     unifiedresources.ResourceTypeAgent,
					Name:     tc.name + "-301",
					Status:   unifiedresources.StatusOnline,
					LastSeen: now,
					Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
					SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
						unifiedresources.SourceAgent: {Status: "online", LastSeen: now},
					},
					Agent: &unifiedresources.AgentData{AgentID: agentID, CPUCount: 8},
					Metrics: &unifiedresources.ResourceMetrics{
						CPU: &unifiedresources.MetricValue{
							Value:   94,
							Percent: 94,
							Unit:    "percent",
							Source:  unifiedresources.SourceAgent,
						},
					},
				},
			})

			adapter := unifiedresources.NewMonitorAdapter(registry)
			frontend := convertResourcesForBroadcast(adapter.GetAll(), adapter)
			if len(frontend) != 1 {
				t.Fatalf("broadcast resources = %d, want one merged guest: %+v", len(frontend), frontend)
			}
			got := frontend[0]
			if got.ID != guestID || got.Type != string(tc.resourceType) || got.Technology != tc.technology {
				t.Fatalf("broadcast resource = %s/%s/%s, want %s/%s/%s", got.ID, got.Type, got.Technology, guestID, tc.resourceType, tc.technology)
			}
			if got.CPU == nil || got.CPU.Current != 0.58 {
				t.Fatalf("broadcast CPU = %+v, want proxmox 0.58", got.CPU)
			}

			var target unifiedresources.MetricsTarget
			if err := json.Unmarshal(got.MetricsTarget, &target); err != nil {
				t.Fatalf("decode metrics target: %v", err)
			}
			if target.ResourceType != tc.targetType || target.ResourceID != guestSourceID {
				t.Fatalf("broadcast metrics target = %+v, want %s/%s", target, tc.targetType, guestSourceID)
			}
		})
	}
}

func hasFrontendResourceName(resources []models.ResourceFrontend, name string) bool {
	for _, resource := range resources {
		if resource.Name == name {
			return true
		}
	}
	return false
}

// hasFrontendResourceNameForHostType returns true only when a docker-host or
// proxmox-node-shaped resource carries the literal Name. Docker Swarm nodes
// added in 89abed099 legitimately publish the swarm-node hostname (e.g.
// "edge-apps-01") as their Name because that is how Docker Swarm identifies
// members; the canonical legacy-label rejection rules below apply only to
// the host-type resources that previously surfaced the lowercase-hyphenated
// host hostname instead of the canonical DisplayName.
func hasFrontendResourceNameForHostType(resources []models.ResourceFrontend, name string) bool {
	for _, resource := range resources {
		if resource.Name != name {
			continue
		}
		switch resource.Type {
		case "docker-host", "agent", "node":
			return true
		}
	}
	return false
}

func TestMonitorGetUnifiedReadStateOrSnapshotUsesStoreResourcesWithoutSnapshotFallback(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	store := &resourceOnlyStore{
		resources: []unifiedresources.Resource{
			{
				ID:       "agent-store-1",
				Type:     unifiedresources.ResourceTypeAgent,
				Name:     "agent-store-1",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
				Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"agent-store-1"}},
			},
		},
	}

	m := &Monitor{
		state:         models.NewState(),
		resourceStore: store,
	}

	readState := m.GetUnifiedReadStateOrSnapshot()
	getter, ok := readState.(unifiedResourceGetter)
	if !ok {
		t.Fatalf("expected read-state adapter with GetAll, got %T", readState)
	}

	resources := getter.GetAll()
	if !hasUnifiedResource(resources, "agent-store-1") {
		t.Fatalf("expected store-backed unified resource in read-state, got %#v", resources)
	}
}

func TestMonitorGetUnifiedReadStateOrSnapshotFallsBackToSnapshotWhenStoreEmpty(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 30, 0, 0, time.UTC)
	state := models.NewState()
	state.UpsertHost(models.Host{
		ID:       "host-snapshot-1",
		Hostname: "host-snapshot-1",
		Status:   "online",
		LastSeen: now,
	})

	m := &Monitor{
		state:         state,
		resourceStore: &resourceOnlyStore{},
	}

	readState := m.GetUnifiedReadStateOrSnapshot()
	getter, ok := readState.(unifiedResourceGetter)
	if !ok {
		t.Fatalf("expected read-state adapter with GetAll, got %T", readState)
	}

	resources := getter.GetAll()
	if !hasUnifiedResourceName(resources, "host-snapshot-1") {
		t.Fatalf("expected snapshot-backed unified resource in read-state, got %#v", resources)
	}
}

func TestMonitorUnifiedResourceSnapshotFallsBackToSnapshotWhenStoreEmpty(t *testing.T) {
	now := time.Date(2026, 3, 6, 13, 0, 0, 0, time.UTC)
	state := models.NewState()
	state.UpsertHost(models.Host{
		ID:       "host-snapshot-2",
		Hostname: "host-snapshot-2",
		Status:   "online",
		LastSeen: now,
	})

	m := &Monitor{
		state:         state,
		resourceStore: &resourceOnlyStore{},
	}

	resources, freshness := m.UnifiedResourceSnapshot()
	if !hasUnifiedResourceName(resources, "host-snapshot-2") {
		t.Fatalf("expected snapshot-backed unified resource, got %#v", resources)
	}
	if freshness.IsZero() {
		t.Fatal("expected non-zero freshness from snapshot fallback")
	}
}

func TestMonitorUnifiedResourceSnapshotIncludesRecentStandaloneHostContinuity(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := config.NewHostContinuityStore(t.TempDir(), nil)
	if err := store.Upsert(config.HostContinuityEntry{
		HostID:       "host-1",
		ReportHostID: "machine-1",
		Hostname:     "host-1.local",
		DisplayName:  "Host One",
		MachineID:    "machine-1",
		AgentVersion: "6.0.0-rc.5",
		LastSeen:     now,
	}); err != nil {
		t.Fatalf("Upsert continuity: %v", err)
	}

	m := &Monitor{
		state:               models.NewState(),
		resourceStore:       unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		hostContinuityStore: store,
	}

	resources, freshness := m.UnifiedResourceSnapshot()
	if !hasUnifiedResourceName(resources, "Host One") {
		t.Fatalf("expected continuity-backed resource in unified snapshot, got %#v", resources)
	}
	if freshness.IsZero() {
		t.Fatal("expected continuity-backed snapshot to carry non-zero freshness")
	}
}

func TestMonitorUnifiedResourceSnapshotPrefersStoreFreshness(t *testing.T) {
	state := models.NewState()
	state.UpsertHost(models.Host{
		ID:       "host-state-1",
		Hostname: "host-state-1",
		Status:   "online",
		LastSeen: time.Date(2026, 3, 6, 13, 0, 0, 0, time.UTC),
	})

	storeFreshness := time.Date(2026, 3, 7, 9, 30, 0, 0, time.UTC)
	m := &Monitor{
		state: state,
		resourceStore: &freshnessResourceStore{
			resourceOnlyStore: resourceOnlyStore{
				resources: []unifiedresources.Resource{
					{
						ID:       "agent-store-1",
						Type:     unifiedresources.ResourceTypeAgent,
						Name:     "agent-store-1",
						Status:   unifiedresources.StatusOnline,
						LastSeen: storeFreshness,
						Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
						Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"agent-store-1"}},
					},
				},
			},
			freshness: storeFreshness,
		},
	}

	_, freshness := m.UnifiedResourceSnapshot()
	if !freshness.Equal(storeFreshness) {
		t.Fatalf("expected store freshness %v, got %v", storeFreshness, freshness)
	}
}

func TestMonitorGetUnifiedReadStateOrSnapshotUsesCanonicalMockUnifiedResources(t *testing.T) {
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })

	graph := mock.CurrentFixtureGraph()
	legacyName := ""
	if len(graph.State.VMs) > 0 {
		legacyName = graph.State.VMs[0].Name
	} else if len(graph.State.Containers) > 0 {
		legacyName = graph.State.Containers[0].Name
	}
	if legacyName == "" {
		t.Fatal("expected canonical mock graph to include at least one legacy resource")
	}

	store := &resourceOnlyStore{
		resources: []unifiedresources.Resource{
			{
				ID:       "store-only-resource",
				Type:     unifiedresources.ResourceTypeAgent,
				Name:     "store-only-resource",
				Status:   unifiedresources.StatusOnline,
				LastSeen: time.Date(2026, 3, 6, 14, 0, 0, 0, time.UTC),
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
				Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"store-only-resource"}},
			},
		},
	}

	m := &Monitor{
		state:         models.NewState(),
		resourceStore: store,
		alertManager:  alerts.NewManager(),
	}
	defer m.alertManager.Stop()

	readState := m.GetUnifiedReadStateOrSnapshot()
	getter, ok := readState.(unifiedResourceGetter)
	if !ok {
		t.Fatalf("expected read-state adapter with GetAll, got %T", readState)
	}

	resources := getter.GetAll()
	if hasUnifiedResource(resources, "store-only-resource") {
		t.Fatal("expected mock-mode read-state to ignore live resource store data")
	}
	if !hasUnifiedResourceName(resources, "truenas-main") {
		t.Fatalf("expected mock-mode read-state to include TrueNAS mock resources, got %#v", resources)
	}
	if !hasUnifiedResourceName(resources, "esxi-01.lab.local") {
		t.Fatalf("expected mock-mode read-state to include VMware mock resources, got %#v", resources)
	}
	if !hasUnifiedResourceName(resources, legacyName) {
		t.Fatalf("expected mock-mode read-state to include legacy mock resource %q, got %#v", legacyName, resources)
	}
}

func TestMonitorBuildBroadcastFrontendStateUsesCanonicalMockUnifiedResources(t *testing.T) {
	previousEnabled := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
	t.Cleanup(func() {
		mustSetMockEnabled(t, false)
		mock.SetMockConfig(previousConfig)
		if previousEnabled {
			mustSetMockEnabled(t, true)
		}
	})

	stableConfig := mock.DefaultConfig
	stableConfig.UpdateInterval = 5 * time.Minute
	mustSetMockEnabled(t, false)
	mock.SetMockConfig(stableConfig)
	mustSetMockEnabled(t, true)

	graph := mock.CurrentFixtureGraph()
	legacyName := ""
	if len(graph.State.VMs) > 0 {
		legacyName = graph.State.VMs[0].Name
	} else if len(graph.State.Containers) > 0 {
		legacyName = graph.State.Containers[0].Name
	}
	if legacyName == "" {
		t.Fatal("expected canonical mock graph to include at least one legacy resource")
	}

	store := &resourceOnlyStore{
		resources: []unifiedresources.Resource{
			{
				ID:       "store-only-resource",
				Type:     unifiedresources.ResourceTypeAgent,
				Name:     "store-only-resource",
				Status:   unifiedresources.StatusOnline,
				LastSeen: time.Date(2026, 3, 6, 14, 0, 0, 0, time.UTC),
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
				Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"store-only-resource"}},
			},
		},
	}

	m := &Monitor{
		state:         models.NewState(),
		resourceStore: store,
		alertManager:  alerts.NewManager(),
	}
	defer m.alertManager.Stop()

	expectedResources, freshness := m.UnifiedResourceSnapshot()
	frontend := m.BuildBroadcastFrontendState()

	if hasFrontendResourceName(frontend.Resources, "store-only-resource") {
		t.Fatal("expected mock-mode broadcast state to ignore live resource store data")
	}
	if !hasFrontendResourceName(frontend.Resources, "truenas-main") {
		t.Fatalf("expected mock-mode broadcast state to include TrueNAS mock resources, got %#v", frontend.Resources)
	}
	if hasFrontendResourceNameForHostType(frontend.Resources, "pve1") {
		t.Fatalf("expected mock-mode broadcast state to stop publishing legacy proxmox node labels, got %#v", frontend.Resources)
	}
	if hasFrontendResourceNameForHostType(frontend.Resources, "edge-apps-01") {
		t.Fatalf("expected mock-mode broadcast state to stop publishing legacy docker host labels, got %#v", frontend.Resources)
	}
	if !hasFrontendResourceName(frontend.Resources, "esxi-01.lab.local") {
		t.Fatalf("expected mock-mode broadcast state to include VMware mock resources, got %#v", frontend.Resources)
	}
	if !hasFrontendResourceName(frontend.Resources, "West Production A") {
		t.Fatalf("expected mock-mode broadcast state to include canonical proxmox host label, got %#v", frontend.Resources)
	}
	if !hasFrontendResourceName(frontend.Resources, "Edge Apps 01") {
		t.Fatalf("expected mock-mode broadcast state to include canonical docker host label, got %#v", frontend.Resources)
	}
	if !hasFrontendResourceName(frontend.Resources, legacyName) {
		t.Fatalf("expected mock-mode broadcast state to include legacy mock resource %q, got %#v", legacyName, frontend.Resources)
	}
	// The broadcast path coalesces resources that share a canonical host
	// merge key (for example, a Kubernetes node that the registry already
	// linked to its host agent), reshapes identity through
	// `attachBroadcastMetricsTargets`, and runs a second coalesce pass
	// inside convertResourcesForBroadcast. Larger fixture sizes
	// legitimately produce more merge candidates and the two coalesce
	// passes can drop a handful of additional rows beyond what a single
	// coalesce on the raw snapshot would; this test's purpose is to
	// confirm the broadcast publishes canonical mock unified resources
	// (not legacy node/docker labels) at SMB-fixture density, not to
	// pin an exact merge count. Require the broadcast count to fall
	// within a 5% tolerance of the convert-pipeline count so future
	// fixture bumps stay green as long as the merge contract is honored.
	expectedBroadcastCount := len(convertResourcesForBroadcast(expectedResources))
	tolerance := expectedBroadcastCount / 20
	if tolerance < 5 {
		tolerance = 5
	}
	drift := expectedBroadcastCount - len(frontend.Resources)
	if drift < 0 {
		drift = -drift
	}
	if drift > tolerance {
		t.Fatalf("expected mock-mode broadcast state within ±%d of canonical unified resource count %d, got %d (raw snapshot count %d)", tolerance, expectedBroadcastCount, len(frontend.Resources), len(expectedResources))
	}
	if freshness.IsZero() {
		t.Fatal("expected canonical mock unified snapshot freshness")
	}
	if frontend.LastUpdate != freshness.UnixMilli() {
		t.Fatalf("expected broadcast lastUpdate %d from canonical mock freshness, got %d", freshness.UnixMilli(), frontend.LastUpdate)
	}
	for _, resource := range frontend.Resources {
		if resource.Type == "node" {
			t.Fatalf("expected canonical broadcast resource types only, got legacy node resource %#v", resource)
		}
	}
}

func TestConvertResourcesForBroadcastIncludesCanonicalHealthContext(t *testing.T) {
	resources := []unifiedresources.Resource{
		{
			ID:               "agent:tower",
			Type:             unifiedresources.ResourceTypeAgent,
			Name:             "Tower",
			Status:           unifiedresources.StatusWarning,
			LastSeen:         time.Date(2026, 5, 8, 11, 30, 0, 0, time.UTC),
			Sources:          []unifiedresources.DataSource{unifiedresources.SourceAgent},
			IncidentCount:    1,
			IncidentSeverity: storagehealth.RiskWarning,
			IncidentSummary:  "Unraid array is running without parity protection",
			Agent: &unifiedresources.AgentData{
				AgentID:               "agent-1",
				Hostname:              "tower",
				OSName:                "Unraid",
				StoragePostureSummary: "Unraid array is running without parity protection",
				StorageRisk: &unifiedresources.StorageRisk{
					Level: storagehealth.RiskWarning,
					Reasons: []unifiedresources.StorageRiskReason{{
						Code:     "unraid_no_parity",
						Severity: storagehealth.RiskWarning,
						Summary:  "Unraid array is running without parity protection",
					}},
				},
				Unraid: &unifiedresources.HostUnraidMeta{
					ArrayStarted:   true,
					ArrayState:     "STARTED",
					PostureSummary: "Unraid array is running without parity protection",
					Risk: &unifiedresources.StorageRisk{
						Level: storagehealth.RiskWarning,
						Reasons: []unifiedresources.StorageRiskReason{{
							Code:     "unraid_no_parity",
							Severity: storagehealth.RiskWarning,
							Summary:  "Unraid array is running without parity protection",
						}},
					},
				},
			},
			Docker: &unifiedresources.DockerData{
				AgentID:      "agent-1",
				HostSourceID: "agent-1",
				Hostname:     "tower",
				Runtime:      "docker",
			},
		},
		{
			ID:       "agent:tower/storage:unraid-array",
			Type:     unifiedresources.ResourceTypeStorage,
			Name:     "Tower Array",
			Status:   unifiedresources.StatusWarning,
			LastSeen: time.Date(2026, 5, 8, 11, 30, 0, 0, time.UTC),
			Sources:  []unifiedresources.DataSource{unifiedresources.SourceAgent},
			Storage: &unifiedresources.StorageMeta{
				Type:           "unraid-array",
				Platform:       "unraid",
				Topology:       "array",
				RiskSummary:    "Unraid array is running without parity protection",
				PostureSummary: "Unraid array is running without parity protection",
				SyncAction:     "check",
				Risk: &unifiedresources.StorageRisk{
					Level: storagehealth.RiskWarning,
					Reasons: []unifiedresources.StorageRiskReason{{
						Code:     "unraid_no_parity",
						Severity: storagehealth.RiskWarning,
						Summary:  "Unraid array is running without parity protection",
					}},
				},
			},
		},
	}

	frontend := convertResourcesForBroadcast(resources)
	if len(frontend) != 2 {
		t.Fatalf("expected 2 frontend resources, got %d", len(frontend))
	}

	var agentPayload struct {
		IncidentSummary string `json:"incidentSummary"`
		Canonical       struct {
			PrimaryID string `json:"primaryId"`
		} `json:"canonicalIdentity"`
		DiscoveryTarget struct {
			ResourceType string `json:"resourceType"`
			AgentID      string `json:"agentId"`
		} `json:"discoveryTarget"`
		MetricsTarget struct {
			ResourceType string `json:"resourceType"`
			ResourceID   string `json:"resourceId"`
		} `json:"metricsTarget"`
		Agent struct {
			StoragePostureSummary string `json:"storagePostureSummary"`
			Unraid                struct {
				PostureSummary string `json:"postureSummary"`
				Risk           struct {
					Reasons []struct {
						Code    string `json:"code"`
						Summary string `json:"summary"`
					} `json:"reasons"`
				} `json:"risk"`
			} `json:"unraid"`
		} `json:"agent"`
	}
	encoded, err := json.Marshal(frontend[0])
	if err != nil {
		t.Fatalf("marshal agent frontend resource: %v", err)
	}
	if err := json.Unmarshal(encoded, &agentPayload); err != nil {
		t.Fatalf("unmarshal agent frontend resource: %v", err)
	}
	if agentPayload.IncidentSummary != "Unraid array is running without parity protection" {
		t.Fatalf("incident summary = %q", agentPayload.IncidentSummary)
	}
	if agentPayload.Canonical.PrimaryID != "agent:agent-1" {
		t.Fatalf("canonical primary ID = %q", agentPayload.Canonical.PrimaryID)
	}
	if agentPayload.DiscoveryTarget.ResourceType != "agent" || agentPayload.DiscoveryTarget.AgentID != "agent-1" {
		t.Fatalf("discovery target = %+v", agentPayload.DiscoveryTarget)
	}
	if agentPayload.MetricsTarget.ResourceType != "agent" || agentPayload.MetricsTarget.ResourceID != "agent-1" {
		t.Fatalf("metrics target = %+v", agentPayload.MetricsTarget)
	}
	if agentPayload.Agent.StoragePostureSummary != "Unraid array is running without parity protection" {
		t.Fatalf("agent storage posture summary = %q", agentPayload.Agent.StoragePostureSummary)
	}
	if agentPayload.Agent.Unraid.PostureSummary != "Unraid array is running without parity protection" {
		t.Fatalf("unraid posture summary = %q", agentPayload.Agent.Unraid.PostureSummary)
	}
	if len(agentPayload.Agent.Unraid.Risk.Reasons) != 1 || agentPayload.Agent.Unraid.Risk.Reasons[0].Code != "unraid_no_parity" {
		t.Fatalf("unraid risk reasons = %#v", agentPayload.Agent.Unraid.Risk.Reasons)
	}

	var storagePayload struct {
		PlatformType string   `json:"platformType"`
		SourceType   string   `json:"sourceType"`
		Sources      []string `json:"sources"`
		PlatformData struct {
			Platform       string `json:"platform"`
			Type           string `json:"type"`
			Topology       string `json:"topology"`
			PostureSummary string `json:"postureSummary"`
			SyncAction     string `json:"syncAction"`
		} `json:"platformData"`
		Storage struct {
			PostureSummary string `json:"postureSummary"`
			Risk           struct {
				Reasons []struct {
					Code string `json:"code"`
				} `json:"reasons"`
			} `json:"risk"`
		} `json:"storage"`
	}
	encoded, err = json.Marshal(frontend[1])
	if err != nil {
		t.Fatalf("marshal storage frontend resource: %v", err)
	}
	if err := json.Unmarshal(encoded, &storagePayload); err != nil {
		t.Fatalf("unmarshal storage frontend resource: %v", err)
	}
	if storagePayload.PlatformType != "agent" || storagePayload.SourceType != "agent" {
		t.Fatalf("expected unraid storage to remain agent-backed, got platform=%q source=%q", storagePayload.PlatformType, storagePayload.SourceType)
	}
	if len(storagePayload.Sources) != 1 || storagePayload.Sources[0] != "agent" {
		t.Fatalf("storage sources = %#v, want [agent]", storagePayload.Sources)
	}
	if storagePayload.PlatformData.Platform != "unraid" || storagePayload.PlatformData.Type != "unraid-array" || storagePayload.PlatformData.Topology != "array" {
		t.Fatalf("expected unraid storage platform data, got %+v", storagePayload.PlatformData)
	}
	if storagePayload.PlatformData.PostureSummary != "Unraid array is running without parity protection" {
		t.Fatalf("platformData posture summary = %q", storagePayload.PlatformData.PostureSummary)
	}
	if storagePayload.Storage.PostureSummary != "Unraid array is running without parity protection" {
		t.Fatalf("storage posture summary = %q", storagePayload.Storage.PostureSummary)
	}
	if len(storagePayload.Storage.Risk.Reasons) != 1 || storagePayload.Storage.Risk.Reasons[0].Code != "unraid_no_parity" {
		t.Fatalf("storage risk reasons = %#v", storagePayload.Storage.Risk.Reasons)
	}
}

func TestMonitorMonitoredSystemUsageWaitsForSupplementalReadinessAndStoreFreshness(t *testing.T) {
	monitor := &Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(nil))

	provider := &testSupplementalInventoryReadinessProvider{
		source: unifiedresources.SourceTrueNAS,
	}
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)

	if usage := monitor.MonitoredSystemUsage(); usage.Available {
		t.Fatalf("expected unsettled supplemental inventory to be unavailable, got %+v", usage)
	}

	readyAt := time.Now().UTC()
	provider.records = []unifiedresources.IngestRecord{{
		SourceID: "system:tn-main",
		Resource: unifiedresources.Resource{
			ID:       "tn-main",
			Type:     unifiedresources.ResourceTypeAgent,
			Name:     "tn-main",
			Status:   unifiedresources.StatusOnline,
			LastSeen: readyAt,
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"tn-main"},
			},
			TrueNAS: &unifiedresources.TrueNASData{
				Hostname: "tn-main",
			},
		},
	}}
	provider.readyAt = readyAt
	provider.settled = true

	if usage := monitor.MonitoredSystemUsage(); usage.Available {
		t.Fatalf("expected stale store freshness to keep usage unavailable, got %+v", usage)
	}

	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)

	usage := monitor.MonitoredSystemUsage()
	if !usage.Available {
		t.Fatalf("expected usage to become available after rebuild, got %+v", usage)
	}
	if usage.Count != 1 {
		t.Fatalf("MonitoredSystemUsage().Count = %d, want 1", usage.Count)
	}
}

func TestMonitorMonitoredSystemUsageIncludesRecentStandaloneHostContinuity(t *testing.T) {
	now := time.Now().UTC()
	store := config.NewHostContinuityStore(t.TempDir(), nil)
	if err := store.Upsert(config.HostContinuityEntry{
		HostID:       "host-1",
		ReportHostID: "machine-1",
		Hostname:     "host-1.local",
		MachineID:    "machine-1",
		LastSeen:     now,
	}); err != nil {
		t.Fatalf("Upsert continuity: %v", err)
	}

	monitor := &Monitor{
		state:               models.NewState(),
		hostContinuityStore: store,
	}

	usage := monitor.MonitoredSystemUsage()
	if !usage.Available {
		t.Fatalf("expected continuity-backed usage to be available, got %+v", usage)
	}
	if usage.Count != 1 {
		t.Fatalf("MonitoredSystemUsage().Count = %d, want 1", usage.Count)
	}
}

func TestMonitorMonitoredSystemUsageDoesNotDoubleCountLinkedProxmoxHostContinuity(t *testing.T) {
	now := time.Now().UTC()
	store := config.NewHostContinuityStore(t.TempDir(), nil)
	if err := store.Upsert(config.HostContinuityEntry{
		HostID:       "host-1",
		ReportHostID: "machine-1",
		Hostname:     "pve-1",
		MachineID:    "machine-1",
		LinkedNodeID: "node-1",
		LastSeen:     now,
	}); err != nil {
		t.Fatalf("Upsert continuity: %v", err)
	}

	state := models.NewState()
	state.UpdateNodes([]models.Node{{
		ID:       "node-1",
		Name:     "pve-1",
		Status:   "online",
		LastSeen: now,
	}})

	monitor := &Monitor{
		state:               state,
		hostContinuityStore: store,
	}

	usage := monitor.MonitoredSystemUsage()
	if !usage.Available {
		t.Fatalf("expected continuity-backed usage to be available, got %+v", usage)
	}
	if usage.Count != 1 {
		t.Fatalf("MonitoredSystemUsage().Count = %d, want 1", usage.Count)
	}
}
