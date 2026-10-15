package manager

import (
	"context"
	"errors"
	"testing"

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
