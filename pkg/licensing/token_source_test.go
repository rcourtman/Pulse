package licensing

import (
	"reflect"
	"testing"
)

type stubTokenClaims struct {
	capabilities      []string
	limits            map[string]int64
	metersEnabled     []string
	planVersion       string
	subscriptionState SubscriptionState
}

func (s stubTokenClaims) EffectiveCapabilities() []string {
	return append([]string(nil), s.capabilities...)
}

func (s stubTokenClaims) EffectiveLimits() map[string]int64 {
	if s.limits == nil {
		return nil
	}
	cp := make(map[string]int64, len(s.limits))
	for k, v := range s.limits {
		cp[k] = v
	}
	return cp
}

func (s stubTokenClaims) EntitlementMetersEnabled() []string {
	return append([]string(nil), s.metersEnabled...)
}

func (s stubTokenClaims) EntitlementPlanVersion() string {
	return s.planVersion
}

func (s stubTokenClaims) EntitlementSubscriptionState() SubscriptionState {
	return s.subscriptionState
}

func TestTokenSourceReturnsClaimValues(t *testing.T) {
	source := NewTokenSource(stubTokenClaims{
		capabilities:      []string{"relay", "ai_autofix"},
		limits:            map[string]int64{"max_agents": 42},
		metersEnabled:     []string{"agents"},
		planVersion:       "cloud_starter",
		subscriptionState: SubStateGrace,
	})

	if got := source.Capabilities(); !reflect.DeepEqual(got, []string{"relay", "ai_autofix"}) {
		t.Fatalf("Capabilities()=%v, want %v", got, []string{"relay", "ai_autofix"})
	}
	if got := source.Limits(); !reflect.DeepEqual(got, map[string]int64{"max_agents": 42}) {
		t.Fatalf("Limits()=%v, want %v", got, map[string]int64{"max_agents": 42})
	}
	if got := source.MetersEnabled(); !reflect.DeepEqual(got, []string{"agents"}) {
		t.Fatalf("MetersEnabled()=%v, want %v", got, []string{"agents"})
	}
	if got := source.PlanVersion(); got != "cloud_starter" {
		t.Fatalf("PlanVersion()=%q, want %q", got, "cloud_starter")
	}
	if got := source.SubscriptionState(); got != SubStateGrace {
		t.Fatalf("SubscriptionState()=%q, want %q", got, SubStateGrace)
	}
}

func TestTokenSourcePreservesMissingPlanVersion(t *testing.T) {
	source := NewTokenSource(stubTokenClaims{
		planVersion:       "",
		subscriptionState: SubStateActive,
		limits:            map[string]int64{"max_agents": 42},
	})

	if got := source.PlanVersion(); got != "" {
		t.Fatalf("PlanVersion()=%q, want empty", got)
	}
	if got := source.SubscriptionState(); got != SubStateActive {
		t.Fatalf("SubscriptionState()=%q, want %q", got, SubStateActive)
	}
	if got := source.Limits()["max_agents"]; got != 42 {
		t.Fatalf("Limits()[max_agents]=%d, want %d", got, 42)
	}
}

func TestTokenSourceNilDefaults(t *testing.T) {
	var source *TokenSource

	if got := source.Capabilities(); got != nil {
		t.Fatalf("Capabilities()=%v, want nil", got)
	}
	if got := source.Limits(); got != nil {
		t.Fatalf("Limits()=%v, want nil", got)
	}
	if got := source.MetersEnabled(); got != nil {
		t.Fatalf("MetersEnabled()=%v, want nil", got)
	}
	if got := source.PlanVersion(); got != "" {
		t.Fatalf("PlanVersion()=%q, want empty", got)
	}
	if got := source.SubscriptionState(); got != SubStateActive {
		t.Fatalf("SubscriptionState()=%q, want %q", got, SubStateActive)
	}
	if got := source.TrialStartedAt(); got != nil {
		t.Fatalf("TrialStartedAt()=%v, want nil", got)
	}
	if got := source.TrialEndsAt(); got != nil {
		t.Fatalf("TrialEndsAt()=%v, want nil", got)
	}
	if got := source.OverflowGrantedAt(); got != nil {
		t.Fatalf("OverflowGrantedAt()=%v, want nil", got)
	}
}
