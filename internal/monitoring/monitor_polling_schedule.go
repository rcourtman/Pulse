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

// An admitted provider read may outlive cancellation. Check at the roster
// and cadence boundaries so a retired monitor does not start another read.
func (m *Monitor) describeInstancesForScheduler(ctx context.Context) []InstanceDescriptor {
	if ctx.Err() != nil {
		return nil
	}
	var descriptors []InstanceDescriptor
	for _, provider := range m.pollProviderSnapshotWithBuiltins() {
		if ctx.Err() != nil {
			return nil
		}
		if provider == nil {
			continue
		}
		providerType := provider.Type()
		if batch, ok := provider.(pollPlanningRosterProvider); ok {
			roster := batch.pollPlanningRoster(m)
			if ctx.Err() != nil {
				return nil
			}
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
			names := provider.ListInstances(m)
			if ctx.Err() != nil {
				return nil
			}
			// Built-in Proxmox providers already return names in order.
			// Only clone when we need to sort: another provider may own
			// and reuse the returned slice.
			if !slices.IsSorted(names) {
				names = slices.Clone(names)
				slices.Sort(names)
			}
			descriptors = slices.Grow(descriptors, len(names))
			fixedProvider, _ := provider.(FixedIntervalPollProvider)
			for _, name := range names {
				desc := InstanceDescriptor{Name: name, Type: providerType}
				if fixedProvider != nil {
					if ctx.Err() != nil {
						return nil
					}
					desc.FixedInterval = fixedProvider.FixedInstanceInterval(m, name)
				}
				descriptors = append(descriptors, desc)
			}
		}
	}

	return descriptors
}

func (m *Monitor) buildScheduledTasks(now time.Time) []ScheduledTask {
	return m.schedulePollDescriptors(now, m.describeInstancesForScheduler(context.Background()))
}

func (m *Monitor) schedulePollDescriptors(now time.Time, descriptors []InstanceDescriptor) []ScheduledTask {
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

	// The planner reads the prior slot and interval under its own lock.
	// Staging those earlier could mask a worker's newer successor.
	return m.scheduler.buildPlan(now, descriptors, queueDepth, m.stalenessTracker)
}

func (m *Monitor) planScheduledPolls(ctx context.Context, now time.Time) {
	if m.pollMetrics != nil {
		progress := m.pollMetrics.workProgress
		id := progress.beginPlanning(time.Now())
		defer progress.finishPlanning(id)
	}
	if ctx.Err() != nil {
		return
	}
	// Bind queue admission to the roster revision before reading providers.
	// A settings refresh can finish while an older planner is still reading.
	revision := m.taskQueue.availabilityPlanRevision()
	descriptors := m.describeInstancesForScheduler(ctx)
	// Roster reads can outlive the monitor that started them. Do not write
	// their results to the adaptive plan cache or queue once retired.
	if ctx.Err() != nil {
		return
	}
	for _, task := range m.schedulePollDescriptors(now, descriptors) {
		if ctx.Err() != nil {
			return
		}
		m.taskQueue.upsertPlan(task, revision)
	}
}

// poll fetches data from all configured instances
func (m *Monitor) poll(ctx context.Context, wsHub *websocket.Hub) {
	defer recoverFromPanic("poll")
	if ctx.Err() != nil {
		return
	}

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
	m.planScheduledPolls(ctx, startTime)
	if ctx.Err() != nil {
		return
	}

	// Update performance metrics atomically to prevent data races when
	// multiple poll() goroutines run concurrently (e.g. mock mode transitions).
	wsClients := 0
	if wsHub != nil {
		wsClients = wsHub.GetClientCount()
	}
	if ctx.Err() != nil {
		return
	}
	m.state.UpdatePollStats(
		time.Since(startTime).Seconds(),
		int64(time.Since(m.startTime).Seconds()),
		wsClients,
	)

	// Sync alert state so broadcasts include the latest acknowledgement data
	if ctx.Err() == nil {
		m.syncAlertsToState()
	}

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
	// The worker still owns this connection's claim. A later result accepted
	// as success by the scheduler ends the terminal failure episode. A partial
	// read can do so too; its outstanding error still lives in pollStatusMap.
	retiredDeadLetter := false
	if hasOutcome && outcome.success && m.deadLetterQueue != nil {
		if record := m.dlqInsightMap[key]; record != nil && !outcome.recordedAt.Before(record.LastAttempt) {
			m.deadLetterQueue.Remove(task.InstanceType, task.InstanceName)
			delete(m.dlqInsightMap, key)
			retiredDeadLetter = true
		}
	}
	m.mu.Unlock()
	if retiredDeadLetter {
		m.pollMetrics.clearDeadLetterCount(string(task.InstanceType), task.InstanceName)
	}

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
	return m.scheduler.planSuccessor(time.Now(), desc, m.taskQueue.Size(), m.stalenessTracker), pollOnCadence
}
