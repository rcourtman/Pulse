package monitoring

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

const schedulerDeadLetterDepthHelp = "Number of tasks currently parked in the dead-letter queue per instance."

// The queue is the only source of live depth. A sampled gauge may outlive
// both the entry and its scheduler, so snapshots never write back to it.
type pollDeadLetterCollector struct {
	sampled *prometheus.GaugeVec
	depth   *prometheus.Desc
	started atomic.Bool

	mu      sync.Mutex
	sources map[*pollDeadLetterRuntime]struct{}
	seen    map[metricKey]struct{}
}

type pollDeadLetterRuntime struct {
	queue *TaskQueue
	done  <-chan struct{}
}

func (s *pollDeadLetterRuntime) active() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

func newPollDeadLetterCollector(sampled *prometheus.GaugeVec) *pollDeadLetterCollector {
	return &pollDeadLetterCollector{
		sampled: sampled,
		depth: prometheus.NewDesc("pulse_scheduler_dead_letter_depth", schedulerDeadLetterDepthHelp,
			[]string{"instance_type", "instance"}, nil),
		sources: make(map[*pollDeadLetterRuntime]struct{}),
		seen:    make(map[metricKey]struct{}),
	}
}

func (pm *PollMetrics) usesLiveDeadLetters() bool {
	return pm.schedulerDeadLetters != nil && pm.schedulerDeadLetters.started.Load()
}

// trackDeadLetterQueue is called once by the owner of a running scheduler,
// not by a scrape or a poll result. In particular two live runtimes with
// the same instance labels are separate sources.
func (pm *PollMetrics) trackDeadLetterQueue(ctx context.Context, q *TaskQueue) func() {
	if pm == nil || pm.schedulerDeadLetters == nil || q == nil || ctx.Err() != nil {
		return func() {}
	}
	c := pm.schedulerDeadLetters
	source := &pollDeadLetterRuntime{queue: q, done: ctx.Done()}
	c.mu.Lock()
	c.sources[source] = struct{}{}
	c.started.Store(true)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		delete(c.sources, source)
		c.mu.Unlock()
	}
}

func (c *pollDeadLetterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.depth
}

func (c *pollDeadLetterCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.started.Load() {
		c.sampled.Collect(ch)
		return
	}

	c.mu.Lock()
	sources := make([]*pollDeadLetterRuntime, 0, len(c.sources))
	for source := range c.sources {
		sources = append(sources, source)
	}
	c.mu.Unlock()

	counts := make(map[metricKey]int)
	for _, source := range sources {
		if !source.active() {
			continue
		}
		tasks := source.queue.PeekAll(0)
		if !source.active() {
			continue // The runtime retired while we waited for the queue.
		}
		for _, task := range tasks {
			counts[makeMetricKey(task.Type, task.Instance)]++
		}
	}

	c.mu.Lock()
	for key := range counts {
		c.seen[key] = struct{}{}
	}
	// Keep a zero for a previously scraped label. Never cache positive
	// samples here, so an older scrape cannot restore a retired queue.
	for key := range c.seen {
		if _, ok := counts[key]; !ok {
			counts[key] = 0
		}
	}
	c.mu.Unlock()

	for key, count := range counts {
		ch <- prometheus.MustNewConstMetric(c.depth, prometheus.GaugeValue, float64(count), key.instanceType, key.instance)
	}
}
