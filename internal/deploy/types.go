package deploy

import "time"

// JobStatus represents the lifecycle state of a deployment job.
type JobStatus string

const (
	JobQueued         JobStatus = "queued"
	JobWaitingSource  JobStatus = "waiting_source"
	JobRunning        JobStatus = "running"
	JobSucceeded      JobStatus = "succeeded"
	JobPartialSuccess JobStatus = "partial_success"
	JobFailed         JobStatus = "failed"
	JobCanceling      JobStatus = "canceling"
	JobCanceled       JobStatus = "canceled"
)

// TargetStatus represents the lifecycle state of a single deployment target.
type TargetStatus string

const (
	TargetPending         TargetStatus = "pending"
	TargetPreflighting    TargetStatus = "preflighting"
	TargetReady           TargetStatus = "ready"
	TargetInstalling      TargetStatus = "installing"
	TargetEnrolling       TargetStatus = "enrolling"
	TargetVerifying       TargetStatus = "verifying"
	TargetSucceeded       TargetStatus = "succeeded"
	TargetFailedRetryable TargetStatus = "failed_retryable"
	TargetFailedPermanent TargetStatus = "failed_permanent"
	TargetSkippedAgent    TargetStatus = "skipped_already_agent"
	TargetSkippedLicense  TargetStatus = "skipped_license"
	TargetCanceled        TargetStatus = "canceled"
)

// EventType classifies deployment audit log entries.
type EventType string

const (
	EventJobCreated          EventType = "job_created"
	EventJobStatusChanged    EventType = "job_status_changed"
	EventTargetStatusChanged EventType = "target_status_changed"
	EventPreflightResult     EventType = "preflight_result"
	EventInstallOutput       EventType = "install_output"
	EventEnrollComplete      EventType = "enroll_complete"
	EventError               EventType = "error"
)

// Job represents a cluster agent deployment job.
type Job struct {
	ID            string     `json:"id"`
	ClusterID     string     `json:"clusterId"`
	ClusterName   string     `json:"clusterName"`
	SourceAgentID string     `json:"sourceAgentId"`
	SourceNodeID  string     `json:"sourceNodeId"`
	OrgID         string     `json:"orgId"`
	Status        JobStatus  `json:"status"`
	MaxParallel   int        `json:"maxParallel"`
	RetryMax      int        `json:"retryMax"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}

// Target represents a single node targeted for agent deployment.
type Target struct {
	ID           string       `json:"id"`
	JobID        string       `json:"jobId"`
	NodeID       string       `json:"nodeId"`
	NodeName     string       `json:"nodeName"`
	NodeIP       string       `json:"nodeIP"`
	Arch         string       `json:"arch,omitempty"`
	Status       TargetStatus `json:"status"`
	ErrorMessage string       `json:"errorMessage,omitempty"`
	Attempts     int          `json:"attempts"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
}

// Event is an immutable audit log entry for a deployment lifecycle.
type Event struct {
	ID        string    `json:"id"`
	JobID     string    `json:"jobId"`
	TargetID  string    `json:"targetId,omitempty"`
	Type      EventType `json:"type"`
	Message   string    `json:"message"`
	Data      string    `json:"data,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
