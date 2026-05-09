package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

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

// AgentResourceActionSummary is the agent-consumable projection of an
// action audit record. Includes the refusal-reason prefix tokens
// (resource_remediation_locked:, plan_drift:) verbatim so agents can
// branch on them without parsing the human message.
type AgentResourceActionSummary struct {
	ID             string    `json:"id"`
	CapabilityName string    `json:"capabilityName"`
	Command        string    `json:"command,omitempty"`
	State          string    `json:"state"`
	Success        bool      `json:"success"`
	ErrorMessage   string    `json:"errorMessage,omitempty"`
	RequestedBy    string    `json:"requestedBy,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
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

// AgentResourceContext is the bundled, agent-consumable situated
// picture of a resource. One read returns:
//   - core identity (canonical id, type, name)
//   - operator-set state, with computed maintenance-window-active flag
//   - active findings (lightweight snapshot of the seven-question schema)
//   - recent action attempts (including refused dispatches with their
//     stable error tokens)
//
// The endpoint trades coverage for shape: an agent gets enough to
// reason about the next move without chaining several calls. Deeper
// detail (full finding records, full audit history) remains available
// via the existing per-finding / per-audit endpoints.
type AgentResourceContext struct {
	CanonicalID    string                         `json:"canonicalId"`
	ResourceType   string                         `json:"resourceType"`
	ResourceName   string                         `json:"resourceName"`
	Technology     string                         `json:"technology,omitempty"`
	OperatorState  *AgentResourceOperatorState    `json:"operatorState,omitempty"`
	ActiveFindings []AgentResourceFindingSnapshot `json:"activeFindings"`
	RecentActions  []AgentResourceActionSummary   `json:"recentActions"`
	GeneratedAt    time.Time                      `json:"generatedAt"`
}

// AgentFindingsProvider returns the active findings for a resource as
// agent-stable snapshots. The implementation lives outside this
// package (the patrol service holds the canonical findings store);
// this interface keeps the api layer free of an `internal/ai` import.
type AgentFindingsProvider interface {
	ActiveFindingsForResource(resourceID string) []AgentResourceFindingSnapshot
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

// AgentContextHandler owns the agent-paradigm bundled context
// endpoint. Kept as a separate type from `ResourceHandlers` so the
// agent surface evolves independently of the resource CRUD surface
// and the resource handler stays focused on its existing concerns.
// The handler reuses the resource handler for registry + store
// access (those are the canonical accessors); the
// agent-findings adapter is held here.
type AgentContextHandler struct {
	resources        *ResourceHandlers
	findingsProvider AgentFindingsProvider
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
	resource, ok := registry.Get(resourceID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "resource_not_found",
			"No resource is registered with this canonical id.")
		return
	}

	store, err := h.resources.getStore(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}

	bundle := AgentResourceContext{
		CanonicalID:    resourceID,
		ResourceType:   string(resource.Type),
		ResourceName:   resource.Name,
		Technology:     resource.Technology,
		ActiveFindings: []AgentResourceFindingSnapshot{},
		RecentActions:  []AgentResourceActionSummary{},
		GeneratedAt:    time.Now().UTC(),
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bundle)
}

// extractAgentResourceContextID parses the canonical resource ID out of
// the URL path. Mirrors the trim/canonicalize pattern used elsewhere
// for nested resource routes.
func extractAgentResourceContextID(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/agent/resource-context/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return unified.CanonicalResourceID(trimmed)
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
		out = append(out, summary)
	}
	return out
}
