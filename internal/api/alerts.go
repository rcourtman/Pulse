package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
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

// GetAlertConfig returns the current alert configuration
func (h *AlertHandlers) GetAlertConfig(w http.ResponseWriter, r *http.Request) {
	config := h.monitor.GetAlertManager().GetConfig()
	
	utils.WriteJSONResponse(w, config)
}

// UpdateAlertConfig updates the alert configuration
func (h *AlertHandlers) UpdateAlertConfig(w http.ResponseWriter, r *http.Request) {
	var config alerts.AlertConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	h.monitor.GetAlertManager().UpdateConfig(config)
	
	// Update notification manager with schedule settings
	if config.Schedule.Cooldown > 0 {
		h.monitor.GetNotificationManager().SetCooldown(config.Schedule.Cooldown)
	}
	if config.Schedule.GroupingWindow > 0 {
		h.monitor.GetNotificationManager().SetGroupingWindow(config.Schedule.GroupingWindow)
	} else if config.Schedule.Grouping.Window > 0 {
		h.monitor.GetNotificationManager().SetGroupingWindow(config.Schedule.Grouping.Window)
	}
	h.monitor.GetNotificationManager().SetGroupingOptions(
		config.Schedule.Grouping.ByNode,
		config.Schedule.Grouping.ByGuest,
	)
	
	// Save to persistent storage
	if err := h.monitor.GetConfigPersistence().SaveAlertConfig(config); err != nil {
		// Log error but don't fail the request
		log.Error().Err(err).Msg("Failed to save alert configuration")
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// GetActiveAlerts returns all active alerts
func (h *AlertHandlers) GetActiveAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := h.monitor.GetAlertManager().GetActiveAlerts()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

// GetAlertHistory returns alert history
func (h *AlertHandlers) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	
	// Check if mock mode is enabled
	mockEnabled := mock.IsMockEnabled()
	log.Info().Bool("mockEnabled", mockEnabled).Msg("GetAlertHistory: checking mock mode")
	
	if mockEnabled {
		history := mock.GetMockAlertHistory(limit)
		log.Info().Int("mockHistoryCount", len(history)).Msg("Returning mock alert history")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(history)
		return
	}
	
	// Get real alert history
	alertHistory := h.monitor.GetAlertManager().GetAlertHistory(limit)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alertHistory)
}

// ClearAlertHistory clears all alert history
func (h *AlertHandlers) ClearAlertHistory(w http.ResponseWriter, r *http.Request) {
	if err := h.monitor.GetAlertManager().ClearAlertHistory(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Alert history cleared"})
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
	
	// Extract alert ID by removing the suffix
	alertID := strings.TrimSuffix(path, suffix)
	if alertID == "" {
		log.Error().
			Str("path", r.URL.Path).
			Msg("Empty alert ID")
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	
	// Log the acknowledge attempt
	log.Debug().
		Str("alertID", alertID).
		Str("path", r.URL.Path).
		Msg("Attempting to acknowledge alert")
	
	// In a real implementation, you'd get the user from authentication
	user := "admin"
	
	if err := h.monitor.GetAlertManager().AcknowledgeAlert(alertID, user); err != nil {
		log.Error().
			Err(err).
			Str("alertID", alertID).
			Msg("Failed to acknowledge alert")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	log.Info().
		Str("alertID", alertID).
		Str("user", user).
		Msg("Alert acknowledged successfully")
	
	// Broadcast updated state to all WebSocket clients
	if h.wsHub != nil {
		state := h.monitor.GetState()
		h.wsHub.BroadcastState(state)
		log.Debug().Msg("Broadcasted state after alert acknowledgment")
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
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
	
	// Extract alert ID by removing the suffix
	alertID := strings.TrimSuffix(path, suffix)
	if alertID == "" {
		log.Error().
			Str("path", r.URL.Path).
			Msg("Empty alert ID")
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	
	h.monitor.GetAlertManager().ClearAlert(alertID)
	
	// Broadcast updated state to all WebSocket clients
	if h.wsHub != nil {
		state := h.monitor.GetState()
		h.wsHub.BroadcastState(state)
		log.Debug().Msg("Broadcasted state after alert clear")
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// BulkAcknowledgeAlerts acknowledges multiple alerts at once
func (h *AlertHandlers) BulkAcknowledgeAlerts(w http.ResponseWriter, r *http.Request) {
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

	var results []map[string]interface{}
	for _, alertID := range request.AlertIDs {
		result := map[string]interface{}{
			"alertId": alertID,
			"success": true,
		}
		if err := h.monitor.GetAlertManager().AcknowledgeAlert(alertID, user); err != nil {
			result["success"] = false
			result["error"] = err.Error()
		}
		results = append(results, result)
	}

	// Broadcast updated state to all WebSocket clients if any alerts were acknowledged
	if h.wsHub != nil {
		hasSuccess := false
		for _, result := range results {
			if success, ok := result["success"].(bool); ok && success {
				hasSuccess = true
				break
			}
		}
		if hasSuccess {
			state := h.monitor.GetState()
			h.wsHub.BroadcastState(state)
			log.Debug().Msg("Broadcasted state after bulk alert acknowledgment")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

// BulkClearAlerts clears multiple alerts at once
func (h *AlertHandlers) BulkClearAlerts(w http.ResponseWriter, r *http.Request) {
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

	var results []map[string]interface{}
	for _, alertID := range request.AlertIDs {
		result := map[string]interface{}{
			"alertId": alertID,
			"success": true,
		}
		h.monitor.GetAlertManager().ClearAlert(alertID)
		results = append(results, result)
	}

	// Broadcast updated state to all WebSocket clients
	if h.wsHub != nil && len(results) > 0 {
		state := h.monitor.GetState()
		h.wsHub.BroadcastState(state)
		log.Debug().Msg("Broadcasted state after bulk alert clear")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

// HandleAlerts routes alert requests to appropriate handlers
func (h *AlertHandlers) HandleAlerts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/alerts")
	
	log.Debug().
		Str("originalPath", r.URL.Path).
		Str("trimmedPath", path).
		Str("method", r.Method).
		Msg("HandleAlerts routing request")
	
	switch {
	case path == "/config" && r.Method == http.MethodGet:
		h.GetAlertConfig(w, r)
	case path == "/config" && r.Method == http.MethodPut:
		h.UpdateAlertConfig(w, r)
	case path == "/active" && r.Method == http.MethodGet:
		h.GetActiveAlerts(w, r)
	case path == "/history" && r.Method == http.MethodGet:
		h.GetAlertHistory(w, r)
	case path == "/history" && r.Method == http.MethodDelete:
		h.ClearAlertHistory(w, r)
	case path == "/bulk/acknowledge" && r.Method == http.MethodPost:
		h.BulkAcknowledgeAlerts(w, r)
	case path == "/bulk/clear" && r.Method == http.MethodPost:
		h.BulkClearAlerts(w, r)
	case strings.HasSuffix(path, "/acknowledge") && r.Method == http.MethodPost:
		h.AcknowledgeAlert(w, r)
	case strings.HasSuffix(path, "/clear") && r.Method == http.MethodPost:
		h.ClearAlert(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}