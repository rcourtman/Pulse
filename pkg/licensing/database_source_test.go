package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
			Limits:            map[string]int64{"max_agents": 50},
			MetersEnabled:     []string{"active_agents"},
			PlanVersion:       "pro-v2",
			SubscriptionState: SubStateActive,
		},
	}

	source := NewDatabaseSource(store, "org-1", time.Hour)

	if got := source.Capabilities(); !reflect.DeepEqual(got, []string{"rbac", "relay"}) {
		t.Fatalf("expected capabilities %v, got %v", []string{"rbac", "relay"}, got)
	}
	if got := source.Limits(); !reflect.DeepEqual(got, map[string]int64{"max_agents": 50}) {
		t.Fatalf("expected limits %v, got %v", map[string]int64{"max_agents": 50}, got)
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

func TestDatabaseSourceLimits_MaxNodesMigration(t *testing.T) {
	store := &mockBillingStore{
		state: &BillingState{
			Limits:            map[string]int64{"max_nodes": 25},
			SubscriptionState: SubStateActive,
		},
	}
	source := NewDatabaseSource(store, "org-1", time.Hour)
	got := source.Limits()
	if got["max_agents"] != 25 {
		t.Fatalf("expected max_agents=25, got %d", got["max_agents"])
	}
	if _, hasOld := got["max_nodes"]; hasOld {
		t.Fatal("expected max_nodes to be absent after migration")
	}
}

func TestDatabaseSourceCanonicalizesCloudPlanVersionAndLimits(t *testing.T) {
	store := &mockBillingStore{
		state: &BillingState{
			PlanVersion:       "cloud_v1",
			Limits:            map[string]int64{"max_agents": 999},
			SubscriptionState: SubStateActive,
		},
	}

	source := NewDatabaseSource(store, "org-1", time.Hour)

	if got := source.PlanVersion(); got != "cloud_starter" {
		t.Fatalf("expected plan_version %q, got %q", "cloud_starter", got)
	}
	if got := source.Limits()["max_agents"]; got != 10 {
		t.Fatalf("expected max_agents=%d, got %d", 10, got)
	}
}

func TestDatabaseSourcePreservesMissingPlanVersion(t *testing.T) {
	store := &mockBillingStore{
		state: &BillingState{
			PlanVersion:       "   ",
			Limits:            map[string]int64{"max_agents": 42},
			SubscriptionState: SubscriptionState(" ACTIVE "),
		},
	}

	source := NewDatabaseSource(store, "org-1", time.Hour)

	if got := source.PlanVersion(); got != "" {
		t.Fatalf("expected missing plan_version to stay empty, got %q", got)
	}
	if got := source.SubscriptionState(); got != SubStateActive {
		t.Fatalf("expected subscription_state %q, got %q", SubStateActive, got)
	}
	if got := source.Limits()["max_agents"]; got != 42 {
		t.Fatalf("expected max_agents=%d, got %d", 42, got)
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
			Limits:            map[string]int64{"max_agents": 50},
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

func TestDatabaseSourceLeaseOnlyStateResolvesTrialEntitlement(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	embeddedBefore := EmbeddedPublicKey
	EmbeddedPublicKey = ""
	t.Cleanup(func() { EmbeddedPublicKey = embeddedBefore })
	t.Setenv(TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	now := time.Now().UTC()
	trialState := BuildTrialBillingState(now, []string{"ai_autofix"})
	token, err := SignEntitlementLeaseToken(priv, EntitlementLeaseClaims{
		OrgID:             "org-1",
		InstanceHost:      "pulse.example.com",
		PlanVersion:       trialState.PlanVersion,
		SubscriptionState: trialState.SubscriptionState,
		Capabilities:      append([]string(nil), trialState.Capabilities...),
		Limits:            map[string]int64{"max_agents": 25},
		TrialStartedAt:    trialState.TrialStartedAt,
		TrialEndsAt:       trialState.TrialEndsAt,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(time.Unix(*trialState.TrialEndsAt, 0).UTC()),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}

	store := &mockBillingStore{
		state: &BillingState{
			EntitlementJWT: token,
			TrialStartedAt: trialState.TrialStartedAt,
		},
	}
	source := NewDatabaseSource(store, "org-1", time.Hour).WithExpectedInstanceHost("pulse.example.com")

	if got := source.SubscriptionState(); got != SubStateTrial {
		t.Fatalf("expected subscription_state %q, got %q", SubStateTrial, got)
	}
	if got := source.Capabilities(); !reflect.DeepEqual(got, []string{"ai_autofix"}) {
		t.Fatalf("expected capabilities %v, got %v", []string{"ai_autofix"}, got)
	}
	if got := source.Limits(); !reflect.DeepEqual(got, map[string]int64{"max_agents": 25}) {
		t.Fatalf("expected limits %v, got %v", map[string]int64{"max_agents": 25}, got)
	}
	if got := source.TrialStartedAt(); got == nil || *got != *trialState.TrialStartedAt {
		t.Fatalf("expected trial_started_at %v, got %v", trialState.TrialStartedAt, got)
	}
	if got := source.TrialEndsAt(); got == nil || *got != *trialState.TrialEndsAt {
		t.Fatalf("expected trial_ends_at %v, got %v", trialState.TrialEndsAt, got)
	}
}

func TestDatabaseSourceLeaseHostMismatchFailsClosed(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	embeddedBefore := EmbeddedPublicKey
	EmbeddedPublicKey = ""
	t.Cleanup(func() { EmbeddedPublicKey = embeddedBefore })
	t.Setenv(TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	now := time.Now().UTC()
	trialState := BuildTrialBillingState(now, []string{FeatureAIAutoFix})
	token, err := SignEntitlementLeaseToken(priv, EntitlementLeaseClaims{
		OrgID:             "org-1",
		InstanceHost:      "pulse-a.example.com",
		PlanVersion:       trialState.PlanVersion,
		SubscriptionState: trialState.SubscriptionState,
		Capabilities:      append([]string(nil), trialState.Capabilities...),
		Limits:            map[string]int64{"max_agents": 25},
		TrialStartedAt:    trialState.TrialStartedAt,
		TrialEndsAt:       trialState.TrialEndsAt,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(time.Unix(*trialState.TrialEndsAt, 0).UTC()),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}

	store := &mockBillingStore{
		state: &BillingState{
			EntitlementJWT: token,
			TrialStartedAt: trialState.TrialStartedAt,
		},
	}
	source := NewDatabaseSource(store, "org-1", time.Hour).WithExpectedInstanceHost("pulse-b.example.com")

	if got := source.SubscriptionState(); got != SubStateExpired {
		t.Fatalf("expected subscription_state %q on host mismatch, got %q", SubStateExpired, got)
	}
	if got := source.Capabilities(); got != nil && len(got) != 0 {
		t.Fatalf("expected capabilities to be stripped on host mismatch, got %v", got)
	}
	if got := source.Limits(); got != nil && len(got) != 0 {
		t.Fatalf("expected limits to be stripped on host mismatch, got %v", got)
	}
	if got := source.TrialStartedAt(); got == nil || *got != *trialState.TrialStartedAt {
		t.Fatalf("expected trial_started_at %v to be preserved, got %v", trialState.TrialStartedAt, got)
	}
}

func TestDatabaseSourceImplementsEntitlementSource(t *testing.T) {
	t.Helper()
	var _ EntitlementSource = (*DatabaseSource)(nil)
}

func ptrInt64(v int64) *int64 {
	return &v
}
