package monitoring

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// PollResult represents the result of a polling operation
type PollResult struct {
	InstanceName string
	InstanceType string // "pve", "pbs", or "pmg"
	Success      bool
	Error        error
	StartTime    time.Time
	EndTime      time.Time
}

// PollTask represents a polling task to be executed
type PollTask struct {
	InstanceName string
	InstanceType string // "pve", "pbs", or "pmg"
	PVEClient    PVEClientInterface
	PBSClient    *pbs.Client
	PMGClient    *pmg.Client
}

// PollerPool manages concurrent polling with channels
type PollerPool struct {
	workers     int
	tasksChan   chan PollTask
	resultsChan chan PollResult
	monitor     *Monitor
	done        chan struct{}
	closed      bool
}

// NewPollerPool creates a new poller pool
func NewPollerPool(workers int, monitor *Monitor) *PollerPool {
	return &PollerPool{
		workers:     workers,
		tasksChan:   make(chan PollTask, workers*2), // Buffer for smooth operation
		resultsChan: make(chan PollResult, workers*2),
		monitor:     monitor,
		done:        make(chan struct{}),
		closed:      false,
	}
}

// Start starts the worker pool
func (p *PollerPool) Start(ctx context.Context) {
	// Start workers
	for i := 0; i < p.workers; i++ {
		go p.worker(ctx, i)
	}

	// Start result collector
	go p.collectResults(ctx)
}

// worker processes polling tasks
func (p *PollerPool) worker(ctx context.Context, id int) {
	if logging.IsLevelEnabled(zerolog.DebugLevel) {
		log.Debug().Int("worker", id).Msg("Poller worker started")
	}

	for {
		select {
		case <-ctx.Done():
			if logging.IsLevelEnabled(zerolog.DebugLevel) {
				log.Debug().Int("worker", id).Msg("Poller worker stopped")
			}
			return
		case task, ok := <-p.tasksChan:
			if !ok {
				if logging.IsLevelEnabled(zerolog.DebugLevel) {
					log.Debug().Int("worker", id).Msg("Task channel closed, worker stopping")
				}
				return
			}

			result := p.executeTask(ctx, task)

			// Send result if context is still active and channel is open
			select {
			case <-ctx.Done():
				return
			default:
				// Use non-blocking send to avoid panic if channel is closed
				select {
				case p.resultsChan <- result:
				case <-ctx.Done():
					return
				default:
					// Channel might be closed, just continue
					if logging.IsLevelEnabled(zerolog.DebugLevel) {
						log.Debug().Int("worker", id).Msg("Results channel appears closed, skipping result")
					}
				}
			}
		}
	}
}

// executeTask executes a single polling task
func (p *PollerPool) executeTask(ctx context.Context, task PollTask) PollResult {
	result := PollResult{
		InstanceName: task.InstanceName,
		InstanceType: task.InstanceType,
		StartTime:    time.Now(),
		Success:      true,
	}

	switch task.InstanceType {
	case "pve":
		if task.PVEClient != nil {
			p.monitor.pollPVEInstance(ctx, task.InstanceName, task.PVEClient)
		} else {
			result.Success = false
			result.Error = errors.NewMonitorError(errors.ErrorTypeInternal, "poll_pve", task.InstanceName, errors.ErrInvalidInput)
		}
	case "pbs":
		if task.PBSClient != nil {
			p.monitor.pollPBSInstance(ctx, task.InstanceName, task.PBSClient)
		} else {
			result.Success = false
			result.Error = errors.NewMonitorError(errors.ErrorTypeInternal, "poll_pbs", task.InstanceName, errors.ErrInvalidInput)
		}
	case "pmg":
		if task.PMGClient != nil {
			p.monitor.pollPMGInstance(ctx, task.InstanceName, task.PMGClient)
		} else {
			result.Success = false
			result.Error = errors.NewMonitorError(errors.ErrorTypeInternal, "poll_pmg", task.InstanceName, errors.ErrInvalidInput)
		}
	default:
		result.Success = false
		result.Error = errors.NewMonitorError(errors.ErrorTypeValidation, "poll_unknown", task.InstanceName, errors.ErrInvalidInput)
	}

	result.EndTime = time.Now()
	return result
}

// collectResults collects polling results
func (p *PollerPool) collectResults(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-p.resultsChan:
			if !ok {
				return
			}

			duration := result.EndTime.Sub(result.StartTime)
			if result.Success {
				log.Debug().
					Str("instance", result.InstanceName).
					Str("type", result.InstanceType).
					Dur("duration", duration).
					Msg("Polling completed successfully")
			} else {
				log.Error().
					Err(result.Error).
					Str("instance", result.InstanceName).
					Str("type", result.InstanceType).
					Dur("duration", duration).
					Msg("Polling failed; request will be retried on next cycle")
			}
		}
	}
}

// SubmitTask submits a polling task
func (p *PollerPool) SubmitTask(ctx context.Context, task PollTask) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.tasksChan <- task:
		return nil
	default:
		// Channel is full
		return errors.NewMonitorError(errors.ErrorTypeInternal, "submit_task", task.InstanceName, errors.ErrTimeout)
	}
}

// Close closes the poller pool
func (p *PollerPool) Close() {
	if p.closed {
		return
	}
	p.closed = true

	// Signal shutdown
	close(p.done)

	// Close task channel to signal workers to stop
	close(p.tasksChan)

	// Don't close resultsChan here - let it drain naturally
	// The collectors will exit when context is done
}

// pollWithChannels implements channel-based concurrent polling
func (m *Monitor) pollWithChannels(ctx context.Context) {
	// Create worker pool based on instance count
	workerCount := len(m.pveClients) + len(m.pbsClients) + len(m.pmgClients)
	if workerCount > 10 {
		workerCount = 10 // Cap at 10 workers
	}
	if workerCount < 2 {
		workerCount = 2 // Minimum 2 workers
	}

	pool := NewPollerPool(workerCount, m)

	// Create a context with timeout for this polling cycle
	// Hardcoded to 10s minus 200ms (matches polling interval)
	timeout := 10*time.Second - 200*time.Millisecond
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Start the pool
	pool.Start(pollCtx)

	// Submit all tasks
	var taskCount int

	// Submit PVE tasks
	for name, client := range m.pveClients {
		task := PollTask{
			InstanceName: name,
			InstanceType: "pve",
			PVEClient:    client,
		}
		if err := pool.SubmitTask(pollCtx, task); err != nil {
			log.Error().Err(err).Str("instance", name).Msg("Failed to submit PVE polling task")
		} else {
			taskCount++
		}
	}

	// Submit PBS tasks
	for name, client := range m.pbsClients {
		task := PollTask{
			InstanceName: name,
			InstanceType: "pbs",
			PBSClient:    client,
		}
		if err := pool.SubmitTask(pollCtx, task); err != nil {
			log.Error().Err(err).Str("instance", name).Msg("Failed to submit PBS polling task")
		} else {
			taskCount++
		}
	}

	// Submit PMG tasks
	for name, client := range m.pmgClients {
		task := PollTask{
			InstanceName: name,
			InstanceType: "pmg",
			PMGClient:    client,
		}
		if err := pool.SubmitTask(pollCtx, task); err != nil {
			log.Error().Err(err).Str("instance", name).Msg("Failed to submit PMG polling task")
		} else {
			taskCount++
		}
	}

	// Wait for all tasks to complete or timeout
	<-pollCtx.Done()

	// Clean up
	pool.Close()

	log.Debug().Int("tasks", taskCount).Msg("Channel-based polling cycle completed")
}
