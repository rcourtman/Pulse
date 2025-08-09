package notifications

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// EnhancedWebhookConfig extends WebhookConfig with template support
type EnhancedWebhookConfig struct {
	WebhookConfig
	Service          string                 `json:"service"`          // discord, slack, teams, pagerduty, generic
	PayloadTemplate  string                 `json:"payloadTemplate"`  // Go template for payload
	RetryEnabled     bool                   `json:"retryEnabled"`
	RetryCount       int                    `json:"retryCount"`
	FilterRules      WebhookFilterRules     `json:"filterRules"`
	CustomFields     map[string]interface{} `json:"customFields"`     // For template variables
	ResponseLogging  bool                   `json:"responseLogging"`  // Log response for debugging
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
	ID           string
	Level        string
	Type         string
	ResourceName string
	ResourceID   string
	Node         string
	Instance     string
	Message      string
	Value        float64
	Threshold    float64
	StartTime    string
	Duration     string
	Timestamp    string
	
	// Additional context
	CustomFields map[string]interface{}
	AlertCount   int
	Alerts       []*alerts.Alert // For grouped alerts
}

// SendEnhancedWebhook sends a webhook with template support
func (n *NotificationManager) SendEnhancedWebhook(webhook EnhancedWebhookConfig, alert *alerts.Alert) error {
	// Check filters
	if !n.shouldSendWebhook(webhook, alert) {
		log.Debug().
			Str("webhook", webhook.Name).
			Str("alertID", alert.ID).
			Msg("Alert filtered out by webhook rules")
		return nil
	}

	// Prepare template data
	data := n.prepareWebhookData(alert, webhook.CustomFields)
	
	// Generate payload from template
	payload, err := n.generatePayloadFromTemplate(webhook.PayloadTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to generate payload: %w", err)
	}

	// Send with retry logic
	if webhook.RetryEnabled {
		return n.sendWebhookWithRetry(webhook, payload)
	}
	
	return n.sendWebhookOnce(webhook, payload)
}

// prepareWebhookData prepares data for template rendering
// NOTE: This function is now defined in notifications.go to be shared
/*
func (n *NotificationManager) prepareWebhookData(alert *alerts.Alert, customFields map[string]interface{}) WebhookPayloadData {
	duration := time.Since(alert.StartTime)
	
	return WebhookPayloadData{
		ID:           alert.ID,
		Level:        string(alert.Level),
		Type:         alert.Type,
		ResourceName: alert.ResourceName,
		ResourceID:   alert.ResourceID,
		Node:         alert.Node,
		Instance:     alert.Instance,
		Message:      alert.Message,
		Value:        alert.Value,
		Threshold:    alert.Threshold,
		StartTime:    alert.StartTime.Format(time.RFC3339),
		Duration:     formatWebhookDuration(duration),
		Timestamp:    time.Now().Format(time.RFC3339),
		CustomFields: customFields,
		AlertCount:   1,
	}
}
*/

// generatePayloadFromTemplate renders the payload using Go templates
// NOTE: This function is now defined in notifications.go to be shared
/*
func (n *NotificationManager) generatePayloadFromTemplate(templateStr string, data WebhookPayloadData) ([]byte, error) {
	// Create template with helper functions
	funcMap := template.FuncMap{
		"title": strings.Title,
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"printf": fmt.Sprintf,
	}
	
	tmpl, err := template.New("webhook").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	// Validate JSON
	var jsonCheck interface{}
	if err := json.Unmarshal(buf.Bytes(), &jsonCheck); err != nil {
		return nil, fmt.Errorf("template produced invalid JSON: %w", err)
	}

	return buf.Bytes(), nil
}
*/

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

// sendWebhookWithRetry implements exponential backoff retry
func (n *NotificationManager) sendWebhookWithRetry(webhook EnhancedWebhookConfig, payload []byte) error {
	maxRetries := webhook.RetryCount
	if maxRetries <= 0 {
		maxRetries = 3
	}
	
	var lastErr error
	backoff := time.Second
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Debug().
				Str("webhook", webhook.Name).
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Msg("Retrying webhook after backoff")
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
		
		err := n.sendWebhookOnce(webhook, payload)
		if err == nil {
			if attempt > 0 {
				log.Info().
					Str("webhook", webhook.Name).
					Int("attempt", attempt).
					Msg("Webhook succeeded after retry")
			}
			return nil
		}
		
		lastErr = err
		log.Warn().
			Err(err).
			Str("webhook", webhook.Name).
			Int("attempt", attempt).
			Msg("Webhook attempt failed")
	}
	
	return fmt.Errorf("webhook failed after %d attempts: %w", maxRetries+1, lastErr)
}

// sendWebhookOnce sends a single webhook request
func (n *NotificationManager) sendWebhookOnce(webhook EnhancedWebhookConfig, payload []byte) error {
	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "Pulse-Monitoring/2.0")

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Log response if enabled
	if webhook.ResponseLogging {
		var respBody bytes.Buffer
		respBody.ReadFrom(resp.Body)
		log.Debug().
			Str("webhook", webhook.Name).
			Int("status", resp.StatusCode).
			Str("response", respBody.String()).
			Msg("Webhook response")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// formatWebhookDuration formats a duration in a human-readable way
// NOTE: This function is now defined in notifications.go to be shared
/*
func formatWebhookDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}
*/

// TestEnhancedWebhook tests a webhook with a specific payload
func (n *NotificationManager) TestEnhancedWebhook(webhook EnhancedWebhookConfig) (int, string, error) {
	// Create test alert
	testAlert := &alerts.Alert{
		ID:           "test-" + time.Now().Format("20060102-150405"),
		Type:         "cpu",
		Level:        "warning",
		ResourceID:   "100",
		ResourceName: "Test VM",
		Node:         "pve-node-01",
		Instance:     "https://192.168.1.100:8006",
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
	
	// Generate payload
	payload, err := n.generatePayloadFromTemplate(webhook.PayloadTemplate, data)
	if err != nil {
		return 0, "", fmt.Errorf("failed to generate payload: %w", err)
	}

	// Send request
	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		return 0, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "Pulse-Monitoring/2.0 (Test)")

	// Send with shorter timeout for testing
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	var respBody bytes.Buffer
	respBody.ReadFrom(resp.Body)

	return resp.StatusCode, respBody.String(), nil
}