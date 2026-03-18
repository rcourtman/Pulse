package deploy

import (
	"testing"
)

func TestJobTransitionTo_Valid(t *testing.T) {
	tests := []struct {
		from JobStatus
		to   JobStatus
	}{
		{JobQueued, JobWaitingSource},
		{JobQueued, JobRunning},
		{JobQueued, JobFailed},
		{JobQueued, JobCanceled},
		{JobWaitingSource, JobRunning},
		{JobWaitingSource, JobFailed},
		{JobWaitingSource, JobCanceled},
		{JobRunning, JobSucceeded},
		{JobRunning, JobPartialSuccess},
		{JobRunning, JobFailed},
		{JobRunning, JobCanceling},
		{JobCanceling, JobCanceled},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			j := &Job{ID: "j1", Status: tt.from}
			if err := j.TransitionTo(tt.to); err != nil {
				t.Fatalf("expected valid transition %s → %s, got error: %v", tt.from, tt.to, err)
			}
			if j.Status != tt.to {
				t.Fatalf("expected status %s, got %s", tt.to, j.Status)
			}
			if j.UpdatedAt.IsZero() {
				t.Fatal("expected updatedAt to be set")
			}
		})
	}
}

func TestJobTransitionTo_Terminal_SetsCompletedAt(t *testing.T) {
	for _, status := range []JobStatus{JobSucceeded, JobPartialSuccess, JobFailed, JobCanceled} {
		t.Run(string(status), func(t *testing.T) {
			from := JobRunning
			if status == JobCanceled {
				from = JobCanceling
			}
			j := &Job{ID: "j1", Status: from}
			if err := j.TransitionTo(status); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if j.CompletedAt == nil {
				t.Fatal("expected completedAt to be set for terminal state")
			}
		})
	}
}

func TestJobTransitionTo_Invalid(t *testing.T) {
	tests := []struct {
		from JobStatus
		to   JobStatus
	}{
		{JobSucceeded, JobRunning},       // terminal → anything
		{JobFailed, JobRunning},          // terminal → anything
		{JobCanceled, JobRunning},        // terminal → anything
		{JobPartialSuccess, JobRunning},  // terminal → anything
		{JobQueued, JobSucceeded},        // skip running
		{JobQueued, JobPartialSuccess},   // skip running
		{JobRunning, JobQueued},          // backwards
		{JobCanceling, JobRunning},       // only → canceled
		{JobWaitingSource, JobSucceeded}, // skip running
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			j := &Job{ID: "j1", Status: tt.from}
			if err := j.TransitionTo(tt.to); err == nil {
				t.Fatalf("expected error for invalid transition %s → %s", tt.from, tt.to)
			}
		})
	}
}

func TestTargetTransitionTo_Valid(t *testing.T) {
	tests := []struct {
		from TargetStatus
		to   TargetStatus
	}{
		{TargetPending, TargetPreflighting},
		{TargetPending, TargetSkippedAgent},
		{TargetPending, TargetSkippedLicense},
		{TargetPending, TargetCanceled},
		{TargetPreflighting, TargetReady},
		{TargetPreflighting, TargetFailedRetryable},
		{TargetPreflighting, TargetFailedPermanent},
		{TargetPreflighting, TargetCanceled},
		{TargetReady, TargetInstalling},
		{TargetReady, TargetCanceled},
		{TargetInstalling, TargetEnrolling},
		{TargetInstalling, TargetFailedRetryable},
		{TargetInstalling, TargetFailedPermanent},
		{TargetInstalling, TargetCanceled},
		{TargetEnrolling, TargetVerifying},
		{TargetEnrolling, TargetFailedRetryable},
		{TargetEnrolling, TargetFailedPermanent},
		{TargetEnrolling, TargetCanceled},
		{TargetVerifying, TargetSucceeded},
		{TargetVerifying, TargetFailedRetryable},
		{TargetVerifying, TargetCanceled},
		{TargetFailedRetryable, TargetPending},
		{TargetFailedRetryable, TargetFailedPermanent},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			tgt := &Target{ID: "t1", Status: tt.from}
			if err := tgt.TransitionTo(tt.to); err != nil {
				t.Fatalf("expected valid transition %s → %s, got error: %v", tt.from, tt.to, err)
			}
			if tgt.Status != tt.to {
				t.Fatalf("expected status %s, got %s", tt.to, tgt.Status)
			}
		})
	}
}

func TestTargetTransitionTo_Invalid(t *testing.T) {
	tests := []struct {
		from TargetStatus
		to   TargetStatus
	}{
		{TargetSucceeded, TargetPending},         // terminal
		{TargetFailedPermanent, TargetPending},   // terminal
		{TargetSkippedAgent, TargetPending},      // terminal
		{TargetSkippedLicense, TargetPending},    // terminal
		{TargetCanceled, TargetPending},          // terminal
		{TargetPending, TargetInstalling},        // skip preflighting
		{TargetPending, TargetSucceeded},         // skip to end
		{TargetReady, TargetEnrolling},           // skip installing
		{TargetVerifying, TargetFailedPermanent}, // not allowed from verifying
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			tgt := &Target{ID: "t1", Status: tt.from}
			if err := tgt.TransitionTo(tt.to); err == nil {
				t.Fatalf("expected error for invalid transition %s → %s", tt.from, tt.to)
			}
		})
	}
}

func TestDeriveStatus(t *testing.T) {
	tests := []struct {
		name     string
		targets  []Target
		expected JobStatus
	}{
		{
			name:     "all succeeded",
			targets:  targets(TargetSucceeded, TargetSucceeded, TargetSucceeded),
			expected: JobSucceeded,
		},
		{
			name:     "all failed",
			targets:  targets(TargetFailedPermanent, TargetFailedPermanent),
			expected: JobFailed,
		},
		{
			name:     "mix of succeeded and failed",
			targets:  targets(TargetSucceeded, TargetFailedPermanent),
			expected: JobPartialSuccess,
		},
		{
			name:     "some still active",
			targets:  targets(TargetSucceeded, TargetInstalling),
			expected: JobRunning,
		},
		{
			name:     "skipped counts as terminal non-success",
			targets:  targets(TargetSucceeded, TargetSkippedAgent),
			expected: JobPartialSuccess,
		},
		{
			name:     "all skipped",
			targets:  targets(TargetSkippedAgent, TargetSkippedLicense),
			expected: JobFailed,
		},
		{
			name:     "retryable counts as active",
			targets:  targets(TargetSucceeded, TargetFailedRetryable),
			expected: JobRunning,
		},
		{
			name:     "empty targets preserves current status",
			targets:  nil,
			expected: JobRunning,
		},
		{
			name:     "canceled targets",
			targets:  targets(TargetCanceled, TargetCanceled),
			expected: JobFailed,
		},
		{
			name:     "succeeded and canceled",
			targets:  targets(TargetSucceeded, TargetCanceled),
			expected: JobPartialSuccess,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &Job{Status: JobRunning}
			got := j.DeriveStatus(tt.targets)
			if got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func targets(statuses ...TargetStatus) []Target {
	out := make([]Target, len(statuses))
	for i, s := range statuses {
		out[i] = Target{Status: s}
	}
	return out
}
