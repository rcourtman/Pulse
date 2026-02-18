package model

import "time"

// Provider identifies the system that produced a recovery point.
// This is intentionally a string (not an enum) to keep forward-compatibility with new platforms.
type Provider string

const (
	ProviderProxmoxPVE Provider = "proxmox-pve"
	ProviderProxmoxPBS Provider = "proxmox-pbs"
	ProviderProxmoxPMG Provider = "proxmox-pmg"
	ProviderKubernetes Provider = "kubernetes"
	ProviderTrueNAS    Provider = "truenas"
	ProviderDocker     Provider = "docker"
	ProviderHostAgent  Provider = "host-agent"
)

type Kind string

const (
	KindSnapshot Kind = "snapshot"
	KindBackup   Kind = "backup"
	KindOther    Kind = "other"
)

type Mode string

const (
	ModeSnapshot Mode = "snapshot"
	ModeLocal    Mode = "local"
	ModeRemote   Mode = "remote"
)

type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeWarning Outcome = "warning"
	OutcomeFailed  Outcome = "failed"
	OutcomeRunning Outcome = "running"
	OutcomeUnknown Outcome = "unknown"
)

// ExternalRef is used when a recovery point cannot be linked to a unified resource.
// Example: a Kubernetes PVC or a TrueNAS dataset.
type ExternalRef struct {
	Type      string            `json:"type"`
	Namespace string            `json:"namespace,omitempty"`
	Name      string            `json:"name,omitempty"`
	UID       string            `json:"uid,omitempty"`
	ID        string            `json:"id,omitempty"`
	Class     string            `json:"class,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

// RecoveryPoint represents a single restore point (snapshot or backup artifact).
//
// IMPORTANT: Recovery points are NOT modeled as unifiedresources.Resource. They are high-cardinality
// time-based artifacts and are persisted separately with retention.
type RecoveryPoint struct {
	ID       string   `json:"id"`
	Provider Provider `json:"provider"`
	Kind     Kind     `json:"kind"`
	Mode     Mode     `json:"mode"`
	Outcome  Outcome  `json:"outcome"`

	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	SizeBytes *int64 `json:"sizeBytes,omitempty"`
	Verified  *bool  `json:"verified,omitempty"`
	Encrypted *bool  `json:"encrypted,omitempty"`
	Immutable *bool  `json:"immutable,omitempty"`

	// Link to a unified resource when possible.
	SubjectResourceID    string       `json:"subjectResourceId,omitempty"`
	RepositoryResourceID string       `json:"repositoryResourceId,omitempty"`
	SubjectRef           *ExternalRef `json:"subjectRef,omitempty"`
	RepositoryRef        *ExternalRef `json:"repositoryRef,omitempty"`

	// Provider-specific details for drill-down (kept small).
	Details map[string]any `json:"details,omitempty"`

	// Display contains normalized, provider-agnostic fields intended for UIs.
	// These fields are derived at ingest time and/or backfilled in the store.
	Display *RecoveryPointDisplay `json:"display,omitempty"`
}

// RecoveryPointDisplay is a normalized, UI-friendly view of a recovery point.
// These fields are safe to depend on across platforms and avoid UIs needing provider-specific heuristics.
type RecoveryPointDisplay struct {
	SubjectLabel    string `json:"subjectLabel,omitempty"`
	SubjectType     string `json:"subjectType,omitempty"`
	IsWorkload      bool   `json:"isWorkload,omitempty"`
	ClusterLabel    string `json:"clusterLabel,omitempty"`
	NodeHostLabel   string `json:"nodeHostLabel,omitempty"`
	NamespaceLabel  string `json:"namespaceLabel,omitempty"`
	EntityIDLabel   string `json:"entityIdLabel,omitempty"`
	RepositoryLabel string `json:"repositoryLabel,omitempty"`
	DetailsSummary  string `json:"detailsSummary,omitempty"`
}

// ProtectionRollup is a per-resource summary derived from recovery points.
type ProtectionRollup struct {
	// RollupID is a stable identifier used for drill-down into points.
	// It corresponds to the internal subject key:
	// - linked resources: "res:<resource-id>"
	// - external refs: "ext:<hash>"
	RollupID string `json:"rollupId"`

	// Latest known subject identity for display in UIs.
	SubjectResourceID string       `json:"subjectResourceId,omitempty"`
	SubjectRef        *ExternalRef `json:"subjectRef,omitempty"`

	LastAttemptAt *time.Time `json:"lastAttemptAt,omitempty"`
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`
	LastOutcome   Outcome    `json:"lastOutcome"`

	// Providers that contributed points within the query window.
	Providers []Provider `json:"providers,omitempty"`
}

type ListPointsOptions struct {
	Provider Provider
	Kind     Kind
	Mode     Mode
	Outcome  Outcome

	SubjectResourceID string
	// RollupID filters by the stable subject key ("res:<id>" or "ext:<hash>").
	// When set, it takes precedence over SubjectResourceID.
	RollupID string

	From *time.Time
	To   *time.Time

	// Query is a best-effort free-text filter over normalized fields.
	Query string

	// Normalized filters derived from the recovery point index.
	ClusterLabel   string
	NodeHostLabel  string
	NamespaceLabel string

	// WorkloadOnly filters points to those whose subject is considered a workload.
	WorkloadOnly bool

	// Verification filters by tri-state verification fields ("verified", "unverified", "unknown").
	Verification string

	Page  int
	Limit int
}

// PointsSeriesBucket is a per-day aggregation of recovery points within a time window.
// Day is formatted as YYYY-MM-DD in the requested (client) timezone.
type PointsSeriesBucket struct {
	Day      string `json:"day"`
	Total    int    `json:"total"`
	Snapshot int    `json:"snapshot"`
	Local    int    `json:"local"`
	Remote   int    `json:"remote"`
}

// PointsFacets provides distinct filter values for recovery points within a time window.
type PointsFacets struct {
	Clusters   []string `json:"clusters,omitempty"`
	NodesHosts []string `json:"nodesHosts,omitempty"`
	Namespaces []string `json:"namespaces,omitempty"`

	HasSize         bool `json:"hasSize,omitempty"`
	HasVerification bool `json:"hasVerification,omitempty"`
	HasEntityID     bool `json:"hasEntityId,omitempty"`
}
