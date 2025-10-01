package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
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
	// Read raw body for debugging
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// NEVER log the body as it contains passwords
	log.Info().
		Msg("Received email config update")

	var config notifications.EmailConfig
	if err := json.Unmarshal(body, &config); err != nil {
		log.Error().Err(err).Msg("Failed to parse email config") // Don't log body with passwords
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If password is empty, preserve the existing password
	if config.Password == "" {
		existingConfig := h.monitor.GetNotificationManager().GetEmailConfig()
		config.Password = existingConfig.Password
	}

	log.Info().
		Bool("enabled", config.Enabled).
		Str("smtp", config.SMTPHost).
		Str("from", config.From).
		Int("toCount", len(config.To)).
		Bool("hasPassword", config.Password != "").
		Msg("Parsed email config")

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

	// Validate webhook URL
	if err := notifications.ValidateWebhookURL(webhook.URL); err != nil {
		http.Error(w, fmt.Sprintf("Invalid webhook URL: %v", err), http.StatusBadRequest)
		return
	}

	// Generate ID if not provided
	if webhook.ID == "" {
		webhook.ID = utils.GenerateID("webhook")
	}

	h.monitor.GetNotificationManager().AddWebhook(webhook)

	// Save webhooks to persistent storage with all fields
	webhooks := h.monitor.GetNotificationManager().GetWebhooks()
	if err := h.monitor.GetConfigPersistence().SaveWebhooks(webhooks); err != nil {
		log.Error().Err(err).Msg("Failed to save webhooks")
	}

	// Return the full webhook data including any extra fields like 'service'
	var responseData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal webhook response data")
		responseData = make(map[string]interface{})
	}
	responseData["id"] = webhook.ID

	if err := utils.WriteJSONResponse(w, responseData); err != nil {
		log.Error().Err(err).Msg("Failed to write webhook creation response")
	}
}

// UpdateWebhook updates an existing webhook
func (h *NotificationHandlers) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	// Extract webhook ID from URL path
	// Path is like /api/notifications/webhooks/{id} after routing
	path := strings.TrimPrefix(r.URL.Path, "/api/notifications/webhooks/")
	webhookID := path

	if webhookID == "" {
		http.Error(w, "Invalid URL - missing webhook ID", http.StatusBadRequest)
		return
	}

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

	// Validate webhook URL
	if err := notifications.ValidateWebhookURL(webhook.URL); err != nil {
		http.Error(w, fmt.Sprintf("Invalid webhook URL: %v", err), http.StatusBadRequest)
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

	// Return the full webhook data including any extra fields like 'service'
	var responseData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal webhook response data")
		responseData = make(map[string]interface{})
	}
	responseData["id"] = webhookID

	if err := utils.WriteJSONResponse(w, responseData); err != nil {
		log.Error().Err(err).Str("webhookID", webhookID).Msg("Failed to write webhook update response")
	}
}

// DeleteWebhook deletes a webhook
func (h *NotificationHandlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	// Extract webhook ID from URL path
	// Path is like /api/notifications/webhooks/{id} after routing
	path := strings.TrimPrefix(r.URL.Path, "/api/notifications/webhooks/")
	webhookID := path

	if webhookID == "" {
		http.Error(w, "Invalid URL - missing webhook ID", http.StatusBadRequest)
		return
	}

	if err := h.monitor.GetNotificationManager().DeleteWebhook(webhookID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Save webhooks to persistent storage
	webhooks := h.monitor.GetNotificationManager().GetWebhooks()
	if err := h.monitor.GetConfigPersistence().SaveWebhooks(webhooks); err != nil {
		log.Error().Err(err).Msg("Failed to save webhooks")
	}

	if err := utils.WriteJSONResponse(w, map[string]string{"status": "success"}); err != nil {
		log.Error().Err(err).Str("webhookID", webhookID).Msg("Failed to write webhook deletion response")
	}
}

// TestNotification sends a test notification
func (h *NotificationHandlers) TestNotification(w http.ResponseWriter, r *http.Request) {
	// Read body for debugging
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// NEVER log the body as it contains passwords
	log.Info().
		Msg("Test notification request received")

	var req struct {
		Method    string                     `json:"method"`              // "email" or "webhook"
		Type      string                     `json:"type"`                // Alternative field name used by frontend
		Config    *notifications.EmailConfig `json:"config,omitempty"`    // Optional config for testing
		WebhookID string                     `json:"webhookId,omitempty"` // For webhook testing
	}

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Support both "method" and "type" field names
	if req.Method == "" && req.Type != "" {
		req.Method = req.Type
	}

	// Get actual node info from monitor state
	state := h.monitor.GetState()
	var nodeInfo *notifications.TestNodeInfo

	// Use first available node and instance
	if len(state.Nodes) > 0 {
		for _, node := range state.Nodes {
			nodeInfo = &notifications.TestNodeInfo{
				NodeName:    node.Name,
				InstanceURL: node.Instance,
			}
			break
		}
	}

	// Handle webhook testing
	if req.Method == "webhook" && req.WebhookID != "" {
		log.Info().
			Str("webhookId", req.WebhookID).
			Msg("Testing specific webhook")

		// Get the webhook by ID and test it
		webhooks := h.monitor.GetNotificationManager().GetWebhooks()
		var foundWebhook *notifications.WebhookConfig
		for _, wh := range webhooks {
			if wh.ID == req.WebhookID {
				foundWebhook = &wh
				break
			}
		}

		if foundWebhook == nil {
			http.Error(w, "Webhook not found", http.StatusNotFound)
			return
		}

		// Send test webhook
		if err := h.monitor.GetNotificationManager().SendTestWebhook(*foundWebhook); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else if req.Method == "email" && req.Config != nil {
		// If config is provided, use it for testing (without saving)
		// If password is empty, use the saved password
		if req.Config.Password == "" {
			savedConfig := h.monitor.GetNotificationManager().GetEmailConfig()
			req.Config.Password = savedConfig.Password
		}

		log.Info().
			Bool("enabled", req.Config.Enabled).
			Str("smtp", req.Config.SMTPHost).
			Str("from", req.Config.From).
			Int("toCount", len(req.Config.To)).
			Strs("to", req.Config.To).
			Bool("hasPassword", req.Config.Password != "").
			Msg("Testing email with provided config")

		if err := h.monitor.GetNotificationManager().SendTestNotificationWithConfig(req.Method, req.Config, nodeInfo); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Use saved config
		if err := h.monitor.GetNotificationManager().SendTestNotification(req.Method); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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

// GetWebhookHistory returns recent webhook delivery history
func (h *NotificationHandlers) GetWebhookHistory(w http.ResponseWriter, r *http.Request) {
	history := h.monitor.GetNotificationManager().GetWebhookHistory()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
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

	if len(basicWebhook.CustomFields) > 0 {
		customFields := make(map[string]interface{}, len(basicWebhook.CustomFields))
		for key, value := range basicWebhook.CustomFields {
			customFields[key] = value
		}
		webhook.CustomFields = customFields
	}

	// If the webhook has a custom template, use it
	if basicWebhook.Template != "" {
		webhook.PayloadTemplate = basicWebhook.Template
	}

	// Try to extract service from body if present
	var serviceCheck struct {
		Service string `json:"service"`
	}
	if err := json.Unmarshal(bodyBytes, &serviceCheck); err == nil && serviceCheck.Service != "" {
		webhook.Service = serviceCheck.Service
		// Also set it in the basic webhook for consistency
		basicWebhook.Service = serviceCheck.Service
		webhook.WebhookConfig.Service = serviceCheck.Service
	}

	log.Info().
		Str("service", webhook.Service).
		Str("url", webhook.URL).
		Str("name", webhook.Name).
		Msg("Testing webhook")

	// Get template for the service (if not using custom template)
	if webhook.PayloadTemplate == "" {
		templates := notifications.GetWebhookTemplates()
		for _, tmpl := range templates {
			if tmpl.Service == webhook.Service {
				webhook.PayloadTemplate = tmpl.PayloadTemplate
				if webhook.Headers == nil {
					webhook.Headers = make(map[string]string)
				}
				// Only copy headers that don't contain template syntax
				// This prevents issues with headers that have Go template expressions
				for k, v := range tmpl.Headers {
					if !strings.Contains(v, "{{") {
						webhook.Headers[k] = v
					}
				}
				log.Info().Str("service", webhook.Service).Msg("Found template for service")
				break
			}
		}
	}

	// If still no template found, use a simple generic template
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
		"status":   status,
		"response": response,
	}

	if err != nil {
		result["error"] = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	} else if status < 200 || status >= 300 {
		// HTTP error from webhook endpoint
		result["error"] = fmt.Sprintf("Webhook returned HTTP %d: %s", status, response)
		result["success"] = false
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
	case path == "/webhook-history" && r.Method == http.MethodGet:
		h.GetWebhookHistory(w, r)
	case path == "/email-providers" && r.Method == http.MethodGet:
		h.GetEmailProviders(w, r)
	case path == "/test" && r.Method == http.MethodPost:
		h.TestNotification(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
