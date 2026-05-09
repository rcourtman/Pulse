package api

import (
	"encoding/json"
	"net/http"
)

// AgentCapability declares one agent-consumable capability Pulse
// exposes — a single point of "here's what you can do and how." The
// shape is intentionally narrow so a future MCP-server slice can
// consume the manifest to register tools, and so external agents
// (Claude Code, custom integrations) can introspect Pulse without
// reading documentation. Each capability names the canonical REST
// path, the method, the scope required, and the stable error
// codes the response may carry — agents branch on those, not on
// human messages.
type AgentCapability struct {
	// Name is the agent-stable identifier for this capability.
	// snake_case to match the convention agents use for tool names.
	// MUST be stable across releases — renaming breaks integrations.
	Name string `json:"name"`

	// Description is a single-sentence explanation an agent can
	// surface to a user when it's deciding whether to call this
	// capability. Phrased imperatively ("Get the situated context for
	// a resource") rather than narratively.
	Description string `json:"description"`

	// Category groups capabilities for agent UIs. Stable values:
	// "context" (read-only situated reads), "operator-state"
	// (per-resource intent writes), "finding" (per-finding lifecycle
	// actions). Agents can filter the manifest by category.
	Category string `json:"category"`

	// Method + Path describe the canonical REST surface. Path
	// segments in `{braces}` are agent-supplied parameters; agents
	// percent-encode them when they substitute (the canonical IDs
	// commonly contain colons).
	Method string `json:"method"`
	Path   string `json:"path"`

	// Scope names the auth scope required to call this capability.
	// Agents that lack the scope must ask the operator to widen
	// their token rather than retrying.
	Scope string `json:"scope"`

	// ResponseShape is the agent-stable name of the response type
	// (e.g. "AgentResourceContext", "ResourceOperatorState"). Agents
	// branch on this to know what to parse; future MCP integrations
	// can map it to tool result schemas.
	ResponseShape string `json:"responseShape,omitempty"`

	// ErrorCodes is the closed set of stable error codes this
	// capability may return on failure. Agents branch on these
	// (e.g. "operator_state_not_set", "operator_state_invalid",
	// "resource_remediation_locked", "plan_drift") rather than
	// parsing human messages. Empty list = the capability uses only
	// generic HTTP-status error responses.
	ErrorCodes []string `json:"errorCodes,omitempty"`

	// RequestBodyShape, when non-empty, names the agent-stable
	// request body shape for non-GET capabilities so agents can
	// validate before sending.
	RequestBodyShape string `json:"requestBodyShape,omitempty"`
}

// AgentCapabilitiesManifest is the discovery document for Pulse's
// agent surface. Any agent that wants to integrate with Pulse fetches
// this once at startup, learns what's available, and calls the named
// capabilities. The manifest itself is read-only and unauthenticated
// (the Pulse capabilities it describes have their own auth scopes).
type AgentCapabilitiesManifest struct {
	// Version pins the manifest contract. Agents validate they
	// understand this version before using the manifest. Bumping
	// version is reserved for breaking shape changes; additive
	// capabilities ship under the same version.
	Version string `json:"version"`

	// Capabilities is the canonical list. Order is stable across
	// requests so agents can diff snapshots cheaply.
	Capabilities []AgentCapability `json:"capabilities"`
}

// agentCapabilitiesManifest is the v1 declaration of Pulse's
// agent-consumable surface. Hand-authored rather than auto-generated
// because the contract decisions (which capabilities are
// agent-stable, what the stable error codes are, what category each
// belongs to) are product-shaping and must not drift behind code
// changes. Adding a capability here is a deliberate "this is part of
// the agent surface" commitment.
var agentCapabilitiesManifest = AgentCapabilitiesManifest{
	Version: "v1",
	Capabilities: []AgentCapability{
		{
			Name:          "get_resource_context",
			Description:   "Return the situated picture of a resource — identity, operator-set state with maintenance-window-active flag, active findings, recent actions including refused dispatches.",
			Category:      "context",
			Method:        http.MethodGet,
			Path:          "/api/agent/resource-context/{resourceId}",
			Scope:         "monitoring:read",
			ResponseShape: "AgentResourceContext",
			ErrorCodes:    []string{"resource_not_found"},
		},
		{
			Name:          "get_operator_state",
			Description:   "Read the operator-set state for a resource (intentionally offline, never auto-remediate, maintenance window, criticality).",
			Category:      "operator-state",
			Method:        http.MethodGet,
			Path:          "/api/resources/{resourceId}/operator-state",
			Scope:         "monitoring:read",
			ResponseShape: "ResourceOperatorState",
			ErrorCodes:    []string{"operator_state_not_set"},
		},
		{
			Name:             "set_operator_state",
			Description:      "Replace the operator-set state for a resource. URL canonicalId wins over body; server populates setAt and setBy from the authenticated identity.",
			Category:         "operator-state",
			Method:           http.MethodPut,
			Path:             "/api/resources/{resourceId}/operator-state",
			Scope:            "monitoring:write",
			RequestBodyShape: "ResourceOperatorStateInput",
			ResponseShape:    "ResourceOperatorState",
			ErrorCodes:       []string{"operator_state_invalid"},
		},
		{
			Name:        "clear_operator_state",
			Description: "Remove any operator-set state for a resource. Idempotent — succeeds whether or not an entry was present.",
			Category:    "operator-state",
			Method:      http.MethodDelete,
			Path:        "/api/resources/{resourceId}/operator-state",
			Scope:       "monitoring:write",
		},
		{
			Name:          "subscribe_events",
			Description:   "Subscribe to the SSE event stream for real-time notifications: finding.created when a new finding is raised, approval.pending when a remediation request enters StatusPending and waits on operator decision, action.completed when an action audit reaches a terminal state (Completed or Failed, including refused-before-dispatch failures with stable error-token prefixes), heartbeat every 15s. Long-lived connection; agents listen instead of polling.",
			Category:      "context",
			Method:        http.MethodGet,
			Path:          "/api/agent/events",
			Scope:         "monitoring:read",
			ResponseShape: "text/event-stream of AgentEvent",
		},
		{
			Name:          "list_findings",
			Description:   "List all Patrol findings (active, dismissed, resolved). Filter client-side on returned shape.",
			Category:      "finding",
			Method:        http.MethodGet,
			Path:          "/api/ai/patrol/findings",
			Scope:         "monitoring:read",
			ResponseShape: "Finding[]",
		},
		{
			Name:             "acknowledge_finding",
			Description:      "Mark a finding as seen but keep it visible. Auto-resolves when the underlying condition clears.",
			Category:         "finding",
			Method:           http.MethodPost,
			Path:             "/api/ai/patrol/acknowledge",
			Scope:            "monitoring:write",
			RequestBodyShape: "{ finding_id: string }",
		},
		{
			Name:             "snooze_finding",
			Description:      "Hide a finding for a defined duration in hours.",
			Category:         "finding",
			Method:           http.MethodPost,
			Path:             "/api/ai/patrol/snooze",
			Scope:            "monitoring:write",
			RequestBodyShape: "{ finding_id: string, duration_hours: number }",
		},
		{
			Name:             "dismiss_finding",
			Description:      "Dismiss a finding with a reason: not_an_issue (permanent suppression), expected_behavior (acknowledged forever), or will_fix_later (7-day reminder commitment).",
			Category:         "finding",
			Method:           http.MethodPost,
			Path:             "/api/ai/patrol/dismiss",
			Scope:            "monitoring:write",
			RequestBodyShape: "{ finding_id: string, reason: \"not_an_issue\"|\"expected_behavior\"|\"will_fix_later\", note?: string }",
		},
		{
			Name:             "resolve_finding",
			Description:      "Manually mark a finding as resolved when the underlying issue has been fixed out-of-band.",
			Category:         "finding",
			Method:           http.MethodPost,
			Path:             "/api/ai/patrol/resolve",
			Scope:            "monitoring:write",
			RequestBodyShape: "{ finding_id: string }",
		},
	},
}

// HandleAgentCapabilitiesManifest serves
// `GET /api/agent/capabilities` — the discovery document for Pulse's
// agent surface. Cacheable, unauthenticated (the underlying
// capabilities have their own scopes); agents fetch this once and
// learn what's available.
func HandleAgentCapabilitiesManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(agentCapabilitiesManifest)
}
