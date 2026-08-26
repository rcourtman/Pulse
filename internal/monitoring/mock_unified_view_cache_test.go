package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

func TestCurrentUnifiedStateViewCachedBetweenMockTicks(t *testing.T) {
	previous := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
	if previous {
		mustSetMockEnabled(t, false)
	}
	testConfig := previousConfig
	testConfig.UpdateInterval = 5 * time.Minute
	mock.SetMockConfig(testConfig)
	mustSetMockEnabled(t, true)
	t.Cleanup(func() {
		mustSetMockEnabled(t, false)
		mock.SetMockConfig(previousConfig)
		mustSetMockEnabled(t, previous)
	})

	m := &Monitor{}

	first := m.currentUnifiedStateView()
	if first.readState == nil {
		t.Fatal("expected a read state from the mock branch")
	}
	second := m.currentUnifiedStateView()
	if first.readState != second.readState {
		t.Fatal("expected the cached view while the fixture data version is unchanged")
	}
	if len(first.resources) == 0 || len(second.resources) != len(first.resources) {
		t.Fatalf("expected identical resource sets, got %d vs %d", len(first.resources), len(second.resources))
	}

	// Toggling mock mode rebuilds the fixture graph and advances the data
	// version, which must invalidate the cached view.
	mustSetMockEnabled(t, false)
	mustSetMockEnabled(t, true)

	third := m.currentUnifiedStateView()
	if third.readState == nil {
		t.Fatal("expected a read state after the fixture graph rebuilt")
	}
	if third.readState == first.readState {
		t.Fatal("expected a rebuilt view after the fixture data version advanced")
	}
}
