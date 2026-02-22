package licensing

import "strings"

func MapStripeSubscriptionStatusToState(status string) SubscriptionState {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return SubStateActive
	case "trialing":
		return SubStateTrial
	case "past_due", "unpaid":
		return SubStateGrace
	case "canceled":
		return SubStateCanceled
	case "paused":
		return SubStateSuspended
	case "incomplete", "incomplete_expired":
		return SubStateExpired
	default:
		// Fail closed: unknown status should not grant paid capabilities.
		return SubStateExpired
	}
}

func ShouldGrantPaidCapabilities(state SubscriptionState) bool {
	switch state {
	case SubStateActive, SubStateTrial, SubStateGrace:
		return true
	default:
		return false
	}
}

func DeriveStripePlanVersion(metadata map[string]string, priceID string) string {
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
