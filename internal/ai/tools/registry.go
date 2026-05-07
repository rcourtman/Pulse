package tools

import (
	"context"
	"fmt"
	"sync"
)

// ToolActionMode describes the state-changing capability of a registered tool.
type ToolActionMode string

const (
	ToolActionRead  ToolActionMode = "read"
	ToolActionMixed ToolActionMode = "mixed"
	ToolActionWrite ToolActionMode = "write"
)

const assistantControlToolsDisabledMessage = "Control tools are disabled. Open Assistant & Patrol settings, then set Pulse Assistant Permissions > Control mode to Controlled before using action tools."

// ToolGovernance records the operator-facing governance contract for a tool.
type ToolGovernance struct {
	ActionMode     ToolActionMode
	ApprovalPolicy string
	Summary        string
}

// ToolGovernanceDescriptor is the read-only manifest used by Assistant prompts
// and action-governance surfaces.
type ToolGovernanceDescriptor struct {
	Name           string
	Description    string
	RequireControl bool
	ActionMode     ToolActionMode
	ApprovalPolicy string
	Summary        string
}

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
		if tool.RequireControl && (controlLevel == ControlLevelReadOnly || controlLevel == "") {
			continue
		}
		result = append(result, tool.Definition)
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
		if tool.RequireControl && (controlLevel == ControlLevelReadOnly || controlLevel == "") {
			continue
		}
		governance := normalizeToolGovernance(tool)
		result = append(result, ToolGovernanceDescriptor{
			Name:           tool.Definition.Name,
			Description:    tool.Definition.Description,
			RequireControl: tool.RequireControl,
			ActionMode:     governance.ActionMode,
			ApprovalPolicy: governance.ApprovalPolicy,
			Summary:        governance.Summary,
		})
	}
	return result
}

func normalizeToolGovernance(tool RegisteredTool) ToolGovernance {
	governance := tool.Governance
	if governance.ActionMode == "" {
		if tool.RequireControl {
			governance.ActionMode = ToolActionWrite
		} else {
			governance.ActionMode = ToolActionRead
		}
	}
	if governance.ApprovalPolicy == "" {
		if governance.ActionMode == ToolActionRead {
			governance.ApprovalPolicy = "no approval required"
		} else if tool.RequireControl {
			governance.ApprovalPolicy = "hidden in read-only mode; approval required in controlled mode"
		} else {
			governance.ApprovalPolicy = "write subactions require the tool's governed approval path"
		}
	}
	if governance.Summary == "" {
		governance.Summary = tool.Definition.Description
	}
	return governance
}

// Execute runs a tool by name
func (r *ToolRegistry) Execute(ctx context.Context, e *PulseToolExecutor, name string, args map[string]interface{}) (CallToolResult, error) {
	r.mu.RLock()
	tool, exists := r.tools[name]
	r.mu.RUnlock()

	if !exists {
		return NewErrorResult(fmt.Errorf("unknown tool: %s", name)), nil
	}

	// Centralized control level check
	if tool.RequireControl {
		if e.controlLevel == ControlLevelReadOnly || e.controlLevel == "" {
			return NewTextResult(assistantControlToolsDisabledMessage), nil
		}
	}

	return tool.Handler(ctx, e, args)
}
