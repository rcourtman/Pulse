package api

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// AgentResourceFindingSnapshot is the agent-consumable projection of a
// finding. Intentionally narrower than the full Finding shape — agents
// reasoning about a resource need the situation, not every internal
// flag. Carries the seven-question schema essentials (title, severity,
// impact, recommendation, confidence, regression count, previous fix)
// plus identity. Agents that need deeper detail can fetch the full
// finding via the existing finding endpoints.
type AgentResourceFindingSnapshot struct {
	ID                         string `json:"id"`
	Title                      string `json:"title"`
	Severity                   string `json:"severity"`
	Category                   string `json:"category,omitempty"`
	Description                string `json:"description,omitempty"`
	Impact                     string `json:"impact,omitempty"`
	Recommendation             string `json:"recommendation,omitempty"`
	Confidence                 string `json:"confidence,omitempty"`
	RegressionCount            int    `json:"regressionCount"`
	PreviousResolvedFixSummary string `json:"previousResolvedFixSummary,omitempty"`
	DetectedAt                 string `json:"detectedAt,omitempty"`
	LastSeenAt                 string `json:"lastSeenAt,omitempty"`
}

// AgentResourceActionVerification is the agent-stable projection of
// the post-execution read-after-write probe the broker runs after
// every successful dispatch. Carries enough for an agent to close
// the certainty loop ("did it actually work?") without fetching
// /api/actions/{id} for full output. `Ran` is the existence bit:
// false means no verification was attempted (the action class has
// no derivable check, or the dispatch failed before verification
// could run). Output is intentionally omitted from the projection
// — verification stdout can be large; agents that need it follow
// up via the audit endpoint. Verification command follows the same
// redaction rule as action commands.
type AgentResourceActionVerification struct {
	Ran             bool      `json:"ran"`
	Success         bool      `json:"success"`
	Command         string    `json:"command,omitempty"`
	CommandRedacted bool      `json:"commandRedacted,omitempty"`
	Note            string    `json:"note,omitempty"`
	RanAt           time.Time `json:"ranAt,omitempty"`
}

// AgentResourceActionSummary is the agent-consumable projection of an
// action audit record. Includes the refusal-reason prefix tokens
// (resource_remediation_locked:, plan_drift:) verbatim so agents can
// branch on them without parsing the human message. Command is present
// only for session callers or API tokens with action execution scope;
// monitoring-read tokens receive commandRedacted instead.
type AgentResourceActionSummary struct {
	ID              string                           `json:"id"`
	CapabilityName  string                           `json:"capabilityName"`
	Command         string                           `json:"command,omitempty"`
	CommandRedacted bool                             `json:"commandRedacted,omitempty"`
	State           string                           `json:"state"`
	Success         bool                             `json:"success"`
	ErrorMessage    string                           `json:"errorMessage,omitempty"`
	Verification    *AgentResourceActionVerification `json:"verification,omitempty"`
	RequestedBy     string                           `json:"requestedBy,omitempty"`
	CreatedAt       time.Time                        `json:"createdAt"`
	UpdatedAt       time.Time                        `json:"updatedAt"`
}

// AgentResourceApprovalSummary is the agent-consumable projection of
// a pending approval request scoped to a specific resource. Carries
// just enough for an agent that holds approval authority to decide
// whether to act, fetch full context via /api/approvals/{id}, or
// escalate. Command is present only for session callers or API tokens
// with action execution scope; monitoring-read tokens receive
// commandRedacted instead. Mirrors the shape of approval.pending SSE
// events so "what's pending right now" (this bundle) and "what just
// became pending" (the SSE stream) speak the same vocabulary.
type AgentResourceApprovalSummary struct {
	ID              string    `json:"id"`
	Command         string    `json:"command,omitempty"`
	CommandRedacted bool      `json:"commandRedacted,omitempty"`
	RiskLevel       string    `json:"riskLevel"`
	RequestedBy     string    `json:"requestedBy,omitempty"`
	RequestedAt     time.Time `json:"requestedAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

// AgentFleetFindingCounts is the per-severity finding rollup carried
// on each fleet-context entry. Keys are the canonical severity
// strings agents already branch on elsewhere (`critical`, `warning`,
// `info`). `total` is the sum so agents can sort/triage without
// summing client-side.
type AgentFleetFindingCounts struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

// AgentFleetResourceSummary is the per-resource thin rollup an agent
// uses for triage. Carries identity, the operator-intent flags that
// gate auto-action (intentionallyOffline, neverAutoRemediate,
// maintenanceWindowActive), per-severity finding counts, and the
// pending-approval count. Designed to be light enough for a one-read
// fleet sweep — agents that want depth on a flagged resource follow
// up via /api/agent/resource-context/{id}.
type AgentFleetResourceSummary struct {
	CanonicalID             string                  `json:"canonicalId"`
	ResourceType            string                  `json:"resourceType"`
	ResourceName            string                  `json:"resourceName"`
	Technology              string                  `json:"technology,omitempty"`
	IntentionallyOffline    bool                    `json:"intentionallyOffline"`
	NeverAutoRemediate      bool                    `json:"neverAutoRemediate"`
	MaintenanceWindowActive bool                    `json:"maintenanceWindowActive"`
	Findings                AgentFleetFindingCounts `json:"findings"`
	PendingApprovalCount    int                     `json:"pendingApprovalCount"`
}

// AgentFleetContext is the bundled, agent-consumable triage view
// across every resource visible to the org. One read returns thin
// per-resource summaries — enough for an agent to pick a focus
// before drilling into the per-resource bundle. The list is always
// an array (never null) so agents iterate without nil-checking.
type AgentFleetContext struct {
	Resources   []AgentFleetResourceSummary `json:"resources"`
	GeneratedAt time.Time                   `json:"generatedAt"`
}

type AgentOperationsLoopStepStatus string

const (
	agentOperationsLoopActionWindow = 30 * 24 * time.Hour

	AgentOperationsLoopStepComplete AgentOperationsLoopStepStatus = "complete"
	AgentOperationsLoopStepCurrent  AgentOperationsLoopStepStatus = "current"
	AgentOperationsLoopStepPending  AgentOperationsLoopStepStatus = "pending"
)

// AgentOperationsLoopStep is a content-safe step rollup for the Patrol
// control loop: Patrol issue evidence, Assistant/context explanation,
// governed approve/reject decision, and verified outcome. Counts are aggregate
// only and never carry finding ids, action ids, resource names, commands,
// prompts, or output. External-agent readiness is a separate capability signal,
// not a Patrol operator step.
type AgentOperationsLoopStep struct {
	ID     string                        `json:"id"`
	Label  string                        `json:"label"`
	Status AgentOperationsLoopStepStatus `json:"status"`
	Count  int                           `json:"count,omitempty"`
}

// AgentOperationsLoopStatus is the agent-consumable status projection for the
// same Pulse Intelligence loop shown in the native Patrol control journey. It
// exists so external MCP agents can orient on the operator stage without
// reverse-engineering it from fleet context, finding lists, approvals, and
// action audits. The payload is intentionally count-only. MCP readiness stays
// separate from steps so optional external access does not look like a required
// user journey stage.
type AgentOperationsLoopStatus struct {
	NextAction                          string                    `json:"nextAction"`
	ProgressLabel                       string                    `json:"progressLabel"`
	Steps                               []AgentOperationsLoopStep `json:"steps"`
	PatrolEvidenceCount                 int                       `json:"patrolEvidenceCount"`
	PatrolIssueEvidenceCount            int                       `json:"patrolIssueEvidenceCount"`
	ActiveFindingCount                  int                       `json:"activeFindingCount"`
	PendingApprovalCount                int                       `json:"pendingApprovalCount"`
	GovernedActionCount                 int                       `json:"governedActionCount"`
	ApprovedDecisionCount               int                       `json:"approvedDecisionCount"`
	RejectedDecisionCount               int                       `json:"rejectedDecisionCount"`
	VerifiedOutcomeCount                int                       `json:"verifiedOutcomeCount"`
	OperationsLoopStarterCount          int                       `json:"operationsLoopStarterCount"`
	AssistantOperationsLoopStarterCount int                       `json:"assistantOperationsLoopStarterCount"`
	PatrolOperationsLoopStarterCount    int                       `json:"patrolOperationsLoopStarterCount"`
	PatrolControlLoopStarterCount       int                       `json:"patrolControlOperationsLoopStarterCount"`
	PatrolControlCompletedLoopCount     int                       `json:"patrolControlCompletedOperationsLoopCount"`
	PatrolControlResolvedLoopCount      int                       `json:"patrolControlResolvedOperationsLoopCount"`
	PatrolControlValueState             string                    `json:"patrolControlValueState"`
	PatrolAutonomyLoopStarterCount      int                       `json:"patrolAutonomyOperationsLoopStarterCount"`
	PatrolAutonomyCompletedLoopCount    int                       `json:"patrolAutonomyCompletedOperationsLoopCount"`
	PatrolAutonomyResolvedLoopCount     int                       `json:"patrolAutonomyResolvedOperationsLoopCount"`
	PatrolAutonomyValueState            string                    `json:"patrolAutonomyValueState"`
	ProActivationLoopStarterCount       int                       `json:"proActivationOperationsLoopStarterCount"`
	ProActivationCompletedLoopCount     int                       `json:"proActivationCompletedOperationsLoopCount"`
	ProActivationResolvedLoopCount      int                       `json:"proActivationResolvedOperationsLoopCount"`
	ProActivationValueProofState        string                    `json:"proActivationValueProofState"`
	MCPOperationsLoopStarterCount       int                       `json:"mcpOperationsLoopStarterCount"`
	ExternalAgentReady                  bool                      `json:"externalAgentReady"`
	WindowStart                         time.Time                 `json:"windowStart"`
	GeneratedAt                         time.Time                 `json:"generatedAt"`

	assistantContextCount           int
	externalAgentCollaborationCount int
}

// AgentResourceOperatorState mirrors the canonical
// `unified.ResourceOperatorState` but with the same JSON shape the
// `/api/resources/{id}/operator-state` endpoint already returns. Kept
// as a separate type so the bundle's JSON can be agent-stable even if
// the underlying store type's JSON tags shift.
type AgentResourceOperatorState struct {
	IntentionallyOffline bool       `json:"intentionallyOffline"`
	NeverAutoRemediate   bool       `json:"neverAutoRemediate"`
	MaintenanceStartAt   *time.Time `json:"maintenanceStartAt,omitempty"`
	MaintenanceEndAt     *time.Time `json:"maintenanceEndAt,omitempty"`
	MaintenanceReason    string     `json:"maintenanceReason,omitempty"`
	Criticality          string     `json:"criticality,omitempty"`
	Note                 string     `json:"note,omitempty"`
	SetAt                time.Time  `json:"setAt"`
	SetBy                string     `json:"setBy,omitempty"`
	// MaintenanceWindowActive reports whether a window covers `now` —
	// computed once on the server so agents don't need to re-evaluate
	// the start/end timestamps client-side.
	MaintenanceWindowActive bool `json:"maintenanceWindowActive"`
}

// AgentResourceContextFact is a bounded, typed fact in the richer
// agent-consumable context pack. Values come from Pulse-owned runtime
// state only; raw command output, config files, environment variables,
// and secret-bearing metadata do not belong here.
type AgentResourceContextFact = agentcontext.Fact

// AgentResourceContextRedaction records why a context section withheld
// a field. It gives agents and UI inspectors a visible safety boundary
// without leaking the underlying value.
type AgentResourceContextRedaction = agentcontext.Redaction

// AgentResourceContextSection groups related facts with one
// provenance/freshness envelope. Additive sections make the context
// pack useful to Assistant and external MCP clients without breaking
// older clients that parse the original top-level fields.
type AgentResourceContextSection = agentcontext.Section

// AgentResourceDiscoveryReadiness is the agent-stable projection of
// service-discovery freshness already carried on resource API payloads.
type AgentResourceDiscoveryReadiness = unified.ResourceDiscoveryReadiness

// AgentResourceContext is the bundled, agent-consumable situated
// picture of a resource. One read returns:
//   - core identity (canonical id, type, name)
//   - operator-set state, with computed maintenance-window-active flag
//   - active findings (lightweight snapshot of the seven-question schema)
//   - recent action attempts (including refused dispatches with their
//     stable error tokens)
//   - additive context sections with provenance, freshness, and
//     redaction metadata for richer Assistant and external-agent
//     grounding
//
// The endpoint trades coverage for shape: an agent gets enough to
// reason about the next move without chaining several calls. Deeper
// detail (full finding records, full audit history) remains available
// via the existing per-finding / per-audit endpoints.
type AgentResourceContext struct {
	CanonicalID        string                           `json:"canonicalId"`
	ResourceType       string                           `json:"resourceType"`
	ResourceName       string                           `json:"resourceName"`
	Technology         string                           `json:"technology,omitempty"`
	OperatorState      *AgentResourceOperatorState      `json:"operatorState,omitempty"`
	DiscoveryReadiness *AgentResourceDiscoveryReadiness `json:"discoveryReadiness,omitempty"`
	ActiveFindings     []AgentResourceFindingSnapshot   `json:"activeFindings"`
	PendingApprovals   []AgentResourceApprovalSummary   `json:"pendingApprovals"`
	RecentActions      []AgentResourceActionSummary     `json:"recentActions"`
	ContextSections    []AgentResourceContextSection    `json:"contextSections"`
	GeneratedAt        time.Time                        `json:"generatedAt"`
}

// AgentFindingsProvider returns active findings as agent-stable snapshots and
// count-only aggregate evidence. The implementation lives outside this package
// (the patrol service holds the canonical findings store); this interface keeps
// the api layer free of an `internal/ai` import.
type AgentFindingsProvider interface {
	ActiveFindingsForResource(resourceID string) []AgentResourceFindingSnapshot
}

type AgentAggregateFindingsProvider interface {
	ActiveFindingCount() int
}

type agentFindingsProvider struct {
	activeForResource func(resourceID string) []AgentResourceFindingSnapshot
	activeCount       func() int
}

func (p agentFindingsProvider) ActiveFindingsForResource(resourceID string) []AgentResourceFindingSnapshot {
	if p.activeForResource == nil {
		return nil
	}
	return p.activeForResource(resourceID)
}

func (p agentFindingsProvider) ActiveFindingCount() int {
	if p.activeCount == nil {
		return 0
	}
	return p.activeCount()
}

// agentFindingsProviderFunc is a function adapter so wire-up code can
// pass a closure without declaring a struct.
type agentFindingsProviderFunc func(resourceID string) []AgentResourceFindingSnapshot

// ActiveFindingsForResource implements AgentFindingsProvider.
func (f agentFindingsProviderFunc) ActiveFindingsForResource(resourceID string) []AgentResourceFindingSnapshot {
	if f == nil {
		return nil
	}
	return f(resourceID)
}

// AgentApprovalsProvider returns still-pending approvals as
// agent-stable projections. Per-resource context needs full
// summaries; fleet context needs only a resource-keyed count map so
// it can aggregate approval pressure without scanning the global
// approval list once per resource. The implementation lives outside
// this package (the approval store is owned by the AIHandler); this
// interface keeps the bundle handler decoupled from the global
// approval store and lets tests pass a fake.
type AgentApprovalsProvider interface {
	PendingApprovalsForResource(resourceID, orgID string) []AgentResourceApprovalSummary
	PendingApprovalCountsByResource(orgID string) map[string]int
}

// agentApprovalStoreProvider adapts the process-global approval
// store into the agent context provider contract. It resolves the
// store at request time so multi-tenant rebuilds that install a new
// global store via approval.SetStore are honored without re-wiring
// the router.
type agentApprovalStoreProvider struct{}

func (agentApprovalStoreProvider) PendingApprovalsForResource(resourceID, orgID string) []AgentResourceApprovalSummary {
	return pendingApprovalsForResourceFromStore(approval.GetStore(), resourceID, orgID)
}

func (agentApprovalStoreProvider) PendingApprovalCountsByResource(orgID string) map[string]int {
	return pendingApprovalCountsByResourceFromStore(approval.GetStore(), orgID)
}

// AgentContextHandler owns the agent-paradigm bundled context
// endpoint. Kept as a separate type from `ResourceHandlers` so the
// agent surface evolves independently of the resource CRUD surface
// and the resource handler stays focused on its existing concerns.
// The handler reuses the resource handler for registry + store
// access (those are the canonical accessors); the
// agent-findings adapter is held here.
type AgentContextHandler struct {
	resources                      *ResourceHandlers
	findingsProvider               AgentFindingsProvider
	approvalsProvider              AgentApprovalsProvider
	workflowPromptActivityProvider AgentWorkflowPromptActivityProvider
	aiUsageProvider                AgentAIUsageProvider
	externalAgentActivityProvider  AgentExternalAgentActivityProvider
	externalAgentReadinessProvider AgentExternalAgentReadinessProvider
}

// AgentWorkflowPromptActivityProvider loads content-free workflow prompt
// starter activity for the current request context. Implementations are
// responsible for tenant resolution so the agent status projection can stay
// scoped to the authenticated org.
type AgentWorkflowPromptActivityProvider interface {
	WorkflowPromptActivityHistory(ctx context.Context) (*config.WorkflowPromptActivityHistoryData, error)
}

type agentWorkflowPromptActivityProviderFunc func(context.Context) (*config.WorkflowPromptActivityHistoryData, error)

func (fn agentWorkflowPromptActivityProviderFunc) WorkflowPromptActivityHistory(ctx context.Context) (*config.WorkflowPromptActivityHistoryData, error) {
	return fn(ctx)
}

// AgentAIUsageProvider loads content-free AI usage evidence for the operations
// loop status projection.
type AgentAIUsageProvider interface {
	AIUsageHistory(ctx context.Context) (*config.AIUsageHistoryData, error)
}

type agentAIUsageProviderFunc func(context.Context) (*config.AIUsageHistoryData, error)

func (fn agentAIUsageProviderFunc) AIUsageHistory(ctx context.Context) (*config.AIUsageHistoryData, error) {
	return fn(ctx)
}

// AgentExternalAgentActivityProvider loads content-free external-agent
// capability activity for the operations loop status projection.
type AgentExternalAgentActivityProvider interface {
	ExternalAgentActivityHistory(ctx context.Context) (*config.ExternalAgentActivityHistoryData, error)
}

type agentExternalAgentActivityProviderFunc func(context.Context) (*config.ExternalAgentActivityHistoryData, error)

func (fn agentExternalAgentActivityProviderFunc) ExternalAgentActivityHistory(ctx context.Context) (*config.ExternalAgentActivityHistoryData, error) {
	return fn(ctx)
}

// AgentExternalAgentReadinessProvider reports whether the current org has one
// non-expired token that can use the full Pulse MCP operations-loop tool set.
// The status payload exposes only the boolean, never token identity or counts.
type AgentExternalAgentReadinessProvider interface {
	ExternalAgentReady(ctx context.Context, manifest agentcapabilities.Manifest, now time.Time) bool
}

type agentExternalAgentReadinessProviderFunc func(context.Context, agentcapabilities.Manifest, time.Time) bool

func (fn agentExternalAgentReadinessProviderFunc) ExternalAgentReady(ctx context.Context, manifest agentcapabilities.Manifest, now time.Time) bool {
	return fn(ctx, manifest, now)
}

// NewAgentContextHandler creates a new agent context handler. The
// resource handler is required (registry + store come from it); the
// findings provider may be set later via SetFindingsProvider.
func NewAgentContextHandler(resources *ResourceHandlers) *AgentContextHandler {
	return &AgentContextHandler{resources: resources}
}

// SetFindingsProvider wires the active-findings adapter. Pass nil to
// disable the active-findings section of the bundle.
func (h *AgentContextHandler) SetFindingsProvider(p AgentFindingsProvider) {
	h.findingsProvider = p
}

// SetApprovalsProvider wires the pending-approvals adapter. Pass nil
// to disable the pending-approvals section of the bundle (callers
// will always see `pendingApprovals: []` rather than a missing field
// so agents can iterate without nil-checking).
func (h *AgentContextHandler) SetApprovalsProvider(p AgentApprovalsProvider) {
	h.approvalsProvider = p
}

// SetWorkflowPromptActivityProvider wires the content-free workflow starter
// activity source used by the operations-loop status projection. Pass nil to
// omit starter counts without affecting Patrol, governance, or verification
// evidence.
func (h *AgentContextHandler) SetWorkflowPromptActivityProvider(p AgentWorkflowPromptActivityProvider) {
	h.workflowPromptActivityProvider = p
}

// SetAIUsageProvider wires content-free Assistant usage evidence into the
// operations-loop status projection.
func (h *AgentContextHandler) SetAIUsageProvider(p AgentAIUsageProvider) {
	h.aiUsageProvider = p
}

// SetExternalAgentActivityProvider wires content-free external-agent activity
// evidence into the operations-loop status projection.
func (h *AgentContextHandler) SetExternalAgentActivityProvider(p AgentExternalAgentActivityProvider) {
	h.externalAgentActivityProvider = p
}

// SetExternalAgentReadinessProvider wires the token-backed external-agent
// setup signal used by the operations-loop status projection.
func (h *AgentContextHandler) SetExternalAgentReadinessProvider(p AgentExternalAgentReadinessProvider) {
	h.externalAgentReadinessProvider = p
}

// HandleResourceContext serves
// `GET /api/agent/resource-context/{id}` — the agent-consumable
// situated picture of a resource. Bundle is computed at request time
// (no caching) so agents always see live state. Limits on the
// recent-actions slice keep the payload bounded; deeper history is
// available via the existing per-resource action audit endpoints.
func (h *AgentContextHandler) HandleResourceContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.resources == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	resourceID := extractAgentResourceContextID(r.URL.Path)
	if resourceID == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.resources.buildRegistry(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	resource, resourceID, ok := presentationResourceByReference(registry, resourceID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, agentcapabilities.AgentErrCodeResourceNotFound,
			"No resource is registered with this canonical id.")
		return
	}

	store, err := h.resources.getStore(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}

	generatedAt := time.Now().UTC()
	resourceCopy := *resource
	attachDiscoveryTarget(&resourceCopy)
	h.resources.attachDiscoveryReadiness(&resourceCopy, generatedAt)

	bundle := AgentResourceContext{
		CanonicalID:        resourceID,
		ResourceType:       string(resourceCopy.Type),
		ResourceName:       resourceCopy.Name,
		Technology:         resourceCopy.Technology,
		DiscoveryReadiness: resourceCopy.DiscoveryReadiness,
		ActiveFindings:     []AgentResourceFindingSnapshot{},
		PendingApprovals:   []AgentResourceApprovalSummary{},
		RecentActions:      []AgentResourceActionSummary{},
		ContextSections:    []AgentResourceContextSection{},
		GeneratedAt:        generatedAt,
	}

	// Operator-set state — single point lookup.
	if state, found, opErr := store.GetResourceOperatorState(resourceID); opErr == nil && found {
		projected := projectAgentResourceOperatorState(state, bundle.GeneratedAt)
		bundle.OperatorState = &projected
	}
	// Operator-state lookup errors are logged-and-ignored: the bundle
	// is still useful without it, and an agent can branch on the
	// `operatorState` field being absent.

	// Active findings — in-memory lookup via the patrol findings store
	// adapter wired at startup.
	if h.findingsProvider != nil {
		bundle.ActiveFindings = h.findingsProvider.ActiveFindingsForResource(resourceID)
		if bundle.ActiveFindings == nil {
			bundle.ActiveFindings = []AgentResourceFindingSnapshot{}
		}
	}

	// Pending approvals — in-memory filter against the approval
	// store, scoped to this org. Same pattern as findings: the
	// provider adapter keeps the api package free of approval-store
	// internals at the bundle layer; the wire-up step in router.go
	// does the global lookup and resource-id filter.
	if h.approvalsProvider != nil {
		bundle.PendingApprovals = h.approvalsProvider.PendingApprovalsForResource(resourceID, orgID)
		if bundle.PendingApprovals == nil {
			bundle.PendingApprovals = []AgentResourceApprovalSummary{}
		}
	}

	// Recent action audits — limit to 10 by default; agents that need
	// deeper history can call the existing per-resource action audit
	// endpoint. Filter window of "since one week ago" mirrors the
	// frontend-side ResourceActionHistory default.
	since := bundle.GeneratedAt.Add(-7 * 24 * time.Hour)
	if audits, auditErr := store.GetActionAudits(resourceID, since, 10); auditErr == nil {
		bundle.RecentActions = projectAgentResourceActions(audits)
		if bundle.RecentActions == nil {
			bundle.RecentActions = []AgentResourceActionSummary{}
		}
	}

	bundle.ContextSections = buildAgentResourceContextSections(resourceCopy, store, bundle)

	redactAgentResourceContextCommandsForRequest(&bundle, r)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bundle)
}

func buildAgentResourceContextSections(resource unified.Resource, store unified.ResourceStore, bundle AgentResourceContext) []AgentResourceContextSection {
	return agentcontext.BuildResourceContextSections(resource, store, agentcontext.BuildOptions{
		GeneratedAt:          bundle.GeneratedAt,
		OperatorState:        agentResourceOperatorStateForContext(bundle.OperatorState),
		ActiveFindingCount:   len(bundle.ActiveFindings),
		PendingApprovalCount: len(bundle.PendingApprovals),
		RecentActionCount:    len(bundle.RecentActions),
	})
}

func agentResourceOperatorStateForContext(state *AgentResourceOperatorState) *agentcontext.OperatorState {
	if state == nil {
		return nil
	}
	return &agentcontext.OperatorState{
		IntentionallyOffline:    state.IntentionallyOffline,
		NeverAutoRemediate:      state.NeverAutoRemediate,
		MaintenanceWindowActive: state.MaintenanceWindowActive,
		MaintenanceStartAt:      state.MaintenanceStartAt,
		MaintenanceEndAt:        state.MaintenanceEndAt,
		Criticality:             state.Criticality,
		NotePresent:             strings.TrimSpace(state.Note) != "",
		SetAt:                   state.SetAt,
	}
}

// extractAgentResourceContextID parses the canonical resource ID out of
// the URL path. Mirrors the trim/canonicalize pattern used elsewhere
// for nested resource routes.
func extractAgentResourceContextID(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/agent/resource-context/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return unified.CanonicalResourceID(trimmed)
}

// HandleFleetContext serves
// `GET /api/agent/fleet-context` — the agent-consumable triage view
// across every resource visible to the org. Each entry is a thin
// rollup (identity + operator flags + per-severity finding counts +
// pending-approval count) so a single read tells an agent where to
// focus. Agents that want depth on a flagged resource follow up via
// /api/agent/resource-context/{id}.
//
// Computed at request time (no caching); the registry walk is the
// dominant cost. Per-resource costs are bounded: one operator-state
// SQLite point lookup, one in-memory findings lookup via the
// findings store's per-resource index, and a single global
// pending-approvals scan that's grouped by canonical resource id
// before the registry walk so the fleet sweep costs O(N) registry
// reads, not O(N*M) approval scans.
func (h *AgentContextHandler) HandleFleetContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.resources == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	orgID := GetOrgID(r.Context())
	registry, err := h.resources.buildRegistry(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	store, err := h.resources.getStore(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	resources := registry.List()
	out := AgentFleetContext{
		Resources:   make([]AgentFleetResourceSummary, 0, len(resources)),
		GeneratedAt: now,
	}

	// Pre-compute per-resource pending-approval counts in a single
	// pass over the bounded global list, indexed by canonical
	// resource id. Beats N independent provider calls because the
	// bounded approval list (MaxApprovals=100) is the same regardless
	// of fleet size — one scan suffices.
	pendingByResource := map[string]int{}
	if h.approvalsProvider != nil {
		counts := h.approvalsProvider.PendingApprovalCountsByResource(orgID)
		if counts != nil {
			pendingByResource = counts
		}
	}

	for _, resource := range resources {
		canonical := unified.CanonicalResourceID(resource.ID)
		summary := AgentFleetResourceSummary{
			CanonicalID:  canonical,
			ResourceType: string(resource.Type),
			ResourceName: resource.Name,
			Technology:   resource.Technology,
		}
		if state, found, opErr := store.GetResourceOperatorState(canonical); opErr == nil && found {
			summary.IntentionallyOffline = state.IntentionallyOffline
			summary.NeverAutoRemediate = state.NeverAutoRemediate
			summary.MaintenanceWindowActive = state.IsInMaintenanceAt(now)
		}
		if h.findingsProvider != nil {
			findings := h.findingsProvider.ActiveFindingsForResource(canonical)
			summary.Findings = countFleetFindingsBySeverity(findings)
		}
		if count, ok := pendingByResource[canonical]; ok {
			summary.PendingApprovalCount = count
		}
		out.Resources = append(out.Resources, summary)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// HandleOperationsLoopStatus serves the content-safe status read for the shared
// Pulse Intelligence Patrol control loop. The canonical route is
// `GET /api/agent/patrol-control/status`; `GET /api/agent/operations-loop/status`
// remains as a compatibility alias. It deliberately summarizes counts and
// stage state instead of exposing finding ids, action ids, commands, prompts,
// output, or resource names.
func (h *AgentContextHandler) HandleOperationsLoopStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.resources == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.resources.buildRegistry(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	store, err := h.resources.getStore(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	windowStart := now.Add(-agentOperationsLoopActionWindow)
	manifest := agentcapabilities.CanonicalManifest()
	status := AgentOperationsLoopStatus{
		Steps:              []AgentOperationsLoopStep{},
		ExternalAgentReady: h.agentOperationsLoopExternalAgentReady(r.Context(), manifest, now),
		WindowStart:        windowStart,
		GeneratedAt:        now,
	}

	pendingByResource := map[string]int{}
	if h.approvalsProvider != nil {
		if counts := h.approvalsProvider.PendingApprovalCountsByResource(orgID); counts != nil {
			pendingByResource = counts
		}
	}

	for _, resource := range registry.List() {
		canonical := unified.CanonicalResourceID(resource.ID)
		if h.findingsProvider != nil {
			status.ActiveFindingCount += len(h.findingsProvider.ActiveFindingsForResource(canonical))
		}
		if pending := pendingByResource[canonical]; pending > 0 {
			status.PendingApprovalCount += pending
		}
	}
	if aggregateProvider, ok := h.findingsProvider.(AgentAggregateFindingsProvider); ok {
		if aggregateActive := aggregateProvider.ActiveFindingCount(); aggregateActive > status.ActiveFindingCount {
			status.ActiveFindingCount = aggregateActive
		}
	}

	if h.workflowPromptActivityProvider != nil {
		if history, err := h.workflowPromptActivityProvider.WorkflowPromptActivityHistory(r.Context()); err == nil {
			starterCounts := agentOperationsLoopWorkflowStarterEvidenceCounts(history, windowStart)
			status.OperationsLoopStarterCount = starterCounts.total
			status.AssistantOperationsLoopStarterCount = starterCounts.assistant
			status.PatrolOperationsLoopStarterCount = starterCounts.patrol
			status.ProActivationLoopStarterCount = starterCounts.proActivation
			status.PatrolControlLoopStarterCount = starterCounts.patrolControl
			status.PatrolAutonomyLoopStarterCount = status.PatrolControlLoopStarterCount
			status.MCPOperationsLoopStarterCount = starterCounts.mcp
		}
	}
	if h.aiUsageProvider != nil {
		if history, err := h.aiUsageProvider.AIUsageHistory(r.Context()); err == nil {
			aiEvidence := telemetry.PulseIntelligenceAIUsageEvidenceFromHistory(history, windowStart)
			status.assistantContextCount = aiEvidence.AssistantContextAICalls + aiEvidence.AssistantToolCalls
		}
	}
	if h.externalAgentActivityProvider != nil {
		if history, err := h.externalAgentActivityProvider.ExternalAgentActivityHistory(r.Context()); err == nil {
			externalEvidence := telemetry.PulseIntelligenceExternalAgentEvidenceFromHistory(history, windowStart)
			status.externalAgentCollaborationCount = externalEvidence.CollaborationCount()
		}
	}

	actionCounts := agentOperationsLoopActionEvidenceCounts(store, windowStart)
	status.GovernedActionCount = actionCounts.governed
	status.ApprovedDecisionCount = actionCounts.approved
	status.RejectedDecisionCount = actionCounts.rejected
	status.VerifiedOutcomeCount = actionCounts.verified
	status.PatrolEvidenceCount = status.ActiveFindingCount + status.PendingApprovalCount + actionCounts.recent
	status.PatrolIssueEvidenceCount = status.ActiveFindingCount + status.PendingApprovalCount + status.GovernedActionCount + status.VerifiedOutcomeCount
	patrolControlProof := agentOperationsLoopPatrolControlProof(status)
	if patrolControlProof.Completed {
		status.PatrolControlCompletedLoopCount = 1
	}
	if patrolControlProof.Resolved {
		status.PatrolControlResolvedLoopCount = 1
	}
	status.PatrolControlValueState = patrolControlProof.ValueProofState
	status.PatrolAutonomyCompletedLoopCount = status.PatrolControlCompletedLoopCount
	status.PatrolAutonomyResolvedLoopCount = status.PatrolControlResolvedLoopCount
	status.PatrolAutonomyValueState = status.PatrolControlValueState
	status.ProActivationCompletedLoopCount = status.PatrolControlCompletedLoopCount
	status.ProActivationResolvedLoopCount = status.PatrolControlResolvedLoopCount
	status.ProActivationValueProofState = status.PatrolControlValueState
	status.NextAction, status.ProgressLabel, status.Steps = agentOperationsLoopProgress(status)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

// countFleetFindingsBySeverity tallies the per-severity counts an
// agent uses to triage. Severity strings outside the canonical set
// (`critical`, `warning`, `info`) are counted in `Total` but not in
// any bucket — defensive against ad-hoc severity values, but the
// schema discourages them.
func countFleetFindingsBySeverity(findings []AgentResourceFindingSnapshot) AgentFleetFindingCounts {
	counts := AgentFleetFindingCounts{}
	for _, f := range findings {
		counts.Total++
		switch strings.ToLower(strings.TrimSpace(f.Severity)) {
		case "critical":
			counts.Critical++
		case "warning":
			counts.Warning++
		case "info":
			counts.Info++
		}
	}
	return counts
}

type agentOperationsLoopActionCounts struct {
	recent   int
	governed int
	approved int
	rejected int
	verified int
}

type agentOperationsLoopWorkflowStarterCounts struct {
	total         int
	assistant     int
	patrol        int
	patrolControl int
	proActivation int
	mcp           int
}

func agentOperationsLoopWorkflowStarterEvidenceCounts(history *config.WorkflowPromptActivityHistoryData, since time.Time) agentOperationsLoopWorkflowStarterCounts {
	var counts agentOperationsLoopWorkflowStarterCounts
	if history == nil {
		return counts
	}
	for _, event := range history.Events {
		if strings.TrimSpace(event.PromptName) != agentcapabilities.PulseWorkflowPromptOperationsLoop {
			continue
		}
		if event.Timestamp.Before(since) {
			continue
		}
		counts.total++
		switch strings.TrimSpace(event.Surface) {
		case config.WorkflowPromptActivitySurfacePulseAssistant:
			counts.assistant++
		case config.WorkflowPromptActivitySurfacePulsePatrol:
			counts.patrol++
			counts.patrolControl++
		case config.WorkflowPromptActivitySurfacePatrolControl:
			counts.patrolControl++
		case config.WorkflowPromptActivitySurfacePatrolAutonomy:
			counts.patrolControl++
		case config.WorkflowPromptActivitySurfaceProActivation:
			counts.proActivation++
			counts.patrolControl++
		case config.WorkflowPromptActivitySurfacePulseMCP:
			counts.mcp++
		}
	}
	return counts
}

func agentOperationsLoopActionEvidenceCounts(store unified.ResourceStore, since time.Time) agentOperationsLoopActionCounts {
	var counts agentOperationsLoopActionCounts
	if store == nil {
		return counts
	}
	recentIDs := map[string]struct{}{}
	governedIDs := map[string]struct{}{}
	approvedIDs := map[string]struct{}{}
	rejectedIDs := map[string]struct{}{}
	verifiedIDs := map[string]struct{}{}
	auditsByID := map[string]unified.ActionAuditRecord{}
	cacheAudit := func(audit unified.ActionAuditRecord) {
		actionID := strings.TrimSpace(audit.ID)
		if actionID == "" {
			return
		}
		auditsByID[actionID] = audit
	}
	addAuditEvidence := func(audit unified.ActionAuditRecord) {
		actionID := strings.TrimSpace(audit.ID)
		if actionID == "" {
			return
		}
		recentIDs[actionID] = struct{}{}
		if pulseIntelligenceActionWasApproved(audit) {
			approvedIDs[actionID] = struct{}{}
		}
		if pulseIntelligenceActionWasRejected(audit) {
			rejectedIDs[actionID] = struct{}{}
		}
		if agentOperationsLoopGovernedAction(audit) {
			governedIDs[actionID] = struct{}{}
		}
		if agentOperationsLoopVerifiedAction(audit) {
			verifiedIDs[actionID] = struct{}{}
		}
	}
	auditForID := func(actionID string) (unified.ActionAuditRecord, bool) {
		actionID = strings.TrimSpace(actionID)
		if actionID == "" {
			return unified.ActionAuditRecord{}, false
		}
		if audit, ok := auditsByID[actionID]; ok {
			return audit, true
		}
		audit, ok, err := store.GetActionAudit(actionID)
		if err != nil || !ok {
			return unified.ActionAuditRecord{}, false
		}
		cacheAudit(audit)
		return audit, true
	}

	if audits, err := store.GetActionAudits("", since, 0); err == nil {
		for _, audit := range audits {
			cacheAudit(audit)
			addAuditEvidence(audit)
		}
	}
	if events, err := store.GetActionLifecycleEvents("", since, 0); err == nil {
		for _, event := range events {
			actionID := strings.TrimSpace(event.ActionID)
			if actionID == "" {
				continue
			}
			recentIDs[actionID] = struct{}{}
			audit, ok := auditForID(actionID)
			if !ok {
				continue
			}
			if pulseIntelligenceActionWasApproved(audit) {
				approvedIDs[actionID] = struct{}{}
			}
			if pulseIntelligenceActionWasRejected(audit) {
				rejectedIDs[actionID] = struct{}{}
			}
			if agentOperationsLoopGovernedAction(audit) {
				governedIDs[actionID] = struct{}{}
			}
			if agentOperationsLoopVerifiedAction(audit) {
				verifiedIDs[actionID] = struct{}{}
			}
		}
	}
	counts.recent = len(recentIDs)
	counts.governed = len(governedIDs)
	counts.approved = len(approvedIDs)
	counts.rejected = len(rejectedIDs)
	counts.verified = len(verifiedIDs)
	return counts
}

func agentOperationsLoopGovernedAction(record unified.ActionAuditRecord) bool {
	return pulseIntelligenceActionWasApproved(record) || pulseIntelligenceActionWasRejected(record)
}

func agentOperationsLoopVerifiedAction(record unified.ActionAuditRecord) bool {
	return pulseIntelligenceActionVerifiedOutcome(record)
}

func agentOperationsLoopContextualCollaborationCount(status AgentOperationsLoopStatus) int {
	return status.assistantContextCount + status.externalAgentCollaborationCount
}

func agentOperationsLoopPatrolControlProof(status AgentOperationsLoopStatus) telemetry.PulseIntelligencePatrolControlProof {
	return telemetry.ClassifyPulseIntelligencePatrolControlProof(telemetry.PulseIntelligencePatrolControlProofInput{
		PatrolControlStarterCount:    status.PatrolControlLoopStarterCount,
		PatrolIssueEvidenceCount:     status.PatrolIssueEvidenceCount,
		ContextualCollaborationCount: agentOperationsLoopContextualCollaborationCount(status),
		ApprovedDecisionCount:        status.ApprovedDecisionCount,
		RejectedDecisionCount:        status.RejectedDecisionCount,
		VerifiedOutcomeCount:         status.VerifiedOutcomeCount,
	})
}

func agentOperationsLoopProgress(status AgentOperationsLoopStatus) (string, string, []AgentOperationsLoopStep) {
	hasIssueEvidence := status.PatrolIssueEvidenceCount > 0
	contextualCollaborationCount := agentOperationsLoopContextualCollaborationCount(status)
	hasContextualCollaboration := contextualCollaborationCount > 0
	hasApprovedDecision := status.ApprovedDecisionCount > 0 || status.VerifiedOutcomeCount > 0
	hasRejectedDecision := status.RejectedDecisionCount > 0
	hasGovernedDecision := hasApprovedDecision || hasRejectedDecision || status.GovernedActionCount > 0
	hasGovernedWork := status.PendingApprovalCount > 0 || hasGovernedDecision
	hasVerifiedOutcome := status.VerifiedOutcomeCount > 0
	hasRejectedTerminalDecision := hasRejectedDecision && !hasApprovedDecision
	hasPatrolControlCompletedLoop := status.PatrolControlCompletedLoopCount > 0
	hasPatrolControlResolvedLoop := status.PatrolControlResolvedLoopCount > 0
	governanceStepCount := status.PendingApprovalCount
	if decisionCount := status.ApprovedDecisionCount + status.RejectedDecisionCount; decisionCount > 0 {
		governanceStepCount = decisionCount
	} else if status.GovernedActionCount > 0 {
		governanceStepCount = status.GovernedActionCount
	}
	verificationStepCount := status.VerifiedOutcomeCount
	if verificationStepCount == 0 && hasRejectedTerminalDecision {
		verificationStepCount = status.RejectedDecisionCount
	}

	steps := []AgentOperationsLoopStep{
		{ID: "patrol", Label: "Patrol", Status: AgentOperationsLoopStepPending, Count: status.PatrolIssueEvidenceCount},
		{ID: "assistant", Label: "Assistant", Status: AgentOperationsLoopStepPending, Count: contextualCollaborationCount},
		{ID: "governance", Label: "Governance", Status: AgentOperationsLoopStepPending, Count: governanceStepCount},
		{ID: "verification", Label: "Verification", Status: AgentOperationsLoopStepPending, Count: verificationStepCount},
	}

	switch {
	case !hasIssueEvidence:
		steps[0].Status = AgentOperationsLoopStepCurrent
		if status.PatrolControlLoopStarterCount > 0 {
			return "run_patrol", "Patrol is ready to check the fleet and investigate the next real infrastructure issue.", steps
		}
		return "run_patrol", "Run Patrol to check for actionable infrastructure issues.", steps
	case !hasContextualCollaboration:
		steps[0].Status = AgentOperationsLoopStepComplete
		steps[1].Status = AgentOperationsLoopStepCurrent
		return "open_assistant", "Open Assistant to explain the Patrol issue and safest next step.", steps
	case !hasGovernedWork:
		steps[0].Status = AgentOperationsLoopStepComplete
		steps[1].Status = AgentOperationsLoopStepCurrent
		return "open_assistant", "Open Assistant to explain the Patrol issue and safest next step.", steps
	case status.PendingApprovalCount > 0 && !hasGovernedDecision:
		steps[0].Status = AgentOperationsLoopStepComplete
		steps[1].Status = AgentOperationsLoopStepComplete
		steps[2].Status = AgentOperationsLoopStepCurrent
		return "review_approvals", "Review the pending governed action approval.", steps
	case status.PendingApprovalCount > 0:
		steps[0].Status = AgentOperationsLoopStepComplete
		steps[1].Status = AgentOperationsLoopStepComplete
		steps[2].Status = AgentOperationsLoopStepCurrent
		return "review_approvals", "Review pending Patrol approvals before treating previous verified work as current.", steps
	case status.ActiveFindingCount > 0 && hasVerifiedOutcome:
		steps[0].Status = AgentOperationsLoopStepComplete
		steps[1].Status = AgentOperationsLoopStepCurrent
		if status.ActiveFindingCount == 1 {
			return "open_assistant", "Open Assistant on the active Patrol finding before treating previous verified work as current.", steps
		}
		return "review_findings", "Review active Patrol findings before treating previous verified work as current.", steps
	case hasRejectedTerminalDecision:
		for i := 0; i < 4; i++ {
			steps[i].Status = AgentOperationsLoopStepComplete
		}
		if hasPatrolControlCompletedLoop {
			return "complete", "Patrol recorded a rejected change decision. Nothing was changed; approve a safer fix before marking the issue resolved.", steps
		}
		return "complete", "Patrol recorded a governed rejection. Nothing was changed.", steps
	case !hasVerifiedOutcome:
		steps[0].Status = AgentOperationsLoopStepComplete
		steps[1].Status = AgentOperationsLoopStepComplete
		steps[2].Status = AgentOperationsLoopStepComplete
		steps[3].Status = AgentOperationsLoopStepCurrent
		if status.PatrolControlLoopStarterCount > 0 {
			return "review_findings", "Verify whether the approved Patrol action fixed the issue.", steps
		}
		return "review_findings", "Verify whether the governed action fixed the issue.", steps
	default:
		for i := 0; i < 4; i++ {
			steps[i].Status = AgentOperationsLoopStepComplete
		}
		if hasPatrolControlResolvedLoop {
			return "complete", "Patrol handled an infrastructure issue, verified the outcome, and recorded what happened.", steps
		}
		return "complete", "Patrol handled an infrastructure issue, verified the outcome, and recorded what happened.", steps
	}
}

func (h *AgentContextHandler) agentOperationsLoopExternalAgentReady(ctx context.Context, manifest agentcapabilities.Manifest, now time.Time) bool {
	if !agentOperationsLoopExternalAgentManifestReady(manifest) {
		return false
	}
	return h != nil &&
		h.externalAgentReadinessProvider != nil &&
		h.externalAgentReadinessProvider.ExternalAgentReady(ctx, manifest, now)
}

func agentOperationsLoopExternalAgentManifestReady(manifest agentcapabilities.Manifest) bool {
	adapter := agentcapabilities.NormalizeMCPAdapterContract(manifest.MCPAdapter)
	if strings.TrimSpace(adapter.Command) == "" ||
		strings.TrimSpace(adapter.ServerName) == "" ||
		strings.TrimSpace(adapter.TokenEnv) == "" ||
		len(adapter.ConfigFamilies) == 0 {
		return false
	}
	if !agentcapabilities.MCPManifestSurfacePromptProjectionSupported(manifest, agentcapabilities.SurfaceIDPulseMCP) {
		return false
	}
	hasOperationsLoopPrompt := false
	for _, prompt := range agentcapabilities.ManifestPulseWorkflowPrompts(manifest) {
		if strings.TrimSpace(prompt.Name) == agentcapabilities.PulseWorkflowPromptOperationsLoop {
			hasOperationsLoopPrompt = true
			break
		}
	}
	if !hasOperationsLoopPrompt {
		return false
	}
	contract, ok := agentcapabilities.ProjectManifestSurfaceToolContract(manifest, agentcapabilities.SurfaceIDPulseMCP)
	if !ok {
		return false
	}
	toolNames := map[string]struct{}{}
	for _, name := range contract.ToolNames {
		name = strings.TrimSpace(name)
		if name != "" {
			toolNames[name] = struct{}{}
		}
	}
	for _, name := range agentcapabilities.OperationsLoopCapabilityNames() {
		if _, ok := toolNames[name]; !ok {
			return false
		}
	}
	return true
}

func agentOperationsLoopExternalAgentTokenReady(manifest agentcapabilities.Manifest, tokens []config.APITokenRecord, now time.Time) bool {
	if !agentOperationsLoopExternalAgentManifestReady(manifest) {
		return false
	}
	surfaceScopes := agentcapabilities.RequiredCapabilityScopes(
		agentcapabilities.ManifestSurfaceToolCapabilities(manifest, agentcapabilities.SurfaceIDPulseMCP),
	)
	if len(surfaceScopes) == 0 {
		return false
	}
	for _, token := range tokens {
		if token.ExpiresAt != nil && now.After(token.ExpiresAt.UTC()) {
			continue
		}
		token = token.Clone()
		coversLoop := true
		for _, scope := range surfaceScopes {
			if strings.TrimSpace(scope) == "" {
				continue
			}
			if !token.HasScope(scope) {
				coversLoop = false
				break
			}
		}
		if coversLoop {
			return true
		}
	}
	return false
}

// projectAgentResourceOperatorState converts the canonical store
// shape into the agent-stable wire shape, computing the
// maintenance-window-active flag once at the server boundary.
func projectAgentResourceOperatorState(
	state unified.ResourceOperatorState,
	now time.Time,
) AgentResourceOperatorState {
	return AgentResourceOperatorState{
		IntentionallyOffline:    state.IntentionallyOffline,
		NeverAutoRemediate:      state.NeverAutoRemediate,
		MaintenanceStartAt:      state.MaintenanceStartAt,
		MaintenanceEndAt:        state.MaintenanceEndAt,
		MaintenanceReason:       state.MaintenanceReason,
		Criticality:             string(state.Criticality),
		Note:                    state.Note,
		SetAt:                   state.SetAt,
		SetBy:                   state.SetBy,
		MaintenanceWindowActive: state.IsInMaintenanceAt(now),
	}
}

func projectAgentResourceActions(
	audits []unified.ActionAuditRecord,
) []AgentResourceActionSummary {
	if len(audits) == 0 {
		return []AgentResourceActionSummary{}
	}
	out := make([]AgentResourceActionSummary, 0, len(audits))
	for _, audit := range audits {
		summary := AgentResourceActionSummary{
			ID:             audit.ID,
			CapabilityName: audit.Request.CapabilityName,
			State:          string(audit.State),
			RequestedBy:    audit.Request.RequestedBy,
			CreatedAt:      audit.CreatedAt,
			UpdatedAt:      audit.UpdatedAt,
		}
		if cmd, ok := audit.Request.Params["command"].(string); ok {
			summary.Command = cmd
		}
		if audit.Result != nil {
			summary.Success = audit.Result.Success
			summary.ErrorMessage = audit.Result.ErrorMessage
		}
		if v := projectAgentResourceVerification(unified.CanonicalActionVerification(audit)); v != nil {
			summary.Verification = v
		}
		out = append(out, summary)
	}
	return out
}

// pendingApprovalsForResourceFromStore is the named, testable form
// of the AgentApprovalsProvider closure the router wires at
// startup. Given an approval store, a canonical resource id, and
// an org id, it returns every still-pending approval that
// (a) belongs to the org via approval.BelongsToOrg, and
// (b) targets exactly the requested resource via
//
//	ApprovalRequest.CanonicalResourceID.
//
// Both filters apply: an approval that matches one but not the
// other is excluded so cross-org and cross-resource leaks are
// impossible at the substrate boundary. Returns nil for an empty
// store, an empty resource id, or no matches; an empty non-nil
// slice would imply "no matches" too but the bundle handler
// normalizes nil to []AgentResourceApprovalSummary{} so the wire
// shape is always an array.
func pendingApprovalsForResourceFromStore(store approvalsPendingProvider, resourceID, orgID string) []AgentResourceApprovalSummary {
	if isNilApprovalsPendingProvider(store) || strings.TrimSpace(resourceID) == "" {
		return nil
	}
	pending := store.GetPendingApprovals()
	if len(pending) == 0 {
		return nil
	}
	out := make([]AgentResourceApprovalSummary, 0, len(pending))
	for _, req := range pending {
		if req == nil {
			continue
		}
		if !approval.BelongsToOrg(req, orgID) {
			continue
		}
		if req.CanonicalResourceID() != resourceID {
			continue
		}
		out = append(out, AgentResourceApprovalSummary{
			ID:          req.ID,
			Command:     req.Command,
			RiskLevel:   string(req.RiskLevel),
			RequestedBy: req.RequestedBy,
			RequestedAt: req.RequestedAt,
			ExpiresAt:   req.ExpiresAt,
		})
	}
	return out
}

// pendingApprovalCountsByResourceFromStore is the fleet-context
// companion to pendingApprovalsForResourceFromStore. It scans the
// bounded pending-approval list once, applies the same org isolation
// predicate, and groups counts by canonical resource id so the fleet
// handler does not perform one store scan per resource.
func pendingApprovalCountsByResourceFromStore(store approvalsPendingProvider, orgID string) map[string]int {
	if isNilApprovalsPendingProvider(store) {
		return nil
	}
	pending := store.GetPendingApprovals()
	if len(pending) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, req := range pending {
		if req == nil {
			continue
		}
		if !approval.BelongsToOrg(req, orgID) {
			continue
		}
		resourceID := req.CanonicalResourceID()
		if resourceID == "" {
			continue
		}
		counts[resourceID]++
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

// approvalsPendingProvider is the minimal accessor surface the
// pending-approvals bundle filter depends on. The production
// implementation is *approval.Store; the interface keeps the
// function unit-testable without bringing the whole store up.
type approvalsPendingProvider interface {
	GetPendingApprovals() []*approval.ApprovalRequest
}

func isNilApprovalsPendingProvider(store approvalsPendingProvider) bool {
	if store == nil {
		return true
	}
	v := reflect.ValueOf(store)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

// projectAgentResourceVerification converts the canonical
// ActionVerificationResult into the agent-stable projection that
// both the resource-context bundle and the action.completed SSE
// payload carry. Returns nil when the source is nil so callers can
// branch on field presence — absent means "no verification result
// recorded" (e.g. refused-before-dispatch failures), distinct from
// "verification was skipped" which surfaces as Ran=false.
func projectAgentResourceVerification(v *unified.ActionVerificationResult) *AgentResourceActionVerification {
	if v == nil {
		return nil
	}
	return &AgentResourceActionVerification{
		Ran:     v.Ran,
		Success: v.Success,
		Command: v.Command,
		Note:    v.Note,
		RanAt:   v.RanAt,
	}
}
