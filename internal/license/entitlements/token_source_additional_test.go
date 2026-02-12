package entitlements_test

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	. "github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

type stubTokenClaims struct {
	capabilities      []string
	limits            map[string]int64
	metersEnabled     []string
	planVersion       string
	subscriptionState license.SubscriptionState
}

func (s stubTokenClaims) EffectiveCapabilities() []string {
	return s.capabilities
}

func (s stubTokenClaims) EffectiveLimits() map[string]int64 {
	return s.limits
}

func (s stubTokenClaims) EntitlementMetersEnabled() []string {
	return s.metersEnabled
}

func (s stubTokenClaims) EntitlementPlanVersion() string {
	return s.planVersion
}

func (s stubTokenClaims) EntitlementSubscriptionState() license.SubscriptionState {
	return s.subscriptionState
}

func TestTokenSourcePlanVersionAndTrialAccessors(t *testing.T) {
	source := NewTokenSource(stubTokenClaims{
		capabilities:      []string{"relay"},
		limits:            map[string]int64{"max_nodes": 10},
		metersEnabled:     []string{"active_agents"},
		planVersion:       "pro-v3",
		subscriptionState: license.SubStateGrace,
	})

	if got := source.PlanVersion(); got != "pro-v3" {
		t.Fatalf("expected plan_version pro-v3, got %q", got)
	}
	if got := source.SubscriptionState(); got != license.SubStateGrace {
		t.Fatalf("expected subscription_state %q, got %q", license.SubStateGrace, got)
	}
	if got := source.TrialStartedAt(); got != nil {
		t.Fatalf("expected nil trial_started_at for token source, got %v", got)
	}
	if got := source.TrialEndsAt(); got != nil {
		t.Fatalf("expected nil trial_ends_at for token source, got %v", got)
	}
}

func TestTokenSourceNilReceiverAndNilClaimsDefaults(t *testing.T) {
	var nilSource *TokenSource

	if got := nilSource.Capabilities(); got != nil {
		t.Fatalf("expected nil capabilities for nil receiver, got %v", got)
	}
	if got := nilSource.Limits(); got != nil {
		t.Fatalf("expected nil limits for nil receiver, got %v", got)
	}
	if got := nilSource.MetersEnabled(); got != nil {
		t.Fatalf("expected nil meters for nil receiver, got %v", got)
	}
	if got := nilSource.PlanVersion(); got != "" {
		t.Fatalf("expected empty plan_version for nil receiver, got %q", got)
	}
	if got := nilSource.SubscriptionState(); got != SubStateActive {
		t.Fatalf("expected subscription_state %q for nil receiver, got %q", SubStateActive, got)
	}
	if got := nilSource.TrialStartedAt(); got != nil {
		t.Fatalf("expected nil trial_started_at for nil receiver, got %v", got)
	}
	if got := nilSource.TrialEndsAt(); got != nil {
		t.Fatalf("expected nil trial_ends_at for nil receiver, got %v", got)
	}

	claimsSource := NewTokenSource(nil)
	if got := claimsSource.Capabilities(); got != nil {
		t.Fatalf("expected nil capabilities for nil claims, got %v", got)
	}
	if got := claimsSource.Limits(); got != nil {
		t.Fatalf("expected nil limits for nil claims, got %v", got)
	}
	if got := claimsSource.MetersEnabled(); got != nil {
		t.Fatalf("expected nil meters for nil claims, got %v", got)
	}
	if got := claimsSource.PlanVersion(); got != "" {
		t.Fatalf("expected empty plan_version for nil claims, got %q", got)
	}
	if got := claimsSource.SubscriptionState(); got != SubStateActive {
		t.Fatalf("expected subscription_state %q for nil claims, got %q", SubStateActive, got)
	}
}

func TestTokenSourcePassThroughClaimsData(t *testing.T) {
	claims := stubTokenClaims{
		capabilities:      []string{"relay", "rbac"},
		limits:            map[string]int64{"max_nodes": 25},
		metersEnabled:     []string{"active_agents"},
		planVersion:       "pro-v2",
		subscriptionState: license.SubStateActive,
	}
	source := NewTokenSource(claims)

	if got := source.Capabilities(); !reflect.DeepEqual(got, claims.capabilities) {
		t.Fatalf("expected capabilities %v, got %v", claims.capabilities, got)
	}
	if got := source.Limits(); !reflect.DeepEqual(got, claims.limits) {
		t.Fatalf("expected limits %v, got %v", claims.limits, got)
	}
	if got := source.MetersEnabled(); !reflect.DeepEqual(got, claims.metersEnabled) {
		t.Fatalf("expected meters %v, got %v", claims.metersEnabled, got)
	}
}
