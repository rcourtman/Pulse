package monitoring

import (
	"testing"
	"time"
)

func TestBackoffConfig_NextDelay(t *testing.T) {
	tests := []struct {
		name       string
		config     backoffConfig
		attempt    int
		rng        float64
		wantMin    time.Duration
		wantMax    time.Duration
		wantExact  time.Duration // for tests without jitter
		checkExact bool
	}{
		{
			name: "first attempt with defaults",
			config: backoffConfig{
				Initial:    5 * time.Second,
				Multiplier: 2,
				Jitter:     0,
				Max:        5 * time.Minute,
			},
			attempt:    0,
			rng:        0.5,
			wantExact:  5 * time.Second,
			checkExact: true,
		},
		{
			name: "second attempt doubles delay",
			config: backoffConfig{
				Initial:    5 * time.Second,
				Multiplier: 2,
				Jitter:     0,
				Max:        5 * time.Minute,
			},
			attempt:    1,
			rng:        0.5,
			wantExact:  10 * time.Second,
			checkExact: true,
		},
		{
			name: "third attempt quadruples delay",
			config: backoffConfig{
				Initial:    5 * time.Second,
				Multiplier: 2,
				Jitter:     0,
				Max:        5 * time.Minute,
			},
			attempt:    2,
			rng:        0.5,
			wantExact:  20 * time.Second,
			checkExact: true,
		},
		{
			name: "respects max delay cap",
			config: backoffConfig{
				Initial:    5 * time.Second,
				Multiplier: 2,
				Jitter:     0,
				Max:        30 * time.Second,
			},
			attempt:    10, // would be 5120 seconds without cap
			rng:        0.5,
			wantExact:  30 * time.Second,
			checkExact: true,
		},
		{
			name: "jitter adds randomness within bounds",
			config: backoffConfig{
				Initial:    10 * time.Second,
				Multiplier: 2,
				Jitter:     0.2, // ±20%
				Max:        5 * time.Minute,
			},
			attempt: 0,
			rng:     0.5, // neutral jitter
			wantMin: 8 * time.Second,  // 10s * (1 - 0.2)
			wantMax: 12 * time.Second, // 10s * (1 + 0.2)
		},
		{
			name: "jitter at max (rng=1.0) increases delay",
			config: backoffConfig{
				Initial:    10 * time.Second,
				Multiplier: 2,
				Jitter:     0.2,
				Max:        5 * time.Minute,
			},
			attempt:    0,
			rng:        1.0,
			wantExact:  12 * time.Second, // 10s * (1 + 0.2)
			checkExact: true,
		},
		{
			name: "jitter at min (rng=0.0) decreases delay",
			config: backoffConfig{
				Initial:    10 * time.Second,
				Multiplier: 2,
				Jitter:     0.2,
				Max:        5 * time.Minute,
			},
			attempt:    0,
			rng:        0.0,
			wantExact:  8 * time.Second, // 10s * (1 - 0.2)
			checkExact: true,
		},
		{
			name: "negative attempt treated as zero",
			config: backoffConfig{
				Initial:    5 * time.Second,
				Multiplier: 2,
				Jitter:     0,
				Max:        5 * time.Minute,
			},
			attempt:    -1,
			rng:        0.5,
			wantExact:  5 * time.Second,
			checkExact: true,
		},
		{
			name: "zero initial uses default 2s",
			config: backoffConfig{
				Initial:    0,
				Multiplier: 2,
				Jitter:     0,
				Max:        5 * time.Minute,
			},
			attempt:    0,
			rng:        0.5,
			wantExact:  2 * time.Second,
			checkExact: true,
		},
		{
			name: "multiplier <= 1 defaults to 2",
			config: backoffConfig{
				Initial:    5 * time.Second,
				Multiplier: 1,
				Jitter:     0,
				Max:        5 * time.Minute,
			},
			attempt:    1,
			rng:        0.5,
			wantExact:  10 * time.Second, // uses multiplier of 2
			checkExact: true,
		},
		{
			name: "jitter > 1 capped at 1",
			config: backoffConfig{
				Initial:    10 * time.Second,
				Multiplier: 2,
				Jitter:     2.0, // should be capped at 1.0
				Max:        5 * time.Minute,
			},
			attempt: 0,
			rng:     0.5,
			wantMin: 0 * time.Second,  // 10s * (1 - 1.0)
			wantMax: 20 * time.Second, // 10s * (1 + 1.0)
		},
		{
			name: "realistic production config",
			config: backoffConfig{
				Initial:    5 * time.Second,
				Multiplier: 2,
				Jitter:     0.2,
				Max:        5 * time.Minute,
			},
			attempt: 5, // 160s base
			rng:     0.5,
			wantMin: 128 * time.Second, // 160s * 0.8
			wantMax: 192 * time.Second, // 160s * 1.2
		},
		{
			name: "max delay applies after jitter",
			config: backoffConfig{
				Initial:    10 * time.Second,
				Multiplier: 2,
				Jitter:     0.2,
				Max:        15 * time.Second,
			},
			attempt: 2, // base would be 40s
			rng:     1.0,
			// 40s * 1.2 = 48s, but capped at 15s
			wantExact:  15 * time.Second,
			checkExact: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.nextDelay(tt.attempt, tt.rng)

			if tt.checkExact {
				if got != tt.wantExact {
					t.Errorf("nextDelay() = %v, want %v", got, tt.wantExact)
				}
			} else {
				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("nextDelay() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

func TestBackoffConfig_ExponentialGrowth(t *testing.T) {
	cfg := backoffConfig{
		Initial:    5 * time.Second,
		Multiplier: 2,
		Jitter:     0,
		Max:        10 * time.Minute,
	}

	expected := []time.Duration{
		5 * time.Second,   // attempt 0
		10 * time.Second,  // attempt 1
		20 * time.Second,  // attempt 2
		40 * time.Second,  // attempt 3
		80 * time.Second,  // attempt 4
		160 * time.Second, // attempt 5
		320 * time.Second, // attempt 6
		10 * time.Minute,  // attempt 7 (capped)
		10 * time.Minute,  // attempt 8 (capped)
	}

	for i, want := range expected {
		got := cfg.nextDelay(i, 0.5)
		if got != want {
			t.Errorf("attempt %d: got %v, want %v", i, got, want)
		}
	}
}

func TestBackoffConfig_JitterDistribution(t *testing.T) {
	cfg := backoffConfig{
		Initial:    10 * time.Second,
		Multiplier: 2,
		Jitter:     0.2,
		Max:        5 * time.Minute,
	}

	// Test that different RNG values produce different delays
	results := make(map[time.Duration]bool)
	rngValues := []float64{0.0, 0.25, 0.5, 0.75, 1.0}

	for _, rng := range rngValues {
		delay := cfg.nextDelay(0, rng)
		results[delay] = true
	}

	// Should have multiple distinct values due to jitter
	if len(results) < 3 {
		t.Errorf("jitter not producing enough variation: got %d unique values, want at least 3", len(results))
	}

	// All results should be within expected bounds
	for delay := range results {
		if delay < 8*time.Second || delay > 12*time.Second {
			t.Errorf("delay %v outside expected jitter range [8s, 12s]", delay)
		}
	}
}
