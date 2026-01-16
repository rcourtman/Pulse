package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// AlertHandlers handles alert-related HTTP endpoints
type AlertHandlers struct {
	monitor *monitoring.Monitor
	wsHub   *websocket.Hub
}

// NewAlertHandlers creates new alert handlers
func NewAlertHandlers(monitor *monitoring.Monitor, wsHub *websocket.Hub) *AlertHandlers {
	return &AlertHandlers{
		monitor: monitor,
		wsHub:   wsHub,
	}
}

// SetMonitor updates the monitor reference for alert handlers.
func (h *AlertHandlers) SetMonitor(m *monitoring.Monitor) {
	h.monitor = m
}

// validateAlertID validates an alert ID for security.
// Alert IDs may contain user-supplied data (e.g., Docker hostnames), so we allow
// printable ASCII characters while blocking control characters and path traversal.
func validateAlertID(alertID string) bool {
	// Guard against empty strings or extremely large payloads that could impact memory usage.
	if len(alertID) == 0 || len(alertID) > 500 {
		return false
	}

	// Reject attempts to traverse directories via crafted path segments.
	if strings.Contains(alertID, "../") || strings.Contains(alertID, "/..") {
		return false
	}

	// Allow printable ASCII characters (32-126) to support user-supplied identifiers
	// like Docker hostnames that may contain parentheses, brackets, etc.
	// Reject control characters (0-31) and DEL (127) which could cause logging issues.
	for _, r := range alertID {
		if r < 32 || r > 126 {
			return false
		}
	}

	// Reject IDs that start or end with spaces (likely malformed)
	if alertID[0] == ' ' || alertID[len(alertID)-1] == ' ' {
		return false
	}

	return true
}

// GetAlertConfig returns the current alert configuration
func (h *AlertHandlers) GetAlertConfig(w http.ResponseWriter, r *http.Request) {
	config := h.monitor.GetAlertManager().GetConfig()

	if err := utils.WriteJSONResponse(w, config); err != nil {
		log.Error().Err(err).Msg("Failed to write alert config response")
	}
}

// UpdateAlertConfig updates the alert configuration
func (h *AlertHandlers) UpdateAlertConfig(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 64KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var config alerts.AlertConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Migrate deprecated GroupingWindow to Grouping.Window if needed before applying.
	groupWindow := config.Schedule.Grouping.Window
	if groupWindow == 0 && config.Schedule.GroupingWindow != 0 {
		groupWindow = config.Schedule.GroupingWindow
		config.Schedule.Grouping.Window = groupWindow
		config.Schedule.GroupingWindow = 0 // Clear deprecated field
		log.Info().
			Int("window", groupWindow).
			Msg("Migrated deprecated GroupingWindow to Grouping.Window")
	}

	h.monitor.GetAlertManager().UpdateConfig(config)
	updatedConfig := h.monitor.GetAlertManager().GetConfig()

	// Update notification manager with schedule settings
	h.monitor.GetNotificationManager().SetCooldown(updatedConfig.Schedule.Cooldown)

	groupWindow = updatedConfig.Schedule.Grouping.Window
	if groupWindow == 0 && updatedConfig.Schedule.GroupingWindow != 0 {
		groupWindow = updatedConfig.Schedule.GroupingWindow
	}
	h.monitor.GetNotificationManager().SetGroupingWindow(groupWindow)
	h.monitor.GetNotificationManager().SetGroupingOptions(
		updatedConfig.Schedule.Grouping.ByNode,
		updatedConfig.Schedule.Grouping.ByGuest,
	)
	h.monitor.GetNotificationManager().SetNotifyOnResolve(updatedConfig.Schedule.NotifyOnResolve)

	// Save to persistent storage
	if err := h.monitor.GetConfigPersistence().SaveAlertConfig(updatedConfig); err != nil {
		// Log error but don't fail the request
		log.Error().Err(err).Msg("Failed to save alert configuration")
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Alert configuration updated successfully",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write alert config update response")
	}
}

// ActivateAlerts activates alert notifications
func (h *AlertHandlers) ActivateAlerts(w http.ResponseWriter, r *http.Request) {
	// Get current config
	config := h.monitor.GetAlertManager().GetConfig()

	// Check if already active
	if config.ActivationState == alerts.ActivationActive {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"success":        true,
			"message":        "Alerts already activated",
			"state":          string(config.ActivationState),
			"activationTime": config.ActivationTime,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write activate response")
		}
		return
	}

	// Activate notifications
	now := time.Now()
	config.ActivationState = alerts.ActivationActive
	config.ActivationTime = &now

	// Update config
	h.monitor.GetAlertManager().UpdateConfig(config)

	// Save to persistent storage
	if err := h.monitor.GetConfigPersistence().SaveAlertConfig(config); err != nil {
		log.Error().Err(err).Msg("Failed to save alert configuration after activation")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Notify about existing critical alerts after activation
	activeAlerts := h.monitor.GetAlertManager().GetActiveAlerts()
	criticalCount := 0
	for _, alert := range activeAlerts {
		if alert.Level == alerts.AlertLevelCritical && !alert.Acknowledged {
			// Re-dispatch critical alerts to trigger notifications
			h.monitor.GetAlertManager().NotifyExistingAlert(alert.ID)
			criticalCount++
		}
	}

	if criticalCount > 0 {
		log.Info().
			Int("criticalAlerts", criticalCount).
			Msg("Sent notifications for existing critical alerts after activation")
	}

	log.Info().Msg("Alert notifications activated")

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success":        true,
		"message":        "Alert notifications activated",
		"state":          string(config.ActivationState),
		"activationTime": config.ActivationTime,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write activate response")
	}
}

// GetActiveAlerts returns all active alerts
func (h *AlertHandlers) GetActiveAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := h.monitor.GetAlertManager().GetActiveAlerts()

	if err := utils.WriteJSONResponse(w, alerts); err != nil {
		log.Error().Err(err).Msg("Failed to write active alerts response")
	}
}

// GetAlertHistory returns alert history
func (h *AlertHandlers) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit := 100
	if limitStr := query.Get("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		switch {
		case err != nil:
			log.Warn().Str("limit", limitStr).Msg("Invalid limit parameter, using default")
		case l < 0:
			http.Error(w, "limit must be non-negative", http.StatusBadRequest)
			return
		case l == 0:
			limit = 0
		case l > 10000:
			log.Warn().Int("limit", l).Msg("Limit exceeds maximum, capping at 10000")
			limit = 10000
		default:
			limit = l
		}
	}

	offset := 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			if o < 0 {
				http.Error(w, "offset must be non-negative", http.StatusBadRequest)
				return
			}
			offset = o
		} else {
			log.Warn().Str("offset", offsetStr).Msg("Invalid offset parameter, ignoring")
		}
	}

	var startTime *time.Time
	if startStr := query.Get("startTime"); startStr != "" {
		parsed, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			http.Error(w, "invalid startTime parameter", http.StatusBadRequest)
			return
		}
		startTime = &parsed
	}

	var endTime *time.Time
	if endStr := query.Get("endTime"); endStr != "" {
		parsed, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			http.Error(w, "invalid endTime parameter", http.StatusBadRequest)
			return
		}
		endTime = &parsed
	}

	if startTime != nil && endTime != nil && endTime.Before(*startTime) {
		http.Error(w, "endTime must be after startTime", http.StatusBadRequest)
		return
	}

	severity := strings.ToLower(strings.TrimSpace(query.Get("severity")))
	switch severity {
	case "", "all":
		severity = ""
	case "warning", "critical":
	default:
		log.Warn().Str("severity", severity).Msg("Invalid severity filter, ignoring")
		severity = ""
	}

	resourceID := strings.TrimSpace(query.Get("resourceId"))

	matchesFilters := func(alertTime time.Time, alertLevel string, alertResourceID string) bool {
		if startTime != nil && alertTime.Before(*startTime) {
			return false
		}
		if endTime != nil && alertTime.After(*endTime) {
			return false
		}
		if severity != "" && !strings.EqualFold(alertLevel, severity) {
			return false
		}
		if resourceID != "" && alertResourceID != resourceID {
			return false
		}
		return true
	}

	trimAlerts := func(alerts []alerts.Alert) []alerts.Alert {
		if offset > 0 {
			if offset >= len(alerts) {
				return alerts[:0]
			}
			alerts = alerts[offset:]
		}
		if limit > 0 && len(alerts) > limit {
			alerts = alerts[:limit]
		}
		return alerts
	}

	trimMockAlerts := func(alerts []models.Alert) []models.Alert {
		if offset > 0 {
			if offset >= len(alerts) {
				return alerts[:0]
			}
			alerts = alerts[offset:]
		}
		if limit > 0 && len(alerts) > limit {
			alerts = alerts[:limit]
		}
		return alerts
	}

	// Check if mock mode is enabled
	mockEnabled := mock.IsMockEnabled()
	log.Debug().Bool("mockEnabled", mockEnabled).Msg("GetAlertHistory: mock mode status")

	fetchLimit := limit
	if fetchLimit > 0 && offset > 0 {
		fetchLimit += offset
	}

	if mockEnabled {
		mockHistory := mock.GetMockAlertHistory(fetchLimit)
		filtered := make([]models.Alert, 0, len(mockHistory))
		for _, alert := range mockHistory {
			if matchesFilters(alert.StartTime, alert.Level, alert.ResourceID) {
				filtered = append(filtered, alert)
			}
		}
		filtered = trimMockAlerts(filtered)
		if err := utils.WriteJSONResponse(w, filtered); err != nil {
			log.Error().Err(err).Msg("Failed to write mock alert history response")
		}
		return
	}

	if h.monitor == nil {
		http.Error(w, "monitor is not initialized", http.StatusServiceUnavailable)
		return
	}

	manager := h.monitor.GetAlertManager()
	var history []alerts.Alert
	if startTime != nil {
		history = manager.GetAlertHistorySince(*startTime, fetchLimit)
	} else {
		history = manager.GetAlertHistory(fetchLimit)
	}

	filtered := make([]alerts.Alert, 0, len(history))
	for _, alert := range history {
		if matchesFilters(alert.StartTime, string(alert.Level), alert.ResourceID) {
			filtered = append(filtered, alert)
		}
	}
	filtered = trimAlerts(filtered)

	if err := utils.WriteJSONResponse(w, filtered); err != nil {
		log.Error().Err(err).Msg("Failed to write alert history response")
	}
}

// GetAlertIncidentTimeline returns the incident timeline for an alert or resource.
func (h *AlertHandlers) GetAlertIncidentTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store := h.monitor.GetIncidentStore()
	if store == nil {
		http.Error(w, "Incident store unavailable", http.StatusServiceUnavailable)
		return
	}

	query := r.URL.Query()
	alertID := strings.TrimSpace(query.Get("alert_id"))
	resourceID := strings.TrimSpace(query.Get("resource_id"))
	startedAtRaw := strings.TrimSpace(query.Get("started_at"))
	if startedAtRaw == "" {
		startedAtRaw = strings.TrimSpace(query.Get("start_time"))
	}
	limit := 0
	if rawLimit := strings.TrimSpace(query.Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if alertID != "" {
		if !validateAlertID(alertID) {
			http.Error(w, "Invalid alert ID", http.StatusBadRequest)
			return
		}
		var startedAt time.Time
		if startedAtRaw != "" {
			parsed, err := time.Parse(time.RFC3339, startedAtRaw)
			if err != nil {
				http.Error(w, "Invalid started_at time", http.StatusBadRequest)
				return
			}
			startedAt = parsed
		}

		var incident *memory.Incident
		if !startedAt.IsZero() {
			incident = store.GetTimelineByAlertAt(alertID, startedAt)
		} else {
			incident = store.GetTimelineByAlertID(alertID)
		}
		if err := utils.WriteJSONResponse(w, incident); err != nil {
			log.Error().Err(err).Msg("Failed to write incident timeline response")
		}
		return
	}

	if resourceID != "" {
		if len(resourceID) > 500 {
			http.Error(w, "Invalid resource ID", http.StatusBadRequest)
			return
		}
		incidents := store.ListIncidentsByResource(resourceID, limit)
		if err := utils.WriteJSONResponse(w, incidents); err != nil {
			log.Error().Err(err).Msg("Failed to write incident list response")
		}
		return
	}

	http.Error(w, "Missing alert_id or resource_id", http.StatusBadRequest)
}

// SaveAlertIncidentNote stores a user note in the incident timeline.
func (h *AlertHandlers) SaveAlertIncidentNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store := h.monitor.GetIncidentStore()
	if store == nil {
		http.Error(w, "Incident store unavailable", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	var req struct {
		AlertID    string `json:"alert_id"`
		IncidentID string `json:"incident_id"`
		Note       string `json:"note"`
		User       string `json:"user,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.AlertID = strings.TrimSpace(req.AlertID)
	req.IncidentID = strings.TrimSpace(req.IncidentID)
	req.Note = strings.TrimSpace(req.Note)
	req.User = strings.TrimSpace(req.User)

	if req.AlertID == "" && req.IncidentID == "" {
		http.Error(w, "alert_id or incident_id is required", http.StatusBadRequest)
		return
	}
	if req.AlertID != "" && !validateAlertID(req.AlertID) {
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}
	if req.Note == "" {
		http.Error(w, "note is required", http.StatusBadRequest)
		return
	}

	if ok := store.RecordNote(req.AlertID, req.IncidentID, req.Note, req.User); !ok {
		http.Error(w, "Failed to save note", http.StatusBadRequest)
		return
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write incident note response")
	}
}

// ClearAlertHistory clears all alert history
func (h *AlertHandlers) ClearAlertHistory(w http.ResponseWriter, r *http.Request) {
	if err := h.monitor.GetAlertManager().ClearAlertHistory(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := utils.WriteJSONResponse(w, map[string]string{"status": "success", "message": "Alert history cleared"}); err != nil {
		log.Error().Err(err).Msg("Failed to write alert history clear confirmation")
	}
}

// UnacknowledgeAlert removes acknowledged status from an alert
func (h *AlertHandlers) UnacknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	// Extract alert ID from URL path: /api/alerts/{id}/unacknowledge
	path := strings.TrimPrefix(r.URL.Path, "/api/alerts/")

	const suffix = "/unacknowledge"
	if !strings.HasSuffix(path, suffix) {
		log.Error().
			Str("path", r.URL.Path).
			Msg("Path does not end with /unacknowledge")
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// Extract alert ID by removing the suffix and decoding encoded characters
	encodedID := strings.TrimSuffix(path, suffix)
	alertID, err := url.PathUnescape(encodedID)
	if err != nil {
		log.Error().Err(err).Str("encodedID", encodedID).Msg("Failed to decode alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if !validateAlertID(alertID) {
		log.Error().
			Str("path", r.URL.Path).
			Str("alertID", alertID).
			Msg("Invalid alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	// Log the unacknowledge attempt
	log.Debug().
		Str("alertID", alertID).
		Str("path", r.URL.Path).
		Msg("Attempting to unacknowledge alert")

	if err := h.monitor.GetAlertManager().UnacknowledgeAlert(alertID); err != nil {
		log.Error().
			Err(err).
			Str("alertID", alertID).
			Msg("Failed to unacknowledge alert")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.monitor.SyncAlertState()

	log.Info().
		Str("alertID", alertID).
		Msg("Alert unacknowledged successfully")

	// Send response immediately
	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to write unacknowledge response")
	}

	// Broadcast updated state to all WebSocket clients after response
	// Do this in a goroutine to avoid blocking the HTTP response
	if h.wsHub != nil {
		go func() {
			state := h.monitor.GetState()
			h.wsHub.BroadcastState(state.ToFrontend())
			log.Debug().Msg("Broadcasted state after alert unacknowledgment")
		}()
	}
}

// AcknowledgeAlert acknowledges an alert
func (h *AlertHandlers) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	// Extract alert ID from URL path: /api/alerts/{id}/acknowledge
	// Alert IDs can contain slashes (e.g., "pve1:qemu/101-cpu")
	// So we need to find the /acknowledge suffix and extract everything before it
	path := strings.TrimPrefix(r.URL.Path, "/api/alerts/")

	const suffix = "/acknowledge"
	if !strings.HasSuffix(path, suffix) {
		log.Error().
			Str("path", r.URL.Path).
			Msg("Path does not end with /acknowledge")
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// Extract alert ID by removing the suffix and decoding encoded characters
	encodedID := strings.TrimSuffix(path, suffix)
	alertID, err := url.PathUnescape(encodedID)
	if err != nil {
		log.Error().Err(err).Str("encodedID", encodedID).Msg("Failed to decode alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if !validateAlertID(alertID) {
		log.Error().
			Str("path", r.URL.Path).
			Str("alertID", alertID).
			Msg("Invalid alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	// Log the acknowledge attempt
	log.Debug().
		Str("alertID", alertID).
		Str("path", r.URL.Path).
		Msg("Attempting to acknowledge alert")

	// In a real implementation, you'd get the user from authentication
	user := "admin"

	log.Debug().
		Str("alertID", alertID).
		Msg("About to call AcknowledgeAlert on manager")

	if err := h.monitor.GetAlertManager().AcknowledgeAlert(alertID, user); err != nil {
		log.Error().
			Err(err).
			Str("alertID", alertID).
			Msg("Failed to acknowledge alert")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.monitor.SyncAlertState()

	log.Info().
		Str("alertID", alertID).
		Str("user", user).
		Msg("Alert acknowledged successfully")

	// Send response immediately
	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to write acknowledge response")
	}

	// Broadcast updated state to all WebSocket clients after response
	// Do this in a goroutine to avoid blocking the HTTP response
	if h.wsHub != nil {
		go func() {
			state := h.monitor.GetState()
			h.wsHub.BroadcastState(state.ToFrontend())
			log.Debug().Msg("Broadcasted state after alert acknowledgment")
		}()
	}
}

// alertIDRequest is used for endpoints that accept alert ID in request body
// This avoids URL encoding issues with alert IDs containing special characters (/, :)
type alertIDRequest struct {
	ID string `json:"id"`
}

// AcknowledgeAlertByBody acknowledges an alert using ID from request body
// POST /api/alerts/acknowledge with {"id": "alert-id"}
// This is the preferred method as it avoids URL encoding issues with reverse proxies
func (h *AlertHandlers) AcknowledgeAlertByBody(w http.ResponseWriter, r *http.Request) {
	var req alertIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode acknowledge request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	alertID := strings.TrimSpace(req.ID)
	if alertID == "" {
		http.Error(w, "Alert ID is required", http.StatusBadRequest)
		return
	}

	if !validateAlertID(alertID) {
		log.Error().Str("alertID", alertID).Msg("Invalid alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	user := "admin"

	if err := h.monitor.GetAlertManager().AcknowledgeAlert(alertID, user); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to acknowledge alert")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.monitor.SyncAlertState()

	log.Info().Str("alertID", alertID).Str("user", user).Msg("Alert acknowledged successfully")

	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to write acknowledge response")
	}

	if h.wsHub != nil {
		go func() {
			state := h.monitor.GetState()
			h.wsHub.BroadcastState(state.ToFrontend())
		}()
	}
}

// UnacknowledgeAlertByBody unacknowledges an alert using ID from request body
// POST /api/alerts/unacknowledge with {"id": "alert-id"}
func (h *AlertHandlers) UnacknowledgeAlertByBody(w http.ResponseWriter, r *http.Request) {
	var req alertIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode unacknowledge request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	alertID := strings.TrimSpace(req.ID)
	if alertID == "" {
		http.Error(w, "Alert ID is required", http.StatusBadRequest)
		return
	}

	if !validateAlertID(alertID) {
		log.Error().Str("alertID", alertID).Msg("Invalid alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if err := h.monitor.GetAlertManager().UnacknowledgeAlert(alertID); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to unacknowledge alert")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.monitor.SyncAlertState()

	log.Info().Str("alertID", alertID).Msg("Alert unacknowledged successfully")

	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to write unacknowledge response")
	}

	if h.wsHub != nil {
		go func() {
			state := h.monitor.GetState()
			h.wsHub.BroadcastState(state.ToFrontend())
		}()
	}
}

// ClearAlertByBody clears an alert using ID from request body
// POST /api/alerts/clear with {"id": "alert-id"}
func (h *AlertHandlers) ClearAlertByBody(w http.ResponseWriter, r *http.Request) {
	var req alertIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode clear request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	alertID := strings.TrimSpace(req.ID)
	if alertID == "" {
		http.Error(w, "Alert ID is required", http.StatusBadRequest)
		return
	}

	if !validateAlertID(alertID) {
		log.Error().Str("alertID", alertID).Msg("Invalid alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if !h.monitor.GetAlertManager().ClearAlert(alertID) {
		http.Error(w, "Alert not found", http.StatusNotFound)
		return
	}

	h.monitor.SyncAlertState()

	log.Info().Str("alertID", alertID).Msg("Alert cleared successfully")

	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to write clear response")
	}

	if h.wsHub != nil {
		go func() {
			state := h.monitor.GetState()
			h.wsHub.BroadcastState(state.ToFrontend())
		}()
	}
}

// ClearAlert manually clears an alert
func (h *AlertHandlers) ClearAlert(w http.ResponseWriter, r *http.Request) {
	// Extract alert ID from URL path: /api/alerts/{id}/clear
	// Alert IDs can contain slashes (e.g., "pve1:qemu/101-cpu")
	// So we need to find the /clear suffix and extract everything before it
	path := strings.TrimPrefix(r.URL.Path, "/api/alerts/")

	const suffix = "/clear"
	if !strings.HasSuffix(path, suffix) {
		log.Error().
			Str("path", r.URL.Path).
			Msg("Path does not end with /clear")
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// Extract alert ID by removing the suffix and decoding encoded characters
	encodedID := strings.TrimSuffix(path, suffix)
	alertID, err := url.PathUnescape(encodedID)
	if err != nil {
		log.Error().Err(err).Str("encodedID", encodedID).Msg("Failed to decode alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if !validateAlertID(alertID) {
		log.Error().
			Str("path", r.URL.Path).
			Str("alertID", alertID).
			Msg("Invalid alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if !h.monitor.GetAlertManager().ClearAlert(alertID) {
		http.Error(w, "Alert not found", http.StatusNotFound)
		return
	}
	h.monitor.SyncAlertState()

	// Send response immediately
	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to write clear alert response")
	}

	// Broadcast updated state to all WebSocket clients after response
	// Do this in a goroutine to avoid blocking the HTTP response
	if h.wsHub != nil {
		go func() {
			state := h.monitor.GetState()
			h.wsHub.BroadcastState(state.ToFrontend())
			log.Debug().Msg("Broadcasted state after alert clear")
		}()
	}
}

// BulkAcknowledgeAlerts acknowledges multiple alerts at once
func (h *AlertHandlers) BulkAcknowledgeAlerts(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

	var request struct {
		AlertIDs []string `json:"alertIds"`
		User     string   `json:"user,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(request.AlertIDs) == 0 {
		http.Error(w, "No alert IDs provided", http.StatusBadRequest)
		return
	}

	user := request.User
	if user == "" {
		user = "admin"
	}

	var (
		results    []map[string]interface{}
		anySuccess bool
	)
	for _, alertID := range request.AlertIDs {
		result := map[string]interface{}{
			"alertId": alertID,
			"success": true,
		}
		if err := h.monitor.GetAlertManager().AcknowledgeAlert(alertID, user); err != nil {
			result["success"] = false
			result["error"] = err.Error()
		} else {
			anySuccess = true
		}
		results = append(results, result)
	}

	if anySuccess {
		h.monitor.SyncAlertState()
	}

	// Send response immediately
	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"results": results,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write bulk acknowledge response")
	}

	// Broadcast updated state to all WebSocket clients if any alerts were acknowledged
	// Do this in a goroutine to avoid blocking the HTTP response
	if h.wsHub != nil && anySuccess {
		go func() {
			state := h.monitor.GetState()
			h.wsHub.BroadcastState(state.ToFrontend())
			log.Debug().Msg("Broadcasted state after bulk alert acknowledgment")
		}()
	}
}

// BulkClearAlerts clears multiple alerts at once
func (h *AlertHandlers) BulkClearAlerts(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

	var request struct {
		AlertIDs []string `json:"alertIds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(request.AlertIDs) == 0 {
		http.Error(w, "No alert IDs provided", http.StatusBadRequest)
		return
	}

	var (
		results    []map[string]interface{}
		anySuccess bool
	)
	for _, alertID := range request.AlertIDs {
		result := map[string]interface{}{
			"alertId": alertID,
			"success": true,
		}
		if h.monitor.GetAlertManager().ClearAlert(alertID) {
			anySuccess = true
		} else {
			result["success"] = false
			result["error"] = "alert not found"
		}
		results = append(results, result)
	}

	if anySuccess {
		h.monitor.SyncAlertState()
	}

	// Send response immediately
	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"results": results,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write bulk clear response")
	}

	// Broadcast updated state to all WebSocket clients after response
	// Do this in a goroutine to avoid blocking the HTTP response
	if h.wsHub != nil && anySuccess {
		go func() {
			state := h.monitor.GetState()
			h.wsHub.BroadcastState(state.ToFrontend())
			log.Debug().Msg("Broadcasted state after bulk alert clear")
		}()
	}
}

// HandleAlerts routes alert requests to appropriate handlers
func (h *AlertHandlers) HandleAlerts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/alerts/")

	log.Debug().
		Str("originalPath", r.URL.Path).
		Str("trimmedPath", path).
		Str("method", r.Method).
		Msg("HandleAlerts routing request")

	switch {
	case path == "config" && r.Method == http.MethodGet:
		if !ensureScope(w, r, config.ScopeMonitoringRead) {
			return
		}
		h.GetAlertConfig(w, r)
	case path == "config" && r.Method == http.MethodPut:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.UpdateAlertConfig(w, r)
	case path == "activate" && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.ActivateAlerts(w, r)
	case path == "active" && r.Method == http.MethodGet:
		if !ensureScope(w, r, config.ScopeMonitoringRead) {
			return
		}
		h.GetActiveAlerts(w, r)
	case path == "history" && r.Method == http.MethodGet:
		if !ensureScope(w, r, config.ScopeMonitoringRead) {
			return
		}
		h.GetAlertHistory(w, r)
	case path == "incidents" && r.Method == http.MethodGet:
		if !ensureScope(w, r, config.ScopeMonitoringRead) {
			return
		}
		h.GetAlertIncidentTimeline(w, r)
	case path == "incidents/note" && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.SaveAlertIncidentNote(w, r)
	case path == "history" && r.Method == http.MethodDelete:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.ClearAlertHistory(w, r)
	case path == "bulk/acknowledge" && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.BulkAcknowledgeAlerts(w, r)
	case path == "bulk/clear" && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.BulkClearAlerts(w, r)
	// Body-based endpoints (preferred - avoids URL encoding issues with reverse proxies)
	case path == "acknowledge" && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.AcknowledgeAlertByBody(w, r)
	case path == "unacknowledge" && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.UnacknowledgeAlertByBody(w, r)
	case path == "clear" && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.ClearAlertByBody(w, r)
	// Legacy path-based endpoints (kept for backwards compatibility)
	case strings.HasSuffix(path, "/acknowledge") && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.AcknowledgeAlert(w, r)
	case strings.HasSuffix(path, "/unacknowledge") && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.UnacknowledgeAlert(w, r)
	case strings.HasSuffix(path, "/clear") && r.Method == http.MethodPost:
		if !ensureScope(w, r, config.ScopeMonitoringWrite) {
			return
		}
		h.ClearAlert(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
