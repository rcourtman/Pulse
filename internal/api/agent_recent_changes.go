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

// agentRecentChangeSafeMetadata is the metadata a monitoring-read token keeps
// on command and runbook changes. Everything else on those records (command
// text, output excerpts, runbook messages and caller-supplied details) can
// carry command payloads and is dropped unless the token has ai:execute.
var agentRecentChangeSafeMetadata = map[string]bool{
	unified.MetadataAlertIdentifier: true,
	unified.MetadataSuccess:         true,
	unified.MetadataOutcome:         true,
	unified.MetadataAutomatic:       true,
	unified.MetadataRunbookID:       true,
	"title":                         true,
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

func redactAgentRecentChangeCommand(entry AgentRecentChange) AgentRecentChange {
	switch entry.Kind {
	case unified.ChangeCommandExecuted, unified.ChangeRunbookExecuted:
	default:
		return entry
	}
	safe := make(map[string]any, len(entry.Metadata))
	redacted := false
	for key, value := range entry.Metadata {
		if agentRecentChangeSafeMetadata[key] {
			safe[key] = value
			continue
		}
		redacted = true
	}
	entry.Metadata = safe
	if entry.Kind == unified.ChangeCommandExecuted {
		// The stored reason embeds the command ("Command succeeded: <cmd>").
		if prefix, _, found := strings.Cut(entry.Reason, ":"); found {
			entry.Reason = prefix
		}
		redacted = true
	}
	entry.CommandRedacted = redacted
	return entry
}
