package agentcapabilities

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const (
	// FleetContextCapabilityName is the manifest-owned fleet triage read.
	FleetContextCapabilityName = "get_fleet_context"
	// ResourceContextCapabilityName is the manifest-owned per-resource depth read.
	ResourceContextCapabilityName = "get_resource_context"
	// PatrolControlStatusCapabilityName is the manifest-owned Patrol control
	// status read. It is count-only and content-safe so external agents can
	// orient on the same Patrol -> Assistant -> governed action ->
	// verification loop without fetching finding/action identifiers or
	// command output first.
	PatrolControlStatusCapabilityName = "get_patrol_control_status"
	// OperationsLoopStatusCapabilityName is retained as a source compatibility
	// alias while the manifest projects Patrol control as the canonical
	// external-agent capability.
	OperationsLoopStatusCapabilityName = PatrolControlStatusCapabilityName
	// ResourceIDArgumentName is the shared path/body argument for canonical
	// resource identity in Pulse Intelligence context and action tools.
	ResourceIDArgumentName = "resourceId"
	// FleetContextCapabilityPath is the manifest-owned fleet triage route.
	FleetContextCapabilityPath = "/api/agent/fleet-context"
	// ResourceContextCapabilityPath is the manifest-owned per-resource context route.
	ResourceContextCapabilityPath = "/api/agent/resource-context/{" + ResourceIDArgumentName + "}"
	// PatrolControlStatusCapabilityPath is the manifest-owned Patrol control
	// status route.
	PatrolControlStatusCapabilityPath = "/api/agent/patrol-control/status"
	// OperationsLoopStatusCapabilityPath is retained as a source compatibility
	// alias while the manifest projects Patrol control as the canonical route.
	OperationsLoopStatusCapabilityPath = PatrolControlStatusCapabilityPath
	// OperationsLoopStatusCompatibilityPath is the legacy route kept for older
	// external-agent clients.
	OperationsLoopStatusCompatibilityPath = "/api/agent/operations-loop/status"
	// GetOperatorStateCapabilityName is the manifest-owned operator-state read.
	GetOperatorStateCapabilityName = "get_operator_state"
	// SetOperatorStateCapabilityName is the manifest-owned operator-state write.
	SetOperatorStateCapabilityName = "set_operator_state"
	// ClearOperatorStateCapabilityName is the manifest-owned operator-state clear.
	ClearOperatorStateCapabilityName = "clear_operator_state"
	// OperatorStateCapabilityPath is the shared operator-state route template.
	OperatorStateCapabilityPath = "/api/resources/{" + ResourceIDArgumentName + "}/operator-state"
	// ListNodesCapabilityName is the manifest-owned configured-source list read.
	ListNodesCapabilityName = "list_nodes"
	// AddNodeCapabilityName is the manifest-owned configured-source create action.
	AddNodeCapabilityName = "add_node"
	// UpdateNodeCapabilityName is the manifest-owned configured-source update action.
	UpdateNodeCapabilityName = "update_node"
	// RemoveNodeCapabilityName is the manifest-owned configured-source delete action.
	RemoveNodeCapabilityName = "remove_node"
	// TestNodeCredentialsCapabilityName is the manifest-owned proposed-source credential validation action.
	TestNodeCredentialsCapabilityName = "test_node_credentials"
	// TestNodeConnectionCapabilityName is the manifest-owned saved-source connection validation action.
	TestNodeConnectionCapabilityName = "test_node_connection"
	// RefreshNodeClusterMembershipCapabilityName is the manifest-owned Proxmox cluster endpoint refresh action.
	RefreshNodeClusterMembershipCapabilityName = "refresh_node_cluster_membership"
	// DiscoverLANCapabilityName is the manifest-owned LAN discovery scan action.
	DiscoverLANCapabilityName = "discover_lan"
	// ListFindingsCapabilityName is the manifest-owned Patrol finding list read.
	ListFindingsCapabilityName = "list_findings"
	// AcknowledgeFindingCapabilityName is the manifest-owned Patrol acknowledge write.
	AcknowledgeFindingCapabilityName = "acknowledge_finding"
	// SnoozeFindingCapabilityName is the manifest-owned Patrol snooze write.
	SnoozeFindingCapabilityName = "snooze_finding"
	// DismissFindingCapabilityName is the manifest-owned Patrol dismiss write.
	DismissFindingCapabilityName = "dismiss_finding"
	// ResolveFindingCapabilityName is the manifest-owned Patrol resolve write.
	ResolveFindingCapabilityName = "resolve_finding"
	// FindingIDArgumentName is the shared argument for Patrol finding identity.
	FindingIDArgumentName = "finding_id"
	// ResolutionNoteArgumentName is the shared operator/agent explanation for
	// why a Patrol finding is being marked resolved.
	ResolutionNoteArgumentName = "resolution_note"
	// NoteArgumentName is the shared optional operator note argument used by
	// Patrol finding lifecycle actions that accept freeform context.
	NoteArgumentName = "note"
	// PlanActionCapabilityName is the manifest-owned governed action planner.
	PlanActionCapabilityName = "plan_action"
	// DecideActionCapabilityName is the manifest-owned governed action decision writer.
	DecideActionCapabilityName = "decide_action"
	// ExecuteActionCapabilityName is the manifest-owned governed action executor.
	ExecuteActionCapabilityName = "execute_action"
	// RequestIDArgumentName is the shared idempotency/correlation argument.
	RequestIDArgumentName = "requestId"
	// ActionIDArgumentName is the shared durable governed action id argument.
	ActionIDArgumentName = "actionId"
	// CapabilityNameArgumentName is the shared target resource capability argument.
	CapabilityNameArgumentName = "capabilityName"
	// ReasonArgumentName is the shared operator-readable reason argument.
	ReasonArgumentName = "reason"
	// RequestedByArgumentName is the shared stable requester identity argument.
	RequestedByArgumentName = "requestedBy"
	// OutcomeArgumentName is the shared approval decision argument.
	OutcomeArgumentName = "outcome"
	// PlanActionCapabilityPath is the manifest-owned governed action planning route.
	PlanActionCapabilityPath = "/api/actions/plan"
	// ActionDecisionCapabilityPath is the manifest-owned governed action decision route template.
	ActionDecisionCapabilityPath = "/api/actions/{" + ActionIDArgumentName + "}/decision"
	// ActionExecutionCapabilityPath is the manifest-owned governed action execution route template.
	ActionExecutionCapabilityPath = "/api/actions/{" + ActionIDArgumentName + "}/execute"
	// EventSubscriptionCapabilityName is the manifest-owned streaming
	// capability. It is intentionally not projected as a request/response tool.
	EventSubscriptionCapabilityName = "subscribe_events"
	// ToolMetaPulseCapabilityKey is the namespaced _meta key containing
	// structured Pulse capability metadata for external-agent tools projected
	// from the canonical manifest.
	ToolMetaPulseCapabilityKey = "pulse.capability"
)

// ProjectedTool is the manifest-backed request/response tool shape consumed by
// MCP and other external-agent adapters.
type ProjectedTool struct {
	Name         string             `json:"name"`
	Title        string             `json:"title,omitempty"`
	Description  string             `json:"description"`
	InputSchema  json.RawMessage    `json:"inputSchema"`
	OutputSchema json.RawMessage    `json:"outputSchema,omitempty"`
	Annotations  *ToolBehaviorHints `json:"annotations,omitempty"`
	Meta         map[string]any     `json:"_meta,omitempty"`
}

// ToolBehaviorHints are the external-agent behavior hints projected from the
// manifest action-mode and approval-policy posture. They are hints for clients,
// not an authorization boundary; Pulse API scopes and governed action approval
// remain the enforcement layer.
type ToolBehaviorHints struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

// pathPlaceholderRE matches `{paramName}` in capability paths.
var pathPlaceholderRE = regexp.MustCompile(`\{([a-zA-Z][a-zA-Z0-9]*)\}`)

// OperationsLoopCapabilityNames returns the canonical capability set required
// for an external agent to run the Pulse Intelligence operations loop without
// falling back to native-only UI state.
func OperationsLoopCapabilityNames() []string {
	return []string{
		OperationsLoopStatusCapabilityName,
		FleetContextCapabilityName,
		ResourceContextCapabilityName,
		ListFindingsCapabilityName,
		PlanActionCapabilityName,
		DecideActionCapabilityName,
		ExecuteActionCapabilityName,
		ResolveFindingCapabilityName,
	}
}

// IsRequestResponseCapability reports whether a capability can be projected as
// a request/response external-agent tool.
func IsRequestResponseCapability(cap Capability) bool {
	return cap.Name != EventSubscriptionCapabilityName
}

// FindCapability returns the capability with the given manifest name.
func FindCapability(capabilities []Capability, name string) (Capability, bool) {
	for _, cap := range capabilities {
		if cap.Name == name {
			return CloneCapability(cap), true
		}
	}
	return Capability{}, false
}

// CapabilityLookupError is returned when a named manifest capability cannot be
// resolved. Adapters can translate the wording for their own surface while
// preserving the shared lookup semantics.
type CapabilityLookupError struct {
	Name string
}

func (e CapabilityLookupError) Error() string {
	return fmt.Sprintf("unknown capability: %s", e.Name)
}

// ResolveCapability returns the capability with the given manifest name or a
// typed lookup error when it is absent.
func ResolveCapability(capabilities []Capability, name string) (Capability, error) {
	cap, ok := FindCapability(capabilities, name)
	if !ok {
		return Capability{}, CapabilityLookupError{Name: name}
	}
	return cap, nil
}

// ResolveRequestResponseCapability returns a manifest capability only when it
// is eligible for request/response tool projection. Streaming capabilities
// intentionally look absent to tools/call-style adapters because they belong
// on the shared SSE subscription path instead.
func ResolveRequestResponseCapability(capabilities []Capability, name string) (Capability, error) {
	cap, err := ResolveCapability(capabilities, name)
	if err != nil {
		return Capability{}, err
	}
	if !IsRequestResponseCapability(cap) {
		return Capability{}, CapabilityLookupError{Name: name}
	}
	return cap, nil
}

// ProjectTool projects one request/response manifest capability into the common
// external-agent tool shape.
func ProjectTool(cap Capability) (ProjectedTool, bool) {
	if !IsRequestResponseCapability(cap) {
		return ProjectedTool{}, false
	}
	return ProjectedTool{
		Name:         cap.Name,
		Title:        CapabilityTitle(cap),
		Description:  ToolDescription(cap),
		InputSchema:  ToolInputSchema(cap),
		OutputSchema: ToolOutputSchema(cap),
		Annotations:  ToolAnnotations(cap),
		Meta:         ToolMeta(cap),
	}, true
}

// CapabilityTitle returns the manifest-owned human-readable title for a
// capability, falling back to a stable sentence-cased projection of the
// programmatic name for older manifests.
func CapabilityTitle(cap Capability) string {
	if title := strings.TrimSpace(cap.Title); title != "" {
		return title
	}
	name := strings.TrimSpace(cap.Name)
	if name == "" {
		return ""
	}
	title := strings.Join(strings.Fields(strings.ReplaceAll(name, "_", " ")), " ")
	return strings.ToUpper(title[:1]) + title[1:]
}

// ProjectTools projects request/response manifest capabilities into the common
// external-agent tool list shape. Streaming capabilities such as
// subscribe_events are intentionally filtered out.
func ProjectTools(capabilities []Capability) []ProjectedTool {
	tools := make([]ProjectedTool, 0, len(capabilities))
	for _, cap := range capabilities {
		tool, ok := ProjectTool(cap)
		if !ok {
			continue
		}
		tools = append(tools, tool)
	}
	return tools
}

// FindManifestSurfaceToolContract resolves a manifest-published surface tool
// contract by stable operator-surface id.
func FindManifestSurfaceToolContract(manifest Manifest, surfaceID string) (SurfaceToolContract, bool) {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return SurfaceToolContract{}, false
	}
	for _, contract := range manifest.SurfaceToolContracts {
		if strings.EqualFold(strings.TrimSpace(contract.SurfaceID), surfaceID) {
			return contract.NormalizeCollections(), true
		}
	}
	return SurfaceToolContract{}, false
}

// ManifestSurfaceToolCapabilities returns the request/response capabilities
// that a manifest-published external-agent surface is allowed to expose as
// tools. The normalized surface tool contract is authoritative; missing
// surface tool contracts expose no tools rather than inferring from raw
// manifest capabilities.
func ManifestSurfaceToolCapabilities(manifest Manifest, surfaceID string) []Capability {
	contract, declared := ResolveManifestSurfaceToolContract(manifest, surfaceID)
	if !declared {
		return []Capability{}
	}
	affordances, _ := ManifestSurfaceAffordances(manifest, contract.SurfaceID)
	if !affordances.Tools {
		return []Capability{}
	}

	capabilities := make([]Capability, 0, len(contract.ToolNames))
	seen := map[string]struct{}{}
	for _, rawName := range contract.ToolNames {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		capability, err := ResolveRequestResponseCapability(manifest.Capabilities, name)
		if err != nil {
			continue
		}
		capabilities = append(capabilities, capability)
	}
	return capabilities
}

// ProjectManifestSurfaceTools projects the manifest-owned tool allowlist for an
// external-agent surface into the shared tools/list shape.
func ProjectManifestSurfaceTools(manifest Manifest, surfaceID string) []ProjectedTool {
	return ProjectTools(ManifestSurfaceToolCapabilities(manifest, surfaceID))
}

// ToolDescription projects manifest-owned capability metadata into the
// description shown by external-agent tool surfaces.
func ToolDescription(cap Capability) string {
	governance := NormalizeCapabilityGovernance(cap)
	var b strings.Builder
	if desc := strings.TrimSpace(cap.Description); desc != "" {
		b.WriteString(desc)
		b.WriteString("\n\n")
	}
	b.WriteString("Pulse capability metadata:")
	if category := strings.TrimSpace(cap.Category); category != "" {
		b.WriteString("\n- category: ")
		b.WriteString(category)
	}
	route := strings.TrimSpace(strings.Join([]string{cap.Method, cap.Path}, " "))
	if route != "" {
		b.WriteString("\n- route: ")
		b.WriteString(route)
	}
	if scope := strings.TrimSpace(cap.Scope); scope != "" {
		b.WriteString("\n- required scope: ")
		b.WriteString(scope)
	}
	b.WriteString("\n- action mode: ")
	b.WriteString(string(governance.ActionMode))
	b.WriteString("\n- approval policy: ")
	b.WriteString(string(governance.ApprovalPolicy))
	if requestShape := strings.TrimSpace(cap.RequestBodyShape); requestShape != "" {
		b.WriteString("\n- request body: ")
		b.WriteString(requestShape)
	}
	if responseShape := strings.TrimSpace(cap.ResponseShape); responseShape != "" {
		b.WriteString("\n- response: ")
		b.WriteString(responseShape)
	}
	if len(cap.ErrorCodes) > 0 {
		b.WriteString("\n- stable error codes: ")
		b.WriteString(strings.Join(cap.ErrorCodes, ", "))
	}
	return b.String()
}

// ToolAnnotations projects manifest governance into external-agent tool
// behavior hints so clients can distinguish read-only, non-destructive
// scan/check, and write-capable operations without carrying a Pulse-specific
// policy table.
func ToolAnnotations(cap Capability) *ToolBehaviorHints {
	return toolBehaviorHintsForActionMode(NormalizeCapabilityGovernance(cap).ActionMode)
}

// ToolGovernanceBehaviorHints projects registry-owned governance into the same
// neutral behavior-hint vocabulary used by manifest-backed external tools.
func ToolGovernanceBehaviorHints(governance ToolGovernanceDescriptor) *ToolBehaviorHints {
	return toolBehaviorHintsForActionMode(NormalizeToolGovernanceDescriptor(governance).ActionMode)
}

func toolBehaviorHintsForActionMode(mode ActionMode) *ToolBehaviorHints {
	switch mode {
	case ActionModeRead, ActionModeMixed, ActionModeWrite:
	default:
		mode = ActionModeRead
	}
	readOnly := mode == ActionModeRead
	destructive := mode == ActionModeWrite
	idempotent := readOnly
	openWorld := true
	return &ToolBehaviorHints{
		ReadOnlyHint:    boolRef(readOnly),
		DestructiveHint: boolRef(destructive),
		IdempotentHint:  boolRef(idempotent),
		OpenWorldHint:   boolRef(openWorld),
	}
}

// CloneToolBehaviorHints returns a detached behavior-hint struct for shared
// Assistant and external-agent tool projections.
func CloneToolBehaviorHints(hints *ToolBehaviorHints) *ToolBehaviorHints {
	if hints == nil {
		return nil
	}
	cloned := &ToolBehaviorHints{Title: hints.Title}
	cloned.ReadOnlyHint = cloneBoolRef(hints.ReadOnlyHint)
	cloned.DestructiveHint = cloneBoolRef(hints.DestructiveHint)
	cloned.IdempotentHint = cloneBoolRef(hints.IdempotentHint)
	cloned.OpenWorldHint = cloneBoolRef(hints.OpenWorldHint)
	return cloned
}

// ToolMeta projects Pulse-specific manifest metadata into the standard _meta
// extension object used by external-agent tools. The human description still
// carries this posture for model selection, while _meta gives clients a
// structured route/scope/governance contract without parsing prose.
func ToolMeta(cap Capability) map[string]any {
	governance := NormalizeCapabilityGovernance(cap)
	capability := map[string]any{}
	setMetaString(capability, "name", cap.Name)
	setMetaString(capability, "category", cap.Category)
	setMetaString(capability, "scope", cap.Scope)
	if requestShape := strings.TrimSpace(cap.RequestBodyShape); requestShape != "" {
		capability["requestBodyShape"] = requestShape
	}
	if responseShape := strings.TrimSpace(cap.ResponseShape); responseShape != "" {
		capability["responseShape"] = responseShape
	}
	if len(cap.ErrorCodes) > 0 {
		capability["errorCodes"] = append([]string(nil), cap.ErrorCodes...)
	}

	route := map[string]any{}
	setMetaString(route, "method", cap.Method)
	setMetaString(route, "path", cap.Path)
	if len(route) > 0 {
		capability["route"] = route
	}

	governanceMeta := map[string]any{}
	setMetaString(governanceMeta, "actionMode", string(governance.ActionMode))
	setMetaString(governanceMeta, "approvalPolicy", string(governance.ApprovalPolicy))
	setMetaString(governanceMeta, "approvalSummary", governance.ApprovalSummary)
	if len(governanceMeta) > 0 {
		capability["governance"] = governanceMeta
	}

	if len(capability) == 0 {
		return nil
	}
	return CloneToolArguments(map[string]any{
		ToolMetaPulseCapabilityKey: capability,
	})
}

func setMetaString(meta map[string]any, key, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		meta[key] = trimmed
	}
}

func boolRef(v bool) *bool {
	return &v
}

func cloneBoolRef(v *bool) *bool {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}

// ToolInputSchema returns the agent-facing input schema for a capability. A
// manifest-owned schema is forwarded verbatim; otherwise a permissive adapter
// fallback is derived from path placeholders and request-body posture.
func ToolInputSchema(cap Capability) json.RawMessage {
	if len(cap.InputSchema) > 0 {
		return CloneRawMessage(cap.InputSchema)
	}

	properties := map[string]any{}
	required := []string{}
	for _, name := range PathParameterNames(cap.Path) {
		properties[name] = map[string]any{
			"type":        "string",
			"description": "Canonical " + name + " (e.g. \"vm:101\", \"container:web-1\")",
		}
		required = append(required, name)
	}
	if CapabilityHasRequestBody(cap) {
		desc := "Request body fields"
		if cap.RequestBodyShape != "" {
			desc = "Request body fields. Shape hint: " + cap.RequestBodyShape
		}
		properties["body"] = map[string]any{
			"type":                 "object",
			"description":          desc,
			"additionalProperties": true,
		}
	}
	return ObjectInputSchema(required, properties, true)
}

// ToolOutputSchema returns the manifest-owned structured output schema for a
// request/response capability. Unlike input schemas, there is no fallback: a
// responseShape string is a human-readable hint, not a validation contract for
// MCP structuredContent.
func ToolOutputSchema(cap Capability) json.RawMessage {
	return CloneRawMessage(cap.OutputSchema)
}

// ProjectedCall is the manifest-owned route/body projection for one capability
// invocation. Adapters still own their transport, auth headers, and response
// wrapping.
type ProjectedCall struct {
	Path    string
	Body    json.RawMessage
	HasBody bool
}

// ProjectCapabilityCall turns agent-facing tool arguments into the Pulse HTTP
// path and optional JSON body declared by the capability manifest.
func ProjectCapabilityCall(cap Capability, args map[string]any) (ProjectedCall, error) {
	path, err := SubstitutePathParameters(cap.Path, args)
	if err != nil {
		return ProjectedCall{}, err
	}

	projected := ProjectedCall{Path: path}
	if !CapabilityHasRequestBody(cap) {
		return projected, nil
	}

	bodyArg, ok := args["body"]
	if !ok {
		pathParams := PathParameterSet(cap.Path)
		cleaned := map[string]any{}
		for k, v := range PublicToolArguments(args) {
			if !pathParams[k] {
				cleaned[k] = v
			}
		}
		bodyArg = cleaned
	}

	body, err := json.Marshal(bodyArg)
	if err != nil {
		return ProjectedCall{}, err
	}
	projected.Body = body
	projected.HasBody = true
	return projected, nil
}

// CapabilityHasRequestBody reports whether adapters should send a JSON request
// body for a capability call.
func CapabilityHasRequestBody(cap Capability) bool {
	return !strings.EqualFold(cap.Method, http.MethodGet) && !strings.EqualFold(cap.Method, http.MethodDelete)
}

// PathParameterNames returns placeholder names declared by a capability path.
func PathParameterNames(path string) []string {
	matches := pathPlaceholderRE.FindAllStringSubmatch(path, -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match[1])
	}
	return names
}

// PathParameterSet returns the set of placeholder names declared in a capability
// path. Used by adapters to keep path arguments out of JSON request bodies.
func PathParameterSet(path string) map[string]bool {
	set := map[string]bool{}
	for _, name := range PathParameterNames(path) {
		set[name] = true
	}
	return set
}

// SubstitutePathParameters replaces `{name}` segments in a capability path with
// the corresponding argument value, encoded as one path segment.
func SubstitutePathParameters(path string, args map[string]any) (string, error) {
	var missing []string
	out := pathPlaceholderRE.ReplaceAllStringFunc(path, func(match string) string {
		name := match[1 : len(match)-1] // strip { and }
		v, ok := args[name]
		if !ok {
			missing = append(missing, name)
			return match
		}
		s, ok := v.(string)
		if !ok {
			missing = append(missing, name+" (not a string)")
			return match
		}
		return EscapePathSegmentParameter(s)
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("missing path argument(s): %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// EscapePathSegmentParameter percent-encodes a path parameter as a single URL
// path segment. It intentionally escapes `/`, `:`, spaces, and other reserved
// bytes.
func EscapePathSegmentParameter(value string) string {
	var b strings.Builder
	for _, c := range []byte(value) {
		if isUnreservedPathByte(c) {
			b.WriteByte(c)
			continue
		}
		_, _ = fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

func isUnreservedPathByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' ||
		c == '.' ||
		c == '_' ||
		c == '~'
}
