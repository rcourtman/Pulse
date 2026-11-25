package monitoring

import (
	"context"
	"sort"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) describeInstancesForScheduler() []InstanceDescriptor {
	providers := m.pollProviderSnapshotWithBuiltins()
	if len(providers) == 0 {
		return nil
	}

	total := 0
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		total += len(provider.ListInstances(m))
	}
	if total == 0 {
		return nil
	}

	descriptors := make([]InstanceDescriptor, 0, total)
	for _, provider := range providers {
		if provider == nil {
			continue
		}

		names := append([]string(nil), provider.ListInstances(m)...)
		if len(names) == 0 {
			continue
		}
		sort.Strings(names)

		providerType := provider.Type()
		fixedProvider, _ := provider.(FixedIntervalPollProvider)
		for _, name := range names {
			desc := InstanceDescriptor{
				Name: name,
				Type: providerType,
			}
			if fixedProvider != nil {
				desc.FixedInterval = fixedProvider.FixedInstanceInterval(m, name)
			}
			if m.scheduler != nil {
				if last, ok := m.scheduler.LastScheduled(providerType, name); ok {
					desc.LastScheduled = last.NextRun
					desc.LastInterval = last.Interval
				}
			}
			if m.stalenessTracker != nil {
				if snap, ok := m.stalenessTracker.snapshot(providerType, name); ok {
					desc.LastSuccess = snap.LastSuccess
					desc.LastFailure = snap.LastError
					desc.Metadata = TaskMetadata{ChangeHash: snap.ChangeHash}
				}
			}
			descriptors = append(descriptors, desc)
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
			// Planning passes run every poll tick, and Upsert overwrites the
			// queued slot. Stamping NextRun=now here re-arms every instance as
			// due immediately each tick, so a fixed-interval instance polls at
			// the tick cadence instead of its configured interval (#1745).
			// Keep a pending future slot, tightening only when the freshly
			// computed interval justifies an earlier run, mirroring the
			// adaptive path's #1437 handling.
			nextRun := now
			if m.taskQueue != nil {
				if pending, ok := m.taskQueue.Get(desc.Type, desc.Name); ok && pending.NextRun.After(now) {
					nextRun = pending.NextRun
					if candidate := now.Add(interval); candidate.Before(nextRun) {
						nextRun = candidate
					}
				}
			}
			tasks = append(tasks, ScheduledTask{
				InstanceName: desc.Name,
				InstanceType: desc.Type,
				NextRun:      nextRun,
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
		m.taskQueue.Upsert(task)
	}
	m.updateQueueDepthMetric()

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

func (m *Monitor) rescheduleTask(task ScheduledTask) {
	if m.taskQueue == nil {
		return
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)
	m.mu.Lock()
	outcome, hasOutcome := m.lastOutcome[key]
	failureCount := m.failureCounts[key]
	m.mu.Unlock()

	if hasOutcome && !outcome.success {
		if !outcome.transient || failureCount >= m.maxRetryAttempts {
			m.sendToDeadLetter(task, outcome.err)
			return
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
		m.taskQueue.Upsert(next)
		return
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
		m.taskQueue.Upsert(next)
		return
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
		m.taskQueue.Upsert(next)
		return
	}
	for _, next := range tasks {
		m.taskQueue.Upsert(next)
	}
}
