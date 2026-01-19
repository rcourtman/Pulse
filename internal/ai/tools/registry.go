package tools

import (
	"context"
	"fmt"
	"sync"
)

// ToolHandler is a function that handles tool execution
type ToolHandler func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error)

// RegisteredTool combines a tool definition with its handler
type RegisteredTool struct {
	Definition     Tool
	Handler        ToolHandler
	RequireControl bool // If true, only available when control level is not read_only
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
			return NewTextResult("Control tools are disabled. Enable them in Settings > AI > Control Level."), nil
		}
	}

	return tool.Handler(ctx, e, args)
}
