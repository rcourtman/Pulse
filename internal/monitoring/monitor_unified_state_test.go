package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

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
