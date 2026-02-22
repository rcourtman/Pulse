package licensing

import (
	"testing"
	"time"
)

func TestEvaluateTrialStartEligibility(t *testing.T) {
	now := time.Unix(1700000000, 0)
	started := now.Add(-24 * time.Hour).Unix()

	tests := []struct {
		name             string
		hasActiveLicense bool
		existing         *BillingState
		wantAllowed      bool
		wantReason       TrialStartDenialReason
	}{
		{
			name:             "denied when license active",
			hasActiveLicense: true,
			existing:         nil,
			wantAllowed:      false,
			wantReason:       TrialStartDeniedLicense,
		},
		{
			name:             "allowed with no state",
			hasActiveLicense: false,
			existing:         nil,
			wantAllowed:      true,
			wantReason:       TrialStartAllowed,
		},
		{
			name:             "denied when trial already used",
			hasActiveLicense: false,
			existing:         &BillingState{TrialStartedAt: &started},
			wantAllowed:      false,
			wantReason:       TrialStartDeniedAlreadyUsed,
		},
		{
			name:             "denied on active subscription",
			hasActiveLicense: false,
			existing:         &BillingState{SubscriptionState: SubStateActive},
			wantAllowed:      false,
			wantReason:       TrialStartDeniedSubscription,
		},
		{
			name:             "denied on grace subscription",
			hasActiveLicense: false,
			existing:         &BillingState{SubscriptionState: SubStateGrace},
			wantAllowed:      false,
			wantReason:       TrialStartDeniedSubscription,
		},
		{
			name:             "denied on suspended subscription",
			hasActiveLicense: false,
			existing:         &BillingState{SubscriptionState: SubStateSuspended},
			wantAllowed:      false,
			wantReason:       TrialStartDeniedSubscription,
		},
		{
			name:             "allowed on expired subscription",
			hasActiveLicense: false,
			existing:         &BillingState{SubscriptionState: SubStateExpired},
			wantAllowed:      true,
			wantReason:       TrialStartAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateTrialStartEligibility(tt.hasActiveLicense, tt.existing)
			if got.Allowed != tt.wantAllowed {
				t.Fatalf("allowed=%t, want %t", got.Allowed, tt.wantAllowed)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("reason=%q, want %q", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestTrialStartError(t *testing.T) {
	tests := []struct {
		name             string
		reason           TrialStartDenialReason
		wantCode         string
		wantMessage      string
		wantIncludeOrgID bool
	}{
		{
			name:             "license active",
			reason:           TrialStartDeniedLicense,
			wantCode:         "trial_not_available",
			wantMessage:      "Trial cannot be started while a license is active",
			wantIncludeOrgID: false,
		},
		{
			name:             "already used",
			reason:           TrialStartDeniedAlreadyUsed,
			wantCode:         "trial_already_used",
			wantMessage:      "Trial has already been used for this organization",
			wantIncludeOrgID: true,
		},
		{
			name:             "subscription active",
			reason:           TrialStartDeniedSubscription,
			wantCode:         "trial_not_available",
			wantMessage:      "Trial cannot be started while a subscription is active",
			wantIncludeOrgID: true,
		},
		{
			name:             "unknown",
			reason:           TrialStartAllowed,
			wantCode:         "",
			wantMessage:      "",
			wantIncludeOrgID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, message, includeOrgID := TrialStartError(tt.reason)
			if code != tt.wantCode {
				t.Fatalf("code=%q, want %q", code, tt.wantCode)
			}
			if message != tt.wantMessage {
				t.Fatalf("message=%q, want %q", message, tt.wantMessage)
			}
			if includeOrgID != tt.wantIncludeOrgID {
				t.Fatalf("includeOrgID=%t, want %t", includeOrgID, tt.wantIncludeOrgID)
			}
		})
	}
}

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
}
