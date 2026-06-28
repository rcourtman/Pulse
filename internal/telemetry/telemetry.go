// Package telemetry provides anonymous usage telemetry for Pulse.
//
// Pulse sends a lightweight ping on startup and once every 24 hours to help the
// developer understand how many active installations exist and which features are
// in use. Telemetry is enabled by default and can be opted out at any time.
//
// # What is sent (the full list — nothing else)
//
// Identity:
//   - A rotating install ID (UUID, generated locally and rotated periodically, not tied to any account)
//   - Pulse version identity (normalized version plus raw build string when it differs)
//   - Platform: "docker" or "binary"
//   - OS and architecture (e.g. "linux/amd64")
//
// Scale (counts only, no names):
//   - Number of PVE nodes, PBS instances, PMG instances
//   - Number of VMs, LXC containers
//   - Number of Pulse Agent hosts, Docker hosts/containers, and Kubernetes clusters/nodes/pods/deployments
//   - Number of storage resources, physical disks, Ceph clusters, and network shares
//   - Number of TrueNAS systems/VMs/apps, VMware hosts/VMs/datastores, and availability targets
//
// Feature usage (booleans and counts, no content):
//   - Whether AI features are enabled
//   - Whether Patrol, discovery, notifications, or AI action capability are enabled
//   - Number of active alerts
//   - Whether relay/remote access is enabled
//   - Whether SSO/OIDC is configured
//   - Whether multi-tenant mode is enabled
//   - Whether a paid license is active
//   - Whether any API tokens are configured
//   - Coarse update funnel counters and last failure category over the current install-ID rotation window
//   - Patrol, Assistant, and external-agent usage counters over the current install-ID rotation window:
//     configured/active/governed-action/approved-execution/resolved-loop state,
//     Patrol control completed-loop and resolved-loop proof reported through
//     legacy pro_activation metric keys for cohort continuity,
//     operations-loop workflow starter request counts by surface, Assistant/Patrol AI calls,
//     Patrol runs/new findings/investigations/resolved findings/autofixes,
//     external-agent readiness/usage, action plans, approval requests, rejected
//     action decisions, approved action decisions, approved action attempts,
//     and approved action successes
//
// # What is NOT sent
//
//   - No IP addresses are stored server-side
//   - No hostnames, node names, VM names, or any infrastructure identifiers
//   - No Proxmox credentials, API tokens, or passwords
//   - No alert content, AI prompts, chat messages, command text, action output, or token values
//   - No action targets, resource IDs, finding IDs, approval actors, or approval reasons
//   - No personally identifiable information of any kind
//
// # How to disable
//
// Set the environment variable PULSE_TELEMETRY=false, or toggle off
// "Anonymous outbound telemetry" in Settings → System → General.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

// pingEndpoint is the URL that receives anonymous telemetry pings.
// It is a var (not const) so that tests can redirect it to a local server.
var pingEndpoint = "https://license.pulserelay.pro/v1/telemetry/ping"

var errInstallIDUnavailable = errors.New("telemetry install id unavailable")

const (
	// heartbeatInterval is the base interval between daily pings.
	// Each cycle adds random jitter of ±maxHeartbeatJitter to prevent
	// thundering-herd effects when many installations start simultaneously.
	heartbeatInterval = 24 * time.Hour

	// maxHeartbeatJitter is the maximum random offset added to each heartbeat.
	maxHeartbeatJitter = 30 * time.Minute

	// startupDelay is how long to wait after startup before sending the first
	// ping, giving the monitor time to connect to nodes and populate state.
	startupDelay = 2 * time.Minute

	// httpTimeout is the maximum time for a single telemetry request.
	httpTimeout = 10 * time.Second

	// installIDFile is the filename persisted in the data directory.
	installIDFile = ".install_id"

	// installIDRotationWindow limits how long the same pseudonymous identifier
	// can be reused before it is rotated locally.
	installIDRotationWindow = 30 * 24 * time.Hour

	// PulseIntelligenceTelemetryWindow is the rolling, content-free usage
	// window used for Patrol, Assistant, and external-agent usage counters. It
	// intentionally matches the install-ID rotation window so counters cannot
	// be linked to one stable pseudonymous identifier indefinitely.
	PulseIntelligenceTelemetryWindow = installIDRotationWindow
)

type installIDRecord struct {
	InstallID string    `json:"install_id"`
	IssuedAt  time.Time `json:"issued_at"`
}

// Ping is the payload sent to the telemetry endpoint.
// Every field is documented here so users can audit exactly what leaves their server.
type Ping struct {
	// Identity
	InstallID          string `json:"install_id"`                   // Rotating UUID, not tied to any account
	Version            string `json:"version"`                      // Normalized Pulse version (e.g. "6.0.0-rc.1")
	VersionRaw         string `json:"version_raw,omitempty"`        // Original version/build string when it differs
	VersionChannel     string `json:"version_channel"`              // "stable", "rc", "dev", or "prerelease"
	VersionBuild       string `json:"version_build,omitempty"`      // Build metadata when present (e.g. git describe suffix)
	VersionDevelopment bool   `json:"version_is_development"`       // True for development/manual builds
	VersionPublished   bool   `json:"version_is_published_release"` // True for published stable/RC asset versions
	Platform           string `json:"platform"`                     // "docker" or "binary"
	OS                 string `json:"os"`                           // runtime.GOOS (e.g. "linux")
	Arch               string `json:"arch"`                         // runtime.GOARCH (e.g. "amd64")
	Event              string `json:"event"`                        // "startup" or "heartbeat"

	// Scale (counts only — no names, IPs, or identifiers)
	PVENodes              int `json:"pve_nodes"`
	PBSInstances          int `json:"pbs_instances"`
	PMGInstances          int `json:"pmg_instances"`
	VMs                   int `json:"vms"`
	Containers            int `json:"containers"`
	AgentHosts            int `json:"agent_hosts"`
	DockerHosts           int `json:"docker_hosts"`
	DockerContainers      int `json:"docker_containers"`
	KubernetesClusters    int `json:"kubernetes_clusters"`
	KubernetesNodes       int `json:"kubernetes_nodes"`
	KubernetesPods        int `json:"kubernetes_pods"`
	KubernetesDeployments int `json:"kubernetes_deployments"`
	StoragePools          int `json:"storage_pools"`
	PhysicalDisks         int `json:"physical_disks"`
	CephClusters          int `json:"ceph_clusters"`
	NetworkShares         int `json:"network_shares"`
	TrueNASSystems        int `json:"truenas_systems"`
	TrueNASVMs            int `json:"truenas_vms"`
	TrueNASApps           int `json:"truenas_apps"`
	VMwareHosts           int `json:"vmware_hosts"`
	VMwareVMs             int `json:"vmware_vms"`
	VMwareDatastores      int `json:"vmware_datastores"`
	AvailabilityTargets   int `json:"availability_targets"`

	// Feature usage (booleans and counts — no content)
	AIEnabled            bool `json:"ai_enabled"`
	PatrolEnabled        bool `json:"patrol_enabled"`
	DiscoveryEnabled     bool `json:"discovery_enabled"`
	NotificationsEnabled bool `json:"notifications_enabled"`
	AIActionsEnabled     bool `json:"ai_actions_enabled"`
	ActiveAlerts         int  `json:"active_alerts"`
	RelayEnabled         bool `json:"relay_enabled"`
	SSOEnabled           bool `json:"sso_enabled"`
	MultiTenant          bool `json:"multi_tenant"`
	PaidLicense          bool `json:"paid_license"`
	HasAPITokens         bool `json:"has_api_tokens"`
	UpdateAttempts30d    int  `json:"update_attempts_30d"`
	UpdateSuccesses30d   int  `json:"update_successes_30d"`
	UpdateFailures30d    int  `json:"update_failures_30d"`
	// Last coarse update failure category; never raw error text.
	UpdateLastFailureCategory string `json:"update_last_failure_category,omitempty"`

	// Pulse Intelligence usage (30-day counts/booleans — no prompts, commands, outputs, resource IDs, or token values)
	PulseIntelligenceLoopConfigured                                bool `json:"pulse_intelligence_loop_configured"`
	PulseIntelligenceLoopActive30d                                 bool `json:"pulse_intelligence_loop_active_30d"`
	PulseIntelligenceCompleteOperationsLoop30d                     bool `json:"pulse_intelligence_complete_operations_loop_30d"`
	PulseIntelligenceApprovedExecutionLoop30d                      bool `json:"pulse_intelligence_approved_execution_loop_30d"`
	PulseIntelligenceResolvedOperationsLoop30d                     bool `json:"pulse_intelligence_resolved_operations_loop_30d"`
	PulseIntelligencePatrolControlCompletedOperationsLoop30d       bool `json:"pulse_intelligence_patrol_control_completed_operations_loop_30d"`
	PulseIntelligencePatrolControlResolvedOperationsLoop30d        bool `json:"pulse_intelligence_patrol_control_resolved_operations_loop_30d"`
	PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d   bool `json:"pulse_intelligence_patrol_control_paid_completed_operations_loop_30d"`
	PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d    bool `json:"pulse_intelligence_patrol_control_paid_resolved_operations_loop_30d"`
	PulseIntelligenceProActivationCompletedOperationsLoop30d       bool `json:"pulse_intelligence_pro_activation_completed_operations_loop_30d"`
	PulseIntelligenceProActivationResolvedOperationsLoop30d        bool `json:"pulse_intelligence_pro_activation_resolved_operations_loop_30d"`
	PulseIntelligenceProActivationPaidCompletedOperationsLoop30d   bool `json:"pulse_intelligence_pro_activation_paid_completed_operations_loop_30d"`
	PulseIntelligenceProActivationPaidResolvedOperationsLoop30d    bool `json:"pulse_intelligence_pro_activation_paid_resolved_operations_loop_30d"`
	PulseIntelligenceGovernedActionActive30d                       bool `json:"pulse_intelligence_governed_action_active_30d"`
	PulseIntelligenceAssistantOperationsLoop30d                    bool `json:"pulse_intelligence_assistant_operations_loop_30d"`
	PulseIntelligenceAssistantApprovedExecutionLoop30d             bool `json:"pulse_intelligence_assistant_approved_execution_loop_30d"`
	PulseIntelligenceAssistantApprovedActionSuccessLoop30d         bool `json:"pulse_intelligence_assistant_approved_action_success_loop_30d"`
	PulseIntelligenceAssistantResolvedOperationsLoop30d            bool `json:"pulse_intelligence_assistant_resolved_operations_loop_30d"`
	PulseIntelligenceExternalAgentOperationsLoop30d                bool `json:"pulse_intelligence_external_agent_operations_loop_30d"`
	PulseIntelligenceExternalAgentApprovedExecutionLoop30d         bool `json:"pulse_intelligence_external_agent_approved_execution_loop_30d"`
	PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d     bool `json:"pulse_intelligence_external_agent_approved_action_success_loop_30d"`
	PulseIntelligenceExternalAgentResolvedOperationsLoop30d        bool `json:"pulse_intelligence_external_agent_resolved_operations_loop_30d"`
	PulseIntelligenceMCPAdapterOperationsLoop30d                   bool `json:"pulse_intelligence_mcp_adapter_operations_loop_30d"`
	PulseIntelligenceMCPAdapterApprovedExecutionLoop30d            bool `json:"pulse_intelligence_mcp_adapter_approved_execution_loop_30d"`
	PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d        bool `json:"pulse_intelligence_mcp_adapter_approved_action_success_loop_30d"`
	PulseIntelligenceMCPAdapterResolvedOperationsLoop30d           bool `json:"pulse_intelligence_mcp_adapter_resolved_operations_loop_30d"`
	PulseIntelligenceOperationsLoopStarterRequests30d              int  `json:"pulse_intelligence_operations_loop_starter_requests_30d"`
	PulseIntelligenceAssistantOperationsLoopStarterRequests30d     int  `json:"pulse_intelligence_assistant_operations_loop_starter_requests_30d"`
	PulseIntelligencePatrolOperationsLoopStarterRequests30d        int  `json:"pulse_intelligence_patrol_operations_loop_starter_requests_30d"`
	PulseIntelligencePatrolControlOperationsLoopStarterRequests30d int  `json:"pulse_intelligence_patrol_control_operations_loop_starter_requests_30d"`
	PulseIntelligenceProActivationOperationsLoopStarterRequests30d int  `json:"pulse_intelligence_pro_activation_operations_loop_starter_requests_30d"`
	PulseIntelligenceMCPOperationsLoopStarterRequests30d           int  `json:"pulse_intelligence_mcp_operations_loop_starter_requests_30d"`
	PulseIntelligenceAssistantAICalls30d                           int  `json:"pulse_intelligence_assistant_ai_calls_30d"`
	PulseIntelligenceAssistantContextAICalls30d                    int  `json:"pulse_intelligence_assistant_context_ai_calls_30d"`
	PulseIntelligenceAssistantToolCalls30d                         int  `json:"pulse_intelligence_assistant_tool_calls_30d"`
	PulseIntelligencePatrolAICalls30d                              int  `json:"pulse_intelligence_patrol_ai_calls_30d"`
	PulseIntelligencePatrolRuns30d                                 int  `json:"pulse_intelligence_patrol_runs_30d"`
	PulseIntelligencePatrolNewFindings30d                          int  `json:"pulse_intelligence_patrol_new_findings_30d"`
	PulseIntelligencePatrolInvestigations30d                       int  `json:"pulse_intelligence_patrol_investigations_30d"`
	PulseIntelligencePatrolResolvedFindings30d                     int  `json:"pulse_intelligence_patrol_resolved_findings_30d"`
	PulseIntelligencePatrolAutofixes30d                            int  `json:"pulse_intelligence_patrol_autofixes_30d"`
	PulseIntelligenceExternalAgentEnabled                          bool `json:"pulse_intelligence_external_agent_enabled"`
	PulseIntelligenceExternalAgentUsed30d                          bool `json:"pulse_intelligence_external_agent_used_30d"`
	PulseIntelligenceMCPAdapterUsed30d                             bool `json:"pulse_intelligence_mcp_adapter_used_30d"`
	PulseIntelligenceExternalAgentContextRequests30d               int  `json:"pulse_intelligence_external_agent_context_requests_30d"`
	PulseIntelligenceExternalAgentEventStreamRequests30d           int  `json:"pulse_intelligence_external_agent_event_stream_requests_30d"`
	PulseIntelligenceExternalAgentProvisioningRequests30d          int  `json:"pulse_intelligence_external_agent_provisioning_requests_30d"`
	PulseIntelligenceExternalAgentOperatorStateRequests30d         int  `json:"pulse_intelligence_external_agent_operator_state_requests_30d"`
	PulseIntelligenceExternalAgentFindingRequests30d               int  `json:"pulse_intelligence_external_agent_finding_requests_30d"`
	PulseIntelligenceExternalAgentActionRequests30d                int  `json:"pulse_intelligence_external_agent_action_requests_30d"`
	PulseIntelligenceActionPlans30d                                int  `json:"pulse_intelligence_action_plans_30d"`
	PulseIntelligenceApprovalRequests30d                           int  `json:"pulse_intelligence_approval_requests_30d"`
	PulseIntelligenceRejectedActionDecisions30d                    int  `json:"pulse_intelligence_rejected_action_decisions_30d"`
	PulseIntelligenceApprovedActionDecisions30d                    int  `json:"pulse_intelligence_approved_action_decisions_30d"`
	PulseIntelligenceApprovedActionAttempts30d                     int  `json:"pulse_intelligence_approved_action_attempts_30d"`
	PulseIntelligenceApprovedActionSuccesses30d                    int  `json:"pulse_intelligence_approved_action_successes_30d"`
}

// Snapshot holds the dynamic state gathered at ping time.
// The telemetry package calls a user-provided SnapshotFunc to populate this,
// keeping the package decoupled from monitor/config internals.
type Snapshot struct {
	PVENodes                                                       int
	PBSInstances                                                   int
	PMGInstances                                                   int
	VMs                                                            int
	Containers                                                     int
	AgentHosts                                                     int
	DockerHosts                                                    int
	DockerContainers                                               int
	KubernetesClusters                                             int
	KubernetesNodes                                                int
	KubernetesPods                                                 int
	KubernetesDeployments                                          int
	StoragePools                                                   int
	PhysicalDisks                                                  int
	CephClusters                                                   int
	NetworkShares                                                  int
	TrueNASSystems                                                 int
	TrueNASVMs                                                     int
	TrueNASApps                                                    int
	VMwareHosts                                                    int
	VMwareVMs                                                      int
	VMwareDatastores                                               int
	AvailabilityTargets                                            int
	AIEnabled                                                      bool
	PatrolEnabled                                                  bool
	DiscoveryEnabled                                               bool
	NotificationsEnabled                                           bool
	AIActionsEnabled                                               bool
	ActiveAlerts                                                   int
	RelayEnabled                                                   bool
	SSOEnabled                                                     bool
	MultiTenant                                                    bool
	PaidLicense                                                    bool
	HasAPITokens                                                   bool
	UpdateAttempts30d                                              int
	UpdateSuccesses30d                                             int
	UpdateFailures30d                                              int
	UpdateLastFailureCategory                                      string
	PulseIntelligenceLoopConfigured                                bool
	PulseIntelligenceLoopActive30d                                 bool
	PulseIntelligenceCompleteOperationsLoop30d                     bool
	PulseIntelligenceApprovedExecutionLoop30d                      bool
	PulseIntelligenceResolvedOperationsLoop30d                     bool
	PulseIntelligencePatrolControlCompletedOperationsLoop30d       bool
	PulseIntelligencePatrolControlResolvedOperationsLoop30d        bool
	PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d   bool
	PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d    bool
	PulseIntelligenceProActivationCompletedOperationsLoop30d       bool
	PulseIntelligenceProActivationResolvedOperationsLoop30d        bool
	PulseIntelligenceProActivationPaidCompletedOperationsLoop30d   bool
	PulseIntelligenceProActivationPaidResolvedOperationsLoop30d    bool
	PulseIntelligenceGovernedActionActive30d                       bool
	PulseIntelligenceAssistantOperationsLoop30d                    bool
	PulseIntelligenceAssistantApprovedExecutionLoop30d             bool
	PulseIntelligenceAssistantApprovedActionSuccessLoop30d         bool
	PulseIntelligenceAssistantResolvedOperationsLoop30d            bool
	PulseIntelligenceExternalAgentOperationsLoop30d                bool
	PulseIntelligenceExternalAgentApprovedExecutionLoop30d         bool
	PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d     bool
	PulseIntelligenceExternalAgentResolvedOperationsLoop30d        bool
	PulseIntelligenceMCPAdapterOperationsLoop30d                   bool
	PulseIntelligenceMCPAdapterApprovedExecutionLoop30d            bool
	PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d        bool
	PulseIntelligenceMCPAdapterResolvedOperationsLoop30d           bool
	PulseIntelligenceOperationsLoopStarterRequests30d              int
	PulseIntelligenceAssistantOperationsLoopStarterRequests30d     int
	PulseIntelligencePatrolOperationsLoopStarterRequests30d        int
	PulseIntelligencePatrolControlOperationsLoopStarterRequests30d int
	PulseIntelligenceProActivationOperationsLoopStarterRequests30d int
	PulseIntelligenceMCPOperationsLoopStarterRequests30d           int
	PulseIntelligenceAssistantAICalls30d                           int
	PulseIntelligenceAssistantContextAICalls30d                    int
	PulseIntelligenceAssistantToolCalls30d                         int
	PulseIntelligencePatrolAICalls30d                              int
	PulseIntelligencePatrolRuns30d                                 int
	PulseIntelligencePatrolNewFindings30d                          int
	PulseIntelligencePatrolInvestigations30d                       int
	PulseIntelligencePatrolResolvedFindings30d                     int
	PulseIntelligencePatrolAutofixes30d                            int
	PulseIntelligenceExternalAgentEnabled                          bool
	PulseIntelligenceExternalAgentOperationsLoopReady              bool
	PulseIntelligenceExternalAgentUsed30d                          bool
	PulseIntelligenceMCPAdapterUsed30d                             bool
	PulseIntelligenceExternalAgentContextRequests30d               int
	PulseIntelligenceExternalAgentEventStreamRequests30d           int
	PulseIntelligenceExternalAgentProvisioningRequests30d          int
	PulseIntelligenceExternalAgentOperatorStateRequests30d         int
	PulseIntelligenceExternalAgentFindingRequests30d               int
	PulseIntelligenceExternalAgentActionRequests30d                int
	PulseIntelligenceActionPlans30d                                int
	PulseIntelligenceApprovalRequests30d                           int
	PulseIntelligenceRejectedActionDecisions30d                    int
	PulseIntelligenceApprovedActionDecisions30d                    int
	PulseIntelligenceApprovedActionAttempts30d                     int
	PulseIntelligenceApprovedActionSuccesses30d                    int
}

// PulseIntelligenceActionSnapshot is the action-governance portion of the
// Pulse Intelligence telemetry loop. It is intentionally count-only so callers
// can aggregate local audit records without exporting action details.
type PulseIntelligenceActionSnapshot struct {
	ActionPlans30d             int
	ApprovalRequests30d        int
	RejectedActionDecisions30d int
	ApprovedActionDecisions30d int
	ApprovedActionAttempts30d  int
	ApprovedActionSuccesses30d int
}

// ApplyUpdateTelemetrySnapshot adds content-free update funnel counters from
// local update history. It reports only aggregate counts and one coarse failure
// category over the install-ID rotation window.
func ApplyUpdateTelemetrySnapshot(s *Snapshot, history *updates.UpdateHistory, now time.Time) {
	if s == nil || history == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	since := now.UTC().Add(-installIDRotationWindow)
	entries := history.ListEntries(updates.HistoryFilter{Action: "update"})
	var lastFailure *updates.UpdateHistoryEntry
	for i := range entries {
		entry := entries[i]
		if entry.Timestamp.IsZero() || entry.Timestamp.UTC().Before(since) {
			continue
		}
		s.UpdateAttempts30d++
		switch entry.Status {
		case updates.StatusSuccess:
			s.UpdateSuccesses30d++
		case updates.StatusFailed, updates.StatusRolledBack:
			s.UpdateFailures30d++
			if lastFailure == nil || entry.Timestamp.After(lastFailure.Timestamp) {
				candidate := entry
				lastFailure = &candidate
			}
		}
	}
	if lastFailure != nil {
		s.UpdateLastFailureCategory = classifyUpdateFailureCategory(*lastFailure)
	}
}

func classifyUpdateFailureCategory(entry updates.UpdateHistoryEntry) string {
	switch entry.Status {
	case updates.StatusRolledBack:
		return "rolled_back"
	case updates.StatusCancelled:
		return "cancelled"
	}
	text := ""
	if entry.Error != nil {
		text = strings.ToLower(strings.TrimSpace(entry.Error.Code + " " + entry.Error.Message + " " + entry.Error.Details))
	}
	switch {
	case strings.Contains(text, "signature"):
		return "signature"
	case strings.Contains(text, "checksum"):
		return "checksum"
	case strings.Contains(text, "download"):
		return "download"
	case strings.Contains(text, "disk space") || strings.Contains(text, "insufficient disk"):
		return "disk_space"
	case strings.Contains(text, "extract") || strings.Contains(text, "archive"):
		return "extract"
	case strings.Contains(text, "backup"):
		return "backup"
	case strings.Contains(text, "apply"):
		return "apply"
	case strings.Contains(text, "restart"):
		return "restart"
	default:
		return "unknown"
	}
}

const (
	PulseIntelligenceProActivationValueProofNotStarted               = "not_started"
	PulseIntelligenceProActivationValueProofInProgress               = "in_progress"
	PulseIntelligenceProActivationValueProofGovernedDecisionRecorded = "governed_decision_recorded"
	// PulseIntelligenceProActivationValueProofVerifiedNeedsExternalMCP remains a
	// tolerated legacy value for API consumers, but MCP readiness is no longer a
	// required Patrol control value gate.
	PulseIntelligenceProActivationValueProofVerifiedNeedsExternalMCP = "verified_needs_mcp"
	PulseIntelligenceProActivationValueProofVerified                 = "verified"
)

// PulseIntelligencePatrolControlProofInput is the count-only evidence needed
// to classify whether the first-party Patrol control loop reached governed
// operations value.
type PulseIntelligencePatrolControlProofInput struct {
	PatrolControlStarterCount    int
	PatrolIssueEvidenceCount     int
	ContextualCollaborationCount int
	ApprovedDecisionCount        int
	RejectedDecisionCount        int
	VerifiedOutcomeCount         int
	// ExternalAgentReady is retained for callers that also report optional MCP
	// readiness; it is not part of the Patrol control value classifier.
	ExternalAgentReady bool
}

// PulseIntelligencePatrolControlProof is the shared classification used by
// the native status endpoint and anonymous outbound telemetry.
type PulseIntelligencePatrolControlProof struct {
	Completed       bool
	Resolved        bool
	ValueProofState string
}

// PulseIntelligencePatrolAutonomyProofInput is the legacy count-only evidence
// shape for the same Patrol control value proof. Keep this type stable for
// existing callers and persisted event surfaces.
type PulseIntelligencePatrolAutonomyProofInput struct {
	PatrolAutonomyStarterCount   int
	PatrolIssueEvidenceCount     int
	ContextualCollaborationCount int
	ApprovedDecisionCount        int
	RejectedDecisionCount        int
	VerifiedOutcomeCount         int
	// ExternalAgentReady is retained for callers that also report optional MCP
	// readiness; it is not part of the Patrol control value classifier.
	ExternalAgentReady bool
}

// PulseIntelligencePatrolAutonomyProof is the legacy name for
// PulseIntelligencePatrolControlProof.
type PulseIntelligencePatrolAutonomyProof = PulseIntelligencePatrolControlProof

// PulseIntelligenceProActivationProofInput is the legacy metric/storage shape
// for the same Patrol control value proof. Keep this type stable for existing
// telemetry callers and persisted event surfaces.
type PulseIntelligenceProActivationProofInput struct {
	ProActivationStarterCount    int
	PatrolIssueEvidenceCount     int
	ContextualCollaborationCount int
	ApprovedDecisionCount        int
	RejectedDecisionCount        int
	VerifiedOutcomeCount         int
	// ExternalAgentReady is retained for callers that also report optional MCP
	// readiness; it is not part of the Patrol control value classifier.
	ExternalAgentReady bool
}

// PulseIntelligenceProActivationProof is the legacy name for
// PulseIntelligencePatrolControlProof.
type PulseIntelligenceProActivationProof = PulseIntelligencePatrolControlProof

// ClassifyPulseIntelligencePatrolControlProof classifies the Patrol control
// loop from counters only. It intentionally mirrors the product proof contract:
// a completed loop can end in an approved verified outcome or a rejected
// governed decision, while resolved value requires approved verification.
func ClassifyPulseIntelligencePatrolControlProof(input PulseIntelligencePatrolControlProofInput) PulseIntelligencePatrolControlProof {
	return classifyPulseIntelligenceOperationsValueProof(
		input.PatrolControlStarterCount,
		input.PatrolIssueEvidenceCount,
		input.ContextualCollaborationCount,
		input.ApprovedDecisionCount,
		input.RejectedDecisionCount,
		input.VerifiedOutcomeCount,
	)
}

// ClassifyPulseIntelligencePatrolAutonomyProof classifies the legacy Patrol
// autonomy proof shape. New callers should use
// ClassifyPulseIntelligencePatrolControlProof.
func ClassifyPulseIntelligencePatrolAutonomyProof(input PulseIntelligencePatrolAutonomyProofInput) PulseIntelligencePatrolAutonomyProof {
	proof := ClassifyPulseIntelligencePatrolControlProof(PulseIntelligencePatrolControlProofInput{
		PatrolControlStarterCount:    input.PatrolAutonomyStarterCount,
		PatrolIssueEvidenceCount:     input.PatrolIssueEvidenceCount,
		ContextualCollaborationCount: input.ContextualCollaborationCount,
		ApprovedDecisionCount:        input.ApprovedDecisionCount,
		RejectedDecisionCount:        input.RejectedDecisionCount,
		VerifiedOutcomeCount:         input.VerifiedOutcomeCount,
		ExternalAgentReady:           input.ExternalAgentReady,
	})
	return PulseIntelligencePatrolAutonomyProof(proof)
}

// ClassifyPulseIntelligenceProActivationProof classifies the legacy Pro
// activation proof shape. New callers should use
// ClassifyPulseIntelligencePatrolControlProof.
func ClassifyPulseIntelligenceProActivationProof(input PulseIntelligenceProActivationProofInput) PulseIntelligenceProActivationProof {
	proof := ClassifyPulseIntelligencePatrolControlProof(PulseIntelligencePatrolControlProofInput{
		PatrolControlStarterCount:    input.ProActivationStarterCount,
		PatrolIssueEvidenceCount:     input.PatrolIssueEvidenceCount,
		ContextualCollaborationCount: input.ContextualCollaborationCount,
		ApprovedDecisionCount:        input.ApprovedDecisionCount,
		RejectedDecisionCount:        input.RejectedDecisionCount,
		VerifiedOutcomeCount:         input.VerifiedOutcomeCount,
		ExternalAgentReady:           input.ExternalAgentReady,
	})
	return PulseIntelligenceProActivationProof(proof)
}

func classifyPulseIntelligenceOperationsValueProof(starterCount int, patrolIssueEvidenceCount int, contextualCollaborationCount int, approvedDecisionCount int, rejectedDecisionCount int, verifiedOutcomeCount int) PulseIntelligencePatrolControlProof {
	starterActive := starterCount > 0
	patrolIssueEvidenceActive := patrolIssueEvidenceCount > 0
	contextualCollaborationActive := contextualCollaborationCount > 0
	approvedVerifiedOutcomeActive := approvedDecisionCount > 0 && verifiedOutcomeCount > 0
	rejectedDecisionWithoutApproval := rejectedDecisionCount > 0 && approvedDecisionCount == 0

	resolved := starterActive &&
		patrolIssueEvidenceActive &&
		contextualCollaborationActive &&
		approvedVerifiedOutcomeActive
	completed := starterActive &&
		patrolIssueEvidenceActive &&
		contextualCollaborationActive &&
		(rejectedDecisionCount > 0 || approvedVerifiedOutcomeActive)

	valueProofState := PulseIntelligenceProActivationValueProofInProgress
	switch {
	case !starterActive:
		valueProofState = PulseIntelligenceProActivationValueProofNotStarted
	case resolved:
		valueProofState = PulseIntelligenceProActivationValueProofVerified
	case rejectedDecisionWithoutApproval:
		valueProofState = PulseIntelligenceProActivationValueProofGovernedDecisionRecorded
	case completed:
		valueProofState = PulseIntelligenceProActivationValueProofGovernedDecisionRecorded
	}

	return PulseIntelligencePatrolControlProof{
		Completed:       completed,
		Resolved:        resolved,
		ValueProofState: valueProofState,
	}
}

// SnapshotFunc returns the current state snapshot for telemetry.
// It is called on each heartbeat to gather fresh data.
type SnapshotFunc func() Snapshot

// Config holds the static configuration for the telemetry runner.
type Config struct {
	Version     string
	DataDir     string
	IsDocker    bool
	Enabled     bool // From cfg.TelemetryEnabled (system settings or env var)
	GetSnapshot SnapshotFunc
}

// runner holds the state for the background heartbeat goroutine.
type runner struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

var (
	mu      sync.Mutex
	current *runner
)

// Start begins anonymous outbound telemetry if enabled.
// It reads or creates a rotating install ID in dataDir, waits for the monitor
// to populate state, sends a startup ping, and schedules a daily heartbeat.
// Call Stop() on shutdown.
//
// This is a no-op when anonymous outbound telemetry is not opted in.
func Start(ctx context.Context, cfg Config) {
	if !cfg.Enabled {
		log.Info().Msg("Anonymous outbound telemetry is disabled (enable via PULSE_TELEMETRY=true or Settings → System)")
		return
	}

	installID := getOrCreateInstallID(cfg.DataDir)
	if installID == "" {
		log.Warn().Msg("Could not determine install ID; telemetry will not run")
		return
	}

	base := basePing(cfg, installID)

	ctx, cancel := context.WithCancel(ctx)
	r := &runner{cancel: cancel}

	mu.Lock()
	if current != nil {
		current.cancel()
	}
	current = r
	mu.Unlock()

	log.Info().
		Str("platform", base.Platform).
		Msg("Anonymous outbound telemetry enabled — sends a rotating install ID, version identity, platform, OS/arch, resource counts, feature flags, and coarse Patrol, Assistant, and external-agent usage counters (nothing else)")

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		// Wait for the monitor to connect and populate state before the first ping.
		startTimer := time.NewTimer(startupDelay)
		select {
		case <-ctx.Done():
			startTimer.Stop()
			return
		case <-startTimer.C:
		}

		// Send startup ping with current snapshot.
		ping := applySnapshot(base, cfg.GetSnapshot)
		ping.Event = "startup"
		send(ctx, ping)

		// Daily heartbeat with jitter.
		for {
			timer := time.NewTimer(jitteredHeartbeat())
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				ping = applySnapshot(base, cfg.GetSnapshot)
				ping.Event = "heartbeat"
				send(ctx, ping)
			}
		}
	}()
}

// Stop shuts down the telemetry background goroutine.
func Stop() {
	mu.Lock()
	r := current
	current = nil
	mu.Unlock()

	if r != nil {
		r.cancel()
		r.wg.Wait()
	}
}

// BuildPreview returns the current heartbeat payload without sending it.
func BuildPreview(cfg Config) (Ping, error) {
	installID := getOrCreateInstallID(cfg.DataDir)
	if installID == "" {
		return Ping{}, errInstallIDUnavailable
	}

	ping := applySnapshot(basePing(cfg, installID), cfg.GetSnapshot)
	ping.Event = "heartbeat"
	return ping, nil
}

// ResetInstallID rotates the locally stored telemetry install ID immediately
// and returns the new pseudonymous identifier.
func ResetInstallID(dataDir string) (string, error) {
	return resetInstallIDAt(dataDir, time.Now().UTC())
}

// IsEnabled reports whether telemetry is enabled.
// Telemetry is on by default; set PULSE_TELEMETRY=false to disable.
func IsEnabled() bool {
	v := os.Getenv("PULSE_TELEMETRY")
	if v == "" {
		return true // enabled by default
	}
	return v == "true" || v == "1"
}

// jitteredHeartbeat returns heartbeatInterval ± a random offset up to maxHeartbeatJitter.
func jitteredHeartbeat() time.Duration {
	jitter := time.Duration(rand.Int63n(int64(2*maxHeartbeatJitter)+1)) - maxHeartbeatJitter
	return heartbeatInterval + jitter
}

func basePing(cfg Config, installID string) Ping {
	versionIdentity := updates.DescribeUsageDataVersion(cfg.Version)
	return Ping{
		InstallID:          installID,
		Version:            versionIdentity.Version,
		VersionRaw:         versionIdentity.RawVersion,
		VersionChannel:     versionIdentity.Channel,
		VersionBuild:       versionIdentity.Build,
		VersionDevelopment: versionIdentity.IsDevelopment,
		VersionPublished:   versionIdentity.IsPublishedRelease,
		Platform:           platformName(cfg.IsDocker),
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
	}
}

func platformName(isDocker bool) string {
	if isDocker {
		return "docker"
	}
	return "binary"
}

// applySnapshot merges dynamic state into the base ping.
func applySnapshot(base Ping, fn SnapshotFunc) Ping {
	ping := base
	if fn == nil {
		return ping
	}
	s := fn()
	ping.PVENodes = s.PVENodes
	ping.PBSInstances = s.PBSInstances
	ping.PMGInstances = s.PMGInstances
	ping.VMs = s.VMs
	ping.Containers = s.Containers
	ping.AgentHosts = s.AgentHosts
	ping.DockerHosts = s.DockerHosts
	ping.DockerContainers = s.DockerContainers
	ping.KubernetesClusters = s.KubernetesClusters
	ping.KubernetesNodes = s.KubernetesNodes
	ping.KubernetesPods = s.KubernetesPods
	ping.KubernetesDeployments = s.KubernetesDeployments
	ping.StoragePools = s.StoragePools
	ping.PhysicalDisks = s.PhysicalDisks
	ping.CephClusters = s.CephClusters
	ping.NetworkShares = s.NetworkShares
	ping.TrueNASSystems = s.TrueNASSystems
	ping.TrueNASVMs = s.TrueNASVMs
	ping.TrueNASApps = s.TrueNASApps
	ping.VMwareHosts = s.VMwareHosts
	ping.VMwareVMs = s.VMwareVMs
	ping.VMwareDatastores = s.VMwareDatastores
	ping.AvailabilityTargets = s.AvailabilityTargets
	ping.AIEnabled = s.AIEnabled
	ping.PatrolEnabled = s.PatrolEnabled
	ping.DiscoveryEnabled = s.DiscoveryEnabled
	ping.NotificationsEnabled = s.NotificationsEnabled
	ping.AIActionsEnabled = s.AIActionsEnabled
	ping.ActiveAlerts = s.ActiveAlerts
	ping.RelayEnabled = s.RelayEnabled
	ping.SSOEnabled = s.SSOEnabled
	ping.MultiTenant = s.MultiTenant
	ping.PaidLicense = s.PaidLicense
	ping.HasAPITokens = s.HasAPITokens
	ping.UpdateAttempts30d = s.UpdateAttempts30d
	ping.UpdateSuccesses30d = s.UpdateSuccesses30d
	ping.UpdateFailures30d = s.UpdateFailures30d
	ping.UpdateLastFailureCategory = s.UpdateLastFailureCategory
	ping.PulseIntelligenceLoopConfigured = s.PulseIntelligenceLoopConfigured
	ping.PulseIntelligenceLoopActive30d = s.PulseIntelligenceLoopActive30d
	ping.PulseIntelligenceCompleteOperationsLoop30d = s.PulseIntelligenceCompleteOperationsLoop30d
	ping.PulseIntelligenceApprovedExecutionLoop30d = s.PulseIntelligenceApprovedExecutionLoop30d
	ping.PulseIntelligenceResolvedOperationsLoop30d = s.PulseIntelligenceResolvedOperationsLoop30d
	ping.PulseIntelligencePatrolControlCompletedOperationsLoop30d = s.PulseIntelligencePatrolControlCompletedOperationsLoop30d
	ping.PulseIntelligencePatrolControlResolvedOperationsLoop30d = s.PulseIntelligencePatrolControlResolvedOperationsLoop30d
	ping.PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d = s.PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d
	ping.PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d = s.PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d
	ping.PulseIntelligenceProActivationCompletedOperationsLoop30d = s.PulseIntelligenceProActivationCompletedOperationsLoop30d
	ping.PulseIntelligenceProActivationResolvedOperationsLoop30d = s.PulseIntelligenceProActivationResolvedOperationsLoop30d
	ping.PulseIntelligenceProActivationPaidCompletedOperationsLoop30d = s.PulseIntelligenceProActivationPaidCompletedOperationsLoop30d
	ping.PulseIntelligenceProActivationPaidResolvedOperationsLoop30d = s.PulseIntelligenceProActivationPaidResolvedOperationsLoop30d
	ping.PulseIntelligenceGovernedActionActive30d = s.PulseIntelligenceGovernedActionActive30d
	ping.PulseIntelligenceAssistantOperationsLoop30d = s.PulseIntelligenceAssistantOperationsLoop30d
	ping.PulseIntelligenceAssistantApprovedExecutionLoop30d = s.PulseIntelligenceAssistantApprovedExecutionLoop30d
	ping.PulseIntelligenceAssistantApprovedActionSuccessLoop30d = s.PulseIntelligenceAssistantApprovedActionSuccessLoop30d
	ping.PulseIntelligenceAssistantResolvedOperationsLoop30d = s.PulseIntelligenceAssistantResolvedOperationsLoop30d
	ping.PulseIntelligenceExternalAgentOperationsLoop30d = s.PulseIntelligenceExternalAgentOperationsLoop30d
	ping.PulseIntelligenceExternalAgentApprovedExecutionLoop30d = s.PulseIntelligenceExternalAgentApprovedExecutionLoop30d
	ping.PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d = s.PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d
	ping.PulseIntelligenceExternalAgentResolvedOperationsLoop30d = s.PulseIntelligenceExternalAgentResolvedOperationsLoop30d
	ping.PulseIntelligenceMCPAdapterOperationsLoop30d = s.PulseIntelligenceMCPAdapterOperationsLoop30d
	ping.PulseIntelligenceMCPAdapterApprovedExecutionLoop30d = s.PulseIntelligenceMCPAdapterApprovedExecutionLoop30d
	ping.PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d = s.PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d
	ping.PulseIntelligenceMCPAdapterResolvedOperationsLoop30d = s.PulseIntelligenceMCPAdapterResolvedOperationsLoop30d
	ping.PulseIntelligenceOperationsLoopStarterRequests30d = s.PulseIntelligenceOperationsLoopStarterRequests30d
	ping.PulseIntelligenceAssistantOperationsLoopStarterRequests30d = s.PulseIntelligenceAssistantOperationsLoopStarterRequests30d
	ping.PulseIntelligencePatrolOperationsLoopStarterRequests30d = s.PulseIntelligencePatrolOperationsLoopStarterRequests30d
	ping.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d = s.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d
	ping.PulseIntelligenceProActivationOperationsLoopStarterRequests30d = s.PulseIntelligenceProActivationOperationsLoopStarterRequests30d
	ping.PulseIntelligenceMCPOperationsLoopStarterRequests30d = s.PulseIntelligenceMCPOperationsLoopStarterRequests30d
	ping.PulseIntelligenceAssistantAICalls30d = s.PulseIntelligenceAssistantAICalls30d
	ping.PulseIntelligenceAssistantContextAICalls30d = s.PulseIntelligenceAssistantContextAICalls30d
	ping.PulseIntelligenceAssistantToolCalls30d = s.PulseIntelligenceAssistantToolCalls30d
	ping.PulseIntelligencePatrolAICalls30d = s.PulseIntelligencePatrolAICalls30d
	ping.PulseIntelligencePatrolRuns30d = s.PulseIntelligencePatrolRuns30d
	ping.PulseIntelligencePatrolNewFindings30d = s.PulseIntelligencePatrolNewFindings30d
	ping.PulseIntelligencePatrolInvestigations30d = s.PulseIntelligencePatrolInvestigations30d
	ping.PulseIntelligencePatrolResolvedFindings30d = s.PulseIntelligencePatrolResolvedFindings30d
	ping.PulseIntelligencePatrolAutofixes30d = s.PulseIntelligencePatrolAutofixes30d
	ping.PulseIntelligenceExternalAgentEnabled = s.PulseIntelligenceExternalAgentEnabled
	ping.PulseIntelligenceExternalAgentUsed30d = s.PulseIntelligenceExternalAgentUsed30d
	ping.PulseIntelligenceMCPAdapterUsed30d = s.PulseIntelligenceMCPAdapterUsed30d
	ping.PulseIntelligenceExternalAgentContextRequests30d = s.PulseIntelligenceExternalAgentContextRequests30d
	ping.PulseIntelligenceExternalAgentEventStreamRequests30d = s.PulseIntelligenceExternalAgentEventStreamRequests30d
	ping.PulseIntelligenceExternalAgentProvisioningRequests30d = s.PulseIntelligenceExternalAgentProvisioningRequests30d
	ping.PulseIntelligenceExternalAgentOperatorStateRequests30d = s.PulseIntelligenceExternalAgentOperatorStateRequests30d
	ping.PulseIntelligenceExternalAgentFindingRequests30d = s.PulseIntelligenceExternalAgentFindingRequests30d
	ping.PulseIntelligenceExternalAgentActionRequests30d = s.PulseIntelligenceExternalAgentActionRequests30d
	ping.PulseIntelligenceActionPlans30d = s.PulseIntelligenceActionPlans30d
	ping.PulseIntelligenceApprovalRequests30d = s.PulseIntelligenceApprovalRequests30d
	ping.PulseIntelligenceRejectedActionDecisions30d = s.PulseIntelligenceRejectedActionDecisions30d
	ping.PulseIntelligenceApprovedActionDecisions30d = s.PulseIntelligenceApprovedActionDecisions30d
	ping.PulseIntelligenceApprovedActionAttempts30d = s.PulseIntelligenceApprovedActionAttempts30d
	ping.PulseIntelligenceApprovedActionSuccesses30d = s.PulseIntelligenceApprovedActionSuccesses30d
	return ping
}

// getOrCreateInstallID reads or generates a rotating install ID in dataDir.
func getOrCreateInstallID(dataDir string) string {
	return getOrCreateInstallIDAt(dataDir, time.Now().UTC())
}

func getOrCreateInstallIDAt(dataDir string, now time.Time) string {
	p := filepath.Join(dataDir, installIDFile)
	now = now.UTC()

	data, err := os.ReadFile(p)
	if err == nil {
		record, ok := parseInstallIDRecord(data)
		if ok && shouldKeepInstallIDRecord(record, now) {
			return record.InstallID
		}
	}

	record := installIDRecord{
		InstallID: uuid.New().String(),
		IssuedAt:  now,
	}
	if err := writeInstallIDRecordAt(dataDir, record); err != nil {
		log.Warn().Err(err).Str("path", p).Msg("Failed to persist install ID")
		// Still use the generated ID for this session.
	}
	return record.InstallID
}

func resetInstallIDAt(dataDir string, now time.Time) (string, error) {
	record := installIDRecord{
		InstallID: uuid.New().String(),
		IssuedAt:  now.UTC(),
	}
	if err := writeInstallIDRecordAt(dataDir, record); err != nil {
		return "", err
	}
	return record.InstallID, nil
}

func writeInstallIDRecordAt(dataDir string, record installIDRecord) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, installIDFile), append(encoded, '\n'), 0600)
}

func parseInstallIDRecord(data []byte) (installIDRecord, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return installIDRecord{}, false
	}

	var record installIDRecord
	if err := json.Unmarshal(trimmed, &record); err == nil {
		record.InstallID = string(bytes.TrimSpace([]byte(record.InstallID)))
		if _, err := uuid.Parse(record.InstallID); err == nil && !record.IssuedAt.IsZero() {
			return record, true
		}
		return installIDRecord{}, false
	}

	legacyID := string(trimmed)
	if _, err := uuid.Parse(legacyID); err == nil {
		// Legacy plaintext IDs are accepted as migration input only. Rotate to a
		// new record immediately instead of preserving an unbounded stable ID.
		return installIDRecord{}, false
	}
	return installIDRecord{}, false
}

func shouldKeepInstallIDRecord(record installIDRecord, now time.Time) bool {
	if _, err := uuid.Parse(record.InstallID); err != nil {
		return false
	}
	issuedAt := record.IssuedAt.UTC()
	if issuedAt.IsZero() || issuedAt.After(now) {
		return false
	}
	return now.Sub(issuedAt) < installIDRotationWindow
}

// send posts a ping to the telemetry endpoint. Failures are silently ignored
// — telemetry must never interfere with normal operation.
func send(ctx context.Context, ping Ping) {
	body, err := json.Marshal(ping)
	if err != nil {
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, pingEndpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("Telemetry ping failed (will retry at next heartbeat)")
		return
	}
	resp.Body.Close()
}
