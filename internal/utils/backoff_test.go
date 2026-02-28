package utils

import (
	"testing"
	"time"
)

func TestExponentialBackoff_BasicAndCapped(t *testing.T) {
	tests := []struct {
		name     string
		failures int
		want     time.Duration
	}{
		{name: "first failure uses base", failures: 1, want: 5 * time.Second},
		{name: "zero failure treated as first", failures: 0, want: 5 * time.Second},
		{name: "second failure doubles", failures: 2, want: 10 * time.Second},
		{name: "third failure quadruples", failures: 3, want: 20 * time.Second},
		{name: "cap applied", failures: 99, want: 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExponentialBackoff(5*time.Second, 5*time.Minute, tt.failures, 0, func() float64 { return 0.5 })
			if got != tt.want {
				t.Fatalf("ExponentialBackoff(..., failures=%d) = %v, want %v", tt.failures, got, tt.want)
			}
		})
	}
}

func TestExponentialBackoff_DoesNotOverflowForHugeFailureCounts(t *testing.T) {
	got := ExponentialBackoff(5*time.Second, 5*time.Minute, 845204966, 0, func() float64 { return 0.5 })
	if got != 5*time.Minute {
		t.Fatalf("huge failure count backoff = %v, want %v", got, 5*time.Minute)
	}
}

func TestExponentialBackoff_JitterBounds(t *testing.T) {
	base := 10 * time.Second
	max := 0 * time.Second
	jitter := 0.1

	min := ExponentialBackoff(base, max, 1, jitter, func() float64 { return 0.0 })
	if min != 9*time.Second {
		t.Fatalf("min jitter delay = %v, want %v", min, 9*time.Second)
	}

	maxJ := ExponentialBackoff(base, max, 1, jitter, func() float64 { return 1.0 })
	if maxJ != 11*time.Second {
		t.Fatalf("max jitter delay = %v, want %v", maxJ, 11*time.Second)
	}
}

func TestExponentialBackoff_ZeroBaseReturnsZero(t *testing.T) {
	if got := ExponentialBackoff(0, time.Minute, 3, 0.1, func() float64 { return 0.5 }); got != 0 {
		t.Fatalf("zero base delay = %v, want 0", got)
	}
}

func TestExponentialBackoff_BaseExceedsMax_CappedOnFirstFailure(t *testing.T) {
	// When baseDelay > maxDelay and failures=1, the loop body is skipped
	// (doublings=0). The post-loop cap must still enforce maxDelay.
	got := ExponentialBackoff(10*time.Minute, 5*time.Minute, 1, 0, func() float64 { return 0.5 })
	if got != 5*time.Minute {
		t.Fatalf("ExponentialBackoff(base=10m, max=5m, failures=1) = %v, want 5m", got)
	}
}
