package api

import (
	"encoding/json"
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
	
	log.Info().
		Str("body", string(body)).
		Msg("Received email config update")
	
	var config notifications.EmailConfig
	if err := json.Unmarshal(body, &config); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to parse email config")
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
	json.Unmarshal(bodyBytes, &responseData)
	responseData["id"] = webhookID
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseData)
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
	// Read body for debugging
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	log.Info().
		Str("body", string(body)).
		Msg("Test notification request received")
	
	var req struct {
		Method    string                    `json:"method"` // "email" or "webhook"
		Type      string                    `json:"type"`   // Alternative field name used by frontend
		Config    *notifications.EmailConfig `json:"config,omitempty"` // Optional config for testing
		WebhookID string                    `json:"webhookId,omitempty"` // For webhook testing
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

