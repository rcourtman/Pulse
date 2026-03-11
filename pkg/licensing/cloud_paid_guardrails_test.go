package licensing

import "testing"

func TestNormalizeBillingStatePreservesMissingPlanVersion(t *testing.T) {
	state := &BillingState{
		PlanVersion:       "   ",
		Limits:            map[string]int64{"max_agents": 42},
		SubscriptionState: SubscriptionState(" ACTIVE "),
	}

	normalized := NormalizeBillingState(state)
	if normalized.PlanVersion != "" {
		t.Fatalf("plan_version=%q, want empty", normalized.PlanVersion)
	}
	if normalized.SubscriptionState != SubStateActive {
		t.Fatalf("subscription_state=%q, want %q", normalized.SubscriptionState, SubStateActive)
	}
	if got := normalized.Limits["max_agents"]; got != 42 {
		t.Fatalf("limits[max_agents]=%d, want %d", got, 42)
	}
}
