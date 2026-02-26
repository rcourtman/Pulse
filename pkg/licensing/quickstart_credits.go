package licensing

import "time"

// QuickstartCreditsTotal is the number of free hosted Patrol runs granted
// to every new workspace.
const QuickstartCreditsTotal = 25

// QuickstartCreditsRemaining returns the number of unused quickstart credits.
// Returns 0 if credits were never granted.
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

// GrantQuickstartCredits marks quickstart credits as granted (idempotent).
// Returns true if credits were newly granted, false if already granted.
func (b *BillingState) GrantQuickstartCredits() bool {
	if b.QuickstartCreditsGranted {
		return false
	}
	b.QuickstartCreditsGranted = true
	now := time.Now().Unix()
	b.QuickstartCreditsGrantedAt = &now
	return true
}

// ConsumeQuickstartCredit decrements one quickstart credit.
// Returns false if no credits remain.
func (b *BillingState) ConsumeQuickstartCredit() bool {
	if !b.HasQuickstartCredits() {
		return false
	}
	b.QuickstartCreditsUsed++
	return true
}
