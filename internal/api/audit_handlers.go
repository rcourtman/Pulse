package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

// AuditHandlers provides HTTP handlers for audit log endpoints.
type AuditHandlers struct{}

// NewAuditHandlers creates a new AuditHandlers instance.
func NewAuditHandlers() *AuditHandlers {
	return &AuditHandlers{}
}

// HandleListAuditEvents returns audit events matching query parameters.
// Query params: user, event, startTime, endTime, limit, offset, success
func (h *AuditHandlers) HandleListAuditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()

	filter := audit.QueryFilter{
		EventType: query.Get("event"),
		User:      query.Get("user"),
	}

	// Parse limit
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	} else {
		filter.Limit = 100 // Default limit
	}

	// Parse offset
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	// Parse startTime
	if startStr := query.Get("startTime"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			filter.StartTime = &t
		}
	}

	// Parse endTime
	if endStr := query.Get("endTime"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			filter.EndTime = &t
		}
	}

	// Parse success
	if successStr := query.Get("success"); successStr != "" {
		success := successStr == "true"
		filter.Success = &success
	}

	logger := audit.GetLogger()

	// Query events from the current logger
	events, err := logger.Query(filter)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "query_failed", "Failed to query audit events", nil)
		return
	}

	countFilter := filter
	countFilter.Limit = 0
	countFilter.Offset = 0

	totalCount, err := logger.Count(countFilter)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "query_failed", "Failed to count audit events", nil)
		return
	}

	// For OSS (ConsoleLogger), events will be empty
	// Return a response indicating the feature status
	response := map[string]interface{}{
		"events":            events,
		"total":             totalCount,
		"persistentLogging": len(events) > 0 || isPersistentLogger(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleVerifyAuditEvent verifies the signature of a specific audit event.
// This is only functional with enterprise audit logging.
func (h *AuditHandlers) HandleVerifyAuditEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/audit/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[1] != "verify" || parts[0] == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	eventID := parts[0]

	// For OSS, return not_available
	if !isPersistentLogger() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"available": false,
			"message":   "Signature verification requires Pulse Pro with enterprise audit logging",
		})
		return
	}

	logger := audit.GetLogger()
	verifier, ok := logger.(interface {
		VerifySignature(event audit.Event) bool
	})
	if !ok {
		writeErrorResponse(w, http.StatusNotImplemented, "verify_unavailable", "Signature verification is not available", nil)
		return
	}

	events, err := logger.Query(audit.QueryFilter{
		ID:    eventID,
		Limit: 1,
	})
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "query_failed", "Failed to query audit event", nil)
		return
	}
	if len(events) == 0 {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Audit event not found", nil)
		return
	}

	verified := verifier.VerifySignature(events[0])
	message := "Event signature verified"
	if !verified {
		message = "Event signature verification failed"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"available": true,
		"verified":  verified,
		"message":   message,
	})
}

// isPersistentLogger checks if we're using a persistent audit logger (enterprise).
func isPersistentLogger() bool {
	logger := audit.GetLogger()
	_, isConsole := logger.(*audit.ConsoleLogger)
	return !isConsole
}
