package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// IncidentArchiveProvider provides explicit, resource-bound reads of saved
// legacy recordings. Live incident evidence comes from the canonical timeline.
type IncidentArchiveProvider interface {
	GetWindow(resourceID, windowID string) (*metrics.IncidentWindow, error)
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
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name: agentcapabilities.PulseKnowledgeToolName,
			Description: `Manage AI knowledge, notes, and incident analysis.

Actions:
- remember: Save a note about a resource for future reference
- recall: Retrieve saved notes about a resource
- incidents: Read retained canonical resource history, including observed state changes, alerts and executed actions. Records preserve observation time, source and any known occurrence time. This is not continuous health or filesystem-capacity coverage. Use pulse_summarize for retained metrics. For a specific action decision or verified execution outcome, use pulse_query action=action with its action_id. Missing timeline events do not prove an action was never executed.
- correlate: Get correlated events around a timestamp

Examples:
- Save note: action="remember", resource_id="101", note="Production database server", category="purpose"
- Recall: action="recall", resource_id="101"
- Get incidents: action="incidents", resource_id="101"
- Correlate events: action="correlate", resource_id="101", window_minutes=30`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "Knowledge action to perform",
						Enum:        []string{"remember", "recall", "incidents", "correlate"},
					},
					"resource_id": {
						Type:        "string",
						Description: "Resource ID to operate on. For incidents use the canonical resource ID returned by pulse_query, including for a resource no longer in current inventory.",
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
						Description: "For incidents: optional legacy recording ID, read as an archive only. Omit to read canonical resource history.",
					},
					"since": {
						Type:        "string",
						Description: "For incidents: earliest observation timestamp (RFC3339, default 24 hours ago). Retention and collection gaps still apply.",
					},
					"timestamp": {
						Type:        "string",
						Description: "For correlate: ISO timestamp to center search around (default: now)",
					},
					"window_minutes": {
						Type:        "integer",
						Description: "For correlate: time window in minutes (default: 15)",
					},
					"limit": {
						Type:        "integer",
						Description: "For incidents: maximum retained events to return, newest first (default 50, range 1-200)",
					},
				},
				Required: []string{"action", "resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeKnowledge(ctx, args)
		},
		Governance: ToolGovernance{
			ActionMode:      ToolActionMixed,
			ApprovalPolicy:  ToolApprovalScopeOnly,
			ApprovalSummary: "recall and analysis are safe; remember records operator-visible knowledge",
			Summary:         "Reads operational memory and records governed knowledge notes when requested.",
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
	default:
		return NewErrorResult(fmt.Errorf("unknown action: %s. Use: remember, recall, incidents, correlate", action)), nil
	}
}

// Tool handler implementations

func (e *PulseToolExecutor) executeGetIncidentWindow(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)
	resourceID = strings.TrimSpace(resourceID)
	windowID, _ := args["window_id"].(string)

	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}

	// Isolate legacy recordings from the canonical timeline. Their sample times
	// are recorder timestamps, not verified source observation timestamps.
	if windowID != "" {
		if e.incidentArchiveProvider == nil {
			return NewErrorResult(fmt.Errorf("legacy incident recording archive is unavailable")), nil
		}
		window, err := e.incidentArchiveProvider.GetWindow(resourceID, windowID)
		if err != nil {
			return NewErrorResult(fmt.Errorf("read legacy incident recording archive: %w", err)), nil
		}
		if window == nil || window.ResourceID != resourceID || window.ID != windowID {
			return NewErrorResult(fmt.Errorf("legacy incident recording not found for the requested resource")), nil
		}
		return NewJSONResult(map[string]interface{}{
			"source":                "legacy_incident_recording",
			"archive_read_only":     true,
			"summary_duration_unit": "nanoseconds",
			"window":                window,
			"evidence_limit":        "Recording timestamps do not establish when the source measured each value. Repeated values may be cached observations. The legacy summary.duration_ms field contains nanoseconds. Stored recording status is historical and does not mean recording is active. This archive is not the canonical incident timeline.",
		}), nil
	}

	limit := intArg(args, "limit", 50)
	if limit < 1 || limit > 200 {
		return NewErrorResult(fmt.Errorf("limit must be between 1 and 200")), nil
	}
	queriedAt := time.Now().UTC()
	since := queriedAt.Add(-24 * time.Hour)
	if value, exists := args["since"]; exists {
		text, ok := value.(string)
		if !ok {
			return NewErrorResult(fmt.Errorf("since must be an RFC3339 timestamp")), nil
		}
		var err error
		since, err = time.Parse(time.RFC3339, text)
		if err != nil || since.After(queriedAt) {
			return NewErrorResult(fmt.Errorf("since must be an RFC3339 timestamp no later than now")), nil
		}
	}
	if e.actionAuditStore == nil {
		return NewErrorResult(fmt.Errorf("canonical resource history is unavailable")), nil
	}
	// This organization-pinned store is also used by the resource history API
	// and Assistant handoffs. Do not reconstruct history from current metrics,
	// match resource names, or include adjacent resources implicitly.
	events, err := e.actionAuditStore.GetRecentChanges(resourceID, since, limit+1)
	if err != nil {
		return NewErrorResult(fmt.Errorf("read canonical resource history: %w", err)), nil
	}
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}
	if events == nil {
		events = []unifiedresources.ResourceChange{}
	}

	return NewJSONResult(map[string]interface{}{
		"resource_id":    resourceID,
		"source":         "canonical_resource_timeline",
		"since":          since,
		"queried_at":     queriedAt,
		"time_basis":     "observed_at",
		"events":         events,
		"count":          len(events),
		"limit":          limit,
		"has_more":       hasMore,
		"coverage":       "retained_records_only",
		"evidence_limit": "These are retained observations, not continuous coverage. Empty history does not establish health or absence of incidents. ObservedAt is when Pulse observed a change, while OccurredAt is present only when its occurrence time is known. A resolution records alert closure, not its cause, workload recovery or a verified action outcome. RelatedResources identifies relationships, not additional targets of this event. This read does not query action records.",
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
