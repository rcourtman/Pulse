package licensing

import "testing"

func TestIsHostedSubscriptionValid(t *testing.T) {
	tests := []struct {
		name        string
		state       SubscriptionState
		hasTrialEnd bool
		want        bool
	}{
		{name: "active", state: SubStateActive, hasTrialEnd: false, want: true},
		{name: "grace", state: SubStateGrace, hasTrialEnd: false, want: true},
		{name: "trial with end", state: SubStateTrial, hasTrialEnd: true, want: true},
		{name: "trial without end", state: SubStateTrial, hasTrialEnd: false, want: false},
		{name: "expired", state: SubStateExpired, hasTrialEnd: true, want: false},
		{name: "suspended", state: SubStateSuspended, hasTrialEnd: true, want: false},
		{name: "canceled", state: SubStateCanceled, hasTrialEnd: true, want: false},
		{name: "unknown", state: SubscriptionState("unknown"), hasTrialEnd: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHostedSubscriptionValid(tt.state, tt.hasTrialEnd); got != tt.want {
				t.Fatalf("valid=%t, want %t", got, tt.want)
			}
		})
	}
}
