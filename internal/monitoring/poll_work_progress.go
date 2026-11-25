package monitoring

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type pollWorkStage uint8

const (
	pollWorkWaitingForSlot pollWorkStage = iota
	pollWorkRunning
	pollWorkStageCount
)

var pollWorkStageNames = [pollWorkStageCount]string{"waiting_for_slot", "running"}

type pollWorkDeadline struct {
	stage    pollWorkStage
	deadline time.Time
}

// pollWorkProgress tracks only claimed slots and active workers, never the
// scheduled inventory. No lock is shared with the scheduler, a monitor or a
// provider. A scrape can measure a task even when it never returns.
type pollWorkProgress struct {
	overdue *prometheus.Desc
	mu      sync.Mutex
	lastID  uint64
	active  map[uint64]pollWorkDeadline
}

func newPollWorkProgress() *pollWorkProgress {
	return &pollWorkProgress{
		overdue: prometheus.NewDesc(
			"pulse_scheduler_claimed_poll_overdue_seconds",
			"Seconds past one poll task timeout for the most overdue claimed poll in each stage; zero if none.",
			[]string{"stage"}, nil,
		),
		active: make(map[uint64]pollWorkDeadline),
	}
}

func (p *pollWorkProgress) begin(stage pollWorkStage, deadline time.Time) uint64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastID++
	p.active[p.lastID] = pollWorkDeadline{stage: stage, deadline: deadline}
	return p.lastID
}

func (p *pollWorkProgress) start(id uint64, deadline time.Time) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.active[id] = pollWorkDeadline{stage: pollWorkRunning, deadline: deadline}
	p.mu.Unlock()
}

func (p *pollWorkProgress) finish(id uint64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	delete(p.active, id)
	p.mu.Unlock()
}

func (p *pollWorkProgress) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.overdue
}

func (p *pollWorkProgress) Collect(ch chan<- prometheus.Metric) {
	var overdue [pollWorkStageCount]time.Duration
	p.mu.Lock()
	now := time.Now()
	for _, work := range p.active {
		overdue[work.stage] = max(overdue[work.stage], now.Sub(work.deadline))
	}
	p.mu.Unlock()

	for stage, wait := range overdue {
		ch <- prometheus.MustNewConstMetric(p.overdue, prometheus.GaugeValue, wait.Seconds(), pollWorkStageNames[stage])
	}
}
