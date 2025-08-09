package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// TestNodeInfo contains information about nodes for test notifications
type TestNodeInfo struct {
	NodeName    string
	InstanceURL string
}

// NotificationManager handles sending notifications
type NotificationManager struct {
	mu           sync.RWMutex
	emailConfig  EmailConfig
	webhooks     []WebhookConfig
	enabled      bool
	cooldown     time.Duration
	lastNotified map[string]time.Time
	groupWindow  time.Duration
	pendingAlerts []*alerts.Alert
	groupTimer   *time.Timer
	groupByNode  bool
	groupByGuest bool
}

// Alert represents an alert (interface to avoid circular dependency)
type Alert interface {
	GetID() string
	GetResourceName() string
	GetType() string
	GetLevel() string
	GetValue() float64
	GetThreshold() float64
	GetMessage() string
	GetNode() string
	GetInstance() string
	GetStartTime() time.Time
}

// EmailConfig holds email notification settings
type EmailConfig struct {
	Enabled    bool     `json:"enabled"`
	SMTPHost   string   `json:"smtpHost"`
	SMTPPort   int      `json:"smtpPort"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	From       string   `json:"from"`
	To         []string `json:"to"`
	TLS        bool     `json:"tls"`
}

// WebhookConfig holds webhook settings
type WebhookConfig struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Enabled bool              `json:"enabled"`
	Service string            `json:"service"` // discord, slack, teams, etc.
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		enabled:      true,
		cooldown:     5 * time.Minute,
		lastNotified: make(map[string]time.Time),
		webhooks:     []WebhookConfig{},
		groupWindow:  30 * time.Second,
		pendingAlerts: make([]*alerts.Alert, 0),
		groupByNode:  true,
		groupByGuest: false,
	}
}

// SetEmailConfig updates email configuration
func (n *NotificationManager) SetEmailConfig(config EmailConfig) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.emailConfig = config
}

// SetCooldown updates the cooldown duration
func (n *NotificationManager) SetCooldown(minutes int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.cooldown = time.Duration(minutes) * time.Minute
	log.Info().Int("minutes", minutes).Msg("Updated notification cooldown")
}

// SetGroupingWindow updates the grouping window duration
func (n *NotificationManager) SetGroupingWindow(seconds int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.groupWindow = time.Duration(seconds) * time.Second
	log.Info().Int("seconds", seconds).Msg("Updated notification grouping window")
}

// SetGroupingOptions updates grouping options
func (n *NotificationManager) SetGroupingOptions(byNode, byGuest bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.groupByNode = byNode
	n.groupByGuest = byGuest
	log.Info().Bool("byNode", byNode).Bool("byGuest", byGuest).Msg("Updated notification grouping options")
}

// AddWebhook adds a webhook configuration
func (n *NotificationManager) AddWebhook(webhook WebhookConfig) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.webhooks = append(n.webhooks, webhook)
}

// UpdateWebhook updates an existing webhook
func (n *NotificationManager) UpdateWebhook(id string, webhook WebhookConfig) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	for i, w := range n.webhooks {
		if w.ID == id {
			n.webhooks[i] = webhook
			return nil
		}
	}
	return fmt.Errorf("webhook not found: %s", id)
}

// DeleteWebhook removes a webhook
func (n *NotificationManager) DeleteWebhook(id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	for i, w := range n.webhooks {
		if w.ID == id {
			n.webhooks = append(n.webhooks[:i], n.webhooks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("webhook not found: %s", id)
}

// GetWebhooks returns all webhook configurations
func (n *NotificationManager) GetWebhooks() []WebhookConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	
	webhooks := make([]WebhookConfig, len(n.webhooks))
	copy(webhooks, n.webhooks)
	return webhooks
}

// GetEmailConfig returns the email configuration
func (n *NotificationManager) GetEmailConfig() EmailConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.emailConfig
}

// SendAlert sends notifications for an alert
func (n *NotificationManager) SendAlert(alert *alerts.Alert) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	if !n.enabled {
		return
	}
	
	// Check cooldown
	lastTime, exists := n.lastNotified[alert.ID]
	if exists && time.Since(lastTime) < n.cooldown {
		log.Debug().
			Str("alertID", alert.ID).
			Dur("timeSince", time.Since(lastTime)).
			Msg("Alert notification in cooldown")
		return
	}
	
	// Add to pending alerts for grouping
	n.pendingAlerts = append(n.pendingAlerts, alert)
	
	// If this is the first alert in the group, start the timer
	if n.groupTimer == nil {
		n.groupTimer = time.AfterFunc(n.groupWindow, func() {
			n.sendGroupedAlerts()
		})
		log.Debug().
			Int("pendingCount", len(n.pendingAlerts)).
			Dur("groupWindow", n.groupWindow).
			Msg("Started alert grouping timer")
	}
}

// sendGroupedAlerts sends all pending alerts as a group
func (n *NotificationManager) sendGroupedAlerts() {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	if len(n.pendingAlerts) == 0 {
		return
	}
	
	// Copy alerts to send
	alertsToSend := make([]*alerts.Alert, len(n.pendingAlerts))
	copy(alertsToSend, n.pendingAlerts)
	
	// Clear pending alerts
	n.pendingAlerts = n.pendingAlerts[:0]
	n.groupTimer = nil
	
	log.Info().
		Int("alertCount", len(alertsToSend)).
		Msg("Sending grouped alert notifications")
	
	// Send notifications
	if n.emailConfig.Enabled {
		go n.sendGroupedEmail(alertsToSend)
	}
	
	webhooks := make([]WebhookConfig, len(n.webhooks))
	copy(webhooks, n.webhooks)
	
	for _, webhook := range webhooks {
		if webhook.Enabled {
			go n.sendGroupedWebhook(webhook, alertsToSend)
		}
	}
	
	// Update last notified time for all alerts
	now := time.Now()
	for _, alert := range alertsToSend {
		n.lastNotified[alert.ID] = now
	}
}

// sendGroupedEmail sends a grouped email notification
func (n *NotificationManager) sendGroupedEmail(alertList []*alerts.Alert) {
	config := n.emailConfig
	
	// Don't check for recipients here - sendHTMLEmail handles empty recipients
	// by using the From address as the recipient
	
	// Generate email using template
	subject, htmlBody, textBody := EmailTemplate(alertList, false)
	
	// Send using HTML-aware method
	n.sendHTMLEmail(subject, htmlBody, textBody, config)
}

// sendEmail sends an email notification
func (n *NotificationManager) sendEmail(alert *alerts.Alert) {
	n.mu.RLock()
	config := n.emailConfig
	n.mu.RUnlock()
	
	// Don't check for recipients here - sendHTMLEmail handles empty recipients
	// by using the From address as the recipient
	
	// Generate email using template
	subject, htmlBody, textBody := EmailTemplate([]*alerts.Alert{alert}, true)
	
	// Send using HTML-aware method
	n.sendHTMLEmail(subject, htmlBody, textBody, config)
}

// sendHTMLEmail sends an HTML email with multipart content
func (n *NotificationManager) sendHTMLEmail(subject, htmlBody, textBody string, config EmailConfig) {
	// Use From address as recipient if To is empty
	recipients := config.To
	if len(recipients) == 0 && config.From != "" {
		recipients = []string{config.From}
		log.Info().
			Str("from", config.From).
			Msg("Using From address as recipient since To is empty")
	}
	
	// Create enhanced email configuration with proper STARTTLS support
	enhancedConfig := EmailProviderConfig{
		EmailConfig: EmailConfig{
			From:     config.From,
			To:       recipients,
			SMTPHost: config.SMTPHost,
			SMTPPort: config.SMTPPort,
			Username: config.Username,
			Password: config.Password,
		},
		StartTLS:      config.SMTPPort == 587 || config.SMTPPort == 25, // Use STARTTLS for common ports
		MaxRetries:    2,
		RetryDelay:    3,
		RateLimit:     60,
		SkipTLSVerify: false,
		AuthRequired:  config.Username != "" && config.Password != "",
	}
	
	// Use enhanced email manager for better compatibility
	enhancedManager := NewEnhancedEmailManager(enhancedConfig)
	
	log.Info().
		Str("smtp", fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)).
		Str("from", config.From).
		Strs("to", recipients).
		Bool("hasAuth", config.Username != "" && config.Password != "").
		Bool("startTLS", enhancedConfig.StartTLS).
		Msg("Attempting to send email via SMTP with enhanced support")
	
	err := enhancedManager.SendEmailWithRetry(subject, htmlBody, textBody)
	
	if err != nil {
		log.Error().
			Err(err).
			Str("smtp", fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)).
			Strs("recipients", recipients).
			Msg("Failed to send email notification")
	} else {
		log.Info().
			Strs("recipients", recipients).
			Int("recipientCount", len(recipients)).
			Msg("Email notification sent successfully")
	}
}

// sendEmailWithContent sends email with given content (plain text)
func (n *NotificationManager) sendEmailWithContent(subject, body string, config EmailConfig) {
	// For backward compatibility, send as plain text
	n.sendHTMLEmail(subject, "", body, config)
}

// sendGroupedWebhook sends a grouped webhook notification
func (n *NotificationManager) sendGroupedWebhook(webhook WebhookConfig, alertList []*alerts.Alert) {
	var jsonData []byte
	var err error
	
	// For Discord, send individual embeds for each alert
	if webhook.Service == "discord" && len(alertList) > 0 {
		// For simplicity, send the first alert with a note about others
		// Discord webhooks work better with single embeds
		alert := alertList[0]
		
		// Convert to enhanced webhook to use template
		enhanced := EnhancedWebhookConfig{
			WebhookConfig: webhook,
			Service:       "discord",
		}
		
		// Get Discord template
		templates := GetWebhookTemplates()
		for _, tmpl := range templates {
			if tmpl.Service == "discord" {
				enhanced.PayloadTemplate = tmpl.PayloadTemplate
				break
			}
		}
		
		// Modify message if multiple alerts
		if len(alertList) > 1 {
			alert.Message = fmt.Sprintf("%s (and %d more alerts)", alert.Message, len(alertList)-1)
		}
		
		// Prepare data and generate payload
		data := n.prepareWebhookData(alert, nil)
		jsonData, err = n.generatePayloadFromTemplate(enhanced.PayloadTemplate, data)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Int("alertCount", len(alertList)).
				Msg("Failed to generate Discord payload for grouped alerts")
			return
		}
	} else {
		// Use generic payload for other services
		payload := map[string]interface{}{
			"alerts": alertList,
			"count": len(alertList),
			"timestamp": time.Now().Unix(),
			"source": "pulse-monitoring",
			"grouped": true,
		}
		
		jsonData, err = json.Marshal(payload)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Int("alertCount", len(alertList)).
				Msg("Failed to marshal grouped webhook payload")
			return
		}
	}
	
	// Send using same request logic
	n.sendWebhookRequest(webhook, jsonData, "grouped")
}

// sendWebhookRequest sends the actual webhook request
func (n *NotificationManager) sendWebhookRequest(webhook WebhookConfig, jsonData []byte, alertType string) {
	// Create request
	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, webhook.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Str("type", alertType).
			Msg("Failed to create webhook request")
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Pulse-Monitoring/2.0")
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Str("type", alertType).
			Msg("Failed to send webhook")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Info().
			Str("webhook", webhook.Name).
			Str("type", alertType).
			Int("status", resp.StatusCode).
			Msg("Webhook notification sent")
	} else {
		log.Warn().
			Str("webhook", webhook.Name).
			Str("type", alertType).
			Int("status", resp.StatusCode).
			Msg("Webhook returned non-success status")
	}
}

// sendWebhook sends a webhook notification
func (n *NotificationManager) sendWebhook(webhook WebhookConfig, alert *alerts.Alert) {
	var jsonData []byte
	var err error
	
	// Check if this is a Discord webhook and use the proper template
	if webhook.Service == "discord" {
		// Convert to enhanced webhook to use template
		enhanced := EnhancedWebhookConfig{
			WebhookConfig: webhook,
			Service:       "discord",
		}
		
		// Get Discord template
		templates := GetWebhookTemplates()
		for _, tmpl := range templates {
			if tmpl.Service == "discord" {
				enhanced.PayloadTemplate = tmpl.PayloadTemplate
				break
			}
		}
		
		// Prepare data and generate payload
		data := n.prepareWebhookData(alert, nil)
		jsonData, err = n.generatePayloadFromTemplate(enhanced.PayloadTemplate, data)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Str("alertID", alert.ID).
				Msg("Failed to generate Discord payload")
			return
		}
	} else {
		// Use generic payload for other services
		payload := map[string]interface{}{
			"alert": alert,
			"timestamp": time.Now().Unix(),
			"source": "pulse-monitoring",
		}
		
		jsonData, err = json.Marshal(payload)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Str("alertID", alert.ID).
				Msg("Failed to marshal webhook payload")
			return
		}
	}
	
	// Send using common request logic
	n.sendWebhookRequest(webhook, jsonData, fmt.Sprintf("alert-%s", alert.ID))
}

// prepareWebhookData prepares data for template rendering
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

// generatePayloadFromTemplate renders the payload using Go templates
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

	return buf.Bytes(), nil
}

// formatWebhookDuration formats a duration in a human-readable way
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

// groupAlerts groups alerts based on configuration
func (n *NotificationManager) groupAlerts(alertList []*alerts.Alert) map[string][]*alerts.Alert {
	groups := make(map[string][]*alerts.Alert)
	
	if !n.groupByNode && !n.groupByGuest {
		// No grouping - all alerts in one group
		groups["all"] = alertList
		return groups
	}
	
	for _, alert := range alertList {
		var key string
		
		if n.groupByNode && n.groupByGuest {
			// Group by both node and guest type
			guestType := "unknown"
			if metadata, ok := alert.Metadata["resourceType"].(string); ok {
				guestType = metadata
			}
			key = fmt.Sprintf("%s-%s", alert.Node, guestType)
		} else if n.groupByNode {
			// Group by node only
			key = alert.Node
		} else if n.groupByGuest {
			// Group by guest type only
			if metadata, ok := alert.Metadata["resourceType"].(string); ok {
				key = metadata
			} else {
				key = "unknown"
			}
		}
		
		groups[key] = append(groups[key], alert)
	}
	
	return groups
}

// SendTestNotification sends a test notification
func (n *NotificationManager) SendTestNotification(method string) error {
	testAlert := &alerts.Alert{
		ID:           "test-alert",
		Type:         "cpu",
		Level:        "warning",
		ResourceID:   "test-resource",
		ResourceName: "Test Resource",
		Node:         "pve-node-01",
		Instance:     "https://192.168.1.100:8006",
		Message:      "This is a test alert from Pulse Monitoring to verify your notification settings are working correctly",
		Value:        95.5,
		Threshold:    90,
		StartTime:    time.Now().Add(-5 * time.Minute), // Show it's been active for 5 minutes
		LastSeen:     time.Now(),
		Metadata: map[string]interface{}{
			"resourceType": "vm",
		},
	}
	
	switch method {
	case "email":
		log.Info().
			Bool("enabled", n.emailConfig.Enabled).
			Str("smtp", n.emailConfig.SMTPHost).
			Int("port", n.emailConfig.SMTPPort).
			Str("from", n.emailConfig.From).
			Int("toCount", len(n.emailConfig.To)).
			Msg("Testing email notification")
		if !n.emailConfig.Enabled {
			return fmt.Errorf("email notifications are not enabled")
		}
		n.sendEmail(testAlert)
		return nil
	case "webhook":
		n.mu.RLock()
		if len(n.webhooks) == 0 {
			n.mu.RUnlock()
			return fmt.Errorf("no webhooks configured")
		}
		// Send to first enabled webhook
		for _, webhook := range n.webhooks {
			if webhook.Enabled {
				n.mu.RUnlock()
				n.sendWebhook(webhook, testAlert)
				return nil
			}
		}
		n.mu.RUnlock()
		return fmt.Errorf("no enabled webhooks found")
	default:
		return fmt.Errorf("unknown notification method: %s", method)
	}
}

// SendTestNotificationWithConfig sends a test notification using provided config
func (n *NotificationManager) SendTestNotificationWithConfig(method string, config *EmailConfig, nodeInfo *TestNodeInfo) error {
	// Use actual node info if provided, otherwise use defaults
	nodeName := "test-node"
	instanceURL := "https://proxmox.local:8006"
	if nodeInfo != nil {
		if nodeInfo.NodeName != "" {
			nodeName = nodeInfo.NodeName
		}
		if nodeInfo.InstanceURL != "" {
			instanceURL = nodeInfo.InstanceURL
		}
	}
	
	testAlert := &alerts.Alert{
		ID:           "test-alert",
		Type:         "cpu",
		Level:        "warning",
		ResourceID:   "test-email-config",
		ResourceName: "Email Configuration Test",
		Node:         nodeName,
		Instance:     instanceURL,
		Message:      "This is a test alert to verify your email notification settings are working correctly",
		Value:        85.5,
		Threshold:    80,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
		Metadata: map[string]interface{}{
			"resourceType": "test",
		},
	}
	
	switch method {
	case "email":
		if config == nil {
			return fmt.Errorf("email configuration is required")
		}
		
		log.Info().
			Bool("enabled", config.Enabled).
			Str("smtp", config.SMTPHost).
			Int("port", config.SMTPPort).
			Str("from", config.From).
			Int("toCount", len(config.To)).
			Msg("Testing email notification with provided config")
			
		if !config.Enabled {
			return fmt.Errorf("email notifications are not enabled in the provided configuration")
		}
		
		if config.SMTPHost == "" || config.From == "" {
			return fmt.Errorf("email configuration is incomplete: SMTP host and from address are required")
		}
		
		// Generate email using template
		subject, htmlBody, textBody := EmailTemplate([]*alerts.Alert{testAlert}, true)
		
		// Send using provided config
		n.sendHTMLEmail(subject, htmlBody, textBody, *config)
		return nil
		
	default:
		return fmt.Errorf("unsupported method for config-based testing: %s", method)
	}
}

// Stop gracefully stops the notification manager
func (n *NotificationManager) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// Cancel any pending group timer
	if n.groupTimer != nil {
		n.groupTimer.Stop()
		n.groupTimer = nil
	}
	
	// Clear pending alerts
	n.pendingAlerts = nil
	
	log.Info().Msg("NotificationManager stopped")
}