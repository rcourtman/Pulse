package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// NotificationManager defines the interface for notification management operations.
type NotificationManager interface {
	GetEmailConfig() notifications.EmailConfig
	SetEmailConfig(notifications.EmailConfig)
	GetAppriseConfig() notifications.AppriseConfig
	SetAppriseConfig(notifications.AppriseConfig)
	GetWebhooks() []notifications.WebhookConfig
	ValidateWebhookURL(string) error
	AddWebhook(notifications.WebhookConfig)
	UpdateWebhook(string, notifications.WebhookConfig) error
	DeleteWebhook(string) error
	SendTestWebhook(notifications.WebhookConfig) error
	SendTestNotificationWithConfig(string, *notifications.EmailConfig, *notifications.TestNodeInfo) error
	SendTestAppriseWithConfig(notifications.AppriseConfig) error
	SendTestNotification(string) error
	GetWebhookHistory() []notifications.WebhookDelivery
	TestEnhancedWebhook(notifications.EnhancedWebhookConfig) (int, string, error)
	GetQueueStats() (map[string]int, error)
}

// NotificationConfigPersistence defines the interface for saving notification configuration.
type NotificationConfigPersistence interface {
	SaveEmailConfig(notifications.EmailConfig) error
	SaveAppriseConfig(notifications.AppriseConfig) error
	SaveWebhooks([]notifications.WebhookConfig) error
	IsEncryptionEnabled() bool
}

// NotificationMonitor defines the interface for monitoring operations used by notification handlers.
type NotificationMonitor interface {
	GetNotificationManager() NotificationManager
	GetConfigPersistence() NotificationConfigPersistence
	GetState() models.StateSnapshot
}

// NotificationHandlers handles notification-related HTTP endpoints
type NotificationHandlers struct {
	mtMonitor     *monitoring.MultiTenantMonitor
	legacyMonitor NotificationMonitor
}

// NewNotificationHandlers creates new notification handlers
func NewNotificationHandlers(mtm *monitoring.MultiTenantMonitor, monitor NotificationMonitor) *NotificationHandlers {
	// If mtm is provided, try to populate legacyMonitor from "default" org if not provided
	if monitor == nil && mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			monitor = NewNotificationMonitorWrapper(m)
		}
	}
	return &NotificationHandlers{
		mtMonitor:     mtm,
		legacyMonitor: monitor,
	}
}

// SetMonitor updates the monitor reference for notification handlers.
func (h *NotificationHandlers) SetMonitor(m NotificationMonitor) {
	h.legacyMonitor = m
}

// SetMultiTenantMonitor updates the multi-tenant monitor reference
func (h *NotificationHandlers) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	h.mtMonitor = mtm
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			h.legacyMonitor = NewNotificationMonitorWrapper(m)
		}
	}
}

func (h *NotificationHandlers) getMonitor(ctx context.Context) NotificationMonitor {
	orgID := GetOrgID(ctx)
	if h.mtMonitor != nil {
		if m, err := h.mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return NewNotificationMonitorWrapper(m)
		}
	}
	return h.legacyMonitor
}

// GetEmailConfig returns the current email configuration
func (h *NotificationHandlers) GetEmailConfig(w http.ResponseWriter, r *http.Request) {
	config := h.getMonitor(r.Context()).GetNotificationManager().GetEmailConfig()

	// For security, don't return the password
	config.Password = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateEmailConfig updates the email configuration
func (h *NotificationHandlers) UpdateEmailConfig(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

	// Read raw body for debugging
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// NEVER log the body as it contains passwords
	log.Info().
		Msg("Received email config update")

	// Parse strict subset to check for presence of fields
	var presenceCheck struct {
		RateLimit *int `json:"rateLimit"`
	}
	if err := json.Unmarshal(body, &presenceCheck); err != nil {
		// Non-fatal, just means we can't do presence check
	}

	var config notifications.EmailConfig
	if err := json.Unmarshal(body, &config); err != nil {
		log.Error().Err(err).Msg("Failed to parse email config") // Don't log body with passwords
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existingConfig := h.getMonitor(r.Context()).GetNotificationManager().GetEmailConfig()

	// If password is empty, preserve the existing password
	if config.Password == "" {
		config.Password = existingConfig.Password
	}

	// If rateLimit was NOT provided (nil in presence check), preserve existing
	if presenceCheck.RateLimit == nil {
		config.RateLimit = existingConfig.RateLimit
	}

	log.Info().
		Bool("enabled", config.Enabled).
		Str("smtp", config.SMTPHost).
		Str("from", config.From).
		Int("toCount", len(config.To)).
		Bool("hasPassword", config.Password != "").
		Int("rateLimit", config.RateLimit).
		Msg("Parsed email config")

	h.getMonitor(r.Context()).GetNotificationManager().SetEmailConfig(config)

	// Save to persistent storage
	if err := h.getMonitor(r.Context()).GetConfigPersistence().SaveEmailConfig(config); err != nil {
		// Log error but don't fail the request
		log.Error().Err(err).Msg("Failed to save email configuration")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// GetAppriseConfig returns the current Apprise configuration.
func (h *NotificationHandlers) GetAppriseConfig(w http.ResponseWriter, r *http.Request) {
	config := h.getMonitor(r.Context()).GetNotificationManager().GetAppriseConfig()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(config); err != nil {
		log.Error().Err(err).Msg("Failed to encode Apprise configuration response")
	}
}

// UpdateAppriseConfig updates the Apprise configuration.
func (h *NotificationHandlers) UpdateAppriseConfig(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 64KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var config notifications.AppriseConfig
	if err := json.Unmarshal(body, &config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Info().
		Bool("enabled", config.Enabled).
		Str("mode", string(config.Mode)).
		Int("targetCount", len(config.Targets)).
		Str("cliPath", config.CLIPath).
		Str("serverUrl", config.ServerURL).
		Str("configKey", config.ConfigKey).
		Bool("hasApiKey", config.APIKey != "").
		Str("apiKeyHeader", config.APIKeyHeader).
		Bool("skipTlsVerify", config.SkipTLSVerify).
		Int("timeoutSeconds", config.TimeoutSeconds).
		Msg("Parsed Apprise configuration update")

	h.getMonitor(r.Context()).GetNotificationManager().SetAppriseConfig(config)

	if err := h.getMonitor(r.Context()).GetConfigPersistence().SaveAppriseConfig(config); err != nil {
		log.Error().Err(err).Msg("Failed to save Apprise configuration")
	}

	normalized := h.getMonitor(r.Context()).GetNotificationManager().GetAppriseConfig()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(normalized); err != nil {
		log.Error().Err(err).Msg("Failed to encode Apprise configuration response")
	}
}

// GetWebhooks returns all webhook configurations with secrets masked
func (h *NotificationHandlers) GetWebhooks(w http.ResponseWriter, r *http.Request) {
	webhooks := h.getMonitor(r.Context()).GetNotificationManager().GetWebhooks()

	// Mask sensitive fields in headers and customFields
	maskedWebhooks := make([]map[string]interface{}, len(webhooks))
	for i, webhook := range webhooks {
		whMap := map[string]interface{}{
			"id":      webhook.ID,
			"name":    webhook.Name,
			"url":     webhook.URL,
			"method":  webhook.Method,
			"enabled": webhook.Enabled,
			"service": webhook.Service,
		}

		// Mask headers - only show keys, not values
		if len(webhook.Headers) > 0 {
			maskedHeaders := make(map[string]string)
			for key := range webhook.Headers {
				maskedHeaders[key] = "***REDACTED***"
			}
			whMap["headers"] = maskedHeaders
		}

		// Mask custom fields - only show keys, not values
		if len(webhook.CustomFields) > 0 {
			maskedFields := make(map[string]string)
			for key := range webhook.CustomFields {
				maskedFields[key] = "***REDACTED***"
			}
			whMap["customFields"] = maskedFields
		}

		// Include template if present
		if webhook.Template != "" {
			whMap["template"] = webhook.Template
		}

		maskedWebhooks[i] = whMap
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(maskedWebhooks)
}

// CreateWebhook creates a new webhook
func (h *NotificationHandlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 64KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

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
	if err := h.getMonitor(r.Context()).GetNotificationManager().ValidateWebhookURL(webhook.URL); err != nil {
		http.Error(w, fmt.Sprintf("Invalid webhook URL: %v", err), http.StatusBadRequest)
		return
	}

	// Generate ID if not provided
	if webhook.ID == "" {
		webhook.ID = utils.GenerateID("webhook")
	}

	h.getMonitor(r.Context()).GetNotificationManager().AddWebhook(webhook)

	// Save webhooks to persistent storage with all fields
	webhooks := h.getMonitor(r.Context()).GetNotificationManager().GetWebhooks()
	if err := h.getMonitor(r.Context()).GetConfigPersistence().SaveWebhooks(webhooks); err != nil {
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

	// Limit request body to 64KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

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

	// Preserve original headers/customFields if the incoming values are redacted
	// This happens when the frontend sends back masked values from GetWebhooks
	existingWebhooks := h.getMonitor(r.Context()).GetNotificationManager().GetWebhooks()
	for _, existing := range existingWebhooks {
		if existing.ID == webhookID {
			// Preserve headers if incoming contains redacted values
			if len(webhook.Headers) > 0 && len(existing.Headers) > 0 {
				hasRedacted := false
				for _, v := range webhook.Headers {
					if v == "***REDACTED***" {
						hasRedacted = true
						break
					}
				}
				if hasRedacted {
					webhook.Headers = existing.Headers
				}
			}
			// Preserve customFields if incoming contains redacted values
			if len(webhook.CustomFields) > 0 && len(existing.CustomFields) > 0 {
				hasRedacted := false
				for _, v := range webhook.CustomFields {
					if v == "***REDACTED***" {
						hasRedacted = true
						break
					}
				}
				if hasRedacted {
					webhook.CustomFields = existing.CustomFields
				}
			}
			break
		}
	}

	// Validate webhook URL
	if err := h.getMonitor(r.Context()).GetNotificationManager().ValidateWebhookURL(webhook.URL); err != nil {
		http.Error(w, fmt.Sprintf("Invalid webhook URL: %v", err), http.StatusBadRequest)
		return
	}

	webhook.ID = webhookID
	if err := h.getMonitor(r.Context()).GetNotificationManager().UpdateWebhook(webhookID, webhook); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Save webhooks to persistent storage
	webhooks := h.getMonitor(r.Context()).GetNotificationManager().GetWebhooks()
	if err := h.getMonitor(r.Context()).GetConfigPersistence().SaveWebhooks(webhooks); err != nil {
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

	if err := h.getMonitor(r.Context()).GetNotificationManager().DeleteWebhook(webhookID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Save webhooks to persistent storage
	webhooks := h.getMonitor(r.Context()).GetNotificationManager().GetWebhooks()
	if err := h.getMonitor(r.Context()).GetConfigPersistence().SaveWebhooks(webhooks); err != nil {
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
		Method    string          `json:"method"`              // "email", "webhook", or "apprise"
		Type      string          `json:"type"`                // Alternative field name used by frontend
		Config    json.RawMessage `json:"config,omitempty"`    // Optional config for testing (email or apprise)
		WebhookID string          `json:"webhookId,omitempty"` // For webhook testing
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
	state := h.getMonitor(r.Context()).GetState()
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
		webhooks := h.getMonitor(r.Context()).GetNotificationManager().GetWebhooks()
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
		if err := h.getMonitor(r.Context()).GetNotificationManager().SendTestWebhook(*foundWebhook); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else if req.Method == "email" && len(req.Config) > 0 {
		var emailConfig notifications.EmailConfig
		if err := json.Unmarshal(req.Config, &emailConfig); err != nil {
			http.Error(w, fmt.Sprintf("Invalid email config: %v", err), http.StatusBadRequest)
			return
		}

		// If password is empty, use the saved password
		if emailConfig.Password == "" {
			savedConfig := h.getMonitor(r.Context()).GetNotificationManager().GetEmailConfig()
			emailConfig.Password = savedConfig.Password
		}

		log.Info().
			Bool("enabled", emailConfig.Enabled).
			Str("smtp", emailConfig.SMTPHost).
			Str("from", emailConfig.From).
			Int("toCount", len(emailConfig.To)).
			Strs("to", emailConfig.To).
			Bool("hasPassword", emailConfig.Password != "").
			Msg("Testing email with provided config")

		if err := h.getMonitor(r.Context()).GetNotificationManager().SendTestNotificationWithConfig(req.Method, &emailConfig, nodeInfo); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else if req.Method == "apprise" && len(req.Config) > 0 {
		var appriseConfig notifications.AppriseConfig
		if err := json.Unmarshal(req.Config, &appriseConfig); err != nil {
			http.Error(w, fmt.Sprintf("Invalid Apprise config: %v", err), http.StatusBadRequest)
			return
		}

		if err := h.getMonitor(r.Context()).GetNotificationManager().SendTestAppriseWithConfig(appriseConfig); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Use saved config
		if err := h.getMonitor(r.Context()).GetNotificationManager().SendTestNotification(req.Method); err != nil {
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

// GetWebhookHistory returns recent webhook delivery history with URLs redacted
func (h *NotificationHandlers) GetWebhookHistory(w http.ResponseWriter, r *http.Request) {
	history := h.getMonitor(r.Context()).GetNotificationManager().GetWebhookHistory()

	// Redact secrets from URLs in history
	for i := range history {
		history[i].WebhookURL = redactSecretsFromURL(history[i].WebhookURL)
		// Note: ResponseBody is not stored in WebhookDelivery struct
		// Error messages are already limited in length
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// redactSecretsFromURL masks tokens and credentials in URLs
func redactSecretsFromURL(urlStr string) string {
	// Redact common patterns like:
	// - /bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11/sendMessage → /botXXX:REDACTED/sendMessage
	// - ?token=abc123 → ?token=REDACTED
	// - ?apikey=abc123 → ?apikey=REDACTED

	// Redact Telegram bot tokens
	if idx := strings.Index(urlStr, "/bot"); idx != -1 {
		// Search for next "/" after "/bot" (starting at idx+4)
		if endIdx := strings.Index(urlStr[idx+4:], "/"); endIdx != -1 {
			urlStr = urlStr[:idx+4] + "REDACTED" + urlStr[idx+4+endIdx:]
		} else {
			// No trailing slash - token extends to end of URL or query string
			if qIdx := strings.Index(urlStr[idx+4:], "?"); qIdx != -1 {
				urlStr = urlStr[:idx+4] + "REDACTED" + urlStr[idx+4+qIdx:]
			} else {
				urlStr = urlStr[:idx+4] + "REDACTED"
			}
		}
	}

	// Redact query parameters with sensitive names
	if qIdx := strings.Index(urlStr, "?"); qIdx != -1 {
		sensitiveParams := []string{"token", "apikey", "api_key", "key", "secret", "password"}
		for _, param := range sensitiveParams {
			pattern := param + "="
			// Search for the pattern after the query string starts
			searchStart := qIdx
			for {
				paramIdx := strings.Index(urlStr[searchStart:], pattern)
				if paramIdx == -1 {
					break
				}
				paramIdx += searchStart // Convert to absolute index

				// Check that we're at a parameter boundary (after ? or &)
				if paramIdx > 0 {
					prevChar := urlStr[paramIdx-1]
					if prevChar != '?' && prevChar != '&' {
						// Not at a boundary - this is part of another param name
						// Move past this match and continue searching
						searchStart = paramIdx + len(pattern)
						continue
					}
				}

				// Valid match - redact the value
				start := paramIdx + len(pattern)
				end := start
				for end < len(urlStr) && urlStr[end] != '&' && urlStr[end] != '#' {
					end++
				}
				urlStr = urlStr[:start] + "REDACTED" + urlStr[end:]
				// After modification, continue from after the inserted REDACTED
				searchStart = start + len("REDACTED")
			}
		}
	}

	return urlStr
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
	status, response, err := h.getMonitor(r.Context()).GetNotificationManager().TestEnhancedWebhook(webhook)

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

// GetNotificationHealth returns health status of notification system
func (h *NotificationHandlers) GetNotificationHealth(w http.ResponseWriter, r *http.Request) {
	// Get queue stats
	queueStats := make(map[string]interface{})
	stats, err := h.getMonitor(r.Context()).GetNotificationManager().GetQueueStats()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get queue stats for health check")
		queueStats["error"] = err.Error()
		queueStats["healthy"] = false
	} else {
		queueStats = map[string]interface{}{
			"pending": stats["pending"],
			"sending": stats["sending"],
			"sent":    stats["sent"],
			"failed":  stats["failed"],
			"dlq":     stats["dlq"],
			"healthy": true,
		}
	}

	// Get config status
	nm := h.getMonitor(r.Context()).GetNotificationManager()
	emailCfg := nm.GetEmailConfig()
	webhooks := nm.GetWebhooks()

	health := map[string]interface{}{
		"queue": queueStats,
		"email": map[string]interface{}{
			"enabled":    emailCfg.Enabled,
			"configured": emailCfg.SMTPHost != "",
		},
		"webhooks": map[string]interface{}{
			"total":   len(webhooks),
			"enabled": countEnabledWebhooks(webhooks),
		},
		"encryption": map[string]interface{}{
			"enabled": h.getMonitor(r.Context()).GetConfigPersistence().IsEncryptionEnabled(),
		},
		"overall_healthy": queueStats["healthy"] == true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func countEnabledWebhooks(webhooks []notifications.WebhookConfig) int {
	count := 0
	for _, wh := range webhooks {
		if wh.Enabled {
			count++
		}
	}
	return count
}

// HandleNotifications routes notification requests to appropriate handlers
func (h *NotificationHandlers) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/notifications")

	requireAnyScope := func(required string, scopes ...string) bool {
		record := getAPITokenRecordFromRequest(r)
		if record == nil {
			return true
		}
		for _, scope := range scopes {
			if scope != "" && record.HasScope(scope) {
				return true
			}
		}
		respondMissingScope(w, required)
		return false
	}

	switch {
	case path == "/email" && r.Method == http.MethodGet:
		if !requireAnyScope(config.ScopeSettingsRead, config.ScopeSettingsRead, config.ScopeSettingsWrite) {
			return
		}
		h.GetEmailConfig(w, r)
	case path == "/email" && r.Method == http.MethodPut:
		if !requireAnyScope(config.ScopeSettingsWrite, config.ScopeSettingsWrite) {
			return
		}
		h.UpdateEmailConfig(w, r)
	case path == "/apprise" && r.Method == http.MethodGet:
		if !requireAnyScope(config.ScopeSettingsRead, config.ScopeSettingsRead, config.ScopeSettingsWrite) {
			return
		}
		h.GetAppriseConfig(w, r)
	case path == "/apprise" && r.Method == http.MethodPut:
		if !requireAnyScope(config.ScopeSettingsWrite, config.ScopeSettingsWrite) {
			return
		}
		h.UpdateAppriseConfig(w, r)
	case path == "/webhooks" && r.Method == http.MethodGet:
		if !requireAnyScope(config.ScopeSettingsRead, config.ScopeSettingsRead, config.ScopeSettingsWrite) {
			return
		}
		h.GetWebhooks(w, r)
	case path == "/webhooks" && r.Method == http.MethodPost:
		if !requireAnyScope(config.ScopeSettingsWrite, config.ScopeSettingsWrite) {
			return
		}
		h.CreateWebhook(w, r)
	case path == "/webhooks/test" && r.Method == http.MethodPost:
		if !requireAnyScope(config.ScopeSettingsWrite, config.ScopeSettingsWrite) {
			return
		}
		h.TestWebhook(w, r)
	case strings.HasPrefix(path, "/webhooks/") && r.Method == http.MethodPut:
		if !requireAnyScope(config.ScopeSettingsWrite, config.ScopeSettingsWrite) {
			return
		}
		h.UpdateWebhook(w, r)
	case strings.HasPrefix(path, "/webhooks/") && r.Method == http.MethodDelete:
		if !requireAnyScope(config.ScopeSettingsWrite, config.ScopeSettingsWrite) {
			return
		}
		h.DeleteWebhook(w, r)
	case path == "/webhook-templates" && r.Method == http.MethodGet:
		if !requireAnyScope(config.ScopeSettingsRead, config.ScopeSettingsRead, config.ScopeSettingsWrite) {
			return
		}
		h.GetWebhookTemplates(w, r)
	case path == "/webhook-history" && r.Method == http.MethodGet:
		if !requireAnyScope(config.ScopeSettingsRead, config.ScopeSettingsRead, config.ScopeSettingsWrite) {
			return
		}
		h.GetWebhookHistory(w, r)
	case path == "/email-providers" && r.Method == http.MethodGet:
		if !requireAnyScope(config.ScopeSettingsRead, config.ScopeSettingsRead, config.ScopeSettingsWrite) {
			return
		}
		h.GetEmailProviders(w, r)
	case path == "/test" && r.Method == http.MethodPost:
		if !requireAnyScope(config.ScopeSettingsWrite, config.ScopeSettingsWrite) {
			return
		}
		h.TestNotification(w, r)
	case path == "/health" && r.Method == http.MethodGet:
		if !requireAnyScope(config.ScopeSettingsRead, config.ScopeSettingsRead, config.ScopeSettingsWrite) {
			return
		}
		h.GetNotificationHealth(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
