package monitoring

import (
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

const schedulerQueueDepthByTypeHelp = "Current scheduler queue depth partitioned by instance type."

// pollQueueCollector reads the active scheduler queues, not the last
// monitor to publish a result or answer Scheduler Health.
type pollQueueCollector struct {
	metrics     *PollMetrics
	depthByType *prometheus.Desc

	// The old sampled gauges support the integration harness, which drives
	// its queue itself. Once a scheduler starts, a sampled value must never
	// replace a live reading, even when the last worker pool stops.
	started atomic.Bool
	mu      sync.Mutex
	workers map[*TaskQueue]int
	seen    map[string]struct{}
}

func newPollQueueCollector(pm *PollMetrics) *pollQueueCollector {
	return &pollQueueCollector{
		metrics: pm,
		depthByType: prometheus.NewDesc("pulse_scheduler_queue_depth", schedulerQueueDepthByTypeHelp,
			[]string{"instance_type"}, nil),
		workers: make(map[*TaskQueue]int),
		seen:    make(map[string]struct{}),
	}
}

func (pm *PollMetrics) usesLiveSchedulerQueues() bool {
	return pm.schedulerQueues != nil && pm.schedulerQueues.started.Load()
}

// trackTaskQueue keeps one reference per worker lifetime but only one queue
// in the process-wide scrape, regardless of the size of the pool.
func (pm *PollMetrics) trackTaskQueue(q *TaskQueue) func() {
	if pm == nil || pm.schedulerQueues == nil || q == nil {
		return func() {}
	}
	c := pm.schedulerQueues
	c.mu.Lock()
	c.workers[q]++
	c.started.Store(true)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.workers[q] == 1 {
			delete(c.workers, q)
		} else {
			c.workers[q]--
		}
	}
}

func (c *pollQueueCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metrics.queueDepth.Desc()
	ch <- c.metrics.schedulerQueueReady.Desc()
	ch <- c.depthByType
}

func (c *pollQueueCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.started.Load() {
		c.metrics.queueDepth.Collect(ch)
		c.metrics.schedulerQueueReady.Collect(ch)
		c.metrics.schedulerQueueDepthByType.Collect(ch)
		return
	}

	c.mu.Lock()
	queues := make([]*TaskQueue, 0, len(c.workers))
	for q := range c.workers {
		queues = append(queues, q)
	}
	c.mu.Unlock()

	depth, due := 0, 0
	byType := make(map[string]int)
	for _, q := range queues {
		// Snapshot counts only heap entries, at the time of the read.
		// Do not hold the registry lock while acquiring a queue lock.
		snapshot := q.Snapshot()
		depth += snapshot.Depth
		due += snapshot.DueWithinSeconds
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
	for name, count := range byType {
		ch <- prometheus.MustNewConstMetric(c.depthByType, prometheus.GaugeValue, float64(count), name)
	}
}
