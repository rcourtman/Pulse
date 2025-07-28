package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rs/zerolog/log"
)

// NotificationHandlers handles notification-related HTTP endpoints
type NotificationHandlers struct {
	monitor *monitoring.Monitor
}

// NewNotificationHandlers creates new notification handlers
func NewNotificationHandlers(monitor *monitoring.Monitor) *NotificationHandlers {
	return &NotificationHandlers{
		monitor: monitor,
	}
}

// GetEmailConfig returns the current email configuration
func (h *NotificationHandlers) GetEmailConfig(w http.ResponseWriter, r *http.Request) {
	config := h.monitor.GetNotificationManager().GetEmailConfig()
	
	// For security, don't return the password
	config.Password = ""
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateEmailConfig updates the email configuration
func (h *NotificationHandlers) UpdateEmailConfig(w http.ResponseWriter, r *http.Request) {
	var config notifications.EmailConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	h.monitor.GetNotificationManager().SetEmailConfig(config)
	
	// Save to persistent storage
	if err := h.monitor.GetConfigPersistence().SaveEmailConfig(config); err != nil {
		// Log error but don't fail the request
		log.Error().Err(err).Msg("Failed to save email configuration")
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// GetWebhooks returns all webhook configurations
func (h *NotificationHandlers) GetWebhooks(w http.ResponseWriter, r *http.Request) {
	webhooks := h.monitor.GetNotificationManager().GetWebhooks()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhooks)
}

// CreateWebhook creates a new webhook
func (h *NotificationHandlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	// Read the raw body to preserve all fields
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	var webhook notifications.WebhookConfig
	if err := json.Unmarshal(bodyBytes, &webhook); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Generate ID if not provided
	if webhook.ID == "" {
		webhook.ID = generateWebhookID()
	}
	
	h.monitor.GetNotificationManager().AddWebhook(webhook)
	
	// Save webhooks to persistent storage with all fields
	webhooks := h.monitor.GetNotificationManager().GetWebhooks()
	if err := h.monitor.GetConfigPersistence().SaveWebhooks(webhooks); err != nil {
		log.Error().Err(err).Msg("Failed to save webhooks")
	}
	
	// Return the full webhook data including any extra fields like 'service'
	var responseData map[string]interface{}
	json.Unmarshal(bodyBytes, &responseData)
	responseData["id"] = webhook.ID
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseData)
}

// UpdateWebhook updates an existing webhook
func (h *NotificationHandlers) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	// Extract webhook ID from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	webhookID := parts[len(parts)-1]
	
	var webhook notifications.WebhookConfig
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	webhook.ID = webhookID
	if err := h.monitor.GetNotificationManager().UpdateWebhook(webhookID, webhook); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	// Save webhooks to persistent storage
	webhooks := h.monitor.GetNotificationManager().GetWebhooks()
	if err := h.monitor.GetConfigPersistence().SaveWebhooks(webhooks); err != nil {
		log.Error().Err(err).Msg("Failed to save webhooks")
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhook)
}

// DeleteWebhook deletes a webhook
func (h *NotificationHandlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	// Extract webhook ID from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	webhookID := parts[len(parts)-1]
	
	if err := h.monitor.GetNotificationManager().DeleteWebhook(webhookID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	// Save webhooks to persistent storage
	webhooks := h.monitor.GetNotificationManager().GetWebhooks()
	if err := h.monitor.GetConfigPersistence().SaveWebhooks(webhooks); err != nil {
		log.Error().Err(err).Msg("Failed to save webhooks")
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// TestNotification sends a test notification
func (h *NotificationHandlers) TestNotification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method string `json:"method"` // "email" or "webhook"
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	if err := h.monitor.GetNotificationManager().SendTestNotification(req.Method); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Test notification sent"})
}

// GetWebhookTemplates returns available webhook templates
func (h *NotificationHandlers) GetWebhookTemplates(w http.ResponseWriter, r *http.Request) {
	templates := notifications.GetWebhookTemplates()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(templates)
}

// GetEmailProviders returns available email providers
func (h *NotificationHandlers) GetEmailProviders(w http.ResponseWriter, r *http.Request) {
	providers := notifications.GetEmailProviders()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

// TestWebhook tests a webhook configuration
func (h *NotificationHandlers) TestWebhook(w http.ResponseWriter, r *http.Request) {
	// First try to decode as basic webhook config
	var basicWebhook notifications.WebhookConfig
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	if err := json.Unmarshal(bodyBytes, &basicWebhook); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Convert to enhanced webhook for testing
	webhook := notifications.EnhancedWebhookConfig{
		WebhookConfig: basicWebhook,
		Service:       "generic", // Default to generic if not specified
		RetryEnabled:  false,     // Don't retry during testing
	}
	
	// Try to extract service from body if present
	var serviceCheck struct {
		Service string `json:"service"`
	}
	if err := json.Unmarshal(bodyBytes, &serviceCheck); err == nil && serviceCheck.Service != "" {
		webhook.Service = serviceCheck.Service
	}
	
	// Get template for the service
	templates := notifications.GetWebhookTemplates()
	for _, tmpl := range templates {
		if tmpl.Service == webhook.Service {
			webhook.PayloadTemplate = tmpl.PayloadTemplate
			if webhook.Headers == nil {
				webhook.Headers = make(map[string]string)
			}
			for k, v := range tmpl.Headers {
				webhook.Headers[k] = v
			}
			break
		}
	}
	
	// If no template found, use a simple generic template
	if webhook.PayloadTemplate == "" {
		webhook.PayloadTemplate = `{
			"alert": {
				"id": "{{.ID}}",
				"type": "{{.Type}}",
				"level": "{{.Level}}",
				"resourceName": "{{.ResourceName}}",
				"node": "{{.Node}}",
				"message": "{{.Message}}",
				"value": {{.Value}},
				"threshold": {{.Threshold}}
			},
			"source": "pulse-monitoring",
			"timestamp": {{.Timestamp}}
		}`
	}
	
	// Test the webhook
	status, response, err := h.monitor.GetNotificationManager().TestEnhancedWebhook(webhook)
	
	result := map[string]interface{}{
		"status": status,
		"response": response,
	}
	
	if err != nil {
		result["error"] = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	} else {
		result["success"] = true
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleNotifications routes notification requests to appropriate handlers
func (h *NotificationHandlers) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/notifications")
	
	switch {
	case path == "/email" && r.Method == http.MethodGet:
		h.GetEmailConfig(w, r)
	case path == "/email" && r.Method == http.MethodPut:
		h.UpdateEmailConfig(w, r)
	case path == "/webhooks" && r.Method == http.MethodGet:
		h.GetWebhooks(w, r)
	case path == "/webhooks" && r.Method == http.MethodPost:
		h.CreateWebhook(w, r)
	case path == "/webhooks/test" && r.Method == http.MethodPost:
		h.TestWebhook(w, r)
	case strings.HasPrefix(path, "/webhooks/") && r.Method == http.MethodPut:
		h.UpdateWebhook(w, r)
	case strings.HasPrefix(path, "/webhooks/") && r.Method == http.MethodDelete:
		h.DeleteWebhook(w, r)
	case path == "/webhook-templates" && r.Method == http.MethodGet:
		h.GetWebhookTemplates(w, r)
	case path == "/email-providers" && r.Method == http.MethodGet:
		h.GetEmailProviders(w, r)
	case path == "/test" && r.Method == http.MethodPost:
		h.TestNotification(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// generateWebhookID generates a unique webhook ID
func generateWebhookID() string {
	return fmt.Sprintf("webhook-%d", time.Now().UnixNano())
}