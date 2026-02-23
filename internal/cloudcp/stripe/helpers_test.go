package stripe

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

func TestMapSubscriptionStatus(t *testing.T) {
	tests := []struct {
		input string
		want  entitlements.SubscriptionState
	}{
		{"active", entitlements.SubStateActive},
		{"Active", entitlements.SubStateActive},
		{"trialing", entitlements.SubStateTrial},
		{"past_due", entitlements.SubStateGrace},
		{"unpaid", entitlements.SubStateGrace},
		{"canceled", entitlements.SubStateCanceled},
		{"paused", entitlements.SubStateSuspended},
		{"incomplete", entitlements.SubStateExpired},
		{"incomplete_expired", entitlements.SubStateExpired},
		{"unknown_status", entitlements.SubStateExpired},
		{"", entitlements.SubStateExpired},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MapSubscriptionStatus(tt.input)
			if got != tt.want {
				t.Errorf("MapSubscriptionStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShouldGrantCapabilities(t *testing.T) {
	tests := []struct {
		state entitlements.SubscriptionState
		want  bool
	}{
		{entitlements.SubStateActive, true},
		{entitlements.SubStateTrial, true},
		{entitlements.SubStateGrace, true},
		{entitlements.SubStateCanceled, false},
		{entitlements.SubStateSuspended, false},
		{entitlements.SubStateExpired, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := ShouldGrantCapabilities(tt.state)
			if got != tt.want {
				t.Errorf("ShouldGrantCapabilities(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestDerivePlanVersion(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		priceID  string
		want     string
	}{
		{"plan_version in metadata", map[string]string{"plan_version": "v2"}, "", "v2"},
		{"plan in metadata", map[string]string{"plan": "pro"}, "", "pro"},
		{"plan_version takes priority", map[string]string{"plan_version": "v3", "plan": "pro"}, "", "v3"},
		{"price ID fallback", nil, "price_123", "stripe_price:price_123"},
		{"generic fallback", nil, "", "stripe"},
		{"nil metadata with price", nil, "price_abc", "stripe_price:price_abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DerivePlanVersion(tt.metadata, tt.priceID)
			if got != tt.want {
				t.Errorf("DerivePlanVersion = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsSafeStripeID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"cus_test123", true},
		{"sub_abc-def", true},
		{"evt_12345678901234567890", true},
		{"", false},
		{"ab", false},
		{"cus_../etc/passwd", false},
		{"cus test", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := IsSafeStripeID(tt.id)
			if got != tt.want {
				t.Errorf("IsSafeStripeID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestRedactMagicLinkURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "removes query token",
			in:   "https://tenant.cloud.example.com/auth/magic-link/verify?token=abc123&foo=bar",
			want: "https://tenant.cloud.example.com/auth/magic-link/verify",
		},
		{
			name: "invalid URL returns empty",
			in:   "not a url",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactMagicLinkURL(tt.in)
			if got != tt.want {
				t.Fatalf("redactMagicLinkURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
