package licensing

import (
	"testing"
	"time"
)

func TestTrialWindow(t *testing.T) {
	now := time.Unix(1700000000, 0)

	startedAt, endsAt := TrialWindow(now, 48*time.Hour)
	if startedAt != now.Unix() {
		t.Fatalf("startedAt=%d, want %d", startedAt, now.Unix())
	}
	if endsAt != now.Add(48*time.Hour).Unix() {
		t.Fatalf("endsAt=%d, want %d", endsAt, now.Add(48*time.Hour).Unix())
	}
}

func TestBuildTrialBillingState(t *testing.T) {
	now := time.Unix(1700000000, 0)
	capsInput := []string{"a", "b"}

	state := BuildTrialBillingState(now, capsInput)
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.SubscriptionState != SubStateTrial {
		t.Fatalf("subscription state=%q, want %q", state.SubscriptionState, SubStateTrial)
	}
	if state.PlanVersion != string(SubStateTrial) {
		t.Fatalf("plan version=%q, want %q", state.PlanVersion, string(SubStateTrial))
	}
	if state.TrialStartedAt == nil || state.TrialEndsAt == nil {
		t.Fatalf("expected trial timestamps to be set, got started=%v ends=%v", state.TrialStartedAt, state.TrialEndsAt)
	}
	if *state.TrialStartedAt != now.Unix() {
		t.Fatalf("trial_started_at=%d, want %d", *state.TrialStartedAt, now.Unix())
	}
	if *state.TrialEndsAt != now.Add(DefaultTrialDuration).Unix() {
		t.Fatalf("trial_ends_at=%d, want %d", *state.TrialEndsAt, now.Add(DefaultTrialDuration).Unix())
	}
	if len(state.Limits) != 0 {
		t.Fatalf("expected empty limits, got %#v", state.Limits)
	}
	if len(state.MetersEnabled) != 0 {
		t.Fatalf("expected empty meters, got %#v", state.MetersEnabled)
	}
	if len(state.Capabilities) != 2 {
		t.Fatalf("capabilities length=%d, want 2", len(state.Capabilities))
	}
	capsInput[0] = "changed"
	if state.Capabilities[0] != "a" {
		t.Fatalf("expected capability copy to be immutable from caller changes")
	}
}

func TestBuildTrialBillingStateWithPlan(t *testing.T) {
	now := time.Unix(1700000000, 0)
	state := BuildTrialBillingStateWithPlan(now, []string{"cloud_feature"}, " cloud_trial ", 72*time.Hour)

	if state.PlanVersion != "cloud_trial" {
		t.Fatalf("plan_version=%q, want %q", state.PlanVersion, "cloud_trial")
	}
	if state.SubscriptionState != SubStateTrial {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, SubStateTrial)
	}
	if *state.TrialStartedAt != now.Unix() {
		t.Fatalf("trial_started_at=%d, want %d", *state.TrialStartedAt, now.Unix())
	}
	if *state.TrialEndsAt != now.Add(72*time.Hour).Unix() {
		t.Fatalf("trial_ends_at=%d, want %d", *state.TrialEndsAt, now.Add(72*time.Hour).Unix())
	}
	if state.QuickstartCreditsGranted {
		t.Fatal("expected quickstart credits not to be granted for new trial workspaces")
	}
	if state.QuickstartCreditsGrantedAt != nil {
		t.Fatal("expected quickstart credits grant timestamp to stay empty")
	}
}
