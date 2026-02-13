package notifications

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// EnhancedWebhookConfig extends WebhookConfig with template support
type EnhancedWebhookConfig struct {
	WebhookConfig
	Service         string                 `json:"service"`         // discord, slack, teams, pagerduty, generic
	PayloadTemplate string                 `json:"payloadTemplate"` // Go template for payload
	RetryEnabled    bool                   `json:"retryEnabled"`
	RetryCount      int                    `json:"retryCount"`
	FilterRules     WebhookFilterRules     `json:"filterRules"`
	CustomFields    map[string]interface{} `json:"customFields"`    // For template variables
	ResponseLogging bool                   `json:"responseLogging"` // Log response for debugging
}

// WebhookFilterRules defines filtering for this webhook
type WebhookFilterRules struct {
	Levels        []string `json:"levels"`        // Only send these levels
	Types         []string `json:"types"`         // Only send these alert types
	Nodes         []string `json:"nodes"`         // Only send from these nodes
	ResourceTypes []string `json:"resourceTypes"` // vm, container, storage, etc
}

// WebhookPayloadData contains all data available for templates
type WebhookPayloadData struct {
	// Alert fields
	ID                 string
	Level              string
	Type               string
	ResourceName       string
	ResourceID         string
	Node               string
	NodeDisplayName    string
	Instance           string
	Message            string
	Value              float64
	Threshold          float64
	ValueFormatted     string
	ThresholdFormatted string
	StartTime          string
	Duration           string
	Timestamp          string
	ResourceType       string
	Acknowledged       bool
	AckTime            string
	AckUser            string

	// Additional context
	Metadata     map[string]interface{}
	CustomFields map[string]interface{}
	AlertCount   int
	Alerts       []*alerts.Alert // For grouped alerts
	ChatID       string          // For Telegram webhooks
	Mention      string          // Platform-specific mention text
}

// SendEnhancedWebhook sends a webhook with template support
func (n *NotificationManager) SendEnhancedWebhook(webhook EnhancedWebhookConfig, alert *alerts.Alert) error {
	// Check filters
	if !n.shouldSendWebhook(webhook, alert) {
		log.Debug().
			Str("webhook", webhook.Name).
			Str("alertID", alert.ID).
			Msg("alert filtered out by webhook rules")
		return nil
	}

	// Prepare template data
	data := n.prepareWebhookData(alert, webhook.CustomFields)

	// Render URL template when placeholders are present
	renderedURL, renderErr := renderWebhookURL(webhook.URL, data)
	if renderErr != nil {
		return fmt.Errorf("failed to render webhook URL template: %w", renderErr)
	}
	webhook.URL = renderedURL

	// Validate webhook URL to prevent SSRF/DNS rebinding attacks
	if err := n.ValidateWebhookURL(webhook.URL); err != nil {
		return fmt.Errorf("webhook URL validation failed: %w", err)
	}

	// Check rate limit
	if !n.checkWebhookRateLimit(webhook.URL) {
		log.Warn().
			Str("webhook", webhook.Name).
			Str("url", webhook.URL).
			Msg("webhook request dropped due to rate limiting")
		return fmt.Errorf("rate limit exceeded for webhook %s", webhook.Name)
	}

	// Service-specific enrichment
	switch webhook.Service {
	case "telegram":
		chatID, chatErr := extractTelegramChatID(webhook.URL)
		if chatErr != nil {
			return fmt.Errorf("failed to extract Telegram chat_id: %w", chatErr)
		}
		if chatID != "" {
			data.ChatID = chatID
			log.Debug().
				Str("webhook", webhook.Name).
				Str("chatID", chatID).
				Msg("extracted Telegram chat_id from rendered URL for enhanced webhook")
		}
	case "pagerduty":
		if data.CustomFields == nil {
			data.CustomFields = make(map[string]interface{})
		}
		if routingKey, ok := webhook.Headers["routing_key"]; ok {
			data.CustomFields["routing_key"] = routingKey
		}
	}

	// Generate payload from template with service-specific handling
	payload, err := n.generatePayloadFromTemplateWithService(webhook.PayloadTemplate, data, webhook.Service)
	if err != nil {
		return fmt.Errorf("failed to generate payload: %w", err)
	}

	// Send with retry logic
	if webhook.RetryEnabled {
		return n.sendWebhookWithRetry(webhook, payload)
	}

	return n.sendWebhookOnce(webhook, payload)
}

// NOTE: prepareWebhookData is now defined in notifications.go to avoid duplication
// NOTE: generatePayloadFromTemplate is now defined in notifications.go to avoid duplication

// shouldSendWebhook checks if alert matches webhook filter rules
func (n *NotificationManager) shouldSendWebhook(webhook EnhancedWebhookConfig, alert *alerts.Alert) bool {
	rules := webhook.FilterRules

	// Check level filter
	if len(rules.Levels) > 0 {
		found := false
		for _, level := range rules.Levels {
			if string(alert.Level) == level {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check type filter
	if len(rules.Types) > 0 {
		found := false
		for _, alertType := range rules.Types {
			if alert.Type == alertType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check node filter
	if len(rules.Nodes) > 0 {
		found := false
		for _, node := range rules.Nodes {
			if alert.Node == node {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check resource type filter
	if len(rules.ResourceTypes) > 0 {
		resourceType, ok := alert.Metadata["resourceType"].(string)
		if !ok {
			return false
		}
		found := false
		for _, rt := range rules.ResourceTypes {
			if resourceType == rt {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// sendWebhookWithRetry implements exponential backoff retry with enhanced error tracking
// Note: When used with the persistent queue, retry behavior is layered:
// - Transport retries (this function): up to RetryCount attempts with exponential backoff
// - Queue retries: up to MaxAttempts (default 3) with exponential backoff
// Total attempts = RetryCount * MaxAttempts (e.g., 3 * 3 = 9 HTTP calls for a single notification)
// This ensures delivery even during transient failures at either layer.
func (n *NotificationManager) sendWebhookWithRetry(webhook EnhancedWebhookConfig, payload []byte) error {
	maxRetries := webhook.RetryCount
	if maxRetries <= 0 {
		maxRetries = WebhookDefaultRetries
	}

	var lastErr error
	var lastResp *http.Response
	backoff := WebhookInitialBackoff
	retryableErrors := 0
	totalAttempts := 0

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Check for Retry-After header from previous response (overrides backoff)
			if lastResp != nil && lastResp.StatusCode == 429 {
				if retryAfter := lastResp.Header.Get("Retry-After"); retryAfter != "" {
					if seconds, err := strconv.Atoi(retryAfter); err == nil {
						customBackoff := time.Duration(seconds) * time.Second
						log.Debug().
							Str("webhook", webhook.Name).
							Dur("retryAfter", customBackoff).
							Msg("using Retry-After header for backoff")
						time.Sleep(customBackoff)
						backoff = customBackoff // Use this for next iteration
					}
				} else {
					// No Retry-After, use exponential backoff
					log.Debug().
						Str("webhook", webhook.Name).
						Int("attempt", attempt).
						Int("maxRetries", maxRetries).
						Dur("backoff", backoff).
						Msg("retrying webhook after backoff")
					time.Sleep(backoff)
				}
			} else {
				// Not a 429, use exponential backoff
				log.Debug().
					Str("webhook", webhook.Name).
					Int("attempt", attempt).
					Int("maxRetries", maxRetries).
					Dur("backoff", backoff).
					Msg("retrying webhook after backoff")
				time.Sleep(backoff)
			}

			// Exponential backoff for next iteration
			backoff *= 2
			if backoff > WebhookMaxBackoff {
				backoff = WebhookMaxBackoff
			}
		}

		resp, err := n.sendWebhookOnceWithResponse(webhook, payload)
		totalAttempts = attempt + 1
		lastResp = resp
		if err == nil {
			if attempt > 0 {
				log.Info().
					Str("webhook", webhook.Name).
					Int("attempt", attempt).
					Int("totalAttempts", attempt+1).
					Msg("webhook succeeded after retry")
			}
			// Log successful delivery
			log.Debug().
				Str("webhook", webhook.Name).
				Str("service", webhook.Service).
				Int("payloadSize", len(payload)).
				Msg("webhook delivered successfully")

			// Track successful delivery
			delivery := WebhookDelivery{
				WebhookName:   webhook.Name,
				WebhookURL:    webhook.URL,
				Service:       webhook.Service,
				AlertID:       "enhanced", // This is for enhanced webhooks, alertID might not be available
				Timestamp:     time.Now(),
				StatusCode:    200, // Assume success
				Success:       true,
				RetryAttempts: attempt,
				PayloadSize:   len(payload),
			}
			n.addWebhookDelivery(delivery)

			return nil
		}

		lastErr = err

		// Determine if error is retryable
		isRetryable := isRetryableWebhookError(err)
		if isRetryable {
			retryableErrors++
		}

		log.Warn().
			Err(err).
			Str("webhook", webhook.Name).
			Str("service", webhook.Service).
			Int("attempt", attempt+1).
			Int("maxRetries", maxRetries+1).
			Bool("retryable", isRetryable).
			Msg("webhook attempt failed")

		// If error is not retryable, stop immediately regardless of attempt.
		if !isRetryable {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Msg("non-retryable webhook error - not attempting retry")
			break
		}
	}

	if totalAttempts == 0 {
		totalAttempts = maxRetries + 1
	}
	retryAttempts := totalAttempts - 1

	// Final error logging with summary
	log.Error().
		Err(lastErr).
		Str("webhook", webhook.Name).
		Str("service", webhook.Service).
		Int("totalAttempts", totalAttempts).
		Int("retryableErrors", retryableErrors).
		Msg("webhook delivery failed after all retry attempts")

	// Track failed delivery
	delivery := WebhookDelivery{
		WebhookName:   webhook.Name,
		WebhookURL:    webhook.URL,
		Service:       webhook.Service,
		AlertID:       "enhanced", // This is for enhanced webhooks, alertID might not be available
		Timestamp:     time.Now(),
		StatusCode:    0, // Unknown status
		Success:       false,
		ErrorMessage:  lastErr.Error(),
		RetryAttempts: retryAttempts,
		PayloadSize:   len(payload),
	}
	n.addWebhookDelivery(delivery)

	return fmt.Errorf("webhook failed after %d attempts: %w", totalAttempts, lastErr)
}

// isRetryableWebhookError determines if a webhook error should trigger a retry
func isRetryableWebhookError(err error) bool {
	errStr := strings.ToLower(err.Error())

	// Network-related errors that should be retried
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network unreachable") {
		return true
	}

	// HTTP status codes that should be retried
	if strings.Contains(errStr, "status 429") || // Rate limited
		strings.Contains(errStr, "status 502") || // Bad Gateway
		strings.Contains(errStr, "status 503") || // Service Unavailable
		strings.Contains(errStr, "status 504") { // Gateway Timeout
		return true
	}

	// 5xx server errors are generally retryable
	for i := 500; i <= 599; i++ {
		if strings.Contains(errStr, fmt.Sprintf("status %d", i)) {
			return true
		}
	}

	// 4xx client errors are generally not retryable
	for i := 400; i <= 499; i++ {
		if strings.Contains(errStr, fmt.Sprintf("status %d", i)) {
			return false
		}
	}

	// Default to retryable for unknown errors
	return true
}

// sendWebhookOnceWithResponse sends a single webhook request and returns the response
func (n *NotificationManager) sendWebhookOnceWithResponse(webhook EnhancedWebhookConfig, payload []byte) (*http.Response, error) {
	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "Pulse-Monitoring/2.0")

	// Send request with secure client
	client := n.createSecureWebhookClient(WebhookTimeout)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, WebhookMaxResponseSize)
	var respBody bytes.Buffer
	bytesRead, err := respBody.ReadFrom(limitedReader)
	if err != nil {
		return resp, fmt.Errorf("failed to read webhook response body: %w", err)
	}

	if bytesRead >= WebhookMaxResponseSize {
		log.Warn().
			Str("webhook", webhook.Name).
			Int64("bytesRead", bytesRead).
			Msg("webhook response exceeded size limit")
	}

	responseBody := respBody.String()

	// Log response if enabled or if there's an error
	if webhook.ResponseLogging || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Debug().
			Str("webhook", webhook.Name).
			Int("status", resp.StatusCode).
			Str("response", responseBody).
			Msg("webhook response")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, responseBody)
	}

	return resp, nil
}

// sendWebhookOnce sends a single webhook request (compatibility wrapper)
func (n *NotificationManager) sendWebhookOnce(webhook EnhancedWebhookConfig, payload []byte) error {
	_, err := n.sendWebhookOnceWithResponse(webhook, payload)
	if err != nil {
		return fmt.Errorf("send webhook %q once: %w", webhook.Name, err)
	}
	return nil
}

// NOTE: formatWebhookDuration is now defined in notifications.go to avoid duplication

// TestEnhancedWebhook tests a webhook with a specific payload
func (n *NotificationManager) TestEnhancedWebhook(webhook EnhancedWebhookConfig) (int, string, error) {
	// Use the configured publicURL if available, otherwise use a placeholder
	instanceURL := n.publicURL
	if instanceURL == "" {
		instanceURL = "https://192.168.1.100:8006"
	}

	// Create test alert
	testAlert := &alerts.Alert{
		ID:           "test-" + time.Now().Format("20060102-150405"),
		Type:         "cpu",
		Level:        "warning",
		ResourceID:   "100",
		ResourceName: "Test VM",
		Node:         "pve-node-01",
		Instance:     instanceURL,
		Message:      "Test webhook notification from Pulse Monitoring",
		Value:        85.5,
		Threshold:    80.0,
		StartTime:    time.Now().Add(-2 * time.Minute),
		LastSeen:     time.Now(),
		Metadata: map[string]interface{}{
			"resourceType": "vm",
		},
	}

	// Prepare data
	data := n.prepareWebhookData(testAlert, webhook.CustomFields)

	// Render webhook URL using template data
	renderedURL, renderErr := renderWebhookURL(webhook.URL, data)
	if renderErr != nil {
		return 0, "", fmt.Errorf("failed to render webhook URL template: %w", renderErr)
	}
	webhook.URL = renderedURL

	// Validate webhook URL to prevent SSRF/DNS rebinding attacks (same validation as live sends)
	if err := n.ValidateWebhookURL(webhook.URL); err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Str("url", webhook.URL).
			Msg("webhook URL validation failed for test request")
		return 0, "", fmt.Errorf("webhook URL validation failed: %w", err)
	}

	// For Telegram, extract chat_id from URL if present
	if webhook.Service == "telegram" {
		if chatID, err := extractTelegramChatID(webhook.URL); err == nil && chatID != "" {
			data.ChatID = chatID
		} else if err != nil {
			log.Warn().
				Err(err).
				Str("webhook", webhook.Name).
				Msg("failed to extract Telegram chat_id during enhanced webhook test")
		}
		// Note: For test webhooks, we don't fail if chat_id is missing
		// as this may be intentional during testing
	} else if webhook.Service == "pagerduty" {
		if data.CustomFields == nil {
			data.CustomFields = make(map[string]interface{})
		}
		if routingKey, ok := webhook.Headers["routing_key"]; ok {
			data.CustomFields["routing_key"] = routingKey
		}
	}

	// Generate payload with service-specific handling
	payload, err := n.generatePayloadFromTemplateWithService(webhook.PayloadTemplate, data, webhook.Service)
	if err != nil {
		return 0, "", fmt.Errorf("failed to generate payload: %w", err)
	}

	// Send request
	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	// For Telegram webhooks, strip chat_id from URL if present
	webhookURL := webhook.URL
	if webhook.Service == "telegram" && strings.Contains(webhookURL, "chat_id=") {
		if u, err := url.Parse(webhookURL); err == nil {
			q := u.Query()
			q.Del("chat_id") // Remove chat_id from query params
			u.RawQuery = q.Encode()
			webhookURL = u.String()
		}
	}

	req, err := http.NewRequest(method, webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return 0, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	// Special handling for ntfy service - add dynamic headers based on test alert level
	if webhook.Service == "ntfy" {
		// Set Content-Type for ntfy (plain text)
		req.Header.Set("Content-Type", "text/plain")

		// Set dynamic headers based on alert level
		title := fmt.Sprintf("%s: %s",
			func() string {
				switch testAlert.Level {
				case alerts.AlertLevelCritical:
					return "CRITICAL"
				case alerts.AlertLevelWarning:
					return "WARNING"
				default:
					return "INFO"
				}
			}(),
			testAlert.ResourceName,
		)
		req.Header.Set("Title", title)

		priority := func() string {
			switch testAlert.Level {
			case alerts.AlertLevelCritical:
				return "urgent"
			case alerts.AlertLevelWarning:
				return "high"
			default:
				return "default"
			}
		}()
		req.Header.Set("Priority", priority)

		tags := fmt.Sprintf("%s,pulse,%s",
			func() string {
				switch testAlert.Level {
				case alerts.AlertLevelCritical:
					return "rotating_light"
				case alerts.AlertLevelWarning:
					return "warning"
				default:
					return "white_check_mark"
				}
			}(),
			testAlert.Type,
		)
		req.Header.Set("Tags", tags)
	}

	// Apply any custom headers from webhook config (these override defaults)
	for key, value := range webhook.Headers {
		// Skip template-like headers (those with {{)
		if !strings.Contains(value, "{{") {
			req.Header.Set(key, value)
		}
	}

	if webhook.Service != "ntfy" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "Pulse-Monitoring/2.0 (Test)")

	// Send with shorter timeout for testing
	client := n.createSecureWebhookClient(WebhookTestTimeout)

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response with size limit
	limitedReader := io.LimitReader(resp.Body, WebhookMaxResponseSize)
	var respBody bytes.Buffer
	if _, err := respBody.ReadFrom(limitedReader); err != nil {
		return 0, "", fmt.Errorf("failed to read webhook response body: %w", err)
	}

	return resp.StatusCode, respBody.String(), nil
}
