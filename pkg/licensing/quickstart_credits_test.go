package licensing

import "testing"

func TestQuickstartCreditsRemaining_NotGranted(t *testing.T) {
	bs := &BillingState{}
	if got := bs.QuickstartCreditsRemaining(); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestQuickstartCreditsRemaining_FullCredits(t *testing.T) {
	bs := &BillingState{QuickstartCreditsGranted: true}
	if got := bs.QuickstartCreditsRemaining(); got != QuickstartCreditsTotal {
		t.Errorf("expected %d, got %d", QuickstartCreditsTotal, got)
	}
}

func TestQuickstartCreditsRemaining_PartiallyUsed(t *testing.T) {
	bs := &BillingState{QuickstartCreditsGranted: true, QuickstartCreditsUsed: 10}
	if got := bs.QuickstartCreditsRemaining(); got != 15 {
		t.Errorf("expected 15, got %d", got)
	}
}

func TestQuickstartCreditsRemaining_Exhausted(t *testing.T) {
	bs := &BillingState{QuickstartCreditsGranted: true, QuickstartCreditsUsed: 25}
	if got := bs.QuickstartCreditsRemaining(); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestQuickstartCreditsRemaining_OverUsed(t *testing.T) {
	bs := &BillingState{QuickstartCreditsGranted: true, QuickstartCreditsUsed: 30}
	if got := bs.QuickstartCreditsRemaining(); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestQuickstartCreditsRemaining_NilState(t *testing.T) {
	var bs *BillingState
	if got := bs.QuickstartCreditsRemaining(); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestHasQuickstartCredits(t *testing.T) {
	tests := []struct {
		name     string
		state    *BillingState
		expected bool
	}{
		{"not granted", &BillingState{}, false},
		{"granted, unused", &BillingState{QuickstartCreditsGranted: true}, true},
		{"exhausted", &BillingState{QuickstartCreditsGranted: true, QuickstartCreditsUsed: 25}, false},
		{"partial", &BillingState{QuickstartCreditsGranted: true, QuickstartCreditsUsed: 10}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.HasQuickstartCredits(); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestGrantQuickstartCredits_Idempotent(t *testing.T) {
	bs := &BillingState{}
	if !bs.GrantQuickstartCredits() {
		t.Fatal("first grant should return true")
	}
	if !bs.QuickstartCreditsGranted {
		t.Fatal("should be granted")
	}
	if bs.QuickstartCreditsGrantedAt == nil {
		t.Fatal("granted_at should be set")
	}
	firstGrantedAt := *bs.QuickstartCreditsGrantedAt

	// Second grant should be idempotent
	if bs.GrantQuickstartCredits() {
		t.Fatal("second grant should return false")
	}
	if *bs.QuickstartCreditsGrantedAt != firstGrantedAt {
		t.Fatal("granted_at should not change on second grant")
	}
}

func TestConsumeQuickstartCredit(t *testing.T) {
	bs := &BillingState{QuickstartCreditsGranted: true}

	// Consume all credits
	for i := 0; i < QuickstartCreditsTotal; i++ {
		if !bs.ConsumeQuickstartCredit() {
			t.Fatalf("consume at iteration %d should succeed", i)
		}
	}

	// Next consume should fail
	if bs.ConsumeQuickstartCredit() {
		t.Fatal("consume after exhaustion should return false")
	}

	if bs.QuickstartCreditsUsed != QuickstartCreditsTotal {
		t.Errorf("expected %d used, got %d", QuickstartCreditsTotal, bs.QuickstartCreditsUsed)
	}
}

func TestConsumeQuickstartCredit_NotGranted(t *testing.T) {
	bs := &BillingState{}
	if bs.ConsumeQuickstartCredit() {
		t.Fatal("consume without grant should return false")
	}
}
