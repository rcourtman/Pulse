package aicontracts

import (
	"context"
	"encoding/json"
	"time"
)

// ---------------------------------------------------------------------------
// Orchestrator dependency interfaces
// ---------------------------------------------------------------------------
// These interfaces define what the investigation orchestrator needs from
// the outside world. They live in pkg/aicontracts so both the OSS repo
// (which assembles the deps) and the enterprise repo (which constructs the
// concrete Orchestrator) can reference them without hitting internal/ visibility.

// OrchestratorChatService provides AI chat session management for investigations.
type OrchestratorChatService interface {
	CreateSession(ctx context.Context) (*OrchestratorChatSession, error)
	ExecuteStream(ctx context.Context, req OrchestratorExecuteRequest, callback OrchestratorStreamCallback) error
	GetMessages(ctx context.Context, sessionID string) ([]OrchestratorMessage, error)
	DeleteSession(ctx context.Context, sessionID string) error
	ListAvailableTools(ctx context.Context, prompt string) []string
	SetAutonomousMode(enabled bool)
}

// OrchestratorCommandExecutor executes commands directly (bypasses the LLM).
type OrchestratorCommandExecutor interface {
	ExecuteCommand(ctx context.Context, command, targetHost string) (output string, exitCode int, err error)
}

// OrchestratorFindingsStore provides access to patrol findings for the orchestrator.
type OrchestratorFindingsStore interface {
	Get(id string) *Finding
	Update(f *Finding) bool
}

// OrchestratorApprovalStore queues fixes for human approval.
type OrchestratorApprovalStore interface {
	Create(approval *OrchestratorApproval) error
}

// OrchestratorInfraContextProvider provides discovered infrastructure context.
type OrchestratorInfraContextProvider interface {
	GetInfrastructureContext() string
}

// OrchestratorAutonomyProvider provides the current autonomy level.
type OrchestratorAutonomyProvider interface {
	GetCurrentAutonomyLevel() string
	IsFullModeUnlocked() bool
}

// OrchestratorFixVerifier verifies that a fix resolved the issue.
type OrchestratorFixVerifier interface {
	VerifyFixResolved(ctx context.Context, finding *Finding) (bool, error)
}

// OrchestratorLicenseChecker provides license feature checking.
type OrchestratorLicenseChecker interface {
	HasFeature(feature string) bool
}

// OrchestratorMetricsCallback receives metrics events from the orchestrator.
type OrchestratorMetricsCallback interface {
	RecordInvestigationOutcome(outcome string)
	RecordFixVerification(result string)
}

// ---------------------------------------------------------------------------
// Supporting types for orchestrator interfaces
// ---------------------------------------------------------------------------

// OrchestratorChatSession represents a chat session.
type OrchestratorChatSession struct {
	ID string `json:"id"`
}

// OrchestratorExecuteRequest represents a chat execution request.
type OrchestratorExecuteRequest struct {
	Prompt         string `json:"prompt"`
	SessionID      string `json:"session_id,omitempty"`
	MaxTurns       int    `json:"max_turns,omitempty"`
	AutonomousMode *bool  `json:"autonomous_mode,omitempty"`
}

// OrchestratorStreamCallback is called for each streaming event.
type OrchestratorStreamCallback func(event OrchestratorStreamEvent)

// OrchestratorStreamEvent represents a streaming event.
type OrchestratorStreamEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// OrchestratorMessage represents a chat message.
type OrchestratorMessage struct {
	ID               string                      `json:"id"`
	Role             string                      `json:"role"`
	Content          string                      `json:"content"`
	ReasoningContent string                      `json:"reasoning_content,omitempty"`
	ToolCalls        []OrchestratorToolCallInfo  `json:"tool_calls"`
	ToolResult       *OrchestratorToolResultInfo `json:"tool_result,omitempty"`
	Timestamp        time.Time                   `json:"timestamp"`
}

func EmptyOrchestratorMessage() OrchestratorMessage {
	return OrchestratorMessage{}.NormalizeCollections()
}

func (m OrchestratorMessage) NormalizeCollections() OrchestratorMessage {
	if m.ToolCalls == nil {
		m.ToolCalls = []OrchestratorToolCallInfo{}
	}
	for i := range m.ToolCalls {
		m.ToolCalls[i] = m.ToolCalls[i].NormalizeCollections()
	}
	return m
}

// OrchestratorToolCallInfo represents a tool invocation.
type OrchestratorToolCallInfo struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

func EmptyOrchestratorToolCallInfo() OrchestratorToolCallInfo {
	return OrchestratorToolCallInfo{}.NormalizeCollections()
}

func (t OrchestratorToolCallInfo) NormalizeCollections() OrchestratorToolCallInfo {
	if t.Input == nil {
		t.Input = map[string]interface{}{}
	}
	return t
}

// OrchestratorToolResultInfo represents the result of a tool invocation.
type OrchestratorToolResultInfo struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// OrchestratorApproval represents a queued approval request.
type OrchestratorApproval struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	FindingID   string    `json:"finding_id"`
	SessionID   string    `json:"session_id"`
	Description string    `json:"description"`
	TargetHost  string    `json:"target_host,omitempty"`
	Command     string    `json:"command"`
	RiskLevel   string    `json:"risk_level"`
	CreatedAt   time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// OrchestratorDeps bundles all dependencies needed to construct an
// investigation orchestrator. The OSS side resolves each field from its
// singletons and passes this struct to the enterprise factory.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// AIFinding / AIFindingsStore — used by FindingsStoreAdapter to bridge between
// the patrol FindingsStore (which uses getter/setter interfaces) and the
// orchestrator's FindingsStore (which uses *Finding structs).
// ---------------------------------------------------------------------------

// OrchestratorAIFinding represents the interface for an AI finding from the ai package.
type OrchestratorAIFinding interface {
	GetID() string
	GetSeverity() string
	GetCategory() string
	GetResourceID() string
	GetResourceName() string
	GetResourceType() string
	GetTitle() string
	GetDescription() string
	GetRecommendation() string
	GetEvidence() string
	GetInvestigationSessionID() string
	GetInvestigationStatus() string
	GetInvestigationOutcome() string
	GetLastInvestigatedAt() *time.Time
	GetInvestigationAttempts() int
	SetInvestigationSessionID(string)
	SetInvestigationStatus(string)
	SetInvestigationOutcome(string)
	SetLastInvestigatedAt(*time.Time)
	SetInvestigationAttempts(int)
}

// OrchestratorAIFindingsStore represents the interface for the AI findings store.
type OrchestratorAIFindingsStore interface {
	Get(id string) OrchestratorAIFinding
	UpdateInvestigation(id, sessionID, status, outcome string, lastInvestigatedAt *time.Time, attempts int) bool
}

// OrchestratorDeps contains all dependencies for constructing an investigation orchestrator.
type OrchestratorDeps struct {
	ChatService   OrchestratorChatService
	CmdExecutor   OrchestratorCommandExecutor
	Store         InvestigationStore
	FindingsStore OrchestratorFindingsStore
	ApprovalStore OrchestratorApprovalStore // may be nil
	Config        InvestigationConfig
	InfraContext  OrchestratorInfraContextProvider // may be nil
	Autonomy      OrchestratorAutonomyProvider
	FixVerifier   OrchestratorFixVerifier
	License       OrchestratorLicenseChecker
	Metrics       OrchestratorMetricsCallback
}
