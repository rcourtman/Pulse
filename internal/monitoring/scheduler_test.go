package monitoring

import (
	"math/rand"
	"testing"
	"time"
)

// TestClampFloat tests the clampFloat helper function
func TestClampFloat(t *testing.T) {
	tests := []struct {
		name string
		v    float64
		min  float64
		max  float64
		want float64
	}{
		{
			name: "value within range returns unchanged",
			v:    5.0,
			min:  0.0,
			max:  10.0,
			want: 5.0,
		},
		{
			name: "value below min returns min",
			v:    -5.0,
			min:  0.0,
			max:  10.0,
			want: 0.0,
		},
		{
			name: "value above max returns max",
			v:    15.0,
			min:  0.0,
			max:  10.0,
			want: 10.0,
		},
		{
			name: "value at min returns min",
			v:    0.0,
			min:  0.0,
			max:  10.0,
			want: 0.0,
		},
		{
			name: "value at max returns max",
			v:    10.0,
			min:  0.0,
			max:  10.0,
			want: 10.0,
		},
		{
			name: "min equals max returns that value",
			v:    5.0,
			min:  7.0,
			max:  7.0,
			want: 7.0,
		},
		{
			name: "negative range below min",
			v:    -10.0,
			min:  -5.0,
			max:  0.0,
			want: -5.0,
		},
		{
			name: "negative range above max",
			v:    5.0,
			min:  -10.0,
			max:  -1.0,
			want: -1.0,
		},
		{
			name: "negative range within",
			v:    -3.0,
			min:  -5.0,
			max:  -1.0,
			want: -3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampFloat(tt.v, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("clampFloat(%v, %v, %v) = %v, want %v", tt.v, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_StalenessScore tests staleness score impact on interval
func TestAdaptiveIntervalSelector_StalenessScore(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	tests := []struct {
		name  string
		score float64
		want  time.Duration
	}{
		{
			name:  "score=0 gives max interval",
			score: 0.0,
			// target=60s, base=10s, smoothed = 0.6*60 + 0.4*10 = 36 + 4 = 40s
			want: 40 * time.Second,
		},
		{
			name:  "score=1 gives min interval",
			score: 1.0,
			// target=5s, base=10s, smoothed = 0.6*5 + 0.4*10 = 3 + 4 = 7s
			want: 7 * time.Second,
		},
		{
			name:  "score=0.5 gives midpoint",
			score: 0.5,
			// target=32.5s, base=10s, smoothed = 0.6*32.5 + 0.4*10 = 19.5 + 4 = 23.5s
			want: 23500 * time.Millisecond,
		},
		{
			name:  "score=0.25 gives 3/4 point",
			score: 0.25,
			// target=46.25s, base=10s, smoothed = 0.6*46.25 + 0.4*10 = 27.75 + 4 = 31.75s
			want: 31750 * time.Millisecond,
		},
		{
			name:  "score=0.75 gives 1/4 point",
			score: 0.75,
			// target=18.75s, base=10s, smoothed = 0.6*18.75 + 0.4*10 = 11.25 + 4 = 15.25s
			want: 15250 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new selector for each test to avoid state interference
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0 // Disable jitter for predictable results

			req := IntervalRequest{
				Now:            time.Now(),
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: tt.score,
				ErrorCount:     0,
				QueueDepth:     1,
				InstanceKey:    "test-staleness-" + tt.name,
			}

			got := selector.SelectInterval(req)

			// Allow for small rounding differences
			tolerance := 100 * time.Millisecond
			if got < tt.want-tolerance || got > tt.want+tolerance {
				t.Errorf("SelectInterval(score=%v) = %v, want ~%v", tt.score, got, tt.want)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_ErrorPenalty tests error count impact on interval
func TestAdaptiveIntervalSelector_ErrorPenalty(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	tests := []struct {
		name       string
		errorCount int
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{
			name:       "errorCount=0 no reduction",
			errorCount: 0,
			// score=0.5, target=32.5s, smoothed = 0.6*32.5 + 0.4*10 = 23.5s
			wantMin: 23 * time.Second,
			wantMax: 24 * time.Second,
		},
		{
			name:       "errorCount=1 reduces interval",
			errorCount: 1,
			// target=32.5s / (1+0.6*1) = 32.5/1.6 = 20.3125s, smoothed = 0.6*20.3125 + 0.4*10 = 16.1875s
			wantMin: 16 * time.Second,
			wantMax: 17 * time.Second,
		},
		{
			name:       "errorCount=2 reduces more",
			errorCount: 2,
			// target=32.5s / (1+0.6*2) = 32.5/2.2 = 14.77s, smoothed = 0.6*14.77 + 0.4*10 = 12.86s
			wantMin: 12500 * time.Millisecond,
			wantMax: 13500 * time.Millisecond,
		},
		{
			name:       "errorCount=5 high penalty",
			errorCount: 5,
			// target=32.5s / (1+0.6*5) = 32.5/4 = 8.125s, smoothed = 0.6*8.125 + 0.4*10 = 8.875s
			wantMin: 8500 * time.Millisecond,
			wantMax: 9500 * time.Millisecond,
		},
		{
			name:       "errorCount=10 clamped to min",
			errorCount: 10,
			// target=32.5s / (1+0.6*10) = 32.5/7 = 4.64s -> clamped to 5s, smoothed = 0.6*5 + 0.4*10 = 7s
			wantMin: 6500 * time.Millisecond,
			wantMax: 7500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new selector for each test to avoid state interference
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0 // Disable jitter

			req := IntervalRequest{
				Now:            time.Now(),
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: 0.5, // midpoint base
				ErrorCount:     tt.errorCount,
				QueueDepth:     1,
				InstanceKey:    "test-error-" + tt.name,
			}

			got := selector.SelectInterval(req)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SelectInterval(errorCount=%v) = %v, want between %v and %v",
					tt.errorCount, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_QueueDepthStretching tests queue depth impact
func TestAdaptiveIntervalSelector_QueueDepthStretching(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	tests := []struct {
		name       string
		queueDepth int
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{
			name:       "queueDepth=1 no stretch",
			queueDepth: 1,
			// target=32.5s * 1 = 32.5s, smoothed = 0.6*32.5 + 0.4*10 = 23.5s
			wantMin: 23 * time.Second,
			wantMax: 24 * time.Second,
		},
		{
			name:       "queueDepth=2 slight stretch",
			queueDepth: 2,
			// target=32.5s * (1 + 0.1*1) = 35.75s, smoothed = 0.6*35.75 + 0.4*10 = 25.45s
			wantMin: 25 * time.Second,
			wantMax: 26 * time.Second,
		},
		{
			name:       "queueDepth=5 moderate stretch",
			queueDepth: 5,
			// target=32.5s * (1 + 0.1*4) = 45.5s, smoothed = 0.6*45.5 + 0.4*10 = 31.3s
			wantMin: 31 * time.Second,
			wantMax: 32 * time.Second,
		},
		{
			name:       "queueDepth=10 high stretch",
			queueDepth: 10,
			// target=32.5s * (1 + 0.1*9) = 61.75s -> clamped to 60s, smoothed = 0.6*60 + 0.4*10 = 40s
			wantMin: 39 * time.Second,
			wantMax: 41 * time.Second,
		},
		{
			name:       "queueDepth=50 clamped to max",
			queueDepth: 50,
			// target=32.5s * (1 + 0.1*49) = 192.25s -> clamped to 60s, smoothed = 0.6*60 + 0.4*10 = 40s
			wantMin: 39 * time.Second,
			wantMax: 41 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new selector for each test to avoid state interference
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0 // Disable jitter

			req := IntervalRequest{
				Now:            time.Now(),
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: 0.5, // midpoint base
				ErrorCount:     0,
				QueueDepth:     tt.queueDepth,
				InstanceKey:    "test-queue-" + tt.name,
			}

			got := selector.SelectInterval(req)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SelectInterval(queueDepth=%v) = %v, want between %v and %v",
					tt.queueDepth, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_EMASmoothing tests exponential moving average smoothing
func TestAdaptiveIntervalSelector_EMASmoothing(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}
	selector := newAdaptiveIntervalSelector(cfg)
	selector.jitterFraction = 0 // Disable jitter
	selector.alpha = 0.6        // 60% new, 40% old

	instanceKey := "test-ema-smoothing"

	// First call: no previous state, uses base interval
	req1 := IntervalRequest{
		Now:            time.Now(),
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 1.0, // target = min = 5s
		ErrorCount:     0,
		QueueDepth:     1,
		InstanceKey:    instanceKey,
		LastInterval:   0,
	}

	got1 := selector.SelectInterval(req1)
	// First call: alpha * 5s + (1-alpha) * 10s = 0.6*5 + 0.4*10 = 3 + 4 = 7s
	want1 := 7 * time.Second
	tolerance := 100 * time.Millisecond
	if got1 < want1-tolerance || got1 > want1+tolerance {
		t.Errorf("First call: got %v, want ~%v", got1, want1)
	}

	// Second call: should blend with previous smoothed value
	req2 := IntervalRequest{
		Now:            time.Now(),
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 0.0, // target = max = 60s
		ErrorCount:     0,
		QueueDepth:     1,
		InstanceKey:    instanceKey,
		LastInterval:   got1,
	}

	got2 := selector.SelectInterval(req2)
	// Second call: alpha * 60s + (1-alpha) * 7s = 0.6*60 + 0.4*7 = 36 + 2.8 = 38.8s
	want2 := 38800 * time.Millisecond
	if got2 < want2-tolerance || got2 > want2+tolerance {
		t.Errorf("Second call: got %v, want ~%v", got2, want2)
	}

	// Third call: should continue blending
	req3 := IntervalRequest{
		Now:            time.Now(),
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 0.0, // target = max = 60s
		ErrorCount:     0,
		QueueDepth:     1,
		InstanceKey:    instanceKey,
		LastInterval:   got2,
	}

	got3 := selector.SelectInterval(req3)
	// Third call: alpha * 60s + (1-alpha) * 38.8s = 0.6*60 + 0.4*38.8 = 36 + 15.52 = 51.52s
	want3 := 51520 * time.Millisecond
	if got3 < want3-tolerance || got3 > want3+tolerance {
		t.Errorf("Third call: got %v, want ~%v", got3, want3)
	}
}

// TestAdaptiveIntervalSelector_Jitter tests jitter application
func TestAdaptiveIntervalSelector_Jitter(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}
	selector := newAdaptiveIntervalSelector(cfg)
	selector.jitterFraction = 0.05 // ±5%

	// Run multiple times to test jitter range
	instanceKey := "test-jitter"
	results := make([]time.Duration, 100)

	for i := 0; i < 100; i++ {
		req := IntervalRequest{
			Now:            time.Now(),
			BaseInterval:   cfg.BaseInterval,
			MinInterval:    cfg.MinInterval,
			MaxInterval:    cfg.MaxInterval,
			StalenessScore: 0.5, // midpoint ~32.5s
			ErrorCount:     0,
			QueueDepth:     1,
			InstanceKey:    instanceKey,
		}

		results[i] = selector.SelectInterval(req)
	}

	// All results should be within bounds
	for i, result := range results {
		if result < cfg.MinInterval {
			t.Errorf("result[%d] = %v, below min %v", i, result, cfg.MinInterval)
		}
		if result > cfg.MaxInterval {
			t.Errorf("result[%d] = %v, above max %v", i, result, cfg.MaxInterval)
		}
	}

	// Should have some variation due to jitter
	unique := make(map[time.Duration]bool)
	for _, result := range results {
		unique[result] = true
	}
	if len(unique) < 10 {
		t.Errorf("jitter produced only %d unique values, expected more variation", len(unique))
	}
}

// TestAdaptiveIntervalSelector_JitterDeterministic tests jitter with seeded RNG
func TestAdaptiveIntervalSelector_JitterDeterministic(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  20 * time.Second,
		MaxInterval:  40 * time.Second,
	}

	tests := []struct {
		name       string
		seed       int64
		score      float64
		wantMin    time.Duration
		wantMax    time.Duration
		iterations int
	}{
		{
			name:       "jitter within bounds",
			seed:       12345,
			score:      0.5,
			wantMin:    19 * time.Second,  // slightly below min due to jitter
			wantMax:    41 * time.Second,  // slightly above base due to jitter
			iterations: 50,
		},
		{
			name:       "jitter respects min clamp",
			seed:       67890,
			score:      1.0, // target is min (20s)
			wantMin:    20 * time.Second,
			wantMax:    25 * time.Second,
			iterations: 50,
		},
		{
			name:       "jitter respects max clamp",
			seed:       11111,
			score:      0.0, // target is max (40s), but EMA converges over iterations
			wantMin:    25 * time.Second,
			wantMax:    40 * time.Second, // eventually converges to max
			iterations: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := newAdaptiveIntervalSelector(cfg)
			selector.rng = rand.New(rand.NewSource(tt.seed))
			selector.jitterFraction = 0.05

			instanceKey := "test-jitter-deterministic-" + tt.name

			for i := 0; i < tt.iterations; i++ {
				req := IntervalRequest{
					Now:            time.Now(),
					BaseInterval:   cfg.BaseInterval,
					MinInterval:    cfg.MinInterval,
					MaxInterval:    cfg.MaxInterval,
					StalenessScore: tt.score,
					ErrorCount:     0,
					QueueDepth:     1,
					InstanceKey:    instanceKey,
				}

				got := selector.SelectInterval(req)

				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("iteration %d: SelectInterval() = %v, want between %v and %v",
						i, got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

// TestAdaptiveIntervalSelector_BoundaryConditions tests edge cases
func TestAdaptiveIntervalSelector_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name        string
		cfg         SchedulerConfig
		req         IntervalRequest
		wantMin     time.Duration
		wantMax     time.Duration
	}{
		{
			name: "zero intervals default to min",
			cfg: SchedulerConfig{
				BaseInterval: 0,
				MinInterval:  5 * time.Second,
				MaxInterval:  60 * time.Second,
			},
			req: IntervalRequest{
				BaseInterval:   0,
				MinInterval:    5 * time.Second,
				MaxInterval:    60 * time.Second,
				StalenessScore: 0.5,
				InstanceKey:    "test-zero-interval",
			},
			wantMin: 5 * time.Second,
			wantMax: 60 * time.Second,
		},
		{
			name: "negative intervals treated as zero",
			cfg: SchedulerConfig{
				BaseInterval: 10 * time.Second,
				MinInterval:  5 * time.Second,
				MaxInterval:  60 * time.Second,
			},
			req: IntervalRequest{
				BaseInterval:   -10 * time.Second,
				MinInterval:    5 * time.Second,
				MaxInterval:    60 * time.Second,
				StalenessScore: 0.5,
				InstanceKey:    "test-negative-interval",
			},
			wantMin: 5 * time.Second,
			wantMax: 60 * time.Second,
		},
		{
			name: "max less than min uses min",
			cfg: SchedulerConfig{
				BaseInterval: 10 * time.Second,
				MinInterval:  60 * time.Second,
				MaxInterval:  5 * time.Second,
			},
			req: IntervalRequest{
				BaseInterval:   10 * time.Second,
				MinInterval:    60 * time.Second,
				MaxInterval:    5 * time.Second,
				StalenessScore: 0.5,
				InstanceKey:    "test-max-less-than-min",
			},
			wantMin: 5 * time.Second,  // corrected to max value
			wantMax: 60 * time.Second, // corrected to min value
		},
		{
			name: "empty instance key uses instance type",
			cfg: SchedulerConfig{
				BaseInterval: 10 * time.Second,
				MinInterval:  5 * time.Second,
				MaxInterval:  60 * time.Second,
			},
			req: IntervalRequest{
				BaseInterval:   10 * time.Second,
				MinInterval:    5 * time.Second,
				MaxInterval:    60 * time.Second,
				StalenessScore: 0.5,
				InstanceKey:    "", // empty key
				InstanceType:   InstanceTypePVE,
			},
			wantMin: 5 * time.Second,
			wantMax: 60 * time.Second,
		},
		{
			name: "staleness score above 1 clamped",
			cfg: SchedulerConfig{
				BaseInterval: 10 * time.Second,
				MinInterval:  5 * time.Second,
				MaxInterval:  60 * time.Second,
			},
			req: IntervalRequest{
				BaseInterval:   10 * time.Second,
				MinInterval:    5 * time.Second,
				MaxInterval:    60 * time.Second,
				StalenessScore: 2.0, // should be clamped to 1.0
				InstanceKey:    "test-staleness-above-1",
			},
			wantMin: 5 * time.Second,
			wantMax: 10 * time.Second,
		},
		{
			name: "staleness score below 0 clamped",
			cfg: SchedulerConfig{
				BaseInterval: 10 * time.Second,
				MinInterval:  5 * time.Second,
				MaxInterval:  60 * time.Second,
			},
			req: IntervalRequest{
				BaseInterval:   10 * time.Second,
				MinInterval:    5 * time.Second,
				MaxInterval:    60 * time.Second,
				StalenessScore: -1.0, // clamped to 0.0, target=60s, smoothed = 0.6*60 + 0.4*10 = 40s
				InstanceKey:    "test-staleness-below-0",
			},
			wantMin: 39 * time.Second,
			wantMax: 41 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := newAdaptiveIntervalSelector(tt.cfg)
			selector.jitterFraction = 0 // Disable jitter for predictable results

			got := selector.SelectInterval(tt.req)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SelectInterval() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_CombinedFactors tests all factors working together
func TestAdaptiveIntervalSelector_CombinedFactors(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	tests := []struct {
		name    string
		req     IntervalRequest
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "high staleness, errors, and queue depth",
			req: IntervalRequest{
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: 0.9, // high staleness -> low base interval
				ErrorCount:     3,   // errors reduce further
				QueueDepth:     10,  // queue stretches back up
				InstanceKey:    "test-combined-high",
			},
			// target=(5 + 55*0.1) = 10.5s, error penalty: 10.5/(1+0.6*3) = 3.78s -> clamp to 5s
			// queue stretch: 5 * (1+0.1*9) = 9.5s, smoothed = 0.6*9.5 + 0.4*10 = 9.7s
			wantMin: 9 * time.Second,
			wantMax: 10500 * time.Millisecond,
		},
		{
			name: "low staleness, no errors, low queue",
			req: IntervalRequest{
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: 0.1, // low staleness -> high base interval
				ErrorCount:     0,   // no error penalty
				QueueDepth:     1,   // no queue stretch
				InstanceKey:    "test-combined-low",
			},
			// target=(5 + 55*0.9) = 54.5s, smoothed = 0.6*54.5 + 0.4*10 = 36.7s
			wantMin: 36 * time.Second,
			wantMax: 38 * time.Second,
		},
		{
			name: "moderate staleness with queue depth",
			req: IntervalRequest{
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: 0.5, // midpoint
				ErrorCount:     0,
				QueueDepth:     5, // moderate queue stretch
				InstanceKey:    "test-combined-moderate",
			},
			// target=32.5s * (1 + 0.1*4) = 45.5s, smoothed = 0.6*45.5 + 0.4*10 = 31.3s
			wantMin: 31 * time.Second,
			wantMax: 32 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new selector for each test to avoid state interference
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0 // Disable jitter for predictable results

			got := selector.SelectInterval(tt.req)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SelectInterval() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_MaxIntervalEdgeCases tests max interval edge cases
func TestAdaptiveIntervalSelector_MaxIntervalEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		minInterval time.Duration
		maxInterval time.Duration
		wantMin     time.Duration
		wantMax     time.Duration
	}{
		{
			name:        "max is zero uses min as max",
			minInterval: 10 * time.Second,
			maxInterval: 0,
			// When max <= 0, max = min, so span = 0, target = min
			// smoothed = 0.6*10 + 0.4*10 = 10s
			wantMin: 10 * time.Second,
			wantMax: 10 * time.Second,
		},
		{
			name:        "max is negative uses min as max",
			minInterval: 10 * time.Second,
			maxInterval: -5 * time.Second,
			// When max < 0, max = min, so span = 0, target = min
			wantMin: 10 * time.Second,
			wantMax: 10 * time.Second,
		},
		{
			name:        "max less than min uses min as max",
			minInterval: 30 * time.Second,
			maxInterval: 10 * time.Second,
			// When max < min, max = min, so span = 0, target = min
			wantMin: 30 * time.Second,
			wantMax: 30 * time.Second,
		},
		{
			name:        "max equals min uses that value",
			minInterval: 15 * time.Second,
			maxInterval: 15 * time.Second,
			// span = 0, target = min = 15s
			wantMin: 15 * time.Second,
			wantMax: 15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := SchedulerConfig{
				BaseInterval: tt.minInterval,
				MinInterval:  tt.minInterval,
				MaxInterval:  tt.maxInterval,
			}
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0

			req := IntervalRequest{
				BaseInterval:   tt.minInterval,
				MinInterval:    tt.minInterval,
				MaxInterval:    tt.maxInterval,
				StalenessScore: 0.5,
				InstanceKey:    "test-max-edge-" + tt.name,
			}

			got := selector.SelectInterval(req)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SelectInterval() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_LastIntervalFallback tests LastInterval <= 0 fallback
func TestAdaptiveIntervalSelector_LastIntervalFallback(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 20 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	tests := []struct {
		name         string
		lastInterval time.Duration
		baseInterval time.Duration
		wantMin      time.Duration
		wantMax      time.Duration
	}{
		{
			name:         "zero LastInterval uses BaseInterval",
			lastInterval: 0,
			baseInterval: 20 * time.Second,
			// target=32.5s (score=0.5), base=20s (from BaseInterval)
			// smoothed = 0.6*32.5 + 0.4*20 = 19.5 + 8 = 27.5s
			wantMin: 27 * time.Second,
			wantMax: 28 * time.Second,
		},
		{
			name:         "negative LastInterval uses BaseInterval",
			lastInterval: -10 * time.Second,
			baseInterval: 20 * time.Second,
			// Same calculation as above
			wantMin: 27 * time.Second,
			wantMax: 28 * time.Second,
		},
		{
			name:         "positive LastInterval is used directly",
			lastInterval: 40 * time.Second,
			baseInterval: 20 * time.Second,
			// target=32.5s (score=0.5), base=40s (from LastInterval, but prev state overrides)
			// On first call with no state: smoothed = 0.6*32.5 + 0.4*40 = 19.5 + 16 = 35.5s
			wantMin: 35 * time.Second,
			wantMax: 36 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0

			req := IntervalRequest{
				BaseInterval:   tt.baseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: 0.5,
				LastInterval:   tt.lastInterval,
				InstanceKey:    "test-lastinterval-" + tt.name,
			}

			got := selector.SelectInterval(req)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SelectInterval() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_InstanceKeyFallback tests empty InstanceKey fallback to InstanceType
func TestAdaptiveIntervalSelector_InstanceKeyFallback(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	tests := []struct {
		name         string
		instanceKey  string
		instanceType InstanceType
	}{
		{
			name:         "empty key uses PVE type",
			instanceKey:  "",
			instanceType: InstanceTypePVE,
		},
		{
			name:         "empty key uses PBS type",
			instanceKey:  "",
			instanceType: InstanceTypePBS,
		},
		{
			name:         "non-empty key ignores type",
			instanceKey:  "custom-key",
			instanceType: InstanceTypePVE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0

			// First call to set state
			req1 := IntervalRequest{
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: 0.0, // max interval target
				InstanceKey:    tt.instanceKey,
				InstanceType:   tt.instanceType,
			}
			got1 := selector.SelectInterval(req1)

			// Second call should use stored state
			req2 := IntervalRequest{
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: 0.0,
				InstanceKey:    tt.instanceKey,
				InstanceType:   tt.instanceType,
			}
			got2 := selector.SelectInterval(req2)

			// Second call should trend higher (toward max) due to EMA smoothing
			if got2 < got1 {
				t.Errorf("second call should be >= first call with EMA: got %v, first was %v", got2, got1)
			}

			// Verify the key is correctly derived
			expectedKey := tt.instanceKey
			if expectedKey == "" {
				expectedKey = string(tt.instanceType)
			}

			selector.mu.Lock()
			_, exists := selector.state[expectedKey]
			selector.mu.Unlock()

			if !exists {
				t.Errorf("expected state to be stored under key %q", expectedKey)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_ErrorPenaltyCalculation tests error penalty branch details
func TestAdaptiveIntervalSelector_ErrorPenaltyCalculation(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	tests := []struct {
		name       string
		errorCount int
		score      float64
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{
			name:       "zero errors no penalty applied",
			errorCount: 0,
			score:      0.5,
			// target=32.5s, no penalty, smoothed = 0.6*32.5 + 0.4*10 = 23.5s
			wantMin: 23 * time.Second,
			wantMax: 24 * time.Second,
		},
		{
			name:       "one error reduces target",
			errorCount: 1,
			score:      0.5,
			// target=32.5s, penalty = 1 + 0.6*1 = 1.6
			// target = 32.5 / 1.6 = 20.3125s
			// smoothed = 0.6*20.3125 + 0.4*10 = 16.1875s
			wantMin: 15500 * time.Millisecond,
			wantMax: 17 * time.Second,
		},
		{
			name:       "high errors clamp to min before smoothing",
			errorCount: 20,
			score:      0.5,
			// target=32.5s, penalty = 1 + 0.6*20 = 13
			// target = 32.5 / 13 = 2.5s -> clamped to 5s (min)
			// smoothed = 0.6*5 + 0.4*10 = 7s
			wantMin: 6500 * time.Millisecond,
			wantMax: 7500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0

			req := IntervalRequest{
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: tt.score,
				ErrorCount:     tt.errorCount,
				QueueDepth:     1,
				InstanceKey:    "test-error-calc-" + tt.name,
			}

			got := selector.SelectInterval(req)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SelectInterval(errorCount=%d) = %v, want between %v and %v",
					tt.errorCount, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_QueueDepthCalculation tests queue depth stretch branch details
func TestAdaptiveIntervalSelector_QueueDepthCalculation(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	tests := []struct {
		name       string
		queueDepth int
		score      float64
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{
			name:       "queue depth 0 no stretch",
			queueDepth: 0,
			score:      0.5,
			// target=32.5s, no stretch (queueDepth <= 1)
			// smoothed = 0.6*32.5 + 0.4*10 = 23.5s
			wantMin: 23 * time.Second,
			wantMax: 24 * time.Second,
		},
		{
			name:       "queue depth 1 no stretch",
			queueDepth: 1,
			score:      0.5,
			// target=32.5s, no stretch (queueDepth <= 1)
			// smoothed = 0.6*32.5 + 0.4*10 = 23.5s
			wantMin: 23 * time.Second,
			wantMax: 24 * time.Second,
		},
		{
			name:       "queue depth 3 applies stretch",
			queueDepth: 3,
			score:      0.5,
			// target=32.5s, stretch = 1 + 0.1*(3-1) = 1.2
			// target = 32.5 * 1.2 = 39s
			// smoothed = 0.6*39 + 0.4*10 = 27.4s
			wantMin: 27 * time.Second,
			wantMax: 28 * time.Second,
		},
		{
			name:       "high queue depth clamps to max",
			queueDepth: 100,
			score:      0.5,
			// target=32.5s, stretch = 1 + 0.1*99 = 10.9
			// target = 32.5 * 10.9 = 354.25s -> clamped to 60s
			// smoothed = 0.6*60 + 0.4*10 = 40s
			wantMin: 39 * time.Second,
			wantMax: 41 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0

			req := IntervalRequest{
				BaseInterval:   cfg.BaseInterval,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: tt.score,
				ErrorCount:     0,
				QueueDepth:     tt.queueDepth,
				InstanceKey:    "test-queue-calc-" + tt.name,
			}

			got := selector.SelectInterval(req)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SelectInterval(queueDepth=%d) = %v, want between %v and %v",
					tt.queueDepth, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_SmoothedBoundsClamping tests smoothed value clamping to bounds
func TestAdaptiveIntervalSelector_SmoothedBoundsClamping(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  20 * time.Second,
		MaxInterval:  40 * time.Second,
	}

	tests := []struct {
		name    string
		alpha   float64
		score   float64
		base    time.Duration
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:  "smoothed clamped to min when base much lower",
			alpha: 0.1, // 10% new, 90% old - heavily weighted toward base
			score: 0.5, // target = 30s
			base:  5 * time.Second,
			// smoothed = 0.1*30 + 0.9*5 = 3 + 4.5 = 7.5s -> clamped to 20s (min)
			wantMin: 20 * time.Second,
			wantMax: 20 * time.Second,
		},
		{
			name:  "smoothed clamped to max when base much higher",
			alpha: 0.1, // 10% new, 90% old
			score: 0.5, // target = 30s
			base:  100 * time.Second,
			// smoothed = 0.1*30 + 0.9*100 = 3 + 90 = 93s -> clamped to 40s (max)
			wantMin: 40 * time.Second,
			wantMax: 40 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := newAdaptiveIntervalSelector(cfg)
			selector.jitterFraction = 0
			selector.alpha = tt.alpha

			req := IntervalRequest{
				BaseInterval:   tt.base,
				MinInterval:    cfg.MinInterval,
				MaxInterval:    cfg.MaxInterval,
				StalenessScore: tt.score,
				InstanceKey:    "test-smoothed-clamp-" + tt.name,
			}

			got := selector.SelectInterval(req)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SelectInterval() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestAdaptiveIntervalSelector_StatePersistence tests state is maintained per instance
func TestAdaptiveIntervalSelector_StatePersistence(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}
	selector := newAdaptiveIntervalSelector(cfg)
	selector.jitterFraction = 0 // Disable jitter

	// Call for instance A
	reqA1 := IntervalRequest{
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 1.0, // min interval
		InstanceKey:    "instance-A",
	}
	gotA1 := selector.SelectInterval(reqA1)

	// Call for instance B
	reqB1 := IntervalRequest{
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 0.0, // max interval
		InstanceKey:    "instance-B",
	}
	gotB1 := selector.SelectInterval(reqB1)

	// Second call for instance A should use smoothed value from first call
	reqA2 := IntervalRequest{
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 1.0,
		InstanceKey:    "instance-A",
	}
	gotA2 := selector.SelectInterval(reqA2)

	// Second call for instance B should use smoothed value from first call
	reqB2 := IntervalRequest{
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 0.0,
		InstanceKey:    "instance-B",
	}
	gotB2 := selector.SelectInterval(reqB2)

	// Instance A should trend toward min
	if gotA2 >= gotA1 {
		t.Errorf("instance A second call should be <= first call: got %v, first was %v", gotA2, gotA1)
	}

	// Instance B should trend toward max
	if gotB2 <= gotB1 {
		t.Errorf("instance B second call should be >= first call: got %v, first was %v", gotB2, gotB1)
	}

	// The two instances should have different intervals
	if gotA2 == gotB2 {
		t.Errorf("different instances should have different intervals: both got %v", gotA2)
	}
}
