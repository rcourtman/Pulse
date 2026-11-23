package alerting

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/apihttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// NotificationQueueHandlers handles notification queue API endpoints
type NotificationQueueHandlers struct {
	mu      sync.RWMutex
	monitor *monitoring.Monitor
}

// NewNotificationQueueHandlers creates new notification queue handlers
func NewNotificationQueueHandlers(monitor *monitoring.Monitor) *NotificationQueueHandlers {
	return &NotificationQueueHandlers{
		monitor: monitor,
	}
}

// SetMonitor refreshes the queue owner after a configuration reload. Keep the
// handler itself stable: registered routes retain its method values.
func (h *NotificationQueueHandlers) SetMonitor(monitor *monitoring.Monitor) {
	h.mu.Lock()
	h.monitor = monitor
	h.mu.Unlock()
}

func (h *NotificationQueueHandlers) getQueue() *notifications.NotificationQueue {
	h.mu.RLock()
	monitor := h.monitor
	h.mu.RUnlock()
	if monitor == nil || monitor.GetNotificationManager() == nil {
		return nil
	}
	return monitor.GetNotificationManager().GetQueue()
}

// deadLetterEntry is the operator view, not the record used for delivery.
type deadLetterEntry struct {
	ID            string                                 `json:"id"`
	Type          string                                 `json:"type"`
	Method        string                                 `json:"method,omitempty"`
	DestinationID string                                 `json:"destinationId,omitempty"`
	Status        notifications.NotificationQueueStatus  `json:"status"`
	Alerts        []deadLetterAlertRef                   `json:"alerts"`
	Links         []operationaltrust.NotificationLink    `json:"links,omitempty"`
	Attempts      int                                    `json:"attempts"`
	MaxAttempts   int                                    `json:"maxAttempts"`
	FailureClass  notifications.NotificationFailureClass `json:"failureClass,omitempty"`
	LastAttempt   *time.Time                             `json:"lastAttempt,omitempty"`
	CreatedAt     time.Time                              `json:"createdAt"`
	NextRetryAt   *time.Time                             `json:"nextRetryAt,omitempty"`
	CompletedAt   *time.Time                             `json:"completedAt,omitempty"`
	PayloadBytes  *int                                   `json:"payloadBytes,omitempty"`
}

type deadLetterAlertRef struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Level      alerts.AlertLevel `json:"level"`
	ResourceID string            `json:"resourceId"`
}

func newDeadLetterEntry(item *notifications.QueuedNotification) deadLetterEntry {
	entry := deadLetterEntry{
		ID:            item.ID,
		Type:          item.Type,
		Method:        item.Method,
		DestinationID: item.DestinationID,
		Status:        item.Status,
		Links:         item.Links,
		Attempts:      item.Attempts,
		MaxAttempts:   item.MaxAttempts,
		LastAttempt:   item.LastAttempt,
		CreatedAt:     item.CreatedAt,
		NextRetryAt:   item.NextRetryAt,
		CompletedAt:   item.CompletedAt,
		PayloadBytes:  item.PayloadBytes,
	}
	if item.Alerts != nil {
		entry.Alerts = make([]deadLetterAlertRef, 0, len(item.Alerts))
		for _, alert := range item.Alerts {
			if alert != nil {
				entry.Alerts = append(entry.Alerts, deadLetterAlertRef{
					ID: alert.ID, Type: alert.Type, Level: alert.Level, ResourceID: alert.ResourceID,
				})
			}
		}
	}
	// Provider diagnostics may contain free-form response text. Keep only a
	// closed class here; per-attempt evidence belongs to the delivery log.
	if item.LastError != nil && *item.LastError != "" {
		entry.FailureClass = notifications.ClassifyNotificationFailure(*item.LastError)
	}
	return entry
}

// GetDLQ returns diagnostic summaries of dead-letter notifications.
func (h *NotificationQueueHandlers) GetDLQ(w http.ResponseWriter, r *http.Request) {
	if !apihttp.EnsureScope(w, r, config.ScopeMonitoringRead) {
		return
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	queue := h.getQueue()
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

	var entries []deadLetterEntry // Preserve the existing null when there are no rows.
	if dlq != nil {
		entries = make([]deadLetterEntry, len(dlq))
		for i, item := range dlq {
			entries[i] = newDeadLetterEntry(item)
		}
	}
	if err := utils.WriteJSONResponse(w, entries); err != nil {
		log.Error().Err(err).Msg("Failed to write DLQ response")
	}
}

// GetQueueStats returns statistics about the notification queue
func (h *NotificationQueueHandlers) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	if !apihttp.EnsureScope(w, r, config.ScopeMonitoringRead) {
		return
	}

	queue := h.getQueue()
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
	if !apihttp.EnsureScope(w, r, config.ScopeSettingsWrite) {
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

	queue := h.getQueue()
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
	if !apihttp.EnsureScope(w, r, config.ScopeSettingsWrite) {
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

	queue := h.getQueue()
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

// RetryTerminalFailures returns all retained terminal failures to the queue
// after an operator has repaired the destination.
func (h *NotificationQueueHandlers) RetryTerminalFailures(w http.ResponseWriter, r *http.Request) {
	if !apihttp.EnsureScope(w, r, config.ScopeSettingsWrite) {
		return
	}
	queue := h.getQueue()
	if queue == nil {
		http.Error(w, "Notification queue not initialized", http.StatusServiceUnavailable)
		return
	}
	affected, err := queue.RetryTerminalFailures()
	if err != nil {
		log.Error().Err(err).Msg("Failed to retry retained terminal notifications")
		http.Error(w, "Failed to retry retained notification failures", http.StatusInternalServerError)
		return
	}
	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success":  true,
		"affected": affected,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write terminal retry response")
	}
}

// DismissTerminalFailures cancels all retained terminal queue rows without
// deleting their immutable delivery audit.
func (h *NotificationQueueHandlers) DismissTerminalFailures(w http.ResponseWriter, r *http.Request) {
	if !apihttp.EnsureScope(w, r, config.ScopeSettingsWrite) {
		return
	}
	queue := h.getQueue()
	if queue == nil {
		http.Error(w, "Notification queue not initialized", http.StatusServiceUnavailable)
		return
	}
	affected, err := queue.DismissTerminalFailures()
	if err != nil {
		log.Error().Err(err).Msg("Failed to dismiss retained terminal notifications")
		http.Error(w, "Failed to dismiss retained notification failures", http.StatusInternalServerError)
		return
	}
	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success":  true,
		"affected": affected,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write terminal dismissal response")
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
	case path == "/api/notifications/terminal-failures/retry" && r.Method == http.MethodPost:
		h.RetryTerminalFailures(w, r)
	case path == "/api/notifications/terminal-failures/dismiss" && r.Method == http.MethodPost:
		h.DismissTerminalFailures(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
