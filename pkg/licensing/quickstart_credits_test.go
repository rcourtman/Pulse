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
