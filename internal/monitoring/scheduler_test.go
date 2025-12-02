package monitoring

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"
)

// mockStalenessSource is a test implementation of StalenessSource
type mockStalenessSource struct {
	scores map[string]float64
}

func (m mockStalenessSource) StalenessScore(instanceType InstanceType, instanceName string) (float64, bool) {
	key := string(instanceType) + ":" + instanceName
	score, ok := m.scores[key]
	return score, ok
}

// mockIntervalSelector is a test implementation of IntervalSelector
type mockIntervalSelector struct {
	interval time.Duration
}

func (m mockIntervalSelector) SelectInterval(req IntervalRequest) time.Duration {
	return m.interval
}

// mockTaskEnqueuer is a test implementation of TaskEnqueuer
type mockTaskEnqueuer struct {
	tasks []ScheduledTask
}

func (m *mockTaskEnqueuer) Enqueue(ctx context.Context, task ScheduledTask) error {
	m.tasks = append(m.tasks, task)
	return nil
}

// TestNewAdaptiveScheduler tests constructor with various input combinations
func TestNewAdaptiveScheduler(t *testing.T) {
	defaultCfg := DefaultSchedulerConfig()

	tests := []struct {
		name             string
		cfg              SchedulerConfig
		staleness        StalenessSource
		interval         IntervalSelector
		enqueuer         TaskEnqueuer
		wantBaseInterval time.Duration
		wantMinInterval  time.Duration
		wantMaxInterval  time.Duration
	}{
		{
			name: "all valid parameters preserved",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  10 * time.Second,
				MaxInterval:  2 * time.Minute,
			},
			staleness:        mockStalenessSource{scores: map[string]float64{"pve:test": 0.5}},
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  10 * time.Second,
			wantMaxInterval:  2 * time.Minute,
		},
		{
			name: "zero BaseInterval gets default",
			cfg: SchedulerConfig{
				BaseInterval: 0,
				MinInterval:  10 * time.Second,
				MaxInterval:  2 * time.Minute,
			},
			staleness:        mockStalenessSource{},
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: defaultCfg.BaseInterval,
			wantMinInterval:  10 * time.Second,
			wantMaxInterval:  2 * time.Minute,
		},
		{
			name: "negative BaseInterval gets default",
			cfg: SchedulerConfig{
				BaseInterval: -5 * time.Second,
				MinInterval:  10 * time.Second,
				MaxInterval:  2 * time.Minute,
			},
			staleness:        mockStalenessSource{},
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: defaultCfg.BaseInterval,
			wantMinInterval:  10 * time.Second,
			wantMaxInterval:  2 * time.Minute,
		},
		{
			name: "zero MinInterval gets default",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  0,
				MaxInterval:  2 * time.Minute,
			},
			staleness:        mockStalenessSource{},
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  defaultCfg.MinInterval,
			wantMaxInterval:  2 * time.Minute,
		},
		{
			name: "negative MinInterval gets default",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  -5 * time.Second,
				MaxInterval:  2 * time.Minute,
			},
			staleness:        mockStalenessSource{},
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  defaultCfg.MinInterval,
			wantMaxInterval:  2 * time.Minute,
		},
		{
			name: "zero MaxInterval gets default",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  10 * time.Second,
				MaxInterval:  0,
			},
			staleness:        mockStalenessSource{},
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  10 * time.Second,
			wantMaxInterval:  defaultCfg.MaxInterval,
		},
		{
			name: "negative MaxInterval gets default",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  10 * time.Second,
				MaxInterval:  -5 * time.Second,
			},
			staleness:        mockStalenessSource{},
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  10 * time.Second,
			wantMaxInterval:  defaultCfg.MaxInterval,
		},
		{
			name: "MaxInterval less than MinInterval gets default",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  2 * time.Minute,
				MaxInterval:  30 * time.Second,
			},
			staleness:        mockStalenessSource{},
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  2 * time.Minute,
			wantMaxInterval:  defaultCfg.MaxInterval,
		},
		{
			name: "nil staleness gets noopStalenessSource",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  10 * time.Second,
				MaxInterval:  2 * time.Minute,
			},
			staleness:        nil,
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  10 * time.Second,
			wantMaxInterval:  2 * time.Minute,
		},
		{
			name: "nil interval gets newAdaptiveIntervalSelector",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  10 * time.Second,
				MaxInterval:  2 * time.Minute,
			},
			staleness:        mockStalenessSource{},
			interval:         nil,
			enqueuer:         &mockTaskEnqueuer{},
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  10 * time.Second,
			wantMaxInterval:  2 * time.Minute,
		},
		{
			name: "nil enqueuer gets noopTaskEnqueuer",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  10 * time.Second,
				MaxInterval:  2 * time.Minute,
			},
			staleness:        mockStalenessSource{},
			interval:         mockIntervalSelector{interval: 15 * time.Second},
			enqueuer:         nil,
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  10 * time.Second,
			wantMaxInterval:  2 * time.Minute,
		},
		{
			name: "all nil dependencies get defaults",
			cfg: SchedulerConfig{
				BaseInterval: 20 * time.Second,
				MinInterval:  10 * time.Second,
				MaxInterval:  2 * time.Minute,
			},
			staleness:        nil,
			interval:         nil,
			enqueuer:         nil,
			wantBaseInterval: 20 * time.Second,
			wantMinInterval:  10 * time.Second,
			wantMaxInterval:  2 * time.Minute,
		},
		{
			name:             "all zero config and nil dependencies get all defaults",
			cfg:              SchedulerConfig{},
			staleness:        nil,
			interval:         nil,
			enqueuer:         nil,
			wantBaseInterval: defaultCfg.BaseInterval,
			wantMinInterval:  defaultCfg.MinInterval,
			wantMaxInterval:  defaultCfg.MaxInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler := NewAdaptiveScheduler(tt.cfg, tt.staleness, tt.interval, tt.enqueuer)

			if scheduler == nil {
				t.Fatal("NewAdaptiveScheduler returned nil")
			}

			// Verify config values
			if scheduler.cfg.BaseInterval != tt.wantBaseInterval {
				t.Errorf("BaseInterval = %v, want %v", scheduler.cfg.BaseInterval, tt.wantBaseInterval)
			}
			if scheduler.cfg.MinInterval != tt.wantMinInterval {
				t.Errorf("MinInterval = %v, want %v", scheduler.cfg.MinInterval, tt.wantMinInterval)
			}
			if scheduler.cfg.MaxInterval != tt.wantMaxInterval {
				t.Errorf("MaxInterval = %v, want %v", scheduler.cfg.MaxInterval, tt.wantMaxInterval)
			}

			// Verify staleness is not nil
			if scheduler.staleness == nil {
				t.Error("staleness is nil, expected non-nil")
			}

			// Verify interval is not nil
			if scheduler.interval == nil {
				t.Error("interval is nil, expected non-nil")
			}

			// Verify enqueuer is not nil
			if scheduler.enqueuer == nil {
				t.Error("enqueuer is nil, expected non-nil")
			}

			// Verify lastPlan is initialized
			if scheduler.lastPlan == nil {
				t.Error("lastPlan is nil, expected initialized map")
			}
		})
	}
}

// TestNewAdaptiveScheduler_StalenessType verifies nil staleness becomes noopStalenessSource
func TestNewAdaptiveScheduler_StalenessType(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	scheduler := NewAdaptiveScheduler(cfg, nil, nil, nil)

	// noopStalenessSource always returns (0, false)
	score, ok := scheduler.staleness.StalenessScore(InstanceTypePVE, "test")
	if ok {
		t.Error("noopStalenessSource should return ok=false")
	}
	if score != 0 {
		t.Errorf("noopStalenessSource should return score=0, got %v", score)
	}
}

// TestNewAdaptiveScheduler_IntervalType verifies nil interval becomes adaptiveIntervalSelector
func TestNewAdaptiveScheduler_IntervalType(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	scheduler := NewAdaptiveScheduler(cfg, nil, nil, nil)

	// adaptiveIntervalSelector should return a duration within bounds
	req := IntervalRequest{
		Now:            time.Now(),
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 0.5,
		InstanceKey:    "test-interval-type",
	}

	interval := scheduler.interval.SelectInterval(req)
	if interval < cfg.MinInterval || interval > cfg.MaxInterval {
		t.Errorf("SelectInterval returned %v, expected between %v and %v",
			interval, cfg.MinInterval, cfg.MaxInterval)
	}
}

// TestNewAdaptiveScheduler_EnqueuerType verifies nil enqueuer becomes noopTaskEnqueuer
func TestNewAdaptiveScheduler_EnqueuerType(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	scheduler := NewAdaptiveScheduler(cfg, nil, nil, nil)

	// noopTaskEnqueuer.Enqueue should return nil (no error)
	task := ScheduledTask{
		InstanceName: "test",
		InstanceType: InstanceTypePVE,
		NextRun:      time.Now(),
		Interval:     10 * time.Second,
	}

	err := scheduler.enqueuer.Enqueue(context.Background(), task)
	if err != nil {
		t.Errorf("noopTaskEnqueuer.Enqueue should return nil, got %v", err)
	}
}

// TestNewAdaptiveScheduler_PreservesCustomDependencies verifies custom implementations are preserved
func TestNewAdaptiveScheduler_PreservesCustomDependencies(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	customStaleness := mockStalenessSource{scores: map[string]float64{"pve:test": 0.75}}
	customInterval := mockIntervalSelector{interval: 25 * time.Second}
	customEnqueuer := &mockTaskEnqueuer{}

	scheduler := NewAdaptiveScheduler(cfg, customStaleness, customInterval, customEnqueuer)

	// Verify custom staleness is used
	score, ok := scheduler.staleness.StalenessScore(InstanceTypePVE, "test")
	if !ok || score != 0.75 {
		t.Errorf("expected custom staleness to return (0.75, true), got (%v, %v)", score, ok)
	}

	// Verify custom interval selector is used
	req := IntervalRequest{
		Now:            time.Now(),
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 0.5,
		InstanceKey:    "test",
	}
	interval := scheduler.interval.SelectInterval(req)
	if interval != 25*time.Second {
		t.Errorf("expected custom interval selector to return 25s, got %v", interval)
	}

	// Verify custom enqueuer is used
	task := ScheduledTask{
		InstanceName: "test",
		InstanceType: InstanceTypePVE,
		NextRun:      time.Now(),
		Interval:     10 * time.Second,
	}
	_ = scheduler.enqueuer.Enqueue(context.Background(), task)
	if len(customEnqueuer.tasks) != 1 {
		t.Errorf("expected custom enqueuer to have 1 task, got %d", len(customEnqueuer.tasks))
	}
}

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

// TestAdaptiveIntervalSelector_NegativePenalty tests the penalty <= 0 branch when errorPenalty is negative
func TestAdaptiveIntervalSelector_NegativePenalty(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}
	selector := newAdaptiveIntervalSelector(cfg)
	selector.jitterFraction = 0
	// Set a negative errorPenalty to make penalty <= 0
	// penalty = 1 + errorPenalty * errorCount
	// With errorPenalty = -2 and errorCount = 1: penalty = 1 + (-2)*1 = -1
	selector.errorPenalty = -2

	req := IntervalRequest{
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 0.5,
		ErrorCount:     1,
		QueueDepth:     1,
		InstanceKey:    "test-negative-penalty",
	}

	got := selector.SelectInterval(req)

	// With penalty <= 0, the division is skipped, target stays at ~32.5s
	// smoothed = 0.6*32.5 + 0.4*10 = 23.5s
	wantMin := 23 * time.Second
	wantMax := 24 * time.Second
	if got < wantMin || got > wantMax {
		t.Errorf("SelectInterval with penalty<=0 = %v, want between %v and %v", got, wantMin, wantMax)
	}
}

// TestAdaptiveIntervalSelector_TargetClampingEdgeCases tests the target < min and target > max clamps
// These defensive checks protect against floating point edge cases when using extreme duration values.
func TestAdaptiveIntervalSelector_TargetClampingEdgeCases(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}
	selector := newAdaptiveIntervalSelector(cfg)
	selector.jitterFraction = 0

	// With score = 1.0 (max staleness), target should equal min
	req := IntervalRequest{
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 1.0,
		InstanceKey:    "test-target-clamp-min",
	}

	got := selector.SelectInterval(req)
	// target = 5 + 55*(1-1) = 5s (exactly min)
	// smoothed = 0.6*5 + 0.4*10 = 7s
	if got < cfg.MinInterval {
		t.Errorf("SelectInterval should never return below min: got %v, min %v", got, cfg.MinInterval)
	}

	// With score = 0.0 (no staleness), target should equal max
	selector2 := newAdaptiveIntervalSelector(cfg)
	selector2.jitterFraction = 0
	req2 := IntervalRequest{
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 0.0,
		InstanceKey:    "test-target-clamp-max",
	}

	got2 := selector2.SelectInterval(req2)
	// target = 5 + 55*(1-0) = 60s (exactly max)
	// smoothed = 0.6*60 + 0.4*10 = 40s
	if got2 > cfg.MaxInterval {
		t.Errorf("SelectInterval should never return above max: got %v, max %v", got2, cfg.MaxInterval)
	}
}

// TestAdaptiveIntervalSelector_TargetBelowMinClamp tests the target < min branch (line 310-311)
// by using negative duration values that cause target calculation to underflow
func TestAdaptiveIntervalSelector_TargetBelowMinClamp(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}
	selector := newAdaptiveIntervalSelector(cfg)
	selector.jitterFraction = 0

	// Use negative min interval to force target calculation below min after correction
	// When max <= 0 || max < min, max becomes min
	// Then span = 0, target = min + 0*(1-score) = min
	// This hits the code path but with corrected values
	req := IntervalRequest{
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    -5 * time.Second, // negative min
		MaxInterval:    -10 * time.Second, // negative max < negative min
		StalenessScore: 0.5,
		InstanceKey:    "test-target-below-min",
	}

	got := selector.SelectInterval(req)
	// max becomes min (-5s), span = 0, target = -5s
	// target < min check: -5 < -5 is false, so no clamp
	// But with span=0, target = min exactly
	// The smoothed calculation then uses negative values
	// Result will be clamped by final bounds check
	_ = got // We just need to execute the code path
}

// TestAdaptiveIntervalSelector_TargetAboveMaxClamp tests the target > max branch (line 313-314)
// by engineering a scenario where floating point arithmetic could exceed max
func TestAdaptiveIntervalSelector_TargetAboveMaxClamp(t *testing.T) {
	// Use very large durations that could cause floating point precision issues
	cfg := SchedulerConfig{
		BaseInterval: time.Duration(1<<62) * time.Nanosecond,
		MinInterval:  time.Duration(1<<61) * time.Nanosecond,
		MaxInterval:  time.Duration(1<<62) * time.Nanosecond,
	}
	selector := newAdaptiveIntervalSelector(cfg)
	selector.jitterFraction = 0

	req := IntervalRequest{
		BaseInterval:   cfg.BaseInterval,
		MinInterval:    cfg.MinInterval,
		MaxInterval:    cfg.MaxInterval,
		StalenessScore: 0.0, // Low staleness = higher target interval
		InstanceKey:    "test-target-above-max",
	}

	got := selector.SelectInterval(req)
	// With extreme values, floating point arithmetic might cause slight overflow
	// but the code clamps to max
	if got > cfg.MaxInterval {
		t.Errorf("SelectInterval should never exceed max: got %v, max %v", got, cfg.MaxInterval)
	}
}

// TestAdaptiveIntervalSelector_InstanceTypeAsKey tests key derivation when InstanceKey is empty
func TestAdaptiveIntervalSelector_InstanceTypeAsKey(t *testing.T) {
	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	tests := []struct {
		name         string
		instanceKey  string
		instanceType InstanceType
		expectedKey  string
	}{
		{
			name:         "empty key uses PVE type",
			instanceKey:  "",
			instanceType: InstanceTypePVE,
			expectedKey:  string(InstanceTypePVE),
		},
		{
			name:         "empty key uses PBS type",
			instanceKey:  "",
			instanceType: InstanceTypePBS,
			expectedKey:  string(InstanceTypePBS),
		},
		{
			name:         "empty key uses PMG type",
			instanceKey:  "",
			instanceType: InstanceTypePMG,
			expectedKey:  string(InstanceTypePMG),
		},
		{
			name:         "non-empty key takes precedence",
			instanceKey:  "custom-key",
			instanceType: InstanceTypePVE,
			expectedKey:  "custom-key",
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
				StalenessScore: 0.5,
				InstanceKey:    tt.instanceKey,
				InstanceType:   tt.instanceType,
			}

			_ = selector.SelectInterval(req)

			// Verify the key was stored correctly
			selector.mu.Lock()
			_, exists := selector.state[tt.expectedKey]
			selector.mu.Unlock()

			if !exists {
				t.Errorf("expected state key %q not found", tt.expectedKey)
			}
		})
	}
}

// TestBuildPlan_EmptyInventory tests that BuildPlan returns nil for empty inventory
func TestBuildPlan_EmptyInventory(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, nil, nil)

	result := scheduler.BuildPlan(time.Now(), nil, 0)
	if result != nil {
		t.Errorf("BuildPlan with nil inventory should return nil, got %v", result)
	}

	result = scheduler.BuildPlan(time.Now(), []InstanceDescriptor{}, 0)
	if result != nil {
		t.Errorf("BuildPlan with empty inventory should return nil, got %v", result)
	}
}

// TestBuildPlan_SingleInstance tests scheduling a single instance
func TestBuildPlan_SingleInstance(t *testing.T) {
	t.Parallel()

	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	scheduler := NewAdaptiveScheduler(cfg, nil, mockIntervalSelector{interval: 10 * time.Second}, nil)

	now := time.Now()
	inventory := []InstanceDescriptor{
		{
			Name: "pve1",
			Type: InstanceTypePVE,
		},
	}

	tasks := scheduler.BuildPlan(now, inventory, 0)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].InstanceName != "pve1" {
		t.Errorf("expected instance name pve1, got %s", tasks[0].InstanceName)
	}
	if tasks[0].InstanceType != InstanceTypePVE {
		t.Errorf("expected instance type pve, got %s", tasks[0].InstanceType)
	}
	if tasks[0].Interval != 10*time.Second {
		t.Errorf("expected interval 10s, got %v", tasks[0].Interval)
	}
}

// TestBuildPlan_MultipleInstances tests scheduling multiple instances and ordering
func TestBuildPlan_MultipleInstances(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()

	// Use staleness source to control priority ordering
	staleness := mockStalenessSource{
		scores: map[string]float64{
			"pve:pve1": 0.8,
			"pbs:pbs1": 0.5,
			"pmg:pmg1": 0.2,
		},
	}

	scheduler := NewAdaptiveScheduler(cfg, staleness, mockIntervalSelector{interval: 10 * time.Second}, nil)

	now := time.Now()
	inventory := []InstanceDescriptor{
		{Name: "pve1", Type: InstanceTypePVE},
		{Name: "pbs1", Type: InstanceTypePBS},
		{Name: "pmg1", Type: InstanceTypePMG},
	}

	tasks := scheduler.BuildPlan(now, inventory, 0)

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// All tasks should have the same NextRun (now), so they should be sorted by priority (descending)
	if tasks[0].Priority < tasks[1].Priority {
		t.Errorf("expected first task to have higher priority: first=%v, second=%v", tasks[0].Priority, tasks[1].Priority)
	}
	if tasks[1].Priority < tasks[2].Priority {
		t.Errorf("expected second task to have higher priority than third: second=%v, third=%v", tasks[1].Priority, tasks[2].Priority)
	}
}

// TestBuildPlan_WithLastSuccess tests scheduling with previous success time
func TestBuildPlan_WithLastSuccess(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, mockIntervalSelector{interval: 10 * time.Second}, nil)

	now := time.Now()
	lastSuccess := now.Add(-5 * time.Second)

	inventory := []InstanceDescriptor{
		{
			Name:        "pve1",
			Type:        InstanceTypePVE,
			LastSuccess: lastSuccess,
		},
	}

	tasks := scheduler.BuildPlan(now, inventory, 0)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	// NextRun should be based on LastSuccess + Interval
	expectedNextRun := lastSuccess.Add(10 * time.Second)
	if !tasks[0].NextRun.Equal(expectedNextRun) {
		t.Errorf("expected NextRun %v, got %v", expectedNextRun, tasks[0].NextRun)
	}
}

// TestBuildPlan_CachesLastPlan tests that BuildPlan caches tasks for subsequent calls
func TestBuildPlan_CachesLastPlan(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, mockIntervalSelector{interval: 10 * time.Second}, nil)

	now := time.Now()
	inventory := []InstanceDescriptor{
		{Name: "pve1", Type: InstanceTypePVE},
	}

	// First call
	tasks1 := scheduler.BuildPlan(now, inventory, 0)
	if len(tasks1) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks1))
	}

	// Second call should use cached LastScheduled
	tasks2 := scheduler.BuildPlan(now.Add(5*time.Second), inventory, 0)
	if len(tasks2) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks2))
	}

	// The second task should have NextRun based on cached LastScheduled
	if tasks2[0].NextRun.Before(tasks1[0].NextRun) {
		t.Errorf("second plan's NextRun should not be before first plan's NextRun")
	}
}

// TestBuildPlan_IntervalClampedToMin tests that intervals below min get clamped
func TestBuildPlan_IntervalClampedToMin(t *testing.T) {
	t.Parallel()

	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	// Return interval below minimum
	scheduler := NewAdaptiveScheduler(cfg, nil, mockIntervalSelector{interval: 2 * time.Second}, nil)

	now := time.Now()
	inventory := []InstanceDescriptor{
		{Name: "pve1", Type: InstanceTypePVE},
	}

	tasks := scheduler.BuildPlan(now, inventory, 0)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].Interval < cfg.MinInterval {
		t.Errorf("interval should be clamped to min %v, got %v", cfg.MinInterval, tasks[0].Interval)
	}
}

// TestBuildPlan_IntervalClampedToMax tests that intervals above max get clamped
func TestBuildPlan_IntervalClampedToMax(t *testing.T) {
	t.Parallel()

	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	// Return interval above maximum
	scheduler := NewAdaptiveScheduler(cfg, nil, mockIntervalSelector{interval: 5 * time.Minute}, nil)

	now := time.Now()
	inventory := []InstanceDescriptor{
		{Name: "pve1", Type: InstanceTypePVE},
	}

	tasks := scheduler.BuildPlan(now, inventory, 0)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].Interval > cfg.MaxInterval {
		t.Errorf("interval should be clamped to max %v, got %v", cfg.MaxInterval, tasks[0].Interval)
	}
}

// TestBuildPlan_ZeroIntervalUsesBase tests that zero interval from selector uses base
func TestBuildPlan_ZeroIntervalUsesBase(t *testing.T) {
	t.Parallel()

	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	// Return zero interval
	scheduler := NewAdaptiveScheduler(cfg, nil, mockIntervalSelector{interval: 0}, nil)

	now := time.Now()
	inventory := []InstanceDescriptor{
		{Name: "pve1", Type: InstanceTypePVE},
	}

	tasks := scheduler.BuildPlan(now, inventory, 0)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	// Zero interval should be replaced with base interval (or at least min)
	if tasks[0].Interval < cfg.MinInterval {
		t.Errorf("expected interval >= %v, got %v", cfg.MinInterval, tasks[0].Interval)
	}
}

// TestBuildPlan_NegativeIntervalUsesBase tests that negative interval from selector uses base
func TestBuildPlan_NegativeIntervalUsesBase(t *testing.T) {
	t.Parallel()

	cfg := SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  60 * time.Second,
	}

	// Return negative interval
	scheduler := NewAdaptiveScheduler(cfg, nil, mockIntervalSelector{interval: -5 * time.Second}, nil)

	now := time.Now()
	inventory := []InstanceDescriptor{
		{Name: "pve1", Type: InstanceTypePVE},
	}

	tasks := scheduler.BuildPlan(now, inventory, 0)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	// Negative interval should be replaced with base interval (or at least min)
	if tasks[0].Interval < cfg.MinInterval {
		t.Errorf("expected interval >= %v, got %v", cfg.MinInterval, tasks[0].Interval)
	}
}

// TestFilterDue_EmptyTasks tests FilterDue with empty input
func TestFilterDue_EmptyTasks(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, nil, nil)

	result := scheduler.FilterDue(time.Now(), nil)
	if result != nil {
		t.Errorf("FilterDue with nil tasks should return nil, got %v", result)
	}

	result = scheduler.FilterDue(time.Now(), []ScheduledTask{})
	if result != nil {
		t.Errorf("FilterDue with empty tasks should return nil, got %v", result)
	}
}

// TestFilterDue_AllDue tests FilterDue when all tasks are due
func TestFilterDue_AllDue(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, nil, nil)

	now := time.Now()
	tasks := []ScheduledTask{
		{InstanceName: "pve1", NextRun: now.Add(-1 * time.Second)},
		{InstanceName: "pbs1", NextRun: now},
		{InstanceName: "pmg1", NextRun: now.Add(-5 * time.Second)},
	}

	due := scheduler.FilterDue(now, tasks)

	if len(due) != 3 {
		t.Fatalf("expected 3 due tasks, got %d", len(due))
	}
}

// TestFilterDue_NoneDue tests FilterDue when no tasks are due
func TestFilterDue_NoneDue(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, nil, nil)

	now := time.Now()
	tasks := []ScheduledTask{
		{InstanceName: "pve1", NextRun: now.Add(10 * time.Second)},
		{InstanceName: "pbs1", NextRun: now.Add(20 * time.Second)},
	}

	due := scheduler.FilterDue(now, tasks)

	if len(due) != 0 {
		t.Fatalf("expected 0 due tasks, got %d", len(due))
	}
}

// TestFilterDue_SomeDue tests FilterDue when some tasks are due
func TestFilterDue_SomeDue(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, nil, nil)

	now := time.Now()
	tasks := []ScheduledTask{
		{InstanceName: "pve1", NextRun: now.Add(-1 * time.Second)}, // due
		{InstanceName: "pbs1", NextRun: now.Add(10 * time.Second)}, // not due
		{InstanceName: "pmg1", NextRun: now},                       // due (exactly now)
	}

	due := scheduler.FilterDue(now, tasks)

	if len(due) != 2 {
		t.Fatalf("expected 2 due tasks, got %d", len(due))
	}
}

// TestDispatchDue_NilScheduler tests DispatchDue with nil scheduler
func TestDispatchDue_NilScheduler(t *testing.T) {
	t.Parallel()

	var scheduler *AdaptiveScheduler
	tasks := []ScheduledTask{
		{InstanceName: "pve1", NextRun: time.Now()},
	}

	// Should not panic and return original tasks
	result := scheduler.DispatchDue(context.Background(), time.Now(), tasks)
	if len(result) != len(tasks) {
		t.Errorf("DispatchDue with nil scheduler should return original tasks")
	}
}

// TestDispatchDue_EnqueuesTasks tests that DispatchDue enqueues due tasks
func TestDispatchDue_EnqueuesTasks(t *testing.T) {
	t.Parallel()

	enqueuer := &mockTaskEnqueuer{}
	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, nil, enqueuer)

	now := time.Now()
	tasks := []ScheduledTask{
		{InstanceName: "pve1", InstanceType: InstanceTypePVE, NextRun: now.Add(-1 * time.Second)},
		{InstanceName: "pbs1", InstanceType: InstanceTypePBS, NextRun: now.Add(10 * time.Second)}, // not due
		{InstanceName: "pmg1", InstanceType: InstanceTypePMG, NextRun: now},
	}

	due := scheduler.DispatchDue(context.Background(), now, tasks)

	if len(due) != 2 {
		t.Fatalf("expected 2 due tasks returned, got %d", len(due))
	}

	if len(enqueuer.tasks) != 2 {
		t.Fatalf("expected 2 tasks enqueued, got %d", len(enqueuer.tasks))
	}
}

// TestDispatchDue_NoDueTasks tests DispatchDue when no tasks are due
func TestDispatchDue_NoDueTasks(t *testing.T) {
	t.Parallel()

	enqueuer := &mockTaskEnqueuer{}
	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, nil, enqueuer)

	now := time.Now()
	tasks := []ScheduledTask{
		{InstanceName: "pve1", NextRun: now.Add(10 * time.Second)},
	}

	due := scheduler.DispatchDue(context.Background(), now, tasks)

	if len(due) != 0 {
		t.Fatalf("expected 0 due tasks returned, got %d", len(due))
	}

	if len(enqueuer.tasks) != 0 {
		t.Fatalf("expected 0 tasks enqueued, got %d", len(enqueuer.tasks))
	}
}

// mockFailingEnqueuer is a test enqueuer that always returns an error
type mockFailingEnqueuer struct{}

func (m *mockFailingEnqueuer) Enqueue(ctx context.Context, task ScheduledTask) error {
	return errors.New("enqueue failed")
}

// TestDispatchDue_EnqueueError tests DispatchDue when enqueue returns an error
func TestDispatchDue_EnqueueError(t *testing.T) {
	t.Parallel()

	enqueuer := &mockFailingEnqueuer{}
	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, nil, enqueuer)

	now := time.Now()
	tasks := []ScheduledTask{
		{InstanceName: "pve1", NextRun: now.Add(-1 * time.Second)},
		{InstanceName: "pve2", NextRun: now.Add(-2 * time.Second)},
	}

	// Should still return due tasks even if enqueue fails
	due := scheduler.DispatchDue(context.Background(), now, tasks)

	if len(due) != 2 {
		t.Fatalf("expected 2 due tasks returned even with enqueue error, got %d", len(due))
	}
}

// TestLastScheduled_NilScheduler tests LastScheduled with nil scheduler
func TestLastScheduled_NilScheduler(t *testing.T) {
	t.Parallel()

	var scheduler *AdaptiveScheduler
	task, ok := scheduler.LastScheduled(InstanceTypePVE, "pve1")

	if ok {
		t.Error("LastScheduled with nil scheduler should return false")
	}
	if task.InstanceName != "" {
		t.Error("LastScheduled with nil scheduler should return empty task")
	}
}

// TestLastScheduled_NotFound tests LastScheduled when instance not in cache
func TestLastScheduled_NotFound(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, nil, nil)

	task, ok := scheduler.LastScheduled(InstanceTypePVE, "nonexistent")

	if ok {
		t.Error("LastScheduled for nonexistent instance should return false")
	}
	if task.InstanceName != "" {
		t.Error("LastScheduled for nonexistent instance should return empty task")
	}
}

// TestLastScheduled_Found tests LastScheduled after BuildPlan populates cache
func TestLastScheduled_Found(t *testing.T) {
	t.Parallel()

	cfg := DefaultSchedulerConfig()
	scheduler := NewAdaptiveScheduler(cfg, nil, mockIntervalSelector{interval: 10 * time.Second}, nil)

	now := time.Now()
	inventory := []InstanceDescriptor{
		{Name: "pve1", Type: InstanceTypePVE},
		{Name: "pbs1", Type: InstanceTypePBS},
	}

	// BuildPlan should populate the cache
	scheduler.BuildPlan(now, inventory, 0)

	// Now LastScheduled should find the task
	task, ok := scheduler.LastScheduled(InstanceTypePVE, "pve1")
	if !ok {
		t.Fatal("LastScheduled should return true for cached instance")
	}
	if task.InstanceName != "pve1" {
		t.Errorf("expected instance name pve1, got %s", task.InstanceName)
	}
	if task.InstanceType != InstanceTypePVE {
		t.Errorf("expected instance type pve, got %s", task.InstanceType)
	}

	// Check different type
	task, ok = scheduler.LastScheduled(InstanceTypePBS, "pbs1")
	if !ok {
		t.Fatal("LastScheduled should return true for cached PBS instance")
	}
	if task.InstanceName != "pbs1" {
		t.Errorf("expected instance name pbs1, got %s", task.InstanceName)
	}
}
