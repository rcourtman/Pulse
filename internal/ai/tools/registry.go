package tools

import (
	"context"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// ToolActionMode describes the state-changing capability of a registered tool.
// It aliases the shared Pulse Intelligence capability action mode so Assistant
// and external-agent manifests cannot drift on the read/mixed/write vocabulary.
type ToolActionMode = agentcapabilities.ActionMode

const (
	ToolActionRead  ToolActionMode = agentcapabilities.ActionModeRead
	ToolActionMixed ToolActionMode = agentcapabilities.ActionModeMixed
	ToolActionWrite ToolActionMode = agentcapabilities.ActionModeWrite
)

// ToolApprovalPolicy describes whether a tool can run with its granted scope or
// must participate in Pulse's governed action-plan approval lifecycle. It
// aliases the shared Pulse Intelligence approval vocabulary so Assistant
// prompts and external-agent manifests cannot drift.
type ToolApprovalPolicy = agentcapabilities.ApprovalPolicy

const (
	ToolApprovalScopeOnly  ToolApprovalPolicy = agentcapabilities.ApprovalPolicyScopeOnly
	ToolApprovalActionPlan ToolApprovalPolicy = agentcapabilities.ApprovalPolicyActionPlan
)

// ControlLevel represents the Assistant permission level for infrastructure
// control. It aliases the shared Pulse Intelligence control vocabulary so
// Assistant tool availability and external-agent adapters cannot drift.
type ControlLevel = agentcapabilities.ControlLevel

const (
	// ControlLevelReadOnly - AI can only query, no control tools available
	ControlLevelReadOnly ControlLevel = agentcapabilities.ControlLevelReadOnly
	// ControlLevelControlled - AI can execute with per-command approval
	ControlLevelControlled ControlLevel = agentcapabilities.ControlLevelControlled
	// ControlLevelAutonomous - AI executes without approval (requires Pro license)
	ControlLevelAutonomous ControlLevel = agentcapabilities.ControlLevelAutonomous
)

// ToolGovernance records the operator-facing governance contract for a tool.
// It aliases the shared Pulse Intelligence shape so Assistant and
// external-agent governance descriptors cannot drift.
type ToolGovernance = agentcapabilities.ToolGovernance

// ToolGovernanceDescriptor is the read-only manifest used by Assistant prompts
// and action-governance surfaces.
type ToolGovernanceDescriptor = agentcapabilities.ToolGovernanceDescriptor

// ToolHandler is a function that handles tool execution
type ToolHandler func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error)

// RegisteredTool combines a tool definition with its handler
type RegisteredTool struct {
	Definition     Tool
	Handler        ToolHandler
	RequireControl bool // If true, only available when control level is not read_only
	Governance     ToolGovernance
}

// ToolRegistry manages tool registration and execution
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]RegisteredTool
	order []string // Preserve registration order
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]RegisteredTool),
		order: make([]string, 0),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool RegisteredTool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool.Definition = tool.Definition.NormalizeCollections()
	name := tool.Definition.Name
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = tool
}

// ListTools returns all tools available for the given control level
func (r *ToolRegistry) ListTools(controlLevel ControlLevel) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.tools))
	for _, name := range r.order {
		tool := r.tools[name]
		// Skip control tools if in read-only mode
		if tool.RequireControl && !agentcapabilities.ControlLevelAllowsControlTools(controlLevel) {
			continue
		}
		result = append(result, tool.Definition.NormalizeCollections())
	}
	return result
}

// ListToolGovernance returns the governed tool manifest available at a control level.
func (r *ToolRegistry) ListToolGovernance(controlLevel ControlLevel) []ToolGovernanceDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ToolGovernanceDescriptor, 0, len(r.tools))
	for _, name := range r.order {
		tool := r.tools[name]
		if tool.RequireControl && !agentcapabilities.ControlLevelAllowsControlTools(controlLevel) {
			continue
		}
		result = append(result, agentcapabilities.NewToolGovernanceDescriptor(
			tool.Definition.Name,
			tool.Definition.Description,
			tool.RequireControl,
			tool.Governance,
		))
	}
	return result
}

// allNames returns the canonical list of registered tool names in
// registration order. Internal helper for KnownToolNames.
func (r *ToolRegistry) allNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Execute runs a tool by name
func (r *ToolRegistry) Execute(ctx context.Context, e *PulseToolExecutor, name string, args map[string]interface{}) (CallToolResult, error) {
	params, invalidResult, ok := agentcapabilities.PrepareToolRegistryExecution(name, args)
	if !ok {
		return invalidResult, nil
	}
	name = params.Name
	args = params.Arguments

	r.mu.RLock()
	tool, exists := r.tools[name]
	r.mu.RUnlock()

	if !exists {
		return agentcapabilities.NewUnknownToolResult(name), nil
	}

	// Centralized control level check
	if tool.RequireControl {
		if !agentcapabilities.ControlLevelAllowsControlTools(e.controlLevel) {
			return agentcapabilities.NewControlToolsDisabledToolResult(), nil
		}
	}

	result, err := tool.Handler(ctx, e, args)
	return result.NormalizeCollections(), err
}
