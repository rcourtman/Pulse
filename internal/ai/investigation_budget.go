package ai

import (
	"sync"
	"time"
)

// InvestigationBudget tracks per-month investigation allowance for free-tier users.
// Community users get a small number of investigations per month at "approval" autonomy
// to form the habit of using Pulse's investigation features (with BYOK, Pulse pays nothing).
type InvestigationBudget struct {
	mu           sync.Mutex
	monthlyLimit int
	used         int
	resetMonth   time.Month
	resetYear    int
}

// NewInvestigationBudget creates a budget with the given monthly limit.
func NewInvestigationBudget(monthlyLimit int) *InvestigationBudget {
	now := time.Now()
	return &InvestigationBudget{
		monthlyLimit: monthlyLimit,
		resetMonth:   now.Month(),
		resetYear:    now.Year(),
	}
}

// TryConsume attempts to consume one investigation from the budget.
// Returns true if budget was available and consumed, false if exhausted.
// Automatically resets the budget when the calendar month changes.
func (b *InvestigationBudget) TryConsume() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if now.Month() != b.resetMonth || now.Year() != b.resetYear {
		b.used = 0
		b.resetMonth = now.Month()
		b.resetYear = now.Year()
	}

	if b.used >= b.monthlyLimit {
		return false
	}
	b.used++
	return true
}

// Remaining returns the number of investigations left this month.
func (b *InvestigationBudget) Remaining() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if now.Month() != b.resetMonth || now.Year() != b.resetYear {
		return b.monthlyLimit
	}

	remaining := b.monthlyLimit - b.used
	if remaining < 0 {
		return 0
	}
	return remaining
}
