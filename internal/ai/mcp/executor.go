package mcp

import (
	"context"
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
	BackupProvider     BackupProvider
	StorageProvider    StorageProvider
	DiskHealthProvider DiskHealthProvider

	// Optional providers - management
	MetadataUpdater     MetadataUpdater
	FindingsManager     FindingsManager
	AgentProfileManager AgentProfileManager

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
	backupProvider     BackupProvider
	storageProvider    StorageProvider
	diskHealthProvider DiskHealthProvider

	// Management providers
	metadataUpdater     MetadataUpdater
	findingsManager     FindingsManager
	agentProfileManager AgentProfileManager

	// Control settings
	controlLevel    ControlLevel
	protectedGuests []string

	// Current execution context
	targetType   string
	targetID     string
	isAutonomous bool

	// Tool registry
	registry *ToolRegistry
}

// NewPulseToolExecutor creates a new Pulse tool executor with the given configuration
func NewPulseToolExecutor(cfg ExecutorConfig) *PulseToolExecutor {
	e := &PulseToolExecutor{
		stateProvider:       cfg.StateProvider,
		policy:              cfg.Policy,
		agentServer:         cfg.AgentServer,
		metricsHistory:      cfg.MetricsHistory,
		baselineProvider:    cfg.BaselineProvider,
		patternProvider:     cfg.PatternProvider,
		alertProvider:       cfg.AlertProvider,
		findingsProvider:    cfg.FindingsProvider,
		backupProvider:      cfg.BackupProvider,
		storageProvider:     cfg.StorageProvider,
		diskHealthProvider:  cfg.DiskHealthProvider,
		metadataUpdater:     cfg.MetadataUpdater,
		findingsManager:     cfg.FindingsManager,
		agentProfileManager: cfg.AgentProfileManager,
		controlLevel:        cfg.ControlLevel,
		protectedGuests:     cfg.ProtectedGuests,
		registry:            NewToolRegistry(),
	}

	// Register all tools
	e.registerTools()

	return e
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

// SetStorageProvider sets the storage provider
func (e *PulseToolExecutor) SetStorageProvider(provider StorageProvider) {
	e.storageProvider = provider
}

// SetDiskHealthProvider sets the disk health provider
func (e *PulseToolExecutor) SetDiskHealthProvider(provider DiskHealthProvider) {
	e.diskHealthProvider = provider
}

// SetAgentProfileManager sets the agent profile manager
func (e *PulseToolExecutor) SetAgentProfileManager(manager AgentProfileManager) {
	e.agentProfileManager = manager
}

// ListTools returns the list of available tools
func (e *PulseToolExecutor) ListTools() []Tool {
	return e.registry.ListTools(e.controlLevel)
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
	// Query tools (always available)
	e.registerQueryTools()

	// Patrol context tools (always available)
	e.registerPatrolTools()

	// Infrastructure tools (always available)
	e.registerInfrastructureTools()

	// Profile tools - read operations always available
	e.registerProfileTools()

	// Control tools (conditional on control level)
	e.registerControlTools()
}
