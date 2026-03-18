package licensing

import (
	"testing"
)

func TestCanTransition_ValidTransitions(t *testing.T) {
	for transition := range validTransitions {
		transition := transition
		t.Run(string(transition.From)+"_to_"+string(transition.To), func(t *testing.T) {
			if !CanTransition(transition.From, transition.To) {
				t.Fatalf("expected transition %s -> %s to be valid", transition.From, transition.To)
			}
		})
	}
}

func TestCanTransition_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from SubscriptionState
		to   SubscriptionState
	}{
		{name: "trial_to_grace", from: SubStateTrial, to: SubStateGrace},
		{name: "trial_to_suspended", from: SubStateTrial, to: SubStateSuspended},
		{name: "active_to_trial", from: SubStateActive, to: SubStateTrial},
		{name: "active_to_expired", from: SubStateActive, to: SubStateExpired},
		{name: "grace_to_suspended", from: SubStateGrace, to: SubStateSuspended},
		{name: "grace_to_trial", from: SubStateGrace, to: SubStateTrial},
		{name: "expired_to_grace", from: SubStateExpired, to: SubStateGrace},
		{name: "expired_to_trial", from: SubStateExpired, to: SubStateTrial},
		{name: "expired_to_suspended", from: SubStateExpired, to: SubStateSuspended},
		{name: "suspended_to_grace", from: SubStateSuspended, to: SubStateGrace},
		{name: "suspended_to_trial", from: SubStateSuspended, to: SubStateTrial},
	}

	states := []SubscriptionState{
		SubStateTrial,
		SubStateActive,
		SubStateGrace,
		SubStateExpired,
		SubStateSuspended,
	}
	for _, state := range states {
		tests = append(tests, struct {
			name string
			from SubscriptionState
			to   SubscriptionState
		}{
			name: string(state) + "_to_itself",
			from: state,
			to:   state,
		})
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if CanTransition(tt.from, tt.to) {
				t.Fatalf("expected transition %s -> %s to be invalid", tt.from, tt.to)
			}
		})
	}
}

func TestValidTransitionsFrom(t *testing.T) {
	tests := []struct {
		from     SubscriptionState
		expected map[SubscriptionState]struct{}
	}{
		{
			from: SubStateTrial,
			expected: map[SubscriptionState]struct{}{
				SubStateActive:  {},
				SubStateExpired: {},
			},
		},
		{
			from: SubStateActive,
			expected: map[SubscriptionState]struct{}{
				SubStateGrace:     {},
				SubStateSuspended: {},
			},
		},
		{
			from: SubStateGrace,
			expected: map[SubscriptionState]struct{}{
				SubStateActive:  {},
				SubStateExpired: {},
			},
		},
		{
			from: SubStateExpired,
			expected: map[SubscriptionState]struct{}{
				SubStateActive: {},
			},
		},
		{
			from: SubStateSuspended,
			expected: map[SubscriptionState]struct{}{
				SubStateActive:  {},
				SubStateExpired: {},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.from), func(t *testing.T) {
			got := ValidTransitionsFrom(tt.from)
			gotSet := make(map[SubscriptionState]struct{}, len(got))
			for _, target := range got {
				gotSet[target] = struct{}{}
			}

			if len(gotSet) != len(tt.expected) {
				t.Fatalf("unexpected number of transitions for %s: got=%d want=%d", tt.from, len(gotSet), len(tt.expected))
			}

			for expectedTarget := range tt.expected {
				if _, ok := gotSet[expectedTarget]; !ok {
					t.Fatalf("missing expected transition for %s -> %s", tt.from, expectedTarget)
				}
			}
		})
	}
}

func TestGetBehavior(t *testing.T) {
	tests := []struct {
		state             SubscriptionState
		operations        OperationClass
		featuresAvailable bool
		showWarning       bool
	}{
		{
			state:             SubStateTrial,
			operations:        OpFull,
			featuresAvailable: true,
			showWarning:       false,
		},
		{
			state:             SubStateActive,
			operations:        OpFull,
			featuresAvailable: true,
			showWarning:       false,
		},
		{
			state:             SubStateGrace,
			operations:        OpFull,
			featuresAvailable: true,
			showWarning:       true,
		},
		{
			state:             SubStateExpired,
			operations:        OpDegraded,
			featuresAvailable: false,
			showWarning:       true,
		},
		{
			state:             SubStateSuspended,
			operations:        OpLocked,
			featuresAvailable: false,
			showWarning:       true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.state), func(t *testing.T) {
			got := GetBehavior(tt.state)
			if got.State != tt.state {
				t.Fatalf("unexpected state in behavior: got=%s want=%s", got.State, tt.state)
			}
			if got.Operations != tt.operations {
				t.Fatalf("unexpected operations for %s: got=%s want=%s", tt.state, got.Operations, tt.operations)
			}
			if got.FeaturesAvailable != tt.featuresAvailable {
				t.Fatalf("unexpected features flag for %s: got=%t want=%t", tt.state, got.FeaturesAvailable, tt.featuresAvailable)
			}
			if got.ShowWarning != tt.showWarning {
				t.Fatalf("unexpected warning flag for %s: got=%t want=%t", tt.state, got.ShowWarning, tt.showWarning)
			}
		})
	}
}

func TestGetBehavior_UnknownState(t *testing.T) {
	got := GetBehavior(SubscriptionState("unknown_state"))
	expected := StateBehaviors[SubStateExpired]
	if got != expected {
		t.Fatalf("expected unknown state fallback to expired behavior: got=%+v want=%+v", got, expected)
	}
}

func TestDefaultDowngradePolicy(t *testing.T) {
	if DefaultDowngradePolicy.SoftHideGraceDays != 30 {
		t.Fatalf("unexpected SoftHideGraceDays: got=%d want=30", DefaultDowngradePolicy.SoftHideGraceDays)
	}
	if DefaultDowngradePolicy.HardDeleteAfterDays != 60 {
		t.Fatalf("unexpected HardDeleteAfterDays: got=%d want=60", DefaultDowngradePolicy.HardDeleteAfterDays)
	}
}
