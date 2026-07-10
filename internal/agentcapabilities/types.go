package agentcapabilities

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ActionMode tells agents whether a capability or tool reads state, can run a
// non-persistent check/scan, or changes Pulse or target-side state.
type ActionMode string

// ApprovalPolicy tells external agents whether the required scope is sufficient
// or whether the capability participates in Pulse's governed action-plan
// lifecycle.
type ApprovalPolicy string

const (
	ActionModeRead  ActionMode = "read"
	ActionModeMixed ActionMode = "mixed"
	ActionModeWrite ActionMode = "write"

	ApprovalPolicyScopeOnly  ApprovalPolicy = "scope_only"
	ApprovalPolicyActionPlan ApprovalPolicy = "action_plan"
)

// ControlToolsDisabledMessage is the stable operator guidance returned when a
// caller attempts to run a control-gated Assistant tool while the shared control
// level is read-only or otherwise not allowed to expose control tools.
const ControlToolsDisabledMessage = "Control tools are disabled. Open Pulse Intelligence settings, then set Pulse Assistant Permissions > Control mode to Controlled before using action tools."

// DefaultApprovalPolicyDescription returns a concise operator-facing
// explanation for a shared approval-policy value.
func DefaultApprovalPolicyDescription(policy ApprovalPolicy) string {
	switch policy {
	case ApprovalPolicyScopeOnly:
		return "required scope is sufficient; no action-plan approval is required"
	case ApprovalPolicyActionPlan:
		return "governed action-plan approval is required before execution"
	default:
		return ""
	}
}

// NormalizeToolGovernance fills the shared governance defaults for a tool
// definition. Assistant registry tools and external-agent projections use this
// instead of carrying their own action-mode and approval-policy fallback rules.
func NormalizeToolGovernance(governance ToolGovernance, requireControl bool, description string) ToolGovernance {
	if governance.ActionMode == "" {
		if requireControl {
			governance.ActionMode = ActionModeWrite
		} else {
			governance.ActionMode = ActionModeRead
		}
	}
	if governance.ApprovalPolicy == "" {
		if governance.ActionMode == ActionModeRead {
			governance.ApprovalPolicy = ApprovalPolicyScopeOnly
		} else if requireControl {
			governance.ApprovalPolicy = ApprovalPolicyActionPlan
		} else {
			governance.ApprovalPolicy = ApprovalPolicyScopeOnly
		}
	}
	if governance.ApprovalSummary == "" {
		if governance.ActionMode == ActionModeRead {
			governance.ApprovalSummary = "no approval required"
		} else if requireControl {
			governance.ApprovalSummary = "hidden in read-only mode; approval required in controlled mode"
		} else if desc := DefaultApprovalPolicyDescription(governance.ApprovalPolicy); desc != "" {
			governance.ApprovalSummary = desc
		} else {
			governance.ApprovalSummary = "write subactions require the tool's governed approval path"
		}
	}
	if governance.Summary == "" {
		governance.Summary = description
	}
	return governance
}

// NormalizeCapabilityGovernance fills the shared governance defaults for a
// manifest capability. Capability projections use HTTP method only as a fallback
// when the manifest has not declared an explicit action mode; the manifest-owned
// actionMode and approvalPolicy remain authoritative when present.
func NormalizeCapabilityGovernance(cap Capability) ToolGovernance {
	governance := ToolGovernance{
		ActionMode:     CapabilityActionMode(cap),
		ApprovalPolicy: cap.ApprovalPolicy,
		Summary:        cap.Description,
	}
	return NormalizeToolGovernance(governance, false, cap.Description)
}

// CapabilityActionMode returns the shared action mode for a manifest
// capability, preserving explicit read/mixed/write values and using method
// posture only as a compatibility fallback for incomplete manifests.
func CapabilityActionMode(cap Capability) ActionMode {
	switch mode := ActionMode(strings.TrimSpace(string(cap.ActionMode))); mode {
	case ActionModeRead, ActionModeMixed, ActionModeWrite:
		return mode
	}
	if strings.EqualFold(cap.Method, http.MethodGet) {
		return ActionModeRead
	}
	return ActionModeWrite
}

// NewToolGovernanceDescriptor builds the shared read-only governance descriptor
// for a registered tool after applying the canonical defaults.
func NewToolGovernanceDescriptor(name, description string, requireControl bool, governance ToolGovernance) ToolGovernanceDescriptor {
	normalized := NormalizeToolGovernance(governance, requireControl, description)
	return ToolGovernanceDescriptor{
		Name:            name,
		Description:     description,
		RequireControl:  requireControl,
		ActionMode:      normalized.ActionMode,
		ApprovalPolicy:  normalized.ApprovalPolicy,
		ApprovalSummary: normalized.ApprovalSummary,
		Summary:         normalized.Summary,
	}
}

// NormalizeToolGovernanceDescriptor applies the shared governance defaults to
// an existing descriptor without changing its identity or control-gating flag.
func NormalizeToolGovernanceDescriptor(descriptor ToolGovernanceDescriptor) ToolGovernanceDescriptor {
	return NewToolGovernanceDescriptor(descriptor.Name, descriptor.Description, descriptor.RequireControl, ToolGovernance{
		ActionMode:      descriptor.ActionMode,
		ApprovalPolicy:  descriptor.ApprovalPolicy,
		ApprovalSummary: descriptor.ApprovalSummary,
		Summary:         descriptor.Summary,
	})
}

// ToolGovernance records the governed posture for a structured tool. Concrete
// surfaces own handlers and availability, while this shared shape keeps
// Assistant prompts and external-agent projections on the same action and
// approval vocabulary.
type ToolGovernance struct {
	ActionMode      ActionMode
	ApprovalPolicy  ApprovalPolicy
	ApprovalSummary string
	Summary         string
}

// ToolGovernanceDescriptor is the read-only tool governance manifest surfaced
// to agents and Assistant prompt builders.
type ToolGovernanceDescriptor struct {
	Name            string         `json:"name"`
	Description     string         `json:"description,omitempty"`
	RequireControl  bool           `json:"requireControl,omitempty"`
	ActionMode      ActionMode     `json:"actionMode"`
	ApprovalPolicy  ApprovalPolicy `json:"approvalPolicy"`
	ApprovalSummary string         `json:"approvalSummary,omitempty"`
	Summary         string         `json:"summary,omitempty"`
}

// Capability declares one agent-consumable Pulse capability: what it does,
// which canonical HTTP route it calls, which scope it needs, how it is governed,
// and which stable error codes agents can branch on. The shape is intentionally
// narrow so MCP, in-app Assistant, and future external-agent adapters can
// project the same manifest without a second local registry.
type Capability struct {
	// Name is the agent-stable snake_case identifier. Renaming breaks
	// integrations and requires a manifest-version decision.
	Name string `json:"name"`

	// Title is the human-readable display name for agent clients. Name remains
	// the programmatic identifier; UI, docs, and MCP tool titles use this field.
	Title string `json:"title,omitempty"`

	// Description is a single-sentence explanation an agent can surface when
	// deciding whether to call this capability.
	Description string `json:"description"`

	// Category groups capabilities for agent UIs. Stable values include
	// "context", "provisioning", "operator-state", "finding", and "action".
	Category string `json:"category"`

	// Method and Path describe the canonical REST surface. Path segments in
	// braces are agent-supplied parameters and must be percent-encoded as
	// single path segments by adapters.
	Method string `json:"method"`
	Path   string `json:"path"`

	// Scope names the auth scope required to call this capability.
	Scope string `json:"scope"`

	// ActionMode tells agents whether the capability reads state, can run a
	// non-persistent check/scan, or changes Pulse or target-side state.
	ActionMode ActionMode `json:"actionMode"`

	// ApprovalPolicy tells agents whether the required scope is sufficient or
	// whether the capability participates in the governed action-plan lifecycle.
	ApprovalPolicy ApprovalPolicy `json:"approvalPolicy"`

	// ResponseShape is the agent-stable response type name.
	ResponseShape string `json:"responseShape,omitempty"`

	// OutputSchema is an optional JSON Schema for the structured result object
	// returned by request/response tools. It is raw JSON so the API can serve it
	// verbatim and MCP adapters can forward the exact same schema.
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`

	// ErrorCodes is the closed set of stable error codes this capability may
	// return on failure. Agents branch on these, not on human messages.
	ErrorCodes []string `json:"errorCodes,omitempty"`

	// RequestBodyShape names the agent-stable request body shape for non-GET
	// capabilities.
	RequestBodyShape string `json:"requestBodyShape,omitempty"`

	// InputSchema is an optional JSON Schema for agent-facing tool arguments.
	// It is raw JSON so the API can serve it verbatim and adapters can forward
	// the exact same schema.
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// CapabilityCategory describes one manifest-owned capability group for agent
// UIs and generated MCP docs. The category IDs on individual capabilities must
// refer to this vocabulary.
type CapabilityCategory struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// SurfaceContractComponent names one singular part of the shared Pulse
// Intelligence runtime contract, such as the shared core or primary built-in
// operator.
type SurfaceContractComponent struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// SurfaceAffordanceContract declares which Pulse Intelligence affordances a
// supported operator-facing surface owns. The manifest carries these booleans
// so Assistant, MCP, docs, and settings UI do not hand-maintain separate
// surface capability claims.
type SurfaceAffordanceContract struct {
	Tools                bool `json:"tools,omitempty"`
	Resources            bool `json:"resources,omitempty"`
	Prompts              bool `json:"prompts,omitempty"`
	CapabilityMetadata   bool `json:"capabilityMetadata,omitempty"`
	InteractiveQuestions bool `json:"interactiveQuestions,omitempty"`
}

// OperatorSurfaceContract describes one supported access path over the shared
// Pulse Intelligence Core. The JSON name remains operatorSurfaces for v1
// manifest compatibility.
type OperatorSurfaceContract struct {
	ID              string                    `json:"id"`
	Label           string                    `json:"label"`
	Description     string                    `json:"description"`
	Native          bool                      `json:"native"`
	ExternalAdapter bool                      `json:"externalAdapter"`
	Affordances     SurfaceAffordanceContract `json:"affordances,omitempty"`
}

// SurfaceContract pins the runtime relationship between the shared Pulse
// Intelligence Core, the primary built-in Patrol operator, and the supported
// access paths. External agents consume this from the same discovery manifest
// as tool capabilities so Pulse MCP cannot drift into a separate product
// contract.
type SurfaceContract struct {
	Core             SurfaceContractComponent  `json:"core"`
	ProactiveEngine  SurfaceContractComponent  `json:"proactiveEngine"`
	OperatorSurfaces []OperatorSurfaceContract `json:"operatorSurfaces"`
}

// MCPAdapterConfigFamily describes one client configuration shape supported by
// the Pulse MCP adapter. Client labels are examples for operator-facing copy;
// Shape is the stable formatter key used by docs and UI projections.
type MCPAdapterConfigFamily struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Shape        string   `json:"shape"`
	Description  string   `json:"description,omitempty"`
	FileHints    []string `json:"fileHints,omitempty"`
	ClientLabels []string `json:"clientLabels,omitempty"`
}

// MCPAdapterContract pins the runtime facts that every Pulse MCP client setup
// must share. Keeping these values in the manifest prevents the first-party UI,
// README generation, and external adapters from hand-maintaining separate
// command, flag, token, and config-shape copies.
type MCPAdapterContract struct {
	ServerName     string                   `json:"serverName"`
	Command        string                   `json:"command"`
	BaseURLFlag    string                   `json:"baseUrlFlag"`
	DefaultBaseURL string                   `json:"defaultBaseUrl"`
	TokenEnv       string                   `json:"tokenEnv"`
	ConfigFamilies []MCPAdapterConfigFamily `json:"configFamilies"`
}

// Manifest is the discovery document for Pulse's agent surface. The API serves
// this shape directly, while adapters such as pulse-mcp and agent-probe consume
// the same type rather than maintaining local wire-shape copies.
type Manifest struct {
	Version string `json:"version"`

	// SurfaceContract is the canonical product/runtime relationship for Pulse
	// Intelligence surfaces. It keeps first-party Assistant, external-agent MCP,
	// and proactive Patrol wording coupled to the same discovery contract as the
	// manifest-backed tools.
	SurfaceContract SurfaceContract `json:"surfaceContract"`

	// SurfaceToolContracts is the manifest-owned static tool posture for
	// external-agent adapters. Native Assistant live tool availability is
	// runtime-specific and stays behind the authenticated Assistant endpoint.
	SurfaceToolContracts []SurfaceToolContract `json:"surfaceToolContracts"`

	// MCPAdapter is the canonical setup contract for the Pulse MCP adapter:
	// server name, command, base URL flag/default, token environment variable,
	// and the supported client configuration families.
	MCPAdapter MCPAdapterContract `json:"mcpAdapter"`

	// RequiredScopes is the canonical deduplicated scope set needed to use every
	// currently declared capability.
	RequiredScopes []string `json:"requiredScopes"`

	// Categories is the manifest-owned presentation order and copy for
	// capability groups. First-party UI, MCP docs, and external clients consume
	// this instead of carrying local category tables.
	Categories []CapabilityCategory `json:"categories"`

	// WorkflowPrompts is the manifest-owned reusable workflow starter metadata.
	// Native Assistant surfaces and external MCP clients consume this same list
	// instead of maintaining prompt catalogues per surface.
	WorkflowPrompts []PulseWorkflowPrompt `json:"workflowPrompts"`

	Capabilities []Capability `json:"capabilities"`
}

// CloneCapability returns a detached capability descriptor so callers cannot
// mutate manifest-owned collection fields through a lookup or projection result.
func CloneCapability(cap Capability) Capability {
	if len(cap.ErrorCodes) > 0 {
		cap.ErrorCodes = append([]string(nil), cap.ErrorCodes...)
	}
	cap.InputSchema = CloneRawMessage(cap.InputSchema)
	cap.OutputSchema = CloneRawMessage(cap.OutputSchema)
	return cap
}

// CloneCapabilities returns detached capability descriptors in the original
// order.
func CloneCapabilities(capabilities []Capability) []Capability {
	if capabilities == nil {
		return nil
	}
	cloned := make([]Capability, len(capabilities))
	for i, cap := range capabilities {
		cloned[i] = CloneCapability(cap)
	}
	return cloned
}

// CloneCapabilityCategories returns detached capability-category descriptors in
// the original manifest order.
func CloneCapabilityCategories(categories []CapabilityCategory) []CapabilityCategory {
	if categories == nil {
		return nil
	}
	cloned := make([]CapabilityCategory, len(categories))
	copy(cloned, categories)
	return cloned
}

// ClonePulseWorkflowPrompt returns a detached workflow prompt descriptor.
func ClonePulseWorkflowPrompt(prompt PulseWorkflowPrompt) PulseWorkflowPrompt {
	prompt.Arguments = append([]PulseWorkflowPromptArgument(nil), prompt.Arguments...)
	return prompt.NormalizeCollections()
}

// ClonePulseWorkflowPrompts returns detached workflow prompt descriptors in the
// original projection order.
func ClonePulseWorkflowPrompts(prompts []PulseWorkflowPrompt) []PulseWorkflowPrompt {
	if prompts == nil {
		return nil
	}
	cloned := make([]PulseWorkflowPrompt, len(prompts))
	for i, prompt := range prompts {
		cloned[i] = ClonePulseWorkflowPrompt(prompt)
	}
	return cloned
}

// CloneSurfaceContract returns a detached copy of the manifest surface
// relationship contract.
func CloneSurfaceContract(contract SurfaceContract) SurfaceContract {
	if contract.OperatorSurfaces != nil {
		contract.OperatorSurfaces = append([]OperatorSurfaceContract(nil), contract.OperatorSurfaces...)
	}
	return contract
}

// CloneMCPAdapterConfigFamilies returns detached MCP adapter configuration
// family descriptors.
func CloneMCPAdapterConfigFamilies(families []MCPAdapterConfigFamily) []MCPAdapterConfigFamily {
	if families == nil {
		return nil
	}
	cloned := make([]MCPAdapterConfigFamily, len(families))
	for i, family := range families {
		cloned[i] = family
		if family.FileHints != nil {
			cloned[i].FileHints = append([]string(nil), family.FileHints...)
		}
		if family.ClientLabels != nil {
			cloned[i].ClientLabels = append([]string(nil), family.ClientLabels...)
		}
	}
	return cloned
}

// CloneMCPAdapterContract returns a detached copy of the manifest-owned Pulse
// MCP adapter setup contract.
func CloneMCPAdapterContract(contract MCPAdapterContract) MCPAdapterContract {
	contract.ConfigFamilies = CloneMCPAdapterConfigFamilies(contract.ConfigFamilies)
	return contract
}
