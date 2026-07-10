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

// executeProposeAction validates and captures a typed action proposal.
// Success and error outputs never echo parameter values.
func (e *PulseToolExecutor) executeProposeAction(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	capture := e.proposalCapture
	if capture == nil {
		return NewErrorResult(fmt.Errorf("action proposals are not available in this run")), nil
	}

	resourceID := unified.CanonicalResourceID(stringArg(args, "resource_id"))
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
