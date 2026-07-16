package aicontracts

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
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
	GetMessages(ctx context.Context, sessionID string) ([]OrchestratorMessage, error)
	DeleteSession(ctx context.Context, sessionID string) error
	// ExecuteInvestigationStream runs one Patrol investigation under the
	// core-owned investigation execution profile and returns the
	// structured result. There is deliberately no generic execution
	// method, no tool listing outside the investigation projection, and
	// no autonomy field anywhere on this interface: non-interactive
	// operation grants no authority, and enterprise code can neither
	// select a profile nor relax one.
	ExecuteInvestigationStream(ctx context.Context, req OrchestratorInvestigationRequest, callback OrchestratorStreamCallback) (*OrchestratorInvestigationResult, error)
	// ListInvestigationTools names the tools an investigation run
	// offers, projected through the same profile path the run uses.
	ListInvestigationTools(ctx context.Context) []string
}

// OrchestratorInvestigationRequest is one investigation run. Correlation
// identity is injected here by the orchestrator from trusted context; the
// model's proposal tool schema never carries it.
type OrchestratorInvestigationRequest struct {
	SessionID        string   `json:"session_id,omitempty"`
	Prompt           string   `json:"prompt"`
	SystemPrompt     string   `json:"system_prompt,omitempty"`
	MaxTurns         int      `json:"max_turns,omitempty"`
	MaxEvidenceCalls int      `json:"max_evidence_calls,omitempty"`
	ExecutionID      string   `json:"execution_id,omitempty"`
	ProposalID       string   `json:"proposal_id"`
	FindingID        string   `json:"finding_id"`
	InvestigationID  string   `json:"investigation_id"`
	EvidenceIDs      []string `json:"evidence_ids,omitempty"`
}

// OrchestratorInvestigationResult is the structured outcome of one
// investigation run. Proposal cardinality is first-class: consumers never
// reconstruct proposals from session messages. A non-nil Proposal is
// immutable, canonically valid, fully correlated, exposure-safe, and
// produced only by a completely successful run - ready for
// OrchestratorActionBroker.Submit.
type OrchestratorInvestigationResult struct {
	Content                string          `json:"content"`
	Proposal               *ActionProposal `json:"proposal,omitempty"`
	FailedProposalAttempts int             `json:"failed_proposal_attempts,omitempty"`
	InputTokens            int             `json:"input_tokens"`
	OutputTokens           int             `json:"output_tokens"`
	ModelTurns             int             `json:"model_turns"`
	EvidenceCalls          int             `json:"evidence_calls"`
	ToolCalls              int             `json:"tool_calls"`
}

// Typed investigation proposal errors surfaced across the contract
// boundary. Any of them means the run produced no actionable proposal.
var (
	ErrInvestigationProposalAmbiguous      = errors.New("ambiguous investigation result: multiple distinct action proposals were submitted")
	ErrInvestigationProposalIntegrity      = errors.New("proposal integrity violation: one tool-use id submitted conflicting payloads")
	ErrInvestigationProposalAttemptsFailed = errors.New("investigation made proposal attempts but none validated")
)

// OrchestratorInvestigationError preserves the independent runtime and
// proposal-channel failures across the Pulse/Enterprise boundary. A proposal
// failure may be handled as a completed needs-attention outcome only when
// RunFailure is nil.
type OrchestratorInvestigationError struct {
	runFailure      error
	proposalFailure error
}

// NewOrchestratorInvestigationError constructs the public cross-repo failure
// without exposing mutable error fields.
func NewOrchestratorInvestigationError(runFailure, proposalFailure error) error {
	if runFailure == nil && proposalFailure == nil {
		return nil
	}
	return &OrchestratorInvestigationError{
		runFailure:      runFailure,
		proposalFailure: proposalFailure,
	}
}

func (e *OrchestratorInvestigationError) Error() string {
	if e == nil {
		return ""
	}
	return errors.Join(e.runFailure, e.proposalFailure).Error()
}

// Unwrap preserves errors.Is/errors.As behavior for both failure channels.
func (e *OrchestratorInvestigationError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return []error{e.runFailure, e.proposalFailure}
}

// RunFailure returns the provider/runtime failure, if any.
func (e *OrchestratorInvestigationError) RunFailure() error {
	if e == nil {
		return nil
	}
	return e.runFailure
}

// ProposalFailure returns the proposal-channel failure, if any.
func (e *OrchestratorInvestigationError) ProposalFailure() error {
	if e == nil {
		return nil
	}
	return e.proposalFailure
}

// OrchestratorFindingsStore provides access to patrol findings for the orchestrator.
type OrchestratorFindingsStore interface {
	Get(id string) *Finding
	Update(f *Finding) bool
}

// OrchestratorInfraContextProvider provides discovered infrastructure context.
type OrchestratorInfraContextProvider interface {
	GetInfrastructureContext() string
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
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Input            map[string]interface{} `json:"input"`
	ThoughtSignature json.RawMessage        `json:"thought_signature,omitempty"`
}

func EmptyOrchestratorToolCallInfo() OrchestratorToolCallInfo {
	return OrchestratorToolCallInfo{}.NormalizeCollections()
}

func (t OrchestratorToolCallInfo) NormalizeCollections() OrchestratorToolCallInfo {
	return OrchestratorToolCallInfoFromProvider(t.ProviderToolCall())
}

// OrchestratorToolCallInfoFromProvider projects the shared provider-facing
// tool-call shape into the public orchestrator contract without duplicating
// Pulse Intelligence normalization rules.
func OrchestratorToolCallInfoFromProvider(tc agentcapabilities.ProviderToolCall) OrchestratorToolCallInfo {
	tc = tc.NormalizeCollections()
	return OrchestratorToolCallInfo{
		ID:               tc.ID,
		Name:             tc.Name,
		Input:            tc.Input,
		ThoughtSignature: tc.ThoughtSignature,
	}
}

// ProviderToolCall returns the shared provider-facing shape used by Assistant
// and MCP-facing Pulse Intelligence tool projections.
func (t OrchestratorToolCallInfo) ProviderToolCall() agentcapabilities.ProviderToolCall {
	return agentcapabilities.ProviderToolCall{
		ID:               t.ID,
		Name:             t.Name,
		Input:            t.Input,
		ThoughtSignature: t.ThoughtSignature,
	}.NormalizeCollections()
}

// OrchestratorToolResultInfo represents the result of a tool invocation.
type OrchestratorToolResultInfo struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// OrchestratorToolResultInfoFromProvider projects the shared provider-facing
// result shape into the public orchestrator contract.
func OrchestratorToolResultInfoFromProvider(result agentcapabilities.ProviderToolResult) OrchestratorToolResultInfo {
	return OrchestratorToolResultInfo{
		ToolUseID: result.ToolUseID,
		Content:   result.Content,
		IsError:   result.IsError,
	}
}

// ProviderToolResult returns the shared provider-facing result shape used by
// Assistant and MCP-facing Pulse Intelligence tool projections.
func (r OrchestratorToolResultInfo) ProviderToolResult() agentcapabilities.ProviderToolResult {
	return agentcapabilities.NewProviderToolResult(r.ToolUseID, r.Content, r.IsError)
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
	Store         InvestigationStore
	FindingsStore OrchestratorFindingsStore
	// ActionBroker is the typed, plan-only proposal seam into the core
	// action lifecycle - the ONLY route from an investigation to an
	// infrastructure mutation. Tenant-bound and actor-stamped by the
	// core adapter. REQUIRED: a missing broker disables the
	// orchestrator; there is no command-execution fallback.
	ActionBroker OrchestratorActionBroker
	Config       InvestigationConfig
	InfraContext OrchestratorInfraContextProvider // may be nil
	Metrics      OrchestratorMetricsCallback
}
