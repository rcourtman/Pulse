package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// StateProvider provides access to infrastructure state
type StateProvider interface {
	GetState() models.StateSnapshot
}

// CommandPolicy evaluates command security
type CommandPolicy interface {
	Evaluate(command string) agentexec.PolicyDecision
}

// AgentServer executes commands on agents
type AgentServer interface {
	GetConnectedAgents() []agentexec.ConnectedAgent
	ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
}

// AgentProfileManager manages centralized agent profiles and assignments.
type AgentProfileManager interface {
	ApplyAgentScope(ctx context.Context, agentID, agentLabel string, settings map[string]interface{}) (profileID, profileName string, created bool, err error)
	AssignProfile(ctx context.Context, agentID, profileID string) (profileName string, err error)
	GetAgentScope(ctx context.Context, agentID string) (*AgentScope, error)
}

// AgentScope summarizes profile scope applied to an agent.
type AgentScope struct {
	AgentID        string
	ProfileID      string
	ProfileName    string
	ProfileVersion int
	Settings       map[string]interface{}
}

// MetadataUpdater updates resource metadata
type MetadataUpdater interface {
	SetResourceURL(resourceType, resourceID, url string) error
}

// FindingsManager manages patrol findings
type FindingsManager interface {
	ResolveFinding(findingID, note string) error
	DismissFinding(findingID, reason, note string) error
}

// MetricsHistoryProvider provides historical metrics for trend analysis
type MetricsHistoryProvider interface {
	GetResourceMetrics(resourceID string, period time.Duration) ([]MetricPoint, error)
	GetAllMetricsSummary(period time.Duration) (map[string]ResourceMetricsSummary, error)
}

// MetricPoint represents a single metric data point
type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	CPU       float64   `json:"cpu"`
	Memory    float64   `json:"memory"`
	Disk      float64   `json:"disk,omitempty"`
}

// ResourceMetricsSummary summarizes metrics for a resource over a period
type ResourceMetricsSummary struct {
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	ResourceType string  `json:"resource_type"`
	AvgCPU       float64 `json:"avg_cpu"`
	MaxCPU       float64 `json:"max_cpu"`
	AvgMemory    float64 `json:"avg_memory"`
	MaxMemory    float64 `json:"max_memory"`
	AvgDisk      float64 `json:"avg_disk,omitempty"`
	MaxDisk      float64 `json:"max_disk,omitempty"`
	Trend        string  `json:"trend"` // "stable", "growing", "declining"
}

// BaselineProvider provides learned baselines for anomaly detection
type BaselineProvider interface {
	GetBaseline(resourceID, metric string) *MetricBaseline
	GetAllBaselines() map[string]map[string]*MetricBaseline // resourceID -> metric -> baseline
}

// MetricBaseline represents learned normal behavior for a metric
type MetricBaseline struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

// PatternProvider provides detected patterns and predictions
type PatternProvider interface {
	GetPatterns() []Pattern
	GetPredictions() []Prediction
}

// Pattern represents a detected operational pattern
type Pattern struct {
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	PatternType  string    `json:"pattern_type"` // "recurring_spike", "gradual_growth", "weekly_cycle"
	Description  string    `json:"description"`
	Confidence   float64   `json:"confidence"`
	LastSeen     time.Time `json:"last_seen"`
}

// Prediction represents a predicted future issue
type Prediction struct {
	ResourceID     string    `json:"resource_id"`
	ResourceName   string    `json:"resource_name"`
	IssueType      string    `json:"issue_type"` // "disk_full", "memory_exhaustion", etc.
	PredictedTime  time.Time `json:"predicted_time"`
	Confidence     float64   `json:"confidence"`
	Recommendation string    `json:"recommendation"`
}

// AlertProvider provides active alerts
type AlertProvider interface {
	GetActiveAlerts() []ActiveAlert
}

// ActiveAlert represents an active alert
type ActiveAlert struct {
	ID           string    `json:"id"`
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	Type         string    `json:"type"` // "cpu", "memory", "disk", "offline"
	Severity     string    `json:"severity"`
	Value        float64   `json:"value"`
	Threshold    float64   `json:"threshold"`
	StartTime    time.Time `json:"start_time"`
	Message      string    `json:"message"`
}

// FindingsProvider provides patrol findings
type FindingsProvider interface {
	GetActiveFindings() []Finding
	GetDismissedFindings() []Finding
}

// Finding represents a patrol finding
type Finding struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	Severity       string    `json:"severity"`
	Category       string    `json:"category"`
	ResourceID     string    `json:"resource_id"`
	ResourceName   string    `json:"resource_name"`
	ResourceType   string    `json:"resource_type"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Recommendation string    `json:"recommendation"`
	Evidence       string    `json:"evidence"`
	DetectedAt     time.Time `json:"detected_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	TimesRaised    int       `json:"times_raised"`
}

// BackupProvider provides backup information
type BackupProvider interface {
	GetBackups() models.Backups
	GetPBSInstances() []models.PBSInstance
}

// StorageProvider provides storage information
type StorageProvider interface {
	GetStorage() []models.Storage
	GetCephClusters() []models.CephCluster
}

// DiskHealthProvider provides disk health information from host agents
type DiskHealthProvider interface {
	GetHosts() []models.Host
}

// ControlLevel represents the AI's permission level for infrastructure control
type ControlLevel string

const (
	// ControlLevelReadOnly - AI can only query, no control tools available
	ControlLevelReadOnly ControlLevel = "read_only"
	// ControlLevelSuggest - AI suggests commands, user must copy/paste to execute
	ControlLevelSuggest ControlLevel = "suggest"
	// ControlLevelControlled - AI can execute with per-command approval
	ControlLevelControlled ControlLevel = "controlled"
	// ControlLevelAutonomous - AI executes without approval (requires Pro license)
	ControlLevelAutonomous ControlLevel = "autonomous"
)

// PulseToolExecutor implements ToolExecutor for Pulse-specific tools
type PulseToolExecutor struct {
	stateProvider   StateProvider
	policy          CommandPolicy
	agentServer     AgentServer
	metadataUpdater MetadataUpdater
	findingsManager FindingsManager

	// Patrol context providers
	metricsHistory   MetricsHistoryProvider
	baselineProvider BaselineProvider
	patternProvider  PatternProvider
	alertProvider    AlertProvider
	findingsProvider FindingsProvider

	// Infrastructure context providers
	backupProvider     BackupProvider
	storageProvider    StorageProvider
	diskHealthProvider DiskHealthProvider

	agentProfileManager AgentProfileManager

	// Control settings
	controlLevel    ControlLevel
	protectedGuests []string // VMIDs that AI cannot control

	// Current execution context
	targetType   string
	targetID     string
	isAutonomous bool
}

// NewPulseToolExecutor creates a new Pulse tool executor
func NewPulseToolExecutor(
	stateProvider StateProvider,
	policy CommandPolicy,
	agentServer AgentServer,
) *PulseToolExecutor {
	return &PulseToolExecutor{
		stateProvider: stateProvider,
		policy:        policy,
		agentServer:   agentServer,
	}
}

// SetMetadataUpdater sets the metadata updater
func (e *PulseToolExecutor) SetMetadataUpdater(updater MetadataUpdater) {
	e.metadataUpdater = updater
}

// SetFindingsManager sets the findings manager
func (e *PulseToolExecutor) SetFindingsManager(manager FindingsManager) {
	e.findingsManager = manager
}

// SetMetricsHistory sets the metrics history provider
func (e *PulseToolExecutor) SetMetricsHistory(provider MetricsHistoryProvider) {
	e.metricsHistory = provider
}

// SetBaselineProvider sets the baseline provider
func (e *PulseToolExecutor) SetBaselineProvider(provider BaselineProvider) {
	e.baselineProvider = provider
}

// SetPatternProvider sets the pattern provider
func (e *PulseToolExecutor) SetPatternProvider(provider PatternProvider) {
	e.patternProvider = provider
}

// SetAlertProvider sets the alert provider
func (e *PulseToolExecutor) SetAlertProvider(provider AlertProvider) {
	e.alertProvider = provider
}

// SetFindingsProvider sets the findings provider
func (e *PulseToolExecutor) SetFindingsProvider(provider FindingsProvider) {
	e.findingsProvider = provider
}

// SetBackupProvider sets the backup provider
func (e *PulseToolExecutor) SetBackupProvider(provider BackupProvider) {
	e.backupProvider = provider
}

// SetStorageProvider sets the storage provider
func (e *PulseToolExecutor) SetStorageProvider(provider StorageProvider) {
	e.storageProvider = provider
}

// SetDiskHealthProvider sets the disk health provider
func (e *PulseToolExecutor) SetDiskHealthProvider(provider DiskHealthProvider) {
	e.diskHealthProvider = provider
}

// SetAgentProfileManager sets the manager for centralized agent profiles.
func (e *PulseToolExecutor) SetAgentProfileManager(manager AgentProfileManager) {
	e.agentProfileManager = manager
}

// SetControlLevel sets the AI control permission level
func (e *PulseToolExecutor) SetControlLevel(level ControlLevel) {
	e.controlLevel = level
}

// SetProtectedGuests sets the list of VMIDs that AI cannot control
func (e *PulseToolExecutor) SetProtectedGuests(vmids []string) {
	e.protectedGuests = vmids
}

// SetContext sets the current execution context
func (e *PulseToolExecutor) SetContext(targetType, targetID string, autonomous bool) {
	e.targetType = targetType
	e.targetID = targetID
	e.isAutonomous = autonomous
}

// ListTools returns the list of available tools
func (e *PulseToolExecutor) ListTools() []Tool {
	tools := []Tool{
		{
			Name:        "pulse_get_agent_scope",
			Description: "Get the current unified agent scope (profile assignment and settings).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"agent_id": {
						Type:        "string",
						Description: "Unified agent ID (preferred if known)",
					},
					"hostname": {
						Type:        "string",
						Description: "Hostname or display name to resolve the agent ID",
					},
				},
			},
		},
		{
			Name:        "pulse_fetch_url",
			Description: "Fetch content from a URL. Use to check if web services are responding or read API endpoints.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"url": {
						Type:        "string",
						Description: "The URL to fetch",
					},
				},
				Required: []string{"url"},
			},
		},
		{
			Name:        "pulse_get_infrastructure_state",
			Description: "Get the current state of all monitored infrastructure including VMs, containers, and hosts.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
		{
			Name:        "pulse_set_resource_url",
			Description: "Set the web URL for a resource in Pulse after discovering a web service.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_type": {
						Type:        "string",
						Description: "Type of resource: 'guest', 'docker', or 'host'",
						Enum:        []string{"guest", "docker", "host"},
					},
					"resource_id": {
						Type:        "string",
						Description: "The resource ID from context",
					},
					"url": {
						Type:        "string",
						Description: "The URL to set (empty to remove)",
					},
				},
				Required: []string{"resource_type", "resource_id"},
			},
		},
		{
			Name:        "pulse_resolve_finding",
			Description: "Mark an AI patrol finding as resolved after fixing the issue.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"finding_id": {
						Type:        "string",
						Description: "The finding ID to resolve",
					},
					"resolution_note": {
						Type:        "string",
						Description: "Brief description of how the issue was resolved",
					},
				},
				Required: []string{"finding_id", "resolution_note"},
			},
		},
		{
			Name:        "pulse_dismiss_finding",
			Description: "Dismiss an AI patrol finding as not an issue or expected behavior.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"finding_id": {
						Type:        "string",
						Description: "The finding ID to dismiss",
					},
					"reason": {
						Type:        "string",
						Description: "Why the finding is being dismissed",
						Enum:        []string{"not_an_issue", "expected_behavior", "will_fix_later"},
					},
					"note": {
						Type:        "string",
						Description: "Explanation of why this is being dismissed",
					},
				},
				Required: []string{"finding_id", "reason", "note"},
			},
		},
		// Patrol context tools
		{
			Name:        "pulse_get_metrics_history",
			Description: "Get historical metrics (CPU, memory, disk) for resources over 24 hours or 7 days. Use this to understand trends and detect anomalies.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"period": {
						Type:        "string",
						Description: "Time period: '24h' for last 24 hours, '7d' for last 7 days",
						Enum:        []string{"24h", "7d"},
					},
					"resource_id": {
						Type:        "string",
						Description: "Optional: specific resource ID. If omitted, returns summary for all resources.",
					},
				},
				Required: []string{"period"},
			},
		},
		{
			Name:        "pulse_get_baselines",
			Description: "Get learned baselines for resources. Baselines represent 'normal' behavior and help detect anomalies.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "Optional: specific resource ID. If omitted, returns all baselines.",
					},
				},
			},
		},
		{
			Name:        "pulse_get_patterns",
			Description: "Get detected operational patterns and predictions. Includes recurring spikes, growth trends, and predicted issues.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
		{
			Name:        "pulse_get_active_alerts",
			Description: "Get all currently active alerts. Use to understand current problems in the infrastructure.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
		{
			Name:        "pulse_get_findings",
			Description: "Get AI patrol findings. Active findings are current issues; dismissed findings show user feedback.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"include_dismissed": {
						Type:        "boolean",
						Description: "If true, also include dismissed findings",
					},
				},
			},
		},
		// Infrastructure context tools
		{
			Name:        "pulse_get_backups",
			Description: "Get backup status for VMs and containers. Shows last backup times, backup jobs, and identifies resources without recent backups.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "Optional: filter by specific VM or container ID",
					},
				},
			},
		},
		{
			Name:        "pulse_get_storage",
			Description: "Get storage pool information including usage, ZFS pool health, and Ceph cluster status.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"storage_id": {
						Type:        "string",
						Description: "Optional: specific storage ID for detailed info",
					},
				},
			},
		},
		{
			Name:        "pulse_get_resource_details",
			Description: "Get detailed information about a specific VM, container, or Docker container including IPs, ports, labels, mounts, and network configuration.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_type": {
						Type:        "string",
						Description: "Type of resource: 'vm', 'container', 'docker'",
						Enum:        []string{"vm", "container", "docker"},
					},
					"resource_id": {
						Type:        "string",
						Description: "The resource ID or name to look up",
					},
				},
				Required: []string{"resource_type", "resource_id"},
			},
		},
		{
			Name:        "pulse_get_disk_health",
			Description: "Get disk health information including SMART data, RAID array status, and Ceph cluster health from host agents.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
	}

	// Add control tools if not in read_only mode
	if e.controlLevel != ControlLevelReadOnly && e.controlLevel != "" {
		controlTools := []Tool{
			{
				Name:        "pulse_run_command",
				Description: "Execute a shell command on Pulse-managed infrastructure. By default runs on the current target, set run_on_host=true for host commands.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]PropertySchema{
						"command": {
							Type:        "string",
							Description: "The shell command to execute",
						},
						"run_on_host": {
							Type:        "boolean",
							Description: "If true, run on the host instead of inside the container/VM",
						},
						"target_host": {
							Type:        "string",
							Description: "Optional hostname of the specific host/node to run the command on",
						},
					},
					Required: []string{"command"},
				},
			},
			{
				Name:        "pulse_control_guest",
				Description: "Control Proxmox VMs and LXC containers. Actions: start, stop, shutdown (graceful), restart. Requires an agent on the Proxmox host.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]PropertySchema{
						"guest_id": {
							Type:        "string",
							Description: "The VMID (e.g., '101') or name of the VM/container to control",
						},
						"action": {
							Type:        "string",
							Description: "Action to perform: start, stop, shutdown, restart",
							Enum:        []string{"start", "stop", "shutdown", "restart"},
						},
						"force": {
							Type:        "boolean",
							Description: "If true, force stop without graceful shutdown (use with caution)",
						},
					},
					Required: []string{"guest_id", "action"},
				},
			},
			{
				Name:        "pulse_control_docker",
				Description: "Control Docker containers. Actions: start, stop, restart. Requires an agent on the Docker host.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]PropertySchema{
						"container": {
							Type:        "string",
							Description: "The container name or ID to control",
						},
						"host": {
							Type:        "string",
							Description: "The Docker host name (required if multiple hosts)",
						},
						"action": {
							Type:        "string",
							Description: "Action to perform: start, stop, restart",
							Enum:        []string{"start", "stop", "restart"},
						},
					},
					Required: []string{"container", "action"},
				},
			},
			{
				Name:        "pulse_set_agent_scope",
				Description: "Update a unified agent's scope via safe profile settings. Use this instead of running raw commands to enable/disable modules like Docker, Kubernetes, or Proxmox.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]PropertySchema{
						"agent_id": {
							Type:        "string",
							Description: "Unified agent ID (preferred if known)",
						},
						"hostname": {
							Type:        "string",
							Description: "Hostname or display name to resolve the agent ID",
						},
						"profile_id": {
							Type:        "string",
							Description: "Assign an existing profile ID (optional; omit to use settings)",
						},
						"settings": {
							Type:        "object",
							Description: "Profile settings (e.g., enable_host, enable_docker, enable_kubernetes, enable_proxmox, proxmox_type, docker_runtime, disable_auto_update, disable_docker_update_checks, kube_include_all_pods, kube_include_all_deployments, log_level, interval, report_ip, disable_ceph)",
						},
					},
				},
			},
		}
		tools = append(tools, controlTools...)
	}

	return tools
}

// ExecuteTool executes a tool and returns the result
func (e *PulseToolExecutor) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (CallToolResult, error) {
	log.Debug().
		Str("tool", name).
		Interface("args", args).
		Msg("Executing Pulse tool")

	switch name {
	case "pulse_run_command":
		return e.executeRunCommand(ctx, args)
	case "pulse_fetch_url":
		return e.executeFetchURL(ctx, args)
	case "pulse_get_infrastructure_state":
		return e.executeGetInfrastructureState(ctx)
	case "pulse_get_agent_scope":
		return e.executeGetAgentScope(ctx, args)
	case "pulse_set_resource_url":
		return e.executeSetResourceURL(ctx, args)
	case "pulse_resolve_finding":
		return e.executeResolveFinding(ctx, args)
	case "pulse_dismiss_finding":
		return e.executeDismissFinding(ctx, args)
	case "pulse_get_metrics_history":
		return e.executeGetMetricsHistory(ctx, args)
	case "pulse_get_baselines":
		return e.executeGetBaselines(ctx, args)
	case "pulse_get_patterns":
		return e.executeGetPatterns(ctx, args)
	case "pulse_get_active_alerts":
		return e.executeGetActiveAlerts(ctx, args)
	case "pulse_get_findings":
		return e.executeGetFindings(ctx, args)
	case "pulse_get_backups":
		return e.executeGetBackups(ctx, args)
	case "pulse_get_storage":
		return e.executeGetStorage(ctx, args)
	case "pulse_get_resource_details":
		return e.executeGetResourceDetails(ctx, args)
	case "pulse_get_disk_health":
		return e.executeGetDiskHealth(ctx, args)
	case "pulse_control_guest":
		return e.executeControlGuest(ctx, args)
	case "pulse_control_docker":
		return e.executeControlDocker(ctx, args)
	case "pulse_set_agent_scope":
		return e.executeSetAgentScope(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown tool: %s", name)), nil
	}
}

func (e *PulseToolExecutor) executeRunCommand(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	command, _ := args["command"].(string)
	runOnHost, _ := args["run_on_host"].(bool)
	targetHost, _ := args["target_host"].(string)

	if command == "" {
		return NewErrorResult(fmt.Errorf("command is required")), nil
	}

	if e.controlLevel == ControlLevelReadOnly || e.controlLevel == "" {
		return NewTextResult("Control tools are disabled. Enable them in Settings > AI > Control Level."), nil
	}

	// Check security policy
	decision := agentexec.PolicyAllow
	if e.policy != nil {
		decision = e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This command is blocked by security policy")), nil
		}
	}

	if e.controlLevel == ControlLevelSuggest {
		return NewTextResult(formatCommandSuggestion(command, runOnHost, targetHost)), nil
	}

	if e.controlLevel == ControlLevelControlled {
		return NewTextResult(formatApprovalNeeded(command, "Control level requires approval")), nil
	}
	if decision == agentexec.PolicyRequireApproval && !e.isAutonomous {
		return NewTextResult(formatApprovalNeeded(command, "Security policy requires approval")), nil
	}

	// Execute via agent server
	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	// Find the appropriate agent
	agentID := e.findAgentForCommand(runOnHost, targetHost)
	if agentID == "" {
		return NewErrorResult(fmt.Errorf("no agent available for target")), nil
	}

	// Determine target type based on runOnHost
	targetType := "container"
	if runOnHost {
		targetType = "host"
	}

	// Execute command
	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: targetType,
		TargetID:   e.targetID,
	})
	if err != nil {
		return NewErrorResult(err), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}
	if result.ExitCode != 0 {
		output = fmt.Sprintf("Exit code %d:\n%s", result.ExitCode, output)
	}

	return NewTextResult(output), nil
}

func (e *PulseToolExecutor) executeSetAgentScope(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.agentProfileManager == nil {
		return NewTextResult("Agent scope management is not available."), nil
	}
	if e.controlLevel == ControlLevelReadOnly || e.controlLevel == "" {
		return NewTextResult("Agent scope tools are disabled. Enable them in Settings > AI > Control Level."), nil
	}

	agentID, _ := args["agent_id"].(string)
	hostname, _ := args["hostname"].(string)
	profileID, _ := args["profile_id"].(string)

	agentID = strings.TrimSpace(agentID)
	hostname = strings.TrimSpace(hostname)
	profileID = strings.TrimSpace(profileID)

	settings := map[string]interface{}{}
	if rawSettings, ok := args["settings"].(map[string]interface{}); ok {
		for key, value := range rawSettings {
			if value != nil {
				settings[key] = value
			}
		}
	}

	if agentID == "" && hostname == "" {
		return NewErrorResult(fmt.Errorf("agent_id or hostname is required")), nil
	}

	agentLabel := agentID
	if agentID == "" {
		if e.stateProvider == nil {
			return NewErrorResult(fmt.Errorf("state provider not available to resolve hostname")), nil
		}
		resolvedID, resolvedLabel := resolveAgentFromHostname(e.stateProvider.GetState(), hostname)
		if resolvedID == "" {
			return NewTextResult(fmt.Sprintf("No agent found for hostname '%s'.", hostname)), nil
		}
		agentID = resolvedID
		agentLabel = resolvedLabel
	} else if e.stateProvider != nil {
		if resolvedLabel := resolveAgentLabel(e.stateProvider.GetState(), agentID); resolvedLabel != "" {
			agentLabel = resolvedLabel
		}
	}

	if profileID != "" && len(settings) > 0 {
		return NewErrorResult(fmt.Errorf("use either profile_id or settings, not both")), nil
	}

	if e.controlLevel == ControlLevelSuggest {
		if profileID != "" {
			return NewTextResult(fmt.Sprintf("Suggestion: assign profile %s to agent %s.", profileID, agentLabel)), nil
		}
		if len(settings) == 0 {
			return NewErrorResult(fmt.Errorf("settings are required when profile_id is not provided")), nil
		}
		return NewTextResult(fmt.Sprintf("Suggestion: apply agent scope to %s with settings: %s", agentLabel, formatSettingsSummary(settings))), nil
	}

	if profileID != "" {
		profileName, err := e.agentProfileManager.AssignProfile(ctx, agentID, profileID)
		if err != nil {
			return NewErrorResult(err), nil
		}
		return NewTextResult(fmt.Sprintf("Assigned profile '%s' (%s) to agent %s. Restart the agent to apply changes.", profileName, profileID, agentLabel)), nil
	}

	if len(settings) == 0 {
		return NewErrorResult(fmt.Errorf("settings are required when profile_id is not provided")), nil
	}

	profileID, profileName, created, err := e.agentProfileManager.ApplyAgentScope(ctx, agentID, agentLabel, settings)
	if err != nil {
		return NewErrorResult(err), nil
	}

	action := "Updated"
	if created {
		action = "Created"
	}
	return NewTextResult(fmt.Sprintf("%s profile '%s' (%s) and assigned to agent %s. Restart the agent to apply changes. Settings: %s", action, profileName, profileID, agentLabel, formatSettingsSummary(settings))), nil
}

func (e *PulseToolExecutor) executeGetAgentScope(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	agentID, _ := args["agent_id"].(string)
	hostname, _ := args["hostname"].(string)
	agentID = strings.TrimSpace(agentID)
	hostname = strings.TrimSpace(hostname)

	if agentID == "" && hostname == "" {
		return NewErrorResult(fmt.Errorf("agent_id or hostname is required")), nil
	}

	agentLabel := agentID
	if agentID == "" {
		if e.stateProvider == nil {
			return NewErrorResult(fmt.Errorf("state provider not available to resolve hostname")), nil
		}
		resolvedID, resolvedLabel := resolveAgentFromHostname(e.stateProvider.GetState(), hostname)
		if resolvedID == "" {
			return NewTextResult(fmt.Sprintf("No agent found for hostname '%s'.", hostname)), nil
		}
		agentID = resolvedID
		agentLabel = resolvedLabel
	} else if e.stateProvider != nil {
		if resolvedLabel := resolveAgentLabel(e.stateProvider.GetState(), agentID); resolvedLabel != "" {
			agentLabel = resolvedLabel
		}
	}

	var scope *AgentScope
	if e.agentProfileManager != nil {
		var err error
		scope, err = e.agentProfileManager.GetAgentScope(ctx, agentID)
		if err != nil {
			return NewTextResult(fmt.Sprintf("Failed to load agent scope for %s: %v", agentLabel, err)), nil
		}
	}

	var observed []string
	var commandsEnabled *bool
	if e.stateProvider != nil {
		observed, commandsEnabled = detectAgentModules(e.stateProvider.GetState(), agentID)
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Agent: %s\n", agentLabel))
	summary.WriteString(fmt.Sprintf("Agent ID: %s\n", agentID))

	if scope == nil {
		summary.WriteString("Assigned profile: none\n")
	} else {
		summary.WriteString(fmt.Sprintf("Assigned profile: %s (%s)\n", scope.ProfileName, scope.ProfileID))
		if scope.ProfileVersion > 0 {
			summary.WriteString(fmt.Sprintf("Profile version: %d\n", scope.ProfileVersion))
		}
	}

	if len(observed) > 0 {
		summary.WriteString(fmt.Sprintf("Observed modules: %s\n", strings.Join(observed, ", ")))
	}
	if commandsEnabled != nil {
		if *commandsEnabled {
			summary.WriteString("AI commands: enabled\n")
		} else {
			summary.WriteString("AI commands: disabled\n")
		}
	}

	if scope != nil && len(scope.Settings) > 0 {
		summary.WriteString("Profile settings:\n")
		for _, line := range formatSettingsLines(scope.Settings) {
			summary.WriteString(line)
		}
	} else {
		summary.WriteString("Profile settings: none\n")
	}

	summary.WriteString("Note: profile changes apply after the agent restarts.")

	return NewTextResult(summary.String()), nil
}

func resolveAgentFromHostname(state models.StateSnapshot, hostname string) (string, string) {
	needle := strings.TrimSpace(hostname)
	if needle == "" {
		return "", ""
	}
	for _, host := range state.Hosts {
		if strings.EqualFold(host.Hostname, needle) || strings.EqualFold(host.DisplayName, needle) || strings.EqualFold(host.ID, needle) {
			label := firstNonEmpty(host.DisplayName, host.Hostname, host.ID)
			return host.ID, label
		}
	}
	for _, host := range state.DockerHosts {
		if strings.EqualFold(host.Hostname, needle) || strings.EqualFold(host.DisplayName, needle) || strings.EqualFold(host.CustomDisplayName, needle) || strings.EqualFold(host.ID, needle) {
			label := firstNonEmpty(host.CustomDisplayName, host.DisplayName, host.Hostname, host.ID)
			agentID := strings.TrimSpace(host.AgentID)
			if agentID == "" {
				agentID = host.ID
			}
			return agentID, label
		}
	}
	return "", ""
}

func resolveAgentLabel(state models.StateSnapshot, agentID string) string {
	needle := strings.TrimSpace(agentID)
	if needle == "" {
		return ""
	}
	for _, host := range state.Hosts {
		if strings.EqualFold(host.ID, needle) {
			return firstNonEmpty(host.DisplayName, host.Hostname, host.ID)
		}
	}
	for _, host := range state.DockerHosts {
		if strings.EqualFold(host.AgentID, needle) || strings.EqualFold(host.ID, needle) {
			return firstNonEmpty(host.CustomDisplayName, host.DisplayName, host.Hostname, host.ID)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func formatSettingsSummary(settings map[string]interface{}) string {
	if len(settings) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, settings[key]))
	}
	return strings.Join(parts, ", ")
}

func formatSettingsLines(settings map[string]interface{}) []string {
	if len(settings) == 0 {
		return []string{"  - none\n"}
	}
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("  - %s: %v\n", key, settings[key]))
	}
	return lines
}

func detectAgentModules(state models.StateSnapshot, agentID string) ([]string, *bool) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, nil
	}

	var modules []string
	var commandsEnabled *bool

	for _, host := range state.Hosts {
		if strings.EqualFold(host.ID, agentID) {
			modules = append(modules, "host")
			val := host.CommandsEnabled
			commandsEnabled = &val
			if host.LinkedNodeID != "" {
				modules = append(modules, "proxmox")
			}
			break
		}
	}

	for _, dockerHost := range state.DockerHosts {
		if strings.EqualFold(dockerHost.AgentID, agentID) || strings.EqualFold(dockerHost.ID, agentID) {
			modules = append(modules, "docker")
			break
		}
	}

	for _, cluster := range state.KubernetesClusters {
		if strings.EqualFold(cluster.AgentID, agentID) {
			modules = append(modules, "kubernetes")
			break
		}
	}

	if len(modules) == 0 {
		return nil, commandsEnabled
	}

	sort.Strings(modules)
	return modules, commandsEnabled
}

func (e *PulseToolExecutor) executeFetchURL(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return NewErrorResult(fmt.Errorf("url is required")), nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Error fetching URL: %v", err)), nil
	}
	defer resp.Body.Close()

	// Limit response size
	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024)) // 50KB limit
	if err != nil {
		return NewTextResult(fmt.Sprintf("Error reading response: %v", err)), nil
	}

	result := fmt.Sprintf("Status: %d\nHeaders: %v\n\nBody:\n%s",
		resp.StatusCode,
		resp.Header,
		string(body))

	return NewTextResult(result), nil
}

func (e *PulseToolExecutor) executeGetInfrastructureState(ctx context.Context) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewErrorResult(fmt.Errorf("state provider not available")), nil
	}

	state := e.stateProvider.GetState()

	// Build a summary of the infrastructure
	var summary strings.Builder
	summary.WriteString("# Infrastructure State\n\n")

	// Proxmox nodes
	if len(state.Nodes) > 0 {
		summary.WriteString("## Proxmox Nodes\n")
		for _, node := range state.Nodes {
			summary.WriteString(fmt.Sprintf("- %s (Status: %s)\n", node.Name, node.Status))
		}
		summary.WriteString("\n")
	}

	// VMs
	if len(state.VMs) > 0 {
		summary.WriteString(fmt.Sprintf("## VMs (%d total)\n", len(state.VMs)))
		for _, vm := range state.VMs {
			summary.WriteString(fmt.Sprintf("- %s (VMID: %d, Status: %s)\n", vm.Name, vm.VMID, vm.Status))
		}
		summary.WriteString("\n")
	}

	// Containers
	if len(state.Containers) > 0 {
		summary.WriteString(fmt.Sprintf("## LXC Containers (%d total)\n", len(state.Containers)))
		for _, ct := range state.Containers {
			summary.WriteString(fmt.Sprintf("- %s (VMID: %d, Status: %s)\n", ct.Name, ct.VMID, ct.Status))
		}
		summary.WriteString("\n")
	}

	// Docker hosts
	if len(state.DockerHosts) > 0 {
		summary.WriteString(fmt.Sprintf("## Docker Hosts (%d total)\n", len(state.DockerHosts)))
		for _, host := range state.DockerHosts {
			name := host.DisplayName
			if name == "" {
				name = host.Hostname
			}
			summary.WriteString(fmt.Sprintf("- %s (%d containers)\n", name, len(host.Containers)))
		}
		summary.WriteString("\n")
	}

	return NewTextResult(summary.String()), nil
}

func (e *PulseToolExecutor) executeSetResourceURL(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)
	url, _ := args["url"].(string)

	if e.metadataUpdater == nil {
		return NewErrorResult(fmt.Errorf("metadata updater not available")), nil
	}

	if err := e.metadataUpdater.SetResourceURL(resourceType, resourceID, url); err != nil {
		return NewErrorResult(err), nil
	}

	if url == "" {
		return NewTextResult(fmt.Sprintf("Cleared URL for %s %s", resourceType, resourceID)), nil
	}
	return NewTextResult(fmt.Sprintf("Set URL for %s %s to %s", resourceType, resourceID, url)), nil
}

func (e *PulseToolExecutor) executeResolveFinding(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	findingID, _ := args["finding_id"].(string)
	resolutionNote, _ := args["resolution_note"].(string)

	if e.findingsManager == nil {
		return NewErrorResult(fmt.Errorf("findings manager not available")), nil
	}

	if err := e.findingsManager.ResolveFinding(findingID, resolutionNote); err != nil {
		return NewErrorResult(err), nil
	}

	return NewTextResult(fmt.Sprintf("Finding %s resolved: %s", findingID, resolutionNote)), nil
}

func (e *PulseToolExecutor) executeDismissFinding(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	findingID, _ := args["finding_id"].(string)
	reason, _ := args["reason"].(string)
	note, _ := args["note"].(string)

	if e.findingsManager == nil {
		return NewErrorResult(fmt.Errorf("findings manager not available")), nil
	}

	if err := e.findingsManager.DismissFinding(findingID, reason, note); err != nil {
		return NewErrorResult(err), nil
	}

	return NewTextResult(fmt.Sprintf("Finding %s dismissed (%s): %s", findingID, reason, note)), nil
}

func (e *PulseToolExecutor) findAgentForCommand(runOnHost bool, targetHost string) string {
	if e.agentServer == nil {
		return ""
	}

	agents := e.agentServer.GetConnectedAgents()
	if len(agents) == 0 {
		return ""
	}

	// If target host is specified, find that agent
	if targetHost != "" {
		for _, agent := range agents {
			if agent.Hostname == targetHost || agent.AgentID == targetHost {
				return agent.AgentID
			}
		}
	}

	// Otherwise return first connected agent
	return agents[0].AgentID
}

// formatApprovalNeeded formats an approval-needed response
func formatApprovalNeeded(command, reason string) string {
	payload := map[string]interface{}{
		"type":           "approval_required",
		"command":        command,
		"reason":         reason,
		"how_to_approve": "Ask the user to click the approval button shown in the UI.",
		"do_not_retry":   true,
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}

// formatPolicyBlocked formats a policy-blocked response
func formatPolicyBlocked(command, reason string) string {
	payload := map[string]interface{}{
		"type":         "policy_blocked",
		"command":      command,
		"reason":       reason,
		"do_not_retry": true,
	}
	b, _ := json.Marshal(payload)
	return "POLICY_BLOCKED: " + string(b)
}

func formatCommandSuggestion(command string, runOnHost bool, targetHost string) string {
	target := "current target"
	if runOnHost {
		target = "host"
	}
	if strings.TrimSpace(targetHost) != "" {
		target = fmt.Sprintf("host %s", targetHost)
	}
	return fmt.Sprintf("Suggested command for %s:\n%s", target, command)
}

// Patrol context tool implementations

func (e *PulseToolExecutor) executeGetMetricsHistory(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	period, _ := args["period"].(string)
	resourceID, _ := args["resource_id"].(string)

	if e.metricsHistory == nil {
		return NewTextResult("Metrics history not available. The system may still be collecting data."), nil
	}

	var duration time.Duration
	switch period {
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	default:
		duration = 24 * time.Hour
	}

	if resourceID != "" {
		// Get metrics for specific resource
		metrics, err := e.metricsHistory.GetResourceMetrics(resourceID, duration)
		if err != nil {
			return NewTextResult(fmt.Sprintf("Error getting metrics: %v", err)), nil
		}
		if len(metrics) == 0 {
			return NewTextResult(fmt.Sprintf("No metrics found for resource %s in last %s", resourceID, period)), nil
		}

		// Format as text
		var result strings.Builder
		result.WriteString(fmt.Sprintf("# Metrics for %s (last %s)\n\n", resourceID, period))
		for _, m := range metrics {
			result.WriteString(fmt.Sprintf("- %s: CPU=%.1f%%, Mem=%.1f%%, Disk=%.1f%%\n",
				m.Timestamp.Format("2006-01-02 15:04"), m.CPU, m.Memory, m.Disk))
		}
		return NewTextResult(result.String()), nil
	}

	// Get summary for all resources
	summary, err := e.metricsHistory.GetAllMetricsSummary(duration)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Error getting metrics summary: %v", err)), nil
	}
	if len(summary) == 0 {
		return NewTextResult("No metrics data available yet. The system is still collecting data."), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("# Metrics Summary (last %s)\n\n", period))
	for _, s := range summary {
		result.WriteString(fmt.Sprintf("## %s (%s)\n", s.ResourceName, s.ResourceType))
		result.WriteString(fmt.Sprintf("- CPU: avg=%.1f%%, max=%.1f%%\n", s.AvgCPU, s.MaxCPU))
		result.WriteString(fmt.Sprintf("- Memory: avg=%.1f%%, max=%.1f%%\n", s.AvgMemory, s.MaxMemory))
		if s.AvgDisk > 0 {
			result.WriteString(fmt.Sprintf("- Disk: avg=%.1f%%, max=%.1f%%\n", s.AvgDisk, s.MaxDisk))
		}
		result.WriteString(fmt.Sprintf("- Trend: %s\n\n", s.Trend))
	}
	return NewTextResult(result.String()), nil
}

func (e *PulseToolExecutor) executeGetBaselines(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)

	if e.baselineProvider == nil {
		return NewTextResult("Baseline data not available. The system needs time to learn normal behavior patterns."), nil
	}

	if resourceID != "" {
		// Get baseline for specific resource
		cpuBaseline := e.baselineProvider.GetBaseline(resourceID, "cpu")
		memBaseline := e.baselineProvider.GetBaseline(resourceID, "memory")

		if cpuBaseline == nil && memBaseline == nil {
			return NewTextResult(fmt.Sprintf("No baseline data for resource %s yet.", resourceID)), nil
		}

		var result strings.Builder
		result.WriteString(fmt.Sprintf("# Baselines for %s\n\n", resourceID))
		if cpuBaseline != nil {
			result.WriteString(fmt.Sprintf("CPU: mean=%.1f%%, stddev=%.1f%%, range=[%.1f%%, %.1f%%]\n",
				cpuBaseline.Mean, cpuBaseline.StdDev, cpuBaseline.Min, cpuBaseline.Max))
		}
		if memBaseline != nil {
			result.WriteString(fmt.Sprintf("Memory: mean=%.1f%%, stddev=%.1f%%, range=[%.1f%%, %.1f%%]\n",
				memBaseline.Mean, memBaseline.StdDev, memBaseline.Min, memBaseline.Max))
		}
		return NewTextResult(result.String()), nil
	}

	// Get all baselines
	allBaselines := e.baselineProvider.GetAllBaselines()
	if len(allBaselines) == 0 {
		return NewTextResult("No baseline data available yet. The system needs time to learn normal behavior patterns."), nil
	}

	var result strings.Builder
	result.WriteString("# Learned Baselines\n\n")
	for resID, metrics := range allBaselines {
		result.WriteString(fmt.Sprintf("## %s\n", resID))
		for metric, baseline := range metrics {
			result.WriteString(fmt.Sprintf("- %s: mean=%.1f%%, stddev=%.1f%%\n",
				metric, baseline.Mean, baseline.StdDev))
		}
		result.WriteString("\n")
	}
	return NewTextResult(result.String()), nil
}

func (e *PulseToolExecutor) executeGetPatterns(_ context.Context, _ map[string]interface{}) (CallToolResult, error) {
	if e.patternProvider == nil {
		return NewTextResult("Pattern detection not available. The system needs more historical data."), nil
	}

	patterns := e.patternProvider.GetPatterns()
	predictions := e.patternProvider.GetPredictions()

	if len(patterns) == 0 && len(predictions) == 0 {
		return NewTextResult("No patterns or predictions detected yet. The system is still analyzing historical data."), nil
	}

	var result strings.Builder
	result.WriteString("# Patterns and Predictions\n\n")

	if len(patterns) > 0 {
		result.WriteString("## Detected Patterns\n")
		for _, p := range patterns {
			result.WriteString(fmt.Sprintf("- %s (%s): %s (confidence: %.0f%%)\n",
				p.ResourceName, p.PatternType, p.Description, p.Confidence*100))
		}
		result.WriteString("\n")
	}

	if len(predictions) > 0 {
		result.WriteString("## Predictions\n")
		for _, p := range predictions {
			result.WriteString(fmt.Sprintf("- %s: %s predicted by %s (confidence: %.0f%%)\n",
				p.ResourceName, p.IssueType, p.PredictedTime.Format("2006-01-02"), p.Confidence*100))
			if p.Recommendation != "" {
				result.WriteString(fmt.Sprintf("  Recommendation: %s\n", p.Recommendation))
			}
		}
	}

	return NewTextResult(result.String()), nil
}

func (e *PulseToolExecutor) executeGetActiveAlerts(_ context.Context, _ map[string]interface{}) (CallToolResult, error) {
	if e.alertProvider == nil {
		return NewTextResult("Alert data not available."), nil
	}

	alerts := e.alertProvider.GetActiveAlerts()
	if len(alerts) == 0 {
		return NewTextResult("No active alerts. All resources are operating within configured thresholds."), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("# Active Alerts (%d)\n\n", len(alerts)))
	for _, a := range alerts {
		result.WriteString(fmt.Sprintf("## %s - %s\n", a.ResourceName, a.Type))
		result.WriteString(fmt.Sprintf("- Severity: %s\n", a.Severity))
		result.WriteString(fmt.Sprintf("- Current: %.1f%% (threshold: %.1f%%)\n", a.Value, a.Threshold))
		result.WriteString(fmt.Sprintf("- Started: %s\n", a.StartTime.Format("2006-01-02 15:04")))
		result.WriteString(fmt.Sprintf("- Message: %s\n\n", a.Message))
	}

	return NewTextResult(result.String()), nil
}

func (e *PulseToolExecutor) executeGetFindings(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	includeDismissed, _ := args["include_dismissed"].(bool)

	if e.findingsProvider == nil {
		return NewTextResult("Patrol findings not available. AI Patrol may not be running."), nil
	}

	activeFindings := e.findingsProvider.GetActiveFindings()
	var dismissedFindings []Finding
	if includeDismissed {
		dismissedFindings = e.findingsProvider.GetDismissedFindings()
	}

	if len(activeFindings) == 0 && len(dismissedFindings) == 0 {
		return NewTextResult("No patrol findings. Either no issues detected or AI Patrol hasn't run yet."), nil
	}

	var result strings.Builder

	if len(activeFindings) > 0 {
		result.WriteString(fmt.Sprintf("# Active Findings (%d)\n\n", len(activeFindings)))
		for _, f := range activeFindings {
			result.WriteString(fmt.Sprintf("## [%s] %s - %s\n", f.Severity, f.ResourceName, f.Title))
			result.WriteString(fmt.Sprintf("- Category: %s\n", f.Category))
			result.WriteString(fmt.Sprintf("- ID: %s\n", f.ID))
			result.WriteString(fmt.Sprintf("- Description: %s\n", f.Description))
			result.WriteString(fmt.Sprintf("- Recommendation: %s\n", f.Recommendation))
			result.WriteString(fmt.Sprintf("- Evidence: %s\n", f.Evidence))
			result.WriteString(fmt.Sprintf("- First detected: %s, seen %d times\n\n",
				f.DetectedAt.Format("2006-01-02 15:04"), f.TimesRaised))
		}
	}

	if len(dismissedFindings) > 0 {
		result.WriteString(fmt.Sprintf("# Dismissed Findings (%d)\n\n", len(dismissedFindings)))
		result.WriteString("These findings were dismissed by users - do NOT re-raise them:\n\n")
		for _, f := range dismissedFindings {
			result.WriteString(fmt.Sprintf("- [%s] %s: %s (key: %s)\n", f.Severity, f.ResourceName, f.Title, f.Key))
		}
	}

	return NewTextResult(result.String()), nil
}

// Infrastructure context tool implementations

func (e *PulseToolExecutor) executeGetBackups(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)

	if e.backupProvider == nil {
		return NewTextResult("Backup information not available."), nil
	}

	backups := e.backupProvider.GetBackups()
	pbsInstances := e.backupProvider.GetPBSInstances()

	var result strings.Builder
	result.WriteString("# Backup Status\n\n")

	// PBS Backups
	if len(backups.PBS) > 0 {
		result.WriteString(fmt.Sprintf("## PBS Backups (%d)\n", len(backups.PBS)))
		for _, b := range backups.PBS {
			if resourceID != "" && b.VMID != resourceID {
				continue
			}
			result.WriteString(fmt.Sprintf("- %s %s: %s on %s/%s (%.1f GB)\n",
				b.BackupType, b.VMID,
				b.BackupTime.Format("2006-01-02 15:04"),
				b.Instance, b.Datastore,
				float64(b.Size)/(1024*1024*1024)))
			if b.Verified {
				result.WriteString("  ✓ Verified\n")
			}
			if b.Protected {
				result.WriteString("  🔒 Protected\n")
			}
		}
		result.WriteString("\n")
	}

	// PVE Backups
	if len(backups.PVE.StorageBackups) > 0 {
		result.WriteString(fmt.Sprintf("## PVE Storage Backups (%d)\n", len(backups.PVE.StorageBackups)))
		for _, b := range backups.PVE.StorageBackups {
			if resourceID != "" && fmt.Sprintf("%d", b.VMID) != resourceID {
				continue
			}
			result.WriteString(fmt.Sprintf("- VMID %d: %s (%.1f GB)\n",
				b.VMID,
				b.Time.Format("2006-01-02 15:04"),
				float64(b.Size)/(1024*1024*1024)))
		}
		result.WriteString("\n")
	}

	// PBS Instances summary
	if len(pbsInstances) > 0 {
		result.WriteString("## PBS Servers\n")
		for _, pbs := range pbsInstances {
			result.WriteString(fmt.Sprintf("- %s (%s): %s\n", pbs.Name, pbs.Host, pbs.Status))
			for _, ds := range pbs.Datastores {
				result.WriteString(fmt.Sprintf("  - %s: %.1f%% used (%.1f GB free)\n",
					ds.Name, ds.Usage*100, float64(ds.Free)/(1024*1024*1024)))
			}
		}
		result.WriteString("\n")
	}

	// Backup jobs
	if len(backups.PVE.BackupTasks) > 0 {
		result.WriteString("## Recent Backup Tasks\n")
		for _, t := range backups.PVE.BackupTasks {
			status := "✓"
			if t.Status != "ok" {
				status = "✗ " + t.Status
			}
			result.WriteString(fmt.Sprintf("- %s: VMID %d on %s (%s)\n",
				status, t.VMID, t.Node, t.StartTime.Format("2006-01-02 15:04")))
		}
	}

	if result.Len() == len("# Backup Status\n\n") {
		return NewTextResult("No backup data available. PBS/PVE backup monitoring may not be configured."), nil
	}

	return NewTextResult(result.String()), nil
}

func (e *PulseToolExecutor) executeGetStorage(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	storageID, _ := args["storage_id"].(string)

	if e.storageProvider == nil {
		return NewTextResult("Storage information not available."), nil
	}

	storage := e.storageProvider.GetStorage()
	cephClusters := e.storageProvider.GetCephClusters()

	var result strings.Builder
	result.WriteString("# Storage Status\n\n")

	// Storage pools
	if len(storage) > 0 {
		result.WriteString("## Storage Pools\n")
		for _, s := range storage {
			if storageID != "" && s.ID != storageID && s.Name != storageID {
				continue
			}
			statusIcon := "✓"
			if s.Status != "available" {
				statusIcon = "⚠"
			}
			result.WriteString(fmt.Sprintf("### %s %s (%s)\n", statusIcon, s.Name, s.Type))
			result.WriteString(fmt.Sprintf("- Usage: %.1f%% (%.1f GB / %.1f GB)\n",
				s.Usage*100,
				float64(s.Used)/(1024*1024*1024),
				float64(s.Total)/(1024*1024*1024)))
			result.WriteString(fmt.Sprintf("- Free: %.1f GB\n", float64(s.Free)/(1024*1024*1024)))
			result.WriteString(fmt.Sprintf("- Content: %s\n", s.Content))
			if s.Shared {
				result.WriteString("- Shared: Yes\n")
			}

			// ZFS pool details
			if s.ZFSPool != nil {
				zfs := s.ZFSPool
				result.WriteString(fmt.Sprintf("- ZFS Pool: %s (State: %s)\n", zfs.Name, zfs.State))
				if zfs.ReadErrors > 0 || zfs.WriteErrors > 0 || zfs.ChecksumErrors > 0 {
					result.WriteString(fmt.Sprintf("  ⚠ Errors: read=%d, write=%d, checksum=%d\n",
						zfs.ReadErrors, zfs.WriteErrors, zfs.ChecksumErrors))
				}
				if zfs.Scan != "" {
					result.WriteString(fmt.Sprintf("  - Scan: %s\n", zfs.Scan))
				}
			}
			result.WriteString("\n")
		}
	}

	// Ceph clusters
	if len(cephClusters) > 0 {
		result.WriteString("## Ceph Clusters\n")
		for _, c := range cephClusters {
			healthIcon := "✓"
			if c.Health != "HEALTH_OK" {
				healthIcon = "⚠"
			}
			result.WriteString(fmt.Sprintf("### %s %s\n", healthIcon, c.Name))
			result.WriteString(fmt.Sprintf("- Health: %s", c.Health))
			if c.HealthMessage != "" {
				result.WriteString(fmt.Sprintf(" - %s", c.HealthMessage))
			}
			result.WriteString("\n")
			result.WriteString(fmt.Sprintf("- Usage: %.1f%% (%.1f TB / %.1f TB)\n",
				c.UsagePercent,
				float64(c.UsedBytes)/(1024*1024*1024*1024),
				float64(c.TotalBytes)/(1024*1024*1024*1024)))
			result.WriteString(fmt.Sprintf("- OSDs: %d up, %d in (of %d total)\n",
				c.NumOSDsUp, c.NumOSDsIn, c.NumOSDs))
			result.WriteString(fmt.Sprintf("- Monitors: %d, Managers: %d\n", c.NumMons, c.NumMgrs))
			result.WriteString("\n")
		}
	}

	if result.Len() == len("# Storage Status\n\n") {
		return NewTextResult("No storage data available."), nil
	}

	return NewTextResult(result.String()), nil
}

func (e *PulseToolExecutor) executeGetResourceDetails(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)

	if e.stateProvider == nil {
		return NewTextResult("State information not available."), nil
	}

	state := e.stateProvider.GetState()
	var result strings.Builder

	switch resourceType {
	case "vm":
		for _, vm := range state.VMs {
			if fmt.Sprintf("%d", vm.VMID) == resourceID || vm.Name == resourceID || vm.ID == resourceID {
				result.WriteString(fmt.Sprintf("# VM: %s (VMID %d)\n\n", vm.Name, vm.VMID))
				result.WriteString(fmt.Sprintf("- Status: %s\n", vm.Status))
				result.WriteString(fmt.Sprintf("- Node: %s\n", vm.Node))
				result.WriteString(fmt.Sprintf("- CPU: %.1f%% (%d cores)\n", vm.CPU*100, vm.CPUs))
				result.WriteString(fmt.Sprintf("- Memory: %.1f%% (%.1f GB / %.1f GB)\n",
					vm.Memory.Usage*100,
					float64(vm.Memory.Used)/(1024*1024*1024),
					float64(vm.Memory.Total)/(1024*1024*1024)))
				if vm.OSName != "" {
					result.WriteString(fmt.Sprintf("- OS: %s\n", vm.OSName))
				}
				if len(vm.Tags) > 0 {
					result.WriteString(fmt.Sprintf("- Tags: %s\n", strings.Join(vm.Tags, ", ")))
				}
				if len(vm.NetworkInterfaces) > 0 {
					result.WriteString("- Network Interfaces:\n")
					for _, nic := range vm.NetworkInterfaces {
						result.WriteString(fmt.Sprintf("  - %s: %s\n", nic.Name, strings.Join(nic.Addresses, ", ")))
					}
				}
				if !vm.LastBackup.IsZero() {
					result.WriteString(fmt.Sprintf("- Last Backup: %s\n", vm.LastBackup.Format("2006-01-02 15:04")))
				}
				return NewTextResult(result.String()), nil
			}
		}
		return NewTextResult(fmt.Sprintf("VM '%s' not found.", resourceID)), nil

	case "container":
		for _, ct := range state.Containers {
			if fmt.Sprintf("%d", ct.VMID) == resourceID || ct.Name == resourceID || ct.ID == resourceID {
				result.WriteString(fmt.Sprintf("# Container: %s (VMID %d)\n\n", ct.Name, ct.VMID))
				result.WriteString(fmt.Sprintf("- Status: %s\n", ct.Status))
				result.WriteString(fmt.Sprintf("- Node: %s\n", ct.Node))
				result.WriteString(fmt.Sprintf("- CPU: %.1f%% (%d cores)\n", ct.CPU*100, ct.CPUs))
				result.WriteString(fmt.Sprintf("- Memory: %.1f%% (%.1f GB / %.1f GB)\n",
					ct.Memory.Usage*100,
					float64(ct.Memory.Used)/(1024*1024*1024),
					float64(ct.Memory.Total)/(1024*1024*1024)))
				if ct.OSName != "" {
					result.WriteString(fmt.Sprintf("- OS: %s\n", ct.OSName))
				}
				if len(ct.Tags) > 0 {
					result.WriteString(fmt.Sprintf("- Tags: %s\n", strings.Join(ct.Tags, ", ")))
				}
				if len(ct.NetworkInterfaces) > 0 {
					result.WriteString("- Network Interfaces:\n")
					for _, nic := range ct.NetworkInterfaces {
						result.WriteString(fmt.Sprintf("  - %s: %s\n", nic.Name, strings.Join(nic.Addresses, ", ")))
					}
				}
				if !ct.LastBackup.IsZero() {
					result.WriteString(fmt.Sprintf("- Last Backup: %s\n", ct.LastBackup.Format("2006-01-02 15:04")))
				}
				return NewTextResult(result.String()), nil
			}
		}
		return NewTextResult(fmt.Sprintf("Container '%s' not found.", resourceID)), nil

	case "docker":
		for _, host := range state.DockerHosts {
			for _, c := range host.Containers {
				if c.ID == resourceID || c.Name == resourceID || strings.HasPrefix(c.ID, resourceID) {
					result.WriteString(fmt.Sprintf("# Docker Container: %s\n\n", c.Name))
					result.WriteString(fmt.Sprintf("- Host: %s\n", host.Hostname))
					result.WriteString(fmt.Sprintf("- Image: %s\n", c.Image))
					result.WriteString(fmt.Sprintf("- State: %s (%s)\n", c.State, c.Status))
					if c.Health != "" {
						result.WriteString(fmt.Sprintf("- Health: %s\n", c.Health))
					}
					result.WriteString(fmt.Sprintf("- CPU: %.1f%%\n", c.CPUPercent))
					result.WriteString(fmt.Sprintf("- Memory: %.1f%% (%.1f MB / %.1f MB)\n",
						c.MemoryPercent,
						float64(c.MemoryUsage)/(1024*1024),
						float64(c.MemoryLimit)/(1024*1024)))
					result.WriteString(fmt.Sprintf("- Restart Count: %d\n", c.RestartCount))

					if len(c.Ports) > 0 {
						result.WriteString("- Ports:\n")
						for _, p := range c.Ports {
							if p.PublicPort > 0 {
								result.WriteString(fmt.Sprintf("  - %s:%d -> %d/%s\n", p.IP, p.PublicPort, p.PrivatePort, p.Protocol))
							} else {
								result.WriteString(fmt.Sprintf("  - %d/%s (internal)\n", p.PrivatePort, p.Protocol))
							}
						}
					}

					if len(c.Networks) > 0 {
						result.WriteString("- Networks:\n")
						for _, n := range c.Networks {
							result.WriteString(fmt.Sprintf("  - %s: %s\n", n.Name, n.IPv4))
						}
					}

					if len(c.Mounts) > 0 {
						result.WriteString("- Mounts:\n")
						for _, m := range c.Mounts {
							rw := "ro"
							if m.RW {
								rw = "rw"
							}
							result.WriteString(fmt.Sprintf("  - %s -> %s (%s)\n", m.Source, m.Destination, rw))
						}
					}

					if len(c.Labels) > 0 {
						result.WriteString("- Labels:\n")
						for k, v := range c.Labels {
							// Skip long or internal labels
							if len(v) > 50 || strings.HasPrefix(k, "org.opencontainers") {
								continue
							}
							result.WriteString(fmt.Sprintf("  - %s: %s\n", k, v))
						}
					}

					if c.UpdateStatus != nil && c.UpdateStatus.UpdateAvailable {
						result.WriteString("- ⚠ Image update available!\n")
					}

					return NewTextResult(result.String()), nil
				}
			}
		}
		return NewTextResult(fmt.Sprintf("Docker container '%s' not found.", resourceID)), nil

	default:
		return NewTextResult("Invalid resource_type. Use 'vm', 'container', or 'docker'."), nil
	}
}

func (e *PulseToolExecutor) executeGetDiskHealth(_ context.Context, _ map[string]interface{}) (CallToolResult, error) {
	if e.diskHealthProvider == nil && e.storageProvider == nil {
		return NewTextResult("Disk health information not available."), nil
	}

	var result strings.Builder
	result.WriteString("# Disk Health Status\n\n")

	hasData := false

	// SMART and RAID data from host agents
	if e.diskHealthProvider != nil {
		hosts := e.diskHealthProvider.GetHosts()
		for _, host := range hosts {
			hostHasData := false

			// SMART data
			if len(host.Sensors.SMART) > 0 {
				if !hostHasData {
					result.WriteString(fmt.Sprintf("## Host: %s\n\n", host.Hostname))
					hostHasData = true
					hasData = true
				}
				result.WriteString("### SMART Data\n")
				for _, disk := range host.Sensors.SMART {
					healthIcon := "✓"
					if disk.Health != "PASSED" && disk.Health != "" {
						healthIcon = "⚠"
					}
					result.WriteString(fmt.Sprintf("- %s %s: %s", healthIcon, disk.Device, disk.Model))
					if disk.Temperature > 0 {
						result.WriteString(fmt.Sprintf(" (%d°C)", disk.Temperature))
					}
					if disk.Health != "" {
						result.WriteString(fmt.Sprintf(" [%s]", disk.Health))
					}
					result.WriteString("\n")
				}
				result.WriteString("\n")
			}

			// RAID arrays
			if len(host.RAID) > 0 {
				if !hostHasData {
					result.WriteString(fmt.Sprintf("## Host: %s\n\n", host.Hostname))
					hostHasData = true
					hasData = true
				}
				result.WriteString("### RAID Arrays\n")
				for _, raid := range host.RAID {
					stateIcon := "✓"
					if raid.State != "active" && raid.State != "clean" {
						stateIcon = "⚠"
					}
					result.WriteString(fmt.Sprintf("- %s %s (%s): %s\n",
						stateIcon, raid.Device, raid.Level, raid.State))
					result.WriteString(fmt.Sprintf("  - Devices: %d active, %d working, %d failed, %d spare\n",
						raid.ActiveDevices, raid.WorkingDevices, raid.FailedDevices, raid.SpareDevices))
					if raid.RebuildPercent > 0 {
						result.WriteString(fmt.Sprintf("  - Rebuilding: %.1f%%\n", raid.RebuildPercent))
					}
				}
				result.WriteString("\n")
			}

			// Host Ceph data
			if host.Ceph != nil {
				ceph := host.Ceph
				if !hostHasData {
					result.WriteString(fmt.Sprintf("## Host: %s\n\n", host.Hostname))
					hostHasData = true
					hasData = true
				}
				result.WriteString("### Ceph Status (from agent)\n")
				healthIcon := "✓"
				if ceph.Health.Status != "HEALTH_OK" {
					healthIcon = "⚠"
				}
				result.WriteString(fmt.Sprintf("- %s Health: %s\n", healthIcon, ceph.Health.Status))
				result.WriteString(fmt.Sprintf("- OSDs: %d up / %d in / %d total\n",
					ceph.OSDMap.NumUp, ceph.OSDMap.NumIn, ceph.OSDMap.NumOSDs))
				result.WriteString(fmt.Sprintf("- PGs: %d (%.1f%% used)\n",
					ceph.PGMap.NumPGs, ceph.PGMap.UsagePercent))
				result.WriteString("\n")
			}
		}
	}

	// Ceph clusters from Proxmox API
	if e.storageProvider != nil {
		cephClusters := e.storageProvider.GetCephClusters()
		if len(cephClusters) > 0 {
			result.WriteString("## Ceph Clusters (from Proxmox)\n\n")
			hasData = true
			for _, c := range cephClusters {
				healthIcon := "✓"
				if c.Health != "HEALTH_OK" {
					healthIcon = "⚠"
				}
				result.WriteString(fmt.Sprintf("### %s %s\n", healthIcon, c.Name))
				result.WriteString(fmt.Sprintf("- Health: %s", c.Health))
				if c.HealthMessage != "" {
					result.WriteString(fmt.Sprintf(" - %s", c.HealthMessage))
				}
				result.WriteString("\n")
				result.WriteString(fmt.Sprintf("- OSDs: %d up, %d in (of %d)\n",
					c.NumOSDsUp, c.NumOSDsIn, c.NumOSDs))
				result.WriteString("\n")
			}
		}
	}

	if !hasData {
		return NewTextResult("No disk health data available. Host agents may not be reporting SMART/RAID data, or no Ceph clusters are configured."), nil
	}

	return NewTextResult(result.String()), nil
}

// Control tool implementations

// GuestInfo represents resolved guest information
type GuestInfo struct {
	VMID     int
	Name     string
	Node     string
	Type     string // "vm" or "lxc"
	Status   string
	Instance string
}

func (e *PulseToolExecutor) executeControlGuest(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	guestID, _ := args["guest_id"].(string)
	action, _ := args["action"].(string)
	force, _ := args["force"].(bool)

	if guestID == "" {
		return NewErrorResult(fmt.Errorf("guest_id is required")), nil
	}
	if action == "" {
		return NewErrorResult(fmt.Errorf("action is required")), nil
	}

	// Validate action
	validActions := map[string]bool{"start": true, "stop": true, "shutdown": true, "restart": true}
	if !validActions[action] {
		return NewErrorResult(fmt.Errorf("invalid action: %s. Use start, stop, shutdown, or restart", action)), nil
	}

	// Check control level
	if e.controlLevel == ControlLevelReadOnly || e.controlLevel == "" {
		return NewTextResult("Control tools are disabled. Enable them in Settings > AI > Control Level."), nil
	}

	// Resolve guest to find VMID, node, and type
	guest, err := e.resolveGuest(guestID)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Could not find guest '%s': %v", guestID, err)), nil
	}

	// Check if guest is protected
	vmidStr := fmt.Sprintf("%d", guest.VMID)
	for _, protected := range e.protectedGuests {
		if protected == vmidStr || protected == guest.Name {
			return NewTextResult(fmt.Sprintf("Guest %s (VMID %d) is protected and cannot be controlled by AI.", guest.Name, guest.VMID)), nil
		}
	}

	// Build the command based on guest type and action
	var command string
	cmdTool := "pct" // LXC containers
	if guest.Type == "vm" {
		cmdTool = "qm"
	}

	switch action {
	case "start":
		command = fmt.Sprintf("%s start %d", cmdTool, guest.VMID)
	case "stop":
		command = fmt.Sprintf("%s stop %d", cmdTool, guest.VMID)
	case "shutdown":
		command = fmt.Sprintf("%s shutdown %d", cmdTool, guest.VMID)
	case "restart":
		// Restart is shutdown + start, but we'll use reboot for simplicity
		command = fmt.Sprintf("%s reboot %d", cmdTool, guest.VMID)
	}

	// Add force flag if requested (only for stop)
	if force && action == "stop" {
		command = fmt.Sprintf("%s stop %d --skiplock", cmdTool, guest.VMID)
	}

	// Check security policy
	if e.policy != nil {
		decision := e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This command is blocked by security policy")), nil
		}
		// For control level "controlled", always require approval
		if e.controlLevel == ControlLevelControlled || (decision == agentexec.PolicyRequireApproval && !e.isAutonomous) {
			return NewTextResult(formatControlApprovalNeeded(guest.Name, guest.VMID, action, command)), nil
		}
	}

	// For "suggest" mode, just return the command suggestion
	if e.controlLevel == ControlLevelSuggest {
		return NewTextResult(formatControlSuggestion(guest.Name, guest.VMID, action, command, guest.Node)), nil
	}

	// Execute the command via agent
	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	// Find agent for the node that owns this guest
	agentID := e.findAgentForNode(guest.Node)
	if agentID == "" {
		return NewTextResult(fmt.Sprintf("No agent available on node '%s'. Install Pulse Unified Agent on the Proxmox host to enable control.", guest.Node)), nil
	}

	// Execute command
	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: "host",
		TargetID:   "",
	})
	if err != nil {
		return NewErrorResult(err), nil
	}

	// Format result
	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}

	if result.ExitCode == 0 {
		return NewTextResult(fmt.Sprintf("Successfully executed '%s' on %s (VMID %d).\n%s", action, guest.Name, guest.VMID, output)), nil
	}

	return NewTextResult(fmt.Sprintf("Command failed (exit code %d):\n%s", result.ExitCode, output)), nil
}

func (e *PulseToolExecutor) executeControlDocker(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	containerName, _ := args["container"].(string)
	hostName, _ := args["host"].(string)
	action, _ := args["action"].(string)

	if containerName == "" {
		return NewErrorResult(fmt.Errorf("container name is required")), nil
	}
	if action == "" {
		return NewErrorResult(fmt.Errorf("action is required")), nil
	}

	// Validate action
	validActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validActions[action] {
		return NewErrorResult(fmt.Errorf("invalid action: %s. Use start, stop, or restart", action)), nil
	}

	// Check control level
	if e.controlLevel == ControlLevelReadOnly || e.controlLevel == "" {
		return NewTextResult("Control tools are disabled. Enable them in Settings > AI > Control Level."), nil
	}

	// Find the Docker container and its host
	container, dockerHost, err := e.resolveDockerContainer(containerName, hostName)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Could not find Docker container '%s': %v", containerName, err)), nil
	}

	// Build the command
	command := fmt.Sprintf("docker %s %s", action, container.Name)

	// Check security policy
	if e.policy != nil {
		decision := e.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			return NewTextResult(formatPolicyBlocked(command, "This command is blocked by security policy")), nil
		}
		if e.controlLevel == ControlLevelControlled || (decision == agentexec.PolicyRequireApproval && !e.isAutonomous) {
			return NewTextResult(formatDockerApprovalNeeded(container.Name, dockerHost.Hostname, action, command)), nil
		}
	}

	// For "suggest" mode, just return the command suggestion
	if e.controlLevel == ControlLevelSuggest {
		return NewTextResult(formatDockerSuggestion(container.Name, dockerHost.Hostname, action, command)), nil
	}

	// Execute the command via agent
	if e.agentServer == nil {
		return NewErrorResult(fmt.Errorf("no agent server available")), nil
	}

	// Find agent for this Docker host
	agentID := e.findAgentForDockerHost(dockerHost)
	if agentID == "" {
		return NewTextResult(fmt.Sprintf("No agent available on Docker host '%s'. Install Pulse Unified Agent on the host to enable control.", dockerHost.Hostname)), nil
	}

	// Execute command
	result, err := e.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: "host",
		TargetID:   "",
	})
	if err != nil {
		return NewErrorResult(err), nil
	}

	// Format result
	output := result.Stdout
	if result.Stderr != "" {
		output += "\n" + result.Stderr
	}

	if result.ExitCode == 0 {
		return NewTextResult(fmt.Sprintf("Successfully executed 'docker %s' on container '%s' (host: %s).\n%s", action, container.Name, dockerHost.Hostname, output)), nil
	}

	return NewTextResult(fmt.Sprintf("Command failed (exit code %d):\n%s", result.ExitCode, output)), nil
}

// resolveGuest finds a guest (VM or container) by VMID or name
func (e *PulseToolExecutor) resolveGuest(guestID string) (*GuestInfo, error) {
	if e.stateProvider == nil {
		return nil, fmt.Errorf("state provider not available")
	}

	state := e.stateProvider.GetState()

	// Try to parse as VMID
	vmid, err := strconv.Atoi(guestID)

	// Search VMs
	for _, vm := range state.VMs {
		if (err == nil && vm.VMID == vmid) || vm.Name == guestID || vm.ID == guestID {
			return &GuestInfo{
				VMID:     vm.VMID,
				Name:     vm.Name,
				Node:     vm.Node,
				Type:     "vm",
				Status:   vm.Status,
				Instance: vm.Instance,
			}, nil
		}
	}

	// Search containers
	for _, ct := range state.Containers {
		if (err == nil && ct.VMID == vmid) || ct.Name == guestID || ct.ID == guestID {
			return &GuestInfo{
				VMID:     ct.VMID,
				Name:     ct.Name,
				Node:     ct.Node,
				Type:     "lxc",
				Status:   ct.Status,
				Instance: ct.Instance,
			}, nil
		}
	}

	return nil, fmt.Errorf("no VM or container found with ID or name '%s'", guestID)
}

// resolveDockerContainer finds a Docker container by name or ID
func (e *PulseToolExecutor) resolveDockerContainer(containerName, hostName string) (*models.DockerContainer, *models.DockerHost, error) {
	if e.stateProvider == nil {
		return nil, nil, fmt.Errorf("state provider not available")
	}

	state := e.stateProvider.GetState()

	for _, host := range state.DockerHosts {
		// If host name specified, only search that host
		if hostName != "" && host.Hostname != hostName && host.DisplayName != hostName {
			continue
		}

		for i, container := range host.Containers {
			if container.Name == containerName ||
				container.ID == containerName ||
				strings.HasPrefix(container.ID, containerName) {
				return &host.Containers[i], &host, nil
			}
		}
	}

	if hostName != "" {
		return nil, nil, fmt.Errorf("container '%s' not found on host '%s'", containerName, hostName)
	}
	return nil, nil, fmt.Errorf("container '%s' not found on any Docker host", containerName)
}

// findAgentForNode finds an agent connected to a specific Proxmox node
func (e *PulseToolExecutor) findAgentForNode(nodeName string) string {
	if e.agentServer == nil {
		return ""
	}

	agents := e.agentServer.GetConnectedAgents()
	for _, agent := range agents {
		// Check if agent hostname matches node name (common setup)
		if agent.Hostname == nodeName {
			return agent.AgentID
		}
		// Also check if agent has a linked node
		// Note: This requires the Host model to have LinkedNodeID populated
	}

	// If no exact match, check hosts for linked agents
	if e.stateProvider != nil {
		state := e.stateProvider.GetState()
		for _, host := range state.Hosts {
			if host.LinkedNodeID != "" {
				// Check if this host's linked node matches
				for _, node := range state.Nodes {
					if node.ID == host.LinkedNodeID && node.Name == nodeName {
						// Find agent for this host
						for _, agent := range agents {
							if agent.Hostname == host.Hostname || agent.AgentID == host.ID {
								return agent.AgentID
							}
						}
					}
				}
			}
		}
	}

	return ""
}

// findAgentForDockerHost finds an agent connected to a Docker host
func (e *PulseToolExecutor) findAgentForDockerHost(dockerHost *models.DockerHost) string {
	if e.agentServer == nil {
		return ""
	}

	agents := e.agentServer.GetConnectedAgents()
	for _, agent := range agents {
		if agent.Hostname == dockerHost.Hostname {
			return agent.AgentID
		}
	}

	return ""
}

// formatControlApprovalNeeded formats a response for guest control needing approval
func formatControlApprovalNeeded(name string, vmid int, action, command string) string {
	payload := map[string]interface{}{
		"type":           "control_approval_required",
		"guest_name":     name,
		"guest_vmid":     vmid,
		"action":         action,
		"command":        command,
		"how_to_approve": "This action requires approval. Ask the user to confirm they want to proceed.",
		"do_not_retry":   true,
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}

// formatDockerApprovalNeeded formats a response for Docker control needing approval
func formatDockerApprovalNeeded(name, host, action, command string) string {
	payload := map[string]interface{}{
		"type":           "control_approval_required",
		"container_name": name,
		"docker_host":    host,
		"action":         action,
		"command":        command,
		"how_to_approve": "This action requires approval. Ask the user to confirm they want to proceed.",
		"do_not_retry":   true,
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}

// formatControlSuggestion formats a command suggestion for "suggest" mode
func formatControlSuggestion(name string, vmid int, action, command, node string) string {
	return fmt.Sprintf(`To %s %s (VMID %d), run this command on node %s:

%s

Copy and paste this command to execute it manually.`, action, name, vmid, node, command)
}

// formatDockerSuggestion formats a Docker command suggestion for "suggest" mode
func formatDockerSuggestion(name, host, action, command string) string {
	return fmt.Sprintf(`To %s container '%s' on host %s, run:

%s

Copy and paste this command to execute it manually.`, action, name, host, command)
}
