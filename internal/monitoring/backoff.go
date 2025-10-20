package monitoring

import (
	"math"
	"time"
)

type backoffConfig struct {
	Initial    time.Duration
	Multiplier float64
	Jitter     float64
	Max        time.Duration
}

func (cfg backoffConfig) nextDelay(attempt int, rng float64) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	base := float64(cfg.Initial)
	if base <= 0 {
		base = float64(2 * time.Second)
	}
	multiplier := cfg.Multiplier
	if multiplier <= 1 {
		multiplier = 2
	}
	delay := base * math.Pow(multiplier, float64(attempt))
	if cfg.Jitter > 0 {
		j := cfg.Jitter
		if j > 1 {
			j = 1
		}
		delay = delay * (1 + (rng*2-1)*j)
	}
	if cfg.Max > 0 && delay > float64(cfg.Max) {
		delay = float64(cfg.Max)
	}
	return time.Duration(delay)
}
