package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// AlertHandlers handles alert-related HTTP endpoints
type AlertHandlers struct {
	monitor *monitoring.Monitor
}

// NewAlertHandlers creates new alert handlers
func NewAlertHandlers(monitor *monitoring.Monitor) *AlertHandlers {
	return &AlertHandlers{
		monitor: monitor,
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
	
	history := h.monitor.GetAlertManager().GetAlertHistory(limit)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
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
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		log.Error().
			Str("path", r.URL.Path).
			Int("parts", len(parts)).
			Msg("Invalid acknowledge URL format")
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	alertID := parts[3]
	
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
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ClearAlert manually clears an alert
func (h *AlertHandlers) ClearAlert(w http.ResponseWriter, r *http.Request) {
	// Extract alert ID from URL path: /api/alerts/{id}/clear
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	alertID := parts[3]
	
	h.monitor.GetAlertManager().ClearAlert(alertID)
	
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

// HandleAlerts routes alert requests to appropriate handlers
func (h *AlertHandlers) HandleAlerts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/alerts")
	
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