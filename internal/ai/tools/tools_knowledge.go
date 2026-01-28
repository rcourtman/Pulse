package tools

import (
	"context"
	"fmt"
	"time"
)

// IncidentRecorderProvider provides access to incident recording data
type IncidentRecorderProvider interface {
	GetWindowsForResource(resourceID string, limit int) []*IncidentWindow
	GetWindow(windowID string) *IncidentWindow
}

// IncidentWindow represents a high-frequency recording window during an incident
type IncidentWindow struct {
	ID           string              `json:"id"`
	ResourceID   string              `json:"resource_id"`
	ResourceName string              `json:"resource_name,omitempty"`
	ResourceType string              `json:"resource_type,omitempty"`
	TriggerType  string              `json:"trigger_type"`
	TriggerID    string              `json:"trigger_id,omitempty"`
	StartTime    time.Time           `json:"start_time"`
	EndTime      *time.Time          `json:"end_time,omitempty"`
	Status       string              `json:"status"`
	DataPoints   []IncidentDataPoint `json:"data_points"`
	Summary      *IncidentSummary    `json:"summary,omitempty"`
}

// IncidentDataPoint represents a single data point in an incident window
type IncidentDataPoint struct {
	Timestamp time.Time          `json:"timestamp"`
	Metrics   map[string]float64 `json:"metrics"`
}

// IncidentSummary provides computed statistics about an incident window
type IncidentSummary struct {
	Duration   time.Duration      `json:"duration_ms"`
	DataPoints int                `json:"data_points"`
	Peaks      map[string]float64 `json:"peaks"`
	Lows       map[string]float64 `json:"lows"`
	Averages   map[string]float64 `json:"averages"`
	Changes    map[string]float64 `json:"changes"`
}

// EventCorrelatorProvider provides access to correlated events
type EventCorrelatorProvider interface {
	GetCorrelationsForResource(resourceID string, window time.Duration) []EventCorrelation
}

// EventCorrelation represents a correlated event
type EventCorrelation struct {
	EventType    string                 `json:"event_type"`
	Timestamp    time.Time              `json:"timestamp"`
	ResourceID   string                 `json:"resource_id"`
	ResourceName string                 `json:"resource_name,omitempty"`
	Description  string                 `json:"description"`
	Severity     string                 `json:"severity,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TopologyProvider provides access to resource relationships
type TopologyProvider interface {
	GetRelatedResources(resourceID string, depth int) []RelatedResource
}

// RelatedResource represents a resource related to another resource
type RelatedResource struct {
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	ResourceType string `json:"resource_type"`
	Relationship string `json:"relationship"`
}

// KnowledgeStoreProvider provides access to stored knowledge/notes
type KnowledgeStoreProvider interface {
	SaveNote(resourceID, note, category string) error
	GetKnowledge(resourceID string, category string) []KnowledgeEntry
}

// KnowledgeEntry represents a stored note about a resource
type KnowledgeEntry struct {
	ID         string    `json:"id"`
	ResourceID string    `json:"resource_id"`
	Note       string    `json:"note"`
	Category   string    `json:"category,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// registerKnowledgeTools registers the pulse_knowledge tool
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

// Tool handler implementations

func (e *PulseToolExecutor) executeGetIncidentWindow(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)
	windowID, _ := args["window_id"].(string)
	limit := intArg(args, "limit", 5)

	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}

	if e.incidentRecorderProvider == nil {
		return NewTextResult("Incident recording data not available. The incident recorder may not be enabled."), nil
	}

	// If a specific window ID is requested
	if windowID != "" {
		window := e.incidentRecorderProvider.GetWindow(windowID)
		if window == nil {
			return NewTextResult(fmt.Sprintf("Incident window '%s' not found.", windowID)), nil
		}
		return NewJSONResult(map[string]interface{}{
			"window": window,
		}), nil
	}

	// Get windows for the resource
	windows := e.incidentRecorderProvider.GetWindowsForResource(resourceID, limit)
	if len(windows) == 0 {
		return NewTextResult(fmt.Sprintf("No incident recording data found for resource '%s'. Incident data is captured when alerts fire.", resourceID)), nil
	}

	return NewJSONResult(map[string]interface{}{
		"resource_id": resourceID,
		"windows":     windows,
		"count":       len(windows),
	}), nil
}

func (e *PulseToolExecutor) executeCorrelateEvents(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)
	timestampStr, _ := args["timestamp"].(string)
	windowMinutes := intArg(args, "window_minutes", 15)

	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}

	if e.eventCorrelatorProvider == nil {
		return NewTextResult("Event correlation not available. The event correlator may not be enabled."), nil
	}

	// Parse timestamp or use now
	var timestamp time.Time
	if timestampStr != "" {
		var err error
		timestamp, err = time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return NewErrorResult(fmt.Errorf("invalid timestamp format: %w", err)), nil
		}
	} else {
		timestamp = time.Now()
	}

	window := time.Duration(windowMinutes) * time.Minute
	correlations := e.eventCorrelatorProvider.GetCorrelationsForResource(resourceID, window)

	if len(correlations) == 0 {
		return NewTextResult(fmt.Sprintf("No correlated events found for resource '%s' within %d minutes of %s.",
			resourceID, windowMinutes, timestamp.Format(time.RFC3339))), nil
	}

	return NewJSONResult(map[string]interface{}{
		"resource_id":    resourceID,
		"timestamp":      timestamp.Format(time.RFC3339),
		"window_minutes": windowMinutes,
		"events":         correlations,
		"count":          len(correlations),
	}), nil
}

func (e *PulseToolExecutor) executeGetRelationshipGraph(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)
	depth := intArg(args, "depth", 1)

	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}

	// Cap depth to prevent excessive traversal
	if depth < 1 {
		depth = 1
	}
	if depth > 3 {
		depth = 3
	}

	if e.topologyProvider == nil {
		return NewTextResult("Topology information not available."), nil
	}

	related := e.topologyProvider.GetRelatedResources(resourceID, depth)
	if len(related) == 0 {
		return NewTextResult(fmt.Sprintf("No relationships found for resource '%s'.", resourceID)), nil
	}

	return NewJSONResult(map[string]interface{}{
		"resource_id":       resourceID,
		"depth":             depth,
		"related_resources": related,
		"count":             len(related),
	}), nil
}

func (e *PulseToolExecutor) executeRemember(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)
	note, _ := args["note"].(string)
	category, _ := args["category"].(string)

	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}
	if note == "" {
		return NewErrorResult(fmt.Errorf("note is required")), nil
	}

	if e.knowledgeStoreProvider == nil {
		return NewTextResult("Knowledge storage not available."), nil
	}

	if err := e.knowledgeStoreProvider.SaveNote(resourceID, note, category); err != nil {
		return NewErrorResult(fmt.Errorf("failed to save note: %w", err)), nil
	}

	response := map[string]interface{}{
		"success":     true,
		"resource_id": resourceID,
		"note":        note,
		"message":     "Note saved successfully",
	}
	if category != "" {
		response["category"] = category
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeRecall(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)
	category, _ := args["category"].(string)

	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}

	if e.knowledgeStoreProvider == nil {
		return NewTextResult("Knowledge storage not available."), nil
	}

	entries := e.knowledgeStoreProvider.GetKnowledge(resourceID, category)
	if len(entries) == 0 {
		if category != "" {
			return NewTextResult(fmt.Sprintf("No notes found for resource '%s' in category '%s'.", resourceID, category)), nil
		}
		return NewTextResult(fmt.Sprintf("No notes found for resource '%s'.", resourceID)), nil
	}

	response := map[string]interface{}{
		"resource_id": resourceID,
		"notes":       entries,
		"count":       len(entries),
	}
	if category != "" {
		response["category"] = category
	}

	return NewJSONResult(response), nil
}
