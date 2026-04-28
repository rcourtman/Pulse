package licensing

// QuickstartCreditsTotal is retained only for historical billing-state
// compatibility. New v6 workspaces must not mint hosted-AI quickstart credits.
const QuickstartCreditsTotal = 25

// QuickstartCreditsRemaining returns the historical unused quickstart credits
// recorded in old billing state. It is read-only compatibility, not an active
// entitlement source.
func (b *BillingState) QuickstartCreditsRemaining() int {
	if b == nil || !b.QuickstartCreditsGranted {
		return 0
	}
	remaining := QuickstartCreditsTotal - b.QuickstartCreditsUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// HasQuickstartCredits returns true if the workspace has unused quickstart credits.
func (b *BillingState) HasQuickstartCredits() bool {
	return b.QuickstartCreditsRemaining() > 0
}
