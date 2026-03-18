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

func TestMonitorGetUnifiedReadStateOrSnapshotUsesMockSnapshotInsteadOfStore(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

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

	if hasUnifiedResource(getter.GetAll(), "store-only-resource") {
		t.Fatal("expected mock-mode read-state to ignore live resource store data")
	}
}
