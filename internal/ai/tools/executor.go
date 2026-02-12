package tools

import (
	"context"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// UnifiedResourceProvider gives the executor access to the unified resource
// registry so that tool handlers can read physical disks, Ceph clusters, etc.
// from the canonical model instead of raw StateSnapshot fields.
type UnifiedResourceProvider interface {
	GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource
}

// ServerVersion is the version of the MCP tool implementation
const ServerVersion = "1.0.0"

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

// BaselineProvider provides learned baselines for anomaly detection
type BaselineProvider interface {
	GetBaseline(resourceID, metric string) *MetricBaseline
	GetAllBaselines() map[string]map[string]*MetricBaseline // resourceID -> metric -> baseline
}

// PatternProvider provides detected patterns and predictions
type PatternProvider interface {
	GetPatterns() []Pattern
	GetPredictions() []Prediction
}

// AlertProvider provides active alerts
type AlertProvider interface {
	GetActiveAlerts() []ActiveAlert
}

// FindingsProvider provides patrol findings
type FindingsProvider interface {
	GetActiveFindings() []Finding
	GetDismissedFindings() []Finding
}

// PatrolFindingCreator is set on the executor during a patrol run to allow
// patrol-specific tools (patrol_report_finding, patrol_resolve_finding,
// patrol_get_findings) to create, resolve, and query findings.
// Outside of a patrol run this is nil, and the tools return a clear error.
type PatrolFindingCreator interface {
	CreateFinding(input PatrolFindingInput) (findingID string, isNew bool, err error)
	ResolveFinding(findingID, reason string) error
	GetActiveFindings(resourceID, minSeverity string) []PatrolFindingInfo
}

// PatrolFindingsChecker tracks whether a patrol run has queried existing findings.
// Tools can use this to enforce calling patrol_get_findings before reporting or resolving.
type PatrolFindingsChecker interface {
	HasCheckedFindings() bool
}

// PatrolFindingInput contains the structured parameters the LLM passes
// to the patrol_report_finding tool.
type PatrolFindingInput struct {
	Key            string `json:"key"`
	Severity       string `json:"severity"`
	Category       string `json:"category"`
	ResourceID     string `json:"resource_id"`
	ResourceName   string `json:"resource_name"`
	ResourceType   string `json:"resource_type"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Recommendation string `json:"recommendation,omitempty"`
	Evidence       string `json:"evidence,omitempty"`
}

// PatrolFindingInfo is a lightweight view of a finding returned by
// PatrolFindingCreator.GetActiveFindings.
type PatrolFindingInfo struct {
	ID           string `json:"id"`
	Key          string `json:"key,omitempty"`
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	ResourceType string `json:"resource_type"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	DetectedAt   string `json:"detected_at"`
}

// BackupProvider provides backup information
type BackupProvider interface {
	GetBackups() models.Backups
	GetPBSInstances() []models.PBSInstance
}

// GuestConfigProvider provides guest configuration data (VM/LXC).
type GuestConfigProvider interface {
	GetGuestConfig(guestType, instance, node string, vmID int) (map[string]interface{}, error)
}

// DiskHealthProvider provides disk health information from host agents
type DiskHealthProvider interface {
	GetHosts() []models.Host
}

// UpdatesProvider provides Docker update operations for MCP tools
type UpdatesProvider interface {
	GetPendingUpdates(hostID string) []ContainerUpdateInfo
	TriggerUpdateCheck(hostID string) (DockerCommandStatus, error)
	UpdateContainer(hostID, containerID, containerName string) (DockerCommandStatus, error)
	IsUpdateActionsEnabled() bool
}

// DiscoveryProvider provides AI-powered infrastructure discovery
type DiscoveryProvider interface {
	GetDiscovery(id string) (*ResourceDiscoveryInfo, error)
	GetDiscoveryByResource(resourceType, hostID, resourceID string) (*ResourceDiscoveryInfo, error)
	ListDiscoveries() ([]*ResourceDiscoveryInfo, error)
	ListDiscoveriesByType(resourceType string) ([]*ResourceDiscoveryInfo, error)
	ListDiscoveriesByHost(hostID string) ([]*ResourceDiscoveryInfo, error)
	FormatForAIContext(discoveries []*ResourceDiscoveryInfo) string
	// TriggerDiscovery initiates discovery for a resource and returns the result
	TriggerDiscovery(ctx context.Context, resourceType, hostID, resourceID string) (*ResourceDiscoveryInfo, error)
}

// ResolvedResourceInfo contains the minimal information needed for tool validation.
// This is an interface to avoid import cycles with the chat package.
type ResolvedResourceInfo interface {
	GetResourceID() string
	GetResourceType() string
	GetTargetHost() string
	GetAgentID() string
	GetAdapter() string
	GetVMID() int
	GetNode() string
	GetAllowedActions() []string
	// New structured identity methods
	GetProviderUID() string
	GetKind() string
	GetAliases() []string
}

// ResourceRegistration contains all fields needed to register a discovered resource.
// This structured approach replaces the long parameter list for clarity.
type ResourceRegistration struct {
	// Identity
	Kind        string   // Resource type: "node", "vm", "lxc", "docker_container", etc.
	ProviderUID string   // Stable provider ID (container ID, VMID, pod UID)
	Name        string   // Primary display name
	Aliases     []string // Additional names that resolve to this resource

	// Scope
	HostUID    string
	HostName   string
	ParentUID  string
	ParentKind string
	ClusterUID string
	Namespace  string

	// Legacy fields (for backwards compatibility)
	VMID          int
	Node          string
	LocationChain []string

	// Executor paths
	Executors []ExecutorRegistration
}

// ExecutorRegistration describes how an executor can reach a resource.
type ExecutorRegistration struct {
	ExecutorID string
	Adapter    string
	Actions    []string
	Priority   int
}

// ResolvedContextProvider provides session-scoped resource resolution.
// Query and discovery tools add resources; action tools validate against them.
// This interface is implemented by the chat package's ResolvedContext.
type ResolvedContextProvider interface {
	// AddResolvedResource adds a resource that was found via query/discovery.
	// Uses the new structured registration format.
	AddResolvedResource(reg ResourceRegistration)

	// GetResolvedResourceByID retrieves a resource by its canonical ID (kind:provider_uid)
	GetResolvedResourceByID(resourceID string) (ResolvedResourceInfo, bool)

	// GetResolvedResourceByAlias retrieves a resource by any of its aliases
	GetResolvedResourceByAlias(alias string) (ResolvedResourceInfo, bool)

	// ValidateResourceForAction checks if a resource can perform an action
	// Returns the resource if valid, error if not found or action not allowed
	ValidateResourceForAction(resourceID, action string) (ResolvedResourceInfo, error)

	// HasAnyResources returns true if at least one resource has been discovered
	HasAnyResources() bool

	// WasRecentlyAccessed checks if a resource was accessed within the given time window.
	// Used for routing validation to distinguish "this turn" from "session-wide" context.
	WasRecentlyAccessed(resourceID string, window time.Duration) bool

	// GetRecentlyAccessedResources returns resource IDs accessed within the given time window.
	GetRecentlyAccessedResources(window time.Duration) []string

	// MarkExplicitAccess marks a resource as recently accessed, indicating user intent.
	// Call this for single-resource operations (get, explicit select) but NOT for bulk
	// operations (list, search) to avoid poisoning routing validation.
	MarkExplicitAccess(resourceID string)
}

// RecentAccessWindow is the time window used to determine "recently referenced" resources.
// Resources accessed within this window are considered to be from the current turn/exchange.
const RecentAccessWindow = 30 * time.Second

// ResourceDiscoveryInfo represents discovered information about a resource
type ResourceDiscoveryInfo struct {
	ID             string              `json:"id"`
	ResourceType   string              `json:"resource_type"`
	ResourceID     string              `json:"resource_id"`
	HostID         string              `json:"host_id"`
	Hostname       string              `json:"hostname"`
	ServiceType    string              `json:"service_type"`
	ServiceName    string              `json:"service_name"`
	ServiceVersion string              `json:"service_version"`
	Category       string              `json:"category"`
	CLIAccess      string              `json:"cli_access"`
	Facts          []DiscoveryFact     `json:"facts"`
	ConfigPaths    []string            `json:"config_paths"`
	DataPaths      []string            `json:"data_paths"`
	LogPaths       []string            `json:"log_paths,omitempty"` // Log file paths or commands (e.g., journalctl)
	Ports          []DiscoveryPortInfo `json:"ports"`
	BindMounts     []DiscoveryMount    `json:"bind_mounts,omitempty"` // For Docker: host->container path mappings
	UserNotes      string              `json:"user_notes,omitempty"`
	Confidence     float64             `json:"confidence"`
	AIReasoning    string              `json:"ai_reasoning,omitempty"`
	DiscoveredAt   time.Time           `json:"discovered_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

// DiscoveryPortInfo represents a listening port discovered on a resource
type DiscoveryPortInfo struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Process  string `json:"process,omitempty"`
	Address  string `json:"address,omitempty"`
}

// DiscoveryMount represents a bind mount (host path -> container path)
type DiscoveryMount struct {
	ContainerName string `json:"container_name,omitempty"` // Docker container name (for Docker inside LXC/VM)
	Source        string `json:"source"`                   // Host path (where to actually write files)
	Destination   string `json:"destination"`              // Container path (what the service sees)
	Type          string `json:"type,omitempty"`           // Mount type: bind, volume, tmpfs
	ReadOnly      bool   `json:"read_only,omitempty"`
}

// DiscoveryFact represents a discovered fact about a resource
type DiscoveryFact struct {
	Category   string  `json:"category"`
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Source     string  `json:"source,omitempty"`
	Confidence float64 `json:"confidence,omitempty"` // 0-1 confidence for this fact
}

// ControlLevel represents the AI's permission level for infrastructure control
type ControlLevel string

const (
	// ControlLevelReadOnly - AI can only query, no control tools available
	ControlLevelReadOnly ControlLevel = "read_only"
	// ControlLevelControlled - AI can execute with per-command approval
	ControlLevelControlled ControlLevel = "controlled"
	// ControlLevelAutonomous - AI executes without approval (requires Pro license)
	ControlLevelAutonomous ControlLevel = "autonomous"
)

// ExecutorConfig holds all dependencies for the tool executor
type ExecutorConfig struct {
	// Required providers
	StateProvider StateProvider
	Policy        CommandPolicy
	AgentServer   AgentServer

	// Optional providers - patrol context
	MetricsHistory   MetricsHistoryProvider
	BaselineProvider BaselineProvider
	PatternProvider  PatternProvider
	AlertProvider    AlertProvider
	FindingsProvider FindingsProvider

	// Optional providers - infrastructure
	BackupProvider BackupProvider

	GuestConfigProvider GuestConfigProvider
	DiskHealthProvider  DiskHealthProvider
	UpdatesProvider     UpdatesProvider

	// Optional providers - management
	MetadataUpdater     MetadataUpdater
	FindingsManager     FindingsManager
	AgentProfileManager AgentProfileManager

	// Optional providers - intelligence
	IncidentRecorderProvider IncidentRecorderProvider
	EventCorrelatorProvider  EventCorrelatorProvider
	TopologyProvider         TopologyProvider
	KnowledgeStoreProvider   KnowledgeStoreProvider

	// Optional providers - discovery
	DiscoveryProvider DiscoveryProvider

	// Optional providers - unified resources
	UnifiedResourceProvider UnifiedResourceProvider
	// Optional typed read access to current infrastructure state.
	// When provided, tool handlers should prefer this over models.StateSnapshot iteration.
	ReadState unifiedresources.ReadState

	// Control settings
	ControlLevel    ControlLevel
	ProtectedGuests []string // VMIDs that AI cannot control
}

// PulseToolExecutor implements ToolExecutor for Pulse-specific tools
type PulseToolExecutor struct {
	// Core providers
	stateProvider StateProvider
	policy        CommandPolicy
	agentServer   AgentServer

	// Patrol context providers
	metricsHistory   MetricsHistoryProvider
	baselineProvider BaselineProvider
	patternProvider  PatternProvider
	alertProvider    AlertProvider
	findingsProvider FindingsProvider

	// Infrastructure context providers
	backupProvider BackupProvider

	guestConfigProvider GuestConfigProvider
	diskHealthProvider  DiskHealthProvider
	updatesProvider     UpdatesProvider

	// Management providers
	metadataUpdater     MetadataUpdater
	findingsManager     FindingsManager
	agentProfileManager AgentProfileManager

	// Intelligence providers
	incidentRecorderProvider IncidentRecorderProvider
	eventCorrelatorProvider  EventCorrelatorProvider
	topologyProvider         TopologyProvider
	knowledgeStoreProvider   KnowledgeStoreProvider

	// Discovery provider
	discoveryProvider DiscoveryProvider

	// Unified resources provider
	unifiedResourceProvider UnifiedResourceProvider
	// Typed state reader. Nil means "legacy-only": tools must fall back to StateSnapshot access.
	readState unifiedresources.ReadState

	// Control settings
	controlLevel    ControlLevel
	protectedGuests []string

	// Current execution context
	targetType   string
	targetID     string
	isAutonomous bool

	// Session-scoped resolved context for resource validation
	// This is set per-session by the agentic loop before tool execution
	resolvedContext ResolvedContextProvider

	// Telemetry callback for recording metrics
	// This is optional - if nil, no telemetry is recorded
	telemetryCallback TelemetryCallback

	// Patrol finding creator — set only during a patrol run, nil otherwise.
	// Enables patrol_report_finding, patrol_resolve_finding, patrol_get_findings tools.
	patrolFindingCreatorMu sync.RWMutex
	patrolFindingCreator   PatrolFindingCreator

	// Tool registry
	registry *ToolRegistry
}

// TelemetryCallback is called when the executor needs to record telemetry.
// This allows the chat layer to handle metrics without import cycles.
type TelemetryCallback interface {
	// RecordStrictResolutionBlock records when strict resolution blocks an action
	RecordStrictResolutionBlock(tool, action string)
	// RecordAutoRecoveryAttempt records an auto-recovery attempt
	RecordAutoRecoveryAttempt(errorCode, tool string)
	// RecordAutoRecoverySuccess records a successful auto-recovery
	RecordAutoRecoverySuccess(errorCode, tool string)
	// RecordRoutingMismatchBlock records when routing validation blocks an operation
	// that targeted a parent host when a child resource was recently referenced.
	// targetKind: "node" (the kind being targeted)
	// childKind: "lxc", "vm", "docker_container" (the kind of the more specific resource)
	RecordRoutingMismatchBlock(tool, targetKind, childKind string)
}

// NewPulseToolExecutor creates a new Pulse tool executor with the given configuration
func NewPulseToolExecutor(cfg ExecutorConfig) *PulseToolExecutor {
	e := &PulseToolExecutor{
		stateProvider:    cfg.StateProvider,
		policy:           cfg.Policy,
		agentServer:      cfg.AgentServer,
		metricsHistory:   cfg.MetricsHistory,
		baselineProvider: cfg.BaselineProvider,
		patternProvider:  cfg.PatternProvider,
		alertProvider:    cfg.AlertProvider,
		findingsProvider: cfg.FindingsProvider,
		backupProvider:   cfg.BackupProvider,

		guestConfigProvider:      cfg.GuestConfigProvider,
		diskHealthProvider:       cfg.DiskHealthProvider,
		updatesProvider:          cfg.UpdatesProvider,
		metadataUpdater:          cfg.MetadataUpdater,
		findingsManager:          cfg.FindingsManager,
		agentProfileManager:      cfg.AgentProfileManager,
		incidentRecorderProvider: cfg.IncidentRecorderProvider,
		eventCorrelatorProvider:  cfg.EventCorrelatorProvider,
		topologyProvider:         cfg.TopologyProvider,
		knowledgeStoreProvider:   cfg.KnowledgeStoreProvider,
		discoveryProvider:        cfg.DiscoveryProvider,
		unifiedResourceProvider:  cfg.UnifiedResourceProvider,
		readState:                cfg.ReadState,
		controlLevel:             cfg.ControlLevel,
		protectedGuests:          cfg.ProtectedGuests,
		registry:                 NewToolRegistry(),
	}

	// Register all tools
	e.registerTools()

	return e
}

func (e *PulseToolExecutor) getReadState() unifiedresources.ReadState {
	return e.readState
}

// SetContext sets the current execution context
func (e *PulseToolExecutor) SetContext(targetType, targetID string, autonomous bool) {
	e.targetType = targetType
	e.targetID = targetID
	e.isAutonomous = autonomous
}

// SetControlLevel updates the control level
func (e *PulseToolExecutor) SetControlLevel(level ControlLevel) {
	e.controlLevel = level
}

// SetProtectedGuests updates the protected guests list
func (e *PulseToolExecutor) SetProtectedGuests(vmids []string) {
	e.protectedGuests = vmids
}

// RegisterTool allows tests or extensions to add tools at runtime.
func (e *PulseToolExecutor) RegisterTool(tool RegisteredTool) {
	e.registry.Register(tool)
}

// Runtime setter methods for updating providers after creation

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

// SetGuestConfigProvider sets the guest config provider
func (e *PulseToolExecutor) SetGuestConfigProvider(provider GuestConfigProvider) {
	e.guestConfigProvider = provider
}

// SetDiskHealthProvider sets the disk health provider
func (e *PulseToolExecutor) SetDiskHealthProvider(provider DiskHealthProvider) {
	e.diskHealthProvider = provider
}

// SetAgentProfileManager sets the agent profile manager
func (e *PulseToolExecutor) SetAgentProfileManager(manager AgentProfileManager) {
	e.agentProfileManager = manager
}

// SetUpdatesProvider sets the updates provider for Docker container updates
func (e *PulseToolExecutor) SetUpdatesProvider(provider UpdatesProvider) {
	e.updatesProvider = provider
}

// SetIncidentRecorderProvider sets the incident recorder provider
func (e *PulseToolExecutor) SetIncidentRecorderProvider(provider IncidentRecorderProvider) {
	e.incidentRecorderProvider = provider
}

// SetEventCorrelatorProvider sets the event correlator provider
func (e *PulseToolExecutor) SetEventCorrelatorProvider(provider EventCorrelatorProvider) {
	e.eventCorrelatorProvider = provider
}

// SetTopologyProvider sets the topology provider for relationship graphs
func (e *PulseToolExecutor) SetTopologyProvider(provider TopologyProvider) {
	e.topologyProvider = provider
}

// SetKnowledgeStoreProvider sets the knowledge store provider for notes
func (e *PulseToolExecutor) SetKnowledgeStoreProvider(provider KnowledgeStoreProvider) {
	e.knowledgeStoreProvider = provider
}

// SetDiscoveryProvider sets the discovery provider for infrastructure discovery
func (e *PulseToolExecutor) SetDiscoveryProvider(provider DiscoveryProvider) {
	e.discoveryProvider = provider
}

// SetUnifiedResourceProvider sets the unified resource provider
func (e *PulseToolExecutor) SetUnifiedResourceProvider(provider UnifiedResourceProvider) {
	e.unifiedResourceProvider = provider
}

// SetResolvedContext sets the session-scoped resolved context for resource validation.
// This should be called by the agentic loop before executing tools for a session.
func (e *PulseToolExecutor) SetResolvedContext(ctx ResolvedContextProvider) {
	e.resolvedContext = ctx
}

// SetTelemetryCallback sets the telemetry callback for recording metrics
func (e *PulseToolExecutor) SetTelemetryCallback(cb TelemetryCallback) {
	e.telemetryCallback = cb
}

// SetPatrolFindingCreator sets (or clears) the patrol finding creator.
// This must be set before a patrol run and cleared after.
func (e *PulseToolExecutor) SetPatrolFindingCreator(creator PatrolFindingCreator) {
	e.patrolFindingCreatorMu.Lock()
	e.patrolFindingCreator = creator
	e.patrolFindingCreatorMu.Unlock()
}

// GetPatrolFindingCreator returns the current patrol finding creator (may be nil).
func (e *PulseToolExecutor) GetPatrolFindingCreator() PatrolFindingCreator {
	e.patrolFindingCreatorMu.RLock()
	defer e.patrolFindingCreatorMu.RUnlock()
	return e.patrolFindingCreator
}

// GetResolvedContext returns the current resolved context (may be nil)
func (e *PulseToolExecutor) GetResolvedContext() ResolvedContextProvider {
	return e.resolvedContext
}

// ListTools returns the list of available tools
func (e *PulseToolExecutor) ListTools() []Tool {
	tools := e.registry.ListTools(e.controlLevel)
	if len(tools) == 0 {
		return tools
	}

	available := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		if e.isToolAvailable(tool.Name) {
			available = append(available, tool)
		}
	}
	return available
}

func (e *PulseToolExecutor) isToolAvailable(name string) bool {
	switch name {
	// Check tool availability based on primary requirements
	case "pulse_query":
		return e.stateProvider != nil
	case "pulse_metrics":
		return e.stateProvider != nil || e.metricsHistory != nil || e.baselineProvider != nil || e.patternProvider != nil
	case "pulse_storage":
		return e.stateProvider != nil || e.unifiedResourceProvider != nil || e.backupProvider != nil || e.diskHealthProvider != nil
	case "pulse_docker":
		return e.stateProvider != nil || e.updatesProvider != nil
	case "pulse_kubernetes":
		return e.stateProvider != nil
	case "pulse_alerts":
		return e.alertProvider != nil || e.findingsProvider != nil || e.findingsManager != nil || e.stateProvider != nil
	case "pulse_read":
		return e.agentServer != nil
	case "pulse_control":
		return e.agentServer != nil && e.stateProvider != nil
	case "pulse_file_edit":
		return e.agentServer != nil
	case "pulse_discovery":
		return e.discoveryProvider != nil
	case "pulse_knowledge":
		return e.knowledgeStoreProvider != nil || e.incidentRecorderProvider != nil || e.eventCorrelatorProvider != nil || e.topologyProvider != nil
	case "pulse_pmg":
		return e.stateProvider != nil
	case "patrol_report_finding", "patrol_resolve_finding", "patrol_get_findings":
		// Always available when registered; handler checks patrolFindingCreator at runtime
		return e.GetPatrolFindingCreator() != nil
	default:
		return e.stateProvider != nil
	}
}

// ExecuteTool executes a tool and returns the result
func (e *PulseToolExecutor) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (CallToolResult, error) {
	log.Debug().
		Str("tool", name).
		Interface("args", args).
		Msg("Executing Pulse tool")

	return e.registry.Execute(ctx, e, name, args)
}

// registerTools registers all available tools
func (e *PulseToolExecutor) registerTools() {
	// All tools registered below (12 tools total)

	// pulse_query - search, get, config, topology, list, health
	e.registerQueryTools()

	// pulse_metrics - performance, temperatures, network, diskio, disks, baselines, patterns
	e.registerMetricsTools()

	// pulse_storage - pools, config, backups, snapshots, ceph, replication, pbs_jobs, raid, disk_health, resource_disks
	e.registerStorageTools()

	// pulse_docker - control, updates, check_updates, update, services, tasks, swarm
	e.registerDockerTools()

	// pulse_kubernetes - clusters, nodes, pods, deployments
	e.registerKubernetesTools()

	// pulse_alerts - list, findings, resolved, resolve, dismiss
	e.registerAlertsTools()

	// pulse_read - read-only operations (exec, file, find, tail, logs)
	// This is ALWAYS classified as ToolKindRead and never triggers VERIFYING
	e.registerReadTools()

	// pulse_control - guest control, run commands (requires control permission)
	// NOTE: For read-only command execution, use pulse_read instead
	e.registerControlTools()

	// pulse_file_edit - read, append, write files (requires control permission)
	e.registerFileTools()

	// pulse_discovery - get, list discoveries
	e.registerDiscoveryTools()

	// pulse_knowledge - remember, recall, incidents, correlate, relationships
	e.registerKnowledgeTools()

	// pulse_pmg - status, mail_stats, queues, spam
	e.registerPMGTools()

	// patrol_report_finding, patrol_resolve_finding, patrol_get_findings
	// These are always registered but only functional when patrolFindingCreator is set.
	e.registerPatrolTools()
}
