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
	// "context" (read-only situated reads), "provisioning"
	// (infrastructure onboarding and source lifecycle),
	// "operator-state" (per-resource intent writes), "finding"
	// (per-finding lifecycle actions), and "action" (governed
	// plan/decision/execute operations). Agents can filter the
	// manifest by category.
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

	// InputSchema is an optional JSON Schema for agent-facing tool
	// arguments. It is hand-authored for capabilities where prose
	// body hints are not enough for reliable model use.
	InputSchema map[string]any `json:"inputSchema,omitempty"`
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

func agentObjectInputSchema(required []string, properties map[string]any) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func nodeIDInputSchema() map[string]any {
	return agentObjectInputSchema([]string{"nodeId"}, map[string]any{
		"nodeId": map[string]any{
			"type":        "string",
			"description": "Configured node id from list_nodes, such as pve:lab or pve-0.",
		},
	})
}

func discoverLANInputSchema() map[string]any {
	return agentObjectInputSchema(nil, map[string]any{
		"subnet": map[string]any{
			"type":        "string",
			"description": "CIDR to scan, such as 192.168.1.0/24, or auto to let Pulse choose the local subnet.",
			"default":     "auto",
		},
		"use_cache": map[string]any{
			"type":        "boolean",
			"description": "Return cached discovery results when available instead of starting a new scan.",
			"default":     false,
		},
	})
}

func addNodeInputSchema() map[string]any {
	schema := agentObjectInputSchema([]string{"type", "name", "host"}, nodeConfigInputProperties(false))
	schema["oneOf"] = []map[string]any{
		{"required": []string{"user", "password"}},
		{"required": []string{"tokenName", "tokenValue"}},
	}
	return schema
}

func testNodeCredentialsInputSchema() map[string]any {
	schema := agentObjectInputSchema([]string{"type", "host"}, nodeConfigInputProperties(false))
	schema["oneOf"] = []map[string]any{
		{"required": []string{"user", "password"}},
		{"required": []string{"tokenName", "tokenValue"}},
	}
	return schema
}

func updateNodeInputSchema() map[string]any {
	properties := nodeConfigInputProperties(true)
	return agentObjectInputSchema([]string{"nodeId"}, properties)
}

func nodeConfigInputProperties(includeNodeID bool) map[string]any {
	properties := map[string]any{
		"type": map[string]any{
			"type":        "string",
			"enum":        []string{"pve", "pbs", "pmg"},
			"description": "Infrastructure source type: pve, pbs, or pmg.",
		},
		"name": map[string]any{
			"type":        "string",
			"description": "Human-readable source name to show in Pulse.",
		},
		"host": map[string]any{
			"type":        "string",
			"description": "Source endpoint URL or host, including scheme and port when needed.",
		},
		"guestURL": map[string]any{
			"type":        "string",
			"description": "Optional guest-accessible URL for navigation.",
		},
		"user": map[string]any{
			"type":        "string",
			"description": "Username for password authentication, or the token owner when needed by the platform.",
		},
		"password": map[string]any{
			"type":        "string",
			"description": "Password used only for setup or password-backed monitoring.",
		},
		"tokenName": map[string]any{
			"type":        "string",
			"description": "API token id or name, such as root@pam!pulse-monitor.",
		},
		"tokenValue": map[string]any{
			"type":        "string",
			"description": "API token secret value. Pulse stores this as a credential and never returns it from list_nodes.",
		},
		"fingerprint": map[string]any{
			"type":        "string",
			"description": "Optional TLS certificate fingerprint for pinned self-signed endpoints.",
		},
		"verifySSL": map[string]any{
			"type":        "boolean",
			"description": "Whether Pulse should require normal TLS certificate validation.",
		},
		"monitorVMs":                   platformBool("PVE", "virtual machines"),
		"monitorContainers":            platformBool("PVE", "containers"),
		"monitorStorage":               platformBool("PVE", "storage"),
		"monitorBackups":               platformBool("PVE or PBS", "backups"),
		"monitorPhysicalDisks":         platformBool("PVE", "physical disks"),
		"physicalDiskPollingMinutes":   integerOption("PVE physical disk polling interval in minutes. Use 0 or omit for the default."),
		"temperatureMonitoringEnabled": platformBool("All source types", "temperature monitoring"),
		"monitorDatastores":            platformBool("PBS", "datastores"),
		"monitorSyncJobs":              platformBool("PBS", "sync jobs"),
		"monitorVerifyJobs":            platformBool("PBS", "verify jobs"),
		"monitorPruneJobs":             platformBool("PBS", "prune jobs"),
		"monitorGarbageJobs":           platformBool("PBS", "garbage collection jobs"),
		"monitorMailStats":             platformBool("PMG", "mail statistics"),
		"monitorQueues":                platformBool("PMG", "mail queues"),
		"monitorQuarantine":            platformBool("PMG", "quarantine"),
		"monitorDomainStats":           platformBool("PMG", "domain statistics"),
		"enabled":                      platformBool("All source types", "collection from this source"),
		"excludeDatastores":            stringArrayOption("PBS datastore names to exclude from monitoring."),
	}
	if includeNodeID {
		properties["nodeId"] = map[string]any{
			"type":        "string",
			"description": "Configured node id from list_nodes, such as pve:lab or pve-0.",
		}
	}
	return properties
}

func platformBool(platform, subject string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": platform + " option for " + subject + ".",
	}
}

func integerOption(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"minimum":     0,
		"description": description,
	}
}

func stringArrayOption(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items": map[string]any{
			"type": "string",
		},
	}
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
			Description:   "Return the situated picture of a resource — identity, operator-set state with maintenance-window-active flag, active findings, pending approvals scoped to this resource, recent actions including refused dispatches. Command fields are redacted for monitoring-read API tokens unless the token also has ai:execute.",
			Category:      "context",
			Method:        http.MethodGet,
			Path:          "/api/agent/resource-context/{resourceId}",
			Scope:         "monitoring:read",
			ResponseShape: "AgentResourceContext",
			ErrorCodes:    []string{"resource_not_found"},
		},
		{
			Name:          "get_fleet_context",
			Description:   "Return a thin per-resource triage rollup across every resource visible to the org — identity, operator flags (intentionallyOffline, neverAutoRemediate, maintenanceWindowActive), per-severity finding counts (total/critical/warning/info), and pending-approval count. One read for 'where do I focus?'; follow up via get_resource_context for depth.",
			Category:      "context",
			Method:        http.MethodGet,
			Path:          "/api/agent/fleet-context",
			Scope:         "monitoring:read",
			ResponseShape: "AgentFleetContext",
		},
		{
			Name:          "list_nodes",
			Description:   "List configured infrastructure sources that Pulse can monitor or manage. Credential secret values are redacted; use the returned id with update_node, remove_node, test_node_connection, or refresh_node_cluster_membership.",
			Category:      "provisioning",
			Method:        http.MethodGet,
			Path:          "/api/config/nodes",
			Scope:         "settings:read",
			ResponseShape: "NodeResponse[]",
		},
		{
			Name:             "add_node",
			Description:      "Add a Proxmox VE, Proxmox Backup Server, or Proxmox Mail Gateway source to Pulse after credentials have been collected, generated, or approved.",
			Category:         "provisioning",
			Method:           http.MethodPost,
			Path:             "/api/config/nodes",
			Scope:            "settings:write",
			RequestBodyShape: "NodeConfigRequest",
			ResponseShape:    "{ status: \"success\" }",
			InputSchema:      addNodeInputSchema(),
		},
		{
			Name:             "update_node",
			Description:      "Update a configured infrastructure source. Omitted fields preserve the current value; tokenValue or password only changes when supplied.",
			Category:         "provisioning",
			Method:           http.MethodPut,
			Path:             "/api/config/nodes/{nodeId}",
			Scope:            "settings:write",
			RequestBodyShape: "NodeConfigRequest",
			ResponseShape:    "{ status: \"success\" }",
			InputSchema:      updateNodeInputSchema(),
		},
		{
			Name:          "remove_node",
			Description:   "Remove a configured infrastructure source from Pulse by node id.",
			Category:      "provisioning",
			Method:        http.MethodDelete,
			Path:          "/api/config/nodes/{nodeId}",
			Scope:         "settings:write",
			ResponseShape: "{ status: \"success\" }",
			InputSchema:   nodeIDInputSchema(),
		},
		{
			Name:             "test_node_credentials",
			Description:      "Validate proposed source credentials and connection details without saving them to Pulse.",
			Category:         "provisioning",
			Method:           http.MethodPost,
			Path:             "/api/config/nodes/test-config",
			Scope:            "settings:write",
			RequestBodyShape: "NodeConfigRequest",
			ResponseShape:    "{ status: \"success\"|\"error\", message: string }",
			InputSchema:      testNodeCredentialsInputSchema(),
		},
		{
			Name:          "test_node_connection",
			Description:   "Validate the saved connection for an existing configured infrastructure source.",
			Category:      "provisioning",
			Method:        http.MethodPost,
			Path:          "/api/config/nodes/{nodeId}/test",
			Scope:         "settings:write",
			ResponseShape: "{ status: \"success\"|\"error\", message: string }",
			InputSchema:   nodeIDInputSchema(),
		},
		{
			Name:          "refresh_node_cluster_membership",
			Description:   "Re-detect Proxmox VE cluster membership and endpoint metadata for a configured source.",
			Category:      "provisioning",
			Method:        http.MethodPost,
			Path:          "/api/config/nodes/{nodeId}/refresh-cluster",
			Scope:         "settings:write",
			ResponseShape: "ClusterRefreshResponse",
			InputSchema:   nodeIDInputSchema(),
		},
		{
			Name:             "discover_lan",
			Description:      "Scan a subnet, or return cached scan results, to find candidate infrastructure hosts before deciding which sources to add to Pulse.",
			Category:         "provisioning",
			Method:           http.MethodPost,
			Path:             "/api/discover",
			Scope:            "settings:write",
			RequestBodyShape: "{ subnet?: string, use_cache?: boolean }",
			ResponseShape:    "ManualDiscoveryResult",
			InputSchema:      discoverLANInputSchema(),
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
			Description:   "Subscribe to the SSE event stream for real-time notifications: finding.created when a new finding is raised, approval.pending when a remediation request enters StatusPending and waits on operator decision, action.completed when an action audit reaches a terminal state (Completed or Failed, including refused-before-dispatch failures with stable error-token prefixes; carries a verification block with the read-after-write probe outcome so agents close the certainty loop without polling /api/actions/{id}), stream-local heartbeat every 15s. Command fields are redacted for monitoring-read API tokens unless the token also has ai:execute. Long-lived connection; agents listen instead of polling.",
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
		{
			Name:             "plan_action",
			Description:      "Plan an action against a resource. The planner validates the request, looks up the capability on the resource, checks executor-owned live availability, and returns an ActionPlan with the approval policy, blast radius, plan hash, and preflight summary. The plan is persisted to the audit history at the planned/pending state only after the live availability check passes, so subsequent decide_action and execute_action calls can reference it by id. Plan-and-execute is a two-step flow when the resulting plan requires approval, one-step otherwise.",
			Category:         "action",
			Method:           http.MethodPost,
			Path:             "/api/actions/plan",
			Scope:            "ai:execute",
			RequestBodyShape: "ActionRequest",
			ResponseShape:    "ActionPlan",
			ErrorCodes:       []string{"invalid_action_request", "resource_not_found", "capability_not_found", "action_execution_unavailable"},
		},
		{
			Name:             "decide_action",
			Description:      "Record an approval decision (approved or rejected) on a previously planned action. The actor is taken from the authenticated identity; an explicit reason can be passed in the body. Idempotent on the persisted decision: re-deciding a non-pending action surfaces the action_not_pending stable code so agents can branch on the conflict rather than retrying blindly.",
			Category:         "action",
			Method:           http.MethodPost,
			Path:             "/api/actions/{actionId}/decision",
			Scope:            "ai:execute",
			RequestBodyShape: "{ outcome: \"approved\"|\"rejected\", reason?: string }",
			ResponseShape:    "ActionDecisionResponse",
			ErrorCodes:       []string{"missing_id", "invalid_id", "invalid_action_decision", "action_not_found", "action_not_pending", "action_plan_expired"},
		},
		{
			Name:             "execute_action",
			Description:      "Execute a previously planned and (when required) approved action. Returns the persisted audit record with the execution result attached. Refuses with stable codes when the action is in the wrong lifecycle state (action_not_approved, action_already_executing, action_execution_final, action_dry_run_only, action_plan_expired), when the approved plan no longer matches the current resource/capability contract (action_plan_drift), when the target is operator-locked against automated remediation (resource_remediation_locked), or when the API instance has no executor wired (action_executor_unavailable). action.completed SSE events fire on every terminal state so agents watching the stream do not need to poll this endpoint after dispatch.",
			Category:         "action",
			Method:           http.MethodPost,
			Path:             "/api/actions/{actionId}/execute",
			Scope:            "ai:execute",
			RequestBodyShape: "{ reason?: string }",
			ResponseShape:    "ActionExecutionResponse",
			ErrorCodes:       []string{"missing_id", "invalid_id", "invalid_action_execution", "action_not_found", "action_not_approved", "action_already_executing", "action_execution_final", "action_dry_run_only", "action_plan_expired", "action_plan_drift", "resource_remediation_locked", "action_executor_unavailable"},
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
