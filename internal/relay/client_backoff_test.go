package relay

import (
	"testing"
	"time"
)

func TestBackoffDelay_ExponentialProgression(t *testing.T) {
	c := &Client{} // backoffDelay only uses package-level constants

	// With jitter, exact values vary. Verify the progression stays within
	// expected bounds: baseReconnectDelay * 2^(failures-1) ± jitter, capped
	// at maxReconnectDelay.
	cases := []struct {
		failures int
		wantMin  time.Duration
		wantMax  time.Duration
	}{
		// failures=1 → base=5s ± 10% → [4.5s, 5.5s]
		{failures: 1, wantMin: 4500 * time.Millisecond, wantMax: 5500 * time.Millisecond},
		// failures=2 → base=10s ± 10% → [9s, 11s]
		{failures: 2, wantMin: 9 * time.Second, wantMax: 11 * time.Second},
		// failures=3 → base=20s ± 10% → [18s, 22s]
		{failures: 3, wantMin: 18 * time.Second, wantMax: 22 * time.Second},
		// failures=4 → base=40s ± 10% → [36s, 44s]
		{failures: 4, wantMin: 36 * time.Second, wantMax: 44 * time.Second},
		// failures=7 → base=320s=5m20s → capped at 5m, then ± 10% → [4m30s, 5m30s]
		{failures: 7, wantMin: 270 * time.Second, wantMax: 330 * time.Second},
		// failures=99 → capped at 5m ± 10% → [4m30s, 5m30s]
		{failures: 99, wantMin: 270 * time.Second, wantMax: 330 * time.Second},
	}

	for _, tc := range cases {
		// Sample multiple times to account for randomized jitter
		for i := 0; i < 20; i++ {
			got := c.backoffDelay(tc.failures)
			if got < tc.wantMin || got > tc.wantMax {
				t.Fatalf("backoffDelay(%d) = %v, want [%v, %v]", tc.failures, got, tc.wantMin, tc.wantMax)
			}
		}
	}
}

func TestBackoffDelay_ZeroAndNegativeFailures(t *testing.T) {
	c := &Client{}

	// failures ≤ 0 is treated as failures=1 by ExponentialBackoff
	for _, failures := range []int{0, -1} {
		got := c.backoffDelay(failures)
		if got < 4500*time.Millisecond || got > 5500*time.Millisecond {
			t.Fatalf("backoffDelay(%d) = %v, want within [4.5s, 5.5s]", failures, got)
		}
	}
}

func TestBackoffDelay_AlwaysPositive(t *testing.T) {
	c := &Client{}

	// Verify the delay is always positive for any reasonable failure count.
	// This validates the safety check in Run() (lines 176-182) shouldn't
	// normally trigger with these constants.
	for failures := 0; failures <= 100; failures++ {
		for i := 0; i < 5; i++ {
			got := c.backoffDelay(failures)
			if got <= 0 {
				t.Fatalf("backoffDelay(%d) = %v, expected positive delay", failures, got)
			}
		}
	}
}

func TestBackoffDelay_CappedAtMax(t *testing.T) {
	c := &Client{}

	// Even at extreme failure counts, delay should not exceed
	// maxReconnectDelay + jitter (maxReconnectDelay * reconnectJitter)
	maxWithJitter := maxReconnectDelay + time.Duration(float64(maxReconnectDelay)*reconnectJitter)

	for _, failures := range []int{10, 50, 100, 1000} {
		for i := 0; i < 20; i++ {
			got := c.backoffDelay(failures)
			if got > maxWithJitter {
				t.Fatalf("backoffDelay(%d) = %v, exceeds max+jitter %v", failures, got, maxWithJitter)
			}
		}
	}
}

func TestNextConsecutiveFailures_StreakBehavior(t *testing.T) {
	// A connected=true session resets the streak to 1 (not 0) because
	// the current disconnection itself is a failure.
	if got := nextConsecutiveFailures(10, true); got != 1 {
		t.Fatalf("nextConsecutiveFailures(10, true) = %d, want 1", got)
	}

	// A failed connection attempt (never registered) increments the streak.
	if got := nextConsecutiveFailures(3, false); got != 4 {
		t.Fatalf("nextConsecutiveFailures(3, false) = %d, want 4", got)
	}

	// First failure from zero starts at 1.
	if got := nextConsecutiveFailures(0, false); got != 1 {
		t.Fatalf("nextConsecutiveFailures(0, false) = %d, want 1", got)
	}
}
