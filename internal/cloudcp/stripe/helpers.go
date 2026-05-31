package stripe

import (
	"strings"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

const (
	CheckoutBillingModeMetadataKey = "checkout_billing_mode"
	CheckoutBillingModeTrial       = "trial"
	CheckoutBillingModeImmediate   = "immediate"
)

// MapSubscriptionStatus converts a Stripe subscription status string to the
// internal SubscriptionState. Unknown statuses fail closed (expired).
func MapSubscriptionStatus(status string) pkglicensing.SubscriptionState {
	return pkglicensing.MapStripeSubscriptionStatusToState(status)
}

// ShouldGrantCapabilities returns true if the subscription state warrants
// granting paid capabilities.
func ShouldGrantCapabilities(state pkglicensing.SubscriptionState) bool {
	return pkglicensing.ShouldGrantPaidCapabilities(state)
}

// DerivePlanVersion extracts a plan version from event metadata, falling back
// to a Stripe price ID prefix or a generic "stripe" string.
func DerivePlanVersion(metadata map[string]string, priceID string) string {
	return pkglicensing.DeriveStripePlanVersion(metadata, priceID)
}

// InitialSubscriptionStateForCheckout resolves the initial stored billing
// state for a checkout-created account before subscription webhooks catch up.
func InitialSubscriptionStateForCheckout(metadata map[string]string) string {
	if strings.EqualFold(strings.TrimSpace(metadata[CheckoutBillingModeMetadataKey]), CheckoutBillingModeImmediate) {
		return "active"
	}
	return "trial"
}

// IsSafeStripeID validates that a Stripe ID (cus_..., sub_...) is safe for
// use as a lookup key. Keeps the check strict to avoid filesystem surprises.
func IsSafeStripeID(stripeID string) bool {
	if len(stripeID) < 5 || len(stripeID) > 128 {
		return false
	}
	for i := 0; i < len(stripeID); i++ {
		c := stripeID[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}
