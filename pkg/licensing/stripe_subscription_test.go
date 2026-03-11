package licensing

import "testing"

func TestMapStripeSubscriptionStatusToState(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   SubscriptionState
	}{
		{name: "active", status: "active", want: SubStateActive},
		{name: "trialing", status: "trialing", want: SubStateTrial},
		{name: "past due", status: "past_due", want: SubStateGrace},
		{name: "unpaid", status: "unpaid", want: SubStateGrace},
		{name: "canceled", status: "canceled", want: SubStateCanceled},
		{name: "paused", status: "paused", want: SubStateSuspended},
		{name: "incomplete", status: "incomplete", want: SubStateExpired},
		{name: "incomplete expired", status: "incomplete_expired", want: SubStateExpired},
		{name: "unknown", status: "unknown", want: SubStateExpired},
		{name: "trim and case", status: "  ACTIVE  ", want: SubStateActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapStripeSubscriptionStatusToState(tt.status)
			if got != tt.want {
				t.Fatalf("state=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldGrantPaidCapabilities(t *testing.T) {
	tests := []struct {
		name  string
		state SubscriptionState
		want  bool
	}{
		{name: "active", state: SubStateActive, want: true},
		{name: "trial", state: SubStateTrial, want: true},
		{name: "grace", state: SubStateGrace, want: true},
		{name: "expired", state: SubStateExpired, want: false},
		{name: "suspended", state: SubStateSuspended, want: false},
		{name: "canceled", state: SubStateCanceled, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldGrantPaidCapabilities(tt.state); got != tt.want {
				t.Fatalf("paid=%t, want %t", got, tt.want)
			}
		})
	}
}

func TestDeriveStripePlanVersion(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		priceID  string
		want     string
	}{
		{
			name:     "plan version wins",
			metadata: map[string]string{"plan_version": "cloud_v6"},
			priceID:  "price_123",
			want:     "cloud_v6",
		},
		{
			name:     "legacy cloud v1 metadata canonicalizes to starter",
			metadata: map[string]string{"plan_version": "cloud-v1"},
			priceID:  "price_123",
			want:     "cloud_starter",
		},
		{
			name:     "shorthand plan metadata canonicalizes to cloud tier",
			metadata: map[string]string{"plan": "power"},
			priceID:  "price_123",
			want:     "cloud_power",
		},
		{
			name:     "plan fallback",
			metadata: map[string]string{"plan": "cloud_v5"},
			priceID:  "price_123",
			want:     "cloud_v5",
		},
		{
			name:     "unknown price fallback",
			metadata: nil,
			priceID:  " price_123 ",
			want:     "stripe_price:price_123",
		},
		{
			name:     "known price resolves to plan",
			metadata: nil,
			priceID:  "price_1T5kflBrHBocJIGHUqPv1dzV",
			want:     "cloud_starter",
		},
		{
			name:     "metadata plan_version takes priority over known price",
			metadata: map[string]string{"plan_version": "cloud_power"},
			priceID:  "price_1T5kflBrHBocJIGHUqPv1dzV",
			want:     "cloud_power",
		},
		{
			name:     "default",
			metadata: nil,
			priceID:  "",
			want:     "stripe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveStripePlanVersion(tt.metadata, tt.priceID); got != tt.want {
				t.Fatalf("plan_version=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestCanonicalizePlanVersion(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"cloud-v1", "cloud_starter"},
		{" cloud_v1 ", "cloud_starter"},
		{"starter", "cloud_starter"},
		{"power", "cloud_power"},
		{"max", "cloud_max"},
		{"founding", "cloud_founding"},
		{"msp_growth", "msp_growth"},
		{"custom_plan", "custom_plan"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			if got := CanonicalizePlanVersion(tt.raw); got != tt.want {
				t.Fatalf("CanonicalizePlanVersion(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
