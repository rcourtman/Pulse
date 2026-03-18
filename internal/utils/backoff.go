package utils

import "time"

const maxDuration = time.Duration(1<<63 - 1)

// ExponentialBackoff computes an exponential retry delay with optional cap and jitter.
//
// Failures is 1-based: failures=1 returns baseDelay before jitter.
// If maxDelay > 0, the base delay is capped before jitter is applied.
// jitterRatio should be in [0,1]; values outside this range are clamped.
// randFloat64 should return values in [0,1]; out-of-range values are clamped.
func ExponentialBackoff(
	baseDelay time.Duration,
	maxDelay time.Duration,
	failures int,
	jitterRatio float64,
	randFloat64 func() float64,
) time.Duration {
	if baseDelay <= 0 {
		return 0
	}
	if failures < 1 {
		failures = 1
	}

	doublings := failures - 1
	if doublings > 62 {
		doublings = 62
	}

	delay := baseDelay
	for i := 0; i < doublings; i++ {
		if maxDelay > 0 && delay >= maxDelay {
			delay = maxDelay
			break
		}

		if delay > maxDuration/2 {
			delay = maxDuration
			break
		}

		delay *= 2

		if maxDelay > 0 && delay >= maxDelay {
			delay = maxDelay
			break
		}
	}

	// Cap after the loop in case baseDelay already exceeds maxDelay
	// (the loop body is skipped entirely when failures=1).
	if maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}

	if jitterRatio <= 0 {
		return delay
	}
	if jitterRatio > 1 {
		jitterRatio = 1
	}

	if randFloat64 == nil {
		randFloat64 = func() float64 { return 0.5 }
	}
	r := randFloat64()
	if r < 0 {
		r = 0
	} else if r > 1 {
		r = 1
	}

	jitter := time.Duration(float64(delay) * jitterRatio * (r*2 - 1))
	if delay+jitter < 0 {
		return 0
	}
	return delay + jitter
}
