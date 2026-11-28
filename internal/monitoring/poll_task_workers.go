package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) pollWorkProgress() *pollWorkProgress {
	if m.pollMetrics == nil {
		return nil
	}
	return m.pollMetrics.workProgress
}

func (m *Monitor) dispatchTaskWorkers(ctx context.Context, limiter *pollTaskWorkerLimiter) {
	defer recoverFromPanic("taskWorkerDispatcher")
	defer m.pollMetrics.trackTaskQueue(ctx, m.taskQueue)()
	progress := m.pollWorkProgress()

	var waiter taskQueueWaiter
	for {
		task, ok := m.taskQueue.waitNext(ctx, &waiter)
		if !ok {
			return
		}
		// The task is no longer in the heap, but may not get a process slot.
		budget := m.taskExecutionTimeout(task.InstanceType)
		due := task.NextRun
		if due.IsZero() {
			due = time.Now()
		}
		work := progress.begin(pollWorkWaitingForSlot, due.Add(budget))
		if !limiter.acquireClaim(ctx, m.taskQueue, task) {
			m.taskQueue.completeClaim(task, ScheduledTask{}, noPollReschedule)
			progress.finish(work)
			if ctx.Err() != nil {
				return
			}
			continue
		}

		progress.start(work, time.Now().Add(budget))
		go func() {
			defer limiter.release()
			m.executeAndRescheduleTask(ctx, task, progress, work)
		}()
	}
}

// A retired claim must not keep the dispatcher away from other eligible
// work. Only the contended path needs a retirement signal.
func (l *pollTaskWorkerLimiter) acquireClaim(ctx context.Context, q *TaskQueue, task ScheduledTask) bool {
	select {
	case <-ctx.Done():
		return false
	case l.slots <- struct{}{}:
	default:
		retired := q.watchClaimRetirement(task)
		if retired == nil {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-retired:
			return false
		case l.slots <- struct{}{}:
		}
	}
	// If the slot and retirement were both ready, retirement wins before any
	// provider work starts. Once admitted, the normal in-flight fences apply.
	if ctx.Err() != nil || !q.canStartClaim(task) {
		l.release()
		return false
	}
	return true
}

func (m *Monitor) taskWorker(ctx context.Context, id int) {
	defer recoverFromPanic(fmt.Sprintf("taskWorker-%d", id))
	defer m.pollMetrics.trackTaskQueue(ctx, m.taskQueue)()
	progress := m.pollWorkProgress()

	if logging.IsLevelEnabled(zerolog.DebugLevel) {
		log.Debug().Int("worker", id).Msg("task worker started")
	}
	var waiter taskQueueWaiter
	for {
		task, ok := m.taskQueue.waitNext(ctx, &waiter)
		if !ok {
			if logging.IsLevelEnabled(zerolog.DebugLevel) {
				log.Debug().Int("worker", id).Msg("task worker stopping")
			}
			return
		}

		work := progress.begin(pollWorkRunning, time.Now().Add(m.taskExecutionTimeout(task.InstanceType)))
		m.executeAndRescheduleTask(ctx, task, progress, work)
	}
}

func (m *Monitor) executeAndRescheduleTask(ctx context.Context, task ScheduledTask, progress *pollWorkProgress, work uint64) {
	// The worker is still occupied until it returns, even after a timeout
	// or while the panic handler is blocked writing to the log sink.
	defer progress.finish(work)

	// Recover at the task boundary so one panic cannot end a reusable worker.
	// The queue claim unwinds before logging; the running marker does not.
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error().
				Str("type", string(task.InstanceType)).
				Str("instance", task.InstanceName).
				Interface("panic", recovered).
				Stack().
				Msg("Recovered from panic in scheduled poll task")
		}
	}()
	var next ScheduledTask
	var reschedule pollReschedule
	defer func() { m.taskQueue.completeClaim(task, next, reschedule) }()

	if ctx.Err() != nil {
		return
	}
	if retryAt := m.scheduledBreakerRetryAt(task); retryAt.After(time.Now()) {
		// A plan or explicit request can be due before the breaker permits a
		// probe. Nothing ran: don't reschedule from the previous result or
		// record a second dead letter for the same failed read.
		next, reschedule = task, pollAfterBackoff
		next.NextRun = retryAt
		return
	}
	m.executeScheduledTask(ctx, task)
	next, reschedule = m.nextScheduledPoll(task)
}
