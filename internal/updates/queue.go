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
}

// NewUpdateQueue creates a new update queue
func NewUpdateQueue() *UpdateQueue {
	return &UpdateQueue{
		maxHistory: 10,
		jobHistory: make([]*UpdateJob, 0, 10),
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

	// Clear current job after a short delay (allow status polling to see completion)
	go func() {
		time.Sleep(30 * time.Second)
		q.mu.Lock()
		if q.currentJob != nil && q.currentJob.ID == jobID {
			q.currentJob = nil
		}
		q.mu.Unlock()
	}()
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
	return q.currentJob
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

	// Return a copy to avoid concurrent access issues
	history := make([]*UpdateJob, len(q.jobHistory))
	copy(history, q.jobHistory)
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

// generateJobID generates a unique job ID
func generateJobID() string {
	return time.Now().Format("20060102-150405.000")
}
