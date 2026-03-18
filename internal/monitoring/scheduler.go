package monitoring

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// InstanceType represents a polling target category.
type InstanceType string

const (
	InstanceTypePVE InstanceType = "pve"
	InstanceTypePBS InstanceType = "pbs"
	InstanceTypePMG InstanceType = "pmg"
)

// StalenessSource provides normalized freshness hints for an instance.
type StalenessSource interface {
	StalenessScore(instanceType InstanceType, instanceName string) (float64, bool)
}

// IntervalSelector chooses the next polling cadence for an instance.
type IntervalSelector interface {
	SelectInterval(req IntervalRequest) time.Duration
}

// TaskEnqueuer receives scheduled tasks for downstream execution.
type TaskEnqueuer interface {
	Enqueue(ctx context.Context, task ScheduledTask) error
}

// IntervalRequest bundles the context required to compute the next polling interval.
type IntervalRequest struct {
	Now            time.Time
	BaseInterval   time.Duration
	MinInterval    time.Duration
	MaxInterval    time.Duration
	LastInterval   time.Duration
	LastSuccess    time.Time
	LastScheduled  time.Time
	StalenessScore float64
	ErrorCount     int
	QueueDepth     int
	InstanceKey    string
	InstanceType   InstanceType
}

// TaskMetadata contains optional scheduling context carried across task planning.
type TaskMetadata struct {
	ChangeHash string
}

// InstanceDescriptor describes a monitored endpoint for scheduling purposes.
type InstanceDescriptor struct {
	Name          string
	Type          InstanceType
	LastSuccess   time.Time
	LastFailure   time.Time
	LastScheduled time.Time
	LastInterval  time.Duration
	ErrorCount    int
	Metadata      TaskMetadata
}

// ScheduledTask represents a single polling opportunity planned by the scheduler.
type ScheduledTask struct {
	InstanceName string
	InstanceType InstanceType
	NextRun      time.Time
	Interval     time.Duration
	Priority     float64
	Metadata     TaskMetadata
}

// SchedulerConfig contains tunables for the adaptive scheduler.
type SchedulerConfig struct {
	BaseInterval time.Duration
	MinInterval  time.Duration
	MaxInterval  time.Duration
}

// DefaultSchedulerConfig returns conservative defaults that preserve current behaviour.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		BaseInterval: 10 * time.Second,
		MinInterval:  5 * time.Second,
		MaxInterval:  5 * time.Minute,
	}
}

// AdaptiveScheduler orchestrates poll execution plans using pluggable scoring strategies.
type AdaptiveScheduler struct {
	cfg       SchedulerConfig
	staleness StalenessSource
	interval  IntervalSelector
	enqueuer  TaskEnqueuer

	mu       sync.RWMutex
	lastPlan map[string]ScheduledTask
}

// NewAdaptiveScheduler constructs a scheduler with safe defaults.
func NewAdaptiveScheduler(cfg SchedulerConfig, staleness StalenessSource, interval IntervalSelector, enqueuer TaskEnqueuer) *AdaptiveScheduler {
	if cfg.BaseInterval <= 0 {
		cfg.BaseInterval = DefaultSchedulerConfig().BaseInterval
	}
	if cfg.MinInterval <= 0 {
		cfg.MinInterval = DefaultSchedulerConfig().MinInterval
	}
	if cfg.MaxInterval <= 0 || cfg.MaxInterval < cfg.MinInterval {
		cfg.MaxInterval = DefaultSchedulerConfig().MaxInterval
	}
	if staleness == nil {
		staleness = noopStalenessSource{}
	}
	if interval == nil {
		interval = newAdaptiveIntervalSelector(cfg)
	}
	if enqueuer == nil {
		enqueuer = noopTaskEnqueuer{}
	}

	return &AdaptiveScheduler{
		cfg:       cfg,
		staleness: staleness,
		interval:  interval,
		enqueuer:  enqueuer,
		lastPlan:  make(map[string]ScheduledTask),
	}
}

// BuildPlan produces an ordered set of scheduled tasks for the supplied inventory.
func (s *AdaptiveScheduler) BuildPlan(now time.Time, inventory []InstanceDescriptor, queueDepth int) []ScheduledTask {
	if len(inventory) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tasks := make([]ScheduledTask, 0, len(inventory))
	for _, inst := range inventory {
		score, ok := s.staleness.StalenessScore(inst.Type, inst.Name)
		if !ok {
			score = 0
		}

		lastScheduled := inst.LastScheduled
		lastInterval := inst.LastInterval
		if cached, exists := s.lastPlan[schedulerKey(inst.Type, inst.Name)]; exists {
			if lastScheduled.IsZero() {
				lastScheduled = cached.NextRun
			}
			if lastInterval == 0 {
				lastInterval = cached.Interval
			}
		}
		if lastInterval == 0 {
			lastInterval = s.cfg.BaseInterval
		}

		currentDepth := queueDepth + len(tasks)
		req := IntervalRequest{
			Now:            now,
			BaseInterval:   s.cfg.BaseInterval,
			MinInterval:    s.cfg.MinInterval,
			MaxInterval:    s.cfg.MaxInterval,
			LastInterval:   lastInterval,
			LastSuccess:    inst.LastSuccess,
			LastScheduled:  lastScheduled,
			StalenessScore: score,
			ErrorCount:     inst.ErrorCount,
			QueueDepth:     currentDepth,
			InstanceKey:    schedulerKey(inst.Type, inst.Name),
			InstanceType:   inst.Type,
		}

		nextInterval := s.interval.SelectInterval(req)
		if nextInterval <= 0 {
			nextInterval = s.cfg.BaseInterval
		}
		if nextInterval < s.cfg.MinInterval {
			nextInterval = s.cfg.MinInterval
		}
		if nextInterval > s.cfg.MaxInterval {
			nextInterval = s.cfg.MaxInterval
		}

		nextRun := now
		if !lastScheduled.IsZero() {
			nextRun = lastScheduled.Add(nextInterval)
		} else if !inst.LastSuccess.IsZero() {
			nextRun = inst.LastSuccess.Add(nextInterval)
		}
		if nextRun.Before(now) {
			nextRun = now
		}

		task := ScheduledTask{
			InstanceName: inst.Name,
			InstanceType: inst.Type,
			NextRun:      nextRun,
			Interval:     nextInterval,
			Priority:     score,
			Metadata:     inst.Metadata,
		}

		s.lastPlan[schedulerKey(inst.Type, inst.Name)] = task
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].NextRun.Equal(tasks[j].NextRun) {
			if tasks[i].Priority == tasks[j].Priority {
				return tasks[i].InstanceName < tasks[j].InstanceName
			}
			return tasks[i].Priority > tasks[j].Priority
		}
		return tasks[i].NextRun.Before(tasks[j].NextRun)
	})

	return tasks
}

// FilterDue returns tasks whose NextRun is at or before now.
func (s *AdaptiveScheduler) FilterDue(now time.Time, tasks []ScheduledTask) []ScheduledTask {
	if len(tasks) == 0 {
		return nil
	}

	due := make([]ScheduledTask, 0, len(tasks))
	for _, task := range tasks {
		if !task.NextRun.After(now) {
			due = append(due, task)
		}
	}
	return due
}

// DispatchDue enqueues due tasks using the configured sink for tracking purposes.
func (s *AdaptiveScheduler) DispatchDue(ctx context.Context, now time.Time, tasks []ScheduledTask) []ScheduledTask {
	if s == nil {
		return tasks
	}
	due := s.FilterDue(now, tasks)
	if len(due) == 0 {
		return due
	}
	for _, task := range due {
		if err := s.enqueuer.Enqueue(ctx, task); err != nil {
			log.Warn().
				Err(err).
				Str("instance", task.InstanceName).
				Str("type", string(task.InstanceType)).
				Msg("Failed to enqueue scheduled task")
		}
	}
	return due
}

// LastScheduled returns the last recorded task for the given instance, if any.
func (s *AdaptiveScheduler) LastScheduled(instanceType InstanceType, instanceName string) (ScheduledTask, bool) {
	if s == nil {
		return ScheduledTask{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.lastPlan[schedulerKey(instanceType, instanceName)]
	return task, ok
}

type noopStalenessSource struct{}

func (noopStalenessSource) StalenessScore(instanceType InstanceType, instanceName string) (float64, bool) {
	return 0, false
}

type adaptiveIntervalSelector struct {
	mu             sync.Mutex
	state          map[string]time.Duration
	rng            *rand.Rand
	alpha          float64
	jitterFraction float64
	queueStretch   float64
	errorPenalty   float64
}

func newAdaptiveIntervalSelector(_ SchedulerConfig) *adaptiveIntervalSelector {
	return &adaptiveIntervalSelector{
		state:          make(map[string]time.Duration),
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
		alpha:          0.6,
		jitterFraction: 0.05,
		queueStretch:   0.1,
		errorPenalty:   0.6,
	}
}

func (a *adaptiveIntervalSelector) SelectInterval(req IntervalRequest) time.Duration {
	min := req.MinInterval
	max := req.MaxInterval
	if max <= 0 || max < min {
		max = min
	}

	score := clampFloat(req.StalenessScore, 0, 1)
	span := float64(max - min)
	// target is mathematically in [min, max] since score ∈ [0,1] and span >= 0
	target := time.Duration(float64(min) + span*(1-score))

	if req.ErrorCount > 0 {
		penalty := 1 + a.errorPenalty*float64(req.ErrorCount)
		if penalty > 0 {
			target = time.Duration(float64(target) / penalty)
			if target < min {
				target = min
			}
		}
	}

	if req.QueueDepth > 1 {
		stretch := 1 + a.queueStretch*float64(req.QueueDepth-1)
		target = time.Duration(float64(target) * stretch)
		if target > max {
			target = max
		}
	}

	base := req.LastInterval
	if base <= 0 {
		base = req.BaseInterval
	}

	var smoothed time.Duration
	key := req.InstanceKey
	if key == "" {
		key = string(req.InstanceType)
	}

	a.mu.Lock()
	prev, ok := a.state[key]
	if ok {
		base = prev
	}
	smoothed = time.Duration(a.alpha*float64(target) + (1-a.alpha)*float64(base))
	if smoothed < min {
		smoothed = min
	}
	if smoothed > max {
		smoothed = max
	}
	a.state[key] = smoothed
	var jitter float64
	if a.jitterFraction > 0 && smoothed > 0 {
		jitter = (a.rng.Float64()*2 - 1) * a.jitterFraction
	}
	a.mu.Unlock()

	if jitter != 0 {
		smoothed = time.Duration(float64(smoothed) * (1 + jitter))
	}

	if smoothed < min {
		smoothed = min
	}
	if smoothed > max {
		smoothed = max
	}

	return smoothed
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

type noopTaskEnqueuer struct{}

func (noopTaskEnqueuer) Enqueue(ctx context.Context, task ScheduledTask) error {
	return nil
}
