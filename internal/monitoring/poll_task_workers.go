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
	progress := m.pollWorkProgress()

	for {
		task, ok := m.taskQueue.WaitNext(ctx)
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
		if !limiter.acquire(ctx) {
			progress.finish(work)
			return
		}

		progress.start(work, time.Now().Add(budget))
		go func() {
			defer limiter.release()
			defer recoverFromPanic(fmt.Sprintf("taskWorker-%s-%s", task.InstanceType, task.InstanceName))
			m.executeAndRescheduleTask(ctx, task, progress, work)
		}()
	}
}

func (m *Monitor) taskWorker(ctx context.Context, id int) {
	defer recoverFromPanic(fmt.Sprintf("taskWorker-%d", id))
	progress := m.pollWorkProgress()

	if logging.IsLevelEnabled(zerolog.DebugLevel) {
		log.Debug().Int("worker", id).Msg("task worker started")
	}
	for {
		task, ok := m.taskQueue.WaitNext(ctx)
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
	// Do not clear this at context expiry. An unreturned worker is still busy.
	defer progress.finish(work)

	m.executeScheduledTask(ctx, task)
	m.rescheduleTask(task)
	m.updateQueueDepthMetric()
}
