package licensing

import "testing"

func TestDefaultBillingState(t *testing.T) {
	state := DefaultBillingState()
	if state == nil {
		t.Fatal("expected non-nil default billing state")
	}
	if state.SubscriptionState != SubStateTrial {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, SubStateTrial)
	}
	if state.PlanVersion != string(SubStateTrial) {
		t.Fatalf("plan_version=%q, want %q", state.PlanVersion, string(SubStateTrial))
	}
	if state.Capabilities == nil || len(state.Capabilities) != 0 {
		t.Fatalf("expected empty capabilities slice, got %#v", state.Capabilities)
	}
	if state.MetersEnabled == nil || len(state.MetersEnabled) != 0 {
		t.Fatalf("expected empty meters_enabled slice, got %#v", state.MetersEnabled)
	}
	if state.Limits == nil || len(state.Limits) != 0 {
		t.Fatalf("expected empty limits map, got %#v", state.Limits)
	}
}

func TestNormalizeBillingState(t *testing.T) {
	trialStarted := int64(100)
	trialEnds := int64(200)
	input := &BillingState{
		Capabilities:      []string{" a ", "b"},
		Limits:            map[string]int64{"max_nodes": 10},
		MetersEnabled:     []string{"meter_a"},
		PlanVersion:       "  ",
		SubscriptionState: SubscriptionState(" ACTIVE "),
		TrialStartedAt:    &trialStarted,
		TrialEndsAt:       &trialEnds,
		StripeCustomerID:  " cus_123 ",
	}

	normalized := NormalizeBillingState(input)
	if normalized.SubscriptionState != SubStateActive {
		t.Fatalf("subscription_state=%q, want %q", normalized.SubscriptionState, SubStateActive)
	}
	if normalized.PlanVersion != string(SubStateActive) {
		t.Fatalf("plan_version=%q, want %q", normalized.PlanVersion, string(SubStateActive))
	}
	if normalized.StripeCustomerID != "cus_123" {
		t.Fatalf("stripe_customer_id=%q, want %q", normalized.StripeCustomerID, "cus_123")
	}
	if normalized.Limits["max_nodes"] != 10 {
		t.Fatalf("limits[max_nodes]=%d, want 10", normalized.Limits["max_nodes"])
	}

	input.Capabilities[0] = "changed"
	input.MetersEnabled[0] = "changed"
	input.Limits["max_nodes"] = 99
	if normalized.Capabilities[0] != " a " {
		t.Fatalf("expected capabilities to be copied")
	}
	if normalized.MetersEnabled[0] != "meter_a" {
		t.Fatalf("expected meters_enabled to be copied")
	}
	if normalized.Limits["max_nodes"] != 10 {
		t.Fatalf("expected limits map to be copied")
	}
}

func TestNormalizeBillingStateNil(t *testing.T) {
	normalized := NormalizeBillingState(nil)
	if normalized.SubscriptionState != SubStateTrial {
		t.Fatalf("subscription_state=%q, want %q", normalized.SubscriptionState, SubStateTrial)
	}
}

func TestIsValidBillingSubscriptionState(t *testing.T) {
	valid := []SubscriptionState{
		SubStateTrial,
		SubStateActive,
		SubStateGrace,
		SubStateExpired,
		SubStateSuspended,
		SubStateCanceled,
	}
	for _, state := range valid {
		if !IsValidBillingSubscriptionState(state) {
			t.Fatalf("expected %q to be valid", state)
		}
	}
	if IsValidBillingSubscriptionState(SubscriptionState("invalid")) {
		t.Fatal("expected invalid state to be rejected")
	}
}
