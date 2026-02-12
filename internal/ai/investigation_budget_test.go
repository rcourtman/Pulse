package ai

import (
	"testing"
)

func TestInvestigationBudget_BasicConsumption(t *testing.T) {
	b := NewInvestigationBudget(3)

	if r := b.Remaining(); r != 3 {
		t.Fatalf("remaining=%d, want 3", r)
	}

	if !b.TryConsume() {
		t.Fatal("expected first consume to succeed")
	}
	if r := b.Remaining(); r != 2 {
		t.Fatalf("remaining=%d, want 2", r)
	}

	if !b.TryConsume() {
		t.Fatal("expected second consume to succeed")
	}
	if !b.TryConsume() {
		t.Fatal("expected third consume to succeed")
	}

	if r := b.Remaining(); r != 0 {
		t.Fatalf("remaining=%d, want 0", r)
	}

	if b.TryConsume() {
		t.Fatal("expected fourth consume to fail (budget exhausted)")
	}
}

func TestInvestigationBudget_MonthlyReset(t *testing.T) {
	b := NewInvestigationBudget(2)

	if !b.TryConsume() {
		t.Fatal("expected first consume to succeed")
	}
	if !b.TryConsume() {
		t.Fatal("expected second consume to succeed")
	}
	if b.TryConsume() {
		t.Fatal("expected third consume to fail")
	}

	// Simulate month change by manipulating internal state.
	b.resetMonth = b.resetMonth - 1
	if b.resetMonth == 0 {
		b.resetMonth = 12
		b.resetYear--
	}

	// After month change, budget should reset.
	if r := b.Remaining(); r != 2 {
		t.Fatalf("remaining after month reset=%d, want 2", r)
	}
	if !b.TryConsume() {
		t.Fatal("expected consume after month reset to succeed")
	}
}
