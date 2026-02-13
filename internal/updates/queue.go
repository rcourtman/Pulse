package updates

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// JobState represents the state of an update job
type JobState string

const (
	JobStateIdle      JobState = "idle"
	JobStateQueued    JobState = "queued"
	JobStateRunning   JobState = "running"
	JobStateCompleted JobState = "completed"
	JobStateFailed    JobState = "failed"
	JobStateCancelled JobState = "cancelled"
)

const updatesQueueComponent = "updates_queue"

// UpdateJob represents a single update job
type UpdateJob struct {
	ID          string
	DownloadURL string
	State       JobState
	StartedAt   time.Time
	CompletedAt time.Time
	Error       error
	Context     context.Context
	Cancel      context.CancelFunc
}

// UpdateQueue manages the update job queue ensuring only one update runs at a time
type UpdateQueue struct {
	mu         sync.RWMutex
	currentJob *UpdateJob
	jobHistory []*UpdateJob
	maxHistory int
	clearTimer *time.Timer
	clearDelay time.Duration
}

// NewUpdateQueue creates a new update queue
func NewUpdateQueue() *UpdateQueue {
	return &UpdateQueue{
		maxHistory: 10,
		jobHistory: make([]*UpdateJob, 0, 10),
		clearDelay: 30 * time.Second,
	}
}

// Enqueue adds a new update job to the queue
// Returns the job ID and a boolean indicating if the job was accepted
func (q *UpdateQueue) Enqueue(downloadURL string) (*UpdateJob, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if there's already a job running
	if q.currentJob != nil && (q.currentJob.State == JobStateQueued || q.currentJob.State == JobStateRunning) {
		log.Warn().
			Str("component", updatesQueueComponent).
			Str("action", "enqueue_rejected").
			Str("current_job_id", q.currentJob.ID).
			Str("current_state", string(q.currentJob.State)).
			Str("requested_download_url", downloadURL).
			Msg("Update job rejected: another job is already running")
		return nil, false
	}

	// A new job supersedes any pending clear of a previous completed job.
	q.stopClearTimerLocked()

	// Create new job
	ctx, cancel := context.WithCancel(context.Background())
	job := &UpdateJob{
		ID:          generateJobID(),
		DownloadURL: downloadURL,
		State:       JobStateQueued,
		StartedAt:   time.Now(),
		Context:     ctx,
		Cancel:      cancel,
	}

	q.currentJob = job
	log.Info().
		Str("component", updatesQueueComponent).
		Str("action", "enqueue").
		Str("job_id", job.ID).
		Str("job_state", string(job.State)).
		Str("download_url", downloadURL).
		Msg("Update job enqueued")

	return job, true
}

// MarkRunning marks the current job as running
func (q *UpdateQueue) MarkRunning(jobID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.currentJob == nil || q.currentJob.ID != jobID {
		return false
	}

	previousState := q.currentJob.State
	q.currentJob.State = JobStateRunning
	log.Info().
		Str("component", updatesQueueComponent).
		Str("action", "mark_running").
		Str("job_id", jobID).
		Str("previous_state", string(previousState)).
		Str("job_state", string(q.currentJob.State)).
		Msg("Update job started")
	return true
}

// MarkCompleted marks the current job as completed
func (q *UpdateQueue) MarkCompleted(jobID string, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.currentJob == nil || q.currentJob.ID != jobID {
		return
	}

	// Ensure resources tied to the job context are released.
	q.currentJob.Cancel()

	q.currentJob.CompletedAt = time.Now()
	if err != nil {
		q.currentJob.State = JobStateFailed
		q.currentJob.Error = err
		log.Error().
			Str("component", updatesQueueComponent).
			Str("action", "complete_failed").
			Err(err).
			Str("job_id", jobID).
			Str("job_state", string(q.currentJob.State)).
			Dur("duration", q.currentJob.CompletedAt.Sub(q.currentJob.StartedAt)).
			Msg("Update job failed")
	} else {
		q.currentJob.State = JobStateCompleted
		log.Info().
			Str("component", updatesQueueComponent).
			Str("action", "complete_succeeded").
			Str("job_id", jobID).
			Str("job_state", string(q.currentJob.State)).
			Dur("duration", q.currentJob.CompletedAt.Sub(q.currentJob.StartedAt)).
			Msg("Update job completed")
	}

	// Add to history
	q.addToHistory(q.currentJob)

	// Clear current job after a short delay (allow status polling to see completion).
	q.scheduleClearCurrentJobLocked(jobID)
}

// Cancel cancels the current job if it matches the given ID
func (q *UpdateQueue) Cancel(jobID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.currentJob == nil || q.currentJob.ID != jobID {
		return false
	}

	if q.currentJob.State == JobStateQueued || q.currentJob.State == JobStateRunning {
		previousState := q.currentJob.State
		q.currentJob.Cancel()
		q.currentJob.State = JobStateCancelled
		q.currentJob.CompletedAt = time.Now()
		log.Info().
			Str("component", updatesQueueComponent).
			Str("action", "cancel").
			Str("job_id", jobID).
			Str("previous_state", string(previousState)).
			Str("job_state", string(q.currentJob.State)).
			Msg("Update job cancelled")
		return true
	}

	return false
}

// GetCurrentJob returns the current job if any
func (q *UpdateQueue) GetCurrentJob() *UpdateJob {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return cloneUpdateJob(q.currentJob)
}

// IsRunning returns true if there's a job currently running or queued
func (q *UpdateQueue) IsRunning() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.currentJob != nil && (q.currentJob.State == JobStateQueued || q.currentJob.State == JobStateRunning)
}

// GetHistory returns the job history
func (q *UpdateQueue) GetHistory() []*UpdateJob {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Return deep copies to avoid exposing mutable internal state.
	history := make([]*UpdateJob, 0, len(q.jobHistory))
	for _, job := range q.jobHistory {
		history = append(history, cloneUpdateJob(job))
	}
	return history
}

// addToHistory adds a job to the history (must be called with lock held)
func (q *UpdateQueue) addToHistory(job *UpdateJob) {
	q.jobHistory = append(q.jobHistory, job)

	// Keep only the last N jobs
	if len(q.jobHistory) > q.maxHistory {
		q.jobHistory = q.jobHistory[len(q.jobHistory)-q.maxHistory:]
	}
}

// stopClearTimerLocked stops and clears the delayed current-job cleanup timer.
// Caller must hold q.mu.
func (q *UpdateQueue) stopClearTimerLocked() {
	if q.clearTimer == nil {
		return
	}
	q.clearTimer.Stop()
	q.clearTimer = nil
}

// scheduleClearCurrentJobLocked schedules delayed cleanup for the provided job ID.
// Caller must hold q.mu.
func (q *UpdateQueue) scheduleClearCurrentJobLocked(jobID string) {
	q.stopClearTimerLocked()

	delay := q.clearDelay
	if delay <= 0 {
		delay = 30 * time.Second
	}

	var timer *time.Timer
	timer = time.AfterFunc(delay, func() {
		q.mu.Lock()
		defer q.mu.Unlock()

		// Only clear the job when the timer still corresponds to the active cleanup schedule.
		if q.clearTimer == timer {
			q.clearTimer = nil
		}
		if q.currentJob != nil && q.currentJob.ID == jobID {
			q.currentJob = nil
		}
	})
	q.clearTimer = timer
}

// generateJobID generates a unique job ID
func generateJobID() string {
	return time.Now().Format("20060102-150405.000")
}

func cloneUpdateJob(job *UpdateJob) *UpdateJob {
	if job == nil {
		return nil
	}
	cloned := *job
	return &cloned
}
