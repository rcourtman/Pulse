package declarativeconfig

import (
	"errors"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	APIVersion         = "pulse.monitoring/v1"
	Kind               = "MonitoringConfig"
	SecretRefPreserved = "preserved"
	SecretRefEnvPrefix = "env:"
)

var (
	ErrInvalidDocument     = errors.New("invalid declarative configuration document")
	ErrSecretMissing       = errors.New("preserved secret has no existing saved value")
	ErrUnresolvedSecretRef = errors.New("unresolved secret reference")
)

// SecretField represents a write-only credential or secret-bearing endpoint
// value inside a declarative configuration document. Exported documents always
// emit secretRef "preserved" with a non-secret hint and never emit plaintext.
type SecretField struct {
	SecretRef    string `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
	Configured   bool   `json:"configured,omitempty" yaml:"configured,omitempty"`
	RedactedHint string `json:"redactedHint,omitempty" yaml:"redactedHint,omitempty"`
	Value        string `json:"value,omitempty" yaml:"value,omitempty"`
}

// Document is the top-level declarative monitoring configuration envelope.
// Omitting a top-level section leaves that domain untouched during planning
// and apply. Providing an explicit empty list clears items when prune is true.
type Document struct {
	APIVersion          string                    `json:"apiVersion" yaml:"apiVersion"`
	Kind                string                    `json:"kind" yaml:"kind"`
	Alerts              *AlertsSection            `json:"alerts,omitempty" yaml:"alerts,omitempty"`
	AvailabilityTargets *[]AvailabilityTargetSpec `json:"availabilityTargets,omitempty" yaml:"availabilityTargets,omitempty"`
	OperatorStates      *[]OperatorStateSpec      `json:"operatorStates,omitempty" yaml:"operatorStates,omitempty"`
	Notifications       *NotificationsSection     `json:"notifications,omitempty" yaml:"notifications,omitempty"`
}

// AlertsSection captures alert thresholds, rules, and intent policies.
type AlertsSection struct {
	Config         *alerts.AlertConfig               `json:"config,omitempty" yaml:"config,omitempty"`
	IntentPolicies *alerts.AlertIntentPolicyDocument `json:"intentPolicies,omitempty" yaml:"intentPolicies,omitempty"`
}

// AvailabilityTargetSpec represents one declarative availability check.
// Volatile configRevision and legacy probeAgentId are omitted in favor of
// canonical observationLocationIds.
type AvailabilityTargetSpec struct {
	ID                            string                               `json:"id" yaml:"id"`
	Name                          string                               `json:"name" yaml:"name"`
	TargetKind                    config.AvailabilityTargetKind        `json:"targetKind,omitempty" yaml:"targetKind,omitempty"`
	Address                       string                               `json:"address" yaml:"address"`
	Protocol                      config.AvailabilityProbeProtocol     `json:"protocol" yaml:"protocol"`
	Port                          int                                  `json:"port,omitempty" yaml:"port,omitempty"`
	Path                          string                               `json:"path,omitempty" yaml:"path,omitempty"`
	UDPMode                       config.AvailabilityUDPMode           `json:"udpMode,omitempty" yaml:"udpMode,omitempty"`
	UDPRequest                    string                               `json:"udpRequest,omitempty" yaml:"udpRequest,omitempty"`
	UDPExpected                   string                               `json:"udpExpectedResponse,omitempty" yaml:"udpExpectedResponse,omitempty"`
	Enabled                       bool                                 `json:"enabled" yaml:"enabled"`
	PollIntervalSecs              int                                  `json:"pollIntervalSeconds,omitempty" yaml:"pollIntervalSeconds,omitempty"`
	TimeoutMillis                 int                                  `json:"timeoutMillis,omitempty" yaml:"timeoutMillis,omitempty"`
	FailureThreshold              int                                  `json:"failureThreshold,omitempty" yaml:"failureThreshold,omitempty"`
	LinkedResourceID              string                               `json:"linkedResourceId,omitempty" yaml:"linkedResourceId,omitempty"`
	CertificateMonitoringDisabled bool                                 `json:"certificateMonitoringDisabled,omitempty" yaml:"certificateMonitoringDisabled,omitempty"`
	CertificateExpiryWarningDays  int                                  `json:"certificateExpiryWarningDays,omitempty" yaml:"certificateExpiryWarningDays,omitempty"`
	ObservationLocationIDs        []string                             `json:"observationLocationIds,omitempty" yaml:"observationLocationIds,omitempty"`
	HTTP                          *AvailabilityHTTPSpec                `json:"http,omitempty" yaml:"http,omitempty"`
	DockerPort                    *config.AvailabilityDockerPortSource `json:"dockerPort,omitempty" yaml:"dockerPort,omitempty"`
}

// AvailabilityHTTPSpec defines an HTTP probe contract with redacted secrets.
type AvailabilityHTTPSpec struct {
	Method            config.AvailabilityHTTPMethod `json:"method" yaml:"method"`
	Headers           []AvailabilityHTTPHeaderSpec  `json:"headers,omitempty" yaml:"headers,omitempty"`
	Authentication    AvailabilityHTTPAuthSpec      `json:"authentication" yaml:"authentication"`
	Body              *SecretField                  `json:"body,omitempty" yaml:"body,omitempty"`
	ExpectedStatusMin int                           `json:"expectedStatusMin" yaml:"expectedStatusMin"`
	ExpectedStatusMax int                           `json:"expectedStatusMax" yaml:"expectedStatusMax"`
	TextContains      string                        `json:"textContains,omitempty" yaml:"textContains,omitempty"`
	JSONPath          string                        `json:"jsonPath,omitempty" yaml:"jsonPath,omitempty"`
	JSONEquals        string                        `json:"jsonEquals,omitempty" yaml:"jsonEquals,omitempty"`
}

// AvailabilityHTTPHeaderSpec represents one HTTP probe request header whose
// value is treated as write-only secret material.
type AvailabilityHTTPHeaderSpec struct {
	ID    string       `json:"id" yaml:"id"`
	Name  string       `json:"name" yaml:"name"`
	Value *SecretField `json:"value,omitempty" yaml:"value,omitempty"`
}

// AvailabilityHTTPAuthSpec holds HTTP probe authentication settings.
type AvailabilityHTTPAuthSpec struct {
	Type        config.AvailabilityHTTPAuthType `json:"type" yaml:"type"`
	Username    string                          `json:"username,omitempty" yaml:"username,omitempty"`
	Password    *SecretField                    `json:"password,omitempty" yaml:"password,omitempty"`
	BearerToken *SecretField                    `json:"bearerToken,omitempty" yaml:"bearerToken,omitempty"`
}

// OperatorStateSpec represents declarative per-resource operator posture
// without volatile audit metadata such as setAt or setBy.
type OperatorStateSpec struct {
	CanonicalID           string                              `json:"canonicalId" yaml:"canonicalId"`
	MonitoringMode        unified.ResourceMonitoringMode      `json:"monitoringMode,omitempty" yaml:"monitoringMode,omitempty"`
	LifecycleState        unified.ResourceLifecycleState      `json:"lifecycleState,omitempty" yaml:"lifecycleState,omitempty"`
	NeverAutoRemediate    bool                                `json:"neverAutoRemediate,omitempty" yaml:"neverAutoRemediate,omitempty"`
	AutoRemediationPolicy unified.AutoRemediationPolicy       `json:"autoRemediationPolicy,omitempty" yaml:"autoRemediationPolicy,omitempty"`
	MaintenanceStartAt    *time.Time                          `json:"maintenanceStartAt,omitempty" yaml:"maintenanceStartAt,omitempty"`
	MaintenanceEndAt      *time.Time                          `json:"maintenanceEndAt,omitempty" yaml:"maintenanceEndAt,omitempty"`
	MaintenanceRecurrence *unified.RecurringMaintenanceWindow `json:"maintenanceRecurrence,omitempty" yaml:"maintenanceRecurrence,omitempty"`
	MaintenanceScope      unified.MaintenanceScope            `json:"maintenanceScope,omitempty" yaml:"maintenanceScope,omitempty"`
	MaintenanceReason     string                              `json:"maintenanceReason,omitempty" yaml:"maintenanceReason,omitempty"`
	Criticality           unified.ResourceCriticality         `json:"criticality,omitempty" yaml:"criticality,omitempty"`
	Note                  string                              `json:"note,omitempty" yaml:"note,omitempty"`
}

// NotificationsSection groups outbound notification destinations.
type NotificationsSection struct {
	Email    *EmailSpec     `json:"email,omitempty" yaml:"email,omitempty"`
	Webhooks *[]WebhookSpec `json:"webhooks,omitempty" yaml:"webhooks,omitempty"`
	Apprise  *AppriseSpec   `json:"apprise,omitempty" yaml:"apprise,omitempty"`
	DeadMan  *DeadManSpec   `json:"deadMan,omitempty" yaml:"deadMan,omitempty"`
}

// EmailSpec holds SMTP notification configuration with a redacted password.
type EmailSpec struct {
	Enabled         bool         `json:"enabled" yaml:"enabled"`
	Provider        string       `json:"provider,omitempty" yaml:"provider,omitempty"`
	SMTPHost        string       `json:"server,omitempty" yaml:"server,omitempty"`
	SMTPPort        int          `json:"port,omitempty" yaml:"port,omitempty"`
	Username        string       `json:"username,omitempty" yaml:"username,omitempty"`
	Password        *SecretField `json:"password,omitempty" yaml:"password,omitempty"`
	From            string       `json:"from,omitempty" yaml:"from,omitempty"`
	To              []string     `json:"to,omitempty" yaml:"to,omitempty"`
	TLS             bool         `json:"tls,omitempty" yaml:"tls,omitempty"`
	StartTLS        bool         `json:"startTLS,omitempty" yaml:"startTLS,omitempty"`
	RateLimit       int          `json:"rateLimit,omitempty" yaml:"rateLimit,omitempty"`
	TagFilter       []string     `json:"tagFilter,omitempty" yaml:"tagFilter,omitempty"`
	TagMode         string       `json:"tagFilterMode,omitempty" yaml:"tagFilterMode,omitempty"`
	MinimumSeverity string       `json:"minimumSeverity,omitempty" yaml:"minimumSeverity,omitempty"`
	SubjectPrefix   string       `json:"subjectPrefix,omitempty" yaml:"subjectPrefix,omitempty"`
	BrandName       string       `json:"brandName,omitempty" yaml:"brandName,omitempty"`
	FooterNote      string       `json:"footerNote,omitempty" yaml:"footerNote,omitempty"`
}

// WebhookSpec holds webhook destination settings with redacted URL, headers,
// custom fields, and signing secret.
type WebhookSpec struct {
	ID              string                 `json:"id" yaml:"id"`
	Name            string                 `json:"name" yaml:"name"`
	URL             *SecretField           `json:"url,omitempty" yaml:"url,omitempty"`
	Method          string                 `json:"method,omitempty" yaml:"method,omitempty"`
	Headers         map[string]SecretField `json:"headers,omitempty" yaml:"headers,omitempty"`
	Enabled         bool                   `json:"enabled" yaml:"enabled"`
	Service         string                 `json:"service,omitempty" yaml:"service,omitempty"`
	Template        string                 `json:"template,omitempty" yaml:"template,omitempty"`
	CustomFields    map[string]SecretField `json:"customFields,omitempty" yaml:"customFields,omitempty"`
	Mention         string                 `json:"mention,omitempty" yaml:"mention,omitempty"`
	TagFilter       []string               `json:"tagFilter,omitempty" yaml:"tagFilter,omitempty"`
	TagMode         string                 `json:"tagFilterMode,omitempty" yaml:"tagFilterMode,omitempty"`
	MinimumSeverity string                 `json:"minimumSeverity,omitempty" yaml:"minimumSeverity,omitempty"`
	SigningSecret   *SecretField           `json:"signingSecret,omitempty" yaml:"signingSecret,omitempty"`
}

// AppriseSpec holds Apprise notification settings with redacted keys and targets.
type AppriseSpec struct {
	Enabled         bool                      `json:"enabled" yaml:"enabled"`
	Mode            notifications.AppriseMode `json:"mode,omitempty" yaml:"mode,omitempty"`
	Targets         []SecretField             `json:"targets,omitempty" yaml:"targets,omitempty"`
	CLIPath         string                    `json:"cliPath,omitempty" yaml:"cliPath,omitempty"`
	TimeoutSeconds  int                       `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
	ServerURL       string                    `json:"serverUrl,omitempty" yaml:"serverUrl,omitempty"`
	ConfigKey       *SecretField              `json:"configKey,omitempty" yaml:"configKey,omitempty"`
	APIKey          *SecretField              `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	APIKeyHeader    string                    `json:"apiKeyHeader,omitempty" yaml:"apiKeyHeader,omitempty"`
	SkipTLSVerify   bool                      `json:"skipTlsVerify,omitempty" yaml:"skipTlsVerify,omitempty"`
	MinimumSeverity string                    `json:"minimumSeverity,omitempty" yaml:"minimumSeverity,omitempty"`
}

// DeadManSpec holds the external dead-man watchdog endpoint configuration.
type DeadManSpec struct {
	PingURL *SecretField `json:"pingUrl,omitempty" yaml:"pingUrl,omitempty"`
}

// Snapshot holds the live server configuration state across the four
// declarative monitoring domains.
type Snapshot struct {
	AlertConfig         *alerts.AlertConfig
	AlertIntentPolicies *alerts.AlertIntentPolicyDocument
	AvailabilityTargets []config.AvailabilityTarget
	OperatorStates      []unified.ResourceOperatorState
	Email               *notifications.EmailConfig
	Webhooks            []notifications.WebhookConfig
	Apprise             *notifications.AppriseConfig
	DeadMan             *notifications.DeadManConfig
}

// PlanOptions controls how omitted items in included list sections are handled.
type PlanOptions struct {
	Prune bool `json:"prune" yaml:"prune"`
}

// ConfigChange describes one deterministic create, update, or delete action.
// Summaries and field lists never contain plaintext secret values.
type ConfigChange struct {
	Domain  string   `json:"domain" yaml:"domain"`
	Action  string   `json:"action" yaml:"action"`
	Key     string   `json:"key" yaml:"key"`
	Summary string   `json:"summary" yaml:"summary"`
	Fields  []string `json:"fields,omitempty" yaml:"fields,omitempty"`
}

// PlanSummary aggregates counts across all planned changes.
type PlanSummary struct {
	Creates    int  `json:"creates" yaml:"creates"`
	Updates    int  `json:"updates" yaml:"updates"`
	Deletes    int  `json:"deletes" yaml:"deletes"`
	Unchanged  int  `json:"unchanged" yaml:"unchanged"`
	HasChanges bool `json:"hasChanges" yaml:"hasChanges"`
}

// PlanResult is the public diff output returned by planning and apply endpoints.
type PlanResult struct {
	APIVersion string         `json:"apiVersion" yaml:"apiVersion"`
	Kind       string         `json:"kind" yaml:"kind"`
	PlanHash   string         `json:"planHash" yaml:"planHash"`
	Prune      bool           `json:"prune" yaml:"prune"`
	Summary    PlanSummary    `json:"summary" yaml:"summary"`
	Changes    []ConfigChange `json:"changes" yaml:"changes"`
}

// ResolvedState carries the concrete server-side state to persist and reload
// when applying a validated plan. It is never serialized directly to API responses.
type ResolvedState struct {
	AlertsChanged              bool
	AlertConfig                *alerts.AlertConfig
	AlertIntentChanged         bool
	AlertIntentPolicies        *alerts.AlertIntentPolicyDocument
	AvailabilityTargetsChanged bool
	AvailabilityTargets        []config.AvailabilityTarget
	OperatorStatesChanged      bool
	OperatorStatesToUpsert     []unified.ResourceOperatorState
	OperatorStatesToClear      []string
	EmailChanged               bool
	Email                      *notifications.EmailConfig
	WebhooksChanged            bool
	Webhooks                   []notifications.WebhookConfig
	AppriseChanged             bool
	Apprise                    *notifications.AppriseConfig
	DeadManChanged             bool
	DeadMan                    *notifications.DeadManConfig
}
