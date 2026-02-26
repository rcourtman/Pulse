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
		Limits:            map[string]int64{"max_agents": 10},
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
	if normalized.Limits["max_agents"] != 10 {
		t.Fatalf("limits[max_nodes]=%d, want 10", normalized.Limits["max_agents"])
	}

	input.Capabilities[0] = "changed"
	input.MetersEnabled[0] = "changed"
	input.Limits["max_agents"] = 99
	if normalized.Capabilities[0] != " a " {
		t.Fatalf("expected capabilities to be copied")
	}
	if normalized.MetersEnabled[0] != "meter_a" {
		t.Fatalf("expected meters_enabled to be copied")
	}
	if normalized.Limits["max_agents"] != 10 {
		t.Fatalf("expected limits map to be copied")
	}
}

func TestNormalizeBillingStateNil(t *testing.T) {
	normalized := NormalizeBillingState(nil)
	if normalized.SubscriptionState != SubStateTrial {
		t.Fatalf("subscription_state=%q, want %q", normalized.SubscriptionState, SubStateTrial)
	}
}

func TestNormalizeBillingState_PreservesAllFields(t *testing.T) {
	trialStarted := int64(100)
	trialEnds := int64(200)
	trialExtended := int64(150)

	input := &BillingState{
		Capabilities:         []string{"relay", "ai"},
		Limits:               map[string]int64{"max_agents": 50},
		MetersEnabled:        []string{"active_agents"},
		PlanVersion:          "pro-v2",
		SubscriptionState:    SubStateActive,
		TrialStartedAt:       &trialStarted,
		TrialEndsAt:          &trialEnds,
		TrialExtendedAt:      &trialExtended,
		Integrity:            "deadbeef",
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_456",
		StripePriceID:        "price_789",
	}

	normalized := NormalizeBillingState(input)

	// Verify every field survives normalization.
	if len(normalized.Capabilities) != 2 {
		t.Fatalf("capabilities: got %v", normalized.Capabilities)
	}
	if normalized.Limits["max_agents"] != 50 {
		t.Fatalf("limits[max_nodes]: got %d", normalized.Limits["max_agents"])
	}
	if normalized.MetersEnabled[0] != "active_agents" {
		t.Fatalf("meters_enabled: got %v", normalized.MetersEnabled)
	}
	if normalized.PlanVersion != "pro-v2" {
		t.Fatalf("plan_version: got %q", normalized.PlanVersion)
	}
	if normalized.SubscriptionState != SubStateActive {
		t.Fatalf("subscription_state: got %q", normalized.SubscriptionState)
	}
	if normalized.TrialStartedAt == nil || *normalized.TrialStartedAt != 100 {
		t.Fatalf("trial_started_at: got %v", normalized.TrialStartedAt)
	}
	if normalized.TrialEndsAt == nil || *normalized.TrialEndsAt != 200 {
		t.Fatalf("trial_ends_at: got %v", normalized.TrialEndsAt)
	}
	if normalized.TrialExtendedAt == nil || *normalized.TrialExtendedAt != 150 {
		t.Fatalf("trial_extended_at: got %v", normalized.TrialExtendedAt)
	}
	if normalized.Integrity != "deadbeef" {
		t.Fatalf("integrity: got %q", normalized.Integrity)
	}
	if normalized.StripeCustomerID != "cus_123" {
		t.Fatalf("stripe_customer_id: got %q", normalized.StripeCustomerID)
	}
	if normalized.StripeSubscriptionID != "sub_456" {
		t.Fatalf("stripe_subscription_id: got %q", normalized.StripeSubscriptionID)
	}
	if normalized.StripePriceID != "price_789" {
		t.Fatalf("stripe_price_id: got %q", normalized.StripePriceID)
	}

	// Verify pointer fields are deep-copied (not aliased).
	*input.TrialStartedAt = 999
	if *normalized.TrialStartedAt != 100 {
		t.Fatal("trial_started_at was aliased, not cloned")
	}
	*input.TrialExtendedAt = 999
	if *normalized.TrialExtendedAt != 150 {
		t.Fatal("trial_extended_at was aliased, not cloned")
	}
}

func TestCloneBillingState_PreservesAllFields(t *testing.T) {
	trialStarted := int64(100)
	trialEnds := int64(200)
	trialExtended := int64(150)

	input := BillingState{
		Capabilities:         []string{"relay", "ai"},
		Limits:               map[string]int64{"max_agents": 50},
		MetersEnabled:        []string{"active_agents"},
		PlanVersion:          "pro-v2",
		SubscriptionState:    SubStateActive,
		TrialStartedAt:       &trialStarted,
		TrialEndsAt:          &trialEnds,
		TrialExtendedAt:      &trialExtended,
		Integrity:            "deadbeef",
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_456",
		StripePriceID:        "price_789",
	}

	cloned := cloneBillingState(input)

	if cloned.Integrity != "deadbeef" {
		t.Fatalf("integrity: got %q", cloned.Integrity)
	}
	if cloned.StripeCustomerID != "cus_123" {
		t.Fatalf("stripe_customer_id: got %q", cloned.StripeCustomerID)
	}
	if cloned.StripeSubscriptionID != "sub_456" {
		t.Fatalf("stripe_subscription_id: got %q", cloned.StripeSubscriptionID)
	}
	if cloned.StripePriceID != "price_789" {
		t.Fatalf("stripe_price_id: got %q", cloned.StripePriceID)
	}
	if cloned.TrialExtendedAt == nil || *cloned.TrialExtendedAt != 150 {
		t.Fatalf("trial_extended_at: got %v", cloned.TrialExtendedAt)
	}

	// Verify deep copy.
	*input.TrialExtendedAt = 999
	input.Capabilities[0] = "changed"
	if *cloned.TrialExtendedAt != 150 {
		t.Fatal("trial_extended_at was aliased")
	}
	if cloned.Capabilities[0] != "relay" {
		t.Fatal("capabilities were aliased")
	}
}

func TestNormalizeThenClone_RoundTrip(t *testing.T) {
	trialStarted := int64(100)
	trialEnds := int64(200)
	trialExtended := int64(150)

	original := &BillingState{
		Capabilities:         []string{"relay"},
		Limits:               map[string]int64{"max_agents": 50},
		MetersEnabled:        []string{"active_agents"},
		PlanVersion:          "pro-v2",
		SubscriptionState:    SubStateActive,
		TrialStartedAt:       &trialStarted,
		TrialEndsAt:          &trialEnds,
		TrialExtendedAt:      &trialExtended,
		Integrity:            "abc123",
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_2",
		StripePriceID:        "price_3",
	}

	normalized := NormalizeBillingState(original)
	cloned := cloneBillingState(*normalized)

	// Every field must survive the full normalize → clone chain.
	if cloned.Integrity != "abc123" {
		t.Fatalf("integrity lost in round-trip: got %q", cloned.Integrity)
	}
	if cloned.StripeCustomerID != "cus_1" {
		t.Fatalf("stripe_customer_id lost: got %q", cloned.StripeCustomerID)
	}
	if cloned.TrialExtendedAt == nil || *cloned.TrialExtendedAt != 150 {
		t.Fatalf("trial_extended_at lost: got %v", cloned.TrialExtendedAt)
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

func TestNormalizeBillingState_MaxNodesToMaxAgentsMigration(t *testing.T) {
	t.Run("legacy_key_migrated", func(t *testing.T) {
		state := &BillingState{
			Limits: map[string]int64{"max_nodes": 10},
		}
		normalized := NormalizeBillingState(state)
		if normalized.Limits["max_agents"] != 10 {
			t.Fatalf("expected max_agents=10, got %d", normalized.Limits["max_agents"])
		}
		if _, hasOld := normalized.Limits["max_nodes"]; hasOld {
			t.Fatal("expected max_nodes to be deleted after migration")
		}
	})

	t.Run("new_key_preserved_legacy_deleted", func(t *testing.T) {
		state := &BillingState{
			Limits: map[string]int64{"max_agents": 15, "max_nodes": 5},
		}
		normalized := NormalizeBillingState(state)
		if normalized.Limits["max_agents"] != 15 {
			t.Fatalf("expected max_agents=15, got %d", normalized.Limits["max_agents"])
		}
		if _, hasOld := normalized.Limits["max_nodes"]; hasOld {
			t.Fatal("expected max_nodes to be deleted when max_agents exists")
		}
	})

	t.Run("no_legacy_key_no_change", func(t *testing.T) {
		state := &BillingState{
			Limits: map[string]int64{"max_agents": 20},
		}
		normalized := NormalizeBillingState(state)
		if normalized.Limits["max_agents"] != 20 {
			t.Fatalf("expected max_agents=20, got %d", normalized.Limits["max_agents"])
		}
	})
}
