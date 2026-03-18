package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

// validAuditEventID matches alphanumeric IDs with hyphens (UUID format)
var validAuditEventID = regexp.MustCompile(`^[a-zA-Z0-9\-]+$`)

var resolveWebhookIPs = net.DefaultResolver.LookupIPAddr

// AuditHandlers provides HTTP handlers for audit log endpoints.
type AuditHandlers struct {
	resourceStoreProvider func(orgID string) (unifiedresources.ResourceStore, error)
}

// NewAuditHandlers creates a new AuditHandlers instance.
func NewAuditHandlers() *AuditHandlers {
	return &AuditHandlers{}
}

// SetResourceStoreProvider configures the store used for unified-resource audit history.
func (h *AuditHandlers) SetResourceStoreProvider(provider func(orgID string) (unifiedresources.ResourceStore, error)) {
	if h == nil {
		return
	}
	h.resourceStoreProvider = provider
}

func (h *AuditHandlers) getResourceStore(orgID string) (unifiedresources.ResourceStore, error) {
	if h == nil || h.resourceStoreProvider == nil {
		return nil, fmt.Errorf("resource audit store unavailable")
	}
	return h.resourceStoreProvider(orgID)
}

// HandleListAuditEvents handles GET /api/audit
func (h *AuditHandlers) HandleListAuditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	logger := getLoggerForOrg(orgID)

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
		"persistentLogging": len(events) > 0 || isPersistentLogger(logger),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleVerifyAuditEvent handles GET /api/audit/{id}/verify
func (h *AuditHandlers) HandleVerifyAuditEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	logger := getLoggerForOrg(orgID)

	eventID := r.PathValue("id")
	if eventID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Missing event ID", nil)
		return
	}

	// Validate event ID format to prevent injection
	if !validAuditEventID.MatchString(eventID) || len(eventID) > 64 {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_id", "Invalid event ID format", nil)
		return
	}

	// For OSS, return not_available
	if !isPersistentLogger(logger) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"available": false,
			"message":   "Signature verification requires Pulse Pro with enterprise audit logging",
		})
		return
	}

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

// HandleGetWebhooks returns the audit webhook configuration.
func (h *AuditHandlers) HandleGetWebhooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	logger := getLoggerForOrg(orgID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"urls": logger.GetWebhookURLs(),
	})
}

// HandleUpdateWebhooks updates the audit webhook configuration.
func (h *AuditHandlers) HandleUpdateWebhooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body size to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB max

	var req struct {
		URLs []string `json:"urls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	// Validate all webhook URLs
	var validatedURLs []string
	for _, rawURL := range req.URLs {
		if err := validateWebhookURL(rawURL); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_url", fmt.Sprintf("Invalid webhook URL: %s", err.Error()), map[string]string{"url": rawURL})
			return
		}
		validatedURLs = append(validatedURLs, rawURL)
	}

	orgID := GetOrgID(r.Context())
	logger := getLoggerForOrg(orgID)
	if err := logger.UpdateWebhookURLs(validatedURLs); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "update_failed", "Failed to update webhooks", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// validateWebhookURL validates a webhook URL for security.
// Returns an error if the URL is invalid or potentially dangerous (SSRF).
func validateWebhookURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format")
	}

	// Only allow http and https schemes
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}

	// Ensure host is present
	if parsed.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	// Extract hostname (without port)
	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Block localhost and loopback addresses
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return fmt.Errorf("localhost URLs are not allowed")
	}

	// Check if hostname is an IP address
	ip := net.ParseIP(hostname)
	if ip != nil {
		// Block private/internal IP ranges (SSRF protection)
		if isPrivateOrReservedIP(ip) {
			return fmt.Errorf("private or reserved IP addresses are not allowed")
		}
		return nil
	}

	// Block common internal hostnames
	lowerHost := strings.ToLower(hostname)
	blockedPatterns := []string{
		"metadata.google",
		"169.254.169.254", // AWS/GCP metadata
		"metadata.azure",
		"internal",
		".local",
		".localhost",
	}
	for _, pattern := range blockedPatterns {
		if strings.Contains(lowerHost, pattern) {
			return fmt.Errorf("internal hostnames are not allowed")
		}
	}

	resolveCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := resolveWebhookIPs(resolveCtx, hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname")
	}
	if len(addrs) == 0 {
		return fmt.Errorf("hostname did not resolve")
	}
	for _, addr := range addrs {
		if isPrivateOrReservedIP(addr.IP) {
			return fmt.Errorf("hostname resolves to private or reserved IP addresses")
		}
	}

	return nil
}

// isPrivateOrReservedIP checks if an IP is private, loopback, or reserved.
func isPrivateOrReservedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Additional checks for reserved ranges
	// 169.254.0.0/16 - Link-local (also covers cloud metadata endpoints)
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		// 0.0.0.0/8
		if ip4[0] == 0 {
			return true
		}
	}

	return false
}

// isPersistentLogger checks if we're using a persistent audit logger (enterprise).
func isPersistentLogger(logger audit.Logger) bool {
	_, isConsole := logger.(*audit.ConsoleLogger)
	return !isConsole
}

// HandleExportAuditEvents handles GET /api/audit/export
func (h *AuditHandlers) HandleExportAuditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	logger := getLoggerForOrg(orgID)

	// Check if persistent logger is available
	if !isPersistentLogger(logger) {
		writeErrorResponse(w, http.StatusNotImplemented, "export_unavailable",
			"Export requires Pulse Pro with enterprise audit logging", nil)
		return
	}

	query := r.URL.Query()

	// Parse format
	format := audit.ExportFormatJSON
	if query.Get("format") == "csv" {
		format = audit.ExportFormatCSV
	}

	// Parse filter
	filter := audit.QueryFilter{
		EventType: query.Get("event"),
		User:      query.Get("user"),
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

	// Parse verification flag
	includeVerification := query.Get("verify") == "true"

	persistentLogger, ok := logger.(audit.PersistentLogger)
	if !ok {
		writeErrorResponse(w, http.StatusNotImplemented, "export_unavailable",
			"Export requires persistent audit logger", nil)
		return
	}

	// Create exporter and export
	exporter := audit.NewExporter(persistentLogger)
	result, err := exporter.Export(filter, format, includeVerification)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "export_failed",
			"Failed to export audit events", nil)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", result.Filename))
	w.Header().Set("X-Event-Count", strconv.Itoa(result.EventCount))
	w.Write(result.Data)
}

// HandleAuditSummary handles GET /api/audit/summary
func (h *AuditHandlers) HandleAuditSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	logger := getLoggerForOrg(orgID)

	// Check if persistent logger is available
	if !isPersistentLogger(logger) {
		writeErrorResponse(w, http.StatusNotImplemented, "summary_unavailable",
			"Summary requires Pulse Pro with enterprise audit logging", nil)
		return
	}

	query := r.URL.Query()

	// Parse filter
	filter := audit.QueryFilter{
		EventType: query.Get("event"),
		User:      query.Get("user"),
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

	// Parse verification flag
	verifySignatures := query.Get("verify") == "true"

	persistentLogger, ok := logger.(audit.PersistentLogger)
	if !ok {
		writeErrorResponse(w, http.StatusNotImplemented, "summary_unavailable",
			"Summary requires persistent audit logger", nil)
		return
	}

	// Create exporter and generate summary
	exporter := audit.NewExporter(persistentLogger)
	summary, err := exporter.GenerateSummary(filter, verifySignatures)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "summary_failed",
			"Failed to generate audit summary", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// HandleListUnifiedActionAudits handles GET /api/audit/actions.
func (h *AuditHandlers) HandleListUnifiedActionAudits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	store, err := h.getResourceStore(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "audit_unavailable", "Unified resource audit history is not available", nil)
		return
	}

	query := r.URL.Query()
	resourceID := unifiedresources.CanonicalResourceID(query.Get("resourceId"))
	since := parseAuditSince(query)
	limit := parseAuditLimit(query.Get("limit"), 100)

	audits, err := store.GetActionAudits(resourceID, since, limit)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "query_failed", "Failed to query action audits", nil)
		return
	}

	response := map[string]any{
		"audits": audits,
		"count":  len(audits),
	}
	if resourceID != "" {
		response["resourceId"] = resourceID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleListUnifiedActionLifecycleEvents handles GET /api/audit/actions/{id}/events.
func (h *AuditHandlers) HandleListUnifiedActionLifecycleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actionID := strings.TrimSpace(r.PathValue("id"))
	if actionID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Missing action ID", nil)
		return
	}
	if !validAuditEventID.MatchString(actionID) || len(actionID) > 64 {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_id", "Invalid action ID format", nil)
		return
	}

	orgID := GetOrgID(r.Context())
	store, err := h.getResourceStore(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "audit_unavailable", "Unified resource audit history is not available", nil)
		return
	}

	query := r.URL.Query()
	since := parseAuditSince(query)
	limit := parseAuditLimit(query.Get("limit"), 100)

	events, err := store.GetActionLifecycleEvents(actionID, since, limit)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "query_failed", "Failed to query action lifecycle events", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"actionId": actionID,
		"events":   events,
		"count":    len(events),
	})
}

// HandleListUnifiedExportAudits handles GET /api/audit/exports.
func (h *AuditHandlers) HandleListUnifiedExportAudits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID := GetOrgID(r.Context())
	store, err := h.getResourceStore(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "audit_unavailable", "Unified resource audit history is not available", nil)
		return
	}

	query := r.URL.Query()
	since := parseAuditSince(query)
	limit := parseAuditLimit(query.Get("limit"), 100)

	records, err := store.GetExportAudits(since, limit)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "query_failed", "Failed to query export audits", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"audits": records,
		"count":  len(records),
	})
}

func parseAuditSince(query url.Values) time.Time {
	for _, key := range []string{"since", "startTime"} {
		raw := strings.TrimSpace(query.Get(key))
		if raw == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func parseAuditLimit(raw string, defaultLimit int) int {
	limit := defaultLimit
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		if parsed, err := strconv.Atoi(trimmed); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit <= 0 {
		return defaultLimit
	}
	return limit
}

func getLoggerForOrg(orgID string) audit.Logger {
	if mgr := GetTenantAuditManager(); mgr != nil {
		return mgr.GetLogger(orgID)
	}
	return audit.GetLogger()
}
