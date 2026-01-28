package tools

import (
	"context"
	"fmt"
)

// registerKnowledgeTools registers the consolidated pulse_knowledge tool
func (e *PulseToolExecutor) registerKnowledgeTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_knowledge",
			Description: `Manage AI knowledge, notes, and incident analysis.

Actions:
- remember: Save a note about a resource for future reference
- recall: Retrieve saved notes about a resource
- incidents: Get high-resolution incident recording data
- correlate: Get correlated events around a timestamp
- relationships: Get resource dependency graph

Examples:
- Save note: action="remember", resource_id="101", note="Production database server", category="purpose"
- Recall: action="recall", resource_id="101"
- Get incidents: action="incidents", resource_id="101"
- Correlate events: action="correlate", resource_id="101", window_minutes=30
- Get relationships: action="relationships", resource_id="101"`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "Knowledge action to perform",
						Enum:        []string{"remember", "recall", "incidents", "correlate", "relationships"},
					},
					"resource_id": {
						Type:        "string",
						Description: "Resource ID to operate on",
					},
					"note": {
						Type:        "string",
						Description: "For remember: the note to save",
					},
					"category": {
						Type:        "string",
						Description: "For remember/recall: note category (purpose, owner, maintenance, issue)",
					},
					"window_id": {
						Type:        "string",
						Description: "For incidents: specific incident window ID",
					},
					"timestamp": {
						Type:        "string",
						Description: "For correlate: ISO timestamp to center search around (default: now)",
					},
					"window_minutes": {
						Type:        "integer",
						Description: "For correlate: time window in minutes (default: 15)",
					},
					"depth": {
						Type:        "integer",
						Description: "For relationships: levels to traverse (default: 1, max: 3)",
					},
					"limit": {
						Type:        "integer",
						Description: "For incidents: max windows to return (default: 5)",
					},
				},
				Required: []string{"action", "resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeKnowledge(ctx, args)
		},
	})
}

// executeKnowledge routes to the appropriate knowledge handler based on action
// Handler functions are implemented in tools_intelligence.go
func (e *PulseToolExecutor) executeKnowledge(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	action, _ := args["action"].(string)
	switch action {
	case "remember":
		return e.executeRemember(ctx, args)
	case "recall":
		return e.executeRecall(ctx, args)
	case "incidents":
		return e.executeGetIncidentWindow(ctx, args)
	case "correlate":
		return e.executeCorrelateEvents(ctx, args)
	case "relationships":
		return e.executeGetRelationshipGraph(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown action: %s. Use: remember, recall, incidents, correlate, relationships", action)), nil
	}
}
