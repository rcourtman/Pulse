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

// ParseStoredSubscriptionState converts the normalized state stored in
// stripe_accounts.subscription_state back into a SubscriptionState. The stored
// form ("active", "trial", "past_due", "canceled") is distinct from the raw
// Stripe status form ("trialing", "incomplete", ...) accepted by
// MapStripeSubscriptionStatusToState.
func ParseStoredSubscriptionState(stored string) SubscriptionState {
	switch strings.ToLower(strings.TrimSpace(stored)) {
	case "active":
		return SubStateActive
	case "trial":
		return SubStateTrial
	case "past_due":
		return SubStateGrace
	case "canceled":
		return SubStateCanceled
	case "grace":
		return SubStateGrace
	case "suspended":
		return SubStateSuspended
	case "expired":
		return SubStateExpired
	default:
		// Fail closed: unknown stored values must not grant paid capabilities.
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
			return CanonicalizePlanVersion(v)
		}
		if v := strings.TrimSpace(metadata["plan"]); v != "" {
			return CanonicalizePlanVersion(v)
		}
		if v := strings.TrimSpace(metadata["price_id"]); v != "" {
			priceID = v
		} else if v := strings.TrimSpace(metadata["plan_id"]); v != "" {
			priceID = v
		}
	}
	trimmedPrice := strings.TrimSpace(priceID)
	if trimmedPrice != "" {
		// Try canonical price→plan lookup before falling back to opaque prefix.
		if plan, ok := PlanVersionForPriceID(trimmedPrice); ok {
			return plan
		}
		return "stripe_price:" + trimmedPrice
	}
	return "stripe"
}

func CanonicalizePlanVersion(raw string) string {
	trimmed := strings.TrimSpace(raw)
	switch strings.ToLower(trimmed) {
	case "cloud", "starter", "cloud-v1", "cloud_v1", "cloud-starter", "cloud_starter":
		return "cloud_starter"
	case "msp", "msp-hosted-v1", "msp_hosted_v1", "msp-starter", "msp_starter":
		return "msp_starter"
	case "power", "cloud-power", "cloud_power":
		return "cloud_power"
	case "max", "cloud-max", "cloud_max":
		return "cloud_max"
	case "founding", "cloud-founding", "cloud_founding":
		return "cloud_founding"
	default:
		return trimmed
	}
}
