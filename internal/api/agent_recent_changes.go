package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// AgentRecentChange is one observed resource change as an agent reads it:
// the canonical change record plus the resource's current name and type,
// which the store does not keep. ResourceName and ResourceType are empty when
// the resource is no longer known; the history itself stays readable.
type AgentRecentChange struct {
	unified.ResourceChange
	ResourceName    string `json:"resourceName,omitempty"`
	ResourceType    string `json:"resourceType,omitempty"`
	CommandRedacted bool   `json:"commandRedacted,omitempty"`
}

// AgentRecentChanges is the response for GET /api/agent/changes. Changes is
// always an array (never null). Count is the total matching the window and
// filters; Truncated says the agent should narrow the read rather than page,
// because one registry rebuild records many changes at the same instant and a
// timestamp cursor would skip some of them.
type AgentRecentChanges struct {
	Changes     []AgentRecentChange `json:"changes"`
	Count       int                 `json:"count"`
	Truncated   bool                `json:"truncated"`
	Since       time.Time           `json:"since"`
	Before      *time.Time          `json:"before,omitempty"`
	Limit       int                 `json:"limit"`
	GeneratedAt time.Time           `json:"generatedAt"`
}

// HandleRecentChanges serves `GET /api/agent/changes`: what Pulse observed
// changing across the fleet, newest first, read from the existing resource
// change store. It records nothing, so it adds no disk writes.
func (h *AgentContextHandler) HandleRecentChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.resources == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	query, parseError := parseAgentRecentChangesQuery(r, now)
	if parseError != "" {
		http.Error(w, parseError, http.StatusBadRequest)
		return
	}

	orgID := GetOrgID(r.Context())
	store, err := h.resources.getStore(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	changes, err := store.GetRecentChangesFiltered(query.resourceID, query.since, query.limit, query.filters)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	count, err := store.CountRecentChangesFiltered(query.resourceID, query.since, query.filters)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}

	// Names are a convenience; a registry failure must not hide the history.
	registry, _ := h.resources.buildRegistry(orgID)
	canReadCommands := requestCanReadAgentCommandPayloads(r)
	out := AgentRecentChanges{
		Changes:     make([]AgentRecentChange, 0, len(changes)),
		Count:       count,
		Truncated:   count > len(changes),
		Since:       query.since,
		Before:      query.filters.ObservedBefore,
		Limit:       query.limit,
		GeneratedAt: now,
	}
	for _, change := range changes {
		entry := AgentRecentChange{ResourceChange: change}
		if registry != nil {
			if resource, ok := registry.Get(unified.CanonicalResourceID(change.ResourceID)); ok && resource != nil {
				entry.ResourceName = resource.Name
				entry.ResourceType = string(resource.Type)
			}
		}
		if !canReadCommands {
			entry = redactAgentRecentChangeCommand(entry)
		}
		out.Changes = append(out.Changes, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type agentRecentChangesQuery struct {
	since      time.Time
	resourceID string
	limit      int
	filters    unified.ResourceChangeFilters
}

// parseAgentRecentChangesQuery reads the list_recent_changes filters. Unlike
// fleet-context, malformed values are a 400 with a reason: an agent that sent
// a bad timestamp or kind should correct it, not silently read the wrong
// window. An oversized limit is capped rather than refused.
func parseAgentRecentChangesQuery(r *http.Request, now time.Time) (agentRecentChangesQuery, string) {
	q := r.URL.Query()
	query := agentRecentChangesQuery{
		since: now.Add(-agentcapabilities.RecentChangesDefaultWindow),
		limit: agentcapabilities.RecentChangesDefaultLimit,
	}
	if raw := strings.TrimSpace(q.Get("since")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return query, "Invalid since value: use an RFC 3339 timestamp"
		}
		query.since = parsed.UTC()
	}
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return query, "Invalid limit value: use a positive integer"
		}
		query.limit = min(parsed, agentcapabilities.RecentChangesMaxLimit)
	}

	filters, err := unified.ParseResourceChangeFilters(q["kind"], q["sourceType"], nil)
	if err != nil {
		return query, err.Error()
	}
	if raw := strings.TrimSpace(q.Get("before")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return query, "Invalid before value: use an RFC 3339 timestamp"
		}
		before := parsed.UTC()
		if !before.After(query.since) {
			return query, "Invalid window: before must be later than since"
		}
		filters.ObservedBefore = &before
	}
	if raw := strings.TrimSpace(q.Get(agentcapabilities.ResourceIDArgumentName)); raw != "" {
		query.resourceID = unified.CanonicalResourceID(raw)
		filters.IncludeRelated = true
	}
	query.filters = filters
	return query, ""
}

// redactAgentRecentChangeCommand projects execution history, not arbitrary
// stored metadata. A familiar key does not make its value safe: command details
// can supply runbook keys, nested objects, or free text under any key.
func redactAgentRecentChangeCommand(entry AgentRecentChange) AgentRecentChange {
	switch entry.Kind {
	case unified.ChangeCommandExecuted, unified.ChangeRunbookExecuted:
	default:
		return entry
	}

	safe := make(map[string]any)
	if id, ok := entry.Metadata[unified.MetadataAlertIdentifier].(string); ok {
		safe[unified.MetadataAlertIdentifier] = id
	}
	if entry.Kind == unified.ChangeCommandExecuted {
		entry.Reason = "Command executed"
		if success, ok := entry.Metadata[unified.MetadataSuccess].(bool); ok {
			safe[unified.MetadataSuccess] = success
			entry.Reason = "Command failed"
			if success {
				entry.Reason = "Command succeeded"
			}
		}
	} else {
		entry.Reason = "Runbook executed"
		if id, ok := entry.Metadata[unified.MetadataRunbookID].(string); ok {
			safe[unified.MetadataRunbookID] = id
		}
		if automatic, ok := entry.Metadata[unified.MetadataAutomatic].(bool); ok {
			safe[unified.MetadataAutomatic] = automatic
		}
		// These are the incident memory's closed outcome values. Unknown or
		// malformed stored values stay undisclosed, never a guessed success.
		if outcome, ok := entry.Metadata[unified.MetadataOutcome].(string); ok {
			switch outcome {
			case "resolved", "partial", "failed", "unknown":
				safe[unified.MetadataOutcome] = outcome
				entry.Reason += " (" + outcome + ")"
			}
		}
	}
	// The canonical builders do not use From/To for execution records. Do
	// not expose free-text legacy values or derive a reason from stored text.
	entry.From = ""
	entry.To = ""
	entry.Metadata = safe
	entry.CommandRedacted = true
	return entry
}
