package monitoring

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

const schedulerQueueDepthByTypeHelp = "Current scheduler queue depth partitioned by instance type."

// pollQueueCollector reads the active scheduler queues, not the last
// monitor to publish a result or answer Scheduler Health.
type pollQueueCollector struct {
	metrics       *PollMetrics
	depthByType   *prometheus.Desc
	queuedOverdue *prometheus.Desc

	// The old sampled gauges support the integration harness, which drives
	// its queue itself. Once a scheduler starts, a sampled value must never
	// replace a live reading, even when the last worker pool stops.
	started atomic.Bool
	mu      sync.Mutex
	workers map[pollQueueRuntime]int
	seen    map[string]struct{}
}

type pollQueueRuntime struct {
	queue *TaskQueue
	done  <-chan struct{}
}

func (r pollQueueRuntime) active() bool {
	select {
	case <-r.done:
		return false
	default:
		return true
	}
}

func newPollQueueCollector(pm *PollMetrics) *pollQueueCollector {
	return &pollQueueCollector{
		metrics: pm,
		depthByType: prometheus.NewDesc("pulse_scheduler_queue_depth", schedulerQueueDepthByTypeHelp,
			[]string{"instance_type"}, nil),
		queuedOverdue: prometheus.NewDesc("pulse_scheduler_queued_poll_overdue_seconds",
			"Seconds past the scheduled time of the most overdue timestamped poll still queued in running schedulers; zero if none.",
			nil, nil),
		workers: make(map[pollQueueRuntime]int),
		seen:    make(map[string]struct{}),
	}
}

func (pm *PollMetrics) usesLiveSchedulerQueues() bool {
	return pm.schedulerQueues != nil && pm.schedulerQueues.started.Load()
}

// trackTaskQueue keeps one reference per worker, sharing a registration for
// workers with the same queue and scheduler lifetime.
func (pm *PollMetrics) trackTaskQueue(ctx context.Context, q *TaskQueue) func() {
	if pm == nil || pm.schedulerQueues == nil || q == nil || ctx.Err() != nil {
		return func() {}
	}
	c := pm.schedulerQueues
	runtime := pollQueueRuntime{queue: q, done: ctx.Done()}
	c.mu.Lock()
	c.workers[runtime]++
	c.started.Store(true)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.workers[runtime] == 1 {
			delete(c.workers, runtime)
		} else {
			c.workers[runtime]--
		}
	}
}

func (c *pollQueueCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metrics.queueDepth.Desc()
	ch <- c.metrics.schedulerQueueReady.Desc()
	ch <- c.depthByType
	ch <- c.queuedOverdue
}

func (c *pollQueueCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.started.Load() {
		c.metrics.queueDepth.Collect(ch)
		c.metrics.schedulerQueueReady.Collect(ch)
		c.metrics.schedulerQueueDepthByType.Collect(ch)
		// Legacy samples contain counts, not live queued work.
		ch <- prometheus.MustNewConstMetric(c.queuedOverdue, prometheus.GaugeValue, 0)
		return
	}

	c.mu.Lock()
	runtimes := make([]pollQueueRuntime, 0, len(c.workers))
	for runtime := range c.workers {
		runtimes = append(runtimes, runtime)
	}
	c.mu.Unlock()

	depth, due := 0, 0
	var oldestOverdue float64
	byType := make(map[string]int)
	counted := make(map[*TaskQueue]struct{}, len(runtimes))
	for _, runtime := range runtimes {
		if _, ok := counted[runtime.queue]; ok || !runtime.active() {
			continue
		}
		// Snapshot counts only heap entries, at the time of the read.
		// Do not hold the registry lock while acquiring a queue lock.
		snapshot := runtime.queue.Snapshot()
		if !runtime.active() {
			continue // The scheduler stopped while we read its queue.
		}
		counted[runtime.queue] = struct{}{}
		depth += snapshot.Depth
		due += snapshot.DueWithinSeconds
		oldestOverdue = max(oldestOverdue, snapshot.OldestOverdueSeconds)
		for name, count := range snapshot.PerType {
			byType[normalizeLabel(name)] += count
		}
	}

	c.mu.Lock()
	for name := range byType {
		c.seen[name] = struct{}{}
	}
	for name := range c.seen {
		if _, ok := byType[name]; !ok {
			byType[name] = 0
		}
	}
	c.mu.Unlock()

	ch <- prometheus.MustNewConstMetric(c.metrics.queueDepth.Desc(), prometheus.GaugeValue, float64(depth))
	ch <- prometheus.MustNewConstMetric(c.metrics.schedulerQueueReady.Desc(), prometheus.GaugeValue, float64(due))
	ch <- prometheus.MustNewConstMetric(c.queuedOverdue, prometheus.GaugeValue, oldestOverdue)
	for name, count := range byType {
		ch <- prometheus.MustNewConstMetric(c.depthByType, prometheus.GaugeValue, float64(count), name)
	}
}
