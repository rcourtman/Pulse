package deploy

import (
	"fmt"
	"time"
)

// validJobTransitions defines allowed state transitions for a Job.
var validJobTransitions = map[JobStatus][]JobStatus{
	JobQueued:        {JobWaitingSource, JobRunning, JobFailed, JobCanceled},
	JobWaitingSource: {JobRunning, JobFailed, JobCanceled},
	JobRunning:       {JobSucceeded, JobPartialSuccess, JobFailed, JobCanceling},
	JobCanceling:     {JobCanceled},
}

// validTargetTransitions defines allowed state transitions for a Target.
var validTargetTransitions = map[TargetStatus][]TargetStatus{
	TargetPending:         {TargetPreflighting, TargetSkippedAgent, TargetSkippedLicense, TargetCanceled},
	TargetPreflighting:    {TargetReady, TargetFailedRetryable, TargetFailedPermanent, TargetCanceled},
	TargetReady:           {TargetInstalling, TargetCanceled},
	TargetInstalling:      {TargetEnrolling, TargetFailedRetryable, TargetFailedPermanent, TargetCanceled},
	TargetEnrolling:       {TargetVerifying, TargetFailedRetryable, TargetFailedPermanent, TargetCanceled},
	TargetVerifying:       {TargetSucceeded, TargetFailedRetryable, TargetCanceled},
	TargetFailedRetryable: {TargetPending, TargetFailedPermanent},
}

// TransitionTo validates the transition and updates the job's status and timestamp.
func (j *Job) TransitionTo(status JobStatus) error {
	allowed, ok := validJobTransitions[j.Status]
	if !ok {
		return fmt.Errorf("job %s: no transitions from terminal state %q", j.ID, j.Status)
	}
	for _, s := range allowed {
		if s == status {
			j.Status = status
			j.UpdatedAt = time.Now().UTC()
			if isJobTerminal(status) {
				now := j.UpdatedAt
				j.CompletedAt = &now
			}
			return nil
		}
	}
	return fmt.Errorf("job %s: invalid transition %q → %q", j.ID, j.Status, status)
}

// TransitionTo validates the transition and updates the target's status and timestamp.
func (t *Target) TransitionTo(status TargetStatus) error {
	allowed, ok := validTargetTransitions[t.Status]
	if !ok {
		return fmt.Errorf("target %s: no transitions from terminal state %q", t.ID, t.Status)
	}
	for _, s := range allowed {
		if s == status {
			t.Status = status
			t.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return fmt.Errorf("target %s: invalid transition %q → %q", t.ID, t.Status, status)
}

// DeriveStatus computes the aggregate job status from its targets.
func (j *Job) DeriveStatus(targets []Target) JobStatus {
	if len(targets) == 0 {
		return j.Status
	}

	var succeeded, terminal, active int
	for i := range targets {
		switch targets[i].Status {
		case TargetSucceeded:
			succeeded++
			terminal++
		case TargetFailedPermanent, TargetSkippedAgent, TargetSkippedLicense, TargetCanceled:
			terminal++
		case TargetPending, TargetPreflighting, TargetReady, TargetInstalling,
			TargetEnrolling, TargetVerifying, TargetFailedRetryable:
			active++
		}
	}

	total := len(targets)
	if active > 0 {
		return JobRunning
	}
	// All targets are terminal.
	if succeeded == total {
		return JobSucceeded
	}
	if succeeded == 0 {
		return JobFailed
	}
	return JobPartialSuccess
}

func isJobTerminal(s JobStatus) bool {
	switch s {
	case JobSucceeded, JobPartialSuccess, JobFailed, JobCanceled:
		return true
	}
	return false
}
