package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// registerProposeTools registers the investigation-only typed action
// proposal capture. The registry policy rejects this tool outside the
// Patrol investigation profile before this handler runs; the handler
// additionally requires the request-local capture sink, which only the
// core investigation entrypoint wires.
func (e *PulseToolExecutor) registerProposeTools() {
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name:        agentcapabilities.PatrolActionCapabilitiesToolName,
			Description: `Read the typed remediation capabilities currently advertised by a canonical resource. Use this after investigation evidence identifies the resource that should actually be changed; the causal resource may differ from the resource named by the original finding. Side-effect-free: this only reads capability names and parameter schemas.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "Canonical resource ID whose advertised typed actions should be inspected",
					},
				},
				Required: []string{"resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeActionCapabilities(ctx, args)
		},
		Governance: ToolGovernance{
			ActionMode:      ToolActionRead,
			ApprovalPolicy:  ToolApprovalScopeOnly,
			ApprovalSummary: "side-effect-free lookup of the canonical resource capability catalog",
			Summary:         "Reads typed actions advertised by an investigation-selected resource; it grants no planning, approval, or execution authority.",
		},
	})

	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name: agentcapabilities.PatrolProposeActionToolName,
			Description: `Propose ONE typed remediation action for the finding under investigation. Side-effect-free: the proposal is validated and recorded for governed planning and approval; nothing executes now.

Reference an advertised resource capability (see the resource's capability catalog) and fill only its declared parameters. Never place secrets in params - sensitive parameters are supplied by an operator at approval time.

Submit at most one proposal per investigation. If no safe remediation exists, conclude without proposing.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "Canonical resource ID the action targets",
					},
					"capability_name": {
						Type:        "string",
						Description: "Advertised capability name on that resource",
					},
					"params": {
						Type:        "object",
						Description: "Values for the capability's declared parameters (non-sensitive only)",
					},
					"reason": {
						Type:        "string",
						Description: "Why this action remediates the finding",
					},
				},
				Required: []string{"resource_id", "capability_name", "reason"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeProposeAction(ctx, args)
		},
		Governance: ToolGovernance{
			ActionMode:      ToolActionRead,
			ApprovalPolicy:  ToolApprovalScopeOnly,
			ApprovalSummary: "side-effect-free proposal capture; the proposed action itself is planned and approved on the canonical action lifecycle",
			Summary:         "Records one validated typed action proposal during a Patrol investigation; planning, approval, and execution stay governed.",
		},
	})
}

type investigationCapabilityCatalogResponse struct {
	ResourceID   string                            `json:"resource_id"`
	Capabilities []investigationCapabilityResponse `json:"capabilities"`
}

type investigationCapabilityResponse struct {
	Name                 string                                 `json:"name"`
	Description          string                                 `json:"description,omitempty"`
	MinimumApprovalLevel string                                 `json:"minimum_approval_level"`
	AutoAuthorization    string                                 `json:"auto_authorization"`
	Platform             string                                 `json:"platform,omitempty"`
	Params               []investigationCapabilityParamResponse `json:"params"`
}

type investigationCapabilityParamResponse struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Enum        []string `json:"enum"`
	Pattern     string   `json:"pattern,omitempty"`
	Description string   `json:"description,omitempty"`
	Sensitive   bool     `json:"sensitive"`
}

func (e *PulseToolExecutor) executeActionCapabilities(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.proposalCapture == nil {
		return NewErrorResult(fmt.Errorf("action capabilities are not available in this run")), nil
	}
	resourceID := e.canonicalProposalResourceID(stringArg(args, "resource_id"))
	capabilities, err := e.proposalCapture.Capabilities(ctx, resourceID)
	if err != nil {
		return NewErrorResult(fmt.Errorf("capability catalog lookup failed for resource %q", resourceID)), nil
	}
	response := investigationCapabilityCatalogResponse{
		ResourceID:   resourceID,
		Capabilities: make([]investigationCapabilityResponse, 0, len(capabilities)),
	}
	for _, capability := range capabilities {
		item := investigationCapabilityResponse{
			Name:                 capability.Name,
			Description:          capability.Description,
			MinimumApprovalLevel: string(capability.MinimumApprovalLevel),
			AutoAuthorization:    string(unified.NormalizeActionAutoAuthorizationClass(capability.AutoAuthorization)),
			Platform:             capability.Platform,
			Params:               make([]investigationCapabilityParamResponse, 0, len(capability.Params)),
		}
		for _, param := range capability.Params {
			item.Params = append(item.Params, investigationCapabilityParamResponse{
				Name: param.Name, Type: param.Type, Required: param.Required,
				Enum: append([]string(nil), param.Enum...), Pattern: param.Pattern,
				Description: param.Description, Sensitive: param.IsSensitive,
			})
		}
		response.Capabilities = append(response.Capabilities, item)
	}
	return NewJSONResult(response), nil
}

// executeProposeAction validates and captures a typed action proposal.
// Success and error outputs never echo parameter values.
func (e *PulseToolExecutor) executeProposeAction(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	capture := e.proposalCapture
	if capture == nil {
		return NewErrorResult(fmt.Errorf("action proposals are not available in this run")), nil
	}

	resourceID := e.canonicalProposalResourceID(stringArg(args, "resource_id"))
	capabilityName := strings.TrimSpace(stringArg(args, "capability_name"))
	reason := strings.TrimSpace(stringArg(args, "reason"))
	params, _ := args["params"].(map[string]interface{})
	if params == nil {
		params = map[string]interface{}{}
	}
	if resourceID == "" || capabilityName == "" || reason == "" {
		capture.RecordFailedAttempt()
		return NewErrorResult(fmt.Errorf("resource_id, capability_name, and reason are required")), nil
	}

	if err := validateProposalAgainstCatalog(ctx, capture.catalog, resourceID, capabilityName, params); err != nil {
		capture.RecordFailedAttempt()
		return NewErrorResult(err), nil
	}

	if err := capture.Submit(InvocationIDFromContext(ctx), resourceID, capabilityName, reason, params); err != nil {
		return NewErrorResult(err), nil
	}
	return NewTextResult(fmt.Sprintf(
		"Proposal recorded: capability %q on resource %q. It will be planned and routed for governed approval; nothing has executed. Conclude the investigation with your diagnosis.",
		capabilityName, resourceID)), nil
}

// canonicalProposalResourceID keeps provider coordinates discovered during an
// investigation from leaking into the governed action lifecycle. Only an exact,
// unique app-container match is translated; ambiguous or unknown references
// continue to the catalog unchanged and fail closed there.
func (e *PulseToolExecutor) canonicalProposalResourceID(reference string) string {
	reference = unified.CanonicalResourceID(reference)
	if reference == "" || e.unifiedResourceProvider == nil {
		return reference
	}

	resolvedID := ""
	for _, resource := range e.unifiedResourceProvider.GetByType(unified.ResourceTypeAppContainer) {
		if !matchesAppContainerActionReference(resource, reference) {
			continue
		}
		candidateID := unified.CanonicalResourceID(resource.ID)
		if candidateID == "" {
			continue
		}
		if resolvedID != "" && resolvedID != candidateID {
			return reference
		}
		resolvedID = candidateID
	}
	if resolvedID != "" {
		return resolvedID
	}
	return reference
}

func matchesAppContainerActionReference(resource unified.Resource, reference string) bool {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return false
	}
	canonicalID := unified.CanonicalResourceID(resource.ID)
	providerID := appContainerProviderID(resource)
	if strings.EqualFold(reference, canonicalID) || strings.EqualFold(reference, providerID) {
		return true
	}
	if resource.Docker == nil || providerID == "" {
		return false
	}
	for _, host := range []string{resource.Docker.AgentID, resource.Docker.HostSourceID, resource.Docker.Hostname} {
		host = strings.TrimSpace(host)
		if host != "" && strings.EqualFold(reference, "docker:"+host+":"+providerID) {
			return true
		}
	}
	return false
}

func stringArg(args map[string]interface{}, key string) string {
	value, _ := args[key].(string)
	return value
}

// SetProposalCapture wires the request-local proposal sink. Only the core
// investigation entrypoint calls this, on the effective request executor;
// clones share the sink deliberately so one run has exactly one capture.
func (e *PulseToolExecutor) SetProposalCapture(capture *ProposalCapture) {
	e.proposalCapture = capture
}
