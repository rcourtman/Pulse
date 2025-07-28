package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

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
	
	if len(config.To) == 0 {
		log.Warn().Msg("No email recipients configured")
		return
	}
	
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
	
	if len(config.To) == 0 {
		log.Warn().Msg("No email recipients configured")
		return
	}
	
	// Generate email using template
	subject, htmlBody, textBody := EmailTemplate([]*alerts.Alert{alert}, true)
	
	// Send using HTML-aware method
	n.sendHTMLEmail(subject, htmlBody, textBody, config)
}

// sendHTMLEmail sends an HTML email with multipart content
func (n *NotificationManager) sendHTMLEmail(subject, htmlBody, textBody string, config EmailConfig) {
	boundary := fmt.Sprintf("===============%d==", time.Now().UnixNano())
	
	// Compose multipart message
	msg := fmt.Sprintf("From: %s\r\n", config.From)
	msg += fmt.Sprintf("To: %s\r\n", strings.Join(config.To, ", "))
	msg += fmt.Sprintf("Subject: %s\r\n", subject)
	msg += fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	msg += "MIME-Version: 1.0\r\n"
	msg += fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
	msg += "\r\n"
	
	// Text part
	msg += fmt.Sprintf("--%s\r\n", boundary)
	msg += "Content-Type: text/plain; charset=\"UTF-8\"\r\n"
	msg += "Content-Transfer-Encoding: 7bit\r\n"
	msg += "\r\n"
	msg += textBody + "\r\n"
	
	// HTML part
	msg += fmt.Sprintf("--%s\r\n", boundary)
	msg += "Content-Type: text/html; charset=\"UTF-8\"\r\n"
	msg += "Content-Transfer-Encoding: 7bit\r\n"
	msg += "\r\n"
	msg += htmlBody + "\r\n"
	
	// End boundary
	msg += fmt.Sprintf("--%s--\r\n", boundary)
	
	// Send email
	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.SMTPHost)
	}
	
	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)
	err := smtp.SendMail(addr, auth, config.From, config.To, []byte(msg))
	
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to send email notification")
	} else {
		log.Info().
			Strs("recipients", config.To).
			Msg("Email notification sent")
	}
}

// sendEmailWithContent sends email with given content (plain text)
func (n *NotificationManager) sendEmailWithContent(subject, body string, config EmailConfig) {
	// For backward compatibility, send as plain text
	n.sendHTMLEmail(subject, "", body, config)
}

// sendGroupedWebhook sends a grouped webhook notification
func (n *NotificationManager) sendGroupedWebhook(webhook WebhookConfig, alertList []*alerts.Alert) {
	// Create webhook payload
	payload := map[string]interface{}{
		"alerts": alertList,
		"count": len(alertList),
		"timestamp": time.Now().Unix(),
		"source": "pulse-monitoring",
		"grouped": true,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Int("alertCount", len(alertList)).
			Msg("Failed to marshal grouped webhook payload")
		return
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
	// Create webhook payload
	payload := map[string]interface{}{
		"alert": alert,
		"timestamp": time.Now().Unix(),
		"source": "pulse-monitoring",
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Str("alertID", alert.ID).
			Msg("Failed to marshal webhook payload")
		return
	}
	
	// Send using common request logic
	n.sendWebhookRequest(webhook, jsonData, fmt.Sprintf("alert-%s", alert.ID))
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