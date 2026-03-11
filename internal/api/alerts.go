package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// AlertManager defines the interface for alert management operations.
type AlertManager interface {
	GetConfig() alerts.AlertConfig
	UpdateConfig(alerts.AlertConfig)
	GetActiveAlerts() []alerts.Alert
	NotifyExistingAlert(id string)
	ClearAlertHistory() error
	UnacknowledgeAlert(id string) error
	AcknowledgeAlert(id, user string) error
	ClearAlert(id string) bool
	GetAlertHistory(limit int) []alerts.Alert
	GetAlertHistorySince(since time.Time, limit int) []alerts.Alert
}

// ConfigPersistence defines the interface for saving configuration.
type ConfigPersistence interface {
	SaveAlertConfig(alerts.AlertConfig) error
}

// AlertMonitor defines the interface for monitoring operations used by alert handlers.
type AlertMonitor interface {
	GetAlertManager() AlertManager
	GetConfigPersistence() ConfigPersistence
	GetIncidentStore() *memory.IncidentStore
	GetNotificationManager() *notifications.NotificationManager
	SyncAlertState()
	BuildFrontendState() models.StateFrontend
}

// AlertHandlers handles alert-related HTTP endpoints
type AlertHandlers struct {
	stateMu        sync.RWMutex
	mtMonitor      *monitoring.MultiTenantMonitor
	defaultMonitor AlertMonitor
	wsHub          *websocket.Hub
}

// NewAlertHandlers creates new alert handlers
func NewAlertHandlers(mtm *monitoring.MultiTenantMonitor, monitor AlertMonitor, wsHub *websocket.Hub) *AlertHandlers {
	// If mtm is provided, try to populate defaultMonitor from "default" org if not provided.
	if monitor == nil && mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			monitor = NewAlertMonitorWrapper(m)
		}
	}
	return &AlertHandlers{
		mtMonitor:      mtm,
		defaultMonitor: monitor,
		wsHub:          wsHub,
	}
}

// SetMultiTenantMonitor updates the multi-tenant monitor reference
func (h *AlertHandlers) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	var defaultMonitor AlertMonitor
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			defaultMonitor = NewAlertMonitorWrapper(m)
		}
	}

	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.mtMonitor = mtm
	if defaultMonitor != nil {
		h.defaultMonitor = defaultMonitor
	}
}

// SetMonitor updates the monitor reference for alert handlers.
func (h *AlertHandlers) SetMonitor(m AlertMonitor) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.defaultMonitor = m
}

func (h *AlertHandlers) getMonitor(ctx context.Context) AlertMonitor {
	h.stateMu.RLock()
	mtMonitor := h.mtMonitor
	defaultMonitor := h.defaultMonitor
	h.stateMu.RUnlock()

	orgID := GetOrgID(ctx)
	if mtMonitor != nil {
		if m, err := mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return NewAlertMonitorWrapper(m)
		}
	}
	return defaultMonitor
}

func (h *AlertHandlers) broadcastStateForContext(ctx context.Context) {
	if h.wsHub == nil {
		return
	}

	orgID := GetOrgID(ctx)
	frontendState := h.getMonitor(ctx).BuildFrontendState()
	if orgID != "" {
		h.wsHub.BroadcastStateToTenant(orgID, frontendState)
		return
	}
	h.wsHub.BroadcastState(frontendState)
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
	config := h.getMonitor(r.Context()).GetAlertManager().GetConfig()

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

	h.getMonitor(r.Context()).GetAlertManager().UpdateConfig(config)
	updatedConfig := h.getMonitor(r.Context()).GetAlertManager().GetConfig()

	// Update notification manager with schedule settings
	notificationMgr := h.getMonitor(r.Context()).GetNotificationManager()
	notificationMgr.SetEnabled(updatedConfig.Enabled && updatedConfig.ActivationState == alerts.ActivationActive)
	notificationMgr.SetCooldown(updatedConfig.Schedule.Cooldown)
	notificationMgr.SetGroupingWindow(updatedConfig.Schedule.Grouping.Window)
	notificationMgr.SetGroupingOptions(
		updatedConfig.Schedule.Grouping.ByNode,
		updatedConfig.Schedule.Grouping.ByGuest,
	)
	notificationMgr.SetNotifyOnResolve(updatedConfig.Schedule.NotifyOnResolve)

	// Save to persistent storage
	if err := h.getMonitor(r.Context()).GetConfigPersistence().SaveAlertConfig(updatedConfig); err != nil {
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
	config := h.getMonitor(r.Context()).GetAlertManager().GetConfig()

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
	h.getMonitor(r.Context()).GetAlertManager().UpdateConfig(config)
	h.getMonitor(r.Context()).GetNotificationManager().SetEnabled(
		config.Enabled && config.ActivationState == alerts.ActivationActive,
	)

	// Save to persistent storage
	if err := h.getMonitor(r.Context()).GetConfigPersistence().SaveAlertConfig(config); err != nil {
		log.Error().Err(err).Msg("Failed to save alert configuration after activation")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Notify about existing critical alerts after activation
	activeAlerts := h.getMonitor(r.Context()).GetAlertManager().GetActiveAlerts()
	criticalCount := 0
	for _, alert := range activeAlerts {
		if alert.Level == alerts.AlertLevelCritical && !alert.Acknowledged {
			// Re-dispatch critical alerts to trigger notifications
			h.getMonitor(r.Context()).GetAlertManager().NotifyExistingAlert(alert.ID)
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
	alerts := h.getMonitor(r.Context()).GetAlertManager().GetActiveAlerts()

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

	if h.getMonitor(r.Context()) == nil {
		http.Error(w, "monitor is not initialized", http.StatusServiceUnavailable)
		return
	}

	manager := h.getMonitor(r.Context()).GetAlertManager()
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

	store := h.getMonitor(r.Context()).GetIncidentStore()
	if store == nil {
		http.Error(w, "Incident store unavailable", http.StatusServiceUnavailable)
		return
	}

	query := r.URL.Query()
	alertID := strings.TrimSpace(query.Get("alert_identifier"))
	if alertID == "" {
		alertID = strings.TrimSpace(query.Get("alert_id"))
	}
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
			http.Error(w, "Invalid alert identifier", http.StatusBadRequest)
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
		if err := utils.WriteJSONResponse(w, exportIncident(incident)); err != nil {
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
		if err := utils.WriteJSONResponse(w, exportIncidents(incidents)); err != nil {
			log.Error().Err(err).Msg("Failed to write incident list response")
		}
		return
	}

	http.Error(w, "Missing alert_identifier or resource_id", http.StatusBadRequest)
}

// SaveAlertIncidentNote stores a user note in the incident timeline.
func (h *AlertHandlers) SaveAlertIncidentNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store := h.getMonitor(r.Context()).GetIncidentStore()
	if store == nil {
		http.Error(w, "Incident store unavailable", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	var req struct {
		AlertIdentifier string `json:"alertIdentifier"`
		AlertID         string `json:"alert_id"`
		IncidentID      string `json:"incident_id"`
		Note            string `json:"note"`
		User            string `json:"user,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.AlertIdentifier = strings.TrimSpace(req.AlertIdentifier)
	req.AlertID = strings.TrimSpace(req.AlertID)
	req.IncidentID = strings.TrimSpace(req.IncidentID)
	req.Note = strings.TrimSpace(req.Note)
	req.User = strings.TrimSpace(req.User)

	alertIdentifier := req.AlertIdentifier
	if alertIdentifier == "" {
		alertIdentifier = req.AlertID
	}

	if alertIdentifier == "" && req.IncidentID == "" {
		http.Error(w, "alertIdentifier or incident_id is required", http.StatusBadRequest)
		return
	}
	if alertIdentifier != "" && !validateAlertID(alertIdentifier) {
		http.Error(w, "Invalid alert identifier", http.StatusBadRequest)
		return
	}
	if req.Note == "" {
		http.Error(w, "note is required", http.StatusBadRequest)
		return
	}

	if ok := store.RecordNote(alertIdentifier, req.IncidentID, req.Note, req.User); !ok {
		http.Error(w, "Failed to save note", http.StatusBadRequest)
		return
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write incident note response")
	}
}

type incidentEventView struct {
	ID        string                   `json:"id"`
	Type      memory.IncidentEventType `json:"type"`
	Timestamp time.Time                `json:"timestamp"`
	Summary   string                   `json:"summary"`
	Details   map[string]interface{}   `json:"details,omitempty"`
}

type incidentView struct {
	ID              string                `json:"id"`
	AlertIdentifier string                `json:"alertIdentifier"`
	LegacyAlertID   string                `json:"legacyAlertId,omitempty"`
	AlertID         string                `json:"alertId"`
	AlertType       string                `json:"alertType"`
	Level           string                `json:"level"`
	ResourceID      string                `json:"resourceId"`
	ResourceName    string                `json:"resourceName"`
	ResourceType    string                `json:"resourceType,omitempty"`
	Node            string                `json:"node,omitempty"`
	Instance        string                `json:"instance,omitempty"`
	Message         string                `json:"message,omitempty"`
	Status          memory.IncidentStatus `json:"status"`
	OpenedAt        time.Time             `json:"openedAt"`
	ClosedAt        *time.Time            `json:"closedAt,omitempty"`
	Acknowledged    bool                  `json:"acknowledged"`
	AckUser         string                `json:"ackUser,omitempty"`
	AckTime         *time.Time            `json:"ackTime,omitempty"`
	Events          []incidentEventView   `json:"events,omitempty"`
}

func exportIncident(incident *memory.Incident) *incidentView {
	if incident == nil {
		return nil
	}
	events := make([]incidentEventView, 0, len(incident.Events))
	for _, event := range incident.Events {
		events = append(events, incidentEventView{
			ID:        event.ID,
			Type:      event.Type,
			Timestamp: event.Timestamp,
			Summary:   event.Summary,
			Details:   event.Details,
		})
	}
	alertIdentifier := strings.TrimSpace(incident.AlertID)
	return &incidentView{
		ID:              incident.ID,
		AlertIdentifier: alertIdentifier,
		AlertID:         alertIdentifier,
		AlertType:       incident.AlertType,
		Level:           incident.Level,
		ResourceID:      incident.ResourceID,
		ResourceName:    incident.ResourceName,
		ResourceType:    incident.ResourceType,
		Node:            incident.Node,
		Instance:        incident.Instance,
		Message:         incident.Message,
		Status:          incident.Status,
		OpenedAt:        incident.OpenedAt,
		ClosedAt:        incident.ClosedAt,
		Acknowledged:    incident.Acknowledged,
		AckUser:         incident.AckUser,
		AckTime:         incident.AckTime,
		Events:          events,
	}
}

func exportIncidents(incidents []*memory.Incident) []*incidentView {
	if len(incidents) == 0 {
		return []*incidentView{}
	}
	exported := make([]*incidentView, 0, len(incidents))
	for _, incident := range incidents {
		if view := exportIncident(incident); view != nil {
			exported = append(exported, view)
		}
	}
	return exported
}

// ClearAlertHistory clears all alert history
func (h *AlertHandlers) ClearAlertHistory(w http.ResponseWriter, r *http.Request) {
	if err := h.getMonitor(r.Context()).GetAlertManager().ClearAlertHistory(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := utils.WriteJSONResponse(w, map[string]string{"status": "success", "message": "Alert history cleared"}); err != nil {
		log.Error().Err(err).Msg("Failed to write alert history clear confirmation")
	}
}

// alertIdentifierRequest is used for endpoints that accept one alert identifier in the request body.
// Canonical v6 clients should send `alertIdentifier`; `id` remains as a compatibility alias.
type alertIdentifierRequest struct {
	AlertIdentifier string `json:"alertIdentifier"`
	ID              string `json:"id"`
}

func (r alertIdentifierRequest) identifier() string {
	if identifier := strings.TrimSpace(r.AlertIdentifier); identifier != "" {
		return identifier
	}
	return strings.TrimSpace(r.ID)
}

type alertIdentifiersRequest struct {
	AlertIdentifiers []string `json:"alertIdentifiers"`
	AlertIDs         []string `json:"alertIds"`
	User             string   `json:"user,omitempty"`
}

func (r alertIdentifiersRequest) identifiers() []string {
	raw := r.AlertIdentifiers
	if len(raw) == 0 {
		raw = r.AlertIDs
	}
	identifiers := make([]string, 0, len(raw))
	for _, identifier := range raw {
		trimmed := strings.TrimSpace(identifier)
		if trimmed != "" {
			identifiers = append(identifiers, trimmed)
		}
	}
	return identifiers
}

// AcknowledgeAlertByBody acknowledges an alert using ID from request body
// POST /api/alerts/acknowledge with {"id": "alert-id"}
// This is the preferred method as it avoids URL encoding issues with reverse proxies
func (h *AlertHandlers) AcknowledgeAlertByBody(w http.ResponseWriter, r *http.Request) {
	var req alertIdentifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode acknowledge request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	alertID := req.identifier()
	if alertID == "" {
		http.Error(w, "Alert identifier is required", http.StatusBadRequest)
		return
	}

	if !validateAlertID(alertID) {
		log.Error().Str("alertID", alertID).Msg("Invalid alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	// Get the authenticated user from the auth middleware (set in X-Authenticated-User header)
	user := w.Header().Get("X-Authenticated-User")
	if user == "" {
		user = "unknown" // Fallback if auth header not set (shouldn't happen in normal flow)
	}

	if err := h.getMonitor(r.Context()).GetAlertManager().AcknowledgeAlert(alertID, user); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to acknowledge alert")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.getMonitor(r.Context()).SyncAlertState()

	log.Info().Str("alertID", alertID).Str("user", user).Msg("Alert acknowledged successfully")

	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to write acknowledge response")
	}

	if h.wsHub != nil {
		ctx := r.Context()
		go func() {
			h.broadcastStateForContext(ctx)
		}()
	}
}

// UnacknowledgeAlertByBody unacknowledges an alert using ID from request body
// POST /api/alerts/unacknowledge with {"id": "alert-id"}
func (h *AlertHandlers) UnacknowledgeAlertByBody(w http.ResponseWriter, r *http.Request) {
	var req alertIdentifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode unacknowledge request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	alertID := req.identifier()
	if alertID == "" {
		http.Error(w, "Alert identifier is required", http.StatusBadRequest)
		return
	}

	if !validateAlertID(alertID) {
		log.Error().Str("alertID", alertID).Msg("Invalid alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if err := h.getMonitor(r.Context()).GetAlertManager().UnacknowledgeAlert(alertID); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to unacknowledge alert")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.getMonitor(r.Context()).SyncAlertState()

	log.Info().Str("alertID", alertID).Msg("Alert unacknowledged successfully")

	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to write unacknowledge response")
	}

	if h.wsHub != nil {
		ctx := r.Context()
		go func() {
			h.broadcastStateForContext(ctx)
		}()
	}
}

// ClearAlertByBody clears an alert using ID from request body
// POST /api/alerts/clear with {"id": "alert-id"}
func (h *AlertHandlers) ClearAlertByBody(w http.ResponseWriter, r *http.Request) {
	var req alertIdentifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode clear request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	alertID := req.identifier()
	if alertID == "" {
		http.Error(w, "Alert identifier is required", http.StatusBadRequest)
		return
	}

	if !validateAlertID(alertID) {
		log.Error().Str("alertID", alertID).Msg("Invalid alert ID")
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if !h.getMonitor(r.Context()).GetAlertManager().ClearAlert(alertID) {
		http.Error(w, "Alert not found", http.StatusNotFound)
		return
	}

	h.getMonitor(r.Context()).SyncAlertState()

	log.Info().Str("alertID", alertID).Msg("Alert cleared successfully")

	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Str("alertID", alertID).Msg("Failed to write clear response")
	}

	if h.wsHub != nil {
		ctx := r.Context()
		go func() {
			h.broadcastStateForContext(ctx)
		}()
	}
}

// BulkAcknowledgeAlerts acknowledges multiple alerts at once
func (h *AlertHandlers) BulkAcknowledgeAlerts(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

	var request alertIdentifiersRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	alertIdentifiers := request.identifiers()
	if len(alertIdentifiers) == 0 {
		http.Error(w, "No alert identifiers provided", http.StatusBadRequest)
		return
	}

	// Get the authenticated user from the auth middleware - ignore request.User to prevent spoofing
	user := w.Header().Get("X-Authenticated-User")
	if user == "" {
		user = "unknown" // Fallback if auth header not set (shouldn't happen in normal flow)
	}

	var (
		results    []map[string]interface{}
		anySuccess bool
	)
	for _, alertID := range alertIdentifiers {
		result := map[string]interface{}{
			"alertIdentifier": alertID,
			"alertId":         alertID,
			"success":         true,
		}
		if err := h.getMonitor(r.Context()).GetAlertManager().AcknowledgeAlert(alertID, user); err != nil {
			result["success"] = false
			result["error"] = err.Error()
		} else {
			anySuccess = true
		}
		results = append(results, result)
	}

	if anySuccess {
		h.getMonitor(r.Context()).SyncAlertState()
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
		ctx := r.Context()
		go func() {
			h.broadcastStateForContext(ctx)
			log.Debug().Msg("Broadcasted state after bulk alert acknowledgment")
		}()
	}
}

// BulkClearAlerts clears multiple alerts at once
func (h *AlertHandlers) BulkClearAlerts(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

	var request alertIdentifiersRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	alertIdentifiers := request.identifiers()
	if len(alertIdentifiers) == 0 {
		http.Error(w, "No alert identifiers provided", http.StatusBadRequest)
		return
	}

	var (
		results    []map[string]interface{}
		anySuccess bool
	)
	for _, alertID := range alertIdentifiers {
		result := map[string]interface{}{
			"alertIdentifier": alertID,
			"alertId":         alertID,
			"success":         true,
		}
		if h.getMonitor(r.Context()).GetAlertManager().ClearAlert(alertID) {
			anySuccess = true
		} else {
			result["success"] = false
			result["error"] = "alert not found"
		}
		results = append(results, result)
	}

	if anySuccess {
		h.getMonitor(r.Context()).SyncAlertState()
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
		ctx := r.Context()
		go func() {
			h.broadcastStateForContext(ctx)
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
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
