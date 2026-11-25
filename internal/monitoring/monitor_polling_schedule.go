package monitoring

import (
	"context"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type pollPlanInstance struct {
	name          string
	fixedInterval time.Duration
}

// A provider that stores its roster and cadences together can return both from
// one read. The planner owns the returned slice and sorts it by instance ID.
// This is not a cache; a worker still checks the current saved target at dispatch.
type pollPlanningRosterProvider interface {
	pollPlanningRoster(m *Monitor) []pollPlanInstance
}

func (m *Monitor) describeInstancesForScheduler() []InstanceDescriptor {
	var descriptors []InstanceDescriptor
	for _, provider := range m.pollProviderSnapshotWithBuiltins() {
		if provider == nil {
			continue
		}
		providerType := provider.Type()
		first := len(descriptors)
		if batch, ok := provider.(pollPlanningRosterProvider); ok {
			roster := batch.pollPlanningRoster(m)
			slices.SortFunc(roster, func(left, right pollPlanInstance) int {
				return strings.Compare(left.name, right.name)
			})
			descriptors = slices.Grow(descriptors, len(roster))
			for _, instance := range roster {
				descriptors = append(descriptors, InstanceDescriptor{
					Name: instance.name, Type: providerType, FixedInterval: instance.fixedInterval,
				})
			}
		} else {
			names := slices.Clone(provider.ListInstances(m))
			slices.Sort(names)
			descriptors = slices.Grow(descriptors, len(names))
			fixedProvider, _ := provider.(FixedIntervalPollProvider)
			for _, name := range names {
				desc := InstanceDescriptor{Name: name, Type: providerType}
				if fixedProvider != nil {
					desc.FixedInterval = fixedProvider.FixedInstanceInterval(m, name)
				}
				descriptors = append(descriptors, desc)
			}
		}

		for i := first; i < len(descriptors); i++ {
			desc := &descriptors[i]
			if m.scheduler != nil {
				if last, ok := m.scheduler.LastScheduled(providerType, desc.Name); ok {
					desc.LastScheduled = last.NextRun
					desc.LastInterval = last.Interval
				}
			}
			if m.stalenessTracker != nil {
				if snap, ok := m.stalenessTracker.snapshot(providerType, desc.Name); ok {
					desc.LastSuccess = snap.LastSuccess
					desc.LastFailure = snap.LastError
					desc.Metadata = TaskMetadata{ChangeHash: snap.ChangeHash}
				}
			}
		}
	}

	return descriptors
}

func (m *Monitor) buildScheduledTasks(now time.Time) []ScheduledTask {
	descriptors := m.describeInstancesForScheduler()
	if len(descriptors) == 0 {
		return nil
	}

	queueDepth := 0
	if m.taskQueue != nil {
		queueDepth = m.taskQueue.Size()
	}

	if m.scheduler == nil {
		tasks := make([]ScheduledTask, 0, len(descriptors))
		for _, desc := range descriptors {
			interval := desc.FixedInterval
			if interval <= 0 {
				interval = m.baseIntervalForInstanceType(desc.Type)
			}
			if interval <= 0 {
				interval = DefaultSchedulerConfig().BaseInterval
			}
			// New targets are due immediately. The queue reconciles this
			// proposal against any already queued target under its own lock.
			tasks = append(tasks, ScheduledTask{
				InstanceName: desc.Name,
				InstanceType: desc.Type,
				NextRun:      now,
				Interval:     interval,
			})
		}
		return tasks
	}

	return m.scheduler.BuildPlan(now, descriptors, queueDepth)
}

// poll fetches data from all configured instances
func (m *Monitor) poll(_ context.Context, wsHub *websocket.Hub) {
	defer recoverFromPanic("poll")

	// Limit concurrent polls to 2 to prevent resource exhaustion
	currentCount := atomic.AddInt32(&m.activePollCount, 1)
	if currentCount > 2 {
		atomic.AddInt32(&m.activePollCount, -1)
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Int32("activePolls", currentCount-1).Msg("too many concurrent polls, skipping")
		}
		return
	}
	defer atomic.AddInt32(&m.activePollCount, -1)

	if logging.IsLevelEnabled(zerolog.DebugLevel) {
		log.Debug().Msg("starting polling cycle")
	}
	startTime := time.Now()
	now := startTime

	plannedTasks := m.buildScheduledTasks(now)
	for _, task := range plannedTasks {
		m.taskQueue.upsertPlan(task)
	}

	// Update performance metrics atomically to prevent data races when
	// multiple poll() goroutines run concurrently (e.g. mock mode transitions).
	wsClients := 0
	if wsHub != nil {
		wsClients = wsHub.GetClientCount()
	}
	m.state.UpdatePollStats(
		time.Since(startTime).Seconds(),
		int64(time.Since(m.startTime).Seconds()),
		wsClients,
	)

	// Sync alert state so broadcasts include the latest acknowledgement data
	m.syncAlertsToState()

	if logging.IsLevelEnabled(zerolog.DebugLevel) {
		log.Debug().Dur("duration", time.Since(startTime)).Msg("polling cycle completed")
	}

	// Broadcasting is now handled by the timer in Start()
}

// scheduledBreakerRetryAt observes the admission fence without consuming its
// half-open probe. executeScheduledTask still asks the breaker for admission.
func (m *Monitor) scheduledBreakerRetryAt(task ScheduledTask) time.Time {
	key := schedulerKey(task.InstanceType, task.InstanceName)
	m.mu.RLock()
	breaker := m.circuitBreakers[key]
	m.mu.RUnlock()
	if breaker == nil {
		return time.Time{}
	}
	_, _, retryAt := breaker.State()
	return retryAt
}

// nextScheduledPoll computes the normal successor without unlocking the
// current claim. The queue installs it only when that worker returns.
func (m *Monitor) nextScheduledPoll(task ScheduledTask) (ScheduledTask, pollReschedule) {
	key := schedulerKey(task.InstanceType, task.InstanceName)
	m.mu.Lock()
	outcome, hasOutcome := m.lastOutcome[key]
	failureCount := m.failureCounts[key]
	m.mu.Unlock()

	if hasOutcome && !outcome.success {
		if !outcome.transient || failureCount >= m.maxRetryAttempts {
			m.sendToDeadLetter(task, outcome.err)
			return ScheduledTask{}, noPollReschedule
		}
		delay := m.backoffCfg.nextDelay(failureCount-1, m.randomFloat())
		if delay <= 0 {
			delay = 5 * time.Second
		}
		if m.config != nil && m.config.AdaptivePollingEnabled && m.config.AdaptivePollingMaxInterval > 0 && m.config.AdaptivePollingMaxInterval <= 15*time.Second {
			maxDelay := 4 * time.Second
			if delay > maxDelay {
				delay = maxDelay
			}
		}
		next := task
		next.Interval = delay
		next.NextRun = time.Now().Add(delay)
		if retryAt := m.scheduledBreakerRetryAt(task); retryAt.After(next.NextRun) {
			next.NextRun = retryAt
		}
		return next, pollAfterBackoff
	}

	fixedInterval := m.fixedIntervalForInstance(task.InstanceType, task.InstanceName)

	if m.scheduler == nil {
		nextInterval := fixedInterval
		if nextInterval <= 0 {
			nextInterval = task.Interval
		}
		if nextInterval <= 0 {
			nextInterval = m.baseIntervalForInstanceType(task.InstanceType)
		}
		if nextInterval <= 0 {
			nextInterval = DefaultSchedulerConfig().BaseInterval
		}
		next := task
		next.NextRun = time.Now().Add(nextInterval)
		next.Interval = nextInterval
		return next, pollOnCadence
	}

	desc := InstanceDescriptor{
		Name:          task.InstanceName,
		Type:          task.InstanceType,
		LastInterval:  task.Interval,
		LastScheduled: task.NextRun,
		FixedInterval: fixedInterval,
	}
	if m.stalenessTracker != nil {
		if snap, ok := m.stalenessTracker.snapshot(task.InstanceType, task.InstanceName); ok {
			desc.LastSuccess = snap.LastSuccess
			desc.LastFailure = snap.LastError
			if snap.ChangeHash != "" {
				desc.Metadata = TaskMetadata{ChangeHash: snap.ChangeHash}
			}
		}
	}

	tasks := m.scheduler.BuildPlan(time.Now(), []InstanceDescriptor{desc}, m.taskQueue.Size())
	if len(tasks) == 0 {
		next := task
		nextInterval := task.Interval
		if nextInterval <= 0 && m.config != nil {
			nextInterval = m.config.AdaptivePollingBaseInterval
		}
		if nextInterval <= 0 {
			nextInterval = DefaultSchedulerConfig().BaseInterval
		}
		next.Interval = nextInterval
		next.NextRun = time.Now().Add(nextInterval)
		return next, pollOnCadence
	}
	return tasks[0], pollOnCadence
}
