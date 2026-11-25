package monitoring

import (
	"context"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) dispatchTaskWorkers(ctx context.Context, limiter *pollTaskWorkerLimiter) {
	defer recoverFromPanic("taskWorkerDispatcher")

	for {
		task, ok := m.taskQueue.WaitNext(ctx)
		if !ok {
			return
		}
		if !limiter.acquire(ctx) {
			return
		}

		go func() {
			defer limiter.release()
			defer recoverFromPanic(fmt.Sprintf("taskWorker-%s-%s", task.InstanceType, task.InstanceName))
			m.executeAndRescheduleTask(ctx, task)
		}()
	}
}

func (m *Monitor) taskWorker(ctx context.Context, id int) {
	defer recoverFromPanic(fmt.Sprintf("taskWorker-%d", id))

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

		m.executeAndRescheduleTask(ctx, task)
	}
}

func (m *Monitor) executeAndRescheduleTask(ctx context.Context, task ScheduledTask) {
	m.executeScheduledTask(ctx, task)
	m.rescheduleTask(task)
	m.updateQueueDepthMetric()
}
