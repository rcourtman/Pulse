package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// NotificationQueueHandlers handles notification queue API endpoints
type NotificationQueueHandlers struct {
	monitor *monitoring.Monitor
}

// NewNotificationQueueHandlers creates new notification queue handlers
func NewNotificationQueueHandlers(monitor *monitoring.Monitor) *NotificationQueueHandlers {
	return &NotificationQueueHandlers{
		monitor: monitor,
	}
}

// GetDLQ returns notifications in the dead letter queue
func (h *NotificationQueueHandlers) GetDLQ(w http.ResponseWriter, r *http.Request) {
	if !ensureScope(w, r, config.ScopeMonitoringRead) {
		return
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	queue := h.monitor.GetNotificationManager().GetQueue()
	if queue == nil {
		http.Error(w, "Notification queue not initialized", http.StatusServiceUnavailable)
		return
	}

	dlq, err := queue.GetDLQ(limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get DLQ")
		http.Error(w, "Failed to retrieve dead letter queue", http.StatusInternalServerError)
		return
	}

	if err := utils.WriteJSONResponse(w, dlq); err != nil {
		log.Error().Err(err).Msg("Failed to write DLQ response")
	}
}

// GetQueueStats returns statistics about the notification queue
func (h *NotificationQueueHandlers) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	if !ensureScope(w, r, config.ScopeMonitoringRead) {
		return
	}

	queue := h.monitor.GetNotificationManager().GetQueue()
	if queue == nil {
		http.Error(w, "Notification queue not initialized", http.StatusServiceUnavailable)
		return
	}

	stats, err := queue.GetQueueStats()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get queue stats")
		http.Error(w, "Failed to retrieve queue statistics", http.StatusInternalServerError)
		return
	}

	if err := utils.WriteJSONResponse(w, stats); err != nil {
		log.Error().Err(err).Msg("Failed to write queue stats response")
	}
}

// RetryDLQItem retries a specific notification from the DLQ
func (h *NotificationQueueHandlers) RetryDLQItem(w http.ResponseWriter, r *http.Request) {
	if !ensureScope(w, r, config.ScopeMonitoringWrite) {
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	var request struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.ID == "" {
		http.Error(w, "Missing notification ID", http.StatusBadRequest)
		return
	}

	queue := h.monitor.GetNotificationManager().GetQueue()
	if queue == nil {
		http.Error(w, "Notification queue not initialized", http.StatusServiceUnavailable)
		return
	}

	// Reset notification to pending status with immediate retry
	if err := queue.ScheduleRetry(request.ID, 0); err != nil {
		log.Error().Err(err).Str("id", request.ID).Msg("Failed to retry DLQ item")
		http.Error(w, "Failed to retry notification", http.StatusInternalServerError)
		return
	}

	log.Info().Str("id", request.ID).Msg("DLQ notification scheduled for retry")

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Notification scheduled for retry",
		"id":      request.ID,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write retry response")
	}
}

// DeleteDLQItem removes a notification from the DLQ permanently
func (h *NotificationQueueHandlers) DeleteDLQItem(w http.ResponseWriter, r *http.Request) {
	if !ensureScope(w, r, config.ScopeMonitoringWrite) {
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	var request struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.ID == "" {
		http.Error(w, "Missing notification ID", http.StatusBadRequest)
		return
	}

	queue := h.monitor.GetNotificationManager().GetQueue()
	if queue == nil {
		http.Error(w, "Notification queue not initialized", http.StatusServiceUnavailable)
		return
	}

	// Update status to deleted/cancelled
	if err := queue.UpdateStatus(request.ID, notifications.QueueStatusCancelled, "Manually deleted from DLQ"); err != nil {
		log.Error().Err(err).Str("id", request.ID).Msg("Failed to delete DLQ item")
		http.Error(w, "Failed to delete notification", http.StatusInternalServerError)
		return
	}

	log.Info().Str("id", request.ID).Msg("DLQ notification deleted")

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Notification deleted from DLQ",
		"id":      request.ID,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write delete response")
	}
}

// HandleNotificationQueue routes notification queue requests
func (h *NotificationQueueHandlers) HandleNotificationQueue(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/notifications/dlq" && r.Method == http.MethodGet:
		h.GetDLQ(w, r)
	case path == "/api/notifications/queue/stats" && r.Method == http.MethodGet:
		h.GetQueueStats(w, r)
	case path == "/api/notifications/dlq/retry" && r.Method == http.MethodPost:
		h.RetryDLQItem(w, r)
	case path == "/api/notifications/dlq/delete" && r.Method == http.MethodPost:
		h.DeleteDLQItem(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
