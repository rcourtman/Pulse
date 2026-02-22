package licensing

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type mockBillingStore struct {
	mu    sync.Mutex
	state *BillingState
	err   error
	calls int
}

func (m *mockBillingStore) GetBillingState(_ string) (*BillingState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	if m.state == nil {
		return nil, nil
	}

	state := cloneBillingState(*m.state)
	return &state, nil
}

func (m *mockBillingStore) setState(state *BillingState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
}

func (m *mockBillingStore) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *mockBillingStore) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestDatabaseSourceHappyPath(t *testing.T) {
	store := &mockBillingStore{
		state: &BillingState{
			Capabilities:      []string{"rbac", "relay"},
			Limits:            map[string]int64{"max_nodes": 50},
			MetersEnabled:     []string{"active_agents"},
			PlanVersion:       "pro-v2",
			SubscriptionState: SubStateActive,
		},
	}

	source := NewDatabaseSource(store, "org-1", time.Hour)

	if got := source.Capabilities(); !reflect.DeepEqual(got, []string{"rbac", "relay"}) {
		t.Fatalf("expected capabilities %v, got %v", []string{"rbac", "relay"}, got)
	}
	if got := source.Limits(); !reflect.DeepEqual(got, map[string]int64{"max_nodes": 50}) {
		t.Fatalf("expected limits %v, got %v", map[string]int64{"max_nodes": 50}, got)
	}
	if got := source.MetersEnabled(); !reflect.DeepEqual(got, []string{"active_agents"}) {
		t.Fatalf("expected meters %v, got %v", []string{"active_agents"}, got)
	}
	if got := source.PlanVersion(); got != "pro-v2" {
		t.Fatalf("expected plan_version %q, got %q", "pro-v2", got)
	}
	if got := source.SubscriptionState(); got != SubStateActive {
		t.Fatalf("expected subscription_state %q, got %q", SubStateActive, got)
	}

	if store.callCount() != 1 {
		t.Fatalf("expected store to be called once, got %d", store.callCount())
	}
}

func TestDatabaseSourceCacheHit(t *testing.T) {
	store := &mockBillingStore{
		state: &BillingState{
			PlanVersion:       "pro-v1",
			SubscriptionState: SubStateActive,
		},
	}

	source := NewDatabaseSource(store, "org-1", time.Hour)

	_ = source.PlanVersion()
	_ = source.PlanVersion()

	if store.callCount() != 1 {
		t.Fatalf("expected cache hit to avoid second store call, got %d calls", store.callCount())
	}
}

func TestDatabaseSourceCacheMissRefresh(t *testing.T) {
	store := &mockBillingStore{
		state: &BillingState{
			PlanVersion:       "pro-v1",
			SubscriptionState: SubStateActive,
		},
	}

	source := NewDatabaseSource(store, "org-1", time.Hour)

	if got := source.PlanVersion(); got != "pro-v1" {
		t.Fatalf("expected initial plan_version %q, got %q", "pro-v1", got)
	}

	store.setState(&BillingState{
		PlanVersion:       "pro-v2",
		SubscriptionState: SubStateActive,
	})

	source.mu.Lock()
	source.cacheTime = time.Now().Add(-2 * time.Hour)
	source.mu.Unlock()

	if got := source.PlanVersion(); got != "pro-v2" {
		t.Fatalf("expected refreshed plan_version %q, got %q", "pro-v2", got)
	}

	if store.callCount() != 2 {
		t.Fatalf("expected store refresh call, got %d calls", store.callCount())
	}
}

func TestDatabaseSourceFailOpenWithStaleCache(t *testing.T) {
	store := &mockBillingStore{
		state: &BillingState{
			Capabilities:      []string{"rbac"},
			PlanVersion:       "pro-v1",
			SubscriptionState: SubStateActive,
		},
	}

	source := NewDatabaseSource(store, "org-1", time.Hour)

	if got := source.Capabilities(); !reflect.DeepEqual(got, []string{"rbac"}) {
		t.Fatalf("expected initial capabilities %v, got %v", []string{"rbac"}, got)
	}

	source.mu.Lock()
	source.cacheTime = time.Now().Add(-2 * time.Hour)
	source.mu.Unlock()

	store.setError(errors.New("store unavailable"))
	store.setState(&BillingState{
		Capabilities:      []string{"new_capability"},
		PlanVersion:       "pro-v2",
		SubscriptionState: SubStateActive,
	})

	if got := source.Capabilities(); !reflect.DeepEqual(got, []string{"rbac"}) {
		t.Fatalf("expected stale cached capabilities on failure, got %v", got)
	}

	if store.callCount() != 2 {
		t.Fatalf("expected refresh attempt with failure, got %d calls", store.callCount())
	}
}

func TestDatabaseSourceFailOpenWithNoCache(t *testing.T) {
	store := &mockBillingStore{
		err: errors.New("store unavailable"),
	}

	source := NewDatabaseSource(store, "org-1", time.Hour)

	if got := source.Capabilities(); got != nil {
		t.Fatalf("expected default nil capabilities, got %v", got)
	}
	if got := source.Limits(); got != nil {
		t.Fatalf("expected default nil limits, got %v", got)
	}
	if got := source.MetersEnabled(); got != nil {
		t.Fatalf("expected default nil meters_enabled, got %v", got)
	}
	if got := source.PlanVersion(); got != "trial" {
		t.Fatalf("expected default plan_version %q, got %q", "trial", got)
	}
	if got := source.SubscriptionState(); got != SubStateTrial {
		t.Fatalf("expected default subscription_state %q, got %q", SubStateTrial, got)
	}
}

func TestDatabaseSourceTrialExpiryMarksExpiredAndStripsCapabilities(t *testing.T) {
	expiredAt := time.Now().Add(-1 * time.Hour).Unix()
	store := &mockBillingStore{
		state: &BillingState{
			Capabilities:      []string{"ai_autofix", "relay"},
			Limits:            map[string]int64{"max_nodes": 50},
			MetersEnabled:     []string{"active_agents"},
			PlanVersion:       "trial",
			SubscriptionState: SubStateTrial,
			TrialStartedAt:    ptrInt64(time.Now().Add(-15 * 24 * time.Hour).Unix()),
			TrialEndsAt:       &expiredAt,
		},
	}

	source := NewDatabaseSource(store, "org-1", time.Hour)

	if got := source.SubscriptionState(); got != SubStateExpired {
		t.Fatalf("expected subscription_state %q, got %q", SubStateExpired, got)
	}
	if got := source.Capabilities(); got != nil && len(got) != 0 {
		t.Fatalf("expected capabilities to be stripped on expiry, got %v", got)
	}
	if got := source.Limits(); got != nil && len(got) != 0 {
		t.Fatalf("expected limits to be stripped on expiry, got %v", got)
	}
	if got := source.MetersEnabled(); got != nil && len(got) != 0 {
		t.Fatalf("expected meters_enabled to be stripped on expiry, got %v", got)
	}
}

func TestDatabaseSourceImplementsEntitlementSource(t *testing.T) {
	t.Helper()
	var _ EntitlementSource = (*DatabaseSource)(nil)
}

func ptrInt64(v int64) *int64 {
	return &v
}
