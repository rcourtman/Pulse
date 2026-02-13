package hostagent

import (
	"testing"
	"time"
)

func TestComputeReconnectDelayExponentialAndCapped(t *testing.T) {
	origBase := reconnectDelay
	origMax := reconnectMaxDelay
	origJitter := reconnectJitterRatio
	origRand := reconnectRandFloat64
	t.Cleanup(func() {
		reconnectDelay = origBase
		reconnectMaxDelay = origMax
		reconnectJitterRatio = origJitter
		reconnectRandFloat64 = origRand
	})

	reconnectDelay = 100 * time.Millisecond
	reconnectMaxDelay = 800 * time.Millisecond
	reconnectJitterRatio = 0
	reconnectRandFloat64 = func() float64 { return 0.5 }

	cases := []struct {
		failures int
		want     time.Duration
	}{
		{failures: 0, want: 100 * time.Millisecond},
		{failures: 1, want: 100 * time.Millisecond},
		{failures: 2, want: 200 * time.Millisecond},
		{failures: 3, want: 400 * time.Millisecond},
		{failures: 4, want: 800 * time.Millisecond},
		{failures: 8, want: 800 * time.Millisecond},
	}

	for _, tc := range cases {
		got := computeReconnectDelay(tc.failures)
		if got != tc.want {
			t.Fatalf("computeReconnectDelay(%d) = %v, want %v", tc.failures, got, tc.want)
		}
	}
}

func TestComputeReconnectDelayJitterRange(t *testing.T) {
	origBase := reconnectDelay
	origMax := reconnectMaxDelay
	origJitter := reconnectJitterRatio
	origRand := reconnectRandFloat64
	t.Cleanup(func() {
		reconnectDelay = origBase
		reconnectMaxDelay = origMax
		reconnectJitterRatio = origJitter
		reconnectRandFloat64 = origRand
	})

	reconnectDelay = time.Second
	reconnectMaxDelay = 0
	reconnectJitterRatio = 0.5

	reconnectRandFloat64 = func() float64 { return 0.0 } // -50%
	if got := computeReconnectDelay(1); got != 500*time.Millisecond {
		t.Fatalf("min jitter delay = %v, want %v", got, 500*time.Millisecond)
	}

	reconnectRandFloat64 = func() float64 { return 1.0 } // +50%
	if got := computeReconnectDelay(1); got != 1500*time.Millisecond {
		t.Fatalf("max jitter delay = %v, want %v", got, 1500*time.Millisecond)
	}
}

func TestComputeReconnectDelayZeroBase(t *testing.T) {
	origBase := reconnectDelay
	t.Cleanup(func() { reconnectDelay = origBase })

	reconnectDelay = 0
	if got := computeReconnectDelay(3); got != 0 {
		t.Fatalf("computeReconnectDelay with zero base = %v, want 0", got)
	}
}
