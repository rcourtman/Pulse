package manager

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

type testPointsSource struct {
	active bool
	orgID  string
	err    error
	called bool
}

func (s *testPointsSource) Active() bool { return s.active }

func (s *testPointsSource) ListPoints(_ context.Context, orgID string, _ recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
	s.called = true
	s.orgID = orgID
	return []recovery.RecoveryPoint{{ID: "source-point"}}, 1, s.err
}

func TestListPointsForOrgPreservesSourceBoundary(t *testing.T) {
	source := &testPointsSource{active: true}
	m := NewWithPointSource(nil, source)
	if _, _, err := m.ListPointsForOrg(context.Background(), " ", recovery.ListPointsOptions{}); err == nil || source.called {
		t.Fatal("missing tenant must fail before reading any source")
	}
	points, total, err := m.ListPointsForOrg(context.Background(), " tenant-a ", recovery.ListPointsOptions{})
	if err != nil || source.orgID != "tenant-a" || total != 1 || len(points) != 1 || points[0].ID != "source-point" {
		t.Fatalf("read = %v, %d, %v; org = %q", points, total, err, source.orgID)
	}
	want := errors.New("source unavailable")
	source.err = want
	if _, _, err := m.ListPointsForOrg(context.Background(), "tenant-a", recovery.ListPointsOptions{}); !errors.Is(err, want) {
		t.Fatalf("source error was lost: %v", err)
	}
	source.called = false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := m.ListPointsForOrg(ctx, "tenant-a", recovery.ListPointsOptions{}); !errors.Is(err, context.Canceled) || source.called {
		t.Fatal("cancelled read must not call the source")
	}
	if _, _, err := New(nil).ListPointsForOrg(context.Background(), "tenant-a", recovery.ListPointsOptions{}); err == nil {
		t.Fatal("unconfigured persisted source must not become an empty success")
	}
	source.active = false
	if _, _, err := m.ListPointsForOrg(context.Background(), "tenant-a", recovery.ListPointsOptions{}); err == nil || source.called {
		t.Fatal("inactive source must defer to the persisted boundary")
	}
}

func TestNew(t *testing.T) {
	// Test that New returns a non-nil Manager
	m := New(nil)
	if m == nil {
		t.Error("New(nil) returned nil")
	}

	// Verify the manager is properly initialized
	if m.stores == nil {
		t.Error("Manager.stores is nil")
	}
}

func TestNew_NilPersistence(t *testing.T) {
	// Test that New handles nil persistence gracefully
	// The Manager itself should be created, even if persistence is nil
	m := New(nil)
	if m == nil {
		t.Fatal("New(nil) should return non-nil Manager")
	}

	// The Manager struct should be valid but operations requiring
	// persistence should fail appropriately
	if m.stores == nil {
		t.Error("Manager.stores should be initialized even with nil persistence")
	}
}

// A deleted org's ID can be claimed again. The org created under it must open
// its own recovery database, not the cached handle of the deleted one.
func TestCloseStoreForOrgEvictsDeletedOrgStore(t *testing.T) {
	ctx := context.Background()
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	m := New(mtp)
	t.Cleanup(func() { _ = m.CloseStoreForOrg("acme") })

	createOrg := func() {
		t.Helper()
		if err := mtp.SaveOrganization(&models.Organization{ID: "acme", DisplayName: "Acme"}); err != nil {
			t.Fatalf("SaveOrganization: %v", err)
		}
	}

	createOrg()
	store, err := m.StoreForOrg("acme")
	if err != nil {
		t.Fatalf("StoreForOrg: %v", err)
	}
	completedAt := time.Now().UTC()
	if err := store.UpsertPoints(ctx, []recovery.RecoveryPoint{{
		ID:          "deleted-org-point",
		Provider:    recovery.ProviderKubernetes,
		Kind:        recovery.KindBackup,
		Mode:        recovery.ModeRemote,
		Outcome:     recovery.OutcomeSuccess,
		CompletedAt: &completedAt,
	}}); err != nil {
		t.Fatalf("UpsertPoints: %v", err)
	}
	if _, total, err := m.ListPointsForOrg(ctx, "acme", recovery.ListPointsOptions{}); err != nil || total != 1 {
		t.Fatalf("points before deletion = %d, %v; want 1", total, err)
	}

	if err := mtp.DeleteOrganization("acme"); err != nil {
		t.Fatalf("DeleteOrganization: %v", err)
	}
	if err := m.CloseStoreForOrg(" acme "); err != nil {
		t.Fatalf("CloseStoreForOrg: %v", err)
	}
	if err := m.CloseStoreForOrg("acme"); err != nil {
		t.Fatalf("CloseStoreForOrg on an evicted org must be a no-op: %v", err)
	}
	var nilManager *Manager
	if err := nilManager.CloseStoreForOrg("acme"); err != nil {
		t.Fatalf("nil manager CloseStoreForOrg: %v", err)
	}

	createOrg()
	reopened, err := m.StoreForOrg("acme")
	if err != nil {
		t.Fatalf("StoreForOrg after recreation: %v", err)
	}
	if reopened == store {
		t.Fatal("recreated org was served the deleted org's store")
	}
	if points, total, err := m.ListPointsForOrg(ctx, "acme", recovery.ListPointsOptions{}); err != nil || total != 0 || len(points) != 0 {
		t.Fatalf("recreated org reads %d points (%v), want none", total, err)
	}
}

// A StoreForOrg call that missed the cache before the eviction must not insert
// its handle after it: eviction waits for constructors that are in flight.
func TestCloseStoreForOrgWaitsForInFlightConstructors(t *testing.T) {
	m := New(nil)

	m.lifecycle.RLock()
	evicted := make(chan error, 1)
	go func() { evicted <- m.CloseStoreForOrg("acme") }()
	select {
	case err := <-evicted:
		m.lifecycle.RUnlock()
		t.Fatalf("eviction overtook an in-flight constructor: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	m.lifecycle.RUnlock()

	select {
	case err := <-evicted:
		if err != nil {
			t.Fatalf("CloseStoreForOrg: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("eviction did not finish after the constructor returned")
	}
}
