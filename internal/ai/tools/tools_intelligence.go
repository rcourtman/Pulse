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

// registerIntelligenceTools registers AI intelligence tools for incident analysis and knowledge management
func (e *PulseToolExecutor) registerIntelligenceTools() {
	// Tool 1: pulse_get_incident_window
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_incident_window",
			Description: `Get high-resolution incident recording data for a resource.

Returns: JSON with incident windows containing high-frequency metrics captured during incidents, including timestamps, metric values, and summary statistics.

Use when: You need detailed metrics data from during an incident to analyze what happened, or to see metrics leading up to an alert.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "The resource ID to get incident data for",
					},
					"window_id": {
						Type:        "string",
						Description: "Optional: specific incident window ID. If omitted, returns most recent windows for the resource.",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of windows to return (default: 5)",
					},
				},
				Required: []string{"resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetIncidentWindow(ctx, args)
		},
	})

	// Tool 2: pulse_correlate_events
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_correlate_events",
			Description: `Get correlated Proxmox events around a timestamp for a resource.

Returns: JSON with events that occurred around the same time as an incident, helping identify root causes.

Use when: You want to understand what other events (config changes, migrations, other alerts) happened around the time of an incident.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "The resource ID to correlate events for",
					},
					"timestamp": {
						Type:        "string",
						Description: "Optional: ISO timestamp to center the search around. Defaults to now.",
					},
					"window_minutes": {
						Type:        "integer",
						Description: "Time window in minutes to search (default: 15)",
					},
				},
				Required: []string{"resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeCorrelateEvents(ctx, args)
		},
	})

	// Tool 3: pulse_get_relationship_graph
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_relationship_graph",
			Description: `Show resource dependencies (host, storage, network).

Returns: JSON with related resources including the host node, storage volumes, and other dependent resources.

Use when: You need to understand what a resource depends on or what might be affected by issues with a resource.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "The resource ID to get relationships for",
					},
					"depth": {
						Type:        "integer",
						Description: "How many levels of relationships to traverse (default: 1, max: 3)",
					},
				},
				Required: []string{"resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetRelationshipGraph(ctx, args)
		},
	})

	// Tool 4: pulse_remember
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_remember",
			Description: `Save a note to memory about a resource.

Use when: You learn something important about a resource that should be remembered for future reference, such as its purpose, known issues, maintenance schedules, or owner information.

Notes are persisted and will be available in future conversations.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "The resource ID to save a note about",
					},
					"note": {
						Type:        "string",
						Description: "The note to save about this resource",
					},
					"category": {
						Type:        "string",
						Description: "Optional category for the note (e.g., 'purpose', 'owner', 'maintenance', 'issue')",
					},
				},
				Required: []string{"resource_id", "note"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeRemember(ctx, args)
		},
	})

	// Tool 5: pulse_recall
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_recall",
			Description: `Recall saved notes about a resource.

Returns: JSON with all saved notes about a resource, optionally filtered by category.

Use when: You need to remember previously stored information about a resource, such as its purpose or known issues.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "The resource ID to recall notes about",
					},
					"category": {
						Type:        "string",
						Description: "Optional: filter notes by category",
					},
				},
				Required: []string{"resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeRecall(ctx, args)
		},
	})
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
