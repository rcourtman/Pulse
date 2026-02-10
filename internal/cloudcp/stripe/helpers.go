package stripe

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

// MapSubscriptionStatus converts a Stripe subscription status string to the
// internal SubscriptionState. Unknown statuses fail closed (expired).
func MapSubscriptionStatus(status string) entitlements.SubscriptionState {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return entitlements.SubStateActive
	case "trialing":
		return entitlements.SubStateTrial
	case "past_due", "unpaid":
		return entitlements.SubStateGrace
	case "canceled":
		return entitlements.SubStateCanceled
	case "paused":
		return entitlements.SubStateSuspended
	case "incomplete", "incomplete_expired":
		return entitlements.SubStateExpired
	default:
		return entitlements.SubStateExpired
	}
}

// ShouldGrantCapabilities returns true if the subscription state warrants
// granting paid capabilities.
func ShouldGrantCapabilities(state entitlements.SubscriptionState) bool {
	switch state {
	case entitlements.SubStateActive, entitlements.SubStateTrial, entitlements.SubStateGrace:
		return true
	default:
		return false
	}
}

// DerivePlanVersion extracts a plan version from event metadata, falling back
// to a Stripe price ID prefix or a generic "stripe" string.
func DerivePlanVersion(metadata map[string]string, priceID string) string {
	if metadata != nil {
		if v := strings.TrimSpace(metadata["plan_version"]); v != "" {
			return v
		}
		if v := strings.TrimSpace(metadata["plan"]); v != "" {
			return v
		}
	}
	if strings.TrimSpace(priceID) != "" {
		return "stripe_price:" + strings.TrimSpace(priceID)
	}
	return "stripe"
}

// IsSafeStripeID validates that a Stripe ID (cus_..., sub_...) is safe for
// use as a lookup key. Keeps the check strict to avoid filesystem surprises.
func IsSafeStripeID(id string) bool {
	if len(id) < 5 || len(id) > 128 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}
