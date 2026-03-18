package notifications

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"
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

	// Event context (alert vs resolved)
	Event         string // "alert" or "resolved"
	ResolvedAt    string // RFC3339 timestamp when alert was resolved
	ResolvedAtISO string // Same as ResolvedAt (alias for template consistency)

	// Additional context
	Metadata     map[string]interface{}
	CustomFields map[string]interface{}
	AlertCount   int
	Alerts       []*alerts.Alert // For grouped alerts
	ChatID       string          // For Telegram webhooks
	Mention      string          // Platform-specific mention text
}

func canonicalWebhookConfigForEnhanced(webhook EnhancedWebhookConfig) WebhookConfig {
	canonical := webhook.WebhookConfig
	if strings.TrimSpace(webhook.Service) != "" {
		canonical.Service = webhook.Service
	}
	if strings.TrimSpace(webhook.PayloadTemplate) != "" {
		canonical.Template = webhook.PayloadTemplate
	}
	return canonical
}

func BuildEnhancedWebhookTestConfig(basicWebhook WebhookConfig, requestedService string) EnhancedWebhookConfig {
	webhook := EnhancedWebhookConfig{
		WebhookConfig: basicWebhook,
		Service:       "generic",
		RetryEnabled:  false,
	}

	if len(basicWebhook.CustomFields) > 0 {
		customFields := make(map[string]interface{}, len(basicWebhook.CustomFields))
		for key, value := range basicWebhook.CustomFields {
			customFields[key] = value
		}
		webhook.CustomFields = customFields
	}

	if basicWebhook.Template != "" {
		webhook.PayloadTemplate = basicWebhook.Template
	}

	if strings.TrimSpace(requestedService) != "" {
		webhook.Service = requestedService
		webhook.WebhookConfig.Service = requestedService
	} else if strings.TrimSpace(basicWebhook.Service) != "" {
		webhook.Service = basicWebhook.Service
	}

	if webhook.PayloadTemplate == "" {
		templates := GetWebhookTemplates()
		for _, tmpl := range templates {
			if tmpl.Service != webhook.Service {
				continue
			}
			webhook.PayloadTemplate = tmpl.PayloadTemplate
			if webhook.Headers == nil {
				webhook.Headers = make(map[string]string)
			}
			for k, v := range tmpl.Headers {
				if !strings.Contains(v, "{{") {
					webhook.Headers[k] = v
				}
			}
			break
		}
	}

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

	return webhook
}

func (n *NotificationManager) prepareEnhancedWebhookExecution(webhook EnhancedWebhookConfig, alert *alerts.Alert) (EnhancedWebhookConfig, []byte, error) {
	data := n.prepareWebhookData(alert, webhook.CustomFields)
	canonical := canonicalWebhookConfigForEnhanced(webhook)

	var err error
	canonical, data, err = n.prepareWebhookDeliveryContext(canonical, data)
	if err != nil {
		return webhook, nil, fmt.Errorf("prepare enhanced webhook context: %w", err)
	}
	webhook.WebhookConfig = canonical

	if webhook.Service == "ntfy" && strings.TrimSpace(webhook.PayloadTemplate) != "" {
		tmpl, err := template.New("enhanced-webhook-ntfy").Parse(webhook.PayloadTemplate)
		if err != nil {
			return webhook, nil, fmt.Errorf("parse ntfy payload template: %w", err)
		}
		var rendered bytes.Buffer
		if err := tmpl.Execute(&rendered, data); err != nil {
			return webhook, nil, fmt.Errorf("render ntfy payload template: %w", err)
		}
		return webhook, rendered.Bytes(), nil
	}

	payload, err := n.renderWebhookPayloadJSON(canonical, data, webhookRenderModeSingle, func() ([]byte, error) {
		if strings.TrimSpace(webhook.PayloadTemplate) == "" {
			return nil, fmt.Errorf("enhanced webhook requires payload template or known service template")
		}
		return n.generatePayloadFromTemplateWithService(webhook.PayloadTemplate, data, webhook.Service)
	})
	if err != nil {
		return webhook, nil, fmt.Errorf("render enhanced webhook payload: %w", err)
	}

	return webhook, payload, nil
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

	webhook, payload, err := n.prepareEnhancedWebhookExecution(webhook, alert)
	if err != nil {
		return err
	}

	if !n.checkWebhookRateLimit(webhook.URL) {
		log.Warn().
			Str("webhook", webhook.Name).
			Str("url", webhook.URL).
			Msg("webhook request dropped due to rate limiting")
		return fmt.Errorf("rate limit exceeded for webhook %s", webhook.Name)
	}

	// Send with retry logic
	if webhook.RetryEnabled {
		return n.sendWebhookWithRetry(webhook, payload)
	}

	_, err = n.executeEnhancedWebhookRequest(webhook, payload, WebhookTimeout, "Pulse-Monitoring/2.0")
	if err != nil {
		return fmt.Errorf("send webhook %q once: %w", webhook.Name, err)
	}
	return nil
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
	var lastResp *webhookHTTPResult
	backoff := WebhookInitialBackoff
	retryableErrors := 0
	totalAttempts := 0

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Check for Retry-After header from previous response (overrides backoff)
			if lastResp != nil && lastResp.statusCode == 429 {
				usedBackoff := false
				if retryAfter := lastResp.headers.Get("Retry-After"); retryAfter != "" {
					if customBackoff, ok := parseRetryAfterBackoff(retryAfter, time.Now()); ok {
						log.Debug().
							Str("webhook", webhook.Name).
							Dur("retryAfter", customBackoff).
							Msg("using Retry-After header for backoff")
						time.Sleep(customBackoff)
						backoff = customBackoff // Use this for next iteration
						usedBackoff = true
					} else {
						log.Debug().
							Str("webhook", webhook.Name).
							Str("retryAfter", retryAfter).
							Msg("invalid Retry-After header; falling back to exponential backoff")
					}
				}
				if !usedBackoff {
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

		resp, err := n.executeEnhancedWebhookRequest(webhook, payload, WebhookTimeout, "Pulse-Monitoring/2.0")
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
			statusCode := http.StatusOK
			if resp != nil {
				statusCode = resp.statusCode
			}
			delivery := WebhookDelivery{
				WebhookName:     webhook.Name,
				WebhookURL:      webhook.URL,
				Service:         webhook.Service,
				AlertIdentifier: "enhanced", // Enhanced webhooks are service-level deliveries, not alert-bound.
				Timestamp:       time.Now(),
				StatusCode:      statusCode,
				Success:         true,
				RetryAttempts:   attempt,
				PayloadSize:     len(payload),
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
	statusCode := 0
	if lastResp != nil {
		statusCode = lastResp.statusCode
	}

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
		WebhookName:     webhook.Name,
		WebhookURL:      webhook.URL,
		Service:         webhook.Service,
		AlertIdentifier: "enhanced", // Enhanced webhooks are service-level deliveries, not alert-bound.
		Timestamp:       time.Now(),
		StatusCode:      statusCode,
		Success:         false,
		ErrorMessage:    lastErr.Error(),
		RetryAttempts:   retryAttempts,
		PayloadSize:     len(payload),
	}
	n.addWebhookDelivery(delivery)

	return fmt.Errorf("webhook failed after %d attempts: %w", totalAttempts, lastErr)
}

func parseRetryAfterBackoff(retryAfter string, now time.Time) (time.Duration, bool) {
	retryAfter = strings.TrimSpace(retryAfter)

	clampBackoff := func(backoff time.Duration) time.Duration {
		if backoff < 0 {
			return 0
		}
		if backoff > WebhookMaxBackoff {
			return WebhookMaxBackoff
		}
		return backoff
	}

	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return clampBackoff(time.Duration(seconds) * time.Second), true
	}

	if retryAt, err := http.ParseTime(retryAfter); err == nil {
		return clampBackoff(retryAt.Sub(now)), true
	}

	return 0, false
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
	if strings.Contains(errStr, "status 408") || // Request Timeout
		strings.Contains(errStr, "status 421") || // Misdirected Request
		strings.Contains(errStr, "status 423") || // Locked
		strings.Contains(errStr, "status 425") || // Too Early
		strings.Contains(errStr, "status 429") || // Rate limited
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

// executeEnhancedWebhookRequest sends a single enhanced webhook request and returns response metadata.
func (n *NotificationManager) executeEnhancedWebhookRequest(webhook EnhancedWebhookConfig, payload []byte, timeout time.Duration, userAgent string) (*webhookHTTPResult, error) {
	canonical := canonicalWebhookConfigForEnhanced(webhook)
	return n.executeWebhookRequest(canonical, payload, webhookRequestOptions{
		timeout:         timeout,
		userAgent:       userAgent,
		responseLogging: webhook.ResponseLogging,
		validateURL:     true,
	})
}

// NOTE: formatWebhookDuration is now defined in notifications.go to avoid duplication

// TestEnhancedWebhook tests a webhook with a specific payload
func (n *NotificationManager) TestEnhancedWebhook(webhook EnhancedWebhookConfig) (int, string, error) {
	testAlert := buildNotificationTestAlert()
	testAlert.ID = "test-" + time.Now().Format("20060102-150405")
	if n.publicURL != "" {
		testAlert.Instance = n.publicURL
	}

	var err error
	webhook, payload, err := n.prepareEnhancedWebhookExecution(webhook, testAlert)
	if err != nil {
		return 0, "", err
	}

	if webhook.Service == "ntfy" {
		headers := make(map[string]string, len(webhook.Headers)+4)
		for key, value := range webhook.Headers {
			if !strings.Contains(value, "{{") {
				headers[key] = value
			}
		}
		webhook.Headers = headers
		webhook.Headers["Content-Type"] = "text/plain"

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
		webhook.Headers["Title"] = title

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
		webhook.Headers["Priority"] = priority

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
		webhook.Headers["Tags"] = tags
	}

	result, err := n.executeEnhancedWebhookRequest(webhook, payload, WebhookTestTimeout, "Pulse-Monitoring/2.0 (Test)")
	if err != nil {
		if result != nil {
			return result.statusCode, result.body, err
		}
		return 0, "", err
	}

	return result.statusCode, result.body, nil
}
