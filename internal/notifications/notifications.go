package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// WebhookDelivery tracks webhook delivery attempts for debugging
type WebhookDelivery struct {
	WebhookName   string    `json:"webhookName"`
	WebhookURL    string    `json:"webhookUrl"`
	Service       string    `json:"service"`
	AlertID       string    `json:"alertId"`
	Timestamp     time.Time `json:"timestamp"`
	StatusCode    int       `json:"statusCode"`
	Success       bool      `json:"success"`
	ErrorMessage  string    `json:"errorMessage,omitempty"`
	RetryAttempts int       `json:"retryAttempts"`
	PayloadSize   int       `json:"payloadSize"`
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
	publicURL    string // Full URL to access Pulse
	groupByGuest bool
	webhookHistory []WebhookDelivery // Keep last 100 webhook deliveries for debugging
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
	Provider   string   `json:"provider"`  // Email provider name (Gmail, SendGrid, etc.)
	SMTPHost   string   `json:"server"`    // Changed from smtpHost to server for frontend consistency
	SMTPPort   int      `json:"port"`      // Changed from smtpPort to port for frontend consistency
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	From       string   `json:"from"`
	To         []string `json:"to"`
	TLS        bool     `json:"tls"`
	StartTLS   bool     `json:"startTLS"`  // STARTTLS support
}

// WebhookConfig holds webhook settings
type WebhookConfig struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers"`
	Enabled  bool              `json:"enabled"`
	Service  string            `json:"service"`  // discord, slack, teams, etc.
	Template string            `json:"template"` // Custom payload template
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(publicURL string) *NotificationManager {
	if publicURL != "" {
		log.Info().Str("publicURL", publicURL).Msg("NotificationManager initialized with public URL")
	} else {
		log.Info().Msg("NotificationManager initialized without public URL - webhook links may not work")
	}
	return &NotificationManager{
		enabled:        true,
		cooldown:       5 * time.Minute,
		lastNotified:   make(map[string]time.Time),
		webhooks:       []WebhookConfig{},
		groupWindow:    30 * time.Second,
		pendingAlerts:  make([]*alerts.Alert, 0),
		groupByNode:    true,
		groupByGuest:   false,
		webhookHistory: make([]WebhookDelivery, 0, 100), // Pre-allocate for 100 entries
		publicURL:      publicURL,
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
	
	if len(n.webhooks) == 0 {
		return []WebhookConfig{}
	}

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
	
	log.Info().
		Str("alertID", alert.ID).
		Bool("enabled", n.enabled).
		Int("webhooks", len(n.webhooks)).
		Bool("emailEnabled", n.emailConfig.Enabled).
		Msg("SendAlert called")
	
	if !n.enabled {
		log.Debug().Msg("Notifications disabled, skipping")
		return
	}
	
	// Check cooldown
	lastTime, exists := n.lastNotified[alert.ID]
	if exists && time.Since(lastTime) < n.cooldown {
		log.Info().
			Str("alertID", alert.ID).
			Dur("timeSince", time.Since(lastTime)).
			Dur("cooldown", n.cooldown).
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

// sendHTMLEmailWithError sends an HTML email with multipart content and returns any error
func (n *NotificationManager) sendHTMLEmailWithError(subject, htmlBody, textBody string, config EmailConfig) error {
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
		StartTLS:      config.StartTLS, // Use the configured StartTLS setting
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
		return fmt.Errorf("failed to send email: %w", err)
	}
	
	log.Info().
		Strs("recipients", recipients).
		Int("recipientCount", len(recipients)).
		Msg("Email notification sent successfully")
	return nil
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
		StartTLS:      config.StartTLS, // Use the configured StartTLS setting
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
	
	// Check if webhook has a custom template first
	// Only use custom template if it's not empty
	if webhook.Template != "" && strings.TrimSpace(webhook.Template) != "" && len(alertList) > 0 {
		// Use custom template with enhanced message for grouped alerts
		alert := alertList[0]
		if len(alertList) > 1 {
			// Build a full list of all alerts
			summary := alert.Message
			otherAlerts := []string{}
			for i := 1; i < len(alertList); i++ { // Show ALL alerts
				otherAlerts = append(otherAlerts, fmt.Sprintf("• %s: %.1f%%", alertList[i].ResourceName, alertList[i].Value))
			}
			if len(otherAlerts) > 0 {
				// For custom templates, we need to escape newlines since they're likely
				// used in shell commands or other contexts that need escaping
				alert.Message = fmt.Sprintf("%s\\n\\n🔔 All %d alerts:\\n%s", summary, len(alertList), strings.Join(otherAlerts, "\\n"))
			}
		}
		
		enhanced := EnhancedWebhookConfig{
			WebhookConfig:   webhook,
			Service:         webhook.Service,
			PayloadTemplate: webhook.Template,
		}
		
		data := n.prepareWebhookData(alert, nil)
		
		// For Telegram webhooks (check URL pattern since service might be empty)
		if strings.Contains(webhook.URL, "api.telegram.org") {
			// Don't need to extract chat_id from URL since it's in the template
			// The template already has the chat_id embedded
		}
		
		jsonData, err = n.generatePayloadFromTemplateWithService(enhanced.PayloadTemplate, data, webhook.Service)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Int("alertCount", len(alertList)).
				Msg("Failed to generate grouped payload from custom template")
			return
		}
	} else if webhook.Service != "" && webhook.Service != "generic" && len(alertList) > 0 {
		// For service-specific webhooks, use the first alert with a note about others
		// For simplicity, send the first alert with a note about others
		// Most webhook services work better with single structured payloads
		alert := alertList[0]
		
		// Convert to enhanced webhook to use template
		enhanced := EnhancedWebhookConfig{
			WebhookConfig: webhook,
			Service:       webhook.Service,
		}
		
		// Get service template
		templates := GetWebhookTemplates()
		templateFound := false
		for _, tmpl := range templates {
			if tmpl.Service == webhook.Service {
				enhanced.PayloadTemplate = tmpl.PayloadTemplate
				templateFound = true
				break
			}
		}
		
		if templateFound {
			// Modify message if multiple alerts - but format differently for Discord
			if len(alertList) > 1 {
				summary := alert.Message
				otherAlerts := []string{}
				for i := 1; i < len(alertList); i++ {
					otherAlerts = append(otherAlerts, fmt.Sprintf("• %s: %.1f%%", alertList[i].ResourceName, alertList[i].Value))
				}
				if len(otherAlerts) > 0 {
					// For Discord, format as a single line list to avoid newline issues
					// Discord embeds don't render \n in description anyway
					if webhook.Service == "discord" {
						// Use comma-separated list for Discord
						alert.Message = fmt.Sprintf("%s | 🔔 %d alerts: %s", summary, len(alertList), strings.Join(otherAlerts, ", "))
					} else {
						// For other services, escape newlines properly
						alert.Message = fmt.Sprintf("%s\\n\\n🔔 All %d alerts:\\n%s", summary, len(alertList), strings.Join(otherAlerts, "\\n"))
					}
				}
			}
			
			// Prepare data and generate payload
			data := n.prepareWebhookData(alert, nil)
			
			// Handle service-specific requirements
			if webhook.Service == "telegram" {
				if chatID, err := extractTelegramChatID(webhook.URL); err == nil && chatID != "" {
					data.ChatID = chatID
				} else if err != nil {
					log.Error().
						Err(err).
						Str("webhook", webhook.Name).
						Msg("Failed to extract Telegram chat_id for grouped notification")
					return // Skip this webhook
				}
			} else if webhook.Service == "pagerduty" {
				if data.CustomFields == nil {
					data.CustomFields = make(map[string]interface{})
				}
				if routingKey, ok := webhook.Headers["routing_key"]; ok {
					data.CustomFields["routing_key"] = routingKey
				}
			}
			
			jsonData, err = n.generatePayloadFromTemplateWithService(enhanced.PayloadTemplate, data, webhook.Service)
			if err != nil {
				log.Error().
					Err(err).
					Str("webhook", webhook.Name).
					Int("alertCount", len(alertList)).
					Msg("Failed to generate payload for grouped alerts")
				return
			}
		} else {
			// No template found, use generic payload
			webhook.Service = "generic"
		}
	}
	
	// Use generic payload if no service or template not found
	// But ONLY if jsonData hasn't been set yet (from custom template)
	if jsonData == nil && (webhook.Service == "" || webhook.Service == "generic") {
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

	// For Telegram webhooks, strip chat_id from URL if present
	// The chat_id should only be in the JSON body, not the URL
	webhookURL := webhook.URL
	if webhook.Service == "telegram" && strings.Contains(webhookURL, "chat_id=") {
		if u, err := url.Parse(webhookURL); err == nil {
			q := u.Query()
			q.Del("chat_id") // Remove chat_id from query params
			u.RawQuery = q.Encode()
			webhookURL = u.String()
			log.Debug().
				Str("original", webhook.URL).
				Str("cleaned", webhookURL).
				Msg("Stripped chat_id from Telegram webhook URL")
		}
	}

	req, err := http.NewRequest(method, webhookURL, bytes.NewBuffer(jsonData))
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
	
	// Special handling for ntfy service
	if webhook.Service == "ntfy" {
		// Set Content-Type for ntfy (plain text)
		req.Header.Set("Content-Type", "text/plain")
		// Note: Dynamic headers for ntfy are set in sendWebhook for individual alerts
	}
	
	// Apply any custom headers from webhook config
	for key, value := range webhook.Headers {
		// Skip template-like headers (those with {{) to prevent errors
		if !strings.Contains(value, "{{") {
			req.Header.Set(key, value)
		}
	}

	// Debug log the payload for Telegram and Gotify webhooks
	if webhook.Service == "telegram" || webhook.Service == "gotify" {
		log.Debug().
			Str("webhook", webhook.Name).
			Str("service", webhook.Service).
			Str("url", webhookURL).
			Str("payload", string(jsonData)).
			Msg("Sending webhook with payload")
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

	// Read response body for logging
	var respBody bytes.Buffer
	respBody.ReadFrom(resp.Body)
	responseBody := respBody.String()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Info().
			Str("webhook", webhook.Name).
			Str("service", webhook.Service).
			Str("type", alertType).
			Int("status", resp.StatusCode).
			Int("payloadSize", len(jsonData)).
			Msg("Webhook notification sent successfully")
		
		// Log response body only in debug mode for successful requests
		if len(responseBody) > 0 {
			log.Debug().
				Str("webhook", webhook.Name).
				Str("response", responseBody).
				Msg("Webhook response body")
		}
	} else {
		log.Warn().
			Str("webhook", webhook.Name).
			Str("service", webhook.Service).
			Str("type", alertType).
			Int("status", resp.StatusCode).
			Str("response", responseBody).
			Msg("Webhook returned non-success status")
	}
}

// sendWebhook sends a webhook notification
func (n *NotificationManager) sendWebhook(webhook WebhookConfig, alert *alerts.Alert) {
	var jsonData []byte
	var err error
	
	// Check if webhook has a custom template first
	// Only use custom template if it's not empty
	if webhook.Template != "" && strings.TrimSpace(webhook.Template) != "" {
		// Use custom template provided by user
		enhanced := EnhancedWebhookConfig{
			WebhookConfig:   webhook,
			Service:         webhook.Service,
			PayloadTemplate: webhook.Template,
		}
		
		// Prepare data and generate payload
		data := n.prepareWebhookData(alert, nil)
		
		// For Telegram, still extract chat_id from URL if present
		if webhook.Service == "telegram" {
			if chatID, err := extractTelegramChatID(webhook.URL); err == nil && chatID != "" {
				data.ChatID = chatID
			} else if err != nil {
				log.Error().
					Err(err).
					Str("webhook", webhook.Name).
					Msg("Failed to extract Telegram chat_id - skipping webhook")
				return // Skip this webhook
			}
		}
		
		jsonData, err = n.generatePayloadFromTemplateWithService(enhanced.PayloadTemplate, data, webhook.Service)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Str("alertID", alert.ID).
				Msg("Failed to generate webhook payload from custom template")
			return
		}
	} else if webhook.Service != "" && webhook.Service != "generic" {
		// Check if this webhook has a service type and use the proper template
		// Convert to enhanced webhook to use template
		enhanced := EnhancedWebhookConfig{
			WebhookConfig: webhook,
			Service:       webhook.Service,
		}
		
		// Get service template
		templates := GetWebhookTemplates()
		templateFound := false
		for _, tmpl := range templates {
			if tmpl.Service == webhook.Service {
				enhanced.PayloadTemplate = tmpl.PayloadTemplate
				templateFound = true
				break
			}
		}
		
		// Only use template if found, otherwise fall back to generic
		if templateFound {
			// Prepare data and generate payload
			data := n.prepareWebhookData(alert, nil)
			
			// For Telegram, extract chat_id from URL if present
			if webhook.Service == "telegram" {
				chatID, err := extractTelegramChatID(webhook.URL)
				if err != nil {
					log.Error().
						Err(err).
						Str("webhook", webhook.Name).
						Str("url", webhook.URL).
						Msg("Failed to extract Telegram chat_id - webhook will fail")
					return // Skip this webhook rather than sending invalid payload
				}
				if chatID != "" {
					data.ChatID = chatID
					log.Debug().
						Str("webhook", webhook.Name).
						Str("chatID", chatID).
						Msg("Extracted Telegram chat_id from URL")
				}
			}
			
			// For PagerDuty, add routing key if present in URL or headers
			if webhook.Service == "pagerduty" {
				if data.CustomFields == nil {
					data.CustomFields = make(map[string]interface{})
				}
				// Check if routing key is in headers
				if routingKey, ok := webhook.Headers["routing_key"]; ok {
					data.CustomFields["routing_key"] = routingKey
				}
			}
			
			jsonData, err = n.generatePayloadFromTemplateWithService(enhanced.PayloadTemplate, data, webhook.Service)
			if err != nil {
				log.Error().
					Err(err).
					Str("webhook", webhook.Name).
					Str("service", webhook.Service).
					Str("alertID", alert.ID).
					Msg("Failed to generate webhook payload")
				return
			}
		} else {
			// No template found, use generic payload
			webhook.Service = "generic"
		}
	}
	
	// Use generic payload if no service or template not found
	// But ONLY if jsonData hasn't been set yet (from custom template)
	if jsonData == nil && (webhook.Service == "" || webhook.Service == "generic") {
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
	
	// Construct full Pulse URL if publicURL is configured
	// The Instance field should contain the full URL to the Pulse dashboard
	instance := ""
	if n.publicURL != "" {
		// Remove trailing slash from publicURL if present
		instance = strings.TrimRight(n.publicURL, "/")
	} else if alert.Instance != "" && (strings.HasPrefix(alert.Instance, "http://") || strings.HasPrefix(alert.Instance, "https://")) {
		// If publicURL is not set but alert.Instance contains a full URL, use it
		instance = alert.Instance
	}
	
	return WebhookPayloadData{
		ID:           alert.ID,
		Level:        string(alert.Level),
		Type:         alert.Type,
		ResourceName: alert.ResourceName,
		ResourceID:   alert.ResourceID,
		Node:         alert.Node,
		Instance:     instance,
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
	return n.generatePayloadFromTemplateWithService(templateStr, data, "")
}

// generatePayloadFromTemplateWithService renders the payload using Go templates with service-specific handling
func (n *NotificationManager) generatePayloadFromTemplateWithService(templateStr string, data WebhookPayloadData, service string) ([]byte, error) {
	// Create template with helper functions
	funcMap := template.FuncMap{
		"title": func(s string) string {
			// Replace deprecated strings.Title with proper title casing
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
		},
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

	// Skip JSON validation for services that use plain text payloads
	if service == "ntfy" {
		// ntfy uses plain text, not JSON
		return buf.Bytes(), nil
	}

	// Validate that the generated payload is valid JSON for other services
	var jsonCheck interface{}
	if err := json.Unmarshal(buf.Bytes(), &jsonCheck); err != nil {
		log.Error().
			Err(err).
			Str("payload", string(buf.Bytes())).
			Msg("Generated webhook payload is invalid JSON")
		return nil, fmt.Errorf("template produced invalid JSON: %w", err)
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


// extractTelegramChatID extracts and validates the chat_id from a Telegram webhook URL
func extractTelegramChatID(webhookURL string) (string, error) {
	if !strings.Contains(webhookURL, "chat_id=") {
		return "", fmt.Errorf("Telegram webhook URL missing chat_id parameter")
	}
	
	u, err := url.Parse(webhookURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}
	
	chatID := u.Query().Get("chat_id")
	if chatID == "" {
		return "", fmt.Errorf("chat_id parameter is empty")
	}
	
	// Validate that chat_id is numeric (Telegram chat IDs are always numeric)
	// Handle negative IDs (group chats) and positive IDs (private chats)
	if strings.HasPrefix(chatID, "-") {
		if !isNumeric(chatID[1:]) {
			return "", fmt.Errorf("chat_id must be numeric, got: %s", chatID)
		}
	} else if !isNumeric(chatID) {
		return "", fmt.Errorf("chat_id must be numeric, got: %s", chatID)
	}
	
	return chatID, nil
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return len(s) > 0
}

// ValidateWebhookURL validates that a webhook URL is safe and properly formed
func ValidateWebhookURL(webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}
	
	u, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	
	// Must be HTTP or HTTPS
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https protocol")
	}
	
	// Block localhost and private network ranges for security
	// Allow them only if explicitly configured (for testing)
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		log.Warn().
			Str("url", webhookURL).
			Msg("Webhook URL points to localhost - this may be intentional for testing")
	}
	
	// Check for private IP ranges (10.x.x.x, 172.16-31.x.x, 192.168.x.x)
	if strings.HasPrefix(host, "10.") || 
	   strings.HasPrefix(host, "192.168.") ||
	   (strings.HasPrefix(host, "172.") && isPrivateRange172(host)) {
		log.Warn().
			Str("url", webhookURL).
			Msg("Webhook URL points to private network - ensure this is intentional")
	}
	
	return nil
}

// isPrivateRange172 checks if an IP is in the 172.16.0.0/12 range
func isPrivateRange172(host string) bool {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return false
	}
	if parts[0] != "172" {
		return false
	}
	
	// Check if second octet is between 16 and 31
	if len(parts[1]) == 0 {
		return false
	}
	
	second := 0
	for _, char := range parts[1] {
		if char < '0' || char > '9' {
			return false
		}
		second = second*10 + int(char-'0')
	}
	
	return second >= 16 && second <= 31
}

// addWebhookDelivery adds a webhook delivery record to the history
func (n *NotificationManager) addWebhookDelivery(delivery WebhookDelivery) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// Add to history
	n.webhookHistory = append(n.webhookHistory, delivery)
	
	// Keep only last 100 entries
	if len(n.webhookHistory) > 100 {
		// Remove oldest entry
		n.webhookHistory = n.webhookHistory[1:]
	}
}

// GetWebhookHistory returns recent webhook delivery history
func (n *NotificationManager) GetWebhookHistory() []WebhookDelivery {
	n.mu.RLock()
	defer n.mu.RUnlock()
	
	// Return a copy to avoid concurrent access issues
	history := make([]WebhookDelivery, len(n.webhookHistory))
	copy(history, n.webhookHistory)
	return history
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

// SendTestWebhook sends a test notification to a specific webhook
func (n *NotificationManager) SendTestWebhook(webhook WebhookConfig) error {
	// Create a test alert for webhook testing with realistic values
	// Use the configured publicURL if available, otherwise use a placeholder
	instanceURL := n.publicURL
	if instanceURL == "" {
		instanceURL = "http://your-pulse-instance:7655"
	}
	
	testAlert := &alerts.Alert{
		ID:           "test-webhook-" + webhook.ID,
		Type:         "cpu",
		Level:        "warning",
		ResourceID:   "webhook-test",
		ResourceName: "Test Alert",
		Node:         "test-node",
		Instance:     instanceURL, // Use the actual Pulse URL
		Message:      fmt.Sprintf("This is a test alert from Pulse to verify your %s webhook is working correctly", webhook.Name),
		Value:        85.5,
		Threshold:    80.0,
		StartTime:    time.Now().Add(-5 * time.Minute), // Alert started 5 minutes ago
		LastSeen:     time.Now(),
		Metadata: map[string]interface{}{
			"webhookName": webhook.Name,
			"webhookURL":  webhook.URL,
			"testTime":    time.Now().Format(time.RFC3339),
		},
	}
	
	// Send the test webhook
	n.sendWebhook(webhook, testAlert)
	return nil
}

// SendTestNotificationWithConfig sends a test notification using provided config
func (n *NotificationManager) SendTestNotificationWithConfig(method string, config *EmailConfig, nodeInfo *TestNodeInfo) error {
	// Use actual node info if provided, otherwise use defaults
	nodeName := "test-node"
	instanceURL := n.publicURL
	if instanceURL == "" {
		instanceURL = "https://proxmox.local:8006"
	}
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
			Strs("to", config.To).
			Bool("smtpEmpty", config.SMTPHost == "").
			Bool("fromEmpty", config.From == "").
			Msg("Testing email notification with provided config")
			
		if !config.Enabled {
			return fmt.Errorf("email notifications are not enabled in the provided configuration")
		}
		
		if config.SMTPHost == "" || config.From == "" {
			return fmt.Errorf("email configuration is incomplete: SMTP host and from address are required")
		}
		
		// Generate email using template
		subject, htmlBody, textBody := EmailTemplate([]*alerts.Alert{testAlert}, true)
		
		// Send using provided config and return any error
		return n.sendHTMLEmailWithError(subject, htmlBody, textBody, *config)
		
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